package spark

import (
	"fmt"
	//"errors"
	//marathon "github.com/gambol99/go-marathon"
	//kapi "golang.org/x/build/kubernetes/api"
	//"golang.org/x/build/kubernetes"
	//"golang.org/x/oauth2"
	//"net/http"
	"bytes"
	"encoding/json"
	"github.com/pivotal-cf/brokerapi"
	"net"
	"strconv"
	"strings"
	"time"
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
)

//==============================================================
//
//==============================================================

const SparkServcieBrokerName_Free = "Spark_One_Worker"
const SparkServcieBrokerName_HighAvailable = "Spark_Three_Workers"

func init() {
	oshandler.Register(SparkServcieBrokerName_Free, &Spark_freeHandler{})
	oshandler.Register(SparkServcieBrokerName_HighAvailable, &Spark_haHandler{})

	logger = lager.NewLogger("spark_openshift")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))
}

var logger lager.Logger

//==============================================================
//
//==============================================================

type Spark_freeHandler struct{}

func (handler *Spark_freeHandler) DoProvision(etcdSaveResult chan error, instanceID string, details brokerapi.ProvisionDetails, planInfo oshandler.PlanInfo, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, oshandler.ServiceInfo, error) {
	return newSparkHandler(1).DoProvision(etcdSaveResult, instanceID, details, planInfo, asyncAllowed)
}

func (handler *Spark_freeHandler) DoLastOperation(myServiceInfo *oshandler.ServiceInfo) (brokerapi.LastOperation, error) {
	return newSparkHandler(1).DoLastOperation(myServiceInfo)
}

func (handler *Spark_freeHandler) DoUpdate(myServiceInfo *oshandler.ServiceInfo, planInfo oshandler.PlanInfo, callbackSaveNewInfo func(*oshandler.ServiceInfo) error, asyncAllowed bool) error {
	return newSparkHandler(1).DoUpdate(myServiceInfo, planInfo, callbackSaveNewInfo, asyncAllowed)
}

func (handler *Spark_freeHandler) DoDeprovision(myServiceInfo *oshandler.ServiceInfo, asyncAllowed bool) (brokerapi.IsAsync, error) {
	return newSparkHandler(1).DoDeprovision(myServiceInfo, asyncAllowed)
}

func (handler *Spark_freeHandler) DoBind(myServiceInfo *oshandler.ServiceInfo, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, oshandler.Credentials, error) {
	return newSparkHandler(1).DoBind(myServiceInfo, bindingID, details)
}

func (handler *Spark_freeHandler) DoUnbind(myServiceInfo *oshandler.ServiceInfo, mycredentials *oshandler.Credentials) error {
	return newSparkHandler(1).DoUnbind(myServiceInfo, mycredentials)
}

//...

type Spark_haHandler struct{}

func (handler *Spark_haHandler) DoProvision(etcdSaveResult chan error, instanceID string, details brokerapi.ProvisionDetails, planInfo oshandler.PlanInfo, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, oshandler.ServiceInfo, error) {
	return newSparkHandler(3).DoProvision(etcdSaveResult, instanceID, details, planInfo, asyncAllowed)
}

func (handler *Spark_haHandler) DoLastOperation(myServiceInfo *oshandler.ServiceInfo) (brokerapi.LastOperation, error) {
	return newSparkHandler(3).DoLastOperation(myServiceInfo)
}

func (handler *Spark_haHandler) DoUpdate(myServiceInfo *oshandler.ServiceInfo, planInfo oshandler.PlanInfo, callbackSaveNewInfo func(*oshandler.ServiceInfo) error, asyncAllowed bool) error {
	return newSparkHandler(3).DoUpdate(myServiceInfo, planInfo, callbackSaveNewInfo, asyncAllowed)
}

func (handler *Spark_haHandler) DoDeprovision(myServiceInfo *oshandler.ServiceInfo, asyncAllowed bool) (brokerapi.IsAsync, error) {
	return newSparkHandler(3).DoDeprovision(myServiceInfo, asyncAllowed)
}

func (handler *Spark_haHandler) DoBind(myServiceInfo *oshandler.ServiceInfo, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, oshandler.Credentials, error) {
	return newSparkHandler(3).DoBind(myServiceInfo, bindingID, details)
}

func (handler *Spark_haHandler) DoUnbind(myServiceInfo *oshandler.ServiceInfo, mycredentials *oshandler.Credentials) error {
	return newSparkHandler(3).DoUnbind(myServiceInfo, mycredentials)
}

//==============================================================
//
//==============================================================

type Spark_Handler struct {
	numWorkers int
}

func newSparkHandler(numWorkers int) *Spark_Handler {
	return &Spark_Handler{
		numWorkers: numWorkers,
	}
}

func (handler *Spark_Handler) DoProvision(etcdSaveResult chan error, instanceID string, details brokerapi.ProvisionDetails, planInfo oshandler.PlanInfo, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, oshandler.ServiceInfo, error) {
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
	sparkSecret := oshandler.GenGUID()

	serviceInfo.Url = instanceIdInTempalte
	serviceInfo.Database = serviceBrokerNamespace // may be not needed
	//serviceInfo.User = oshandler.NewElevenLengthID()
	serviceInfo.Password = sparkSecret

	println()
	println("instanceIdInTempalte = ", instanceIdInTempalte)
	println("serviceBrokerNamespace = ", serviceBrokerNamespace)
	println()

	go func() {
		err := <-etcdSaveResult
		if err != nil {
			return
		}

		// master spark
		output, err := createSparkResources_Master(instanceIdInTempalte, serviceBrokerNamespace, sparkSecret)
		if err != nil {
			destroySparkResources_Master(output, serviceBrokerNamespace)

			return
		}

		// todo: improve watch. Pod may be already running before watching!
		startSparkOrchestrationJob(&sparkOrchestrationJob{
			cancelled:  false,
			cancelChan: make(chan struct{}),

			isProvisioning:  true,
			serviceInfo:     &serviceInfo,
			planNumWorkers:  handler.numWorkers,
			masterResources: output,
			//slavesResources:   nil,
			//zeppelinResources: nil,
		})

	}()

	var input sparkResources_Master
	err := loadSparkResources_Master(instanceIdInTempalte, serviceBrokerNamespace, sparkSecret, &input)
	if err != nil {
		return serviceSpec, serviceInfo, err
	}

	master_web_host := input.webroute.Spec.Host
	master_web_port := "80" // strconv.Itoa(master_res.mastersvc.Spec.Ports[0].Port)
	master_web_uri := "http://" + net.JoinHostPort(master_web_host, master_web_port)

	serviceSpec.DashboardURL = master_web_uri

	//>>>
	serviceSpec.Credentials = getCredentialsOnPrivision(&serviceInfo)
	//<<<

	return serviceSpec, serviceInfo, nil
}

func (handler *Spark_Handler) DoLastOperation(myServiceInfo *oshandler.ServiceInfo) (brokerapi.LastOperation, error) {
	// try to get state from running job
	job := getSparkOrchestrationJob(myServiceInfo.Url)
	if job != nil {
		return brokerapi.LastOperation{
			State:       brokerapi.InProgress,
			Description: "In progress .",
		}, nil
	}

	// assume in provisioning

	// the job may be finished or interrupted or running in another instance.

	// check master route, if it doesn't exist, return failed
	master_res, _ := getSparkResources_Master(myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.Password)
	if master_res.webroute.Name == "" {
		return brokerapi.LastOperation{
			State:       brokerapi.Failed,
			Description: "Failed!",
		}, nil
	}

	// check workers

	workers_res, _ := getSparkResources_Workers(myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.Password, handler.numWorkers)

	// todo: stat running workers pods

	if workers_res.workerrc.Name == "" || workers_res.workerrc.Spec.Replicas == nil || workers_res.workerrc.Status.Replicas < *workers_res.workerrc.Spec.Replicas {
		return brokerapi.LastOperation{
			State:       brokerapi.InProgress,
			Description: "In progress.",
		}, nil
	} else {
		return brokerapi.LastOperation{
			State:       brokerapi.Succeeded,
			Description: "Succeeded!",
		}, nil
	}

	// todo: check zeppelin
}

func (handler *Spark_Handler) DoUpdate(myServiceInfo *oshandler.ServiceInfo, planInfo oshandler.PlanInfo, callbackSaveNewInfo func(*oshandler.ServiceInfo) error, asyncAllowed bool) error {
	return nil
}

func (handler *Spark_Handler) DoDeprovision(myServiceInfo *oshandler.ServiceInfo, asyncAllowed bool) (brokerapi.IsAsync, error) {
	go func() {
		job := getSparkOrchestrationJob(myServiceInfo.Url)
		if job != nil {
			job.cancel()

			// wait job to exit
			for {
				time.Sleep(7 * time.Second)
				if nil == getSparkOrchestrationJob(myServiceInfo.Url) {
					break
				}
			}
		}

		// ...

		println("to destroy resources")

		zeppelin_res, _ := getSparkResources_Zeppelin(myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.Password)
		destroySparkResources_Zeppelin(zeppelin_res, myServiceInfo.Database)

		workers_res, _ := getSparkResources_Workers(myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.Password, handler.numWorkers)
		destroySparkResources_Workers(workers_res, myServiceInfo.Database)

		master_res, _ := getSparkResources_Master(myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.Password)
		destroySparkResources_Master(master_res, myServiceInfo.Database)
	}()

	return brokerapi.IsAsync(false), nil
}

// please note: the bsi may be still not fully initialized when calling the function.
func getCredentialsOnPrivision(myServiceInfo *oshandler.ServiceInfo) oshandler.Credentials {
	var master_res sparkResources_Master
	err := loadSparkResources_Master(myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.Password, &master_res)
	if err != nil {
		return oshandler.Credentials{}
	}

	var zeppelin_res sparkResources_Zeppelin
	err = loadSparkResources_Zeppelin(myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.Password, &zeppelin_res)
	if err != nil {
		return oshandler.Credentials{}
	}

	// todo: check if pods are created and running, return error on false.

	//master_host := master_res.webroute.Spec.Host
	master_host := fmt.Sprintf("%s.%s.%s", master_res.mastersvc.Name, myServiceInfo.Database, oshandler.ServiceDomainSuffix(false))
	master_port := strconv.Itoa(master_res.mastersvc.Spec.Ports[0].Port)
	master_uri := "spark://" + net.JoinHostPort(master_host, master_port)
	zeppelin_host := zeppelin_res.route.Spec.Host
	zeppelin_port := "80"
	zeppelin_uri := "http://" + net.JoinHostPort(zeppelin_host, zeppelin_port)

	return oshandler.Credentials{
		Uri:      fmt.Sprintf("spark: %s zeppelin: %s", master_uri, zeppelin_uri),
		Hostname: master_host,
		Port:     master_port,
		Username: "",
		Password: myServiceInfo.Password,
	}
}

func (handler *Spark_Handler) DoBind(myServiceInfo *oshandler.ServiceInfo, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, oshandler.Credentials, error) {
	// todo: handle errors

	master_res, _ := getSparkResources_Master(myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.Password)

	zeppelin_res, _ := getSparkResources_Zeppelin(myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.Password)

	// todo: check if pods are created and running, return error on false.

	//master_host := master_res.webroute.Spec.Host
	master_host := fmt.Sprintf("%s.%s.%s", master_res.mastersvc.Name, myServiceInfo.Database, oshandler.ServiceDomainSuffix(false))
	master_port := strconv.Itoa(master_res.mastersvc.Spec.Ports[0].Port)
	master_uri := "spark://" + net.JoinHostPort(master_host, master_port)
	zeppelin_host := zeppelin_res.route.Spec.Host
	zeppelin_port := "80"
	zeppelin_uri := "http://" + net.JoinHostPort(zeppelin_host, zeppelin_port)

	mycredentials := oshandler.Credentials{
		Uri:      fmt.Sprintf("spark: %s zeppelin: %s", master_uri, zeppelin_uri),
		Hostname: master_host,
		Port:     master_port,
		Username: "",
		Password: myServiceInfo.Password,
	}

	myBinding := brokerapi.Binding{Credentials: mycredentials}

	return myBinding, mycredentials, nil
}

func (handler *Spark_Handler) DoUnbind(myServiceInfo *oshandler.ServiceInfo, mycredentials *oshandler.Credentials) error {
	// do nothing

	return nil
}

//===============================================================
//
//===============================================================

var sparkOrchestrationJobs = map[string]*sparkOrchestrationJob{}
var sparkOrchestrationJobsMutex sync.Mutex

func getSparkOrchestrationJob(instanceId string) *sparkOrchestrationJob {
	sparkOrchestrationJobsMutex.Lock()
	defer sparkOrchestrationJobsMutex.Unlock()

	return sparkOrchestrationJobs[instanceId]
}

func startSparkOrchestrationJob(job *sparkOrchestrationJob) {
	sparkOrchestrationJobsMutex.Lock()
	defer sparkOrchestrationJobsMutex.Unlock()

	if sparkOrchestrationJobs[job.serviceInfo.Url] == nil {
		sparkOrchestrationJobs[job.serviceInfo.Url] = job
		go func() {
			job.run()

			sparkOrchestrationJobsMutex.Lock()
			delete(sparkOrchestrationJobs, job.serviceInfo.Url)
			sparkOrchestrationJobsMutex.Unlock()
		}()
	}
}

type sparkOrchestrationJob struct {
	//instanceId string // use serviceInfo.

	cancelled   bool
	cancelChan  chan struct{}
	cancelMetex sync.Mutex

	isProvisioning bool // false for deprovisionings

	serviceInfo    *oshandler.ServiceInfo
	planNumWorkers int

	masterResources *sparkResources_Master
	//slavesResources   *sparkResources_Workers
	//zeppelinResources   *sparkResources_Zeppelin
}

func (job *sparkOrchestrationJob) cancel() {
	job.cancelMetex.Lock()
	defer job.cancelMetex.Unlock()

	if !job.cancelled {
		job.cancelled = true
		close(job.cancelChan)
	}
}

func (job *sparkOrchestrationJob) run() {
	serviceInfo := job.serviceInfo
	rc := job.masterResources.masterrc
	uri := "/namespaces/" + serviceInfo.Database + "/replicationcontrollers/" + rc.Name
	statuses, cancel, err := oshandler.OC().KWatch(uri)
	if err != nil {
		logger.Error("start watching master rc", err)
		job.isProvisioning = false
		destroySparkResources_Master(job.masterResources, serviceInfo.Database)
		return
	}

	for {
		var status oshandler.WatchStatus
		select {
		case <-job.cancelChan:
			close(cancel)
			return
		case status, _ = <-statuses:
			break
		}

		if status.Err != nil {
			close(cancel)

			logger.Error("watch master rc error", status.Err)
			job.isProvisioning = false
			destroySparkResources_Master(job.masterResources, serviceInfo.Database)
			return
		} else {
			//logger.Debug("watch etcd pod, status.Info: " + string(status.Info))
		}

		var wrcs watchReplicationControllerStatus
		if err := json.Unmarshal(status.Info, &wrcs); err != nil {
			close(cancel)

			logger.Error("parse master rc status", err)
			job.isProvisioning = false
			destroySparkResources_Master(job.masterResources, serviceInfo.Database)
			return
		}

		if wrcs.Object.Spec.Replicas == nil { // should not happen
			close(cancel)

			logger.Error("master rc error", err)
			job.isProvisioning = false
			destroySparkResources_Master(job.masterResources, serviceInfo.Database)
			return
		}

		println("watch master rs status.replicas: ", wrcs.Object.Status.Replicas)

		if wrcs.Object.Status.Replicas >= *wrcs.Object.Spec.Replicas {
			close(cancel)
			break
		}
	}

	// wait util master pod is running

	for {
		if job.cancelled {
			return
		}

		//
		n, _ := statRunningPodsByLabels(serviceInfo.Database, rc.Labels)

		println("running pods = ", n)

		if n >= *rc.Spec.Replicas {
			break
		}

		//
		time.Sleep(10 * time.Second)
	}

	// ...

	if job.cancelled {
		return
	}

	time.Sleep(7 * time.Second)

	if job.cancelled {
		return
	}

	// ...

	println("to create workers resources")

	err = job.createSparkResources_Workers(serviceInfo.Url, serviceInfo.Database, serviceInfo.Password)
	if err != nil {
		logger.Error("create workers resources error", err)
		return
	}

	// ...

	println("to create zeppelin resources")

	err = job.createSparkResources_Zeppelin(serviceInfo.Url, serviceInfo.Database, serviceInfo.Password)
	if err != nil {
		logger.Error("create zeppelin resources error", err)
		return
	}
}

//=======================================================================
//
//=======================================================================

var SparkTemplateData_Master []byte = nil

func loadSparkResources_Master(instanceID, serviceBrokerNamespace, sparkSecret string, res *sparkResources_Master) error {
	if SparkTemplateData_Master == nil {
		f, err := os.Open("spark-master.yaml")
		if err != nil {
			return err
		}
		SparkTemplateData_Master, err = ioutil.ReadAll(f)
		if err != nil {
			return err
		}

		spark_image := oshandler.SparkImage()
		spark_image = strings.TrimSpace(spark_image)
		if len(spark_image) > 0 {
			SparkTemplateData_Master = bytes.Replace(
				SparkTemplateData_Master,
				[]byte("http://spark-image-place-holder/spark-openshift-orchestration"),
				[]byte(spark_image),
				-1)
		}
		endpoint_postfix := oshandler.EndPointSuffix()
		endpoint_postfix = strings.TrimSpace(endpoint_postfix)
		if len(endpoint_postfix) > 0 {
			SparkTemplateData_Master = bytes.Replace(
				SparkTemplateData_Master,
				[]byte("endpoint-postfix-place-holder"),
				[]byte(endpoint_postfix),
				-1)
		}
	}

	yamlTemplates := SparkTemplateData_Master

	yamlTemplates = bytes.Replace(yamlTemplates, []byte("instanceid"), []byte(instanceID), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("pass*****"), []byte(sparkSecret), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("local-service-postfix-place-holder"),
		[]byte(serviceBrokerNamespace + oshandler.ServiceDomainSuffix(true)), -1)

	//println("========= Boot yamlTemplates ===========")
	//println(string(yamlTemplates))
	//println()

	decoder := oshandler.NewYamlDecoder(yamlTemplates)
	decoder.
		Decode(&res.masterrc).
		Decode(&res.mastersvc).
		Decode(&res.webroute).
		Decode(&res.websvc)

	return decoder.Err
}

var SparkTemplateData_Workers []byte = nil

func loadSparkResources_Workers(instanceID, serviceBrokerNamespace, sparkSecret string, numWorkers int, res *sparkResources_Workers) error {
	if SparkTemplateData_Workers == nil {
		f, err := os.Open("spark-worker.yaml")
		if err != nil {
			return err
		}
		SparkTemplateData_Workers, err = ioutil.ReadAll(f)
		if err != nil {
			return err
		}

		spark_image := oshandler.SparkImage()
		spark_image = strings.TrimSpace(spark_image)
		if len(spark_image) > 0 {
			SparkTemplateData_Workers = bytes.Replace(
				SparkTemplateData_Workers,
				[]byte("http://spark-image-place-holder/spark-openshift-orchestration"),
				[]byte(spark_image),
				-1)
		}
	}

	// todo: max length of res names in kubernetes is 24

	yamlTemplates := SparkTemplateData_Workers

	yamlTemplates = bytes.Replace(yamlTemplates, []byte("instanceid"), []byte(instanceID), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("pass*****"), []byte(sparkSecret), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("num-workers-place-holder"), []byte(strconv.Itoa(numWorkers)), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("local-service-postfix-place-holder"),
		[]byte(serviceBrokerNamespace + oshandler.ServiceDomainSuffix(true)), -1)

	//println("========= HA yamlTemplates ===========")
	//println(string(yamlTemplates))
	//println()

	decoder := oshandler.NewYamlDecoder(yamlTemplates)
	decoder.
		Decode(&res.workerrc)

	return decoder.Err
}

var SparkTemplateData_Zeppelin []byte = nil

func loadSparkResources_Zeppelin(instanceID, serviceBrokerNamespace, sparkSecret string, res *sparkResources_Zeppelin) error {
	if SparkTemplateData_Zeppelin == nil {
		f, err := os.Open("spark-zeppelin.yaml")
		if err != nil {
			return err
		}
		SparkTemplateData_Zeppelin, err = ioutil.ReadAll(f)
		if err != nil {
			return err
		}

		zepplin_image := oshandler.ZepplinImage()
		zepplin_image = strings.TrimSpace(zepplin_image)
		if len(zepplin_image) > 0 {
			SparkTemplateData_Zeppelin = bytes.Replace(
				SparkTemplateData_Zeppelin,
				[]byte("http://zepplin-image-place-holder/zepplin-openshift-orchestration"),
				[]byte(zepplin_image),
				-1)
		}
		endpoint_postfix := oshandler.EndPointSuffix()
		endpoint_postfix = strings.TrimSpace(endpoint_postfix)
		if len(endpoint_postfix) > 0 {
			SparkTemplateData_Zeppelin = bytes.Replace(
				SparkTemplateData_Zeppelin,
				[]byte("endpoint-postfix-place-holder"),
				[]byte(endpoint_postfix),
				-1)
		}
	}

	// todo: max length of res names in kubernetes is 24

	yamlTemplates := SparkTemplateData_Zeppelin

	yamlTemplates = bytes.Replace(yamlTemplates, []byte("instanceid"), []byte(instanceID), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("pass*****"), []byte(sparkSecret), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("local-service-postfix-place-holder"),
		[]byte(serviceBrokerNamespace + oshandler.ServiceDomainSuffix(true)), -1)

	//println("========= HA yamlTemplates ===========")
	//println(string(yamlTemplates))
	//println()

	decoder := oshandler.NewYamlDecoder(yamlTemplates)
	decoder.
		Decode(&res.rc).
		Decode(&res.svc).
		Decode(&res.route)

	return decoder.Err
}

type sparkResources_Master struct {
	masterrc  kapi.ReplicationController
	mastersvc kapi.Service
	webroute  routeapi.Route
	websvc    kapi.Service
}

type sparkResources_Workers struct {
	workerrc kapi.ReplicationController
}

type sparkResources_Zeppelin struct {
	rc    kapi.ReplicationController
	svc   kapi.Service
	route routeapi.Route
}

func createSparkResources_Master(instanceId, serviceBrokerNamespace, sparkSecret string) (*sparkResources_Master, error) {
	var input sparkResources_Master
	err := loadSparkResources_Master(instanceId, serviceBrokerNamespace, sparkSecret, &input)
	if err != nil {
		return nil, err
	}

	var output sparkResources_Master

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	// here, not use job.post
	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		KPost(prefix+"/replicationcontrollers", &input.masterrc, &output.masterrc).
		KPost(prefix+"/services", &input.mastersvc, &output.mastersvc).
		OPost(prefix+"/routes", &input.webroute, &output.webroute).
		KPost(prefix+"/services", &input.websvc, &output.websvc)

	if osr.Err != nil {
		logger.Error("createSparkResources_Master", osr.Err)
	}

	return &output, osr.Err
}

func getSparkResources_Master(instanceId, serviceBrokerNamespace, sparkSecret string) (*sparkResources_Master, error) {
	var output sparkResources_Master

	var input sparkResources_Master
	err := loadSparkResources_Master(instanceId, serviceBrokerNamespace, sparkSecret, &input)
	if err != nil {
		return &output, err
	}

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		KGet(prefix+"/replicationcontrollers/"+input.masterrc.Name, &output.masterrc).
		KGet(prefix+"/services/"+input.mastersvc.Name, &output.mastersvc).
		OGet(prefix+"/routes/"+input.webroute.Name, &output.webroute).
		KGet(prefix+"/services/"+input.websvc.Name, &output.websvc)

	if osr.Err != nil {
		logger.Error("getSparkResources_Master", osr.Err)
	}

	return &output, osr.Err
}

func destroySparkResources_Master(masterRes *sparkResources_Master, serviceBrokerNamespace string) {
	// todo: add to retry queue on fail

	go func() { kdel_rc(serviceBrokerNamespace, &masterRes.masterrc) }()
	go func() { kdel(serviceBrokerNamespace, "services", masterRes.mastersvc.Name) }()
	go func() { odel(serviceBrokerNamespace, "routes", masterRes.webroute.Name) }()
	go func() { kdel(serviceBrokerNamespace, "services", masterRes.websvc.Name) }()
}

func (job *sparkOrchestrationJob) createSparkResources_Workers(instanceId, serviceBrokerNamespace, sparkSecret string) error {
	var input sparkResources_Workers
	err := loadSparkResources_Workers(instanceId, serviceBrokerNamespace, sparkSecret, job.planNumWorkers, &input)
	if err != nil {
		return err
	}

	var output sparkResources_Workers
	go func() {
		if err := job.kpost(serviceBrokerNamespace, "replicationcontrollers", &input.workerrc, &output.workerrc); err != nil {
			return
		}
	}()

	return nil
}

func getSparkResources_Workers(instanceId, serviceBrokerNamespace, sparkSecret string, numWorkers int) (*sparkResources_Workers, error) {
	var output sparkResources_Workers

	var input sparkResources_Workers
	err := loadSparkResources_Workers(instanceId, serviceBrokerNamespace, sparkSecret, numWorkers, &input)
	if err != nil {
		return &output, err
	}

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		KGet(prefix+"/replicationcontrollers/"+input.workerrc.Name, &output.workerrc)

	if osr.Err != nil {
		logger.Error("getSparkResources_Workers", osr.Err)
	}

	return &output, osr.Err
}

func destroySparkResources_Workers(haRes *sparkResources_Workers, serviceBrokerNamespace string) {
	// todo: add to retry queue on fail

	go func() { kdel_rc(serviceBrokerNamespace, &haRes.workerrc) }()
}

func (job *sparkOrchestrationJob) createSparkResources_Zeppelin(instanceId, serviceBrokerNamespace, sparkSecret string) error {
	var input sparkResources_Zeppelin
	err := loadSparkResources_Zeppelin(instanceId, serviceBrokerNamespace, sparkSecret, &input)
	if err != nil {
		return err
	}

	var output sparkResources_Zeppelin
	go func() {
		if err := job.kpost(serviceBrokerNamespace, "replicationcontrollers", &input.rc, &output.rc); err != nil {
			return
		}
		if err := job.kpost(serviceBrokerNamespace, "services", &input.svc, &output.svc); err != nil {
			return
		}
		if err := job.opost(serviceBrokerNamespace, "routes", &input.route, &output.route); err != nil {
			return
		}
	}()

	return nil
}

func getSparkResources_Zeppelin(instanceId, serviceBrokerNamespace, sparkSecret string) (*sparkResources_Zeppelin, error) {
	var output sparkResources_Zeppelin

	var input sparkResources_Zeppelin
	err := loadSparkResources_Zeppelin(instanceId, serviceBrokerNamespace, sparkSecret, &input)
	if err != nil {
		return &output, err
	}

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		KGet(prefix+"/replicationcontrollers/"+input.rc.Name, &output.rc).
		KGet(prefix+"/services/"+input.svc.Name, &output.svc).
		OGet(prefix+"/routes/"+input.route.Name, &output.route)

	if osr.Err != nil {
		logger.Error("getSparkResources_Zeppelin", osr.Err)
	}

	return &output, osr.Err
}

func destroySparkResources_Zeppelin(zeppelinRes *sparkResources_Zeppelin, serviceBrokerNamespace string) {
	// todo: add to retry queue on fail

	go func() { kdel_rc(serviceBrokerNamespace, &zeppelinRes.rc) }()
	go func() { kdel(serviceBrokerNamespace, "services", zeppelinRes.svc.Name) }()
	go func() { odel(serviceBrokerNamespace, "routes", zeppelinRes.route.Name) }()
}

//===============================================================
//
//===============================================================

func (job *sparkOrchestrationJob) kpost(serviceBrokerNamespace, typeName string, body interface{}, into interface{}) error {
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

func (job *sparkOrchestrationJob) opost(serviceBrokerNamespace, typeName string, body interface{}, into interface{}) error {
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
				logger.Error("watch HA spark rc error", status.Err)
				close(cancel)
				return
			} else {
				//logger.Debug("watch spark HA rc, status.Info: " + string(status.Info))
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
