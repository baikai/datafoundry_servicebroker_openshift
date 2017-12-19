package ai

import (
	"fmt"
	"bytes"
	"encoding/json"
	"github.com/pivotal-cf/brokerapi"
	"strconv"
	"strings"
	"time"
	"io/ioutil"
	"os"
	"sync"

	"github.com/pivotal-golang/lager"

	kapi "k8s.io/kubernetes/pkg/api/v1"

	oshandler "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/handler"
)


const CommSrvBrokerName = "ai"

func init() {
	oshandler.Register(CommSrvBrokerName, &CommSrvBroker_freeHandler{})

	logger = lager.NewLogger(CommSrvBrokerName)
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))
}

var logger lager.Logger

type CommSrvBroker_freeHandler struct {}

func (handler *CommSrvBroker_freeHandler) DoProvision(etcdSaveResult chan error, instanceID string, details brokerapi.ProvisionDetails, planInfo oshandler.PlanInfo, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, oshandler.ServiceInfo, error) {
	return newCommSrvBrokerHandler().DoProvision(etcdSaveResult, instanceID, details, planInfo, asyncAllowed)
}

func (handler *CommSrvBroker_freeHandler) DoLastOperation(myServiceInfo *oshandler.ServiceInfo) (brokerapi.LastOperation, error) {
	return newCommSrvBrokerHandler().DoLastOperation(myServiceInfo)
}

func (handler *CommSrvBroker_freeHandler) DoUpdate(myServiceInfo *oshandler.ServiceInfo, planInfo oshandler.PlanInfo, callbackSaveNewInfo func(*oshandler.ServiceInfo) error, asyncAllowed bool) error {
	return newCommSrvBrokerHandler().DoUpdate(myServiceInfo, planInfo, callbackSaveNewInfo, asyncAllowed)
}

func (handler *CommSrvBroker_freeHandler) DoDeprovision(myServiceInfo *oshandler.ServiceInfo, asyncAllowed bool) (brokerapi.IsAsync, error) {
	return newCommSrvBrokerHandler().DoDeprovision(myServiceInfo, asyncAllowed)
}

func (handler *CommSrvBroker_freeHandler) DoBind(myServiceInfo *oshandler.ServiceInfo, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, oshandler.Credentials, error) {
	return newCommSrvBrokerHandler().DoBind(myServiceInfo, bindingID, details)
}

func (handler *CommSrvBroker_freeHandler) DoUnbind(myServiceInfo *oshandler.ServiceInfo, mycredentials *oshandler.Credentials) error {
	return newCommSrvBrokerHandler().DoUnbind(myServiceInfo, mycredentials)
}

type CommSrvBroker_Handler struct {
}

func newCommSrvBrokerHandler() *CommSrvBroker_Handler {
	return &CommSrvBroker_Handler{}
}


func (handler *CommSrvBroker_Handler) DoProvision(etcdSaveResult chan error, instanceID string, details brokerapi.ProvisionDetails, planInfo oshandler.PlanInfo, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, oshandler.ServiceInfo, error) {
	// initialize link to openshift firstly

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
	//srvUser := oshandler.NewElevenLengthID()
	srvPassword := oshandler.GenGUID()

	serviceInfo.Url = instanceIdInTempalte
	serviceInfo.Database = serviceBrokerNamespace // may be not needed
	//serviceInfo.User = srvUser
	serviceInfo.Password = srvPassword

	println()
	println("instanceIdInTempalte = ", instanceIdInTempalte)
	println("serviceBrokerNamespace = ", serviceBrokerNamespace)
	println()

	go func() {
		err := <-etcdSaveResult
		if err != nil {
			return
		}

		output, err := createInstance(instanceIdInTempalte, serviceBrokerNamespace, srvPassword)

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

func (handler *CommSrvBroker_Handler) DoLastOperation(myServiceInfo *oshandler.ServiceInfo) (brokerapi.LastOperation, error) {
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
	more_res, _ := getRedisResources_More(myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.Password)

	//ok := func(rc *kapi.ReplicationController) bool {
	//	if rc == nil || rc.Name == "" || rc.Spec.Replicas == nil || rc.Status.Replicas < *rc.Spec.Replicas {
	//		return false
	//	}
	//	return true
	//}
	ok := func(rc *kapi.ReplicationController) bool {
		println("rc.Name =", rc.Name)
		if rc == nil || rc.Name == "" || rc.Spec.Replicas == nil || rc.Status.Replicas < *rc.Spec.Replicas {
			return false
		}
		n, _ := startRunningPodsByLabels(myServiceInfo.Database, rc.Labels)
		println("n =", n)
		return n >= *rc.Spec.Replicas
	}

	//println("num_ok_rcs = ", num_ok_rcs)

	if ok(&more_res.rc) && ok(&more_res.rcSentinel) {
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

func (handler *CommSrvBroker_Handler) DoUpdate(myServiceInfo *oshandler.ServiceInfo, planInfo oshandler.PlanInfo, callbackSaveNewInfo func(*oshandler.ServiceInfo) error, asyncAllowed bool) error {
	return nil
}

func (handler *CommSrvBroker_Handler) DoDeprovision(myServiceInfo *oshandler.ServiceInfo, asyncAllowed bool) (brokerapi.IsAsync, error) {
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

		more_res, _ := getRedisResources_More(myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.Password)
		destroyRedisResources_More(more_res, myServiceInfo.Database)

		master_res, _ := getSrvResources(myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.Password)
		destroySrvResources(master_res, myServiceInfo.Database)
	}()

	return brokerapi.IsAsync(false), nil
}

// please note: the bsi may be still not fully initialized when calling the function.
func getCredentialsOnPrivision(myServiceInfo *oshandler.ServiceInfo) oshandler.Credentials {
	var more_res redisResources_More
	err := loadRedisResources_More(myServiceInfo.Url, myServiceInfo.Password, &more_res)

	if err != nil {
		return oshandler.Credentials{}
	}
	
	client_port := &more_res.serviceSentinel.Spec.Ports[0]

	cluser_name := "cluster-" + more_res.serviceSentinel.Name
	host := fmt.Sprintf("%s.%s.%s", more_res.serviceSentinel.Name, myServiceInfo.Database, oshandler.ServiceDomainSuffix(false))
	port := strconv.Itoa(client_port.Port)
	//host := master_res.routeMQ.Spec.Host
	//port := "80"

	return oshandler.Credentials{
		Uri:      "",
		Hostname: host,
		Port:     port,
		//Username: myServiceInfo.User,
		Password: myServiceInfo.Password,
		Name:     cluser_name,
	}
}

func (handler *CommSrvBroker_Handler) DoBind(myServiceInfo *oshandler.ServiceInfo, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, oshandler.Credentials, error) {
	// todo: handle errors

	// master_res may has been shutdown normally.

	more_res, err := getRedisResources_More(myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.Password)
	if err != nil {
		return brokerapi.Binding{}, oshandler.Credentials{}, err
	}

	client_port := &more_res.serviceSentinel.Spec.Ports[0]
	//if client_port == nil {
	//	return brokerapi.Binding{}, oshandler.Credentials{}, errors.New("client port not found")
	//}

	cluser_name := "cluster-" + more_res.serviceSentinel.Name
	host := fmt.Sprintf("%s.%s.%s", more_res.serviceSentinel.Name, myServiceInfo.Database, oshandler.ServiceDomainSuffix(false))
	port := strconv.Itoa(client_port.Port)
	//host := master_res.routeMQ.Spec.Host
	//port := "80"

	mycredentials := oshandler.Credentials{
		Uri:      "",
		Hostname: host,
		Port:     port,
		//Username: myServiceInfo.User,
		Password: myServiceInfo.Password,
		Name:     cluser_name,
	}

	myBinding := brokerapi.Binding{Credentials: mycredentials}

	return myBinding, mycredentials, nil
}

func (handler *CommSrvBroker_Handler) DoUnbind(myServiceInfo *oshandler.ServiceInfo, mycredentials *oshandler.Credentials) error {
	// do nothing

	return nil
}

//=======================================================================
//
//=======================================================================

var srvOrchestrationJobs = map[string]*srvOrchestrationJob{}
var srvOrchestrationJobsMutex sync.Mutex

func getSrvOrchestrationJob(instanceId string) *srvOrchestrationJob {
	srvOrchestrationJobsMutex.Lock()
	job := srvOrchestrationJobs[instanceId]
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
	moreResources   *redisResources_More
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

	job.createRedisResources_More(serviceInfo.Url, serviceInfo.Database, serviceInfo.Password)
}

//=======================================================================
//
//=======================================================================

var SrvTemplateData []byte = nil

func loadSrvResources(instanceID, srvPassword string, res *srvResources) error {
	if SrvTemplateData == nil {
		f, err := os.Open("redis-master.yaml")
		if err != nil {
			return err
		}
		SrvTemplateData, err = ioutil.ReadAll(f)
		if err != nil {
			return err
		}

		srv_image := oshandler.RedisImage()
		srv_image = strings.TrimSpace(srv_image)
		if len(srv_image) > 0 {
			SrvTemplateData = bytes.Replace(
				SrvTemplateData,
				[]byte("http://redis-image-place-holder/redis-openshift-orchestration"),
				[]byte(srv_image),
				-1)
		}
	}

	yamlTemplates := SrvTemplateData

	yamlTemplates = bytes.Replace(yamlTemplates, []byte("instanceid"), []byte(instanceID), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("pass*****"), []byte(srvPassword), -1)

	//println("========= Boot yamlTemplates ===========")
	//println(string(yamlTemplates))
	//println()

	decoder := oshandler.NewYamlDecoder(yamlTemplates)
	decoder.
		//Decode(&res.pod)
		Decode(&res.rc)

	return decoder.Err
}

var RedisTemplateData_More []byte = nil

func loadRedisResources_More(instanceID, redisPassword string, res *redisResources_More) error {
	if RedisTemplateData_More == nil {
		f, err := os.Open("redis-more.yaml")
		if err != nil {
			return err
		}
		RedisTemplateData_More, err = ioutil.ReadAll(f)
		if err != nil {
			return err
		}
		redis_image := oshandler.RedisImage()
		redis_image = strings.TrimSpace(redis_image)
		if len(redis_image) > 0 {
			RedisTemplateData_More = bytes.Replace(
				RedisTemplateData_More,
				[]byte("http://redis-image-place-holder/redis-openshift-orchestration"),
				[]byte(redis_image),
				-1)
		}
	}

	// ...

	yamlTemplates := RedisTemplateData_More

	yamlTemplates = bytes.Replace(yamlTemplates, []byte("instanceid"), []byte(instanceID), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("pass*****"), []byte(redisPassword), -1)

	//println("========= More yamlTemplates ===========")
	//println(string(yamlTemplates))
	//println()

	decoder := oshandler.NewYamlDecoder(yamlTemplates)
	decoder.
		Decode(&res.serviceSentinel).
		Decode(&res.rc).
		Decode(&res.rcSentinel)

	return decoder.Err
}

type srvResources struct {
	//pod      kapi.Pod
	rc kapi.ReplicationController
}

type redisResources_More struct {
	serviceSentinel kapi.Service
	rc              kapi.ReplicationController
	rcSentinel      kapi.ReplicationController
}

func createInstance(instanceId, serviceBrokerNamespace, srvPassword string) (*srvResources, error) {
	var input srvResources
	err := loadSrvResources(instanceId, srvPassword, &input)
	if err != nil {
		return nil, err
	}

	var output srvResources

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	// here, not use job.post
	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		//KPost(prefix + "/pods", &input.pod, &output.pod)
		KPost(prefix+"/replicationcontrollers", &input.rc, &output.rc)

	if osr.Err != nil {
		logger.Error("createRedisResources_Master", osr.Err)
	}

	return &output, osr.Err
}

func getSrvResources(instanceId, serviceBrokerNamespace, srvPassword string) (*srvResources, error) {
	var output srvResources

	var input srvResources
	err := loadSrvResources(instanceId, srvPassword, &input)
	if err != nil {
		return &output, err
	}

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		//KGet(prefix + "/pods/" + input.pod.Name, &output.pod)
		KGet(prefix+"/replicationcontrollers/"+input.rc.Name, &output.rc)

	if osr.Err != nil {
		logger.Error("getSrvResources", osr.Err)
	}

	return &output, osr.Err
}

func destroySrvResources(srvRes *srvResources, serviceBrokerNamespace string) {
	// todo: add to retry queue on fail

	//go func() {kdel (serviceBrokerNamespace, "pods", masterRes.pod.Name)}()
	go func() { kdel_rc(serviceBrokerNamespace, &srvRes.rc) }()
}

func (job *srvOrchestrationJob) createRedisResources_More(instanceId, serviceBrokerNamespace, redisPassword string) error {
	var input redisResources_More
	err := loadRedisResources_More(instanceId, redisPassword, &input)
	if err != nil {
		return err
	}

	var output redisResources_More

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

func getRedisResources_More(instanceId, serviceBrokerNamespace, redisPassword string) (*redisResources_More, error) {
	var output redisResources_More

	var input redisResources_More
	err := loadRedisResources_More(instanceId, redisPassword, &input)
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

func destroyRedisResources_More(moreRes *redisResources_More, serviceBrokerNamespace string) {
	// todo: add to retry queue on fail

	go func() { kdel(serviceBrokerNamespace, "services", moreRes.serviceSentinel.Name) }()
	go func() { kdel_rc(serviceBrokerNamespace, &moreRes.rc) }()
	go func() { kdel_rc(serviceBrokerNamespace, &moreRes.rcSentinel) }()
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
				logger.Error("watch HA redis rc error", status.Err)
				close(cancel)
				return
			} else {
				//logger.Debug("watch redis HA rc, status.Info: " + string(status.Info))
			}

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

		println("\n pods.Items[", i, "].Status.Phase =", pod.Status.Phase, "\n")

		if pod.Status.Phase == kapi.PodRunning {
			nrunnings++
		}
	}

	return nrunnings, nil
}
