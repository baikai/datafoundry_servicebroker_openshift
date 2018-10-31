package zookeeper

import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
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
	"os"
	"strconv"
	"strings"
	"time"
)

//==============================================================
//初始化Log
//==============================================================

const ZookeeperServcieBrokerName_Standalone = "ZooKeeper_standalone"

func init() {
	oshandler.Register(ZookeeperServcieBrokerName_Standalone, &Zookeeper_freeHandler{})

	//logger = lager.NewLogger(ZookeeperServcieBrokerName_Standalone)
	//logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))
}

//var logger lager.Logger

//==============================================================
//
//==============================================================

type Zookeeper_freeHandler struct{}

func (handler *Zookeeper_freeHandler) DoProvision(etcdSaveResult chan error, instanceID string, details brokerapi.ProvisionDetails, planInfo oshandler.PlanInfo, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, oshandler.ServiceInfo, error) {
	return newZookeeperHandler().DoProvision(etcdSaveResult, instanceID, details, planInfo, asyncAllowed)
}

func (handler *Zookeeper_freeHandler) DoLastOperation(myServiceInfo *oshandler.ServiceInfo) (brokerapi.LastOperation, error) {
	return newZookeeperHandler().DoLastOperation(myServiceInfo)
}

func (handler *Zookeeper_freeHandler) DoUpdate(myServiceInfo *oshandler.ServiceInfo, planInfo oshandler.PlanInfo, callbackSaveNewInfo func(*oshandler.ServiceInfo) error, asyncAllowed bool) error {
	return newZookeeperHandler().DoUpdate(myServiceInfo, planInfo, callbackSaveNewInfo, asyncAllowed)
}

func (handler *Zookeeper_freeHandler) DoDeprovision(myServiceInfo *oshandler.ServiceInfo, asyncAllowed bool) (brokerapi.IsAsync, error) {
	return newZookeeperHandler().DoDeprovision(myServiceInfo, asyncAllowed)
}

func (handler *Zookeeper_freeHandler) DoBind(myServiceInfo *oshandler.ServiceInfo, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, oshandler.Credentials, error) {
	return newZookeeperHandler().DoBind(myServiceInfo, bindingID, details)
}

func (handler *Zookeeper_freeHandler) DoUnbind(myServiceInfo *oshandler.ServiceInfo, mycredentials *oshandler.Credentials) error {
	return newZookeeperHandler().DoUnbind(myServiceInfo, mycredentials)
}

//==============================================================
//
//==============================================================

type Zookeeper_Handler struct {
}

func newZookeeperHandler() *Zookeeper_Handler {
	return &Zookeeper_Handler{}
}

func (handler *Zookeeper_Handler) DoProvision(etcdSaveResult chan error, instanceID string, details brokerapi.ProvisionDetails, planInfo oshandler.PlanInfo, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, oshandler.ServiceInfo, error) {
	//初始化到openshift的链接

	serviceSpec := brokerapi.ProvisionedServiceSpec{IsAsync: asyncAllowed}
	serviceInfo := oshandler.ServiceInfo{}

	serviceSpec.IsAsync = true

	instanceIdInTempalte := strings.ToLower(oshandler.NewThirteenLengthID())
	serviceBrokerNamespace := oshandler.OC().Namespace()
	zookeeperUser := "super" // oshandler.NewElevenLengthID()
	zookeeperPassword := oshandler.GenGUID()

	serviceInfo.Url = instanceIdInTempalte
	serviceInfo.Database = serviceBrokerNamespace // may be not needed
	serviceInfo.User = zookeeperUser
	serviceInfo.Password = zookeeperPassword

	logger.Info("Zookeeper Creating ...", map[string]interface{}{"instanceIdInTempalte": instanceIdInTempalte, "serviceBrokerNamespace": serviceBrokerNamespace})

	go func() {
		err := <-etcdSaveResult
		if err != nil {
			return
		}

		// master zookeeper
		output, err := CreateZookeeperResources_Master(instanceIdInTempalte, serviceBrokerNamespace, zookeeperUser, zookeeperPassword)

		if err != nil {
			DestroyZookeeperResources_Master(output, serviceBrokerNamespace)

			return
		}
	}()

	serviceSpec.DashboardURL = "" // "http://" + net.JoinHostPort(output.route.Spec.Host, "80")

	//>>>
	serviceSpec.Credentials = getCredentialsOnPrivision(&serviceInfo)
	//<<<

	return serviceSpec, serviceInfo, nil
}

func (handler *Zookeeper_Handler) DoLastOperation(myServiceInfo *oshandler.ServiceInfo) (brokerapi.LastOperation, error) {

	// assume in provisioning

	// the job may be finished or interrupted or running in another instance.

	master_res, _ := GetZookeeperResources_Master(myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.User, myServiceInfo.Password)

	ok := func(rc *kapi.ReplicationController) bool {
		if rc == nil || rc.Name == "" || rc.Spec.Replicas == nil || rc.Status.Replicas < *rc.Spec.Replicas {
			return false
		}
		n, _ := statRunningPodsByLabels(myServiceInfo.Database, rc.Labels)
		return n >= *rc.Spec.Replicas
	}

	if ok(&master_res.rc1) && ok(&master_res.rc2) && ok(&master_res.rc3) {
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

func (handler *Zookeeper_Handler) DoUpdate(myServiceInfo *oshandler.ServiceInfo, planInfo oshandler.PlanInfo, callbackSaveNewInfo func(*oshandler.ServiceInfo) error, asyncAllowed bool) error {
	return nil
}

func (handler *Zookeeper_Handler) DoDeprovision(myServiceInfo *oshandler.ServiceInfo, asyncAllowed bool) (brokerapi.IsAsync, error) {
	// ...

	logger.Infoln("to destroy resources")

	master_res, _ := GetZookeeperResources_Master(myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.User, myServiceInfo.Password)

	DestroyZookeeperResources_Master(master_res, myServiceInfo.Database)

	return brokerapi.IsAsync(false), nil
}

// please note: the bsi may be still not fully initialized when calling the function.
func getCredentialsOnPrivision(myServiceInfo *oshandler.ServiceInfo) oshandler.Credentials {
	// todo: handle errors
	var master_res ZookeeperResources_Master
	err := LoadZookeeperResources_Master(myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.User, myServiceInfo.Password, &master_res)
	if err != nil {
		return oshandler.Credentials{}
	}

	host, port, err := master_res.ServiceHostPort(myServiceInfo.Database)
	if err != nil {
		return oshandler.Credentials{}
	}

	return oshandler.Credentials{
		Uri:      "",
		Hostname: host,
		Port:     port,
		Username: myServiceInfo.User,
		Password: myServiceInfo.Password,
	}
}

func (handler *Zookeeper_Handler) DoBind(myServiceInfo *oshandler.ServiceInfo, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, oshandler.Credentials, error) {
	// todo: handle errors

	master_res, _ := GetZookeeperResources_Master(myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.User, myServiceInfo.Password)

	host, port, err := master_res.ServiceHostPort(myServiceInfo.Database)
	if err != nil {
		return brokerapi.Binding{}, oshandler.Credentials{}, nil
	}

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

func (handler *Zookeeper_Handler) DoUnbind(myServiceInfo *oshandler.ServiceInfo, mycredentials *oshandler.Credentials) error {
	// do nothing

	return nil
}

//==============================================================
// interfaces for other service brokers which depend on zk
//==============================================================

func WatchZookeeperOrchestration(instanceId, serviceBrokerNamespace, zookeeperUser, zookeeperPassword string) (result <-chan bool, cancel chan<- struct{}, err error) {
	var input ZookeeperResources_Master
	err = LoadZookeeperResources_Master(instanceId, serviceBrokerNamespace, zookeeperUser, zookeeperPassword, &input)
	if err != nil {
		return
	}

	var output ZookeeperResources_Master
	err = getZookeeperResources_Master(serviceBrokerNamespace, &input, &output)
	if err != nil {
		return
	}

	rc1 := &output.rc1
	rc2 := &output.rc2
	rc3 := &output.rc3
	rc1.Status.Replicas = 0
	rc2.Status.Replicas = 0
	rc3.Status.Replicas = 0

	theresult := make(chan bool)
	result = theresult
	cancelled := make(chan struct{})
	cancel = cancelled

	go func() {
		ok := func(rc *kapi.ReplicationController) bool {
			if rc == nil || rc.Name == "" || rc.Spec.Replicas == nil {
				return false
			}

			if rc.Status.Replicas < *rc.Spec.Replicas {
				rc.Status.Replicas, _ = statRunningPodsByLabels(serviceBrokerNamespace, rc.Labels)

				logger.Infoln("rc = ", rc, ", rc.Status.Replicas = ", rc.Status.Replicas)
			}

			return rc.Status.Replicas >= *rc.Spec.Replicas
		}

		for {
			if ok(rc1) && ok(rc2) && ok(rc3) {
				theresult <- true

				return
			}

			var valid bool
			select {
			case <-cancelled:
				valid = false

			case <-time.After(15 * time.Second):
				// bug: pod phase change will not trigger rc status change.
				// so need this case
				continue
			}

			if !valid {
				theresult <- false

				return
			}
		}
	}()

	return
}

//=======================================================================
// the zookeeper functions may be called by outer packages
//=======================================================================

var ZookeeperTemplateData_Master []byte = nil

func LoadZookeeperResources_Master(instanceID, serviceBrokerNamespace, zookeeperUser, zookeeperPassword string, res *ZookeeperResources_Master) error {

	if ZookeeperTemplateData_Master == nil {
		f, err := os.Open("zookeeper-with-dashboard.yaml")
		if err != nil {
			return err
		}
		ZookeeperTemplateData_Master, err = ioutil.ReadAll(f)
		if err != nil {
			return err
		}
		endpoint_postfix := oshandler.EndPointSuffix()
		endpoint_postfix = strings.TrimSpace(endpoint_postfix)
		if len(endpoint_postfix) > 0 {
			ZookeeperTemplateData_Master = bytes.Replace(
				ZookeeperTemplateData_Master,
				[]byte("endpoint-postfix-place-holder"),
				[]byte(endpoint_postfix),
				-1)
		}
		zookeeper_image := oshandler.ZookeeperImage()
		zookeeper_image = strings.TrimSpace(zookeeper_image)
		if len(zookeeper_image) > 0 {
			ZookeeperTemplateData_Master = bytes.Replace(
				ZookeeperTemplateData_Master,
				[]byte("http://zookeeper-exhibitor-image-place-holder/zookeeper-exhibitor-openshift-orchestration"),
				[]byte(zookeeper_image),
				-1)
		}
	}

	sum := sha1.Sum([]byte(fmt.Sprintf("%s:%s", zookeeperUser, zookeeperPassword)))
	zoo_password := fmt.Sprintf("%s:%s", zookeeperUser, base64.StdEncoding.EncodeToString(sum[:]))

	yamlTemplates := ZookeeperTemplateData_Master

	yamlTemplates = bytes.Replace(yamlTemplates, []byte("instanceid"), []byte(instanceID), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("super:password-place-holder"), []byte(zoo_password), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("local-service-postfix-place-holder"),
		[]byte(serviceBrokerNamespace+oshandler.ServiceDomainSuffix(true)), -1)

	decoder := oshandler.NewYamlDecoder(yamlTemplates)
	decoder.
		Decode(&res.service).
		Decode(&res.svc1).
		Decode(&res.svc2).
		Decode(&res.svc3).
		Decode(&res.rc1).
		Decode(&res.rc2).
		Decode(&res.rc3).
		Decode(&res.route)

	return decoder.Err
}

type ZookeeperResources_Master struct {
	service kapi.Service

	svc1 kapi.Service
	svc2 kapi.Service
	svc3 kapi.Service
	rc1  kapi.ReplicationController
	rc2  kapi.ReplicationController
	rc3  kapi.ReplicationController

	route routeapi.Route
}

func (masterRes *ZookeeperResources_Master) ServiceHostPort(serviceBrokerNamespace string) (string, string, error) {

	client_port := oshandler.GetServicePortByName(&masterRes.service, "client")
	if client_port == nil {
		return "", "", errors.New("client port not found")
	}

	host := fmt.Sprintf("%s.%s.%s", masterRes.service.Name, serviceBrokerNamespace, oshandler.ServiceDomainSuffix(false))
	port := strconv.Itoa(client_port.Port)

	return host, port, nil
}

func CreateZookeeperResources_Master(instanceId, serviceBrokerNamespace, zookeeperUser, zookeeperPassword string) (*ZookeeperResources_Master, error) {
	var input ZookeeperResources_Master
	err := LoadZookeeperResources_Master(instanceId, serviceBrokerNamespace, zookeeperUser, zookeeperPassword, &input)
	if err != nil {
		return nil, err
	}

	var output ZookeeperResources_Master

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	// here, not use job.post
	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		KPost(prefix+"/services", &input.service, &output.service).
		KPost(prefix+"/services", &input.svc1, &output.svc1).
		KPost(prefix+"/services", &input.svc2, &output.svc2).
		KPost(prefix+"/services", &input.svc3, &output.svc3).
		KPost(prefix+"/replicationcontrollers", &input.rc1, &output.rc1).
		KPost(prefix+"/replicationcontrollers", &input.rc2, &output.rc2).
		KPost(prefix+"/replicationcontrollers", &input.rc3, &output.rc3).
		OPost(prefix+"/routes", &input.route, &output.route)

	if osr.Err != nil {
		logger.Error("createZookeeperResources_Master", osr.Err)
	}

	return &output, osr.Err
}

func GetZookeeperResources_Master(instanceId, serviceBrokerNamespace, zookeeperUser, zookeeperPassword string) (*ZookeeperResources_Master, error) {
	var output ZookeeperResources_Master

	var input ZookeeperResources_Master
	err := LoadZookeeperResources_Master(instanceId, serviceBrokerNamespace, zookeeperUser, zookeeperPassword, &input)
	if err != nil {
		return &output, err
	}

	err = getZookeeperResources_Master(serviceBrokerNamespace, &input, &output)
	return &output, err
}

func getZookeeperResources_Master(serviceBrokerNamespace string, input, output *ZookeeperResources_Master) error {
	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		KGet(prefix+"/services/"+input.service.Name, &output.service).
		KGet(prefix+"/services/"+input.svc1.Name, &output.svc1).
		KGet(prefix+"/services/"+input.svc2.Name, &output.svc2).
		KGet(prefix+"/services/"+input.svc3.Name, &output.svc3).
		KGet(prefix+"/replicationcontrollers/"+input.rc1.Name, &output.rc1).
		KGet(prefix+"/replicationcontrollers/"+input.rc2.Name, &output.rc2).
		KGet(prefix+"/replicationcontrollers/"+input.rc3.Name, &output.rc3).
		// old bsi has no route, so get route may be error
		OGet(prefix+"/routes/"+input.route.Name, &output.route)

	if osr.Err != nil {
		logger.Error("getZookeeperResources_Master", osr.Err)
	}

	return osr.Err
}

func DestroyZookeeperResources_Master(masterRes *ZookeeperResources_Master, serviceBrokerNamespace string) {
	// todo: add to retry queue on fail

	go func() { kdel(serviceBrokerNamespace, "services", masterRes.service.Name) }()
	go func() { kdel(serviceBrokerNamespace, "services", masterRes.svc1.Name) }()
	go func() { kdel(serviceBrokerNamespace, "services", masterRes.svc2.Name) }()
	go func() { kdel(serviceBrokerNamespace, "services", masterRes.svc3.Name) }()
	go func() { kdel_rc(serviceBrokerNamespace, &masterRes.rc1) }()
	go func() { kdel_rc(serviceBrokerNamespace, &masterRes.rc2) }()
	go func() { kdel_rc(serviceBrokerNamespace, &masterRes.rc3) }()
	go func() { odel(serviceBrokerNamespace, "routes", masterRes.route.Name) }()
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
		logger.Error("Modify Zookeeper rc", osr.Err)
		return
	}

	// start watching rc status

	statuses, cancel, err := oshandler.OC().KWatch(uri)
	if err != nil {
		logger.Error("Start Watching Zookeeper rc", err)
		return
	}

	go func() {
		for {
			status, _ := <-statuses

			if status.Err != nil {
				logger.Error("Watch Zookeeper rc error", status.Err)
				close(cancel)
				return
			} else {
				//logger.Debug("watch zookeeper HA rc, status.Info: " + string(status.Info))
			}

			var wrcs watchReplicationControllerStatus
			if err := json.Unmarshal(status.Info, &wrcs); err != nil {
				logger.Error("Parse Master Zookeeper rc status", err)
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
