package storm_pvc

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
	"github.com/pivotal-cf/brokerapi"
	"strconv"
	"strings"
	"time"
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

	oshandler "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/handler"
	"github.com/asiainfoLDP/datafoundry_servicebroker_openshift/servicebroker/zookeeper_pvc"
)

//==============================================================
//
//==============================================================

// WAERNING: the development of this service broker is not finished yet!

const StormServcieBrokerName_Standalone = "Storm_volumes_standalone"

func init() {
	oshandler.Register(StormServcieBrokerName_Standalone, &Storm_freeHandler{})

	logger = lager.NewLogger(StormServcieBrokerName_Standalone)
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))
}

var logger lager.Logger

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

type Storm_Handler struct {
}

func newStormHandler() *Storm_Handler {
	return &Storm_Handler{}
}

func (handler *Storm_Handler) DoProvision(etcdSaveResult chan error, instanceID string, details brokerapi.ProvisionDetails, planInfo oshandler.PlanInfo, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, oshandler.ServiceInfo, error) {
	//初始化到openshift的链接

	serviceSpec := brokerapi.ProvisionedServiceSpec{IsAsync: asyncAllowed}
	serviceInfo := oshandler.ServiceInfo{}

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
	zookeeperUser := "super" // oshandler.NewElevenLengthID()
	zookeeperPassword := oshandler.GenGUID()

	serviceInfo.Url = instanceIdInTempalte
	serviceInfo.Database = serviceBrokerNamespace // may be not needed
	//serviceInfo.User = stormUser
	//serviceInfo.Password = stormPassword
	serviceInfo.Admin_user = zookeeperUser
	serviceInfo.Admin_password = zookeeperPassword

	println()
	println("instanceIdInTempalte = ", instanceIdInTempalte)
	println("serviceBrokerNamespace = ", serviceBrokerNamespace)
	println()

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
		output, err := zookeeper_pvc.CreateZookeeperResources_Master(instanceIdInTempalte, serviceBrokerNamespace, zookeeperUser, zookeeperPassword, nil)
		if err != nil {
			zookeeper_pvc.DestroyZookeeperResources_Master(output, serviceBrokerNamespace)

			return
		}

		startStormOrchestrationJob(&stormOrchestrationJob{
			cancelled:  false,
			cancelChan: make(chan struct{}),

			stormHandler:       handler,
			serviceInfo:        &serviceInfo,
			zookeeperResources: output,
		})

	}()

	serviceSpec.DashboardURL = ""

	//>>>
	// storm pvc has not implemented yet.
	// serviceSpec.Credentials = getCredentialsOnPrivision(&serviceInfo)
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

	nimbus_res, _ := getStormResources_Nimbus(myServiceInfo.Url, myServiceInfo.Database)             //, myServiceInfo.User, myServiceInfo.Password)
	uisuperviser_res, _ := getStormResources_UiSuperviser(myServiceInfo.Url, myServiceInfo.Database) //, myServiceInfo.User, myServiceInfo.Password)

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

	if ok(&nimbus_res.rc) && ok(&uisuperviser_res.superviserrc) && ok(&uisuperviser_res.uirc) {
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
	return errors.New("not implemented")
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

		println("to destroy zookeeper resources")

		zookeeper_res, _ := zookeeper_pvc.GetZookeeperResources_Master(myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.Admin_user, myServiceInfo.Admin_password, nil)
		zookeeper_pvc.DestroyZookeeperResources_Master(zookeeper_res, myServiceInfo.Database)

		// ...

		println("to destroy storm resources")

		nimbus_res, _ := getStormResources_Nimbus(myServiceInfo.Url, myServiceInfo.Database) //, myServiceInfo.User, myServiceInfo.Password)
		destroyStormResources_Nimbus(nimbus_res, myServiceInfo.Database)

		uisuperviser_res, _ := getStormResources_UiSuperviser(myServiceInfo.Url, myServiceInfo.Database) //, myServiceInfo.User, myServiceInfo.Password)
		destroyStormResources_UiSuperviser(uisuperviser_res, myServiceInfo.Database)
	}()

	return brokerapi.IsAsync(false), nil
}

func (handler *Storm_Handler) DoBind(myServiceInfo *oshandler.ServiceInfo, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, oshandler.Credentials, error) {
	// todo: handle errors

	zookeeper_res, err := zookeeper_pvc.GetZookeeperResources_Master(myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.Admin_user, myServiceInfo.Admin_password, nil)
	if err != nil {
		return brokerapi.Binding{}, oshandler.Credentials{}, err
	}

	zk_host, zk_port, err := zookeeper_res.ServiceHostPort(myServiceInfo.Database)
	if err != nil {
		return brokerapi.Binding{}, oshandler.Credentials{}, nil
	}

	uisuperviser_res, err := getStormResources_UiSuperviser(myServiceInfo.Url, myServiceInfo.Database) //, myServiceInfo.User, myServiceInfo.Password)
	if err != nil {
		return brokerapi.Binding{}, oshandler.Credentials{}, err
	}

	ui_host := uisuperviser_res.uiroute.Spec.Host
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

	zookeeperResources *zookeeper_pvc.ZookeeperResources_Master
	nimbusResources    *stormResources_Nimbus
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
	println("-- stormOrchestrationJob start --")

	result, cancel, err := zookeeper_pvc.WatchZookeeperOrchestration(job.serviceInfo.Url, job.serviceInfo.Database, job.serviceInfo.Admin_user, job.serviceInfo.Admin_password, nil)
	if err != nil {
		zookeeper_res, _ := zookeeper_pvc.GetZookeeperResources_Master(job.serviceInfo.Url, job.serviceInfo.Database, job.serviceInfo.Admin_user, job.serviceInfo.Admin_password, nil)
		zookeeper_pvc.DestroyZookeeperResources_Master(zookeeper_res, job.serviceInfo.Database)
		return
	}

	var succeeded bool
	select {
	case <-job.cancelChan:
		close(cancel)
		return
	case succeeded = <-result:
		close(cancel)
		break
	}

	println("-- stormOrchestrationJob done, succeeded:", succeeded)

	if !succeeded {
		zookeeper_pvc.DestroyZookeeperResources_Master(job.zookeeperResources, job.serviceInfo.Database)
		return
	}

	// ...

	println("  to create storm nimbus resources")

	job.nimbusResources, err = job.createStormResources_Nimbus(job.serviceInfo.Url, job.serviceInfo.Database) //, job.serviceInfo.User, job.serviceInfo.Password)
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

	time.Sleep(10 * time.Second) // maybe numbus is not fullly inited yet

	if job.cancelled {
		return
	}

	println("  to create storm ui+supervisor resources")

	err = job.createStormResources_UiSuperviser(job.serviceInfo.Url, job.serviceInfo.Database) //, job.serviceInfo.User, job.serviceInfo.Password)
	if err != nil {
		logger.Error("createStormResources_UiSuperviser", err)
	}
}

//=======================================================================
//
//=======================================================================

var StormTemplateData_Nimbus []byte = nil

func loadStormResources_Nimbus(instanceID, serviceBrokerNamespace /*, stormUser, stormPassword*/ string, res *stormResources_Nimbus) error {
	if StormTemplateData_Nimbus == nil {
		f, err := os.Open("storm-nimbus.yaml")
		if err != nil {
			return err
		}
		StormTemplateData_Nimbus, err = ioutil.ReadAll(f)
		if err != nil {
			return err
		}
		storm_image := oshandler.StormImage()
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
		[]byte(serviceBrokerNamespace + oshandler.ServiceDomainSuffix(true)), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("dnsmasq*****"), []byte(oshandler.DnsmasqServer()), -1)

	//println("========= Boot yamlTemplates ===========")
	//println(string(yamlTemplates))
	//println()

	decoder := oshandler.NewYamlDecoder(yamlTemplates)
	decoder.
		Decode(&res.service).
		Decode(&res.rc)

	return decoder.Err
}

var StormTemplateData_UiSuperviser []byte = nil

func loadStormResources_UiSuperviser(instanceID, serviceBrokerNamespace /*, stormUser, stormPassword*/ string, res *stormResources_UiSuperviser) error {
	if StormTemplateData_UiSuperviser == nil {
		f, err := os.Open("storm-ui-supervisor.yaml")
		if err != nil {
			return err
		}
		StormTemplateData_UiSuperviser, err = ioutil.ReadAll(f)
		if err != nil {
			return err
		}
		storm_image := oshandler.StormImage()
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
		[]byte(serviceBrokerNamespace + oshandler.ServiceDomainSuffix(true)), -1)

	//println("========= Boot yamlTemplates ===========")
	//println(string(yamlTemplates))
	//println()

	decoder := oshandler.NewYamlDecoder(yamlTemplates)
	decoder.
		Decode(&res.superviserrc).
		Decode(&res.uiservice).
		Decode(&res.uiroute).
		Decode(&res.uirc)

	return decoder.Err
}

type stormResources_Nimbus struct {
	service kapi.Service
	rc      kapi.ReplicationController
}

type stormResources_UiSuperviser struct {
	superviserrc kapi.ReplicationController

	uiservice kapi.Service
	uiroute   routeapi.Route
	uirc      kapi.ReplicationController
}

func (job *stormOrchestrationJob) createStormResources_Nimbus(instanceId, serviceBrokerNamespace /*, stormUser, stormPassword*/ string) (*stormResources_Nimbus, error) {
	var input stormResources_Nimbus
	err := loadStormResources_Nimbus(instanceId, serviceBrokerNamespace /*, stormUser, stormPassword*/, &input)
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

	err = job.kpost(serviceBrokerNamespace, "services", &input.service, &output.service)
	if err != nil {
		return &output, err
	}
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
	err := loadStormResources_Nimbus(instanceId, serviceBrokerNamespace /*, stormUser, stormPassword*/, &input)
	if err != nil {
		return &output, err
	}

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		KGet(prefix+"/services/"+input.service.Name, &output.service).
		KGet(prefix+"/replicationcontrollers/"+input.rc.Name, &output.rc)

	if osr.Err != nil {
		logger.Error("getStormResources_Nimbus", osr.Err)
	}

	return &output, osr.Err
}

func destroyStormResources_Nimbus(nimbusRes *stormResources_Nimbus, serviceBrokerNamespace string) {
	// todo: add to retry queue on fail

	go func() { kdel(serviceBrokerNamespace, "services", nimbusRes.service.Name) }()
	go func() { kdel_rc(serviceBrokerNamespace, &nimbusRes.rc) }()
}

func (job *stormOrchestrationJob) createStormResources_UiSuperviser(instanceId, serviceBrokerNamespace /*, stormUser, stormPassword*/ string) error {
	var input stormResources_UiSuperviser

	err := loadStormResources_UiSuperviser(instanceId, serviceBrokerNamespace /*, stormUser, stormPassword*/, &input)
	if err != nil {
		//return nil, err
		return err
	}

	var output stormResources_UiSuperviser
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
	}()

	return nil
}

func getStormResources_UiSuperviser(instanceId, serviceBrokerNamespace /*, stormUser, stormPassword*/ string) (*stormResources_UiSuperviser, error) {
	var output stormResources_UiSuperviser

	var input stormResources_UiSuperviser
	err := loadStormResources_UiSuperviser(instanceId, serviceBrokerNamespace /*, stormUser, stormPassword*/, &input)
	if err != nil {
		return &output, err
	}

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		KGet(prefix+"/replicationcontrollers/"+input.superviserrc.Name, &output.superviserrc).
		KGet(prefix+"/services/"+input.uiservice.Name, &output.uiservice).
		OGet(prefix+"/routes/"+input.uiroute.Name, &output.uiroute).
		KGet(prefix+"/replicationcontrollers/"+input.uirc.Name, &output.uirc)

	if osr.Err != nil {
		logger.Error("getStormResources_UiSuperviser", osr.Err)
	}

	return &output, osr.Err
}

func destroyStormResources_UiSuperviser(uisuperviserRes *stormResources_UiSuperviser, serviceBrokerNamespace string) {
	// todo: add to retry queue on fail

	go func() { kdel_rc(serviceBrokerNamespace, &uisuperviserRes.superviserrc) }()
	go func() { kdel_rc(serviceBrokerNamespace, &uisuperviserRes.uirc) }()
	go func() { odel(serviceBrokerNamespace, "routes", uisuperviserRes.uiroute.Name) }()
	go func() { kdel(serviceBrokerNamespace, "services", uisuperviserRes.uiservice.Name) }()
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
