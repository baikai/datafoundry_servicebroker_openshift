package redissingle_pvc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	oshandler "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/handler"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-golang/lager"
	"io/ioutil"
	kapi "k8s.io/kubernetes/pkg/api/v1"
	"os"
	"strconv"
	"strings"
	"time"
)

//==============================================================
//初始化Log
//==============================================================

const RedisSingleServcieBrokerName_Standalone = "Redis_volumes_single"

func init() {
	oshandler.Register(RedisSingleServcieBrokerName_Standalone, &RedisSingle_freeHandler{})

	logger = lager.NewLogger(RedisSingleServcieBrokerName_Standalone)
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))
}

var logger lager.Logger

//==============================================================
//
//==============================================================

type RedisSingle_freeHandler struct{}

func (handler *RedisSingle_freeHandler) DoProvision(etcdSaveResult chan error, instanceID string, details brokerapi.ProvisionDetails, planInfo oshandler.PlanInfo, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, oshandler.ServiceInfo, error) {
	return newRedisSingleHandler().DoProvision(etcdSaveResult, instanceID, details, planInfo, asyncAllowed)
}

func (handler *RedisSingle_freeHandler) DoLastOperation(myServiceInfo *oshandler.ServiceInfo) (brokerapi.LastOperation, error) {
	return newRedisSingleHandler().DoLastOperation(myServiceInfo)
}

func (handler *RedisSingle_freeHandler) DoUpdate(myServiceInfo *oshandler.ServiceInfo, planInfo oshandler.PlanInfo, callbackSaveNewInfo func(*oshandler.ServiceInfo) error, asyncAllowed bool) error {
	return newRedisSingleHandler().DoUpdate(myServiceInfo, planInfo, callbackSaveNewInfo, asyncAllowed)
}

func (handler *RedisSingle_freeHandler) DoDeprovision(myServiceInfo *oshandler.ServiceInfo, asyncAllowed bool) (brokerapi.IsAsync, error) {
	return newRedisSingleHandler().DoDeprovision(myServiceInfo, asyncAllowed)
}

func (handler *RedisSingle_freeHandler) DoBind(myServiceInfo *oshandler.ServiceInfo, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, oshandler.Credentials, error) {
	return newRedisSingleHandler().DoBind(myServiceInfo, bindingID, details)
}

func (handler *RedisSingle_freeHandler) DoUnbind(myServiceInfo *oshandler.ServiceInfo, mycredentials *oshandler.Credentials) error {
	return newRedisSingleHandler().DoUnbind(myServiceInfo, mycredentials)
}

//==============================================================
//挂卷配置
//==============================================================

func volumeBaseName(instanceId string) string {
	return "rdscls-" + instanceId
}

func masterPvcName(volumes []oshandler.Volume) string {
	if len(volumes) > 0 {
		return volumes[0].Volume_name
	}
	return ""
}

//==============================================================
//
//==============================================================

type RedisSingle_Handler struct {
}

func newRedisSingleHandler() *RedisSingle_Handler {
	return &RedisSingle_Handler{}
}

func (handler *RedisSingle_Handler) DoProvision(etcdSaveResult chan error, instanceID string, details brokerapi.ProvisionDetails, planInfo oshandler.PlanInfo, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, oshandler.ServiceInfo, error) {
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
	}

	logger.Info("Redis Creating ...", map[string]interface{}{"instanceIdInTempalte": instanceIdInTempalte, "serviceBrokerNamespace": serviceBrokerNamespace})

	// ...

	serviceInfo.Url = instanceIdInTempalte
	serviceInfo.Database = serviceBrokerNamespace // may be not needed
	serviceInfo.Password = redisPassword

	serviceInfo.Volumes = volumes

	//>> may be not optimized
	var template redisResources_Master
	err := loadRedisSingleResources_Master(
		serviceInfo.Url,
		serviceInfo.Password,
		serviceInfo.Volumes,
		&template)
	if err != nil {
		return serviceSpec, oshandler.ServiceInfo{}, err
	}
	//<<

	nodePort, err := createRedisSingleResources_NodePort(
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
			logger.Error("redis single create volume", err)
			handler.DoDeprovision(&serviceInfo, true)
			return
		}

		println("createRedisSingleResources_Master ...")

		// create master res

		output, err := createRedisSingleResources_Master(
			serviceInfo.Url,
			serviceInfo.Database,
			serviceInfo.Password,
			serviceInfo.Volumes,
		)
		if err != nil {
			logger.Error("redis createRedisSingleResources_Master error", err)

			destroyRedisSingleResources_Master(output, serviceInfo.Database)
			oshandler.DeleteVolumns(serviceInfo.Database, volumes)

			return
		}
	}()

	// ...

	serviceSpec.DashboardURL = ""

	//>>>
	serviceSpec.Credentials = getCredentialsOnPrivision(&serviceInfo, nodePort)
	//<<<

	return serviceSpec, serviceInfo, nil
}

func (handler *RedisSingle_Handler) DoLastOperation(myServiceInfo *oshandler.ServiceInfo) (brokerapi.LastOperation, error) {

	volumeJob := oshandler.GetCreatePvcVolumnJob(volumeBaseName(myServiceInfo.Url))
	if volumeJob != nil {
		return brokerapi.LastOperation{
			State:       brokerapi.InProgress,
			Description: "in progress.",
		}, nil
	}

	master_res, err := getRedisSingleResources_Master(
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

	ok := func(rc *kapi.ReplicationController) bool {
		println("rc.Name =", rc.Name)
		if rc == nil || rc.Name == "" || rc.Spec.Replicas == nil || rc.Status.Replicas < *rc.Spec.Replicas {
			return false
		}
		n, _ := statRunningPodsByLabels(myServiceInfo.Database, rc.Labels)
		println("n =", n)
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

func (handler *RedisSingle_Handler) DoUpdate(myServiceInfo *oshandler.ServiceInfo, planInfo oshandler.PlanInfo, callbackSaveNewInfo func(*oshandler.ServiceInfo) error, asyncAllowed bool) error {
	return errors.New("not implemented")
}

func (handler *RedisSingle_Handler) DoDeprovision(myServiceInfo *oshandler.ServiceInfo, asyncAllowed bool) (brokerapi.IsAsync, error) {
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

		master_res, _ := getRedisSingleResources_Master(
			myServiceInfo.Url,
			myServiceInfo.Database,
			myServiceInfo.Password,
			myServiceInfo.Volumes,
		)
		destroyRedisSingleResources_Master(master_res, myServiceInfo.Database)

		// ...

		fmt.Println("to destroy volumes:", myServiceInfo.Volumes)

		oshandler.DeleteVolumns(myServiceInfo.Database, myServiceInfo.Volumes)
	}()

	return brokerapi.IsAsync(false), nil
}

// please note: the bsi may be still not fully initialized when calling the function.
func getCredentialsOnPrivision(myServiceInfo *oshandler.ServiceInfo, nodePort *redisResources_Master) oshandler.Credentials {

	ndhost := oshandler.RandomNodeAddress()
	var svchost, svcport, ndport string
	if nodePort != nil && len(nodePort.serviceNodePort.Spec.Ports) > 0 {
		port := &nodePort.serviceNodePort.Spec.Ports[0]
		ndport = strconv.Itoa(port.NodePort)
		svchost = fmt.Sprintf("%s.%s.%s", nodePort.serviceNodePort.Name, myServiceInfo.Database, oshandler.ServiceDomainSuffix(false))
		svcport = strconv.Itoa(port.Port)
	}

	return oshandler.Credentials{
		Uri:      fmt.Sprintf("internal address: %s:%s", svchost, svcport),
		Hostname: ndhost,
		Port:     ndport,
		Password: myServiceInfo.Password,
	}
}

func (handler *RedisSingle_Handler) DoBind(myServiceInfo *oshandler.ServiceInfo, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, oshandler.Credentials, error) {
	// todo: handle errors

	// master_res may has been shutdown normally.

	master_res, err := getRedisSingleResources_Master(
		myServiceInfo.Url,
		myServiceInfo.Database,
		myServiceInfo.Password,
		myServiceInfo.Volumes,
	)
	if err != nil {
		return brokerapi.Binding{}, oshandler.Credentials{}, err
	}

	mycredentials := getCredentialsOnPrivision(myServiceInfo, master_res)

	myBinding := brokerapi.Binding{Credentials: mycredentials}

	return myBinding, mycredentials, nil
}

func (handler *RedisSingle_Handler) DoUnbind(myServiceInfo *oshandler.ServiceInfo, mycredentials *oshandler.Credentials) error {
	// do nothing

	return nil
}

//=======================================================================
//
//=======================================================================

var RedisSingleTemplateData_Master []byte = nil

func loadRedisSingleResources_Master(instanceID, redisPassword string, volumes []oshandler.Volume, res *redisResources_Master) error {
	if RedisSingleTemplateData_Master == nil {

		f, err := os.Open("redis-single-pvc-master.yaml")
		if err != nil {
			return err
		}
		RedisSingleTemplateData_Master, err = ioutil.ReadAll(f)
		if err != nil {
			return err
		}
		redis_image := oshandler.Redis32Image()
		redis_image = strings.TrimSpace(redis_image)
		if len(redis_image) > 0 {
			RedisSingleTemplateData_Master = bytes.Replace(
				RedisSingleTemplateData_Master,
				[]byte("http://redis-image-place-holder/redis-openshift-orchestration"),
				[]byte(redis_image),
				-1)
		}
	}

	// ...

	masterPvcName := masterPvcName(volumes)

	yamlTemplates := RedisSingleTemplateData_Master

	yamlTemplates = bytes.Replace(yamlTemplates, []byte("instanceid"), []byte(instanceID), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("pass*****"), []byte(redisPassword), -1)

	yamlTemplates = bytes.Replace(yamlTemplates, []byte("pvcname*****master"), []byte(masterPvcName), -1)

	decoder := oshandler.NewYamlDecoder(yamlTemplates)
	decoder.
		Decode(&res.serviceNodePort).
		Decode(&res.rc)

	return decoder.Err
}

type redisResources_Master struct {
	serviceNodePort kapi.Service
	rc              kapi.ReplicationController
}

func createRedisSingleResources_Master(instanceId, serviceBrokerNamespace, redisPassword string, volumes []oshandler.Volume) (*redisResources_Master, error) {
	var input redisResources_Master
	err := loadRedisSingleResources_Master(instanceId, redisPassword, volumes, &input)
	if err != nil {
		return nil, err
	}

	var output redisResources_Master

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	// here, not use job.post
	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		KPost(prefix+"/replicationcontrollers", &input.rc, &output.rc)

	if osr.Err != nil {
		logger.Error("createRedisSingleResources_Master", osr.Err)
	}

	return &output, osr.Err
}

func createRedisSingleResources_NodePort(input *redisResources_Master, serviceBrokerNamespace string) (*redisResources_Master, error) {
	var output redisResources_Master

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	// here, not use job.post
	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.KPost(prefix+"/services", &input.serviceNodePort, &output.serviceNodePort)

	if osr.Err != nil {
		logger.Error("createRedisSingleResources_NodePort", osr.Err)
	}

	return &output, osr.Err
}

func getRedisSingleResources_Master(instanceId, serviceBrokerNamespace, redisPassword string, volumes []oshandler.Volume) (*redisResources_Master, error) {
	var output redisResources_Master

	var input redisResources_Master
	err := loadRedisSingleResources_Master(instanceId, redisPassword, volumes, &input)
	if err != nil {
		return &output, err
	}

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		KGet(prefix+"/services/"+input.serviceNodePort.Name, &output.serviceNodePort).
		KGet(prefix+"/replicationcontrollers/"+input.rc.Name, &output.rc)

	if osr.Err != nil {
		logger.Error("getRedisSingleResources_Master", osr.Err)
	}

	return &output, osr.Err
}

func destroyRedisSingleResources_Master(masterRes *redisResources_Master, serviceBrokerNamespace string) {
	// todo: add to retry queue on fail

	go func() { kdel(serviceBrokerNamespace, "services", masterRes.serviceNodePort.Name) }()
	go func() { kdel_rc(serviceBrokerNamespace, &masterRes.rc) }()
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
				logger.Error("Watch  Redis rc error", status.Err)
				close(cancel)
				return
			} else {
				//logger.Debug("watch redis HA rc, status.Info: " + string(status.Info))
			}

			var wrcs watchReplicationControllerStatus
			if err := json.Unmarshal(status.Info, &wrcs); err != nil {
				logger.Error("Parse Master Redis rc status", err)
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
