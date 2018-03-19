package escluster

import (
	"bytes"
	"encoding/json"
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

	oshandler "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/handler"
)

// SrvBrokerName : service broker name for AI
const srvBrokerName = "escluster"

func init() {
	oshandler.Register(srvBrokerName, &SrvBrokerFreeHandler{})

	logger = lager.NewLogger(srvBrokerName)
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

// DoProvision required interface for service broker handler
func (handler *SrvBrokerHandler) DoProvision(etcdSaveResult chan error, instanceID string, details brokerapi.ProvisionDetails, planInfo oshandler.PlanInfo, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, oshandler.ServiceInfo, error) {
	// initialize connection to openshift firstly

	serviceSpec := brokerapi.ProvisionedServiceSpec{IsAsync: asyncAllowed}
	serviceInfo := oshandler.ServiceInfo{}

	//if asyncAllowed == false {
	//	return serviceSpec, serviceInfo, errors.New("Sync mode is not supported")
	//}
	serviceSpec.IsAsync = true

	//instanceIdInTempalte   := instanceID // todo: ok?
	instanceIDInTemplate := strings.ToLower(oshandler.NewThirteenLengthID())
	//serviceBrokerNamespace := ServiceBrokerNamespace
	serviceBrokerNamespace := oshandler.OC().Namespace()
	//srvUser := oshandler.NewElevenLengthID()
	srvPassword := oshandler.GenGUID()

	serviceInfo.Url = instanceIDInTemplate
	serviceInfo.Database = serviceBrokerNamespace // may be not needed
	//serviceInfo.User = srvUser
	serviceInfo.Password = srvPassword

	println()
	println("instanceIDInTemplate = ", instanceIDInTemplate)
	println("serviceBrokerNamespace = ", serviceBrokerNamespace)
	println()

	logger.Debug("serviceInfo->" + instanceIDInTemplate + "," + serviceBrokerNamespace + "," +
		serviceInfo.Service_name)

	go func() {
		err := <-etcdSaveResult
		if err != nil {
			return
		}

		output, err := createInstance(instanceIDInTemplate, serviceBrokerNamespace, srvPassword)

		if err != nil {
			destroySrvResources(output, serviceBrokerNamespace)

			return
		}

		// todo: maybe it is better to create a new job

		// todo: improve watch. Pod may be already running before watching!
		startSrvOrchestrationJob(&srvOrchestrationJob{
			cancelled:  false,
			cancelChan: make(chan struct{}),

			serviceInfo:     &serviceInfo,
			masterResources: output,
			moreResources:   nil,
		})

	}()

	serviceSpec.DashboardURL = ""

	//>>>
	serviceSpec.Credentials = getCredentialsOnPrivision(&serviceInfo)
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

	//master_res, _ := getRedisResources_Master (myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.Password)
	moreRes, _ := getAIResourcesMore(myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.Password)

	//ok := func(rc *kapi.ReplicationController) bool {
	//	if rc == nil || rc.Name == "" || rc.Spec.Replicas == nil || rc.Status.Replicas < *rc.Spec.Replicas {
	//		return false
	//	}
	//	return true
	//}
	ok := func(rc *kapiv1.ReplicationController) bool {
		println("rc.Name =", rc.Name)
		if rc == nil || rc.Name == "" || rc.Spec.Replicas == nil || rc.Status.Replicas < *rc.Spec.Replicas {
			return false
		}
		n, _ := startRunningPodsByLabels(myServiceInfo.Database, rc.Labels)
		println("n =", n)
		return n >= *rc.Spec.Replicas
	}

	//println("num_ok_rcs = ", num_ok_rcs)

	if ok(&moreRes.rc) && ok(&moreRes.rcSentinel) {
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

// DoUpdate update operation for service broker handler
func (handler *SrvBrokerHandler) DoUpdate(myServiceInfo *oshandler.ServiceInfo, planInfo oshandler.PlanInfo, callbackSaveNewInfo func(*oshandler.ServiceInfo) error, asyncAllowed bool) error {
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

		// ...

		println("to destroy resources")

		moreRes, _ := getAIResourcesMore(myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.Password)
		destroyAIResourcesMore(moreRes, myServiceInfo.Database)

		masterRes, _ := getSrvResources(myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.Password)
		destroySrvResources(masterRes, myServiceInfo.Database)
	}()

	return brokerapi.IsAsync(false), nil
}

// please note: the bsi may be still not fully initialized when calling the function.
func getCredentialsOnPrivision(myServiceInfo *oshandler.ServiceInfo) oshandler.Credentials {
	var moreRes aiResourcesMore
	err := loadAIResourcesMore(myServiceInfo.Url, myServiceInfo.Password, &moreRes)

	if err != nil {
		return oshandler.Credentials{}
	}

	clientPort := &moreRes.serviceSentinel.Spec.Ports[0]

	cluserName := "cluster-" + moreRes.serviceSentinel.Name
	host := fmt.Sprintf("%s.%s.%s", moreRes.serviceSentinel.Name, myServiceInfo.Database, oshandler.ServiceDomainSuffix(false))
	port := strconv.Itoa(clientPort.Port)
	//host := master_res.routeMQ.Spec.Host
	//port := "80"

	return oshandler.Credentials{
		Uri:      "",
		Hostname: host,
		Port:     port,
		//Username: myServiceInfo.User,
		Password: myServiceInfo.Password,
		Name:     cluserName,
	}
}

// DoBind bind interface for service broker handler
func (handler *SrvBrokerHandler) DoBind(myServiceInfo *oshandler.ServiceInfo, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, oshandler.Credentials, error) {
	// todo: handle errors

	// master_res may has been shutdown normally.

	moreRes, err := getAIResourcesMore(myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.Password)
	if err != nil {
		return brokerapi.Binding{}, oshandler.Credentials{}, err
	}

	clientPort := &moreRes.serviceSentinel.Spec.Ports[0]
	//if client_port == nil {
	//	return brokerapi.Binding{}, oshandler.Credentials{}, errors.New("client port not found")
	//}

	cluserName := "cluster-" + moreRes.serviceSentinel.Name
	host := fmt.Sprintf("%s.%s.%s", moreRes.serviceSentinel.Name, myServiceInfo.Database, oshandler.ServiceDomainSuffix(false))
	port := strconv.Itoa(clientPort.Port)
	//host := master_res.routeMQ.Spec.Host
	//port := "80"

	mycredentials := oshandler.Credentials{
		Uri:      "",
		Hostname: host,
		Port:     port,
		//Username: myServiceInfo.User,
		Password: myServiceInfo.Password,
		Name:     cluserName,
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

	serviceInfo *oshandler.ServiceInfo

	masterResources *srvResources
	moreResources   *aiResourcesMore
}

func (job *srvOrchestrationJob) cancel() {
	job.cancelMetex.Lock()
	defer job.cancelMetex.Unlock()

	if !job.cancelled {
		job.cancelled = true
		close(job.cancelChan)
	}
}

type watchPodStatus struct {
	// The type of watch update contained in the message
	Type string `json:"type"`
	// Pod details
	Object kapi.Pod `json:"object"`
}

func (job *srvOrchestrationJob) run() {
	serviceInfo := job.serviceInfo
	//pod := job.masterResources.pod
	rc := &job.masterResources.rc

	for {
		if job.cancelled {
			return
		}

		n, _ := startRunningPodsByLabels(serviceInfo.Database, rc.Labels)

		println("n = ", n, ", *job.masterResources.rc.Spec.Replicas = ", *rc.Spec.Replicas)

		if n < *rc.Spec.Replicas {
			time.Sleep(10 * time.Second)
		} else {
			break
		}
	}

	println("instance is running now")

	time.Sleep(5 * time.Second)

	if job.cancelled {
		return
	}

	// create more resources

	job.createRedisResourcesMore(serviceInfo.Url, serviceInfo.Database, serviceInfo.Password)
}

//=======================================================================
//
//=======================================================================

// SrvTemplateData template data for service broker
var SrvTemplateData []byte

func loadSrvResources(instanceID, srvPassword string, res *srvResources) error {
	if SrvTemplateData == nil {
		f, err := os.Open("es-cluster.yaml")
		if err != nil {
			return err
		}
		SrvTemplateData, err = ioutil.ReadAll(f)
		if err != nil {
			return err
		}

		srvImage := oshandler.EsclusterImage()
		srvImage = strings.TrimSpace(srvImage)
		if len(srvImage) > 0 {
			SrvTemplateData = bytes.Replace(
				SrvTemplateData,
				[]byte("http://docker-registry/es-cluster-image")
				),
				[]byte(srvImage),
				-1)
		}
	}

	yamlTemplates := SrvTemplateData

	yamlTemplates = bytes.Replace(yamlTemplates, []byte("instanceid"), []byte(instanceID), -1)
	//yamlTemplates = bytes.Replace(yamlTemplates, []byte("pass*****"), []byte(srvPassword), -1)

	logger.Debug("loadSrvResources(), yaml templates info->" + string(yamlTemplates))

	decoder := oshandler.NewYamlDecoder(yamlTemplates)
	decoder.Decode(&res.sts)

	return decoder.Err
}

// AITemplateDataMore additional template data for AI
var AITemplateDataMore []byte

func loadAIResourcesMore(instanceID, redisPassword string, res *aiResourcesMore) error {
	if AITemplateDataMore == nil {
		f, err := os.Open("redis-more.yaml")
		if err != nil {
			return err
		}
		AITemplateDataMore, err = ioutil.ReadAll(f)
		if err != nil {
			return err
		}
		aiImage := oshandler.RedisImage()
		aiImage = strings.TrimSpace(aiImage)
		if len(aiImage) > 0 {
			AITemplateDataMore = bytes.Replace(
				AITemplateDataMore,
				[]byte("http://redis-image-place-holder/redis-openshift-orchestration"),
				[]byte(aiImage),
				-1)
		}
	}

	// ...

	yamlTemplates := AITemplateDataMore

	yamlTemplates = bytes.Replace(yamlTemplates, []byte("instanceid"), []byte(instanceID), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("pass*****"), []byte(redisPassword), -1)

	//println("========= More yamlTemplates ===========")
	//println(string(yamlTemplates))
	//println()

	decoder := oshandler.NewYamlDecoder(yamlTemplates)
	decoder.Decode(&res.serviceSentinel).
		Decode(&res.rc).
		Decode(&res.rcSentinel)

	return decoder.Err
}

type srvResources struct {
	sts kapiv1b1.StatefulSet
}

type aiResourcesMore struct {
	serviceClient  kapiv1.Service
	serviceCluster kapiv1.Service
}

func createInstance(instanceID, serviceBrokerNamespace, srvPassword string) (*srvResources, error) {
	var input srvResources
	err := loadSrvResources(instanceID, srvPassword, &input)
	if err != nil {
		return nil, err
	}

	var output srvResources

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	// here, not use job.post
	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.Kv1b1Post(prefix+"/statefulset", &input.sts, &output.sts)

	if osr.Err != nil {
		msg = "createInstance(), create statefulset " + instanceID + " failed with error->"
		logger.Error(msg, osr.Err)
	}
	else {
		logger.Debug("createInstance(), create statefulset succeed->", output)
	}

	return &output, osr.Err
}

func getSrvResources(instanceID, serviceBrokerNamespace, srvPassword string) (*srvResources, error) {
	var output srvResources

	var input srvResources
	err := loadSrvResources(instanceID, srvPassword, &input)
	if err != nil {
		return &output, err
	}

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.Kv1b1Get(prefix+"/statefulset/"+input.sts.Name, &output.sts)

	if osr.Err != nil {
		logger.Error("getSrvResources", osr.Err)
	}

	return &output, osr.Err
}

func destroySrvResources(srvRes *srvResources, serviceBrokerNamespace string) {
	// todo: add to retry queue on fail

	//go func() {kdel (serviceBrokerNamespace, "pods", masterRes.pod.Name)}()
	go func() { kdelSts(serviceBrokerNamespace, &srvRes.sts) }()
}

func (job *srvOrchestrationJob) createRedisResourcesMore(instanceID, serviceBrokerNamespace, redisPassword string) error {
	var input aiResourcesMore
	err := loadAIResourcesMore(instanceID, redisPassword, &input)
	if err != nil {
		return err
	}

	var output aiResourcesMore

	/*
		osr := oshandler.NewOpenshiftREST(oshandler.OC())

		// here, not use job.post
		prefix := "/namespaces/" + serviceBrokerNamespace
		osr.
			KPost(prefix + "/services", &input.serviceSentinel, &output.serviceSentinel).
			KPost(prefix + "/replicationcontrollers", &input.rc, &output.rc).
			KPost(prefix + "/replicationcontrollers", &input.rcSentinel, &output.rcSentinel)

		if osr.Err != nil {
			logger.Error("createRedisResources_More", osr.Err)
		}
	*/

	go func() {
		if err := job.kpost(serviceBrokerNamespace, "services", &input.serviceSentinel, &output.serviceSentinel); err != nil {
			return
		}
		if err := job.kpost(serviceBrokerNamespace, "replicationcontrollers", &input.rc, &output.rc); err != nil {
			return
		}
		if err := job.kpost(serviceBrokerNamespace, "replicationcontrollers", &input.rcSentinel, &output.rcSentinel); err != nil {
			return
		}
	}()

	return nil
}

func getAIResourcesMore(instanceID, serviceBrokerNamespace, redisPassword string) (*aiResourcesMore, error) {
	var output aiResourcesMore

	var input aiResourcesMore
	err := loadAIResourcesMore(instanceID, redisPassword, &input)
	if err != nil {
		return &output, err
	}

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		KGet(prefix+"/services/"+input.serviceSentinel.Name, &output.serviceSentinel).
		KGet(prefix+"/replicationcontrollers/"+input.rc.Name, &output.rc).
		KGet(prefix+"/replicationcontrollers/"+input.rcSentinel.Name, &output.rcSentinel)

	if osr.Err != nil {
		logger.Error("getRedisResources_More", osr.Err)
	}

	return &output, osr.Err
}

func destroyAIResourcesMore(moreRes *aiResourcesMore, serviceBrokerNamespace string) {
	// todo: add to retry queue on fail

	go func() { kdel(serviceBrokerNamespace, "services", moreRes.serviceSentinel.Name) }()
	go func() { kdelRc(serviceBrokerNamespace, &moreRes.rc) }()
	go func() { kdelRc(serviceBrokerNamespace, &moreRes.rcSentinel) }()
}

//===============================================================
//
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

func kdelSts(serviceBrokerNamespace string, sts *kapiv1b1.StatefulSet) {
	// looks pods will be auto deleted when rc is deleted.

	if sts == nil || sts.Name == "" {
		return
	}

	println("to delete statefulset ", sts.Name)

	logger.Debug("kdelSts(), to delete statefulset " + sts.Name)

	uri := "/namespaces/" + serviceBrokerNamespace + "/statefulset/" + sts.Name

	// scale down to 0 firstly

	zero := 0
	sts.Spec.Replicas = &zero
	osr := oshandler.NewOpenshiftREST(oshandler.OC()).Kv1b1Put(uri, sts, nil)
	if osr.Err != nil {
		logger.Error("kdelSts(), scale down statefulset to 0", osr.Err)
		return
	}

	// start watching stateful status

	statuses, cancel, err := oshandler.OC().KWatch(uri)
	if err != nil {
		logger.Error("start watching HA rc", err)
		return
	}

	go func() {
		for {
			status, _ := <-statuses

			if status.Err != nil {
				logger.Error("watch HA redis rc error", status.Err)
				close(cancel)
				return
			}
			//else {
			//logger.Debug("watch redis HA rc, status.Info: " + string(status.Info))
			//}

			var wrcs watchReplicationControllerStatus
			if err := json.Unmarshal(status.Info, &wrcs); err != nil {
				logger.Error("parse master HA rc status", err)
				close(cancel)
				return
			}

			if wrcs.Object.Status.Replicas <= 0 {
				break
			}
		}

		// ...

		kdel(serviceBrokerNamespace, "statefulset", sts.Name)
	}()

	return
}

type watchReplicationControllerStatus struct {
	// The type of watch update contained in the message
	Type string `json:"type"`
	// RC details
	Object kapi.ReplicationController `json:"object"`
}

func startRunningPodsByLabels(serviceBrokerNamespace string, labels map[string]string) (int, error) {

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

		println("\n pods.Items[", i, "].Status.Phase =", pod.Status.Phase)

		if pod.Status.Phase == kapi.PodRunning {
			nrunnings++
		}
	}

	return nrunnings, nil
}
