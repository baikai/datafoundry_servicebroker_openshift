package ocsp

import (
	"errors"
	"fmt"
	//marathon "github.com/gambol99/go-marathon"
	//kapi "golang.org/x/build/kubernetes/api"
	//"golang.org/x/build/kubernetes"
	//"golang.org/x/oauth2"
	//"net/http"
	"bytes"
	"encoding/json"
	//"net"
	"strconv"
	"strings"

	"github.com/pivotal-cf/brokerapi"
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
)

//==============================================================
//
//==============================================================

const OcspServcieBrokerName_Standalone = "Ocsp_standalone"

func init() {
	oshandler.Register(OcspServcieBrokerName_Standalone, &Ocsp_freeHandler{})

	logger = lager.NewLogger(OcspServcieBrokerName_Standalone)
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))
}

var logger lager.Logger

//==============================================================
//
//==============================================================

type Ocsp_freeHandler struct{}

func (handler *Ocsp_freeHandler) DoProvision(etcdSaveResult chan error, instanceID string, details brokerapi.ProvisionDetails, planInfo oshandler.PlanInfo, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, oshandler.ServiceInfo, error) {
	return newOcspHandler().DoProvision(etcdSaveResult, instanceID, details, planInfo, asyncAllowed)
}

func (handler *Ocsp_freeHandler) DoLastOperation(myServiceInfo *oshandler.ServiceInfo) (brokerapi.LastOperation, error) {
	return newOcspHandler().DoLastOperation(myServiceInfo)
}

func (handler *Ocsp_freeHandler) DoUpdate(myServiceInfo *oshandler.ServiceInfo, planInfo oshandler.PlanInfo, callbackSaveNewInfo func(*oshandler.ServiceInfo) error, asyncAllowed bool) error {
	return newOcspHandler().DoUpdate(myServiceInfo, planInfo, callbackSaveNewInfo, asyncAllowed)
}

func (handler *Ocsp_freeHandler) DoDeprovision(myServiceInfo *oshandler.ServiceInfo, asyncAllowed bool) (brokerapi.IsAsync, error) {
	return newOcspHandler().DoDeprovision(myServiceInfo, asyncAllowed)
}

func (handler *Ocsp_freeHandler) DoBind(myServiceInfo *oshandler.ServiceInfo, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, oshandler.Credentials, error) {
	return newOcspHandler().DoBind(myServiceInfo, bindingID, details)
}

func (handler *Ocsp_freeHandler) DoUnbind(myServiceInfo *oshandler.ServiceInfo, mycredentials *oshandler.Credentials) error {
	return newOcspHandler().DoUnbind(myServiceInfo, mycredentials)
}

//==============================================================
type Ocsp_Parameters struct {
	mysql_host     string
	mysql_port     string
	mysql_database string
	mysql_user     string
	mysql_pass     string
	codis_addr     string
	ocsp_user      string
}

var (
	G_mysql_host     = "mysql_host"
	G_mysql_port     = "mysql_port"
	G_mysql_database = "mysql_database"
	G_mysql_user     = "mysql_user"
	G_mysql_pass     = "mysql_pass"
	G_codis_addr     = "codis_addr"
	G_ocsp_user      = "ocsp_user"
)

func (ocspPara *Ocsp_Parameters) GetParaMeters(details *brokerapi.ProvisionDetails) (err error) {
	if details.Parameters == nil {
		err = errors.New("[OCSP] No parameters about mysql and codis.")
		return
	}

	var ok bool
	if ocspPara.mysql_host, ok = details.Parameters[G_mysql_host].(string); !ok {
		err = errors.New("[OCSP] No parameter " + G_mysql_host)
		return
	}
	if ocspPara.mysql_port, ok = details.Parameters[G_mysql_port].(string); !ok {
		err = errors.New("[OCSP] No parameter " + G_mysql_port)
		return
	}
	if ocspPara.mysql_database, ok = details.Parameters[G_mysql_database].(string); !ok {
		err = errors.New("[OCSP] No parameter " + G_mysql_database)
		return
	}
	if ocspPara.mysql_user, ok = details.Parameters[G_mysql_user].(string); !ok {
		err = errors.New("[OCSP] No parameter " + G_mysql_user)
		return
	}
	if ocspPara.mysql_pass, ok = details.Parameters[G_mysql_pass].(string); !ok {
		err = errors.New("[OCSP] No parameter " + G_mysql_pass)
		return
	}
	if ocspPara.codis_addr, ok = details.Parameters[G_codis_addr].(string); !ok {
		err = errors.New("[OCSP] No parameter " + G_codis_addr)
		return
	}
	if ocspPara.ocsp_user, ok = details.Parameters[G_ocsp_user].(string); !ok {
		err = errors.New("[OCSP] No parameter " + G_ocsp_user)
		return
	}

	return
}

type Ocsp_Handler struct {
}

func newOcspHandler() *Ocsp_Handler {
	return &Ocsp_Handler{}
}

func (handler *Ocsp_Handler) DoProvision(etcdSaveResult chan error, instanceID string, details brokerapi.ProvisionDetails, planInfo oshandler.PlanInfo, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, oshandler.ServiceInfo, error) {
	//初始化到openshift的链接

	serviceSpec := brokerapi.ProvisionedServiceSpec{IsAsync: asyncAllowed}
	serviceInfo := oshandler.ServiceInfo{}

	//if asyncAllowed == false {
	//	return serviceSpec, serviceInfo, errors.New("Sync mode is not supported")
	//}
	serviceSpec.IsAsync = true

	instanceIdInTempalte := strings.ToLower(oshandler.NewThirteenLengthID())
	//serviceBrokerNamespace := ServiceBrokerNamespace
	serviceBrokerNamespace := oshandler.OC().Namespace()
	ocspUser := " "
	ocspPassword := " " //oshandler.GenGUID()

	println()
	println("instanceIdInTempalte = ", instanceIdInTempalte)
	println("serviceBrokerNamespace = ", serviceBrokerNamespace)
	println()

	// master ocsp

	serviceInfo.Url = instanceIdInTempalte
	serviceInfo.Database = serviceBrokerNamespace // may be not needed
	serviceInfo.User = ocspUser
	serviceInfo.Password = ocspPassword //NEO4J_AUTH

	var ocspPara Ocsp_Parameters = Ocsp_Parameters{}
	if err := ocspPara.GetParaMeters(&details); err != nil {
		return serviceSpec, serviceInfo, err
	}

	// ...
	go func() {
		err := <-etcdSaveResult
		if err != nil {
			return
		}

		println("createOcspResources_Master ...")

		// create master res

		output, err := createOcspResources_Master(
			serviceInfo.Url,
			serviceInfo.Database,
			serviceInfo.User,
			serviceInfo.Password,
			details,
		)
		if err != nil {
			println(" ocsp createOcspResources_Master error: ", err)
			logger.Error("ocsp createOcspResources_Master error", err)

			destroyOcspResources_Master(output, serviceBrokerNamespace)
			//oshandler.DeleteVolumns(serviceInfo.Database, volumes)

			return
		}
	}()

	//serviceSpec.DashboardURL = "http://" + net.JoinHostPort(template.routeAdmin.Spec.Host, "80")

	//>>>
	serviceSpec.Credentials = getCredentialsOnPrivision(&serviceInfo)
	serviceSpec.DashboardURL = serviceSpec.Credentials.(oshandler.Credentials).Uri
	//<<<

	return serviceSpec, serviceInfo, nil
}

func (handler *Ocsp_Handler) DoLastOperation(myServiceInfo *oshandler.ServiceInfo) (brokerapi.LastOperation, error) {

	// assume in provisioning

	/*volumeJob := oshandler.GetCreatePvcVolumnJob(volumeBaseName(myServiceInfo.Url))
	if volumeJob != nil {
		return brokerapi.LastOperation{
			State:       brokerapi.InProgress,
			Description: "in progress.",
		}, nil
	}*/

	// the job may be finished or interrupted or running in another instance.

	master_res, _ := getOcspResources_Master(
		myServiceInfo.Url,
		myServiceInfo.Database,
		myServiceInfo.User,
		myServiceInfo.Password,
	)

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

func (handler *Ocsp_Handler) DoUpdate(myServiceInfo *oshandler.ServiceInfo, planInfo oshandler.PlanInfo, callbackSaveNewInfo func(*oshandler.ServiceInfo) error, asyncAllowed bool) error {
	/*go func() {
		// Update volume
		volumeBaseName := volumeBaseName(myServiceInfo.Url)
		result := oshandler.StartExpandPvcVolumnJob(
			volumeBaseName,
			myServiceInfo.Database,
			myServiceInfo.Volumes,
			planInfo.Volume_size,
		)

		err := <-result
		if err != nil {
			logger.Error("ocsp expand volume error", err)
			return
		}

		println("ocsp expand volumens done")

		for i := range myServiceInfo.Volumes {
			myServiceInfo.Volumes[i].Volume_size = planInfo.Volume_size
		}
		err = callbackSaveNewInfo(myServiceInfo)
		if err != nil {
			logger.Error("ocsp expand volume succeeded but save info error", err)
		}
	}()
	*/
	return nil
}

func (handler *Ocsp_Handler) DoDeprovision(myServiceInfo *oshandler.ServiceInfo, asyncAllowed bool) (brokerapi.IsAsync, error) {
	go func() {
		/*volumeJob := oshandler.GetCreatePvcVolumnJob(volumeBaseName(myServiceInfo.Url))
		if volumeJob != nil {
			volumeJob.Cancel()

			// wait job to exit
			for {
				time.Sleep(7 * time.Second)
				if nil == oshandler.GetCreatePvcVolumnJob(volumeBaseName(myServiceInfo.Url)) {
					break
				}
			}
		}*/

		println("to destroy resources:", myServiceInfo.Url)

		master_res, _ := getOcspResources_Master(
			myServiceInfo.Url,
			myServiceInfo.Database,
			myServiceInfo.User,
			myServiceInfo.Password,
		)
		destroyOcspResources_Master(master_res, myServiceInfo.Database)
	}()

	return brokerapi.IsAsync(false), nil
}

// please note: the bsi may be still not fully initialized when calling the function.
func getCredentialsOnPrivision(myServiceInfo *oshandler.ServiceInfo) oshandler.Credentials {
	var master_res ocspResources_Master
	err := loadOcspResources_Master(myServiceInfo.Url, myServiceInfo.User, myServiceInfo.Password, nil, &master_res)
	if err != nil {
		return oshandler.Credentials{}
	}

	port := "80"
	host := master_res.route.Spec.Host
	//var port string

	return oshandler.Credentials{
		Uri:      fmt.Sprintf("http://%s:%s", host, port),
		Hostname: host,
		Port:     port,
		Username: myServiceInfo.User,
		Password: myServiceInfo.Password,
		//Vhost:    master_res.routeAdmin.Spec.Host,
		Vhost: fmt.Sprintf("%s.%s.%s", master_res.service.Name, myServiceInfo.Database, oshandler.ServiceDomainSuffix(false)),
	}
}

func (handler *Ocsp_Handler) DoBind(myServiceInfo *oshandler.ServiceInfo, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, oshandler.Credentials, error) {
	// todo: handle errors

	master_res, err := getOcspResources_Master(
		myServiceInfo.Url,
		myServiceInfo.Database,
		myServiceInfo.User,
		myServiceInfo.Password,
	)
	if err != nil {
		return brokerapi.Binding{}, oshandler.Credentials{}, err
	}

	http_port := oshandler.GetServicePortByName(&master_res.service, "ocsp-http-port")
	if http_port == nil {
		return brokerapi.Binding{}, oshandler.Credentials{}, errors.New("ocsp-http-port not found")
	}

	host := fmt.Sprintf("%s.%s.%s", master_res.service.Name, myServiceInfo.Database, oshandler.ServiceDomainSuffix(false))
	port := strconv.Itoa(http_port.Port)
	//host := master_res.routeMQ.Spec.Host
	//port := "80"

	mycredentials := oshandler.Credentials{
		Uri:      fmt.Sprintf("http://%s:%s@%s:%s", myServiceInfo.User, myServiceInfo.Password, host, port),
		Hostname: host,
		Port:     port,
		Username: myServiceInfo.User,
		Password: myServiceInfo.Password,
	}

	myBinding := brokerapi.Binding{Credentials: mycredentials}

	return myBinding, mycredentials, nil
}

func (handler *Ocsp_Handler) DoUnbind(myServiceInfo *oshandler.ServiceInfo, mycredentials *oshandler.Credentials) error {
	// do nothing

	return nil
}

//=======================================================================
//
//=======================================================================

var OcspTemplateData_Master []byte = nil

func loadOcspResources_Master(instanceID, ocspUser, ocspPassword string, details *brokerapi.ProvisionDetails, res *ocspResources_Master) error {
	if OcspTemplateData_Master == nil {
		f, err := os.Open("ocsp.yaml")
		if err != nil {
			return err
		}
		OcspTemplateData_Master, err = ioutil.ReadAll(f)
		if err != nil {
			return err
		}
		ocspImage := oshandler.OcspImage()
		ocspImage = strings.TrimSpace(ocspImage)
		if len(ocspImage) > 0 {
			OcspTemplateData_Master = bytes.Replace(
				OcspTemplateData_Master,
				[]byte("http://ocsp-image-place-holder/ocsp-openshift-orchestration"),
				[]byte(ocspImage),
				-1)
		}
		endpoint_postfix := oshandler.EndPointSuffix()
		endpoint_postfix = strings.TrimSpace(endpoint_postfix)
		if len(endpoint_postfix) > 0 {
			OcspTemplateData_Master = bytes.Replace(
				OcspTemplateData_Master,
				[]byte("endpoint-postfix-place-holder"),
				[]byte(endpoint_postfix),
				-1)
		}
	}

	yamlTemplates := OcspTemplateData_Master

	yamlTemplates = bytes.Replace(yamlTemplates, []byte("instanceid"), []byte(instanceID), -1)

	//If details is not nil , this function is used for doprovision.
	if details != nil {
		var ocspPara Ocsp_Parameters = Ocsp_Parameters{}
		if err := ocspPara.GetParaMeters(details); err != nil {
			return err
		}
		yamlTemplates = bytes.Replace(yamlTemplates, []byte("**MYSQL_HOST**"), []byte(ocspPara.mysql_host), -1)
		yamlTemplates = bytes.Replace(yamlTemplates, []byte("**MYSQL_PORT**"), []byte(ocspPara.mysql_port), -1)
		yamlTemplates = bytes.Replace(yamlTemplates, []byte("**MYSQL_DATABASE**"), []byte(ocspPara.mysql_database), -1)
		yamlTemplates = bytes.Replace(yamlTemplates, []byte("**MYSQL_UNAME**"), []byte(ocspPara.mysql_user), -1)
		yamlTemplates = bytes.Replace(yamlTemplates, []byte("**MYSQL_PWD**"), []byte(ocspPara.mysql_pass), -1)
		yamlTemplates = bytes.Replace(yamlTemplates, []byte("**OCSP_USER**"), []byte(ocspPara.ocsp_user), -1)
		yamlTemplates = bytes.Replace(yamlTemplates, []byte("**CODIS_ADDR**"), []byte(ocspPara.codis_addr), -1)
	}

	yamlTemplates = bytes.Replace(yamlTemplates, []byte("**OCM**"), []byte(oshandler.OcspOcm()), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("**OCM_PORT**"), []byte(oshandler.OcspOcmPort()), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("**HDP_VERSION**"), []byte(oshandler.OcspHdpVersion()), -1)

	//println("========= Boot yamlTemplates ===========")
	//println(string(yamlTemplates))
	//println()

	decoder := oshandler.NewYamlDecoder(yamlTemplates)
	decoder.
		Decode(&res.rc).
		Decode(&res.service).
		Decode(&res.route)

	return decoder.Err
}

type ocspResources_Master struct {
	rc      kapi.ReplicationController
	service kapi.Service
	route   routeapi.Route
}

func createOcspResources_Master(instanceId, serviceBrokerNamespace, ocspUser, ocspPassword string, details brokerapi.ProvisionDetails) (*ocspResources_Master, error) {
	var input ocspResources_Master
	err := loadOcspResources_Master(instanceId, ocspUser, ocspPassword, &details, &input)
	if err != nil {
		return nil, err
	}

	var output ocspResources_Master

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	// here, not use job.post
	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		KPost(prefix+"/replicationcontrollers", &input.rc, &output.rc).
		OPost(prefix+"/routes", &input.route, &output.route).
		//OPost(prefix + "/routes", &input.routeMQ, &output.routeMQ).
		KPost(prefix+"/services", &input.service, &output.service)

	if osr.Err != nil {
		logger.Error("createOcspResources_Master", osr.Err)
	}

	return &output, osr.Err
}

func getOcspResources_Master(instanceId, serviceBrokerNamespace, ocspUser, ocspPassword string) (*ocspResources_Master, error) {
	var output ocspResources_Master

	var input ocspResources_Master
	err := loadOcspResources_Master(instanceId, ocspUser, ocspPassword, nil, &input)
	if err != nil {
		return &output, err
	}

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		KGet(prefix+"/replicationcontrollers/"+input.rc.Name, &output.rc).
		OGet(prefix+"/routes/"+input.route.Name, &output.route).
		//OGet(prefix + "/routes/" + input.routeMQ.Name, &output.routeMQ).
		KGet(prefix+"/services/"+input.service.Name, &output.service)

	if osr.Err != nil {
		logger.Error("getOcspResources_Master", osr.Err)
	}

	return &output, osr.Err
}

func destroyOcspResources_Master(masterRes *ocspResources_Master, serviceBrokerNamespace string) {
	// todo: add to retry queue on fail
	if masterRes == nil {
		return
	}
	go func() { kdel_rc(serviceBrokerNamespace, &masterRes.rc) }()
	go func() { odel(serviceBrokerNamespace, "routes", masterRes.route.Name) }()
	//go func() {odel (serviceBrokerNamespace, "routes", masterRes.routeMQ.Name)}()
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
				logger.Error("watch HA ocsp rc error", status.Err)
				close(cancel)
				return
			} else {
				//logger.Debug("watch ocsp HA rc, status.Info: " + string(status.Info))
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
