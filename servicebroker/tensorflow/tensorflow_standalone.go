package tensorflow

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	oshandler "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/handler"
	routeapi "github.com/openshift/origin/route/api/v1"
	"github.com/pivotal-cf/brokerapi"
	//"github.com/pivotal-golang/lager"
	logger "github.com/golang/glog"
	"io/ioutil"
	kapi "k8s.io/kubernetes/pkg/api/v1"
	"net"
	"os"
	"strconv"
	"strings"
)

//==============================================================
//初始化Log
//==============================================================

const TensorFlowServcieBrokerName_Standalone = "TensorFlow_standalone"

func init() {
	oshandler.Register(TensorFlowServcieBrokerName_Standalone, &TensorFlow_freeHandler{})

	//logger = lager.NewLogger(TensorFlowServcieBrokerName_Standalone)
	//logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))
}

//var logger lager.Logger

//==============================================================
//
//==============================================================

type TensorFlow_freeHandler struct{}

func (handler *TensorFlow_freeHandler) DoProvision(etcdSaveResult chan error, instanceID string, details brokerapi.ProvisionDetails, planInfo oshandler.PlanInfo, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, oshandler.ServiceInfo, error) {
	return newTensorFlowHandler().DoProvision(etcdSaveResult, instanceID, details, planInfo, asyncAllowed)
}

func (handler *TensorFlow_freeHandler) DoLastOperation(myServiceInfo *oshandler.ServiceInfo) (brokerapi.LastOperation, error) {
	return newTensorFlowHandler().DoLastOperation(myServiceInfo)
}

func (handler *TensorFlow_freeHandler) DoUpdate(myServiceInfo *oshandler.ServiceInfo, planInfo oshandler.PlanInfo, callbackSaveNewInfo func(*oshandler.ServiceInfo) error, asyncAllowed bool) error {
	return newTensorFlowHandler().DoUpdate(myServiceInfo, planInfo, callbackSaveNewInfo, asyncAllowed)
}

func (handler *TensorFlow_freeHandler) DoDeprovision(myServiceInfo *oshandler.ServiceInfo, asyncAllowed bool) (brokerapi.IsAsync, error) {
	return newTensorFlowHandler().DoDeprovision(myServiceInfo, asyncAllowed)
}

func (handler *TensorFlow_freeHandler) DoBind(myServiceInfo *oshandler.ServiceInfo, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, oshandler.Credentials, error) {
	return newTensorFlowHandler().DoBind(myServiceInfo, bindingID, details)
}

func (handler *TensorFlow_freeHandler) DoUnbind(myServiceInfo *oshandler.ServiceInfo, mycredentials *oshandler.Credentials) error {
	return newTensorFlowHandler().DoUnbind(myServiceInfo, mycredentials)
}

//==============================================================
//
//==============================================================

type TensorFlow_Handler struct {
}

func newTensorFlowHandler() *TensorFlow_Handler {
	return &TensorFlow_Handler{}
}

func (handler *TensorFlow_Handler) DoProvision(etcdSaveResult chan error, instanceID string, details brokerapi.ProvisionDetails, planInfo oshandler.PlanInfo, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, oshandler.ServiceInfo, error) {
	//初始化到openshift的链接

	serviceSpec := brokerapi.ProvisionedServiceSpec{IsAsync: asyncAllowed}
	serviceInfo := oshandler.ServiceInfo{}

	serviceSpec.IsAsync = true

	instanceIdInTempalte := strings.ToLower(oshandler.NewThirteenLengthID())
	serviceBrokerNamespace := oshandler.OC().Namespace()
	tensorflowUser := oshandler.NewElevenLengthID()
	tensorflowPassword := oshandler.GenGUID()

	serviceInfo.Url = instanceIdInTempalte
	serviceInfo.Database = serviceBrokerNamespace // may be not needed
	serviceInfo.User = tensorflowUser
	serviceInfo.Password = tensorflowPassword

	logger.Info("Tensorflow Creating ...", map[string]interface{}{"instanceIdInTempalte": instanceIdInTempalte, "serviceBrokerNamespace": serviceBrokerNamespace})

	go func() {
		err := <-etcdSaveResult
		if err != nil {
			return
		}

		// master tensorflow
		output, err := createTensorFlowResources_Master(instanceIdInTempalte, serviceBrokerNamespace, tensorflowUser, tensorflowPassword)

		if err != nil {
			destroyTensorFlowResources_Master(output, serviceBrokerNamespace)

			return
		}

	}()

	var input tensorflowResources_Master
	err := loadTensorFlowResources_Master(instanceIdInTempalte, tensorflowUser, tensorflowPassword, &input)
	if err != nil {
		return serviceSpec, serviceInfo, err
	}

	serviceSpec.DashboardURL = "http://" + net.JoinHostPort(input.route.Spec.Host, "80")

	//>>>
	serviceSpec.Credentials = getCredentialsOnPrivision(&serviceInfo)
	//<<<

	return serviceSpec, serviceInfo, nil
}

func (handler *TensorFlow_Handler) DoLastOperation(myServiceInfo *oshandler.ServiceInfo) (brokerapi.LastOperation, error) {

	// assume in provisioning

	// the job may be finished or interrupted or running in another instance.

	master_res, _ := getTensorFlowResources_Master(myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.User, myServiceInfo.Password)

	ok := func(rc *kapi.ReplicationController) bool {
		if rc == nil || rc.Name == "" || rc.Spec.Replicas == nil || rc.Status.Replicas < *rc.Spec.Replicas {
			return false
		}
		n, _ := statRunningPodsByLabels(myServiceInfo.Database, rc.Labels)
		return n >= *rc.Spec.Replicas
	}

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

func (handler *TensorFlow_Handler) DoUpdate(myServiceInfo *oshandler.ServiceInfo, planInfo oshandler.PlanInfo, callbackSaveNewInfo func(*oshandler.ServiceInfo) error, asyncAllowed bool) error {
	return nil
}

func (handler *TensorFlow_Handler) DoDeprovision(myServiceInfo *oshandler.ServiceInfo, asyncAllowed bool) (brokerapi.IsAsync, error) {
	// ...

	logger.Infoln("to destroy resources")

	master_res, _ := getTensorFlowResources_Master(myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.User, myServiceInfo.Password)
	destroyTensorFlowResources_Master(master_res, myServiceInfo.Database)

	return brokerapi.IsAsync(false), nil
}

// please note: the bsi may be still not fully initialized when calling the function.
func getCredentialsOnPrivision(myServiceInfo *oshandler.ServiceInfo) oshandler.Credentials {
	var master_res tensorflowResources_Master
	err := loadTensorFlowResources_Master(myServiceInfo.Url, myServiceInfo.User, myServiceInfo.Password, &master_res)
	if err != nil {
		return oshandler.Credentials{}
	}

	web_port := oshandler.GetServicePortByName(&master_res.service, "web")
	if web_port == nil {
		return oshandler.Credentials{}
	}

	host := fmt.Sprintf("%s.%s.%s", master_res.service.Name, myServiceInfo.Database, oshandler.ServiceDomainSuffix(false))
	port := strconv.Itoa(web_port.Port)

	return oshandler.Credentials{
		Uri:      "",
		Hostname: host,
		Port:     port,
	}
}

func (handler *TensorFlow_Handler) DoBind(myServiceInfo *oshandler.ServiceInfo, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, oshandler.Credentials, error) {
	// todo: handle errors

	master_res, err := getTensorFlowResources_Master(myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.User, myServiceInfo.Password)
	if err != nil {
		return brokerapi.Binding{}, oshandler.Credentials{}, err
	}

	web_port := oshandler.GetServicePortByName(&master_res.service, "web")
	if web_port == nil {
		return brokerapi.Binding{}, oshandler.Credentials{}, errors.New("web port not found")
	}

	host := fmt.Sprintf("%s.%s.%s", master_res.service.Name, myServiceInfo.Database, oshandler.ServiceDomainSuffix(false))
	port := strconv.Itoa(web_port.Port)

	mycredentials := oshandler.Credentials{
		Uri:      "",
		Hostname: host,
		Port:     port,
	}

	myBinding := brokerapi.Binding{Credentials: mycredentials}

	return myBinding, mycredentials, nil
}

func (handler *TensorFlow_Handler) DoUnbind(myServiceInfo *oshandler.ServiceInfo, mycredentials *oshandler.Credentials) error {
	// do nothing

	return nil
}

//=======================================================================
//
//=======================================================================

var TensorFlowTemplateData_Master []byte = nil

func loadTensorFlowResources_Master(instanceID, tensorflowUser, tensorflowPassword string, res *tensorflowResources_Master) error {
	if TensorFlowTemplateData_Master == nil {
		f, err := os.Open("tensorflow.yaml")
		if err != nil {
			return err
		}
		TensorFlowTemplateData_Master, err = ioutil.ReadAll(f)
		if err != nil {
			return err
		}
		endpoint_postfix := oshandler.EndPointSuffix()
		endpoint_postfix = strings.TrimSpace(endpoint_postfix)
		if len(endpoint_postfix) > 0 {
			TensorFlowTemplateData_Master = bytes.Replace(
				TensorFlowTemplateData_Master,
				[]byte("endpoint-postfix-place-holder"),
				[]byte(endpoint_postfix),
				-1)
		}
		tensorflow_image := oshandler.TensorFlowImage()
		tensorflow_image = strings.TrimSpace(tensorflow_image)
		if len(tensorflow_image) > 0 {
			TensorFlowTemplateData_Master = bytes.Replace(
				TensorFlowTemplateData_Master,
				[]byte("http://tensorflow-image-place-holder/tensorflow-openshift-orchestration"),
				[]byte(tensorflow_image),
				-1)
		}
	}

	// ...

	yamlTemplates := TensorFlowTemplateData_Master

	yamlTemplates = bytes.Replace(yamlTemplates, []byte("instanceid"), []byte(instanceID), -1)

	decoder := oshandler.NewYamlDecoder(yamlTemplates)
	decoder.
		Decode(&res.rc).
		Decode(&res.route).
		Decode(&res.service)

	return decoder.Err
}

type tensorflowResources_Master struct {
	rc      kapi.ReplicationController
	route   routeapi.Route
	service kapi.Service
}

func createTensorFlowResources_Master(instanceId, serviceBrokerNamespace, tensorflowUser, tensorflowPassword string) (*tensorflowResources_Master, error) {
	var input tensorflowResources_Master
	err := loadTensorFlowResources_Master(instanceId, tensorflowUser, tensorflowPassword, &input)
	if err != nil {
		return nil, err
	}

	var output tensorflowResources_Master

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	// here, not use job.post
	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		KPost(prefix+"/replicationcontrollers", &input.rc, &output.rc).
		OPost(prefix+"/routes", &input.route, &output.route).
		KPost(prefix+"/services", &input.service, &output.service)

	if osr.Err != nil {
		logger.Error("createTensorFlowResources_Master", osr.Err)
	}

	return &output, osr.Err
}

func getTensorFlowResources_Master(instanceId, serviceBrokerNamespace, tensorflowUser, tensorflowPassword string) (*tensorflowResources_Master, error) {
	var output tensorflowResources_Master

	var input tensorflowResources_Master
	err := loadTensorFlowResources_Master(instanceId, tensorflowUser, tensorflowPassword, &input)
	if err != nil {
		return &output, err
	}

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		KGet(prefix+"/replicationcontrollers/"+input.rc.Name, &output.rc).
		OGet(prefix+"/routes/"+input.route.Name, &output.route).
		KGet(prefix+"/services/"+input.service.Name, &output.service)

	if osr.Err != nil {
		logger.Error("getTensorFlowResources_Master", osr.Err)
	}

	return &output, osr.Err
}

func destroyTensorFlowResources_Master(masterRes *tensorflowResources_Master, serviceBrokerNamespace string) {
	// todo: add to retry queue on fail

	go func() { kdel_rc(serviceBrokerNamespace, &masterRes.rc) }()
	go func() { odel(serviceBrokerNamespace, "routes", masterRes.route.Name) }()
	go func() { kdel(serviceBrokerNamespace, "services", masterRes.service.Name) }()
}

//===============================================================
//
//===============================================================

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

func odel(serviceBrokerNamespace, typeName, resName string) error {
	if resName == "" {
		return nil
	}

	logger.Infoln("to delete ", typeName, "/", resName)

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

	logger.Infoln("to delete pods on replicationcontroller", rc.Name)

	uri := "/namespaces/" + serviceBrokerNamespace + "/replicationcontrollers/" + rc.Name

	// modfiy rc replicas to 0

	zero := 0
	rc.Spec.Replicas = &zero
	osr := oshandler.NewOpenshiftREST(oshandler.OC()).KPut(uri, rc, nil)
	if osr.Err != nil {
		logger.Error("Modify Tensorflow rc", osr.Err)
		return
	}

	// start watching rc status

	statuses, cancel, err := oshandler.OC().KWatch(uri)
	if err != nil {
		logger.Error("Start Watching Tensorflow rc", err)
		return
	}

	go func() {
		for {
			status, _ := <-statuses

			if status.Err != nil {
				logger.Error("Watch Tensorflow rc error", status.Err)
				close(cancel)
				return
			} else {
				//logger.Debug("watch tensorflow HA rc, status.Info: " + string(status.Info))
			}

			var wrcs watchReplicationControllerStatus
			if err := json.Unmarshal(status.Info, &wrcs); err != nil {
				logger.Error("Parse Master Tensorflow rc status", err)
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
