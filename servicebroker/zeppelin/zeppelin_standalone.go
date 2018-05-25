package zeppelin

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	oshandler "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/handler"
	routeapi "github.com/openshift/origin/route/api/v1"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-golang/lager"
	"io/ioutil"
	kapi "k8s.io/kubernetes/pkg/api/v1"
	"net/http"
	"os"
	"strconv"
	"strings"
)

//==============================================================
//初始化Log
//==============================================================

const ZeppelinServcieBrokerName_Standalone = "Zeppelin_standalone"

const (
	// API parameters passed from clients

	Key_ZeppelinMemory = oshandler.Memory // "memory", don't change
	Key_ZeppelinCPU    = oshandler.CPU    //"cpu",don't change

	DefaultZeppelinMemory         = 2000
	DefaultZeppelinCPU    float64 = 1.0
)

func init() {
	oshandler.Register(ZeppelinServcieBrokerName_Standalone, &Zeppelin_freeHandler{})

	logger = lager.NewLogger(ZeppelinServcieBrokerName_Standalone)
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))
}

var logger lager.Logger

var httpClient = &http.Client{
	Transport: &http.Transport{},
	Timeout:   0,
}

//==============================================================
//
//==============================================================

type Zeppelin_freeHandler struct{}

func (handler *Zeppelin_freeHandler) DoProvision(etcdSaveResult chan error, instanceID string, details brokerapi.ProvisionDetails, planInfo oshandler.PlanInfo, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, oshandler.ServiceInfo, error) {
	return newZeppelinHandler().DoProvision(etcdSaveResult, instanceID, details, planInfo, asyncAllowed)
}

func (handler *Zeppelin_freeHandler) DoLastOperation(myServiceInfo *oshandler.ServiceInfo) (brokerapi.LastOperation, error) {
	return newZeppelinHandler().DoLastOperation(myServiceInfo)
}

func (handler *Zeppelin_freeHandler) DoUpdate(myServiceInfo *oshandler.ServiceInfo, planInfo oshandler.PlanInfo, callbackSaveNewInfo func(*oshandler.ServiceInfo) error, asyncAllowed bool) error {
	return newZeppelinHandler().DoUpdate(myServiceInfo, planInfo, callbackSaveNewInfo, asyncAllowed)
}

func (handler *Zeppelin_freeHandler) DoDeprovision(myServiceInfo *oshandler.ServiceInfo, asyncAllowed bool) (brokerapi.IsAsync, error) {
	return newZeppelinHandler().DoDeprovision(myServiceInfo, asyncAllowed)
}

func (handler *Zeppelin_freeHandler) DoBind(myServiceInfo *oshandler.ServiceInfo, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, oshandler.Credentials, error) {
	return newZeppelinHandler().DoBind(myServiceInfo, bindingID, details)
}

func (handler *Zeppelin_freeHandler) DoUnbind(myServiceInfo *oshandler.ServiceInfo, mycredentials *oshandler.Credentials) error {
	return newZeppelinHandler().DoUnbind(myServiceInfo, mycredentials)
}

//==============================================================
//
//==============================================================

type Zeppelin_Handler struct {
}

func newZeppelinHandler() *Zeppelin_Handler {
	return &Zeppelin_Handler{}
}

func retrieveMemoryFromPlanInfo(planInfo oshandler.PlanInfo, defaultMemory int) (nodeMemory int, err error) {
	memorySettings, ok := planInfo.ParameterSettings[Key_ZeppelinMemory]
	if !ok {
		err = errors.New(Key_ZeppelinMemory + " settings not found")
		nodeMemory = defaultMemory
		return
	}

	fMemory, err := oshandler.ParseFloat64(planInfo.MoreParameters[Key_ZeppelinMemory])
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
	cpuSettings, ok := planInfo.ParameterSettings[Key_ZeppelinCPU]
	if !ok {
		err = errors.New(Key_ZeppelinCPU + " settings not found")
		nodeCPU = defaultCPU
		return
	}

	fCPU, err := oshandler.ParseFloat64(planInfo.MoreParameters[Key_ZeppelinCPU])
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

func (handler *Zeppelin_Handler) DoProvision(etcdSaveResult chan error, instanceID string, details brokerapi.ProvisionDetails, planInfo oshandler.PlanInfo, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, oshandler.ServiceInfo, error) {
	//初始化到openshift的链接

	serviceSpec := brokerapi.ProvisionedServiceSpec{IsAsync: asyncAllowed}
	serviceInfo := oshandler.ServiceInfo{}

	//获取memory参数，获取不到就设置为默认的500
	zeppelinMemory, err := retrieveMemoryFromPlanInfo(planInfo, DefaultZeppelinMemory) // Mi
	if err != nil {
		println("retrieveMemoryFromPlanInfo error: ", err.Error())
	}

	zeppelinCPU, err := retrieveCPUFromPlanInfo(planInfo, DefaultZeppelinCPU)
	if err != nil {
		println("retrieveCPUFromPlanInfo error: ", err.Error())
	}

	logger.Info("Zeppelin Limit parameters...", map[string]interface{}{"cpu": strconv.FormatFloat(zeppelinCPU, 'f', 1, 64), "memory": strconv.Itoa(zeppelinMemory) + "Mi"})

	serviceSpec.IsAsync = true

	instanceIdInTempalte := strings.ToLower(oshandler.NewThirteenLengthID())
	serviceBrokerNamespace := oshandler.OC().Namespace()

	logger.Info("Zeppelin Creating ...", map[string]interface{}{"instanceIdInTempalte": instanceIdInTempalte, "serviceBrokerNamespace": serviceBrokerNamespace})

	serviceInfo.Url = instanceIdInTempalte
	serviceInfo.Database = serviceBrokerNamespace // may be not needed
	serviceInfo.User = ""
	serviceInfo.Password = ""
	serviceInfo.Miscs = map[string]string{
		Key_ZeppelinMemory: strconv.Itoa(zeppelinMemory),
		Key_ZeppelinCPU:    strconv.FormatFloat(zeppelinCPU, 'f', 1, 64),
	}

	go func() {
		err := <-etcdSaveResult
		if err != nil {
			return
		}

		// master
		output, err := createZeppelinResources_Master(instanceIdInTempalte, serviceBrokerNamespace, serviceInfo.User, serviceInfo.Password, zeppelinMemory, zeppelinCPU)

		if err != nil {
			destroyZeppelinResources_Master(output, serviceBrokerNamespace)

			return
		}

	}()

	var input zeppelinResources_Master
	err = loadZeppelinResources_Master(instanceIdInTempalte, serviceInfo.User, serviceInfo.Password, zeppelinMemory, zeppelinCPU, &input)
	if err != nil {
		logger.Error("loadZeppelinResources_Master error", err)
		return serviceSpec, serviceInfo, err
	}

	serviceSpec.DashboardURL = fmt.Sprintf("http://%s", input.route.Spec.Host)

	//>>>
	serviceSpec.Credentials = getCredentialsOnPrivision(&serviceInfo)
	//<<<

	return serviceSpec, serviceInfo, nil
}

func (handler *Zeppelin_Handler) DoLastOperation(myServiceInfo *oshandler.ServiceInfo) (brokerapi.LastOperation, error) {

	// assume in provisioning

	// the job may be finished or interrupted or running in another instance.

	memory, _ := strconv.Atoi(myServiceInfo.Miscs[Key_ZeppelinMemory])
	cpu, _ := strconv.ParseFloat(myServiceInfo.Miscs[Key_ZeppelinCPU], 64)
	master_res, _ := getZeppelinResources_Master(myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.User, myServiceInfo.Password, memory, cpu)

	ok := func(rc *kapi.ReplicationController) bool {
		if rc == nil || rc.Name == "" || rc.Spec.Replicas == nil || rc.Status.Replicas < *rc.Spec.Replicas {
			return false
		}
		n, _ := statRunningPodsByLabels(myServiceInfo.Database, rc.Labels)
		return n >= *rc.Spec.Replicas
	}

	// todo: check if http get dashboard request is ok

	if ok(&master_res.rc) {
		req, _ := http.NewRequest("GET", "http://"+master_res.route.Spec.Host, nil)
		request, err := httpClient.Do(req)
		defer request.Body.Close()
		if err == nil {
			if request.StatusCode >= 200 && request.StatusCode < 400 {
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

func (handler *Zeppelin_Handler) DoUpdate(myServiceInfo *oshandler.ServiceInfo, planInfo oshandler.PlanInfo, callbackSaveNewInfo func(*oshandler.ServiceInfo) error, asyncAllowed bool) error {
	return nil
}

func (handler *Zeppelin_Handler) DoDeprovision(myServiceInfo *oshandler.ServiceInfo, asyncAllowed bool) (brokerapi.IsAsync, error) {
	// ...

	println("to destroy resources")

	memory, _ := strconv.Atoi(myServiceInfo.Miscs[Key_ZeppelinMemory])
	cpu, _ := strconv.ParseFloat(myServiceInfo.Miscs[Key_ZeppelinCPU], 64)

	master_res, _ := getZeppelinResources_Master(myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.User, myServiceInfo.Password, memory, cpu)
	destroyZeppelinResources_Master(master_res, myServiceInfo.Database)

	return brokerapi.IsAsync(false), nil
}

// please note: the bsi may be still not fully initialized when calling the function.
func getCredentialsOnPrivision(myServiceInfo *oshandler.ServiceInfo) oshandler.Credentials {
	var master_res zeppelinResources_Master
	memory, _ := strconv.Atoi(myServiceInfo.Miscs[Key_ZeppelinMemory])
	cpu, _ := strconv.ParseFloat(myServiceInfo.Miscs[Key_ZeppelinCPU], 64)
	err := loadZeppelinResources_Master(myServiceInfo.Url, myServiceInfo.User, myServiceInfo.Password, memory, cpu, &master_res)
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
		Username: myServiceInfo.User,
		Password: myServiceInfo.Password,
	}
}

func (handler *Zeppelin_Handler) DoBind(myServiceInfo *oshandler.ServiceInfo, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, oshandler.Credentials, error) {
	// todo: handle errors

	memory, _ := strconv.Atoi(myServiceInfo.Miscs[Key_ZeppelinMemory])
	cpu, _ := strconv.ParseFloat(myServiceInfo.Miscs[Key_ZeppelinCPU], 64)
	master_res, err := getZeppelinResources_Master(myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.User, myServiceInfo.Password, memory, cpu)
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
		Username: myServiceInfo.User,
		Password: myServiceInfo.Password,
	}

	myBinding := brokerapi.Binding{Credentials: mycredentials}

	return myBinding, mycredentials, nil
}

func (handler *Zeppelin_Handler) DoUnbind(myServiceInfo *oshandler.ServiceInfo, mycredentials *oshandler.Credentials) error {
	// do nothing

	return nil
}

//=======================================================================
//
//=======================================================================

var ZeppelinTemplateData_Master []byte = nil

func loadZeppelinResources_Master(instanceID, zeppelinUser, zeppelinPassword string, zeppelinMemory int, zeppelinCPU float64, res *zeppelinResources_Master) error {
	if ZeppelinTemplateData_Master == nil {
		f, err := os.Open("zeppelin.yaml")
		if err != nil {
			return err
		}
		ZeppelinTemplateData_Master, err = ioutil.ReadAll(f)
		if err != nil {
			return err
		}
		endpoint_postfix := oshandler.EndPointSuffix()
		endpoint_postfix = strings.TrimSpace(endpoint_postfix)
		if len(endpoint_postfix) > 0 {
			ZeppelinTemplateData_Master = bytes.Replace(
				ZeppelinTemplateData_Master,
				[]byte("endpoint-postfix-place-holder"),
				[]byte(endpoint_postfix),
				-1)
		}
		zeppelin_image := oshandler.ZepplinImage()
		zeppelin_image = strings.TrimSpace(zeppelin_image)
		if len(zeppelin_image) > 0 {
			ZeppelinTemplateData_Master = bytes.Replace(
				ZeppelinTemplateData_Master,
				[]byte("http://zeppelin-image-place-holder/zeppelin-openshift-orchestration"),
				[]byte(zeppelin_image),
				-1)
		}
	}

	// ...

	yamlTemplates := ZeppelinTemplateData_Master

	yamlTemplates = bytes.Replace(yamlTemplates, []byte("instanceid"), []byte(instanceID), -1)

	yamlTemplates = bytes.Replace(yamlTemplates, []byte("memory*****"), []byte(strconv.Itoa(zeppelinMemory)), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("cpu*****"), []byte(strconv.FormatFloat(zeppelinCPU, 'f', 1, 64)), -1)

	decoder := oshandler.NewYamlDecoder(yamlTemplates)
	decoder.
		Decode(&res.rc).
		Decode(&res.route).
		Decode(&res.service)

	return decoder.Err
}

type zeppelinResources_Master struct {
	rc      kapi.ReplicationController
	route   routeapi.Route
	service kapi.Service
}

func createZeppelinResources_Master(instanceId, serviceBrokerNamespace, zeppelinUser, zeppelinPassword string, zeppelinMemory int, zeppelinCPU float64) (*zeppelinResources_Master, error) {
	var input zeppelinResources_Master
	err := loadZeppelinResources_Master(instanceId, zeppelinUser, zeppelinPassword, zeppelinMemory, zeppelinCPU, &input)
	if err != nil {
		return nil, err
	}

	var output zeppelinResources_Master

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	// here, not use job.post
	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		KPost(prefix+"/replicationcontrollers", &input.rc, &output.rc).
		OPost(prefix+"/routes", &input.route, &output.route).
		KPost(prefix+"/services", &input.service, &output.service)

	if osr.Err != nil {
		logger.Error("createZeppelinResources_Master", osr.Err)
	}

	return &output, osr.Err
}

func getZeppelinResources_Master(instanceId, serviceBrokerNamespace, zeppelinUser, zeppelinPassword string, zeppelinMemory int, zeppelinCPU float64) (*zeppelinResources_Master, error) {
	var output zeppelinResources_Master

	var input zeppelinResources_Master
	err := loadZeppelinResources_Master(instanceId, zeppelinUser, zeppelinPassword, zeppelinMemory, zeppelinCPU, &input)
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
		logger.Error("getZeppelinResources_Master", osr.Err)
	}

	return &output, osr.Err
}

func destroyZeppelinResources_Master(masterRes *zeppelinResources_Master, serviceBrokerNamespace string) {
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
		logger.Error("Modify Zeppelin rc", osr.Err)
		return
	}

	// start watching rc status

	statuses, cancel, err := oshandler.OC().KWatch(uri)
	if err != nil {
		logger.Error("Start Watching Zeppelin rc", err)
		return
	}

	go func() {
		for {
			status, _ := <-statuses

			if status.Err != nil {
				logger.Error("Watch Zeppelin rc error", status.Err)
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
