package rabbitmq


import (
	"fmt"
	"errors"
	//marathon "github.com/gambol99/go-marathon"
	//kapi "golang.org/x/build/kubernetes/api"
	//"golang.org/x/build/kubernetes"
	//"golang.org/x/oauth2"
	//"net/http"
	"net"
	"github.com/pivotal-cf/brokerapi"
	//"time"
	"strconv"
	"strings"
	"bytes"
	"encoding/json"
	//"crypto/sha1"
	//"encoding/base64"
	//"text/template"
	//"io"
	"io/ioutil"
	"os"
	//"sync"
	
	"github.com/pivotal-golang/lager"
	
	//"k8s.io/kubernetes/pkg/util/yaml"
	kapi "k8s.io/kubernetes/pkg/api/v1"
	routeapi "github.com/openshift/origin/route/api/v1"
	
	oshandler "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/handler"
)

//==============================================================
// 
//==============================================================

const RabbitmqServcieBrokerName_Standalone = "RabbitMQ_standalone"

func init() {
	oshandler.Register(RabbitmqServcieBrokerName_Standalone, &Rabbitmq_freeHandler{})
	
	logger = lager.NewLogger(RabbitmqServcieBrokerName_Standalone)
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))
}

var logger lager.Logger

//==============================================================
// 
//==============================================================

type Rabbitmq_freeHandler struct{}

func (handler *Rabbitmq_freeHandler) DoProvision(instanceID string, details brokerapi.ProvisionDetails, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, oshandler.ServiceInfo, error) {
	return newRabbitmqHandler().DoProvision(instanceID, details, asyncAllowed)
}

func (handler *Rabbitmq_freeHandler) DoLastOperation(myServiceInfo *oshandler.ServiceInfo) (brokerapi.LastOperation, error) {
	return newRabbitmqHandler().DoLastOperation(myServiceInfo)
}

func (handler *Rabbitmq_freeHandler) DoDeprovision(myServiceInfo *oshandler.ServiceInfo, asyncAllowed bool) (brokerapi.IsAsync, error) {
	return newRabbitmqHandler().DoDeprovision(myServiceInfo, asyncAllowed)
}

func (handler *Rabbitmq_freeHandler) DoBind(myServiceInfo *oshandler.ServiceInfo, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, oshandler.Credentials, error) {
	return newRabbitmqHandler().DoBind(myServiceInfo, bindingID, details)
}

func (handler *Rabbitmq_freeHandler) DoUnbind(myServiceInfo *oshandler.ServiceInfo, mycredentials *oshandler.Credentials) error {
	return newRabbitmqHandler().DoUnbind(myServiceInfo, mycredentials)
}

//==============================================================
// 
//==============================================================

type Rabbitmq_Handler struct{
}

func newRabbitmqHandler() *Rabbitmq_Handler {
	return &Rabbitmq_Handler{}
}

func (handler *Rabbitmq_Handler) DoProvision(instanceID string, details brokerapi.ProvisionDetails, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, oshandler.ServiceInfo, error) {
	//初始化到openshift的链接
	
	serviceSpec := brokerapi.ProvisionedServiceSpec{IsAsync: asyncAllowed}
	serviceInfo := oshandler.ServiceInfo{}
	
	//if asyncAllowed == false {
	//	return serviceSpec, serviceInfo, errors.New("Sync mode is not supported")
	//}
	serviceSpec.IsAsync = true
	
	//instanceIdInTempalte   := instanceID // todo: ok?
	instanceIdInTempalte   := strings.ToLower(oshandler.NewThirteenLengthID())
	//serviceBrokerNamespace := ServiceBrokerNamespace
	serviceBrokerNamespace := oshandler.OC().Namespace()
	rabbitmqUser := oshandler.NewElevenLengthID()
	rabbitmqPassword := oshandler.GenGUID()
	
	println()
	println("instanceIdInTempalte = ", instanceIdInTempalte)
	println("serviceBrokerNamespace = ", serviceBrokerNamespace)
	println()
	
	// master rabbitmq
	
	output, err := createRabbitmqResources_Master(instanceIdInTempalte, serviceBrokerNamespace, rabbitmqUser, rabbitmqPassword)

	if err != nil {
		destroyRabbitmqResources_Master(output, serviceBrokerNamespace)
		
		return serviceSpec, serviceInfo, err
	}
	
	serviceInfo.Url = instanceIdInTempalte
	serviceInfo.Database = serviceBrokerNamespace // may be not needed
	serviceInfo.User = rabbitmqUser
	serviceInfo.Password = rabbitmqPassword
	
	serviceSpec.DashboardURL = "http://" + net.JoinHostPort(output.routeAdmin.Spec.Host, "80")
	
	return serviceSpec, serviceInfo, nil
}

func (handler *Rabbitmq_Handler) DoLastOperation(myServiceInfo *oshandler.ServiceInfo) (brokerapi.LastOperation, error) {
	
	// assume in provisioning
	
	// the job may be finished or interrupted or running in another instance.
	
	master_res, _ := getRabbitmqResources_Master (myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.User, myServiceInfo.Password)
	
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
		n, _ := statRunningPodsByLabels (myServiceInfo.Database, rc.Labels)
		return n >= *rc.Spec.Replicas
	}
	
	//println("num_ok_rcs = ", num_ok_rcs)
	
	if ok (&master_res.rc) {
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

func (handler *Rabbitmq_Handler) DoDeprovision(myServiceInfo *oshandler.ServiceInfo, asyncAllowed bool) (brokerapi.IsAsync, error) {
	// ...
	
	println("to destroy resources")
	
	master_res, _ := getRabbitmqResources_Master (myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.User, myServiceInfo.Password)
	destroyRabbitmqResources_Master (master_res, myServiceInfo.Database)
	
	return brokerapi.IsAsync(false), nil
}

func (handler *Rabbitmq_Handler) DoBind(myServiceInfo *oshandler.ServiceInfo, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, oshandler.Credentials, error) {
	// todo: handle errors
	
	master_res, err := getRabbitmqResources_Master (myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.User, myServiceInfo.Password)
	if err != nil {
		return brokerapi.Binding{}, oshandler.Credentials{}, err
	}
	
	mq_port := oshandler.GetServicePortByName(&master_res.service, "mq")
	if mq_port == nil {
		return brokerapi.Binding{}, oshandler.Credentials{}, errors.New("mq port not found")
	}
	
	host := fmt.Sprintf("%s.%s.svc.cluster.local", master_res.service.Name, myServiceInfo.Database)
	port := strconv.Itoa(mq_port.Port)
	//host := master_res.routeMQ.Spec.Host
	//port := "80"
	
	mycredentials := oshandler.Credentials{
		Uri:      fmt.Sprintf("amqp://%s:%s@%s:%s", myServiceInfo.User, myServiceInfo.Password, host, port),
		Hostname: host,
		Port:     port,
		Username: myServiceInfo.User,
		Password: myServiceInfo.Password,
	}

	myBinding := brokerapi.Binding{Credentials: mycredentials}

	return myBinding, mycredentials, nil
}

func (handler *Rabbitmq_Handler) DoUnbind(myServiceInfo *oshandler.ServiceInfo, mycredentials *oshandler.Credentials) error {
	// do nothing
	
	return nil
}

//=======================================================================
// 
//=======================================================================

var RabbitmqTemplateData_Master []byte = nil

func loadRabbitmqResources_Master(instanceID, rabbitmqUser, rabbitmqPassword string, res *rabbitmqResources_Master) error {
	if RabbitmqTemplateData_Master == nil {
		f, err := os.Open("rabbitmq.yaml")
		if err != nil {
			return err
		}
		RabbitmqTemplateData_Master, err = ioutil.ReadAll(f)
		if err != nil {
			return err
		}
		endpoint_postfix := oshandler.EndPointSuffix()
		endpoint_postfix = strings.TrimSpace(endpoint_postfix)
		if len(endpoint_postfix) > 0 {
			RabbitmqTemplateData_Master = bytes.Replace(
				RabbitmqTemplateData_Master, 
				[]byte("endpoint-postfix-place-holder"), 
				[]byte(endpoint_postfix), 
				-1)
		}
	}
	
	// ...
	
	yamlTemplates := RabbitmqTemplateData_Master
	
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("instanceid"), []byte(instanceID), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("user*****"), []byte(rabbitmqUser), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("pass*****"), []byte(rabbitmqPassword), -1)
	
	//println("========= Boot yamlTemplates ===========")
	//println(string(yamlTemplates))
	//println()

	
	decoder := oshandler.NewYamlDecoder(yamlTemplates)
	decoder.
		Decode(&res.rc).
		Decode(&res.routeAdmin).
		//Decode(&res.routeMQ).
		Decode(&res.service)
	
	return decoder.Err
}

type rabbitmqResources_Master struct {
	rc         kapi.ReplicationController
	routeAdmin routeapi.Route
	//routeMQ    routeapi.Route
	service    kapi.Service
}
	
func createRabbitmqResources_Master (instanceId, serviceBrokerNamespace, rabbitmqUser, rabbitmqPassword string) (*rabbitmqResources_Master, error) {
	var input rabbitmqResources_Master
	err := loadRabbitmqResources_Master(instanceId, rabbitmqUser, rabbitmqPassword, &input)
	if err != nil {
		return nil, err
	}
	
	var output rabbitmqResources_Master
	
	osr := oshandler.NewOpenshiftREST(oshandler.OC())
	
	// here, not use job.post
	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		KPost(prefix + "/replicationcontrollers", &input.rc, &output.rc).
		OPost(prefix + "/routes", &input.routeAdmin, &output.routeAdmin).
		//OPost(prefix + "/routes", &input.routeMQ, &output.routeMQ).
		KPost(prefix + "/services", &input.service, &output.service)
	
	if osr.Err != nil {
		logger.Error("createRabbitmqResources_Master", osr.Err)
	}
	
	return &output, osr.Err
}
	
func getRabbitmqResources_Master (instanceId, serviceBrokerNamespace, rabbitmqUser, rabbitmqPassword string) (*rabbitmqResources_Master, error) {
	var output rabbitmqResources_Master
	
	var input rabbitmqResources_Master
	err := loadRabbitmqResources_Master(instanceId, rabbitmqUser, rabbitmqPassword, &input)
	if err != nil {
		return &output, err
	}
	
	osr := oshandler.NewOpenshiftREST(oshandler.OC())
	
	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		KGet(prefix + "/replicationcontrollers/" + input.rc.Name, &output.rc).
		OGet(prefix + "/routes/" + input.routeAdmin.Name, &output.routeAdmin).
		//OGet(prefix + "/routes/" + input.routeMQ.Name, &output.routeMQ).
		KGet(prefix + "/services/" + input.service.Name, &output.service)
	
	if osr.Err != nil {
		logger.Error("getRabbitmqResources_Master", osr.Err)
	}
	
	return &output, osr.Err
}

func destroyRabbitmqResources_Master (masterRes *rabbitmqResources_Master, serviceBrokerNamespace string) {
	// todo: add to retry queue on fail

	go func() {kdel_rc (serviceBrokerNamespace, &masterRes.rc)}()
	go func() {odel (serviceBrokerNamespace, "routes", masterRes.routeAdmin.Name)}()
	//go func() {odel (serviceBrokerNamespace, "routes", masterRes.routeMQ.Name)}()
	go func() {kdel (serviceBrokerNamespace, "services", masterRes.service.Name)}()
}

//===============================================================
// 
//===============================================================

func kpost (serviceBrokerNamespace, typeName string, body interface{}, into interface{}) error {
	println("to create ", typeName)
	
	uri := fmt.Sprintf("/namespaces/%s/%s", serviceBrokerNamespace, typeName)
	i, n := 0, 5
RETRY:
	
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

func opost (serviceBrokerNamespace, typeName string, body interface{}, into interface{}) error {
	println("to create ", typeName)
	
	uri := fmt.Sprintf("/namespaces/%s/%s", serviceBrokerNamespace, typeName)
	i, n := 0, 5
RETRY:
	
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
	
func kdel (serviceBrokerNamespace, typeName, resName string) error {
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

func odel (serviceBrokerNamespace, typeName, resName string) error {
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

func kdel_rc (serviceBrokerNamespace string, rc *kapi.ReplicationController) {
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
	
	statuses, cancel, err := oshandler.OC().KWatch (uri)
	if err != nil {
		logger.Error("start watching HA rc", err)
		return
	}
	
	go func() {
		for {
			status, _ := <- statuses
			
			if status.Err != nil {
				logger.Error("watch HA rabbitmq rc error", status.Err)
				close(cancel)
				return
			} else {
				//logger.Debug("watch rabbitmq HA rc, status.Info: " + string(status.Info))
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
			nrunnings ++
		}
	}
	
	return nrunnings, nil
}

// todo: 

/*
bin/zkCli.sh 127.0.0.1:2181
bin/zkCli.sh -server sb-instanceid-zk:2181

echo conf|nc localhost 2181
echo cons|nc localhost 2181
echo ruok|nc localhost 2181
echo srst|nc localhost 2181
echo crst|nc localhost 2181
echo dump|nc localhost 2181
echo srvr|nc localhost 2181
echo stat|nc localhost 2181
echo mntr|nc localhost 2181
*/