package rabbitmq_pvc

import (
	"errors"
	"fmt"
	"bytes"
	"encoding/json"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/pivotal-cf/brokerapi"
	"io/ioutil"
	"os"
	"github.com/pivotal-golang/lager"
	routeapi "github.com/openshift/origin/route/api/v1"
	kapi "k8s.io/kubernetes/pkg/api/v1"
	oshandler "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/handler"
)

//==============================================================
//初始化Log
//==============================================================

const RabbitmqServcieBrokerName_Standalone = "RabbitMQ_volumes_standalone"

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

func (handler *Rabbitmq_freeHandler) DoProvision(etcdSaveResult chan error, instanceID string, details brokerapi.ProvisionDetails, planInfo oshandler.PlanInfo, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, oshandler.ServiceInfo, error) {
	return newRabbitmqHandler().DoProvision(etcdSaveResult, instanceID, details, planInfo, asyncAllowed)
}

func (handler *Rabbitmq_freeHandler) DoLastOperation(myServiceInfo *oshandler.ServiceInfo) (brokerapi.LastOperation, error) {
	return newRabbitmqHandler().DoLastOperation(myServiceInfo)
}

func (handler *Rabbitmq_freeHandler) DoUpdate(myServiceInfo *oshandler.ServiceInfo, planInfo oshandler.PlanInfo, callbackSaveNewInfo func(*oshandler.ServiceInfo) error, asyncAllowed bool) error {
	return newRabbitmqHandler().DoUpdate(myServiceInfo, planInfo, callbackSaveNewInfo, asyncAllowed)
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
//挂卷
//==============================================================

// version 1:
//   one peer volume,

func volumeBaseName(instanceId string) string {
	return "rbbtmq-" + instanceId
}

func peerPvcName0(volumes []oshandler.Volume) string {
	if len(volumes) > 0 {
		return volumes[0].Volume_name
	}
	return ""
}

//==============================================================
//
//==============================================================

type Rabbitmq_Handler struct {
}

func newRabbitmqHandler() *Rabbitmq_Handler {
	return &Rabbitmq_Handler{}
}

func (handler *Rabbitmq_Handler) DoProvision(etcdSaveResult chan error, instanceID string, details brokerapi.ProvisionDetails, planInfo oshandler.PlanInfo, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, oshandler.ServiceInfo, error) {
	//初始化到openshift的链接

	serviceSpec := brokerapi.ProvisionedServiceSpec{IsAsync: asyncAllowed}
	serviceInfo := oshandler.ServiceInfo{}

	serviceSpec.IsAsync = true

	instanceIdInTempalte := strings.ToLower(oshandler.NewThirteenLengthID())
	serviceBrokerNamespace := oshandler.OC().Namespace()
	rabbitmqUser := oshandler.NewElevenLengthID()
	rabbitmqPassword := oshandler.GenGUID()

	volumeBaseName := volumeBaseName(instanceIdInTempalte)
	volumes := []oshandler.Volume{
		// one peer volume
		{
			Volume_size: planInfo.Volume_size,
			Volume_name: volumeBaseName + "-0",
		},
	}

	logger.Info("Rabbitmq Creating ...", map[string]interface{}{"instanceIdInTempalte": instanceIdInTempalte, "serviceBrokerNamespace": serviceBrokerNamespace})


	// master rabbitmq

	serviceInfo.Url = instanceIdInTempalte
	serviceInfo.Database = serviceBrokerNamespace // may be not needed
	serviceInfo.User = rabbitmqUser
	serviceInfo.Password = rabbitmqPassword

	serviceInfo.Volumes = volumes

	//>> may be not optimized
	var template rabbitmqResources_Master
	err := loadRabbitmqResources_Master(
		serviceInfo.Url,
		serviceInfo.User,
		serviceInfo.Password,
		serviceInfo.Volumes,
		&template)
	if err != nil {
		logger.Error("loadRabbitmqResources_Master error",err)
		return serviceSpec, oshandler.ServiceInfo{}, err
	}
	//<<

	nodePort, err := createRabbitmqResources_NodePort(
		&template,
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

		// create volumes

		result := oshandler.StartCreatePvcVolumnJob(
			volumeBaseName,
			serviceInfo.Database,
			serviceInfo.Volumes,
		)

		err = <-result
		if err != nil {
			logger.Error("rabbitmq create volume", err)
			handler.DoDeprovision(&serviceInfo, true)
			return
		}

		println("createRabbitmqResources_Master ...")

		// create master res

		output, err := createRabbitmqResources_Master(
			serviceInfo.Url,
			serviceInfo.Database,
			serviceInfo.User,
			serviceInfo.Password,
			serviceInfo.Volumes,
		)
		if err != nil {

			destroyRabbitmqResources_Master(output, serviceBrokerNamespace)
			oshandler.DeleteVolumns(serviceInfo.Database, volumes)

			return
		}
	}()

	serviceSpec.DashboardURL = "http://" + net.JoinHostPort(template.routeAdmin.Spec.Host, "80")

	//>>>
	serviceSpec.Credentials = getCredentialsOnPrivision(&serviceInfo, nodePort)
	//<<<

	return serviceSpec, serviceInfo, nil
}

func (handler *Rabbitmq_Handler) DoLastOperation(myServiceInfo *oshandler.ServiceInfo) (brokerapi.LastOperation, error) {

	// assume in provisioning

	volumeJob := oshandler.GetCreatePvcVolumnJob(volumeBaseName(myServiceInfo.Url))
	if volumeJob != nil {
		return brokerapi.LastOperation{
			State:       brokerapi.InProgress,
			Description: "in progress.",
		}, nil
	}

	// the job may be finished or interrupted or running in another instance.

	master_res, _ := getRabbitmqResources_Master(
		myServiceInfo.Url,
		myServiceInfo.Database,
		myServiceInfo.User,
		myServiceInfo.Password,
		myServiceInfo.Volumes,
	)

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

func (handler *Rabbitmq_Handler) DoUpdate(myServiceInfo *oshandler.ServiceInfo, planInfo oshandler.PlanInfo, callbackSaveNewInfo func(*oshandler.ServiceInfo) error, asyncAllowed bool) error {
	return errors.New("not implemented")
}

func (handler *Rabbitmq_Handler) DoDeprovision(myServiceInfo *oshandler.ServiceInfo, asyncAllowed bool) (brokerapi.IsAsync, error) {
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

		println("to destroy resources:", myServiceInfo.Url)

		master_res, _ := getRabbitmqResources_Master(
			myServiceInfo.Url,
			myServiceInfo.Database,
			myServiceInfo.User,
			myServiceInfo.Password,
			myServiceInfo.Volumes,
		)
		destroyRabbitmqResources_Master(master_res, myServiceInfo.Database)

		// ...

		println("to destroy volumes:", myServiceInfo.Volumes)

		oshandler.DeleteVolumns(myServiceInfo.Database, myServiceInfo.Volumes)
	}()

	return brokerapi.IsAsync(false), nil
}

// please note: the bsi may be still not fully initialized when calling the function.
func getCredentialsOnPrivision(myServiceInfo *oshandler.ServiceInfo, nodePort *rabbitmqResources_Master) oshandler.Credentials {
	var master_res rabbitmqResources_Master
	err := loadRabbitmqResources_Master(myServiceInfo.Url, myServiceInfo.User, myServiceInfo.Password, myServiceInfo.Volumes, &master_res)
	if err != nil {
		logger.Error("loadRabbitmqResources_Master error",err)
		return oshandler.Credentials{}
	}

	mq_port := oshandler.GetServicePortByName(&master_res.service, "mq")
	if mq_port == nil {
		return oshandler.Credentials{}
	}

	svchost := fmt.Sprintf("%s.%s.%s", master_res.service.Name, myServiceInfo.Database, oshandler.ServiceDomainSuffix(false))
	svcport := strconv.Itoa(mq_port.Port)

	ndhost := oshandler.RandomNodeAddress()
	var ndport string = ""
	if nodePort != nil && len(nodePort.serviceNodePort.Spec.Ports) > 0 {
		ndport = strconv.Itoa(nodePort.serviceNodePort.Spec.Ports[0].NodePort)
	}

	return oshandler.Credentials{
		Uri:      fmt.Sprintf("amqp://%s:%s@%s:%s", myServiceInfo.User, myServiceInfo.Password, svchost, svcport),
		Hostname: ndhost,
		Port:     ndport,
		Username: myServiceInfo.User,
		Password: myServiceInfo.Password,
		Vhost:    svchost,
	}
}

func (handler *Rabbitmq_Handler) DoBind(myServiceInfo *oshandler.ServiceInfo, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, oshandler.Credentials, error) {
	// todo: handle errors

	master_res, err := getRabbitmqResources_Master(
		myServiceInfo.Url,
		myServiceInfo.Database,
		myServiceInfo.User,
		myServiceInfo.Password,
		myServiceInfo.Volumes,
	)
	if err != nil {
		return brokerapi.Binding{}, oshandler.Credentials{}, err
	}

	mq_port := oshandler.GetServicePortByName(&master_res.service, "mq")
	if mq_port == nil {
		return brokerapi.Binding{}, oshandler.Credentials{}, errors.New("mq port not found")
	}

	host := fmt.Sprintf("%s.%s.%s", master_res.service.Name, myServiceInfo.Database, oshandler.ServiceDomainSuffix(false))
	port := strconv.Itoa(mq_port.Port)


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

func loadRabbitmqResources_Master(instanceID, rabbitmqUser, rabbitmqPassword string, volumes []oshandler.Volume, res *rabbitmqResources_Master) error {
	if RabbitmqTemplateData_Master == nil {
		f, err := os.Open("rabbitmq-pvc.yaml")
		if err != nil {
			return err
		}
		RabbitmqTemplateData_Master, err = ioutil.ReadAll(f)
		if err != nil {
			return err
		}
		rabbitmq_image := oshandler.RabbitmqImage()
		rabbitmq_image = strings.TrimSpace(rabbitmq_image)
		if len(rabbitmq_image) > 0 {
			RabbitmqTemplateData_Master = bytes.Replace(
				RabbitmqTemplateData_Master,
				[]byte("http://rabbitmq-image-place-holder/rabbitmq-openshift-orchestration"),
				[]byte(rabbitmq_image),
				-1)
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
	peerPvcName0 := peerPvcName0(volumes)

	yamlTemplates := RabbitmqTemplateData_Master

	yamlTemplates = bytes.Replace(yamlTemplates, []byte("instanceid"), []byte(instanceID), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("user*****"), []byte(rabbitmqUser), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("pass*****"), []byte(rabbitmqPassword), -1)

	yamlTemplates = bytes.Replace(yamlTemplates, []byte("pvcname*****node"), []byte(peerPvcName0), -1)


	decoder := oshandler.NewYamlDecoder(yamlTemplates)
	decoder.
		Decode(&res.rc).
		Decode(&res.routeAdmin).
		Decode(&res.service).
		Decode(&res.serviceNodePort)

	return decoder.Err
}

type rabbitmqResources_Master struct {
	rc         kapi.ReplicationController
	routeAdmin routeapi.Route
	service         kapi.Service
	serviceNodePort kapi.Service
}

func createRabbitmqResources_Master(instanceId, serviceBrokerNamespace, rabbitmqUser, rabbitmqPassword string, volumes []oshandler.Volume) (*rabbitmqResources_Master, error) {
	var input rabbitmqResources_Master
	err := loadRabbitmqResources_Master(instanceId, rabbitmqUser, rabbitmqPassword, volumes, &input)
	if err != nil {
		logger.Error("loadRabbitmqResources_Master error",err)
		return nil, err
	}

	var output rabbitmqResources_Master

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	// here, not use job.post
	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		KPost(prefix+"/replicationcontrollers", &input.rc, &output.rc).
		OPost(prefix+"/routes", &input.routeAdmin, &output.routeAdmin).
		KPost(prefix+"/services", &input.service, &output.service)

	if osr.Err != nil {
		logger.Error("createRabbitmqResources_Master", osr.Err)
	}

	return &output, osr.Err
}

func createRabbitmqResources_NodePort(input *rabbitmqResources_Master, serviceBrokerNamespace string) (*rabbitmqResources_Master, error) {
	var output rabbitmqResources_Master

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	// here, not use job.post
	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.KPost(prefix+"/services", &input.serviceNodePort, &output.serviceNodePort)

	if osr.Err != nil {
		logger.Error("createRabbitmqResources_NodePort", osr.Err)
	}

	return &output, osr.Err
}

func getRabbitmqResources_Master(instanceId, serviceBrokerNamespace, rabbitmqUser, rabbitmqPassword string, volumes []oshandler.Volume) (*rabbitmqResources_Master, error) {
	var output rabbitmqResources_Master

	var input rabbitmqResources_Master
	err := loadRabbitmqResources_Master(instanceId, rabbitmqUser, rabbitmqPassword, volumes, &input)
	if err != nil {
		logger.Error("loadRabbitmqResources_Master error",err)
		return &output, err
	}

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		KGet(prefix+"/replicationcontrollers/"+input.rc.Name, &output.rc).
		OGet(prefix+"/routes/"+input.routeAdmin.Name, &output.routeAdmin).
		KGet(prefix+"/services/"+input.service.Name, &output.service).
		KGet(prefix+"/services/"+input.serviceNodePort.Name, &output.serviceNodePort)

	if osr.Err != nil {
		logger.Error("getRabbitmqResources_Master", osr.Err)
	}

	return &output, osr.Err
}

func destroyRabbitmqResources_Master(masterRes *rabbitmqResources_Master, serviceBrokerNamespace string) {
	// todo: add to retry queue on fail

	go func() { kdel_rc(serviceBrokerNamespace, &masterRes.rc) }()
	go func() { odel(serviceBrokerNamespace, "routes", masterRes.routeAdmin.Name) }()
	go func() { kdel(serviceBrokerNamespace, "services", masterRes.service.Name) }()
	go func() { kdel(serviceBrokerNamespace, "services", masterRes.serviceNodePort.Name) }()
}

//===============================================================
//
//===============================================================


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
			nrunnings++
		}
	}

	return nrunnings, nil
}