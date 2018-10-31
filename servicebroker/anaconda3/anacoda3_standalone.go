package anaconda3

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
	kresource "k8s.io/kubernetes/pkg/api/resource"
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

const AnacodaServcieBrokerName_Standalone = "Anaconda_standalone"

const (
	// API parameters passed from clients

	Key_AnacondaMemory = oshandler.Memory // "memory", don't change
	Key_AnacondaCPU    = oshandler.CPU    //"cpu",don't change

	DefaultAnacondaMemory         = 2000
	DefaultAnacondaCPU    float64 = 1.0
)

func init() {
	oshandler.Register(AnacodaServcieBrokerName_Standalone, &Anacoda_freeHandler{})

	//logger = lager.NewLogger(AnacodaServcieBrokerName_Standalone)
	//logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))
}

//var logger lager.Logger

var httpClient = &http.Client{
	Transport: &http.Transport{},
	Timeout:   0,
}

//==============================================================
//
//==============================================================

type Anacoda_freeHandler struct{}

func (handler *Anacoda_freeHandler) DoProvision(etcdSaveResult chan error, instanceID string, details brokerapi.ProvisionDetails, planInfo oshandler.PlanInfo, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, oshandler.ServiceInfo, error) {
	return newAnacodaHandler().DoProvision(etcdSaveResult, instanceID, details, planInfo, asyncAllowed)
}

func (handler *Anacoda_freeHandler) DoLastOperation(myServiceInfo *oshandler.ServiceInfo) (brokerapi.LastOperation, error) {
	return newAnacodaHandler().DoLastOperation(myServiceInfo)
}

func (handler *Anacoda_freeHandler) DoUpdate(myServiceInfo *oshandler.ServiceInfo, planInfo oshandler.PlanInfo, callbackSaveNewInfo func(*oshandler.ServiceInfo) error, asyncAllowed bool) error {
	return newAnacodaHandler().DoUpdate(myServiceInfo, planInfo, callbackSaveNewInfo, asyncAllowed)
}

func (handler *Anacoda_freeHandler) DoDeprovision(myServiceInfo *oshandler.ServiceInfo, asyncAllowed bool) (brokerapi.IsAsync, error) {
	return newAnacodaHandler().DoDeprovision(myServiceInfo, asyncAllowed)
}

func (handler *Anacoda_freeHandler) DoBind(myServiceInfo *oshandler.ServiceInfo, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, oshandler.Credentials, error) {
	return newAnacodaHandler().DoBind(myServiceInfo, bindingID, details)
}

func (handler *Anacoda_freeHandler) DoUnbind(myServiceInfo *oshandler.ServiceInfo, mycredentials *oshandler.Credentials) error {
	return newAnacodaHandler().DoUnbind(myServiceInfo, mycredentials)
}

//==============================================================
//
//==============================================================

type Anacoda_Handler struct {
}

func newAnacodaHandler() *Anacoda_Handler {
	return &Anacoda_Handler{}
}

func retrieveMemoryFromPlanInfo(planInfo oshandler.PlanInfo, defaultMemory int) (nodeMemory int, err error) {
	memorySettings, ok := planInfo.ParameterSettings[Key_AnacondaMemory]
	if !ok {
		err = errors.New(Key_AnacondaMemory + " settings not found")
		nodeMemory = defaultMemory
		return
	}

	fMemory, err := oshandler.ParseFloat64(planInfo.MoreParameters[Key_AnacondaMemory])
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
	cpuSettings, ok := planInfo.ParameterSettings[Key_AnacondaCPU]
	if !ok {
		err = errors.New(Key_AnacondaCPU + " settings not found")
		nodeCPU = defaultCPU
		return
	}

	fCPU, err := oshandler.ParseFloat64(planInfo.MoreParameters[Key_AnacondaCPU])
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

func (handler *Anacoda_Handler) DoProvision(etcdSaveResult chan error, instanceID string, details brokerapi.ProvisionDetails, planInfo oshandler.PlanInfo, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, oshandler.ServiceInfo, error) {
	//初始化到openshift的链接

	serviceSpec := brokerapi.ProvisionedServiceSpec{IsAsync: asyncAllowed}
	serviceInfo := oshandler.ServiceInfo{}

	//获取memory参数，获取不到就设置为默认的500
	anacondaMemory, err := retrieveMemoryFromPlanInfo(planInfo, DefaultAnacondaMemory) // Mi
	if err != nil {
		logger.Infoln("retrieveMemoryFromPlanInfo error: ", err.Error())
	}

	anacondaCPU, err := retrieveCPUFromPlanInfo(planInfo, DefaultAnacondaCPU)
	if err != nil {
		logger.Infoln("retrieveCPUFromPlanInfo error: ", err.Error())
	}

	logger.Info("Anaconda Limit parameters...", map[string]interface{}{"cpu": strconv.FormatFloat(anacondaCPU, 'f', 1, 64), "memory": strconv.Itoa(anacondaMemory) + "Mi"})

	serviceSpec.IsAsync = true

	instanceIdInTempalte := strings.ToLower(oshandler.NewThirteenLengthID())
	serviceBrokerNamespace := oshandler.OC().Namespace()

	logger.Info("Anaconda Creating ...", map[string]interface{}{"instanceIdInTempalte": instanceIdInTempalte, "serviceBrokerNamespace": serviceBrokerNamespace})


	//if asyncAllowed == false {
	//	return serviceSpec, serviceInfo, errors.New("Sync mode is not supported")
	//}
	serviceSpec.IsAsync = true

	//instanceIdInTempalte   := instanceID // todo: ok?
	//instanceIdInTempalte := strings.ToLower(oshandler.NewThirteenLengthID())
	//serviceBrokerNamespace := ServiceBrokerNamespace
	//serviceBrokerNamespace := oshandler.OC().Namespace()

	serviceInfo.Url = instanceIdInTempalte
	serviceInfo.Database = serviceBrokerNamespace // may be not needed
	serviceInfo.User = ""
	serviceInfo.Password = ""
	serviceInfo.Miscs = map[string]string{
		Key_AnacondaMemory: strconv.Itoa(anacondaMemory),
		Key_AnacondaCPU:    strconv.FormatFloat(anacondaCPU, 'f', 1, 64),
	}

	go func() {
		err := <-etcdSaveResult
		if err != nil {
			return
		}

		// master
		output, err := createAnacodaResources_Master(instanceIdInTempalte, serviceBrokerNamespace, serviceInfo.User, serviceInfo.Password,anacondaMemory,anacondaCPU)

		if err != nil {
			logger.Error("createAnacodaResources_Master error ", err)
			destroyAnacodaResources_Master(output, serviceBrokerNamespace)
			return
		}

	}()

	var input anacodaResources_Master
	err = loadAnacodaResources_Master(instanceIdInTempalte, serviceInfo.User, serviceInfo.Password,anacondaMemory,anacondaCPU,&input)
	if err != nil {
		logger.Error("loadAnacodaResources_Master error ", err)
		return serviceSpec, serviceInfo, err
	}

	serviceSpec.DashboardURL = fmt.Sprintf("http://%s", input.route.Spec.Host)

	//>>>
	serviceSpec.Credentials = getCredentialsOnPrivision(&serviceInfo)
	//<<<

	return serviceSpec, serviceInfo, nil
}

func (handler *Anacoda_Handler) DoLastOperation(myServiceInfo *oshandler.ServiceInfo) (brokerapi.LastOperation, error) {

	// assume in provisioning

	// the job may be finished or interrupted or running in another instance.

	memory, _ := strconv.Atoi(myServiceInfo.Miscs[Key_AnacondaMemory])
	cpu, _ := strconv.ParseFloat(myServiceInfo.Miscs[Key_AnacondaCPU], 64)
	master_res, _ := getAnacodaResources_Master(myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.User, myServiceInfo.Password,memory,cpu)

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

func (handler *Anacoda_Handler) DoUpdate(myServiceInfo *oshandler.ServiceInfo, planInfo oshandler.PlanInfo, callbackSaveNewInfo func(*oshandler.ServiceInfo) error, asyncAllowed bool) error {

	var oldMemory int
	var oldCPU float64

	{
		nMemory, err := oshandler.ParseInt64(myServiceInfo.Miscs[Key_AnacondaMemory])
		if err != nil {
			nMemory = DefaultAnacondaMemory
		}
		oldMemory = int(nMemory)

		nCPU, err := oshandler.ParseFloat64(myServiceInfo.Miscs[Key_AnacondaCPU])
		if err != nil {
			nCPU = DefaultAnacondaCPU
		}
		oldCPU = nCPU
	}

	newMemory, err := retrieveMemoryFromPlanInfo(planInfo, oldMemory) // Mi
	if err != nil {
		logger.Infoln("retrieveMemoryFromPlanInfo error: ", err.Error())
	}

	newCPU, err := retrieveCPUFromPlanInfo(planInfo, oldCPU)
	if err != nil {
		logger.Infoln("retrieveCPUFromPlanInfo error: ", err.Error())
	}

	logger.Info("Anaconda old parameters...", map[string]interface{}{"cpu": strconv.FormatFloat(oldCPU, 'f', 1, 64), "memory": strconv.Itoa(oldMemory) + "Mi"})

	logger.Info("Anaconda new parameters...", map[string]interface{}{"cpu": strconv.FormatFloat(newCPU, 'f', 1, 64), "memory": strconv.Itoa(newMemory) + "Mi"})

	if true {

		authInfoChanged := myServiceInfo.Miscs[Key_AnacondaMemory] != strconv.Itoa(newMemory) ||
			myServiceInfo.Miscs[Key_AnacondaCPU] != strconv.FormatFloat(newCPU, 'f', 1, 64)

		logger.Infoln("To update Anaconda instance.")

		go func() {
			if authInfoChanged {
				// ...
				err := updateAnacondaResources(myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.User, myServiceInfo.Password,
					newMemory, newCPU, authInfoChanged)
				if err != nil {
					logger.Infoln("Failed to update storm external instance (nimbus). Error:", err.Error())
					return
				}
			}

			logger.Infoln("Zeppelin instance is updated.")

			myServiceInfo.Miscs[Key_AnacondaMemory] = strconv.Itoa(newMemory)
			myServiceInfo.Miscs[Key_AnacondaCPU] = strconv.FormatFloat(newCPU, 'f', 1, 64)

			err = callbackSaveNewInfo(myServiceInfo)
			if err != nil {
				logger.Error("Zeppelin instance is update but save info error", err)
			}

		}()
	} else {
		logger.Infoln("Zeppelin instance is not update.")
	}

	return nil
}

func updateAnacondaResources(instanceId, serviceBrokerNamespace, zeppelinUsr, zeppelinPassward string,
	newMemory int, newCPU float64,
	authInfoChanged bool) error {

	var input anacodaResources_Master

	err := loadAnacodaResources_Master(instanceId, zeppelinUsr, zeppelinPassward, newMemory, newCPU, &input)

	if err != nil {
		logger.Error("updateStormResources_Superviser. load error", err)
		return err
	}

	prefix := "/namespaces/" + serviceBrokerNamespace

	var middle anacodaResources_Master
	osr := oshandler.NewOpenshiftREST(oshandler.OC())
	osr.
		KGet(prefix+"/replicationcontrollers/"+input.rc.Name, &middle.rc)

	if osr.Err != nil {
		logger.Error("updateAnacondaResources. get error", osr.Err)
		return osr.Err
	}

	// update ...

	if middle.rc.Spec.Template == nil || len(middle.rc.Spec.Template.Spec.Containers) == 0 {
		err = errors.New("rc.Template is nil or len(containers) == 0")
		logger.Error("updateAnacondaResources, zrrc.", err)
		return err
	}

	// Limit Memory
	{
		m, err := kresource.ParseQuantity(strconv.Itoa(newMemory) + "Mi")
		if err != nil {
			logger.Error("updateAnacondaResources.", err)
			return err
		}
		middle.rc.Spec.Template.Spec.Containers[0].Resources.Limits[kapi.ResourceMemory] = *m
	}

	// Limit CPU
	{
		c, err := kresource.ParseQuantity(strconv.FormatFloat(newCPU, 'f', 1, 64))
		if err != nil {
			logger.Error("updateAnacondaResources.", err)
			return err
		}

		middle.rc.Spec.Template.Spec.Containers[0].Resources.Limits[kapi.ResourceCPU] = *c
	}

	//updateing PUT
	var output anacodaResources_Master
	osr.
		KPut(prefix+"/replicationcontrollers/"+input.rc.Name, &middle.rc, &output.rc)
	if osr.Err != nil {
		logger.Error("updateAnacondaResources. update error", osr.Err)
		return osr.Err
	}

	// ...

	if authInfoChanged {
		n, err2 := deleteCreatedPodsByLabels(serviceBrokerNamespace, middle.rc.Labels)
		logger.Infoln("updateStormResources_Nimbus:", n, "pods are deleted.")
		if err2 != nil {
			err = err2
		} else {
			time.Sleep(time.Second * 10)
		}
	}

	return err
}

func deleteCreatedPodsByLabels(serviceBrokerNamespace string, labels map[string]string) (int, error) {

	logger.Infoln("to delete created pods in", serviceBrokerNamespace)
	if len(labels) == 0 {
		return 0, errors.New("labels can't be blank in deleteCreatedPodsByLabels")
	}

	uri := "/namespaces/" + serviceBrokerNamespace + "/pods"

	pods := kapi.PodList{}

	osr := oshandler.NewOpenshiftREST(oshandler.OC()).KList(uri, labels, &pods)
	if osr.Err != nil {
		return 0, osr.Err
	}

	ndeleted := 0

	for i := range pods.Items {
		pod := &pods.Items[i]

		logger.Infoln("\n pods.Items[", i, "].Status.Phase =", pod.Status.Phase, "\n")

		if pod.Status.Phase != kapi.PodSucceeded {
			ndeleted++
			kdel(serviceBrokerNamespace, "pods", pod.Name)
		}
	}

	return ndeleted, nil
}


func (handler *Anacoda_Handler) DoDeprovision(myServiceInfo *oshandler.ServiceInfo, asyncAllowed bool) (brokerapi.IsAsync, error) {
	// ...

	logger.Infoln("to destroy resources")

	memory, _ := strconv.Atoi(myServiceInfo.Miscs[Key_AnacondaMemory])
	cpu, _ := strconv.ParseFloat(myServiceInfo.Miscs[Key_AnacondaCPU], 64)

	master_res, _ := getAnacodaResources_Master(myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.User, myServiceInfo.Password,memory,cpu)
	destroyAnacodaResources_Master(master_res, myServiceInfo.Database)

	return brokerapi.IsAsync(false), nil
}

// please note: the bsi may be still not fully initialized when calling the function.
func getCredentialsOnPrivision(myServiceInfo *oshandler.ServiceInfo) oshandler.Credentials {
	var master_res anacodaResources_Master
	memory, _ := strconv.Atoi(myServiceInfo.Miscs[Key_AnacondaMemory])
	cpu, _ := strconv.ParseFloat(myServiceInfo.Miscs[Key_AnacondaCPU], 64)
	err := loadAnacodaResources_Master(myServiceInfo.Url, myServiceInfo.User, myServiceInfo.Password,memory,cpu ,&master_res)
	if err != nil {
		logger.Error("getCredentialsOnPrivision loadAnacodaResources_Master error ", err)
		return oshandler.Credentials{}
	}

	web_port := oshandler.GetServicePortByName(&master_res.service, "web")
	if web_port == nil {
		return oshandler.Credentials{}
	}

	host := fmt.Sprintf("%s.%s.%s", master_res.service.Name, myServiceInfo.Database, oshandler.ServiceDomainSuffix(false))
	port := strconv.Itoa(web_port.Port)
	//host := master_res.routeMQ.Spec.Host
	//port := "80"

	return oshandler.Credentials{
		Uri:      "",
		Hostname: host,
		Port:     port,
		Username: myServiceInfo.User,
		Password: myServiceInfo.Password,
	}
}

func (handler *Anacoda_Handler) DoBind(myServiceInfo *oshandler.ServiceInfo, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, oshandler.Credentials, error) {
	// todo: handle errors
	memory, _ := strconv.Atoi(myServiceInfo.Miscs[Key_AnacondaMemory])
	cpu, _ := strconv.ParseFloat(myServiceInfo.Miscs[Key_AnacondaCPU], 64)
	master_res, err := getAnacodaResources_Master(myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.User, myServiceInfo.Password,memory,cpu)
	if err != nil {
		logger.Error("DoBind getAnacodaResources_Master error ", err)
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

func (handler *Anacoda_Handler) DoUnbind(myServiceInfo *oshandler.ServiceInfo, mycredentials *oshandler.Credentials) error {
	// do nothing

	return nil
}

//=======================================================================
//
//=======================================================================

var AnacondaTemplateData_Master []byte = nil

func loadAnacodaResources_Master(instanceID, anacodaUser, anacodaPassword string, anacondaMemory int, anacondaCPU float64,res *anacodaResources_Master) error {
	if AnacondaTemplateData_Master == nil {
		f, err := os.Open("anaconda3.yaml")
		if err != nil {
			logger.Error("open yaml error ", err)
			return err
		}
		AnacondaTemplateData_Master, err = ioutil.ReadAll(f)
		if err != nil {
			return err
		}
		endpoint_postfix := oshandler.EndPointSuffix()
		endpoint_postfix = strings.TrimSpace(endpoint_postfix)
		if len(endpoint_postfix) > 0 {
			AnacondaTemplateData_Master = bytes.Replace(
				AnacondaTemplateData_Master,
				[]byte("endpoint-postfix-place-holder"),
				[]byte(endpoint_postfix),
				-1)
		}
		anacoda_image := oshandler.AnacodaImage()
		anacoda_image = strings.TrimSpace(anacoda_image)
		if len(anacoda_image) > 0 {
			AnacondaTemplateData_Master = bytes.Replace(
				AnacondaTemplateData_Master,
				[]byte("http://anaconda3-image-place-holder/anaconda3-openshift-orchestration"),
				[]byte(anacoda_image),
				-1)
		}
	}

	// ...

	yamlTemplates := AnacondaTemplateData_Master

	yamlTemplates = bytes.Replace(yamlTemplates, []byte("instanceid"), []byte(instanceID), -1)
	//yamlTemplates = bytes.Replace(yamlTemplates, []byte("sb-token"), []byte(anacodaPassword), -1)
	//yamlTemplates = bytes.Replace(yamlTemplates, []byte("user*****"), []byte(anacondaUser), -1)
	//yamlTemplates = bytes.Replace(yamlTemplates, []byte("pass*****"), []byte(anacondaPassword), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("memory*****"), []byte(strconv.Itoa(anacondaMemory)), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("cpu*****"), []byte(strconv.FormatFloat(anacondaCPU, 'f', 1, 64)), -1)



	//logger.Infoln("========= Boot yamlTemplates ===========")
	//logger.Infoln(string(yamlTemplates))
	//logger.Infoln()

	decoder := oshandler.NewYamlDecoder(yamlTemplates)
	decoder.
		Decode(&res.rc).
		Decode(&res.route).
		Decode(&res.service)

	return decoder.Err
}

type anacodaResources_Master struct {
	rc      kapi.ReplicationController
	route   routeapi.Route
	service kapi.Service
}

func createAnacodaResources_Master(instanceId, serviceBrokerNamespace, anacondaUser, anacondaPassword string,anacondaMemory int, anacondaCPU float64) (*anacodaResources_Master, error) {
	var input anacodaResources_Master
	err := loadAnacodaResources_Master(instanceId, anacondaUser, anacondaPassword, anacondaMemory,anacondaCPU,&input)
	if err != nil {
		logger.Error("createAnacodaResources_Master loadAnacodaResources_Master error ", err)
		return nil, err
	}

	var output anacodaResources_Master

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	// here, not use job.post
	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		KPost(prefix+"/replicationcontrollers", &input.rc, &output.rc).
		OPost(prefix+"/routes", &input.route, &output.route).
		KPost(prefix+"/services", &input.service, &output.service)

	if osr.Err != nil {
		logger.Error("createAnacodaResources_Master", osr.Err)
	}

	return &output, osr.Err
}

func getAnacodaResources_Master(instanceId, serviceBrokerNamespace, anacodaUser, anacodaPassword string,anacondaMemory int, anacondaCPU float64) (*anacodaResources_Master, error) {
	var output anacodaResources_Master

	var input anacodaResources_Master
	err := loadAnacodaResources_Master(instanceId, anacodaUser, anacodaPassword,anacondaMemory,anacondaCPU, &input)
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
		logger.Error("getAnacodaResources_Master", osr.Err)
	}

	return &output, osr.Err
}

func destroyAnacodaResources_Master(masterRes *anacodaResources_Master, serviceBrokerNamespace string) {
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

	logger.Infoln("to delete pods on replicationcontroller", rc.Name)

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
				logger.Error("watch HA anaconda rc error", status.Err)
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
