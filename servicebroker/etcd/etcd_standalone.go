package etcd

import (
	"bytes"
	"encoding/json"
	"fmt"
	oshandler "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/handler"
	etcd "github.com/coreos/etcd/client"
	routeapi "github.com/openshift/origin/route/api/v1"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-golang/lager"
	"io/ioutil"
	kapi "k8s.io/kubernetes/pkg/api/v1"
	"net"
	"os"
	"strings"
	"time"
)

//==============================================================
//
//==============================================================

const EtcdServcieBrokerName_Standalone = "ETCD_standalone"

func init() {
	oshandler.Register(EtcdServcieBrokerName_Standalone, &Etcd_sampleHandler{})

	logger = lager.NewLogger(EtcdServcieBrokerName_Standalone)
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))
}

var logger lager.Logger

//==============================================================
//
//==============================================================

const EtcdBindRole = "binduser"

const EtcdGeneralUser = "etcduser"

//const ServiceBrokerNamespace = "default" // use oshandler.OC().Namespace instead

type Etcd_sampleHandler struct{}

func (handler *Etcd_sampleHandler) DoProvision(etcdSaveResult chan error, instanceID string, details brokerapi.ProvisionDetails, planInfo oshandler.PlanInfo, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, oshandler.ServiceInfo, error) {
	//初始化到openshift的链接

	serviceSpec := brokerapi.ProvisionedServiceSpec{IsAsync: asyncAllowed}
	serviceInfo := oshandler.ServiceInfo{}

	serviceSpec.IsAsync = true

	//instanceIdInTempalte   := instanceID // todo: ok?
	instanceIdInTempalte := strings.ToLower(oshandler.NewThirteenLengthID())
	//serviceBrokerNamespace := ServiceBrokerNamespace
	serviceBrokerNamespace := oshandler.OC().Namespace()
	rootPassword := oshandler.GenGUID()
	etcduser := EtcdGeneralUser //oshandler.NewElevenLengthID() // oshandler.GenGUID()[:16]
	etcdPassword := oshandler.GenGUID()

	serviceInfo.Url = instanceIdInTempalte
	serviceInfo.Database = serviceBrokerNamespace // may be not needed
	serviceInfo.Admin_password = rootPassword
	serviceInfo.User = etcduser
	serviceInfo.Password = etcdPassword

	println()
	println("instanceIdInTempalte = ", instanceIdInTempalte)
	println("serviceBrokerNamespace = ", serviceBrokerNamespace)
	println()

	// boot etcd

	go func() {
		err := <-etcdSaveResult
		if err != nil {
			return
		}

		output, err := createEtcdResources_HA(
			instanceIdInTempalte, serviceBrokerNamespace,
			rootPassword, etcduser, etcdPassword)

		if err != nil {
			destroyEtcdResources_HA(output, serviceBrokerNamespace)

			return
		}

	}()

	serviceSpec.DashboardURL = ""

	//>>>
	serviceSpec.Credentials = getCredentialsOnPrivision(&serviceInfo)
	//<<<

	return serviceSpec, serviceInfo, nil
}

func (handler *Etcd_sampleHandler) DoLastOperation(myServiceInfo *oshandler.ServiceInfo) (brokerapi.LastOperation, error) {

	// only check the statuses of 3 ReplicationControllers. The etcd pods may be not running well.

	ok := func(rc *kapi.ReplicationController) bool {
		if rc == nil || rc.Name == "" || rc.Spec.Replicas == nil || rc.Status.Replicas < *rc.Spec.Replicas {
			return false
		}
		n, _ := statRunningPodsByLabels(myServiceInfo.Database, rc.Labels)
		return n >= *rc.Spec.Replicas
	}

	ha_res, _ := getEtcdResources_HA(
		myServiceInfo.Url, myServiceInfo.Database,
		myServiceInfo.Admin_password, myServiceInfo.User, myServiceInfo.Password)

	//println("num_ok_rcs = ", num_ok_rcs)

	if ok(&ha_res.etcdrc1) && ok(&ha_res.etcdrc2) && ok(&ha_res.etcdrc3) && ok(&ha_res.etcdrc0) {
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

func (handler *Etcd_sampleHandler) DoUpdate(myServiceInfo *oshandler.ServiceInfo, planInfo oshandler.PlanInfo, callbackSaveNewInfo func(*oshandler.ServiceInfo) error, asyncAllowed bool) error {
	return nil
}

func (handler *Etcd_sampleHandler) DoDeprovision(myServiceInfo *oshandler.ServiceInfo, asyncAllowed bool) (brokerapi.IsAsync, error) {

	ha_res, _ := getEtcdResources_HA(
		myServiceInfo.Url, myServiceInfo.Database,
		myServiceInfo.Admin_password, myServiceInfo.User, myServiceInfo.Password)

	destroyEtcdResources_HA(ha_res, myServiceInfo.Database)

	return brokerapi.IsAsync(false), nil
}

// please note: the bsi may be still not fully initialized when calling the function.
func getCredentialsOnPrivision(myServiceInfo *oshandler.ServiceInfo) oshandler.Credentials {
	var ha_res etcdResources_HA
	err := loadEtcdResources_HA(myServiceInfo.Url, myServiceInfo.Admin_password, myServiceInfo.User, myServiceInfo.Password, &ha_res)
	if err != nil {
		return oshandler.Credentials{}
	}

	if ha_res.route.Name == "" {
		return oshandler.Credentials{}
	}

	etcd_addr, host, port := ha_res.endpoint()
	println("etcd addr: ", etcd_addr)
	//etcd_addrs := []string{etcd_addr}

	return oshandler.Credentials{
		Uri:      etcd_addr,
		Hostname: host,
		Port:     port,
		Username: myServiceInfo.User,
		Password: myServiceInfo.Password,
	}
}

func (handler *Etcd_sampleHandler) DoBind(myServiceInfo *oshandler.ServiceInfo, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, oshandler.Credentials, error) {
	// output.route.Spec.Host

	ha_res, err := getEtcdResources_HA(
		myServiceInfo.Url, myServiceInfo.Database,
		myServiceInfo.Admin_password, myServiceInfo.User, myServiceInfo.Password)

	if ha_res.route.Name == "" {
		return brokerapi.Binding{}, oshandler.Credentials{}, err
	}

	etcd_addr, host, port := ha_res.endpoint()
	println("etcd addr: ", etcd_addr)
	//etcd_addrs := []string{etcd_addr}

	mycredentials := oshandler.Credentials{
		Uri:      etcd_addr,
		Hostname: host,
		Port:     port,
		Username: myServiceInfo.User,
		Password: myServiceInfo.Password,
	}

	myBinding := brokerapi.Binding{Credentials: mycredentials}

	return myBinding, mycredentials, nil
}

func (handler *Etcd_sampleHandler) DoUnbind(myServiceInfo *oshandler.ServiceInfo, mycredentials *oshandler.Credentials) error {

	return nil
}

//=============================================================

func newUnauthrizedEtcdClient(etcdEndPoints []string) (etcd.Client, error) {
	cfg := etcd.Config{
		Endpoints:               etcdEndPoints,
		Transport:               etcd.DefaultTransport,
		HeaderTimeoutPerRequest: 15 * time.Second,
	}
	return etcd.New(cfg)
}

func newAuthrizedEtcdClient(etcdEndPoints []string, etcdUser, etcdPassword string) (etcd.Client, error) {
	cfg := etcd.Config{
		Endpoints:               etcdEndPoints,
		Transport:               etcd.DefaultTransport,
		HeaderTimeoutPerRequest: 15 * time.Second,
		Username:                etcdUser,
		Password:                etcdPassword,
	}
	return etcd.New(cfg)
}

//===============================================================
//
//===============================================================

var EtcdTemplateData_HA []byte = nil

func loadEtcdResources_HA(instanceID, rootPassword, user, password string, res *etcdResources_HA) error {
	if EtcdTemplateData_HA == nil {
		f, err := os.Open("etcd.yaml")
		if err != nil {
			return err
		}
		EtcdTemplateData_HA, err = ioutil.ReadAll(f)
		if err != nil {
			return err
		}

		etcd_image := oshandler.EtcdImage()
		etcd_image = strings.TrimSpace(etcd_image)
		if len(etcd_image) > 0 {
			EtcdTemplateData_HA = bytes.Replace(
				EtcdTemplateData_HA,
				[]byte("http://etcd-image-place-holder/etcd-openshift-orchestration"),
				[]byte(etcd_image),
				-1)
		}
		endpoint_postfix := oshandler.EndPointSuffix()
		endpoint_postfix = strings.TrimSpace(endpoint_postfix)
		if len(endpoint_postfix) > 0 {
			EtcdTemplateData_HA = bytes.Replace(
				EtcdTemplateData_HA,
				[]byte("endpoint-postfix-place-holder"),
				[]byte(endpoint_postfix),
				-1)
		}
	}

	yamlTemplates := EtcdTemplateData_HA

	yamlTemplates = bytes.Replace(yamlTemplates, []byte("instanceid"), []byte(instanceID), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("#ETCDROOTPASSWORD#"), []byte(rootPassword), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("#ETCDUSERNAME#"), []byte(user), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("#ETCDUSERPASSWORD#"), []byte(password), -1)

	decoder := oshandler.NewYamlDecoder(yamlTemplates)
	decoder.
		Decode(&res.svc).
		Decode(&res.route).
		Decode(&res.etcdrc1).
		Decode(&res.etcdsvc1).
		Decode(&res.etcdrc2).
		Decode(&res.etcdsvc2).
		Decode(&res.etcdrc3).
		Decode(&res.etcdsvc3).
		Decode(&res.etcdrc0).
		Decode(&res.etcdsvc0)

	return decoder.Err
}

type etcdResources_HA struct {
	svc   kapi.Service
	route routeapi.Route

	etcdrc1  kapi.ReplicationController
	etcdsvc1 kapi.Service
	etcdrc2  kapi.ReplicationController
	etcdsvc2 kapi.Service
	etcdrc3  kapi.ReplicationController
	etcdsvc3 kapi.Service

	etcdrc0  kapi.ReplicationController
	etcdsvc0 kapi.Service
}

func (haRes *etcdResources_HA) endpoint() (string, string, string) {
	port := "80" // strconv.Itoa(bootRes.service.Spec.Ports[0].Port)
	host := haRes.route.Spec.Host
	return "http://" + net.JoinHostPort(host, port), host, port
}

func createEtcdResources_HA(instanceId, serviceBrokerNamespace, rootPassword, user, password string) (*etcdResources_HA, error) {
	var input etcdResources_HA
	err := loadEtcdResources_HA(instanceId, rootPassword, user, password, &input)
	if err != nil {
		return nil, err
	}

	var output etcdResources_HA

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		KPost(prefix+"/services", &input.svc, &output.svc).
		OPost(prefix+"/routes", &input.route, &output.route).
		KPost(prefix+"/replicationcontrollers", &input.etcdrc1, &output.etcdrc1).
		KPost(prefix+"/services", &input.etcdsvc1, &output.etcdsvc1).
		KPost(prefix+"/replicationcontrollers", &input.etcdrc2, &output.etcdrc2).
		KPost(prefix+"/services", &input.etcdsvc2, &output.etcdsvc2).
		KPost(prefix+"/replicationcontrollers", &input.etcdrc3, &output.etcdrc3).
		KPost(prefix+"/services", &input.etcdsvc3, &output.etcdsvc3).
		KPost(prefix+"/replicationcontrollers", &input.etcdrc0, &output.etcdrc0).
		KPost(prefix+"/services", &input.etcdsvc0, &output.etcdsvc0)

	if osr.Err != nil {
		logger.Error("createEtcdResources_HA", osr.Err)
	}

	return &output, nil
}

func getEtcdResources_HA(instanceId, serviceBrokerNamespace, rootPassword, user, password string) (*etcdResources_HA, error) {
	var output etcdResources_HA

	var input etcdResources_HA
	err := loadEtcdResources_HA(instanceId, rootPassword, user, password, &input)
	if err != nil {
		return &output, err
	}

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		KGet(prefix+"/services/"+input.svc.Name, &output.svc).
		OGet(prefix+"/routes/"+input.route.Name, &output.route).
		KGet(prefix+"/services/"+input.etcdsvc1.Name, &output.etcdsvc1).
		KGet(prefix+"/replicationcontrollers/"+input.etcdrc1.Name, &output.etcdrc1).
		KGet(prefix+"/services/"+input.etcdsvc2.Name, &output.etcdsvc2).
		KGet(prefix+"/replicationcontrollers/"+input.etcdrc2.Name, &output.etcdrc2).
		KGet(prefix+"/services/"+input.etcdsvc3.Name, &output.etcdsvc3).
		KGet(prefix+"/replicationcontrollers/"+input.etcdrc3.Name, &output.etcdrc3).
		KGet(prefix+"/services/"+input.etcdsvc0.Name, &output.etcdsvc0).
		KGet(prefix+"/replicationcontrollers/"+input.etcdrc0.Name, &output.etcdrc0)

	if osr.Err != nil {
		logger.Error("getEtcdResources_HA", osr.Err)
	}

	return &output, osr.Err
}

func destroyEtcdResources_HA(haRes *etcdResources_HA, serviceBrokerNamespace string) {
	// todo: add to retry queue on fail

	go func() { kdel(serviceBrokerNamespace, "services", haRes.svc.Name) }()
	go func() { odel(serviceBrokerNamespace, "routes", haRes.route.Name) }()

	go func() { kdel(serviceBrokerNamespace, "services", haRes.etcdsvc1.Name) }()
	go func() { kdel_rc(serviceBrokerNamespace, &haRes.etcdrc1) }()

	go func() { kdel(serviceBrokerNamespace, "services", haRes.etcdsvc2.Name) }()
	go func() { kdel_rc(serviceBrokerNamespace, &haRes.etcdrc2) }()

	go func() { kdel(serviceBrokerNamespace, "services", haRes.etcdsvc3.Name) }()
	go func() { kdel_rc(serviceBrokerNamespace, &haRes.etcdrc3) }()

	go func() { kdel(serviceBrokerNamespace, "services", haRes.etcdsvc0.Name) }()
	go func() { kdel_rc(serviceBrokerNamespace, &haRes.etcdrc0) }()
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
				logger.Error("watch HA etcd rc error", status.Err)
				close(cancel)
				return
			}

			var wrcs watchReplicationControllerStatus
			if err := json.Unmarshal(status.Info, &wrcs); err != nil {
				logger.Error("parse boot HA rc status", err)
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

// todo:
//   use etcd clientv3 instead, which is able to close a client.
