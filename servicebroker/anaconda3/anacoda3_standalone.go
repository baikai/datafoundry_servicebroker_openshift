package anaconda3

import (
	"errors"
	"fmt"
	//marathon "github.com/gambol99/go-marathon"
	//kapi "golang.org/x/build/kubernetes/api"
	//"golang.org/x/build/kubernetes"
	//"golang.org/x/oauth2"
	//"net/http"
	//"net"
	"github.com/pivotal-cf/brokerapi"
	//"time"
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	//"crypto/sha1"
	//"encoding/base64"
	//"text/template"
	//"io"
	"io/ioutil"
	"os"
	//"sync"

	"github.com/pivotal-golang/lager"

	//"k8s.io/kubernetes/pkg/util/yaml"
	routeapi "github.com/openshift/origin/route/api/v1"
	kapi "k8s.io/kubernetes/pkg/api/v1"

	oshandler "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/handler"
	"net/http"
	"crypto/tls"
)

//==============================================================
//
//==============================================================

const AnacodaServcieBrokerName_Standalone = "Anaconda_standalone"

func init() {
	oshandler.Register(AnacodaServcieBrokerName_Standalone, &Anacoda_freeHandler{})

	logger = lager.NewLogger(AnacodaServcieBrokerName_Standalone)
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))
}

var logger lager.Logger

var httpClient = &http.Client{
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	},
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

func (handler *Anacoda_Handler) DoProvision(etcdSaveResult chan error, instanceID string, details brokerapi.ProvisionDetails, planInfo oshandler.PlanInfo, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, oshandler.ServiceInfo, error) {
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


	println()
	println("instanceIdInTempalte = ", instanceIdInTempalte)
	println("serviceBrokerNamespace = ", serviceBrokerNamespace)
	println()

	serviceInfo.Url = instanceIdInTempalte
	serviceInfo.Database = serviceBrokerNamespace // may be not needed
	serviceInfo.User = ""
	serviceInfo.Password = oshandler.GenGUID()

	go func() {
		err := <-etcdSaveResult
		if err != nil {
			return
		}

		// master
		output, err := createAnacodaResources_Master(instanceIdInTempalte, serviceBrokerNamespace, serviceInfo.User, serviceInfo.Password)

		if err != nil {
			destroyAnacodaResources_Master(output, serviceBrokerNamespace)

			return
		}

	}()

	var input anacodaResources_Master
	err := loadAnacodaResources_Master(instanceIdInTempalte,serviceInfo.User, serviceInfo.Password, &input)
	if err != nil {
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

	master_res, _ := getAnacodaResources_Master(myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.User, myServiceInfo.Password)

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

	// todo: check if http get dashboard request is ok

	if ok(&master_res.rc) {
		req, _ := http.NewRequest("GET", "http://"+master_res.route.Spec.Host, nil)
		request,_ :=httpClient.Do(req)
		defer request.Body.Close()
		if request.StatusCode == 200{
			return brokerapi.LastOperation{
				State:       brokerapi.Succeeded,
				Description: "Succeeded!",
			}, nil

		}
	}
	return brokerapi.LastOperation{
		State:       brokerapi.InProgress,
		Description: "In progress.",
	}, nil
}

func (handler *Anacoda_Handler) DoUpdate(myServiceInfo *oshandler.ServiceInfo, planInfo oshandler.PlanInfo, callbackSaveNewInfo func(*oshandler.ServiceInfo) error, asyncAllowed bool) error {
	return nil
}

func (handler *Anacoda_Handler) DoDeprovision(myServiceInfo *oshandler.ServiceInfo, asyncAllowed bool) (brokerapi.IsAsync, error) {
	// ...

	println("to destroy resources")

	master_res, _ := getAnacodaResources_Master(myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.User, myServiceInfo.Password)
	destroyAnacodaResources_Master(master_res, myServiceInfo.Database)

	return brokerapi.IsAsync(false), nil
}

// please note: the bsi may be still not fully initialized when calling the function.
func getCredentialsOnPrivision(myServiceInfo *oshandler.ServiceInfo) oshandler.Credentials {
	var master_res anacodaResources_Master
	err := loadAnacodaResources_Master(myServiceInfo.Url, myServiceInfo.User, myServiceInfo.Password, &master_res)
	if err != nil {
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

	master_res, err := getAnacodaResources_Master(myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.User, myServiceInfo.Password)
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

func (handler *Anacoda_Handler) DoUnbind(myServiceInfo *oshandler.ServiceInfo, mycredentials *oshandler.Credentials) error {
	// do nothing

	return nil
}

//=======================================================================
//
//=======================================================================

var AnacondaTemplateData_Master []byte = nil

func loadAnacodaResources_Master(instanceID, anacodaUser, anacodaPassword string, res *anacodaResources_Master) error {
	if AnacondaTemplateData_Master == nil {
		f, err := os.Open("anaconda3.yaml")
		if err != nil {
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
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("sb-token"), []byte(anacodaPassword), -1)
	//yamlTemplates = bytes.Replace(yamlTemplates, []byte("user*****"), []byte(anacondaUser), -1)
	//yamlTemplates = bytes.Replace(yamlTemplates, []byte("pass*****"), []byte(anacondaPassword), -1)

	//println("========= Boot yamlTemplates ===========")
	//println(string(yamlTemplates))
	//println()



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

func createAnacodaResources_Master(instanceId, serviceBrokerNamespace, anacondaUser, anacondaPassword string) (*anacodaResources_Master, error) {
	var input anacodaResources_Master
	err := loadAnacodaResources_Master(instanceId, anacondaUser, anacondaPassword, &input)
	if err != nil {
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

func getAnacodaResources_Master(instanceId, serviceBrokerNamespace, anacodaUser, anacodaPassword string) (*anacodaResources_Master, error) {
	var output anacodaResources_Master

	var input anacodaResources_Master
	err := loadAnacodaResources_Master(instanceId, anacodaUser, anacodaPassword, &input)
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

func kpost(serviceBrokerNamespace, typeName string, body interface{}, into interface{}) error {
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

func opost(serviceBrokerNamespace, typeName string, body interface{}, into interface{}) error {
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
				logger.Error("watch HA anaconda rc error", status.Err)
				close(cancel)
				return
			} else {
				//logger.Debug("watch anaconda HA rc, status.Info: " + string(status.Info))
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


