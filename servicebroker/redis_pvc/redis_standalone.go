package redis_pvc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	oshandler "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/handler"
	"github.com/pivotal-cf/brokerapi"
	//"github.com/pivotal-golang/lager"
	logger "github.com/golang/glog"
	"io/ioutil"
	kapi "k8s.io/kubernetes/pkg/api/v1"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

//==============================================================
//初始化Log
//==============================================================

const RedisServcieBrokerName_Standalone = "Redis_volumes_standalone"

func init() {
	oshandler.Register(RedisServcieBrokerName_Standalone, &Redis_freeHandler{})

	//logger = lager.NewLogger(RedisServcieBrokerName_Standalone)
	//logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))
}

//var logger lager.Logger

//==============================================================
//
//==============================================================

type Redis_freeHandler struct{}

func (handler *Redis_freeHandler) DoProvision(etcdSaveResult chan error, instanceID string, details brokerapi.ProvisionDetails, planInfo oshandler.PlanInfo, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, oshandler.ServiceInfo, error) {
	return newRedisHandler().DoProvision(etcdSaveResult, instanceID, details, planInfo, asyncAllowed)
}

func (handler *Redis_freeHandler) DoLastOperation(myServiceInfo *oshandler.ServiceInfo) (brokerapi.LastOperation, error) {
	return newRedisHandler().DoLastOperation(myServiceInfo)
}

func (handler *Redis_freeHandler) DoUpdate(myServiceInfo *oshandler.ServiceInfo, planInfo oshandler.PlanInfo, callbackSaveNewInfo func(*oshandler.ServiceInfo) error, asyncAllowed bool) error {
	return newRedisHandler().DoUpdate(myServiceInfo, planInfo, callbackSaveNewInfo, asyncAllowed)
}

func (handler *Redis_freeHandler) DoDeprovision(myServiceInfo *oshandler.ServiceInfo, asyncAllowed bool) (brokerapi.IsAsync, error) {
	return newRedisHandler().DoDeprovision(myServiceInfo, asyncAllowed)
}

func (handler *Redis_freeHandler) DoBind(myServiceInfo *oshandler.ServiceInfo, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, oshandler.Credentials, error) {
	return newRedisHandler().DoBind(myServiceInfo, bindingID, details)
}

func (handler *Redis_freeHandler) DoUnbind(myServiceInfo *oshandler.ServiceInfo, mycredentials *oshandler.Credentials) error {
	return newRedisHandler().DoUnbind(myServiceInfo, mycredentials)
}

//==============================================================
//
//==============================================================

// version 1:
//   one master volume, two slave volumes,

func volumeBaseName(instanceId string) string {
	return "rds-" + instanceId
}

func masterPvcName(volumes []oshandler.Volume) string {
	if len(volumes) > 0 {
		return volumes[0].Volume_name
	}
	return ""
}

func slavePvcName1(volumes []oshandler.Volume) string {
	if len(volumes) > 1 {
		return volumes[1].Volume_name
	}
	return ""
}

func slavePvcName2(volumes []oshandler.Volume) string {
	if len(volumes) > 2 {
		return volumes[2].Volume_name
	}
	return ""
}

//==============================================================
//
//==============================================================

type Redis_Handler struct {
}

func newRedisHandler() *Redis_Handler {
	return &Redis_Handler{}
}

func (handler *Redis_Handler) DoProvision(etcdSaveResult chan error, instanceID string, details brokerapi.ProvisionDetails, planInfo oshandler.PlanInfo, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, oshandler.ServiceInfo, error) {
	//初始化到openshift的链接

	serviceSpec := brokerapi.ProvisionedServiceSpec{IsAsync: asyncAllowed}
	serviceInfo := oshandler.ServiceInfo{}

	serviceSpec.IsAsync = true

	instanceIdInTempalte := strings.ToLower(oshandler.NewThirteenLengthID())
	serviceBrokerNamespace := oshandler.OC().Namespace()
	redisPassword := oshandler.GenGUID()

	volumeBaseName := volumeBaseName(instanceIdInTempalte)
	volumes := []oshandler.Volume{
		// one master volume
		{
			Volume_size: planInfo.Volume_size,
			Volume_name: volumeBaseName + "-0",
		},
		// two slave volumes
		{
			Volume_size: planInfo.Volume_size,
			Volume_name: volumeBaseName + "-1",
		},
		{
			Volume_size: planInfo.Volume_size,
			Volume_name: volumeBaseName + "-2",
		},
	}

	logger.Info("Redis Creating ...", map[string]interface{}{"instanceIdInTempalte": instanceIdInTempalte, "serviceBrokerNamespace": serviceBrokerNamespace})

	// ...

	serviceInfo.Url = instanceIdInTempalte
	serviceInfo.Database = serviceBrokerNamespace // may be not needed
	serviceInfo.Password = redisPassword

	serviceInfo.Volumes = volumes

	asyncResultChan := serviceInfo.MakeAsyncResult()

	// ...
	go func() {
		err := <-etcdSaveResult
		if err != nil {
			return
		}

		// create volumes

		result := oshandler.StartCreatePvcVolumnJob(
			volumeBaseName,
			serviceInfo.Database,
			serviceInfo.Volumes,
		)

		err = <-result
		if err != nil {
			logger.Error("redis create volume", err)
			handler.DoDeprovision(&serviceInfo, true)

			asyncResultChan <- err
			return
		}

		logger.Infoln("createRedisResources_Master ...")

		// create master res

		output, err := createRedisResources_Master(
			serviceInfo.Url,
			serviceInfo.Database,
			serviceInfo.Password,
			serviceInfo.Volumes,
		)
		if err != nil {

			destroyRedisResources_Master(output, serviceInfo.Database)
			oshandler.DeleteVolumns(serviceInfo.Database, volumes)

			asyncResultChan <- err
			return
		}

		// create other resources ...

		startRedisOrchestrationJob(&redisOrchestrationJob{
			cancelled:  false,
			cancelChan: make(chan struct{}),

			serviceInfo:     &serviceInfo,
			masterResources: output,
		})

		asyncResultChan <- nil
	}()

	// ...

	serviceSpec.DashboardURL = ""

	//>>>
	serviceSpec.Credentials = getCredentialsOnPrivision(&serviceInfo)
	//<<<

	return serviceSpec, serviceInfo, nil
}

func (handler *Redis_Handler) DoLastOperation(myServiceInfo *oshandler.ServiceInfo) (brokerapi.LastOperation, error) {

	if myServiceInfo.ProvisionFailureInfo != "" {
		return brokerapi.LastOperation{
			State:       brokerapi.Failed,
			Description: myServiceInfo.ProvisionFailureInfo,
		}, nil
	}

	volumeJob := oshandler.GetCreatePvcVolumnJob(volumeBaseName(myServiceInfo.Url))
	if volumeJob != nil {
		return brokerapi.LastOperation{
			State:       brokerapi.InProgress,
			Description: "in progress.",
		}, nil
	}

	_, err := getRedisResources_Master(
		myServiceInfo.Url,
		myServiceInfo.Database,
		myServiceInfo.Password,
		myServiceInfo.Volumes,
	)

	if err != nil {
		return brokerapi.LastOperation{
			State:       brokerapi.Failed,
			Description: "In progress .",
		}, err
	}

	// try to get state from running job
	orchJob := getRedisOrchestrationJob(myServiceInfo.Url)
	if orchJob != nil {
		return brokerapi.LastOperation{
			State:       brokerapi.InProgress,
			Description: "In progress .",
		}, nil
	}

	// assume in provisioning

	// the job may be finished or interrupted or running in another instance.

	more_res, _ := getRedisResources_More(
		myServiceInfo.Url,
		myServiceInfo.Database,
		myServiceInfo.Password,
		myServiceInfo.Volumes,
	)

	ok := func(rc *kapi.ReplicationController) bool {
		logger.Infoln("rc.Name =", rc.Name)
		if rc == nil || rc.Name == "" || rc.Spec.Replicas == nil || rc.Status.Replicas < *rc.Spec.Replicas {
			return false
		}
		n, _ := statRunningPodsByLabels(myServiceInfo.Database, rc.Labels)
		logger.Infoln("n =", n)
		return n >= *rc.Spec.Replicas
	}

	if ok(&more_res.rc1) && ok(&more_res.rc2) && ok(&more_res.rcSentinel) {
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

func (handler *Redis_Handler) DoUpdate(myServiceInfo *oshandler.ServiceInfo, planInfo oshandler.PlanInfo, callbackSaveNewInfo func(*oshandler.ServiceInfo) error, asyncAllowed bool) error {
	return errors.New("not implemented")
}

func (handler *Redis_Handler) DoDeprovision(myServiceInfo *oshandler.ServiceInfo, asyncAllowed bool) (brokerapi.IsAsync, error) {
	go func() {
		// ...
		volumeJob := oshandler.GetCreatePvcVolumnJob(volumeBaseName(myServiceInfo.Url))
		if volumeJob != nil {
			volumeJob.Cancel()

			// wait job to exit
			for {
				time.Sleep(7 * time.Second)
				if nil == oshandler.GetCreatePvcVolumnJob(volumeBaseName(myServiceInfo.Url)) {
					break
				}
			}
		}

		// ...

		job := getRedisOrchestrationJob(myServiceInfo.Url)
		if job != nil {
			job.cancel()

			// wait job to exit
			for {
				time.Sleep(7 * time.Second)
				if nil == getRedisOrchestrationJob(myServiceInfo.Url) {
					break
				}
			}
		}

		// ...

		logger.Infoln("to destroy resources:", myServiceInfo.Url)

		more_res, _ := getRedisResources_More(
			myServiceInfo.Url,
			myServiceInfo.Database,
			myServiceInfo.Password,
			myServiceInfo.Volumes,
		)
		destroyRedisResources_More(more_res, myServiceInfo.Database)

		master_res, _ := getRedisResources_Master(
			myServiceInfo.Url,
			myServiceInfo.Database,
			myServiceInfo.Password,
			myServiceInfo.Volumes,
		)
		destroyRedisResources_Master(master_res, myServiceInfo.Database)

		// ...

		logger.Infoln("to destroy volumes:", myServiceInfo.Volumes)

		oshandler.DeleteVolumns(myServiceInfo.Database, myServiceInfo.Volumes)
	}()

	return brokerapi.IsAsync(false), nil
}

// please note: the bsi may be still not fully initialized when calling the function.
func getCredentialsOnPrivision(myServiceInfo *oshandler.ServiceInfo) oshandler.Credentials {
	var more_res redisResources_More
	err := loadRedisResources_More(myServiceInfo.Url, myServiceInfo.Password, myServiceInfo.Volumes, &more_res)
	if err != nil {
		logger.Error("loadRedisResources_More error", err)
		return oshandler.Credentials{}
	}

	client_port := &more_res.serviceSentinel.Spec.Ports[0]

	cluser_name := "cluster-" + more_res.serviceSentinel.Name
	host := fmt.Sprintf("%s.%s.%s", more_res.serviceSentinel.Name, myServiceInfo.Database, oshandler.ServiceDomainSuffix(false))
	port := strconv.Itoa(client_port.Port)

	return oshandler.Credentials{
		Uri:      "",
		Hostname: host,
		Port:     port,
		//Username: myServiceInfo.User,
		Password: myServiceInfo.Password,
		Name:     cluser_name,
	}
}

func (handler *Redis_Handler) DoBind(myServiceInfo *oshandler.ServiceInfo, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, oshandler.Credentials, error) {
	// todo: handle errors

	// master_res may has been shutdown normally.

	more_res, err := getRedisResources_More(
		myServiceInfo.Url,
		myServiceInfo.Database,
		myServiceInfo.Password,
		myServiceInfo.Volumes,
	)
	if err != nil {
		return brokerapi.Binding{}, oshandler.Credentials{}, err
	}

	client_port := &more_res.serviceSentinel.Spec.Ports[0]

	cluser_name := "cluster-" + more_res.serviceSentinel.Name
	host := fmt.Sprintf("%s.%s.%s", more_res.serviceSentinel.Name, myServiceInfo.Database, oshandler.ServiceDomainSuffix(false))
	port := strconv.Itoa(client_port.Port)

	mycredentials := oshandler.Credentials{
		Uri:      "",
		Hostname: host,
		Port:     port,
		Password: myServiceInfo.Password,
		Name:     cluser_name,
	}

	myBinding := brokerapi.Binding{Credentials: mycredentials}

	return myBinding, mycredentials, nil
}

func (handler *Redis_Handler) DoUnbind(myServiceInfo *oshandler.ServiceInfo, mycredentials *oshandler.Credentials) error {
	// do nothing

	return nil
}

//=======================================================================
//
//=======================================================================

// todo: it is best to save jobs in mysql firstly, ...
// now, when the server instance is terminated, jobs are lost.

//==============

var redisOrchestrationJobs = map[string]*redisOrchestrationJob{}
var redisOrchestrationJobsMutex sync.Mutex

func getRedisOrchestrationJob(instanceId string) *redisOrchestrationJob {
	redisOrchestrationJobsMutex.Lock()
	job := redisOrchestrationJobs[instanceId]
	redisOrchestrationJobsMutex.Unlock()

	return job
}

func startRedisOrchestrationJob(job *redisOrchestrationJob) {
	redisOrchestrationJobsMutex.Lock()
	defer redisOrchestrationJobsMutex.Unlock()

	if redisOrchestrationJobs[job.serviceInfo.Url] == nil {
		redisOrchestrationJobs[job.serviceInfo.Url] = job
		go func() {
			job.run()

			redisOrchestrationJobsMutex.Lock()
			delete(redisOrchestrationJobs, job.serviceInfo.Url)
			redisOrchestrationJobsMutex.Unlock()
		}()
	}
}

type redisOrchestrationJob struct {
	//instanceId string // use serviceInfo.

	cancelled   bool
	cancelChan  chan struct{}
	cancelMetex sync.Mutex

	serviceInfo *oshandler.ServiceInfo

	masterResources *redisResources_Master
}

func (job *redisOrchestrationJob) cancel() {
	job.cancelMetex.Lock()
	defer job.cancelMetex.Unlock()

	if !job.cancelled {
		job.cancelled = true
		close(job.cancelChan)
	}
}

func (job *redisOrchestrationJob) run() {

	logger.Infoln("startRedisOrchestrationJob ...")

	serviceInfo := job.serviceInfo

	rc := &job.masterResources.rc

	for {
		if job.cancelled {
			return
		}

		n, _ := statRunningPodsByLabels(serviceInfo.Database, rc.Labels)

		logger.Infoln("n = ", n, ", *job.masterResources.rc.Spec.Replicas = ", *rc.Spec.Replicas)

		if n < *rc.Spec.Replicas {
			time.Sleep(10 * time.Second)
		} else {
			break
		}
	}

	logger.Infoln("redis master pod is running now")

	time.Sleep(5 * time.Second)

	if job.cancelled {
		return
	}

	// create more resources

	logger.Infoln("createRedisResources_More ...")

	job.createRedisResources_More(
		serviceInfo.Url,
		serviceInfo.Database,
		serviceInfo.Password,
		serviceInfo.Volumes,
	)
}

//=======================================================================
//
//=======================================================================

var RedisTemplateData_Master []byte = nil

func loadRedisResources_Master(instanceID, redisPassword string, volumes []oshandler.Volume, res *redisResources_Master) error {
	if RedisTemplateData_Master == nil {

		f, err := os.Open("redis-pvc-master.yaml")
		if err != nil {
			return err
		}
		RedisTemplateData_Master, err = ioutil.ReadAll(f)
		if err != nil {
			return err
		}
		redis_image := oshandler.RedisImage()
		redis_image = strings.TrimSpace(redis_image)
		if len(redis_image) > 0 {
			RedisTemplateData_Master = bytes.Replace(
				RedisTemplateData_Master,
				[]byte("http://redis-image-place-holder/redis-openshift-orchestration"),
				[]byte(redis_image),
				-1)
		}
	}

	// ...

	masterPvcName := masterPvcName(volumes)

	yamlTemplates := RedisTemplateData_Master

	yamlTemplates = bytes.Replace(yamlTemplates, []byte("instanceid"), []byte(instanceID), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("pass*****"), []byte(redisPassword), -1)

	yamlTemplates = bytes.Replace(yamlTemplates, []byte("pvcname*****master"), []byte(masterPvcName), -1)

	decoder := oshandler.NewYamlDecoder(yamlTemplates)
	decoder.
		Decode(&res.rc)

	return decoder.Err
}

var RedisTemplateData_More []byte = nil

func loadRedisResources_More(instanceID, redisPassword string, volumes []oshandler.Volume, res *redisResources_More) error {

	if RedisTemplateData_More == nil {

		f, err := os.Open("redis-pvc-more.yaml")
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

	slavePvcName1 := slavePvcName1(volumes)
	slavePvcName2 := slavePvcName2(volumes)

	yamlTemplates := RedisTemplateData_More

	yamlTemplates = bytes.Replace(yamlTemplates, []byte("instanceid"), []byte(instanceID), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("pass*****"), []byte(redisPassword), -1)

	yamlTemplates = bytes.Replace(yamlTemplates, []byte("pvcname*****slave1"), []byte(slavePvcName1), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("pvcname*****slave2"), []byte(slavePvcName2), -1)

	decoder := oshandler.NewYamlDecoder(yamlTemplates)
	decoder.
		Decode(&res.serviceSentinel).
		Decode(&res.rc1).
		Decode(&res.rc2).
		Decode(&res.rcSentinel)

	return decoder.Err
}

type redisResources_Master struct {
	rc kapi.ReplicationController
}

type redisResources_More struct {
	serviceSentinel kapi.Service
	rc1             kapi.ReplicationController
	rc2             kapi.ReplicationController
	rcSentinel      kapi.ReplicationController
}

func createRedisResources_Master(instanceId, serviceBrokerNamespace, redisPassword string, volumes []oshandler.Volume) (*redisResources_Master, error) {
	var input redisResources_Master
	err := loadRedisResources_Master(instanceId, redisPassword, volumes, &input)
	if err != nil {
		logger.Error("loadRedisResources_Master error", err)
		return nil, err
	}

	var output redisResources_Master

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	// here, not use job.post
	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		KPost(prefix+"/replicationcontrollers", &input.rc, &output.rc)

	if osr.Err != nil {
		logger.Error("createRedisResources_Master", osr.Err)
	}

	return &output, osr.Err
}

func getRedisResources_Master(instanceId, serviceBrokerNamespace, redisPassword string, volumes []oshandler.Volume) (*redisResources_Master, error) {
	var output redisResources_Master

	var input redisResources_Master
	err := loadRedisResources_Master(instanceId, redisPassword, volumes, &input)
	if err != nil {
		logger.Error("loadRedisResources_Master error", err)
		return &output, err
	}

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		KGet(prefix+"/replicationcontrollers/"+input.rc.Name, &output.rc)

	if osr.Err != nil {
		logger.Error("getRedisResources_Master", osr.Err)
	}

	return &output, osr.Err
}

func destroyRedisResources_Master(masterRes *redisResources_Master, serviceBrokerNamespace string) {
	// todo: add to retry queue on fail

	////go func() {kdel (serviceBrokerNamespace, "pods", masterRes.pod.Name)}()
	//go func() { kdel_rc(serviceBrokerNamespace, &masterRes.rc) }()

	// Changed to synced to avoid being deleted behind volumes.
	//kdel (serviceBrokerNamespace, "pods", masterRes.pod.Name)
	kdel_rc(serviceBrokerNamespace, &masterRes.rc)
}

func (job *redisOrchestrationJob) createRedisResources_More(instanceId, serviceBrokerNamespace, redisPassword string, volumes []oshandler.Volume) error {
	var input redisResources_More
	err := loadRedisResources_More(instanceId, redisPassword, volumes, &input)
	if err != nil {
		return err
	}

	var output redisResources_More

	go func() {
		if err := job.kpost(serviceBrokerNamespace, "services", &input.serviceSentinel, &output.serviceSentinel); err != nil {
			return
		}
		if err := job.kpost(serviceBrokerNamespace, "replicationcontrollers", &input.rc1, &output.rc1); err != nil {
			return
		}
		if err := job.kpost(serviceBrokerNamespace, "replicationcontrollers", &input.rc2, &output.rc2); err != nil {
			return
		}
		if err := job.kpost(serviceBrokerNamespace, "replicationcontrollers", &input.rcSentinel, &output.rcSentinel); err != nil {
			return
		}
	}()

	return nil
}

func getRedisResources_More(instanceId, serviceBrokerNamespace, redisPassword string, volumes []oshandler.Volume) (*redisResources_More, error) {
	var output redisResources_More

	var input redisResources_More
	err := loadRedisResources_More(instanceId, redisPassword, volumes, &input)
	if err != nil {
		logger.Error("loadRedisResources_More error", err)
		return &output, err
	}

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		KGet(prefix+"/services/"+input.serviceSentinel.Name, &output.serviceSentinel).
		KGet(prefix+"/replicationcontrollers/"+input.rc1.Name, &output.rc1).
		KGet(prefix+"/replicationcontrollers/"+input.rc2.Name, &output.rc2).
		KGet(prefix+"/replicationcontrollers/"+input.rcSentinel.Name, &output.rcSentinel)

	if osr.Err != nil {
		logger.Error("getRedisResources_More", osr.Err)
	}

	return &output, osr.Err
}

func destroyRedisResources_More(moreRes *redisResources_More, serviceBrokerNamespace string) {
	// todo: add to retry queue on fail

	//go func() { kdel(serviceBrokerNamespace, "services", moreRes.serviceSentinel.Name) }()
	//go func() { kdel_rc(serviceBrokerNamespace, &moreRes.rc1) }()
	//go func() { kdel_rc(serviceBrokerNamespace, &moreRes.rc2) }()
	//go func() { kdel_rc(serviceBrokerNamespace, &moreRes.rcSentinel) }()

	//go func() { kdel(serviceBrokerNamespace, "services", moreRes.serviceSentinel.Name) }()
	//go func() { kdel_rc(serviceBrokerNamespace, &moreRes.rc1) }()
	//go func() { kdel_rc(serviceBrokerNamespace, &moreRes.rc2) }()
	//go func() { kdel_rc(serviceBrokerNamespace, &moreRes.rcSentinel) }()

	// Changed to synced to avoid being deleted behind volumes.

	kdel(serviceBrokerNamespace, "services", moreRes.serviceSentinel.Name)
	kdel_rc(serviceBrokerNamespace, &moreRes.rc1)
	kdel_rc(serviceBrokerNamespace, &moreRes.rc2)
	kdel_rc(serviceBrokerNamespace, &moreRes.rcSentinel)

	kdel(serviceBrokerNamespace, "services", moreRes.serviceSentinel.Name)
	kdel_rc(serviceBrokerNamespace, &moreRes.rc1)
	kdel_rc(serviceBrokerNamespace, &moreRes.rc2)
	kdel_rc(serviceBrokerNamespace, &moreRes.rcSentinel)
}

//===============================================================
//
//===============================================================

func (job *redisOrchestrationJob) kpost(serviceBrokerNamespace, typeName string, body interface{}, into interface{}) error {
	logger.Infoln("to create ", typeName)

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

func (job *redisOrchestrationJob) opost(serviceBrokerNamespace, typeName string, body interface{}, into interface{}) error {
	logger.Infoln("to create ", typeName)

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

	logger.Infoln("to delete ", typeName, "/", resName)

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

func kdel_rc(serviceBrokerNamespace string, rc *kapi.ReplicationController) {
	// looks pods will be auto deleted when rc is deleted.

	if rc == nil || rc.Name == "" {
		return
	}

	logger.Infoln("to delete pods on replicationcontroller", rc.Name)

	uri := "/namespaces/" + serviceBrokerNamespace + "/replicationcontrollers/" + rc.Name

	// modfiy rc replicas to 0

	zero := 0
	rc.Spec.Replicas = &zero
	osr := oshandler.NewOpenshiftREST(oshandler.OC()).KPut(uri, rc, nil)
	if osr.Err != nil {
		logger.Error("Modify Redis rc", osr.Err)
		return
	}

	// start watching rc status

	statuses, cancel, err := oshandler.OC().KWatch(uri)
	if err != nil {
		logger.Error("Start Watching Redis rc", err)
		return
	}

	go func() {
		for {
			status, _ := <-statuses

			if status.Err != nil {
				logger.Error("Watch Redis rc error", status.Err)
				close(cancel)
				return
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

func statRunningPodsByLabels(serviceBrokerNamespace string, labels map[string]string) (int, error) {

	logger.Infoln("to list pods in", serviceBrokerNamespace)

	uri := "/namespaces/" + serviceBrokerNamespace + "/pods"

	pods := kapi.PodList{}

	osr := oshandler.NewOpenshiftREST(oshandler.OC()).KList(uri, labels, &pods)
	if osr.Err != nil {
		return 0, osr.Err
	}

	nrunnings := 0

	for i := range pods.Items {
		pod := &pods.Items[i]

		logger.Infoln("\n pods.Items[", i, "].Status.Phase =", pod.Status.Phase, "\n")

		if pod.Status.Phase == kapi.PodRunning {
			nrunnings++
		}
	}

	return nrunnings, nil
}
