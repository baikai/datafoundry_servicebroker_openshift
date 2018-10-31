package dataiku

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
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

//==============================================================
//
//==============================================================

const DataikuServcieBrokerName_Volume_Standalone = "Dataiku_volumes_standalone"

const (
	// API parameters passed from clients

	Key_DataikuMemory = oshandler.Memory // "memory", don't change
	Key_DataikuCPU    = oshandler.CPU    //"cpu",don't change

	DefaultDataikuMemory         = 2000
	DefaultDataikuCPU    float64 = 1.0
)

func init() {
	oshandler.Register(DataikuServcieBrokerName_Volume_Standalone, &Dataiku_freeHandler{})

	//logger = lager.NewLogger(DataikuServcieBrokerName_Volume_Standalone)
	//logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))
}

//var logger lager.Logger

func volumeBaseName(instanceId string) string {
	return "dataiku-" + instanceId
}

func peerPvcName0(volumes []oshandler.Volume) string {
	if len(volumes) > 0 {
		return volumes[0].Volume_name
	}
	return ""
}

var httpClient = &http.Client{
	Transport: &http.Transport{},
	Timeout:   0,
}

//==============================================================
//
//==============================================================

type Dataiku_freeHandler struct{}

func (handler *Dataiku_freeHandler) DoProvision(etcdSaveResult chan error, instanceID string, details brokerapi.ProvisionDetails, planInfo oshandler.PlanInfo, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, oshandler.ServiceInfo, error) {
	return newDataikuHandler().DoProvision(etcdSaveResult, instanceID, details, planInfo, asyncAllowed)
}

func (handler *Dataiku_freeHandler) DoLastOperation(myServiceInfo *oshandler.ServiceInfo) (brokerapi.LastOperation, error) {
	return newDataikuHandler().DoLastOperation(myServiceInfo)
}

func (handler *Dataiku_freeHandler) DoUpdate(myServiceInfo *oshandler.ServiceInfo, planInfo oshandler.PlanInfo, callbackSaveNewInfo func(*oshandler.ServiceInfo) error, asyncAllowed bool) error {
	return newDataikuHandler().DoUpdate(myServiceInfo, planInfo, callbackSaveNewInfo, asyncAllowed)
}

func (handler *Dataiku_freeHandler) DoDeprovision(myServiceInfo *oshandler.ServiceInfo, asyncAllowed bool) (brokerapi.IsAsync, error) {
	return newDataikuHandler().DoDeprovision(myServiceInfo, asyncAllowed)
}

func (handler *Dataiku_freeHandler) DoBind(myServiceInfo *oshandler.ServiceInfo, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, oshandler.Credentials, error) {
	return newDataikuHandler().DoBind(myServiceInfo, bindingID, details)
}

func (handler *Dataiku_freeHandler) DoUnbind(myServiceInfo *oshandler.ServiceInfo, mycredentials *oshandler.Credentials) error {
	return newDataikuHandler().DoUnbind(myServiceInfo, mycredentials)
}

//==============================================================
//
//==============================================================

type Dataiku_Handler struct {
}

func newDataikuHandler() *Dataiku_Handler {
	return &Dataiku_Handler{}
}

func retrieveMemoryFromPlanInfo(planInfo oshandler.PlanInfo, defaultMemory int) (nodeMemory int, err error) {
	memorySettings, ok := planInfo.ParameterSettings[Key_DataikuMemory]
	if !ok {
		err = errors.New(Key_DataikuMemory + " settings not found")
		nodeMemory = defaultMemory
		return
	}

	fMemory, err := oshandler.ParseFloat64(planInfo.MoreParameters[Key_DataikuMemory])
	if err != nil {
		nodeMemory = defaultMemory
		return
	}

	if float64(fMemory) > memorySettings.Max {
		err = fmt.Errorf("too large memory specfied: %f > %f", fMemory, memorySettings.Max)
	}

	if float64(fMemory) < memorySettings.Default {
		err = fmt.Errorf("too small memory specfied: %f < %f", fMemory, memorySettings.Default)
	}

	fMemory = memorySettings.Validate(fMemory)
	nodeMemory = int(1000 * fMemory)

	return
}

func retrieveCPUFromPlanInfo(planInfo oshandler.PlanInfo, defaultCPU float64) (nodeCPU float64, err error) {
	cpuSettings, ok := planInfo.ParameterSettings[Key_DataikuCPU]
	if !ok {
		err = errors.New(Key_DataikuCPU + " settings not found")
		nodeCPU = defaultCPU
		return
	}

	fCPU, err := oshandler.ParseFloat64(planInfo.MoreParameters[Key_DataikuCPU])
	if err != nil {
		nodeCPU = defaultCPU
		return
	}

	if fCPU > cpuSettings.Max {
		err = fmt.Errorf("too large cpu specfied: %f > %f", fCPU, cpuSettings.Max)
	}

	if fCPU < cpuSettings.Default {
		err = fmt.Errorf("too small cpu specfied: %f < %f", fCPU, cpuSettings.Default)
	}

	fCPU = cpuSettings.Validate(fCPU)
	nodeCPU = fCPU

	return
}

func (handler *Dataiku_Handler) DoProvision(etcdSaveResult chan error, instanceID string, details brokerapi.ProvisionDetails, planInfo oshandler.PlanInfo, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, oshandler.ServiceInfo, error) {

	//初始化到openshift的链接

	serviceSpec := brokerapi.ProvisionedServiceSpec{IsAsync: asyncAllowed}
	serviceInfo := oshandler.ServiceInfo{}

	//获取memory参数，获取不到就设置为默认的2000
	dataikuMemory, err := retrieveMemoryFromPlanInfo(planInfo, DefaultDataikuMemory) // Mi
	if err != nil {
		logger.Infoln("retrieveMemoryFromPlanInfo error: ", err.Error())
	}

	dataikuCPU, err := retrieveCPUFromPlanInfo(planInfo, DefaultDataikuCPU)
	if err != nil {
		logger.Infoln("retrieveCPUFromPlanInfo error: ", err.Error())
	}

	logger.Info("Dataiku Limit parameters...", map[string]interface{}{"cpu": strconv.FormatFloat(dataikuCPU, 'f', 1, 64), "memory": strconv.Itoa(dataikuMemory) + "Mi"})

	serviceSpec.IsAsync = true

	instanceIdInTempalte := strings.ToLower(oshandler.NewThirteenLengthID())
	serviceBrokerNamespace := oshandler.OC().Namespace()
	dataikuUser := ""     // oshandler.NewElevenLengthID()
	dataikuPassword := "" // oshandler.GenGUID()

	volumeBaseName := volumeBaseName(instanceIdInTempalte)
	volumes := []oshandler.Volume{
		// one volume
		{
			Volume_size: planInfo.Volume_size,
			Volume_name: volumeBaseName,
		},
	}

	logger.Info("Dataiku Creating ...", map[string]interface{}{"instanceIdInTempalte": instanceIdInTempalte, "serviceBrokerNamespace": serviceBrokerNamespace})

	serviceInfo.Url = instanceIdInTempalte
	serviceInfo.Database = serviceBrokerNamespace // may be not needed
	serviceInfo.Volumes = volumes
	serviceInfo.User = dataikuUser
	serviceInfo.Password = dataikuPassword
	serviceInfo.Miscs = map[string]string{
		Key_DataikuMemory: strconv.Itoa(dataikuMemory),
		Key_DataikuCPU:    strconv.FormatFloat(dataikuCPU, 'f', 1, 64),
	}

	go func() {
		err := <-etcdSaveResult
		if err != nil {
			return
		}

		result := oshandler.StartCreatePvcVolumnJob(
			volumeBaseName,
			serviceInfo.Database,
			serviceInfo.Volumes,
		)
		err = <-result
		if err != nil {
			logger.Error("dataiku create volume", err)
			oshandler.DeleteVolumns(serviceInfo.Database, serviceInfo.Volumes)
			return
		}

		// master dataiku
		output, err := createDataikuResources_Master(instanceIdInTempalte, serviceBrokerNamespace, dataikuUser, dataikuPassword, dataikuMemory, dataikuCPU, volumes)

		if err != nil {
			destroyDataikuResources_HA(output, serviceBrokerNamespace)
			oshandler.DeleteVolumns(serviceInfo.Database, serviceInfo.Volumes)

			return
		}

	}()

	var input dataikuResources_HA
	err = loadDataikuResources_HA(instanceIdInTempalte, dataikuUser, dataikuPassword, dataikuMemory, dataikuCPU, volumes, &input)
	if err != nil {
		return serviceSpec, serviceInfo, err
	}

	serviceSpec.DashboardURL = fmt.Sprintf("http://%s", input.route.Spec.Host)

	//>>>
	serviceSpec.Credentials = getCredentialsOnPrivision(&serviceInfo)
	//<<<

	return serviceSpec, serviceInfo, nil
}

func (handler *Dataiku_Handler) DoLastOperation(myServiceInfo *oshandler.ServiceInfo) (brokerapi.LastOperation, error) {

	// assume in provisioning

	// the job may be finished or interrupted or running in another instance.

	memory, _ := strconv.Atoi(myServiceInfo.Miscs[Key_DataikuMemory])
	cpu, _ := strconv.ParseFloat(myServiceInfo.Miscs[Key_DataikuCPU], 64)
	master_res, _ := getDataikuResources_HA(myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.User, myServiceInfo.Password, memory, cpu, myServiceInfo.Volumes)

	ok := func(rc *kapi.ReplicationController) bool {
		if rc == nil || rc.Name == "" || rc.Spec.Replicas == nil || rc.Status.Replicas < *rc.Spec.Replicas {
			return false
		}
		n, _ := statRunningPodsByLabels(myServiceInfo.Database, rc.Labels)
		return n >= *rc.Spec.Replicas
	}

	//logger.Infoln("num_ok_rcs = ", num_ok_rcs)

	// todo: check if http get dashboard request is ok

	if ok(&master_res.rc) {
		req, _ := http.NewRequest("GET", "http://"+master_res.route.Spec.Host, nil)
		response, err := httpClient.Do(req)
		defer response.Body.Close()
		if err == nil {
			if response.StatusCode >= 200 && response.StatusCode < 400 {
				return brokerapi.LastOperation{
					State:       brokerapi.Succeeded,
					Description: "Succeeded!",
				}, nil
			}
		}
	}
	return brokerapi.LastOperation{
		State:       brokerapi.InProgress,
		Description: "In progress.",
	}, nil

}

func (handler *Dataiku_Handler) DoUpdate(myServiceInfo *oshandler.ServiceInfo, planInfo oshandler.PlanInfo, callbackSaveNewInfo func(*oshandler.ServiceInfo) error, asyncAllowed bool) error {
	return nil
}

func (handler *Dataiku_Handler) DoDeprovision(myServiceInfo *oshandler.ServiceInfo, asyncAllowed bool) (brokerapi.IsAsync, error) {
	// ...


	/*
	logger.Infoln("to destroy resources")

	memory, _ := strconv.Atoi(myServiceInfo.Miscs[Key_DataikuMemory])
	cpu, _ := strconv.ParseFloat(myServiceInfo.Miscs[Key_DataikuCPU], 64)
	master_res, _ := getDataikuResources_Master(myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.User, myServiceInfo.Password, memory, cpu, myServiceInfo.Volumes)
	destroyDataikuResources_Master(master_res, myServiceInfo.Database)

	return brokerapi.IsAsync(false), nil
	*/

	memory, _ := strconv.Atoi(myServiceInfo.Miscs[Key_DataikuMemory])
	cpu, _ := strconv.ParseFloat(myServiceInfo.Miscs[Key_DataikuCPU], 64)
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

		logger.Debug("to destroy master resources")

		ha_res, _ := getDataikuResources_HA(myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.User, myServiceInfo.Password,memory, cpu, myServiceInfo.Volumes)

		destroyDataikuResources_HA(ha_res, myServiceInfo.Database)
		logger.Infoln("destroy master resources done")

		logger.Infoln("to destroy volumes:", myServiceInfo.Volumes)

		oshandler.DeleteVolumns(myServiceInfo.Database, myServiceInfo.Volumes)
		logger.Infoln("to destroy volumes done")

	}()

	return brokerapi.IsAsync(false), nil
}

func (handler *Dataiku_Handler) DoBind(myServiceInfo *oshandler.ServiceInfo, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, oshandler.Credentials, error) {
	// todo: handle errors

	memory, _ := strconv.Atoi(myServiceInfo.Miscs[Key_DataikuMemory])
	cpu, _ := strconv.ParseFloat(myServiceInfo.Miscs[Key_DataikuCPU], 64)

	master_res, err := getDataikuResources_HA(myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.User, myServiceInfo.Password, memory, cpu, myServiceInfo.Volumes)
	if err != nil {
		return brokerapi.Binding{}, oshandler.Credentials{}, err
	}

	web_port := oshandler.GetServicePortByName(&master_res.service, "web")
	if web_port == nil {
		return brokerapi.Binding{}, oshandler.Credentials{}, errors.New("web port not found")
	}

	host := fmt.Sprintf("%s.%s.%s", master_res.service.Name, myServiceInfo.Database, oshandler.ServiceDomainSuffix(false))
	port := strconv.Itoa(web_port.Port)
	//host := master_res.routeMQ.Spec.Host
	//port := "80"

	mycredentials := oshandler.Credentials{
		Uri:      "",
		Hostname: host,
		Port:     port,
		Username: myServiceInfo.User,
		Password: myServiceInfo.Password,
	}

	myBinding := brokerapi.Binding{Credentials: mycredentials}

	return myBinding, mycredentials, nil
}

func (handler *Dataiku_Handler) DoUnbind(myServiceInfo *oshandler.ServiceInfo, mycredentials *oshandler.Credentials) error {
	// do nothing

	return nil
}

//=======================================================================
//
//=======================================================================
type dataikuResources_HA struct {
	rc      kapi.ReplicationController
	route   routeapi.Route
	service kapi.Service
}

// please note: the bsi may be still not fully initialized when calling the function.
func getCredentialsOnPrivision(myServiceInfo *oshandler.ServiceInfo) oshandler.Credentials {
	var master_res dataikuResources_HA
	memory, _ := strconv.Atoi(myServiceInfo.Miscs[Key_DataikuMemory])
	cpu, _ := strconv.ParseFloat(myServiceInfo.Miscs[Key_DataikuCPU], 64)
	err := loadDataikuResources_HA(myServiceInfo.Url, myServiceInfo.User, myServiceInfo.Password, memory, cpu, myServiceInfo.Volumes, &master_res)
	if err != nil {
		logger.Error("getCredentialsOnPrivision loadDataikuResources_Master error ", err)
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
		Username: myServiceInfo.User,
		Password: myServiceInfo.Password,
	}
}

func createDataikuResources_Master(instanceId, serviceBrokerNamespace, dataikuUser, dataikuPassword string, dataikuMemory int, dataikuCPU float64, volumes []oshandler.Volume) (*dataikuResources_HA, error) {
	var input dataikuResources_HA
	err := loadDataikuResources_HA(instanceId, dataikuUser, dataikuPassword, dataikuMemory, dataikuCPU, volumes, &input)
	if err != nil {
		logger.Error("createDataikuResources_Master loadDataikuResources_Master error ", err)
		return nil, err
	}

	var output dataikuResources_HA

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	// here, not use job.post
	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		KPost(prefix+"/replicationcontrollers", &input.rc, &output.rc).
		OPost(prefix+"/routes", &input.route, &output.route).
		KPost(prefix+"/services", &input.service, &output.service)

	if osr.Err != nil {
		logger.Error("createDataikuResources_Master", osr.Err)
	}

	return &output, osr.Err
}

var DataikuTemplateData_Master []byte = nil

func loadDataikuResources_HA(instanceID, dataikuUser, dataikuPassword string, dataikuMemory int, dataikuCPU float64, volumes []oshandler.Volume, res *dataikuResources_HA) error {
	if DataikuTemplateData_Master == nil {
		f, err := os.Open("dataiku-pvc.yaml")
		if err != nil {
			logger.Error("loadDataikuResources_Master open yaml error ", err)
			return err
		}
		DataikuTemplateData_Master, err = ioutil.ReadAll(f)
		if err != nil {
			logger.Error("loadDataikuResources_Master ReadAll error ", err)
			return err
		}
		endpoint_postfix := oshandler.EndPointSuffix()
		endpoint_postfix = strings.TrimSpace(endpoint_postfix)
		if len(endpoint_postfix) > 0 {
			DataikuTemplateData_Master = bytes.Replace(
				DataikuTemplateData_Master,
				[]byte("endpoint-postfix-place-holder"),
				[]byte(endpoint_postfix),
				-1)
		}
		dataiku_image := oshandler.DataikuImage()
		dataiku_image = strings.TrimSpace(dataiku_image)
		if len(dataiku_image) > 0 {
			DataikuTemplateData_Master = bytes.Replace(
				DataikuTemplateData_Master,
				[]byte("http://dataiku-image-place-holder/dataiku-openshift-orchestration"),
				[]byte(dataiku_image),
				-1)
		}
	}

	// ...
	peerPvcName0 := peerPvcName0(volumes)

	yamlTemplates := DataikuTemplateData_Master

	yamlTemplates = bytes.Replace(yamlTemplates, []byte("instanceid"), []byte(instanceID), -1)

	yamlTemplates = bytes.Replace(yamlTemplates, []byte("memory*****"), []byte(strconv.Itoa(dataikuMemory)), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("cpu*****"), []byte(strconv.FormatFloat(dataikuCPU, 'f', 1, 64)), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("pvcname*****node"), []byte(peerPvcName0), -1)

	decoder := oshandler.NewYamlDecoder(yamlTemplates)
	decoder.
		Decode(&res.rc).
		Decode(&res.route).
		Decode(&res.service)

	return decoder.Err
}

func getDataikuResources_HA(instanceId, serviceBrokerNamespace, dataikuUser, dataikuPassword string, dataikuMemory int, dataikuCPU float64, volumes []oshandler.Volume) (*dataikuResources_HA, error) {
	var output dataikuResources_HA

	var input dataikuResources_HA
	err := loadDataikuResources_HA(instanceId, dataikuUser, dataikuPassword, dataikuMemory, dataikuCPU, volumes, &input)
	if err != nil {
		logger.Error("loadDataikuResources_Master error ", err)
		return &output, err
	}

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		KGet(prefix+"/replicationcontrollers/"+input.rc.Name, &output.rc).
		OGet(prefix+"/routes/"+input.route.Name, &output.route).
		KGet(prefix+"/services/"+input.service.Name, &output.service)

	if osr.Err != nil {
		logger.Error("getDataikuResources_Master", osr.Err)
	}

	return &output, osr.Err
}

func destroyDataikuResources_HA(masterRes *dataikuResources_HA, serviceBrokerNamespace string) {
	// todo: add to retry queue on fail

	//go func() { kdel_rc(serviceBrokerNamespace, &masterRes.rc) }()
	//go func() { odel(serviceBrokerNamespace, "routes", masterRes.route.Name) }()
	//go func() { kdel(serviceBrokerNamespace, "services", masterRes.service.Name) }()

	// avoid deleting behind deleting volumes.
	kdel_rc(serviceBrokerNamespace, &masterRes.rc)
	odel(serviceBrokerNamespace, "routes", masterRes.route.Name)
	kdel(serviceBrokerNamespace, "services", masterRes.service.Name)
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

type watchReplicationControllerStatus struct {
	// The type of watch update contained in the message
	Type string `json:"type"`
	// RC details
	Object kapi.ReplicationController `json:"object"`
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
		logger.Error("modify Dataiku rc", osr.Err)
		return
	}

	// start watching rc status

	statuses, cancel, err := oshandler.OC().KWatch(uri)
	if err != nil {
		logger.Error("Start Watching Dataiku rc", err)
		return
	}

	go func() {
		for {
			status, _ := <-statuses

			if status.Err != nil {
				logger.Error("Watch Dataiku rc error", status.Err)
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
