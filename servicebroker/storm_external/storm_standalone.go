package storm_external

import (
	"errors"
	"fmt"
	//marathon "github.com/gambol99/go-marathon"
	//kapi "golang.org/x/build/kubernetes/api"
	//"golang.org/x/build/kubernetes"
	//"golang.org/x/oauth2"
	//"net/http"
	//"net"
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/pivotal-cf/brokerapi"
	//"crypto/sha1"
	//"encoding/base64"
	//"text/template"
	//"io"
	"io/ioutil"
	"os"
	"sync"

	"github.com/pivotal-golang/lager"

	//"k8s.io/kubernetes/pkg/util/yaml"
	routeapi "github.com/openshift/origin/route/api/v1"
	kapi "k8s.io/kubernetes/pkg/api/v1"
	kutil "k8s.io/kubernetes/pkg/util"
	kresource "k8s.io/kubernetes/pkg/api/resource"

	oshandler "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/handler"
	//"github.com/asiainfoLDP/datafoundry_servicebroker_openshift/servicebroker/zookeeper"
	//"github.com/asiainfoLDP/datafoundry_servicebroker_openshift/servicebroker/zookeeper"
)

//==============================================================
//
//==============================================================

const StormServcieBrokerName_Standalone = "Storm_external_standalone"

func init() {
	oshandler.Register(StormServcieBrokerName_Standalone, &Storm_freeHandler{})

	logger = lager.NewLogger(StormServcieBrokerName_Standalone)
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))
}

var logger lager.Logger

//==============================================================
//
//==============================================================

const (
	Key_StormLocalHostname = "storm.local.hostname"
	
	SupervisorWorksEnvName = "NUM_WORKERS_PER_SUPERVISOR"

	Key_NumSupervisors   = "supervisors"
	Key_NumWorkers       = "workers"
	Key_SupervisorMemory = oshandler.Memory // "memory", don't change

	DefaultNumSupervisors          = 2
	DefaultNumWorkersPerSupervisor = 4
	DefaultSupervisorMemory        = 500
)

func RetrieveStormLocalHostname(m map[string]string) string {
	n := m[Key_StormLocalHostname]
	if n == "" {
		n = oshandler.NodeDomain(0)
	}
	return n
}

func BuildStormZkEntryRoot(instanceId string) string {
	//return fmt.Sprintf("/storm/%s", instanceId)
	// Baikai: it is best not to use the /storm prefix,
	// for it is the default pzk root ath for a storm cluster.
	return fmt.Sprintf("/%s", instanceId)
}

//==============================================================
//
//==============================================================

type Storm_freeHandler struct{}

func (handler *Storm_freeHandler) DoProvision(etcdSaveResult chan error, instanceID string, details brokerapi.ProvisionDetails, planInfo oshandler.PlanInfo, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, oshandler.ServiceInfo, error) {
	return newStormHandler().DoProvision(etcdSaveResult, instanceID, details, planInfo, asyncAllowed)
}

func (handler *Storm_freeHandler) DoLastOperation(myServiceInfo *oshandler.ServiceInfo) (brokerapi.LastOperation, error) {
	return newStormHandler().DoLastOperation(myServiceInfo)
}

func (handler *Storm_freeHandler) DoUpdate(myServiceInfo *oshandler.ServiceInfo, planInfo oshandler.PlanInfo, callbackSaveNewInfo func(*oshandler.ServiceInfo) error, asyncAllowed bool) error {
	return newStormHandler().DoUpdate(myServiceInfo, planInfo, callbackSaveNewInfo, asyncAllowed)
}

func (handler *Storm_freeHandler) DoDeprovision(myServiceInfo *oshandler.ServiceInfo, asyncAllowed bool) (brokerapi.IsAsync, error) {
	return newStormHandler().DoDeprovision(myServiceInfo, asyncAllowed)
}

func (handler *Storm_freeHandler) DoBind(myServiceInfo *oshandler.ServiceInfo, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, oshandler.Credentials, error) {
	return newStormHandler().DoBind(myServiceInfo, bindingID, details)
}

func (handler *Storm_freeHandler) DoUnbind(myServiceInfo *oshandler.ServiceInfo, mycredentials *oshandler.Credentials) error {
	return newStormHandler().DoUnbind(myServiceInfo, mycredentials)
}

//==============================================================
//
//==============================================================

// get number of supervisors
func retrieveNumNodesFromPlanInfo(planInfo oshandler.PlanInfo, defaultNodes int) (numNodes int, err error) {
	nodesSettings, ok := planInfo.ParameterSettings[Key_NumSupervisors]
	if !ok {
		err = errors.New(Key_NumSupervisors + " settings not found")
		numNodes = defaultNodes
		return
	}

	nodes64, err := oshandler.ParseInt64(planInfo.MoreParameters[Key_NumSupervisors])
	if err != nil {
		numNodes = defaultNodes
		return
	}
	numNodes = int(nodes64)

	if float64(numNodes) > nodesSettings.Max {
		err = fmt.Errorf("too many nodes specfied: %d > %f", numNodes, nodesSettings.Max)
	}

	if float64(numNodes) < nodesSettings.Default {
		err = fmt.Errorf("too few nodes specfied: %d < %f", numNodes, nodesSettings.Default)
	}

	numNodes = int(nodesSettings.Validate(float64(numNodes)))

	return
}

// get supervisor memory
func retrieveNodeMemoryFromPlanInfo(planInfo oshandler.PlanInfo, defaultMemory int) (nodeMemory int, err error) {
	memorySettings, ok := planInfo.ParameterSettings[Key_SupervisorMemory]
	if !ok {
		err = errors.New(Key_SupervisorMemory + " settings not found")
		nodeMemory = defaultMemory
		return
	}

	fMemory, err := oshandler.ParseFloat64(planInfo.MoreParameters[Key_SupervisorMemory])
	if err != nil {
		nodeMemory = defaultMemory
		return
	}

	if float64(fMemory) > memorySettings.Max {
		err = fmt.Errorf("too large memory specfied: %f > %f", fMemory, memorySettings.Max)
	}

	if float64(fMemory) < memorySettings.Default {
		err = fmt.Errorf("too small memory specfied: %f < %f", fMemory, memorySettings.Default)
	}

	fMemory = memorySettings.Validate(fMemory)
	nodeMemory = int(1000 * fMemory)

	return
}

// get number workers per supervisors
func retrieveNumWorkersPerSupervisorFromPlanInfo(planInfo oshandler.PlanInfo, defaultWorkers int) (numWorkers int, err error) {
	workersSettings, ok := planInfo.ParameterSettings[Key_NumWorkers]
	if !ok {
		err = errors.New(Key_NumWorkers + " settings not found")
		numWorkers = defaultWorkers
		return
	}

	workers64, err := oshandler.ParseInt64(planInfo.MoreParameters[Key_NumWorkers])
	if err != nil {
		numWorkers = defaultWorkers
		return
	}
	numWorkers = int(workers64)

	if float64(numWorkers) > workersSettings.Max {
		err = fmt.Errorf("too many workers specfied: %d > %f", numWorkers, workersSettings.Max)
	}

	if float64(numWorkers) < workersSettings.Default {
		err = fmt.Errorf("too few workers specfied: %d < %f", numWorkers, workersSettings.Default)
	}

	numWorkers = int(workersSettings.Validate(float64(numWorkers)))

	return
}

//==============================================================
//
//==============================================================

type Storm_Handler struct {
}

func newStormHandler() *Storm_Handler {
	return &Storm_Handler{}
}
	

func (handler *Storm_Handler) DoProvision(etcdSaveResult chan error, instanceID string, details brokerapi.ProvisionDetails, planInfo oshandler.PlanInfo, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, oshandler.ServiceInfo, error) {
	//初始化到openshift的链接

	serviceSpec := brokerapi.ProvisionedServiceSpec{IsAsync: asyncAllowed}
	serviceInfo := oshandler.ServiceInfo{}

	//numSuperVisors := 2
	//numWorkers := 4
	//supervisorMemory := 500
	numSupervisors, err := retrieveNumNodesFromPlanInfo(planInfo, DefaultNumSupervisors)
	if err != nil {
		println("retrieveNumNodesFromPlanInfo error: ", err.Error())
	}

	supervisorMemory, err := retrieveNodeMemoryFromPlanInfo(planInfo, DefaultSupervisorMemory) // Mi
	if err != nil {
		println("retrieveNodeMemoryFromPlanInfo error: ", err.Error())
	}
	
	numWorkersPerSupervisor, err := retrieveNumWorkersPerSupervisorFromPlanInfo(planInfo, DefaultNumWorkersPerSupervisor)
	if err != nil {
		println("retrieveNumWorkersPerSupervisorFromPlanInfo error: ", err.Error())
	}

	println("new storm cluster parameters: numSupervisors=", numSupervisors,
		 ", supervisorMemory=", supervisorMemory, "Mi",
		 ", numWorkersPerSupervisor=", numWorkersPerSupervisor)

	//if asyncAllowed == false {
	//	return serviceSpec, serviceInfo, errors.New("Sync mode is not supported")
	//}
	serviceSpec.IsAsync = true

	//instanceIdInTempalte   := instanceID // todo: ok?
	instanceIdInTempalte := strings.ToLower(oshandler.NewThirteenLengthID())
	//serviceBrokerNamespace := ServiceBrokerNamespace
	serviceBrokerNamespace := oshandler.OC().Namespace()
	//stormUser := oshandler.NewElevenLengthID()
	//stormPassword := oshandler.GenGUID()
	//zookeeperUser := "super" // oshandler.NewElevenLengthID()
	//zookeeperPassword := oshandler.GenGUID()

	println()
	println("instanceIdInTempalte = ", instanceIdInTempalte)
	println("serviceBrokerNamespace = ", serviceBrokerNamespace)
	println()

	serviceInfo.Url = instanceIdInTempalte
	serviceInfo.Database = serviceBrokerNamespace // may be not needed
	//serviceInfo.User = stormUser
	//serviceInfo.Password = stormPassword
	//serviceInfo.Admin_user = zookeeperUser
	//serviceInfo.Admin_password = zookeeperPassword
	serviceInfo.Miscs = map[string]string{
		Key_StormLocalHostname: oshandler.RandomNodeDomain(),
		Key_NumSupervisors:     strconv.Itoa(numSupervisors),
		Key_NumWorkers:         strconv.Itoa(numWorkersPerSupervisor),
		Key_SupervisorMemory:   strconv.Itoa(supervisorMemory),
	}

	//>> may be not optimized
	// var nimbus *stormResources_Nimbus
	nimbus := &stormResources_Nimbus{}
	err = loadStormResources_Nimbus(
		serviceInfo.Url,
		serviceInfo.Database,
		"",
		0,
		nimbus)
	if err != nil {
		return serviceSpec, oshandler.ServiceInfo{}, err
	}

	// var others *stormResources_UiSuperviserDrps
	others := &stormResources_UiSuperviserDrps{}
	err = loadStormResources_UiSuperviser(
		serviceInfo.Url,
		serviceInfo.Database,
		2, 4, 500, // the values are non-sense
		"",
		0,
		others)
	if err != nil {
		return serviceSpec, oshandler.ServiceInfo{}, err
	}
	//<<

	nimbus, others, err = createStormNodePorts(
		nimbus,
		others,
		serviceInfo.Database,
	)
	if err != nil {
		return serviceSpec, oshandler.ServiceInfo{}, err
	}

	// ...

	go func() {
		err := <-etcdSaveResult
		if err != nil {
			return
		}

		// nimbus storm
		//output, err := createStormResources_Nimbus(instanceIdInTempalte, serviceBrokerNamespace, stormUser, stormPassword)
		//if err != nil {
		//	destroyStormResources_Nimbus(output, serviceBrokerNamespace)
		//	return serviceSpec, serviceInfo, err
		//}
		// nimbus zookeeper

		//output, err := CreateZookeeperResources_Master(instanceIdInTempalte, serviceBrokerNamespace, zookeeperUser, zookeeperPassword)
		//if err != nil {
		//	DestroyZookeeperResources_Master(output, serviceBrokerNamespace)
		//
		//	return
		//}

		startStormOrchestrationJob(&stormOrchestrationJob{
			cancelled:  false,
			cancelChan: make(chan struct{}),

			stormHandler: handler,
			serviceInfo:  &serviceInfo,
			//zookeeperResources: output,

			nimbusNodePort: nimbus.serviceNodePort.Spec.Ports[0].NodePort,

			numSuperVisors:   numSupervisors,
			numWorkers:       numWorkersPerSupervisor,
			supervisorMemory: supervisorMemory,
		})

	}()

	serviceSpec.DashboardURL = ""

	//>>>
	serviceSpec.Credentials = getCredentialsOnPrivision(&serviceInfo, nimbus, others)
	//<<<

	return serviceSpec, serviceInfo, nil
}

func (handler *Storm_Handler) DoLastOperation(myServiceInfo *oshandler.ServiceInfo) (brokerapi.LastOperation, error) {
	// try to get state from running job
	job := getStormOrchestrationJob(myServiceInfo.Url)
	if job != nil {
		return brokerapi.LastOperation{
			State:       brokerapi.InProgress,
			Description: "In progress .",
		}, nil
	}

	// assume in provisioning

	// the job may be finished or interrupted or running in another instance.

	nimbus_res, _ := getStormResources_Nimbus(myServiceInfo.Url, myServiceInfo.Database)                 //, myServiceInfo.User, myServiceInfo.Password)
	uisuperviserdrps_res, _ := getStormResources_UiSuperviser(myServiceInfo.Url, myServiceInfo.Database) //, myServiceInfo.User, myServiceInfo.Password)

	//nodeport := oshandler.GetServicePortByName(&nimbus_res.nodeport, "storm-nimbus-port")
	//if nodeport == nil || nodeport.NodePort < 0 {
	//	return brokerapi.LastOperation{
	//		State:       brokerapi.InProgress,
	//		Description: "In progress ..",
	//	}, nil
	//}

	//ok := func(rc *kapi.ReplicationController) bool {
	//	if rc == nil || rc.Name == "" || rc.Spec.Replicas == nil || rc.Status.Replicas < *rc.Spec.Replicas {
	//		return false
	//	}
	//	return true
	//}
	ok := func(rc *kapi.ReplicationController) bool {
		if rc == nil || rc.Name == "" || rc.Spec.Replicas == nil || rc.Status.Replicas < *rc.Spec.Replicas {
			return false
		}
		n, _ := statRunningPodsByLabels(myServiceInfo.Database, rc.Labels)
		return n >= *rc.Spec.Replicas
	}

	//println("num_ok_rcs = ", num_ok_rcs)

	if ok(&nimbus_res.rc) && ok(&uisuperviserdrps_res.superviserrc) && ok(&uisuperviserdrps_res.uirc) && ok(&uisuperviserdrps_res.drpcrc) {
		return brokerapi.LastOperation{
			State:       brokerapi.Succeeded,
			Description: "Succeeded!",
		}, nil
	} else {
		return brokerapi.LastOperation{
			State:       brokerapi.InProgress,
			Description: "In progress.",
		}, nil
	}
}

func (handler *Storm_Handler) DoUpdate(myServiceInfo *oshandler.ServiceInfo, planInfo oshandler.PlanInfo, callbackSaveNewInfo func(*oshandler.ServiceInfo) error, asyncAllowed bool) error {

	var oldNumSupervisors, oldSupervisorMemory, oldNumWorkers int
	
	{
		nodes64, err := oshandler.ParseInt64(myServiceInfo.Miscs[Key_NumSupervisors])
		if err != nil {
			nodes64 = DefaultNumSupervisors
		}
		oldNumSupervisors = int(nodes64)
		
		//fMemory, err := oshandler.ParseFloat64(myServiceInfo.Miscs[Key_SupervisorMemory])
		nMemory, err := oshandler.ParseInt64(myServiceInfo.Miscs[Key_SupervisorMemory])
		if err != nil {
			nMemory = DefaultNumWorkersPerSupervisor
		}
		oldSupervisorMemory = int(nMemory)
		
		workers64, err := oshandler.ParseInt64(myServiceInfo.Miscs[Key_NumWorkers])
		if err != nil {
			workers64 = DefaultSupervisorMemory
		}
		oldNumWorkers = int(workers64)
	}
	
	numSupervisors, err := retrieveNumNodesFromPlanInfo(planInfo, oldNumSupervisors)
	if err != nil {
		println("[DoUpdate] retrieveNumNodesFromPlanInfo error: ", err.Error())
	}

	supervisorMemory, err := retrieveNodeMemoryFromPlanInfo(planInfo, oldSupervisorMemory) // Mi
	if err != nil {
		println("[DoUpdate] retrieveNodeMemoryFromPlanInfo error: ", err.Error())
	}
	
	numWorkersPerSupervisor, err := retrieveNumWorkersPerSupervisorFromPlanInfo(planInfo, oldNumWorkers)
	if err != nil {
		println("[DoUpdate] retrieveNumWorkersPerSupervisorFromPlanInfo error: ", err.Error())
	}

	println("[DoUpdate] new storm cluster parameters: numSupervisors=", numSupervisors,
		 ", supervisorMemory=", supervisorMemory, "Mi",
		 ", numWorkersPerSupervisor=", numWorkersPerSupervisor)
	println("[DoUpdate] old storm cluster parameters: oldNumSupervisors=", oldNumSupervisors,
		 ", oldSupervisorMemory=", oldSupervisorMemory, "Mi",
		 ", oldNumWorkers=", oldNumWorkers)
	
	if // numSupervisors != numWorkersPerSupervisor || numWorkersPerSupervisor != oldNumWorkers || supervisorMemory != oldSupervisorMemory ||
		true {
	
		println("To update storm external instance.")
		
		go func () {
			err := updateStormResources_Superviser(myServiceInfo.Url, myServiceInfo.Database,
				numSupervisors, numWorkersPerSupervisor, supervisorMemory)
			if err != nil {
				println("Failed to update storm external instance. Error:", err.Error())
				return
			}
			
			println("Storm external instance is updated.")
			
			myServiceInfo.Miscs[Key_NumSupervisors] = strconv.Itoa(numSupervisors)
			myServiceInfo.Miscs[Key_SupervisorMemory] = strconv.Itoa(supervisorMemory)
			myServiceInfo.Miscs[Key_NumWorkers] = strconv.Itoa(numWorkersPerSupervisor)
			
			err = callbackSaveNewInfo(myServiceInfo)
			if err != nil {
				logger.Error("Storm external instance is update but save info error", err)
			}
		
		}()
	} else {
		println("Storm external instance is not update.")
	}
	
	return nil
}

func (handler *Storm_Handler) DoDeprovision(myServiceInfo *oshandler.ServiceInfo, asyncAllowed bool) (brokerapi.IsAsync, error) {
	go func() {
		job := getStormOrchestrationJob(myServiceInfo.Url)
		if job != nil {
			job.cancel()

			// wait job to exit
			for {
				time.Sleep(7 * time.Second)
				if nil == getStormOrchestrationJob(myServiceInfo.Url) {
					break
				}
			}
		}

		// ...

		//println("to destroy zookeeper resources")
		//
		//zookeeper_res, _ := GetZookeeperResources_Master(myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.Admin_user, myServiceInfo.Admin_password)
		//DestroyZookeeperResources_Master(zookeeper_res, myServiceInfo.Database)

		// ...

		println("to destroy storm resources")

		nimbus_res, _ := getStormResources_Nimbus(myServiceInfo.Url, myServiceInfo.Database) //, myServiceInfo.User, myServiceInfo.Password)
		destroyStormResources_Nimbus(nimbus_res, myServiceInfo.Database)

		uisuperviserdrps_res, _ := getStormResources_UiSuperviser(myServiceInfo.Url, myServiceInfo.Database) //, myServiceInfo.User, myServiceInfo.Password)
		destroyStormResources_UiSuperviser(uisuperviserdrps_res, myServiceInfo.Database)
	}()

	return brokerapi.IsAsync(false), nil
}

// please note: the bsi may be still not fully initialized when calling the function.
func getCredentialsOnPrivision(myServiceInfo *oshandler.ServiceInfo, nimbus *stormResources_Nimbus, others *stormResources_UiSuperviserDrps) oshandler.Credentials {
	nimbus_host := RetrieveStormLocalHostname(myServiceInfo.Miscs)
	nimbus_port := strconv.Itoa(nimbus.serviceNodePort.Spec.Ports[0].NodePort)

	var uisuperviserdrps_res stormResources_UiSuperviserDrps
	err := loadStormResources_UiSuperviser(myServiceInfo.Url, myServiceInfo.Database, /*, stormUser, stormPassword*/
		2, 4, 500, // the values are non-sense
		"", 0, &uisuperviserdrps_res)
	if err != nil {
		return oshandler.Credentials{}
	}

	drpc_host := oshandler.RandomNodeAddress()
	drpc_port := strconv.Itoa(others.drpcserviceNodePort.Spec.Ports[0].NodePort)

	ui_host := uisuperviserdrps_res.uiroute.Spec.Host
	ui_port := "80"

	return oshandler.Credentials{
		Uri:      fmt.Sprintf("drpc: %s:%s, ui: %s:%s", drpc_host, drpc_port, ui_host, ui_port),
		Hostname: nimbus_host,
		Port:     nimbus_port,
		//Username: myServiceInfo.User,
		//Password: myServiceInfo.Password,
		// todo: need return zookeeper password?
	}
}

func (handler *Storm_Handler) DoBind(myServiceInfo *oshandler.ServiceInfo, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, oshandler.Credentials, error) {
	/*
		// todo: handle errors
		zookeeper_res, err := zookeeper.GetZookeeperResources_Master(myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.Admin_user, myServiceInfo.Admin_password)
		if err != nil {
			return brokerapi.Binding{}, oshandler.Credentials{}, err
		}

		zk_host, zk_port, err := zookeeper_res.ServiceHostPort(myServiceInfo.Database)
		if err != nil {
			return brokerapi.Binding{}, oshandler.Credentials{}, nil
		}

		uisuperviserdrps_res, err := getStormResources_UiSuperviser(myServiceInfo.Url, myServiceInfo.Database) //, myServiceInfo.User, myServiceInfo.Password)
		if err != nil {
			return brokerapi.Binding{}, oshandler.Credentials{}, err
		}

		ui_host := uisuperviserdrps_res.uiroute.Spec.Host
		ui_port := "80"

		nimbus_res, err := getStormResources_Nimbus(myServiceInfo.Url, myServiceInfo.Database) //, myServiceInfo.User, myServiceInfo.Password)
		if err != nil {
			return brokerapi.Binding{}, oshandler.Credentials{}, err
		}

		storm_nimbus_port := oshandler.GetServicePortByName(&nimbus_res.service, "storm-nimbus-port")
		if storm_nimbus_port == nil {
			return brokerapi.Binding{}, oshandler.Credentials{}, errors.New("storm-nimbus-port port not found")
		}

		host := fmt.Sprintf("%s.%s.%s", nimbus_res.service.Name, myServiceInfo.Database, oshandler.ServiceDomainSuffix(false))
		port := strconv.Itoa(storm_nimbus_port.Port)
		//host := nimbus_res.routeMQ.Spec.Host
		//port := "80"

		mycredentials := oshandler.Credentials{
			Uri:      fmt.Sprintf("storm-nimbus: %s:%s storm-UI: %s:%s zookeeper: %s:%s", host, port, ui_host, ui_port, zk_host, zk_port),
			Hostname: host,
			Port:     port,
			//Username: myServiceInfo.User,
			//Password: myServiceInfo.Password,
			// todo: need return zookeeper password?
		}
	*/

	nimbus_res, err := getStormResources_Nimbus(myServiceInfo.Url, myServiceInfo.Database) //, myServiceInfo.User, myServiceInfo.Password)
	if err != nil {
		return brokerapi.Binding{}, oshandler.Credentials{}, err
	}
	uisuperviserdrps_res, err := getStormResources_UiSuperviser(myServiceInfo.Url, myServiceInfo.Database) //, myServiceInfo.User, myServiceInfo.Password)
	if err != nil {
		return brokerapi.Binding{}, oshandler.Credentials{}, err
	}

	mycredentials := getCredentialsOnPrivision(myServiceInfo, nimbus_res, uisuperviserdrps_res)

	myBinding := brokerapi.Binding{Credentials: mycredentials}

	return myBinding, mycredentials, nil
}

func (handler *Storm_Handler) DoUnbind(myServiceInfo *oshandler.ServiceInfo, mycredentials *oshandler.Credentials) error {
	// do nothing

	return nil
}

//===============================================================
//
//===============================================================

var stormOrchestrationJobs = map[string]*stormOrchestrationJob{}
var stormOrchestrationJobsMutex sync.Mutex

func getStormOrchestrationJob(instanceId string) *stormOrchestrationJob {
	stormOrchestrationJobsMutex.Lock()
	defer stormOrchestrationJobsMutex.Unlock()

	return stormOrchestrationJobs[instanceId]
}

func startStormOrchestrationJob(job *stormOrchestrationJob) {
	stormOrchestrationJobsMutex.Lock()
	defer stormOrchestrationJobsMutex.Unlock()

	if stormOrchestrationJobs[job.serviceInfo.Url] == nil {
		stormOrchestrationJobs[job.serviceInfo.Url] = job
		go func() {
			job.run()

			stormOrchestrationJobsMutex.Lock()
			delete(stormOrchestrationJobs, job.serviceInfo.Url)
			stormOrchestrationJobsMutex.Unlock()
		}()
	}
}

type stormOrchestrationJob struct {
	//instanceId string // use serviceInfo.

	cancelled   bool
	cancelChan  chan struct{}
	cancelMetex sync.Mutex

	stormHandler *Storm_Handler

	serviceInfo *oshandler.ServiceInfo

	//zookeeperResources *ZookeeperResources_Master
	nimbusResources *stormResources_Nimbus

	nimbusNodePort int

	numSuperVisors, numWorkers, supervisorMemory int
}

func (job *stormOrchestrationJob) cancel() {
	job.cancelMetex.Lock()
	defer job.cancelMetex.Unlock()

	if !job.cancelled {
		job.cancelled = true
		close(job.cancelChan)
	}
}

func (job *stormOrchestrationJob) run() {

	println("  to create storm numbus resources")

	var err error
	job.nimbusResources, err = job.createStormResources_Nimbus(job.serviceInfo.Url, job.serviceInfo.Database, // job.serviceInfo.User, job.serviceInfo.Password)
		RetrieveStormLocalHostname(job.serviceInfo.Miscs), job.nimbusNodePort)
	if err != nil {
		// todo: add job.handler for other service brokers
		job.stormHandler.DoDeprovision(job.serviceInfo, true)
		return
	}

	// wait nimbus full initialized

	rc := &job.nimbusResources.rc

	ok := func(rc *kapi.ReplicationController) bool {
		if rc == nil || rc.Name == "" || rc.Spec.Replicas == nil {
			return false
		}

		if rc.Status.Replicas < *rc.Spec.Replicas {
			rc.Status.Replicas, _ = statRunningPodsByLabels(job.serviceInfo.Database, rc.Labels)

			println("rc = ", rc, ", rc.Status.Replicas = ", rc.Status.Replicas)
		}

		return rc.Status.Replicas >= *rc.Spec.Replicas
	}

	for {
		if ok(rc) {
			break
		}

		select {
		case <-job.cancelChan:
			return
		case <-time.After(15 * time.Second):
			// pod phase change will not trigger rc status change.
			// so need this case
			continue
		}
	}

	// ...

	if job.cancelled {
		return
	}

	time.Sleep(15 * time.Second) // maybe numbus is not fullly inited yet

	if job.cancelled {
		return
	}

	println("  to create storm ui+supervisor+drps resources")

	err = job.createStormResources_UiSuperviserDrpc(job.serviceInfo.Url, job.serviceInfo.Database, //, job.serviceInfo.User, job.serviceInfo.Password)
		job.numSuperVisors, job.numWorkers, job.supervisorMemory,
		RetrieveStormLocalHostname(job.serviceInfo.Miscs), job.nimbusNodePort)
	if err != nil {
		logger.Error("createStormResources_UiSuperviserDrpc", err)
	}
}

//=======================================================================
//
//=======================================================================

var StormTemplateData_Nimbus []byte = nil

func loadStormResources_Nimbus(instanceID, serviceBrokerNamespace /*, stormUser, stormPassword*/, stormLocalHostname string, thriftPort int, res *stormResources_Nimbus) error {
	if StormTemplateData_Nimbus == nil {
		f, err := os.Open("storm-external-nimbus.yaml")
		if err != nil {
			return err
		}
		StormTemplateData_Nimbus, err = ioutil.ReadAll(f)
		if err != nil {
			return err
		}
		storm_image := oshandler.StormExternalImage()
		storm_image = strings.TrimSpace(storm_image)
		if len(storm_image) > 0 {
			StormTemplateData_Nimbus = bytes.Replace(
				StormTemplateData_Nimbus,
				[]byte("http://storm-image-place-holder/storm-openshift-orchestration"),
				[]byte(storm_image),
				-1)
		}
	}

	// ...

	yamlTemplates := StormTemplateData_Nimbus

	yamlTemplates = bytes.Replace(yamlTemplates, []byte("instanceid"), []byte(instanceID), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("local-service-postfix-place-holder"),
		[]byte(serviceBrokerNamespace+oshandler.ServiceDomainSuffix(true)), -1)
	//yamlTemplates = bytes.Replace(yamlTemplates, []byte("dnsmasq*****"), []byte(oshandler.DnsmasqServer()), -1)

	yamlTemplates = bytes.Replace(yamlTemplates, []byte("external-zookeeper-server1*****"), []byte(oshandler.ExternalZookeeperServer(0)), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("external-zookeeper-server2*****"), []byte(oshandler.ExternalZookeeperServer(1)), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("external-zookeeper-server3*****"), []byte(oshandler.ExternalZookeeperServer(2)), -1)

	if strings.TrimSpace(stormLocalHostname) == "" {
		stormLocalHostname = "whatever"
	}
	if thriftPort == 0 {
		thriftPort = 12345
	}
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("zk-root*****"), []byte(BuildStormZkEntryRoot(instanceID)), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("storm-local-hostname*****"), []byte(stormLocalHostname), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("thrift-port*****"), []byte(strconv.Itoa(thriftPort)), -1)

	//  "externalIPs" : ['Apple', 'Orange', 'Strawberry', 'Mango']

	// oshandler.RandomNodeAddress()

	//println("========= Boot yamlTemplates ===========")
	//println(string(yamlTemplates))
	//println()

	decoder := oshandler.NewYamlDecoder(yamlTemplates)
	decoder.
		//Decode(&res.service).
		Decode(&res.serviceNodePort).
		Decode(&res.rc)

	return decoder.Err
}

var StormTemplateData_UiSuperviser []byte = nil

func loadStormResources_UiSuperviser(instanceID, serviceBrokerNamespace /*, stormUser, stormPassword*/ string,
	numSuperVisors, numWorkers, supervisorContainerMemory int,
	stormLocalHostname string, thriftPort int, res *stormResources_UiSuperviserDrps) error {
	if StormTemplateData_UiSuperviser == nil {
		f, err := os.Open("storm-external-others.yaml")
		if err != nil {
			return err
		}
		StormTemplateData_UiSuperviser, err = ioutil.ReadAll(f)
		if err != nil {
			return err
		}
		storm_image := oshandler.StormExternalImage()
		storm_image = strings.TrimSpace(storm_image)
		if len(storm_image) > 0 {
			StormTemplateData_UiSuperviser = bytes.Replace(
				StormTemplateData_UiSuperviser,
				[]byte("http://storm-image-place-holder/storm-openshift-orchestration"),
				[]byte(storm_image),
				-1)
		}
		endpoint_postfix := oshandler.EndPointSuffix()
		endpoint_postfix = strings.TrimSpace(endpoint_postfix)
		if len(endpoint_postfix) > 0 {
			StormTemplateData_UiSuperviser = bytes.Replace(
				StormTemplateData_UiSuperviser,
				[]byte("endpoint-postfix-place-holder"),
				[]byte(endpoint_postfix),
				-1)
		}
	}

	// ...

	yamlTemplates := StormTemplateData_UiSuperviser

	yamlTemplates = bytes.Replace(yamlTemplates, []byte("instanceid"), []byte(instanceID), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("local-service-postfix-place-holder"),
		[]byte(serviceBrokerNamespace+oshandler.ServiceDomainSuffix(true)), -1)

	yamlTemplates = bytes.Replace(yamlTemplates, []byte("external-zookeeper-server1*****"), []byte(oshandler.ExternalZookeeperServer(0)), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("external-zookeeper-server2*****"), []byte(oshandler.ExternalZookeeperServer(1)), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("external-zookeeper-server3*****"), []byte(oshandler.ExternalZookeeperServer(2)), -1)

	if strings.TrimSpace(stormLocalHostname) == "" {
		stormLocalHostname = "whatever"
	}
	if thriftPort == 0 {
		thriftPort = 12345
	}
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("zk-root*****"), []byte(BuildStormZkEntryRoot(instanceID)), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("storm-local-hostname*****"), []byte(stormLocalHostname), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("thrift-port*****"), []byte(strconv.Itoa(thriftPort)), -1)

	yamlTemplates = bytes.Replace(yamlTemplates, []byte("supervisor-memory*****"), []byte(strconv.Itoa(supervisorContainerMemory)), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("num-supervisors*****"), []byte(strconv.Itoa(numSuperVisors)), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("workers-per-supervisor*****"), []byte(strconv.Itoa(numWorkers)), -1)

	//println("========= Boot yamlTemplates ===========")
	//println(string(yamlTemplates))
	//println()

	decoder := oshandler.NewYamlDecoder(yamlTemplates)
	decoder.
		Decode(&res.superviserrc).
		Decode(&res.uiservice).
		Decode(&res.uiroute).
		Decode(&res.uirc).
		//Decode(&res.drpcservice).
		Decode(&res.drpcserviceNodePort).
		Decode(&res.drpcrc)

	return decoder.Err
}

type stormResources_Nimbus struct {
	//service  kapi.Service
	serviceNodePort kapi.Service
	rc              kapi.ReplicationController
}

type stormResources_UiSuperviserDrps struct {
	superviserrc kapi.ReplicationController

	uiservice kapi.Service
	uiroute   routeapi.Route
	uirc      kapi.ReplicationController

	//drpcservice kapi.Service
	drpcserviceNodePort kapi.Service
	drpcrc              kapi.ReplicationController
}

func createStormNodePorts(nimbus *stormResources_Nimbus, others *stormResources_UiSuperviserDrps, serviceBrokerNamespace string) (*stormResources_Nimbus, *stormResources_UiSuperviserDrps, error) {
	var nimbus_output stormResources_Nimbus
	var others_output stormResources_UiSuperviserDrps

	url := "/namespaces/" + serviceBrokerNamespace + "/services"

	// create nimbus nodeport
	var numbus_middle stormResources_Nimbus
	osr := oshandler.NewOpenshiftREST(oshandler.OC())
	osr.KPost(url, &nimbus.serviceNodePort, &numbus_middle.serviceNodePort)
	if osr.Err != nil {
		logger.Error("createNimbusNodePort", osr.Err)
		return &nimbus_output, &others_output, osr.Err
	}

	// modify nimbus nodeport to make port and targetPort as the same value of nodePort
	port := &numbus_middle.serviceNodePort.Spec.Ports[0]
	port.Port = port.NodePort
	port.TargetPort = kutil.IntOrString{
		Kind:   kutil.IntstrInt,
		IntVal: port.NodePort,
	}
	osr = oshandler.NewOpenshiftREST(oshandler.OC())
	osr.KPut(url+"/"+numbus_middle.serviceNodePort.Name, &numbus_middle.serviceNodePort, &nimbus_output.serviceNodePort)
	if osr.Err != nil {
		logger.Error("modifyNimbusNodePort", osr.Err)
		return &nimbus_output, &others_output, osr.Err
	}

	// create others nodeport
	osr = oshandler.NewOpenshiftREST(oshandler.OC())
	osr.KPost(url, &others.drpcserviceNodePort, &others_output.drpcserviceNodePort)
	if osr.Err != nil {
		logger.Error("createOthersNodePort", osr.Err)
		return &nimbus_output, &others_output, osr.Err
	}

	// ...
	return &nimbus_output, &others_output, nil
}

func (job *stormOrchestrationJob) createStormResources_Nimbus(instanceId, serviceBrokerNamespace /*, stormUser, stormPassword*/, stormLocalHostname string, thriftPort int) (*stormResources_Nimbus, error) {
	var input stormResources_Nimbus
	err := loadStormResources_Nimbus(instanceId, serviceBrokerNamespace, /*, stormUser, stormPassword*/
		stormLocalHostname, thriftPort, &input)
	if err != nil {
		return nil, err
	}

	var output stormResources_Nimbus

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	/*
		// here, not use job.post
		prefix := "/namespaces/" + serviceBrokerNamespace
		osr.
			KPost(prefix + "/services", &input.service, &output.service).
			KPost(prefix + "/replicationcontrollers", &input.rc, &output.rc)

		if osr.Err != nil {
			logger.Error("createStormResources_Nimbus", osr.Err)
		}
	*/

	//err = job.kpost(serviceBrokerNamespace, "services", &input.service, &output.service)
	//if err != nil {
	//	return &output, err
	//}
	//err = job.kpost(serviceBrokerNamespace, "services", &input.serviceNodePort, &output.serviceNodePort)
	//if err != nil {
	//	return &output, err
	//}
	err = job.kpost(serviceBrokerNamespace, "replicationcontrollers", &input.rc, &output.rc)
	if err != nil {
		return &output, err
	}

	return &output, osr.Err

	/*
		go func() {
			if err := job.kpost (serviceBrokerNamespace, "services", &input.service, &output.service); err != nil {
				return
			}
			if err := job.kpost (serviceBrokerNamespace, "replicationcontrollers", &input.rc, &output.rc); err != nil {
				return
			}
		}()

		return nil
	*/
}

func getStormResources_Nimbus(instanceId, serviceBrokerNamespace /*, stormUser, stormPassword*/ string) (*stormResources_Nimbus, error) {
	var output stormResources_Nimbus

	var input stormResources_Nimbus
	err := loadStormResources_Nimbus(instanceId, serviceBrokerNamespace /*, stormUser, stormPassword*/, "", 0, &input)
	if err != nil {
		return &output, err
	}

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		//KGet(prefix+"/services/"+input.service.Name, &output.service).
		KGet(prefix+"/services/"+input.serviceNodePort.Name, &output.serviceNodePort).
		KGet(prefix+"/replicationcontrollers/"+input.rc.Name, &output.rc)

	if osr.Err != nil {
		logger.Error("getStormResources_Nimbus", osr.Err)
	}

	return &output, osr.Err
}

func destroyStormResources_Nimbus(nimbusRes *stormResources_Nimbus, serviceBrokerNamespace string) {
	// todo: add to retry queue on failnodeport

	//go func() { kdel(serviceBrokerNamespace, "services", nimbusRes.service.Name) }()
	go func() { kdel(serviceBrokerNamespace, "services", nimbusRes.serviceNodePort.Name) }()
	go func() { kdel_rc(serviceBrokerNamespace, &nimbusRes.rc) }()
}

func (job *stormOrchestrationJob) createStormResources_UiSuperviserDrpc(instanceId, serviceBrokerNamespace /*, stormUser, stormPassword*/ string,
	numSuperVisors, numWorkers, supervisorContainerMemory int,
	stormLocalHostname string, thriftPort int) error {
	var input stormResources_UiSuperviserDrps

	err := loadStormResources_UiSuperviser(instanceId, serviceBrokerNamespace, /*, stormUser, stormPassword*/
		numSuperVisors, numWorkers, supervisorContainerMemory,
		stormLocalHostname, thriftPort, &input)
	if err != nil {
		//return nil, err
		return err
	}

	var output stormResources_UiSuperviserDrps
	/*
		osr := oshandler.NewOpenshiftREST(oshandler.OC())

		// here, not use job.post
		prefix := "/namespaces/" + serviceBrokerNamespace
		osr.
			KPost(prefix + "/services", &input.service, &output.service).
			KPost(prefix + "/replicationcontrollers", &input.rc, &output.rc)

		if osr.Err != nil {
			logger.Error("createStormResources_UiSuperviser", osr.Err)
		}

		return &output, osr.Err
	*/
	go func() {
		if err := job.kpost(serviceBrokerNamespace, "replicationcontrollers", &input.superviserrc, &output.superviserrc); err != nil {
			return
		}

		if err := job.kpost(serviceBrokerNamespace, "services", &input.uiservice, &output.uiservice); err != nil {
			return
		}
		if err := job.opost(serviceBrokerNamespace, "routes", &input.uiroute, &output.uiroute); err != nil {
			return
		}
		if err := job.kpost(serviceBrokerNamespace, "replicationcontrollers", &input.uirc, &output.uirc); err != nil {
			return
		}

		//if err := job.kpost(serviceBrokerNamespace, "services", &input.drpcserviceNodePort, &output.drpcserviceNodePort); err != nil {
		//	return
		//}
		if err := job.kpost(serviceBrokerNamespace, "replicationcontrollers", &input.drpcrc, &output.drpcrc); err != nil {
			return
		}
	}()

	return nil
}

func getStormResources_UiSuperviser(instanceId, serviceBrokerNamespace /*, stormUser, stormPassword*/ string) (*stormResources_UiSuperviserDrps, error) {
	var output stormResources_UiSuperviserDrps

	var input stormResources_UiSuperviserDrps
	err := loadStormResources_UiSuperviser(instanceId, serviceBrokerNamespace, /*, stormUser, stormPassword*/
		2, 4, 500, // the values are non-sense
		"", 0, &input)
	if err != nil {
		return &output, err
	}

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		KGet(prefix+"/replicationcontrollers/"+input.superviserrc.Name, &output.superviserrc).
		KGet(prefix+"/services/"+input.uiservice.Name, &output.uiservice).
		OGet(prefix+"/routes/"+input.uiroute.Name, &output.uiroute).
		KGet(prefix+"/replicationcontrollers/"+input.uirc.Name, &output.uirc).
		KGet(prefix+"/services/"+input.drpcserviceNodePort.Name, &output.drpcserviceNodePort).
		KGet(prefix+"/replicationcontrollers/"+input.drpcrc.Name, &output.drpcrc)

	if osr.Err != nil {
		logger.Error("getStormResources_UiSuperviser", osr.Err)
	}

	return &output, osr.Err
}

func destroyStormResources_UiSuperviser(uisuperviserRes *stormResources_UiSuperviserDrps, serviceBrokerNamespace string) {
	// todo: add to retry queue on fail
	// todo: the anonymous function wrappers are not essential.
	go func() { kdel_rc(serviceBrokerNamespace, &uisuperviserRes.superviserrc) }()
	go func() { kdel_rc(serviceBrokerNamespace, &uisuperviserRes.uirc) }()
	go func() { kdel_rc(serviceBrokerNamespace, &uisuperviserRes.drpcrc) }()
	go func() { odel(serviceBrokerNamespace, "routes", uisuperviserRes.uiroute.Name) }()
	go func() { kdel(serviceBrokerNamespace, "services", uisuperviserRes.uiservice.Name) }()
	go func() { kdel(serviceBrokerNamespace, "services", uisuperviserRes.drpcserviceNodePort.Name) }()
}



//============= update supervisor

func updateStormResources_Superviser(instanceId, serviceBrokerNamespace /*, stormUser, stormPassword*/ string,
	numSuperVisors, numWorkers, supervisorContainerMemory int) error {
	
	// 
	var input stormResources_UiSuperviserDrps
	err := loadStormResources_UiSuperviser(instanceId, serviceBrokerNamespace, /*, stormUser, stormPassword*/
		2, 4, 500, // the values are non-sense
		"", 0, &input)
	if err != nil {
		logger.Error("updateStormResources_Superviser. load error", err)
		return err
	}
	
	prefix := "/namespaces/" + serviceBrokerNamespace
	
	var middle stormResources_UiSuperviserDrps
	osr := oshandler.NewOpenshiftREST(oshandler.OC())
	osr.
		KGet(prefix+"/replicationcontrollers/"+input.superviserrc.Name, &middle.superviserrc)

	if osr.Err != nil {
		logger.Error("updateStormResources_Superviser. get error", osr.Err)
		return osr.Err
	}
	
	// todo: maybe should check the neccessarity to update ...
	
	// update
	
	if middle.superviserrc.Spec.Template == nil || len(middle.superviserrc.Spec.Template.Spec.Containers) == 0 {
		err = errors.New("rc.Template is nil or len(containers) == 0")
		logger.Error("updateStormResources_Superviser.", err)
		return err
	}
	
	{
		// image
		middle.superviserrc.Spec.Template.Spec.Containers[0].Image = oshandler.StormExternalImage()
	}
	
	{
		// number of supervisors
		middle.superviserrc.Spec.Replicas = &numSuperVisors
	}
	
	{
		// number of workers
		envs := middle.superviserrc.Spec.Template.Spec.Containers[0].Env
		found := false
		for i := range envs {
			if envs[i].Name == SupervisorWorksEnvName {
				envs[i].Value = strconv.Itoa(numWorkers)
				
				found = true
				break
			}
		}
		if !found {
			middle.superviserrc.Spec.Template.Spec.Containers[0].Env = append(envs, 
				kapi.EnvVar {Name: SupervisorWorksEnvName, Value: strconv.Itoa(numWorkers)},
			)
		}
	}
	
	{
		// supervisor memory limit
		q, err := kresource.ParseQuantity(strconv.Itoa(supervisorContainerMemory) + "Mi")
		if err != nil {
			logger.Error("updateStormResources_Superviser.", err)
			return err
		}
		
		middle.superviserrc.Spec.Template.Spec.Containers[0].Resources.Limits[kapi.ResourceMemory] = *q
	}
	
	
	var output stormResources_UiSuperviserDrps
	osr.
		KPut(prefix+"/replicationcontrollers/"+input.superviserrc.Name, &middle.superviserrc, &output.superviserrc)
	if osr.Err != nil {
		logger.Error("updateStormResources_Superviser. update error", osr.Err)
		return osr.Err
	}
	
	return nil
}

//===============================================================
//
//===============================================================

func (job *stormOrchestrationJob) kpost(serviceBrokerNamespace, typeName string, body interface{}, into interface{}) error {
	println("to create ", typeName)

	uri := fmt.Sprintf("/namespaces/%s/%s", serviceBrokerNamespace, typeName)
	i, n := 0, 5
RETRY:
	if job.cancelled {
		return nil
	}

	osr := oshandler.NewOpenshiftREST(oshandler.OC()).KPost(uri, body, into)
	if osr.Err == nil {
		logger.Info("create " + typeName + " succeeded")
	} else {
		i++
		if i < n {
			logger.Error(fmt.Sprintf("%d> create (%s) error", i, typeName), osr.Err)
			goto RETRY
		} else {
			logger.Error(fmt.Sprintf("create (%s) failed", typeName), osr.Err)
			return osr.Err
		}
	}

	return nil
}

func (job *stormOrchestrationJob) opost(serviceBrokerNamespace, typeName string, body interface{}, into interface{}) error {
	println("to create ", typeName)

	uri := fmt.Sprintf("/namespaces/%s/%s", serviceBrokerNamespace, typeName)
	i, n := 0, 5
RETRY:
	if job.cancelled {
		return nil
	}

	osr := oshandler.NewOpenshiftREST(oshandler.OC()).OPost(uri, body, into)
	if osr.Err == nil {
		logger.Info("create " + typeName + " succeeded")
	} else {
		i++
		if i < n {
			logger.Error(fmt.Sprintf("%d> create (%s) error", i, typeName), osr.Err)
			goto RETRY
		} else {
			logger.Error(fmt.Sprintf("create (%s) failed", typeName), osr.Err)
			return osr.Err
		}
	}

	return nil
}

func kdel(serviceBrokerNamespace, typeName, resName string) error {
	if resName == "" {
		return nil
	}

	println("to delete ", typeName, "/", resName)

	uri := fmt.Sprintf("/namespaces/%s/%s/%s", serviceBrokerNamespace, typeName, resName)
	i, n := 0, 5
RETRY:
	osr := oshandler.NewOpenshiftREST(oshandler.OC()).KDelete(uri, nil)
	if osr.Err == nil {
		logger.Info("delete " + uri + " succeeded")
	} else {
		i++
		if i < n {
			logger.Error(fmt.Sprintf("%d> delete (%s) error", i, uri), osr.Err)
			goto RETRY
		} else {
			logger.Error(fmt.Sprintf("delete (%s) failed", uri), osr.Err)
			return osr.Err
		}
	}

	return nil
}

func odel(serviceBrokerNamespace, typeName, resName string) error {
	if resName == "" {
		return nil
	}

	println("to delete ", typeName, "/", resName)

	uri := fmt.Sprintf("/namespaces/%s/%s/%s", serviceBrokerNamespace, typeName, resName)
	i, n := 0, 5
RETRY:
	osr := oshandler.NewOpenshiftREST(oshandler.OC()).ODelete(uri, nil)
	if osr.Err == nil {
		logger.Info("delete " + uri + " succeeded")
	} else {
		i++
		if i < n {
			logger.Error(fmt.Sprintf("%d> delete (%s) error", i, uri), osr.Err)
			goto RETRY
		} else {
			logger.Error(fmt.Sprintf("delete (%s) failed", uri), osr.Err)
			return osr.Err
		}
	}

	return nil
}

/*
func kdel_rc (serviceBrokerNamespace string, rc *kapi.ReplicationController) {
	kdel (serviceBrokerNamespace, "replicationcontrollers", rc.Name)
}
*/

func kdel_rc(serviceBrokerNamespace string, rc *kapi.ReplicationController) {
	// looks pods will be auto deleted when rc is deleted.

	if rc == nil || rc.Name == "" {
		return
	}

	println("to delete pods on replicationcontroller", rc.Name)

	uri := "/namespaces/" + serviceBrokerNamespace + "/replicationcontrollers/" + rc.Name

	// modfiy rc replicas to 0

	zero := 0
	rc.Spec.Replicas = &zero
	osr := oshandler.NewOpenshiftREST(oshandler.OC()).KPut(uri, rc, nil)
	if osr.Err != nil {
		logger.Error("modify HA rc", osr.Err)
		return
	}

	// start watching rc status

	statuses, cancel, err := oshandler.OC().KWatch(uri)
	if err != nil {
		logger.Error("start watching HA rc", err)
		return
	}

	go func() {
		for {
			status, _ := <-statuses

			if status.Err != nil {
				logger.Error("watch HA storm rc error", status.Err)
				close(cancel)
				return
			} else {
				//logger.Debug("watch storm HA rc, status.Info: " + string(status.Info))
			}

			var wrcs watchReplicationControllerStatus
			if err := json.Unmarshal(status.Info, &wrcs); err != nil {
				logger.Error("parse nimbus HA rc status", err)
				close(cancel)
				return
			}

			if wrcs.Object.Status.Replicas <= 0 {
				break
			}
		}

		// ...

		kdel(serviceBrokerNamespace, "replicationcontrollers", rc.Name)
	}()

	return
}

type watchReplicationControllerStatus struct {
	// The type of watch update contained in the message
	Type string `json:"type"`
	// RC details
	Object kapi.ReplicationController `json:"object"`
}

func statRunningPodsByLabels(serviceBrokerNamespace string, labels map[string]string) (int, error) {

	println("to list pods in", serviceBrokerNamespace)

	uri := "/namespaces/" + serviceBrokerNamespace + "/pods"

	pods := kapi.PodList{}

	osr := oshandler.NewOpenshiftREST(oshandler.OC()).KList(uri, labels, &pods)
	if osr.Err != nil {
		return 0, osr.Err
	}

	nrunnings := 0

	for i := range pods.Items {
		pod := &pods.Items[i]

		println("\n pods.Items[", i, "].Status.Phase =", pod.Status.Phase, "\n")

		if pod.Status.Phase == kapi.PodRunning {
			nrunnings++
		}
	}

	return nrunnings, nil
}
