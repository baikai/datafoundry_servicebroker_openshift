package escluster

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pivotal-cf/brokerapi"

	"github.com/pivotal-golang/lager"

	kapiv1 "k8s.io/kubernetes/pkg/api/v1"

	kapiv1b1 "k8s.io/kubernetes/pkg/apis/apps/v1beta1"

	routeapi "github.com/openshift/origin/route/api/v1"

	oshandler "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/handler"
)

// EsClusterServiceBrokerName : broker name should be service name + '_' + plan name
const EsClusterServiceBrokerName = "EsCluster_vol"

func init() {
	oshandler.Register(EsClusterServiceBrokerName, &SrvBrokerFreeHandler{})

	logger = lager.NewLogger(EsClusterServiceBrokerName)
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))
}

var logger lager.Logger

// SrvBrokerFreeHandler  free functionality
type SrvBrokerFreeHandler struct{}

// DoProvision create service instance per request
func (handler *SrvBrokerFreeHandler) DoProvision(etcdSaveResult chan error, instanceID string, details brokerapi.ProvisionDetails, planInfo oshandler.PlanInfo, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, oshandler.ServiceInfo, error) {
	return newSrvBrokerHandler().DoProvision(etcdSaveResult, instanceID, details, planInfo, asyncAllowed)
}

// DoLastOperation redo last operation against service instance
func (handler *SrvBrokerFreeHandler) DoLastOperation(myServiceInfo *oshandler.ServiceInfo) (brokerapi.LastOperation, error) {
	return newSrvBrokerHandler().DoLastOperation(myServiceInfo)
}

// DoUpdate update interface for free handler
func (handler *SrvBrokerFreeHandler) DoUpdate(myServiceInfo *oshandler.ServiceInfo, planInfo oshandler.PlanInfo, callbackSaveNewInfo func(*oshandler.ServiceInfo) error, asyncAllowed bool) error {
	return newSrvBrokerHandler().DoUpdate(myServiceInfo, planInfo, callbackSaveNewInfo, asyncAllowed)
}

// DoDeprovision interface for free handler
func (handler *SrvBrokerFreeHandler) DoDeprovision(myServiceInfo *oshandler.ServiceInfo, asyncAllowed bool) (brokerapi.IsAsync, error) {
	return newSrvBrokerHandler().DoDeprovision(myServiceInfo, asyncAllowed)
}

// DoBind interface for free handler
func (handler *SrvBrokerFreeHandler) DoBind(myServiceInfo *oshandler.ServiceInfo, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, oshandler.Credentials, error) {
	return newSrvBrokerHandler().DoBind(myServiceInfo, bindingID, details)
}

// DoUnbind interface for free handler
func (handler *SrvBrokerFreeHandler) DoUnbind(myServiceInfo *oshandler.ServiceInfo, mycredentials *oshandler.Credentials) error {
	return newSrvBrokerHandler().DoUnbind(myServiceInfo, mycredentials)
}

// SrvBrokerHandler  broker handler
type SrvBrokerHandler struct {
}

func newSrvBrokerHandler() *SrvBrokerHandler {
	return &SrvBrokerHandler{}
}

// parameters for containers
type podParas struct {
	//	mem, cpu       string
	replicas, disk string
}

// DoProvision required interface for service broker handler
func (handler *SrvBrokerHandler) DoProvision(etcdSaveResult chan error, instanceID string, details brokerapi.ProvisionDetails, planInfo oshandler.PlanInfo, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, oshandler.ServiceInfo, error) {

	serviceSpec := brokerapi.ProvisionedServiceSpec{IsAsync: asyncAllowed}
	serviceInfo := oshandler.ServiceInfo{}

	serviceSpec.IsAsync = true

	logger.Debug("DoProvision(), instanceID is " + instanceID)
	instanceIDInTemplate := strings.ToLower(oshandler.NewThirteenLengthID())

	serviceBrokerNamespace := oshandler.OC().Namespace()

	serviceInfo.Url = instanceIDInTemplate
	serviceInfo.Database = serviceBrokerNamespace // may be not needed

	var paras podParas
	if replicas, ok := details.Parameters["replicas"].(string); ok {
		logger.Debug("DoProvision(), replicas=" + replicas)
		paras.replicas = replicas
	} else {
		logger.Debug("DoProvision(), replicas information missed")
	}
	if volsize, ok := details.Parameters["volume"].(string); ok {
		logger.Debug("DoProvision(), volume size=" + volsize)
		paras.disk = volsize
	} else {
		logger.Debug("DoProvision, volume size missed")
	}

	println()
	println("instanceIDInTemplate = ", instanceIDInTemplate)
	println("serviceBrokerNamespace = ", serviceBrokerNamespace)
	println()

	logger.Debug("DoProvision(), serviceInfo: " + instanceIDInTemplate + "," + serviceBrokerNamespace + "," +
		serviceInfo.Service_name)

	// objects created for elastic search cluster
	var output *esResources

	go func() {

		err := <-etcdSaveResult

		if err != nil {
			logger.Error("DoProvision(), etcd save failed before creating instance", err)
			return
		}

		logger.Debug("DoProvison(), before creating instance")
		output, err = createInstance(instanceIDInTemplate, serviceBrokerNamespace, &paras)

		if err != nil {
			destroyEsResources(output, serviceBrokerNamespace)

			return
		}

		logger.Debug("DoProvision(), to start orchestration job, statefulset name is " +
			output.sts.Name + ", cluster service name is " + output.srvCluster.Name +
			", route name is " + output.route.Name)
		// todo: maybe it is better to create a new job

		// todo: improve watch. Pod may be already running before watching!
		startSrvOrchestrationJob(&srvOrchestrationJob{
			cancelled:  false,
			cancelChan: make(chan struct{}),

			serviceInfo: &serviceInfo,
			clusterRes:  output,
		})

	}()

	serviceSpec.DashboardURL = ""

	//>>>
	serviceSpec.Credentials = getCredentialsOnPrivision(&serviceInfo, output, &paras)
	//<<<

	return serviceSpec, serviceInfo, nil
}

// DoLastOperation interface for service broker handler
func (handler *SrvBrokerHandler) DoLastOperation(myServiceInfo *oshandler.ServiceInfo) (brokerapi.LastOperation, error) {
	// try to get state from running job
	job := getSrvOrchestrationJob(myServiceInfo.Url)
	if job != nil {
		return brokerapi.LastOperation{
			State:       brokerapi.InProgress,
			Description: "In progress .",
		}, nil
	}

	// assume in provisioning

	// the job may be finished or interrupted or running in another instance.

	esRes, _ := getSrvResources(myServiceInfo.Url, myServiceInfo.Database, nil)

	ok := func(sts *kapiv1b1.StatefulSet) bool {
		println("rc.Name =", sts.Name)
		if sts == nil || sts.Name == "" || sts.Spec.Replicas == nil || sts.Status.Replicas < *sts.Spec.Replicas {
			return false
		}
		n, _ := countRunningPodsByLabels(myServiceInfo.Database, sts.Labels)
		println("n =", n)
		return n >= *sts.Spec.Replicas
	}

	if ok(&esRes.sts) {
		return brokerapi.LastOperation{
			State:       brokerapi.Succeeded,
			Description: "Succeeded!",
		}, nil
	}

	return brokerapi.LastOperation{
		State:       brokerapi.InProgress,
		Description: "In progress.",
	}, nil

}

// DoUpdate update operation for service broker handler
func (handler *SrvBrokerHandler) DoUpdate(myServiceInfo *oshandler.ServiceInfo, planInfo oshandler.PlanInfo, callbackSaveNewInfo func(*oshandler.ServiceInfo) error, asyncAllowed bool) error {
	srvNamespace := myServiceInfo.Database
	var paras podParas

	/*
		currently pvc doesn't support automatic resize
		before k8s 1.9, heketi api could be called to expand glusterfs volume size directly
	*/
	esRes, _ := getSrvResources(myServiceInfo.Url, myServiceInfo.Database, nil)

	uri := "/namespaces/" + srvNamespace + "/statefulsets/" + esRes.sts.Name

	if replicas, ok := planInfo.MoreParameters["replicas"].(string); ok {
		logger.Debug("DoUpdate(), replicas=" + replicas)
		paras.replicas = replicas
	} else {
		logger.Debug("DoUpdate(), replicas not specified")
		paras.replicas = "0"
	}

	logger.Debug("DoUpdate(), current replicas is " + strconv.Itoa(int(*esRes.sts.Spec.Replicas)) + ", updated replicas is " + paras.replicas)

	upReplicas, err := strconv.Atoi(paras.replicas)
	if err != nil {
		return err
	}

	if upReplicas > 0 {
		if (*esRes.sts.Spec.Replicas) > int32(upReplicas) {
			logger.Debug("DoUpdate(), updated replicas is " + paras.replicas +
				" and less than current replicas " + strconv.Itoa(int(*esRes.sts.Spec.Replicas)))
			return errors.New("specified replicas is less than current replicas")
		} else if (*esRes.sts.Spec.Replicas) < int32(upReplicas) {
			*esRes.sts.Spec.Replicas = int32(upReplicas)

			osr := oshandler.NewOpenshiftREST(oshandler.OC()).Kv1b1Put(uri, esRes.sts, nil)
			if osr.Err != nil {
				logger.Error("DoUpdate(), scale statefulset failed.", osr.Err)
				return osr.Err
			}
			// wait until increase nodes completed
			for {
				n, _ := countRunningPodsByLabels(myServiceInfo.Database, esRes.sts.Labels)

				logger.Debug("DoUpdate(), pods already running is " + strconv.Itoa(int(n)))

				if n < *esRes.sts.Spec.Replicas {
					time.Sleep(10 * time.Second)
				} else {
					logger.Debug("DoUpdate(), increased nodes number to " + strconv.Itoa(int(*esRes.sts.Spec.Replicas)))
					break
				}
			}
		} else {
			logger.Info("DoUpdate, nodes number is not chnaged.")
		}
	}

	if volsize, ok := planInfo.MoreParameters["volume"].(string); ok {
		// need to fetch pvc information again
		if upReplicas > 0 {
			osr := oshandler.NewOpenshiftREST(oshandler.OC())

			prefix := "/namespaces/" + myServiceInfo.Database
			osr.KList(prefix+"/persistentvolumeclaims/", esRes.sts.Labels, &esRes.pvcs)

			if osr.Err != nil {
				logger.Error("DoUpdate(), refetch pvcs failed", osr.Err)
				return err
			}
		}

		logger.Debug("DoUpdate(), volume size=" + volsize)
		paras.disk = volsize
		// need to expand volume directly, pvc is not impacted, actually it's only object
		var vols = make([]oshandler.Volume, len(esRes.pvcs.Items))
		for i, pvc := range esRes.pvcs.Items {
			vols[i].Volume_name = pvc.Name
			quan := pvc.Spec.Resources.Requests["storage"]
			vols[i].Volume_size = int(quan.Value())
			logger.Debug("DoUpdate(), volume name=" + vols[i].Volume_name + ", volume size=" +
				strconv.Itoa(vols[i].Volume_size))
		}

		for _, vol := range myServiceInfo.Volumes {
			logger.Debug("DoUpdate(), " + strconv.Itoa(vol.Volume_size) + "," + strconv.Itoa(vol.Volume_size))
		}

		vsize, _ := strconv.Atoi(volsize)
		result := oshandler.StartExpandPvcVolumnJob(
			myServiceInfo.Url,
			myServiceInfo.Database,
			vols,
			vsize,
		)
		err := <-result
		if err != nil {
			logger.Error("DoUpdate(), expand volume failed", err)
			return err
		}
		logger.Debug("DoUpdate(), expand volume finished")
	} else {
		logger.Debug("DoUpdate(), volume size not specified")
	}

	return nil
}

// DoDeprovision de-provision interface for service broker handler
func (handler *SrvBrokerHandler) DoDeprovision(myServiceInfo *oshandler.ServiceInfo, asyncAllowed bool) (brokerapi.IsAsync, error) {
	go func() {
		job := getSrvOrchestrationJob(myServiceInfo.Url)
		if job != nil {
			job.cancel()

			// wait job to exit
			for {
				time.Sleep(7 * time.Second)
				if nil == getSrvOrchestrationJob(myServiceInfo.Url) {
					break
				}
			}
		}

		logger.Debug("DoDeprovision(), deprovision instance of " + myServiceInfo.Url + "," + myServiceInfo.Service_name)
		//esRes, _ := getStRes(myServiceInfo.Url, myServiceInfo.Database)
		esRes, _ := getSrvResources(myServiceInfo.Url, myServiceInfo.Database, nil)
		destroySrvResources(esRes, myServiceInfo.Database)
	}()

	return brokerapi.IsAsync(false), nil
}

// please note: the bsi may be still not fully initialized when calling the function.
func getCredentialsOnPrivision(myServiceInfo *oshandler.ServiceInfo, output *esResources, paras *podParas) oshandler.Credentials {

	var stsRes esResources
	err := loadSrvResources(myServiceInfo.Url, &stsRes, paras)

	if err != nil {
		logger.Error("getCredentialsOnProvision(), failed to load resources", err)
		return oshandler.Credentials{}
	}

	clusterName := stsRes.sts.Name
	//host := fmt.Sprintf("%s.%s.%s", stsRes.sts.Name, myServiceInfo.Database, oshandler.ServiceDomainSuffix(false))
	url := fmt.Sprintf("%s", stsRes.route.Spec.Host)
	host := url

	logger.Debug("getCredentialsOnProvision(), load yaml templates successfully, cluster name is " +
		clusterName + "," + host)

	return oshandler.Credentials{
		Uri:      url,
		Hostname: host,
		Name:     clusterName,
	}
}

// DoBind bind interface for service broker handler
func (handler *SrvBrokerHandler) DoBind(myServiceInfo *oshandler.ServiceInfo, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, oshandler.Credentials, error) {
	// todo: handle errors

	esRes, err := getSrvResources(myServiceInfo.Url, myServiceInfo.Database, nil)
	if err != nil {
		return brokerapi.Binding{}, oshandler.Credentials{}, err
	}

	//if client_port == nil {
	//	return brokerapi.Binding{}, oshandler.Credentials{}, errors.New("client port not found")
	//}

	cluserName := "cluster-" + esRes.sts.Name
	// host := fmt.Sprintf("%s.%s.%s", esRes.route.Spec.Host, myServiceInfo.Database, oshandler.ServiceDomainSuffix(false))
	url := fmt.Sprintf("%s", esRes.route.Spec.Host)
	host := url

	mycredentials := oshandler.Credentials{
		Uri:      "",
		Hostname: host,
		//Port:     port,
		//Username: myServiceInfo.User,
		//Password: myServiceInfo.Password,
		Name: cluserName,
	}

	myBinding := brokerapi.Binding{Credentials: mycredentials}

	return myBinding, mycredentials, nil
}

// DoUnbind unbind interface for service broker handler
func (handler *SrvBrokerHandler) DoUnbind(myServiceInfo *oshandler.ServiceInfo, mycredentials *oshandler.Credentials) error {
	// do nothing

	return nil
}

//=======================================================================
//
//=======================================================================

var srvOrchestrationJobs = map[string]*srvOrchestrationJob{}
var srvOrchestrationJobsMutex sync.Mutex

func getSrvOrchestrationJob(instanceID string) *srvOrchestrationJob {
	srvOrchestrationJobsMutex.Lock()
	job := srvOrchestrationJobs[instanceID]
	srvOrchestrationJobsMutex.Unlock()

	return job
}

func startSrvOrchestrationJob(job *srvOrchestrationJob) {
	srvOrchestrationJobsMutex.Lock()
	defer srvOrchestrationJobsMutex.Unlock()

	if srvOrchestrationJobs[job.serviceInfo.Url] == nil {
		srvOrchestrationJobs[job.serviceInfo.Url] = job
		go func() {
			job.run()

			srvOrchestrationJobsMutex.Lock()
			delete(srvOrchestrationJobs, job.serviceInfo.Url)
			srvOrchestrationJobsMutex.Unlock()
		}()
	}
}

type srvOrchestrationJob struct {
	//instanceId string // use serviceInfo.

	cancelled   bool
	cancelChan  chan struct{}
	cancelMetex sync.Mutex

	// service info for brokers
	serviceInfo *oshandler.ServiceInfo

	clusterRes *esResources
}

func (job *srvOrchestrationJob) cancel() {
	job.cancelMetex.Lock()
	defer job.cancelMetex.Unlock()

	if !job.cancelled {
		job.cancelled = true
		close(job.cancelChan)
	}
}

func (job *srvOrchestrationJob) run() {
	serviceInfo := job.serviceInfo

	/*
		Noted: It's a little tricky here, for statefulset, service should be created before
		pods as it need to find peers by resolving hostname, without service created as
		pre-condition, hostname could not be resolved
	*/
	job.createEsServices(serviceInfo.Url, serviceInfo.Database, serviceInfo.Password)

	sts := &job.clusterRes.sts

	logger.Debug("srvOrchestrationJob.run(), waiting for pods to be ready that belong to " +
		job.clusterRes.sts.Name)

	for {
		if job.cancelled {
			return
		}

		n, _ := countRunningPodsByLabels(serviceInfo.Database, sts.Labels)

		logger.Debug("srvOrchestrationJob.run(), pods already running is " + strconv.Itoa(int(n)))

		if n < *sts.Spec.Replicas {
			time.Sleep(10 * time.Second)
		} else {
			logger.Debug("srvOrchestrationJob.run(), statefulset.spec.Replicas=" +
				strconv.Itoa(int(*sts.Spec.Replicas)) + ", " + strconv.Itoa(int(n)) + " pods are running now.")
			break
		}
	}

	println("instance is running now")

	time.Sleep(5 * time.Second)

	if job.cancelled {
		return
	}

	logger.Debug("srvOrchestrationJob.run(), statefulset " + sts.Name + " is ready now")

}

//=======================================================================
//
//=======================================================================

// EsTemplateData template data for service broker
var EsTemplateData []byte

func loadSrvResources(instanceID string, res *esResources, paras *podParas) error {
	if EsTemplateData == nil || paras != nil {
		f, err := os.Open("es-cluster.yaml")
		if err != nil {
			logger.Error("loadSrvResources(), failed to open es-cluster.yaml", err)
			return err
		}
		EsTemplateData, err = ioutil.ReadAll(f)
		if err != nil {
			logger.Error("loadSrvResources(), failed to read template data", err)
			f.Close()
			return err
		}

		f.Close()

		esImage := oshandler.EsclusterImage()
		esImage = strings.TrimSpace(esImage)
		if len(esImage) > 0 {
			EsTemplateData = bytes.Replace(
				EsTemplateData,
				[]byte("http://docker-registry/es-cluster-image"),
				[]byte(esImage),
				-1)
		}

		epPostfix := oshandler.EndPointSuffix()
		epPostfix = strings.TrimSpace(epPostfix)
		if len(epPostfix) > 0 {
			EsTemplateData = bytes.Replace(
				EsTemplateData,
				[]byte("endpoint-postfix-place-holder"),
				[]byte(epPostfix),
				-1)
		}

		if paras == nil {
			paras = &podParas{"1", "1Gi"}
			logger.Debug("loadSrvResources(), initialize paras for pass decode check")
		}
		var repn, volsize string

		rep, err := strconv.Atoi(paras.replicas)
		logger.Debug("loadSrvResources(), replicas passed in is " + strconv.Itoa(rep))
		if rep <= 1 {
			logger.Error("loadSrvResources(), replicas passed in is invalid, set replicas to default value 3", err)
			// set replicas to default value
			rep = 2
		}

		volsize = paras.disk
		repn = paras.replicas
		logger.Debug("loadSrvResources(), updated parameters, replicas=" + paras.replicas +
			",disk=" + paras.disk)

		EsTemplateData = bytes.Replace(
			EsTemplateData,
			[]byte("replica-num"),
			[]byte(repn),
			-1)

		if len(paras.disk) > 0 {
			EsTemplateData = bytes.Replace(
				EsTemplateData,
				[]byte("disk-size"),
				[]byte(volsize),
				-1)
		}

		logger.Debug("loadSrvResources(), loaded yaml templates")
	}

	yamlTemplates := EsTemplateData

	logger.Debug("loadSrvResources(), instanceID passed in " + instanceID)

	yamlTemplates = bytes.Replace(yamlTemplates, []byte("instanceid"), []byte(instanceID), -1)

	logger.Debug("loadSrvResources(), yaml templates, " + string(yamlTemplates))

	decoder := oshandler.NewYamlDecoder(yamlTemplates)

	decoder.Decode(&res.srvCluster).Decode(&res.route).Decode(&res.sts)

	if decoder.Err != nil {
		logger.Error("loadSrvResources(), decode yaml template failed", decoder.Err)
	}

	return decoder.Err
}

// Deployment for elastic cluster includes 3 components, statefulset, service for cluster,
// and route for connecting outside of cluster
type esResources struct {
	sts        kapiv1b1.StatefulSet
	srvCluster kapiv1.Service
	route      routeapi.Route
	pvcs       kapiv1.PersistentVolumeClaimList
}

func createInstance(instanceID, serviceBrokerNamespace string, paras *podParas) (*esResources, error) {
	var input esResources
	err := loadSrvResources(instanceID, &input, paras)
	if err != nil {
		return nil, err
	}

	logger.Debug("createInstance(), load yaml templates succeed")

	var output esResources

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	// here, not use job.post
	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.Kv1b1Post(prefix+"/statefulsets", &input.sts, &output.sts)

	if osr.Err != nil {
		msg := "createInstance(), create statefulset " + instanceID + " failed with error " + osr.Err.Error()
		logger.Error(msg, osr.Err)
	} else {
		logger.Debug("createInstance(), create statefulset succeed, name is " + output.sts.Name)
	}

	// here, output.sts is assigned value from response, however, other objects such as services
	// are not assigned value. So need to assign value from input.
	output.srvCluster = input.srvCluster
	output.route = input.route

	return &output, osr.Err
}

func destroyEsResources(srvRes *esResources, serviceBrokerNamespace string) {
	// todo: add to retry queue on fail

	// delete statefulset only
	go func() { kdelSts(serviceBrokerNamespace, &srvRes.sts, &srvRes.pvcs) }()
}

func (job *srvOrchestrationJob) createEsServices(instanceID, serviceBrokerNamespace, srvPassword string) error {

	var output esResources

	logger.Debug("createEsServices(), prepare to create services for es-cluster " + instanceID)

	// for statefulset, services should be created before pods, no need to invoke this method
	// in another thread

	logger.Debug("createEsServices(), to create cluster services, name is " +
		job.clusterRes.srvCluster.Name + ", label is " +
		job.clusterRes.srvCluster.Labels["app"])
	if err := job.kpost(serviceBrokerNamespace, "services", job.clusterRes.srvCluster, &output.srvCluster); err != nil {
		logger.Debug("createEsServices(), create cluster service failed with " + err.Error())
		return err
	}
	job.clusterRes.srvCluster = output.srvCluster

	logger.Debug("createEsServices(), to create route for cluster service, name is " +
		job.clusterRes.route.Name + ", label is " + job.clusterRes.route.Labels["app"] +
		", host is " + job.clusterRes.route.Spec.Host)
	if err := job.opost(serviceBrokerNamespace, "routes", job.clusterRes.route, &output.route); err != nil {
		logger.Debug("createEsServices(), create route failed with " + err.Error())
		return err
	}
	job.clusterRes.route = output.route

	return nil
}

func getSrvResources(instanceID, serviceBrokerNamespace string, paras *podParas) (*esResources, error) {
	var output esResources

	var input esResources

	err := loadSrvResources(instanceID, &input, paras)
	if err != nil {
		return &output, err
	}

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		KGet(prefix+"/services/"+input.srvCluster.Name, &output.srvCluster).
		OGet(prefix+"/routes/"+input.route.Name, &output.route).
		Kv1b1Get(prefix+"/statefulsets/"+input.sts.Name, &output.sts).
		KList(prefix+"/persistentvolumeclaims/", input.sts.Labels, &output.pvcs)

	if osr.Err != nil {
		logger.Error("getSrvResources(), retrieving resources failed, ", osr.Err)
	}

	logger.Debug("getSrvResources(), cluster service, " + output.srvCluster.Name + "," + output.srvCluster.Kind)
	logger.Debug("getSrvResources(), route, " + output.route.Name + "," + output.route.Kind)
	logger.Debug("getSrvResources(), statefulset, " + output.sts.Name + "," + output.sts.Kind)
	// display all persistent volume claim
	listPvcs(&output.pvcs)
	return &output, osr.Err
}

func listPvcs(pvcs *kapiv1.PersistentVolumeClaimList) {
	var msg string
	for _, pvc := range pvcs.Items {
		msg += "pvc=" + pvc.Name + ", pv=" + pvc.Spec.VolumeName + "\n"
	}
	logger.Debug("listPvcs(), pvc is " + msg)
}

func destroySrvResources(esRes *esResources, serviceBrokerNamespace string) {
	// todo: add to retry queue on fail

	go func() { kdel(serviceBrokerNamespace, "services", esRes.srvCluster.Name) }()
	go func() { odel(serviceBrokerNamespace, "routes", esRes.route.Name) }()
	// need to delete pvcs after pods are deleted successully
	go func() { kdelSts(serviceBrokerNamespace, &esRes.sts, &esRes.pvcs) }()

}

//===============================================================
//node
//===============================================================

func (job *srvOrchestrationJob) kpost(serviceBrokerNamespace, typeName string, body interface{}, into interface{}) error {
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

func (job *srvOrchestrationJob) opost(serviceBrokerNamespace, typeName string, body interface{}, into interface{}) error {
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

	uri := fmt.Sprintf("/namespaces/%s/%s/%s", serviceBrokerNamespace, typeName, resName)
	i, n := 0, 5

	logger.Debug("kdel(), to delete object " + uri)

RETRY:
	var osr *oshandler.OpenshiftREST
	if typeName == "statefulsets" {
		osr = oshandler.NewOpenshiftREST(oshandler.OC()).Kv1b1Delete(uri, nil)
	} else {
		osr = oshandler.NewOpenshiftREST(oshandler.OC()).KDelete(uri, nil)
	}

	if osr.Err == nil {
		logger.Info("kdel(), delete " + uri + " succeeded")
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

func kdelSts(serviceBrokerNamespace string, sts *kapiv1b1.StatefulSet, pvcs *kapiv1.PersistentVolumeClaimList) {
	// looks pods will be auto deleted when rc is deleted.

	if sts == nil || sts.Name == "" {
		return
	}

	logger.Debug("kdelSts(), to delete statefulset " + sts.Name)

	uri := "/namespaces/" + serviceBrokerNamespace + "/statefulsets/" + sts.Name

	// scale down to 0 firstly
	var zero int32
	sts.Spec.Replicas = &zero
	osr := oshandler.NewOpenshiftREST(oshandler.OC()).Kv1b1Put(uri, sts, nil)
	if osr.Err != nil {
		logger.Error("kdelSts(), scale down statefulset to 0", osr.Err)
		return
	}

	// start watching stateful status
	statuses, cancel, err := oshandler.OC().Kv1b1Watch(uri)
	if err != nil {
		logger.Error("start watching statefulset "+sts.Name, err)
		return
	}

	go func() {
		for {
			status, _ := <-statuses

			if status.Err != nil {
				logger.Error("watch es-cluster statefulset error", status.Err)
				close(cancel)
				return
			}
			//else {dfProxyApiPrefix
			//logger.Debug("watch redis HA rc, status.Info: " + string(status.Info))
			//}

			var wsts watchStatefulsetStatus
			if err := json.Unmarshal(status.Info, &wsts); err != nil {
				logger.Error("parse statefulset status", err)
				close(cancel)
				return
			}

			if wsts.Object.Status.Replicas <= 0 {
				break
			}
		}

		// ...

		if err := kdel(serviceBrokerNamespace, "statefulsets", sts.Name); err != nil {
			logger.Error("kdelSts(), delete statefulset failed", err)
		}

		logger.Debug("kdelSts(), statefulset " + sts.Name + " deleted successfully. Start to release persistent storage ")
		for _, pvc := range pvcs.Items {
			kdel(serviceBrokerNamespace, "persistentvolumeclaims", pvc.Name)
		}

	}()

	return
}

type watchStatefulsetStatus struct {
	// The type of watch update contained in the message
	Type string `json:"type"`
	// Statefulset details
	Object kapiv1b1.StatefulSet `json:"object"`
}

func countRunningPodsByLabels(serviceBrokerNamespace string, labels map[string]string) (int32, error) {

	logger.Debug("countRunningPodsByLabels(), list pods in " + serviceBrokerNamespace)

	uri := "/namespaces/" + serviceBrokerNamespace + "/pods"

	pods := kapiv1.PodList{}

	osr := oshandler.NewOpenshiftREST(oshandler.OC()).KList(uri, labels, &pods)
	if osr.Err != nil {
		logger.Error("countRunningPodsByLabels(), list pods failed with err "+osr.Err.Error(), osr.Err)
		return 0, osr.Err
	}

	var nrunnings int32

	for i := range pods.Items {
		pod := &pods.Items[i]

		println("\n pods.Items[", i, "].Status.Phase =", pod.Status.Phase)

		if pod.Status.Phase == kapiv1.PodRunning {
			nrunnings++
		}
	}

	logger.Debug("countRunningPodsByLabels(), pods already running is " + strconv.Itoa(int(nrunnings)))

	return nrunnings, nil
}
