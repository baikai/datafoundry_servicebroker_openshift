package kafka

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/pivotal-cf/brokerapi"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pivotal-golang/lager"

	kapi "k8s.io/kubernetes/pkg/api/v1"

	oshandler "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/handler"
	"github.com/asiainfoLDP/datafoundry_servicebroker_openshift/servicebroker/zookeeper"
)

//==============================================================
//
//==============================================================

const KafkaServcieBrokerName_Standalone = "Kafka_standalone"

func init() {
	oshandler.Register(KafkaServcieBrokerName_Standalone, &Kafka_freeHandler{})

	logger = lager.NewLogger(KafkaServcieBrokerName_Standalone)
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))
}

var logger lager.Logger

//==============================================================
//
//==============================================================

type Kafka_freeHandler struct{}

func (handler *Kafka_freeHandler) DoProvision(etcdSaveResult chan error, instanceID string, details brokerapi.ProvisionDetails, planInfo oshandler.PlanInfo, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, oshandler.ServiceInfo, error) {
	return newKafkaHandler().DoProvision(etcdSaveResult, instanceID, details, planInfo, asyncAllowed)
}

func (handler *Kafka_freeHandler) DoLastOperation(myServiceInfo *oshandler.ServiceInfo) (brokerapi.LastOperation, error) {
	return newKafkaHandler().DoLastOperation(myServiceInfo)
}

func (handler *Kafka_freeHandler) DoUpdate(myServiceInfo *oshandler.ServiceInfo, planInfo oshandler.PlanInfo, callbackSaveNewInfo func(*oshandler.ServiceInfo) error, asyncAllowed bool) error {
	return newKafkaHandler().DoUpdate(myServiceInfo, planInfo, callbackSaveNewInfo, asyncAllowed)
}

func (handler *Kafka_freeHandler) DoDeprovision(myServiceInfo *oshandler.ServiceInfo, asyncAllowed bool) (brokerapi.IsAsync, error) {
	return newKafkaHandler().DoDeprovision(myServiceInfo, asyncAllowed)
}

func (handler *Kafka_freeHandler) DoBind(myServiceInfo *oshandler.ServiceInfo, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, oshandler.Credentials, error) {
	return newKafkaHandler().DoBind(myServiceInfo, bindingID, details)
}

func (handler *Kafka_freeHandler) DoUnbind(myServiceInfo *oshandler.ServiceInfo, mycredentials *oshandler.Credentials) error {
	return newKafkaHandler().DoUnbind(myServiceInfo, mycredentials)
}

//==============================================================
//
//==============================================================

type Kafka_Handler struct {
}

func newKafkaHandler() *Kafka_Handler {
	return &Kafka_Handler{}
}

func (handler *Kafka_Handler) DoProvision(etcdSaveResult chan error, instanceID string, details brokerapi.ProvisionDetails, planInfo oshandler.PlanInfo, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, oshandler.ServiceInfo, error) {
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
	//kafkaUser := oshandler.NewElevenLengthID()
	//kafkaPassword := oshandler.GenGUID()
	zookeeperUser := "super" // oshandler.NewElevenLengthID()
	zookeeperPassword := oshandler.GenGUID()

	println()
	println("instanceIdInTempalte = ", instanceIdInTempalte)
	println("serviceBrokerNamespace = ", serviceBrokerNamespace)
	println()

	serviceInfo.Url = instanceIdInTempalte
	serviceInfo.Database = serviceBrokerNamespace // may be not needed
	//serviceInfo.User = kafkaUser
	//serviceInfo.Password = kafkaPassword
	serviceInfo.Admin_user = zookeeperUser
	serviceInfo.Admin_password = zookeeperPassword

	// master kafka
	//output, err := createKafkaResources_Master(instanceIdInTempalte, serviceBrokerNamespace, kafkaUser, kafkaPassword)
	//if err != nil {
	//	destroyKafkaResources_Master(output, serviceBrokerNamespace)
	//	return serviceSpec, serviceInfo, err
	//}
	// master zookeeper
	go func() {
		err := <-etcdSaveResult
		if err != nil {
			return
		}

		output, err := zookeeper.CreateZookeeperResources_Master(instanceIdInTempalte, serviceBrokerNamespace, zookeeperUser, zookeeperPassword)
		if err != nil {
			zookeeper.DestroyZookeeperResources_Master(output, serviceBrokerNamespace)

			return
		}

		startKafkaOrchestrationJob(&kafkaOrchestrationJob{
			cancelled:  false,
			cancelChan: make(chan struct{}),

			serviceInfo:        &serviceInfo,
			zookeeperResources: output,
		})

	}()

	serviceSpec.DashboardURL = ""

	//>>>
	serviceSpec.Credentials = getCredentialsOnPrivision(&serviceInfo)
	//<<<

	return serviceSpec, serviceInfo, nil
}

func (handler *Kafka_Handler) DoLastOperation(myServiceInfo *oshandler.ServiceInfo) (brokerapi.LastOperation, error) {
	// try to get state from running job
	job := getKafkaOrchestrationJob(myServiceInfo.Url)
	if job != nil {
		return brokerapi.LastOperation{
			State:       brokerapi.InProgress,
			Description: "In progress .",
		}, nil
	}

	// assume in provisioning

	// the job may be finished or interrupted or running in another instance.

	master_res, _ := getKafkaResources_Master(myServiceInfo.Url, myServiceInfo.Database) //, myServiceInfo.User, myServiceInfo.Password)

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

	if ok(&master_res.rc) {
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

func (handler *Kafka_Handler) DoUpdate(myServiceInfo *oshandler.ServiceInfo, planInfo oshandler.PlanInfo, callbackSaveNewInfo func(*oshandler.ServiceInfo) error, asyncAllowed bool) error {
	return nil
}

func (handler *Kafka_Handler) DoDeprovision(myServiceInfo *oshandler.ServiceInfo, asyncAllowed bool) (brokerapi.IsAsync, error) {
	go func() {
		job := getKafkaOrchestrationJob(myServiceInfo.Url)
		if job != nil {
			job.cancel()

			// wait job to exit
			for {
				time.Sleep(7 * time.Second)
				if nil == getKafkaOrchestrationJob(myServiceInfo.Url) {
					break
				}
			}
		}

		// ...

		println("to destroy zookeeper resources")

		zookeeper_res, _ := zookeeper.GetZookeeperResources_Master(myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.Admin_user, myServiceInfo.Admin_password)
		zookeeper.DestroyZookeeperResources_Master(zookeeper_res, myServiceInfo.Database)

		// ...

		println("to destroy kafka resources")

		master_res, _ := getKafkaResources_Master(myServiceInfo.Url, myServiceInfo.Database) //, myServiceInfo.User, myServiceInfo.Password)
		destroyKafkaResources_Master(master_res, myServiceInfo.Database)
	}()

	return brokerapi.IsAsync(false), nil
}

// please note: the bsi may be still not fully initialized when calling the function.
func getCredentialsOnPrivision(myServiceInfo *oshandler.ServiceInfo) oshandler.Credentials {
	var zookeeper_res zookeeper.ZookeeperResources_Master
	err := zookeeper.LoadZookeeperResources_Master(myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.Admin_user, myServiceInfo.Admin_password, &zookeeper_res)
	if err != nil {
		return oshandler.Credentials{}
	}

	zk_host, zk_port, err := zookeeper_res.ServiceHostPort(myServiceInfo.Database)
	if err != nil {
		return oshandler.Credentials{}
	}

	var master_res kafkaResources_Master
	err = loadKafkaResources_Master(myServiceInfo.Url, myServiceInfo.Database /*, kafkaUser, kafkaPassword*/, &master_res)
	if err != nil {
		return oshandler.Credentials{}
	}

	kafka_port := oshandler.GetServicePortByName(&master_res.service, "kafka-port")
	if kafka_port == nil {
		return oshandler.Credentials{}
	}

	host := fmt.Sprintf("%s.%s.%s", master_res.service.Name, myServiceInfo.Database, oshandler.ServiceDomainSuffix(false))
	port := strconv.Itoa(kafka_port.Port)
	//host := master_res.routeMQ.Spec.Host
	//port := "80"

	return oshandler.Credentials{
		Uri: fmt.Sprintf("kafka: %s:%s zookeeper: %s:%s (SuperUser: %s, Password: %s)",
			host, port, zk_host, zk_port, myServiceInfo.Admin_user, myServiceInfo.Admin_password),
		Hostname: host,
		Port:     port,
		//Username: myServiceInfo.User,
		//Password: myServiceInfo.Password,
		// todo: need return zookeeper password?
	}
}

func (handler *Kafka_Handler) DoBind(myServiceInfo *oshandler.ServiceInfo, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, oshandler.Credentials, error) {
	// todo: handle errors

	zookeeper_res, err := zookeeper.GetZookeeperResources_Master(myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.Admin_user, myServiceInfo.Admin_password)
	if err != nil {
		return brokerapi.Binding{}, oshandler.Credentials{}, err
	}

	zk_host, zk_port, err := zookeeper_res.ServiceHostPort(myServiceInfo.Database)
	if err != nil {
		return brokerapi.Binding{}, oshandler.Credentials{}, nil
	}

	master_res, err := getKafkaResources_Master(myServiceInfo.Url, myServiceInfo.Database) //, myServiceInfo.User, myServiceInfo.Password)
	if err != nil {
		return brokerapi.Binding{}, oshandler.Credentials{}, err
	}

	kafka_port := oshandler.GetServicePortByName(&master_res.service, "kafka-port")
	if kafka_port == nil {
		return brokerapi.Binding{}, oshandler.Credentials{}, errors.New("kafka-port port not found")
	}

	host := fmt.Sprintf("%s.%s.%s", master_res.service.Name, myServiceInfo.Database, oshandler.ServiceDomainSuffix(false))
	port := strconv.Itoa(kafka_port.Port)
	//host := master_res.routeMQ.Spec.Host
	//port := "80"

	mycredentials := oshandler.Credentials{
		Uri: fmt.Sprintf("kafka: %s:%s zookeeper: %s:%s (SuperUser: %s, Password: %s)",
			host, port, zk_host, zk_port, myServiceInfo.Admin_user, myServiceInfo.Admin_password),
		Hostname: host,
		Port:     port,
		//Username: myServiceInfo.User,
		//Password: myServiceInfo.Password,
		// todo: need return zookeeper password?
	}

	myBinding := brokerapi.Binding{Credentials: mycredentials}

	return myBinding, mycredentials, nil
}

func (handler *Kafka_Handler) DoUnbind(myServiceInfo *oshandler.ServiceInfo, mycredentials *oshandler.Credentials) error {
	// do nothing

	return nil
}

//===============================================================
//
//===============================================================

var kafkaOrchestrationJobs = map[string]*kafkaOrchestrationJob{}
var kafkaOrchestrationJobsMutex sync.Mutex

func getKafkaOrchestrationJob(instanceId string) *kafkaOrchestrationJob {
	kafkaOrchestrationJobsMutex.Lock()
	defer kafkaOrchestrationJobsMutex.Unlock()

	return kafkaOrchestrationJobs[instanceId]
}

func startKafkaOrchestrationJob(job *kafkaOrchestrationJob) {
	kafkaOrchestrationJobsMutex.Lock()
	defer kafkaOrchestrationJobsMutex.Unlock()

	if kafkaOrchestrationJobs[job.serviceInfo.Url] == nil {
		kafkaOrchestrationJobs[job.serviceInfo.Url] = job
		go func() {
			job.run()

			kafkaOrchestrationJobsMutex.Lock()
			delete(kafkaOrchestrationJobs, job.serviceInfo.Url)
			kafkaOrchestrationJobsMutex.Unlock()
		}()
	}
}

type kafkaOrchestrationJob struct {
	//instanceId string // use serviceInfo.

	cancelled   bool
	cancelChan  chan struct{}
	cancelMetex sync.Mutex

	serviceInfo *oshandler.ServiceInfo

	zookeeperResources *zookeeper.ZookeeperResources_Master
}

func (job *kafkaOrchestrationJob) cancel() {
	job.cancelMetex.Lock()
	defer job.cancelMetex.Unlock()

	if !job.cancelled {
		job.cancelled = true
		close(job.cancelChan)
	}
}

func (job *kafkaOrchestrationJob) run() {
	println("-- kafkaOrchestrationJob start --")

	result, cancel, err := zookeeper.WatchZookeeperOrchestration(job.serviceInfo.Url, job.serviceInfo.Database, job.serviceInfo.Admin_user, job.serviceInfo.Admin_password)
	if err != nil {
		zookeeper_res, _ := zookeeper.GetZookeeperResources_Master(job.serviceInfo.Url, job.serviceInfo.Database, job.serviceInfo.Admin_user, job.serviceInfo.Admin_password)
		zookeeper.DestroyZookeeperResources_Master(zookeeper_res, job.serviceInfo.Database)
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

	println("-- kafkaOrchestrationJob done, succeeded:", succeeded)

	if succeeded {
		println("  to create kafka resources")

		err = job.createKafkaResources_Master(job.serviceInfo.Url, job.serviceInfo.Database) //, job.serviceInfo.User, job.serviceInfo.Password)
		if err != nil {
			logger.Error("createKafkaResources_Master", err)
		} else {
			println("  succeeded to create kafka resources")
		}
	}
}

//=======================================================================
//
//=======================================================================

var KafkaTemplateData_Master []byte = nil

func loadKafkaResources_Master(instanceID, serviceBrokerNamespace /*, kafkaUser, kafkaPassword*/ string, res *kafkaResources_Master) error {
	if KafkaTemplateData_Master == nil {
		f, err := os.Open("kafka.yaml")
		if err != nil {
			return err
		}
		KafkaTemplateData_Master, err = ioutil.ReadAll(f)
		if err != nil {
			return err
		}
		kafka_image := oshandler.KafkaImage()
		kafka_image = strings.TrimSpace(kafka_image)
		if len(kafka_image) > 0 {
			KafkaTemplateData_Master = bytes.Replace(
				KafkaTemplateData_Master,
				[]byte("http://kafka-image-place-holder/kafka-openshift-orchestration"),
				[]byte(kafka_image),
				-1)
		}
	}

	// ...

	yamlTemplates := KafkaTemplateData_Master

	yamlTemplates = bytes.Replace(yamlTemplates, []byte("instanceid"), []byte(instanceID), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("local-service-postfix-place-holder"),
		[]byte(serviceBrokerNamespace+oshandler.ServiceDomainSuffix(true)), -1)

	//println("========= Boot yamlTemplates ===========")
	//println(string(yamlTemplates))
	//println()

	decoder := oshandler.NewYamlDecoder(yamlTemplates)
	decoder.
		Decode(&res.service).
		Decode(&res.rc)

	return decoder.Err
}

type kafkaResources_Master struct {
	service kapi.Service
	rc      kapi.ReplicationController
}

func (job *kafkaOrchestrationJob) createKafkaResources_Master(instanceId, serviceBrokerNamespace /*, kafkaUser, kafkaPassword*/ string) error {
	var input kafkaResources_Master
	err := loadKafkaResources_Master(instanceId, serviceBrokerNamespace /*, kafkaUser, kafkaPassword*/, &input)
	if err != nil {
		//return nil, err
		return err
	}

	var output kafkaResources_Master
	/*
		osr := oshandler.NewOpenshiftREST(oshandler.OC())

		// here, not use job.post
		prefix := "/namespaces/" + serviceBrokerNamespace
		osr.
			KPost(prefix + "/services", &input.service, &output.service).
			KPost(prefix + "/replicationcontrollers", &input.rc, &output.rc)

		if osr.Err != nil {
			logger.Error("createKafkaResources_Master", osr.Err)
		}

		return &output, osr.Err
	*/
	go func() {
		if err := job.kpost(serviceBrokerNamespace, "services", &input.service, &output.service); err != nil {
			logger.Error("createKafkaResources_Master.create service", err)
			return
		}
		if err := job.kpost(serviceBrokerNamespace, "replicationcontrollers", &input.rc, &output.rc); err != nil {
			logger.Error("createKafkaResources_Master.create rc", err)
			return
		}
	}()

	return nil
}

func getKafkaResources_Master(instanceId, serviceBrokerNamespace /*, kafkaUser, kafkaPassword*/ string) (*kafkaResources_Master, error) {
	var output kafkaResources_Master

	var input kafkaResources_Master
	err := loadKafkaResources_Master(instanceId, serviceBrokerNamespace /*, kafkaUser, kafkaPassword*/, &input)
	if err != nil {
		return &output, err
	}

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		KGet(prefix+"/services/"+input.service.Name, &output.service).
		KGet(prefix+"/replicationcontrollers/"+input.rc.Name, &output.rc)

	if osr.Err != nil {
		logger.Error("getKafkaResources_Master", osr.Err)
	}

	return &output, osr.Err
}

func destroyKafkaResources_Master(masterRes *kafkaResources_Master, serviceBrokerNamespace string) {
	// todo: add to retry queue on fail

	go func() { kdel(serviceBrokerNamespace, "services", masterRes.service.Name) }()
	go func() { kdel_rc(serviceBrokerNamespace, &masterRes.rc) }()
}

//===============================================================
//
//===============================================================

func (job *kafkaOrchestrationJob) kpost(serviceBrokerNamespace, typeName string, body interface{}, into interface{}) error {
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

func (job *kafkaOrchestrationJob) opost(serviceBrokerNamespace, typeName string, body interface{}, into interface{}) error {
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
				logger.Error("watch HA kafka rc error", status.Err)
				close(cancel)
				return
			} else {
				//logger.Debug("watch kafka HA rc, status.Info: " + string(status.Info))
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
