package mongo_pvc

import (
	"errors"
	"fmt"
	//marathon "github.com/gambol99/go-marathon"
	//kapi "golang.org/x/build/kubernetes/api"
	//"golang.org/x/build/kubernetes"
	//"golang.org/x/oauth2"
	//"net/http"
	//"net"
	"bytes"
	"encoding/json"
	"github.com/pivotal-cf/brokerapi"
	"strconv"
	"strings"
	"time"
	//"text/template"
	//"io"
	"io/ioutil"
	"os"
	//"sync"

	"github.com/pivotal-golang/lager"

	//"k8s.io/kubernetes/pkg/util/yaml"
	kapi "k8s.io/kubernetes/pkg/api/v1"
	//routeapi "github.com/openshift/origin/route/api/v1"

	oshandler "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/handler"
)

//==============================================================
//
//==============================================================

const MongoServcieBrokerName_Standalone = "Mongo_volumes_standalone"

func init() {
	oshandler.Register(MongoServcieBrokerName_Standalone, &Mongo_freeHandler{})

	logger = lager.NewLogger(MongoServcieBrokerName_Standalone)
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))
}

var logger lager.Logger

//==============================================================
//
//==============================================================

type Mongo_freeHandler struct{}

func (handler *Mongo_freeHandler) DoProvision(etcdSaveResult chan error, instanceID string, details brokerapi.ProvisionDetails, planInfo oshandler.PlanInfo, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, oshandler.ServiceInfo, error) {
	return newMongoHandler().DoProvision(etcdSaveResult, instanceID, details, planInfo, asyncAllowed)
}

func (handler *Mongo_freeHandler) DoLastOperation(myServiceInfo *oshandler.ServiceInfo) (brokerapi.LastOperation, error) {
	return newMongoHandler().DoLastOperation(myServiceInfo)
}

func (handler *Mongo_freeHandler) DoUpdate(myServiceInfo *oshandler.ServiceInfo, planInfo oshandler.PlanInfo, callbackSaveNewInfo func(*oshandler.ServiceInfo) error, asyncAllowed bool) error {
	return newMongoHandler().DoUpdate(myServiceInfo, planInfo, callbackSaveNewInfo, asyncAllowed)
}

func (handler *Mongo_freeHandler) DoDeprovision(myServiceInfo *oshandler.ServiceInfo, asyncAllowed bool) (brokerapi.IsAsync, error) {
	return newMongoHandler().DoDeprovision(myServiceInfo, asyncAllowed)
}

func (handler *Mongo_freeHandler) DoBind(myServiceInfo *oshandler.ServiceInfo, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, oshandler.Credentials, error) {
	return newMongoHandler().DoBind(myServiceInfo, bindingID, details)
}

func (handler *Mongo_freeHandler) DoUnbind(myServiceInfo *oshandler.ServiceInfo, mycredentials *oshandler.Credentials) error {
	return newMongoHandler().DoUnbind(myServiceInfo, mycredentials)
}

//==============================================================
//
//==============================================================

func volumeBaseName(instanceId string) string {
	return "mng-" + instanceId
}

func nodePvcName0(volumes []oshandler.Volume) string {
	if len(volumes) > 0 {
		return volumes[0].Volume_name
	}
	return ""
}

func nodePvcName1(volumes []oshandler.Volume) string {
	if len(volumes) > 1 {
		return volumes[1].Volume_name
	}
	return ""
}

func nodePvcName2(volumes []oshandler.Volume) string {
	if len(volumes) > 2 {
		return volumes[2].Volume_name
	}
	return ""
}

//==============================================================
//
//==============================================================

type Mongo_Handler struct {
}

func newMongoHandler() *Mongo_Handler {
	return &Mongo_Handler{}
}

func (handler *Mongo_Handler) DoProvision(etcdSaveResult chan error, instanceID string, details brokerapi.ProvisionDetails, planInfo oshandler.PlanInfo, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, oshandler.ServiceInfo, error) {
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
	mongoUser := "admin" // oshandler.NewElevenLengthID()
	mongoPassword := oshandler.GenGUID()

	volumeBaseName := volumeBaseName(instanceIdInTempalte)
	volumes := []oshandler.Volume{
		// one master volume
		{
			Volume_size: planInfo.Volume_size,
			Volume_name: volumeBaseName + "-0",
		},
		// two slave volumes
		{
			Volume_size: planInfo.Volume_size,
			Volume_name: volumeBaseName + "-1",
		},
		{
			Volume_size: planInfo.Volume_size,
			Volume_name: volumeBaseName + "-2",
		},
	}

	println()
	println("instanceIdInTempalte = ", instanceIdInTempalte)
	println("serviceBrokerNamespace = ", serviceBrokerNamespace)
	println()

	serviceInfo.Url = instanceIdInTempalte
	serviceInfo.Database = serviceBrokerNamespace // may be not needed
	serviceInfo.User = mongoUser
	serviceInfo.Password = mongoPassword

	serviceInfo.Volumes = volumes

	// ...
	go func() {
		err := <-etcdSaveResult
		if err != nil {
			return
		}

		// create volume
		result := oshandler.StartCreatePvcVolumnJob(
			volumeBaseName,
			serviceInfo.Database,
			serviceInfo.Volumes,
		)

		err = <-result
		if err != nil {
			logger.Error("mongo create volume", err)
			handler.DoDeprovision(&serviceInfo, true)
			return
		}

		println("createMongoResources_Master ...")

		// todo: consider if DoDeprovision is called now, ...

		// create master res

		output, err := CreateMongoResources_Master(
			serviceInfo.Url,
			serviceInfo.Database,
			serviceInfo.User,
			serviceInfo.Password,
			volumes,
		)

		if err != nil {
			println(" mongo createMongoResources_Master error: ", err)
			logger.Error("mongo createMongoResources_Master error", err)

			DestroyMongoResources_Master(output, serviceBrokerNamespace)
			oshandler.DeleteVolumns(serviceInfo.Database, volumes)

			return
		}
	}()

	serviceSpec.DashboardURL = "" // "http://" + net.JoinHostPort(output.route.Spec.Host, "80")

	//>>>
	serviceSpec.Credentials = getCredentialsOnPrivision(&serviceInfo)
	//<<<

	return serviceSpec, serviceInfo, nil
}

/*
func (handler *Mongo_Handler) DoProvision(instanceID string, details brokerapi.ProvisionDetails, planInfo oshandler.PlanInfo, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, oshandler.ServiceInfo, error) {
	//初始化到openshift的链接

	serviceSpec := brokerapi.ProvisionedServiceSpec{IsAsync: asyncAllowed}
	serviceInfo := oshandler.ServiceInfo{}

	//if asyncAllowed == false {
	//	return serviceSpec, serviceInfo, errors.New("Sync mode is not supported")
	//}
	serviceSpec.IsAsync = true

	//instanceIdInTempalte   := instanceID // todo: ok?
	instanceIdInTempalte   := strings.ToLower(oshandler.NewThirteenLengthID())
	//serviceBrokerNamespace := ServiceBrokerNamespace
	serviceBrokerNamespace := oshandler.OC().Namespace()
	mongoUser := "super" // oshandler.NewElevenLengthID()
	mongoPassword := oshandler.GenGUID()

	println()
	println("instanceIdInTempalte = ", instanceIdInTempalte)
	println("serviceBrokerNamespace = ", serviceBrokerNamespace)
	println()

	// master mongo

	output, err := CreateMongoResources_Master(instanceIdInTempalte, serviceBrokerNamespace, mongoUser, mongoPassword)

	if err != nil {
		DestroyMongoResources_Master(output, serviceBrokerNamespace)

		return serviceSpec, serviceInfo, err
	}

	serviceInfo.Url = instanceIdInTempalte
	serviceInfo.Database = serviceBrokerNamespace // may be not needed
	serviceInfo.User = mongoUser
	serviceInfo.Password = mongoPassword

	serviceSpec.DashboardURL = "" // "http://" + net.JoinHostPort(output.route.Spec.Host, "80")

	return serviceSpec, serviceInfo, nil
}
*/

func (handler *Mongo_Handler) DoLastOperation(myServiceInfo *oshandler.ServiceInfo) (brokerapi.LastOperation, error) {

	// assume in provisioning

	volumeJob := oshandler.GetCreatePvcVolumnJob(volumeBaseName(myServiceInfo.Url))
	if volumeJob != nil {
		return brokerapi.LastOperation{
			State:       brokerapi.InProgress,
			Description: "in progress.",
		}, nil
	}

	// ...

	master_res, _ := GetMongoResources_Master(myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.User, myServiceInfo.Password, myServiceInfo.Volumes)

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

func (handler *Mongo_Handler) DoUpdate(myServiceInfo *oshandler.ServiceInfo, planInfo oshandler.PlanInfo, callbackSaveNewInfo func(*oshandler.ServiceInfo) error, asyncAllowed bool) error {
	return errors.New("not implemented")
}

func (handler *Mongo_Handler) DoDeprovision(myServiceInfo *oshandler.ServiceInfo, asyncAllowed bool) (brokerapi.IsAsync, error) {
	// ...

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

		println("to destroy master resources")

		master_res, _ := GetMongoResources_Master(myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.User, myServiceInfo.Password, myServiceInfo.Volumes)
		// under current frame, it is not a good idea to return here
		//if err != nil {
		//	return brokerapi.IsAsync(false), err
		//}
		DestroyMongoResources_Master(master_res, myServiceInfo.Database)

		println("to destroy volumes:", myServiceInfo.Volumes)

		oshandler.DeleteVolumns(myServiceInfo.Database, myServiceInfo.Volumes)
	}()

	return brokerapi.IsAsync(false), nil
}

// please note: the bsi may be still not fully initialized when calling the function.
func getCredentialsOnPrivision(myServiceInfo *oshandler.ServiceInfo) oshandler.Credentials {
	var master_res MongoResources_Master
	err := loadMongoResources_Master(myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.User, myServiceInfo.Password, myServiceInfo.Volumes, &master_res)
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

func (handler *Mongo_Handler) DoBind(myServiceInfo *oshandler.ServiceInfo, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, oshandler.Credentials, error) {
	// todo: handle errors

	master_res, _ := GetMongoResources_Master(myServiceInfo.Url, myServiceInfo.Database, myServiceInfo.User, myServiceInfo.Password, myServiceInfo.Volumes)

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

func (handler *Mongo_Handler) DoUnbind(myServiceInfo *oshandler.ServiceInfo, mycredentials *oshandler.Credentials) error {
	// do nothing

	return nil
}

//=======================================================================
// the mongo functions may be called by outer packages
//=======================================================================

var MongoTemplateData_Master []byte = nil

func loadMongoResources_Master(instanceID, serviceBrokerNamespace, mongoUser, mongoPassword string, volumes []oshandler.Volume, res *MongoResources_Master) error {
	/*
		if MongoTemplateData_Master == nil {
			f, err := os.Open("mongo.yaml")
			if err != nil {
				return err
			}
			MongoTemplateData_Master, err = ioutil.ReadAll(f)
			if err != nil {
				return err
			}
			mongo_image := oshandler.MongoImage()
			mongo_image = strings.TrimSpace(mongo_image)
			if len(mongo_image) > 0 {
				MongoTemplateData_Master = bytes.Replace(
					MongoTemplateData_Master,
					[]byte("http://mongo-image-place-holder/mongo-openshift-orchestration"),
					[]byte(mongo_image),
					-1)
			}
		}
	*/

	if MongoTemplateData_Master == nil {

		f, err := os.Open("mongo-pvc.yaml")
		if err != nil {
			return err
		}
		MongoTemplateData_Master, err = ioutil.ReadAll(f)
		if err != nil {
			return err
		}
		endpoint_postfix := oshandler.EndPointSuffix()
		endpoint_postfix = strings.TrimSpace(endpoint_postfix)
		if len(endpoint_postfix) > 0 {
			MongoTemplateData_Master = bytes.Replace(
				MongoTemplateData_Master,
				[]byte("endpoint-postfix-place-holder"),
				[]byte(endpoint_postfix),
				-1)
		}
		//mongo_image := oshandler.MongoExhibitorImage()
		mongo_image := oshandler.MongoVolumeImage()
		mongo_image = strings.TrimSpace(mongo_image)
		if len(mongo_image) > 0 {
			MongoTemplateData_Master = bytes.Replace(
				MongoTemplateData_Master,
				[]byte("http://mongo-image-place-holder/mongo-with-volumes-orchestration"),
				[]byte(mongo_image),
				-1)
		}
	}

	// ...

	// ...

	nodePvcName0 := nodePvcName0(volumes)
	nodePvcName1 := nodePvcName1(volumes)
	nodePvcName2 := nodePvcName2(volumes)

	yamlTemplates := MongoTemplateData_Master

	yamlTemplates = bytes.Replace(yamlTemplates, []byte("instanceid"), []byte(instanceID), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("#ADMINUSER#"), []byte(mongoUser), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("#ADMINPASSWORD#"), []byte(mongoPassword), -1)
	//yamlTemplates = bytes.Replace(yamlTemplates, []byte("local-service-postfix-place-holder"), []byte(serviceBrokerNamespace + oshandler.ServiceDomainSuffix(true)), -1)

	yamlTemplates = bytes.Replace(yamlTemplates, []byte("pvcname*****node0"), []byte(nodePvcName0), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("pvcname*****node1"), []byte(nodePvcName1), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("pvcname*****node2"), []byte(nodePvcName2), -1)

	//println("========= Boot yamlTemplates ===========")
	//println(string(yamlTemplates))
	//println()

	decoder := oshandler.NewYamlDecoder(yamlTemplates)
	decoder.
		Decode(&res.svc1).
		Decode(&res.svc2).
		Decode(&res.svc3).
		Decode(&res.rc1).
		Decode(&res.rc2).
		Decode(&res.rc3)

	return decoder.Err
}

type MongoResources_Master struct {
	svc1 kapi.Service
	svc2 kapi.Service
	svc3 kapi.Service
	rc1  kapi.ReplicationController
	rc2  kapi.ReplicationController
	rc3  kapi.ReplicationController
}

func (masterRes *MongoResources_Master) ServiceHostPort(serviceBrokerNamespace string) (string, string, error) {

	client_port := oshandler.GetServicePortByName(&masterRes.svc1, "mongo-svc-port")
	if client_port == nil {
		return "", "", errors.New("mongo-svc-port port not found")
	}

	// assusme client_port are the same for all nodes

	port := strconv.Itoa(client_port.Port)

	postfix := fmt.Sprintf("%s.%s:%s", serviceBrokerNamespace, oshandler.ServiceDomainSuffix(false), port)

	host := fmt.Sprintf("%s.%s;%s.%s;%s.%s",
		masterRes.svc1.Name, postfix,
		masterRes.svc2.Name, postfix,
		masterRes.svc3.Name, postfix)

	return host, port, nil
}

func CreateMongoResources_Master(instanceId, serviceBrokerNamespace, mongoUser, mongoPassword string, volumes []oshandler.Volume) (*MongoResources_Master, error) {
	var input MongoResources_Master
	err := loadMongoResources_Master(instanceId, serviceBrokerNamespace, mongoUser, mongoPassword, volumes, &input)
	if err != nil {
		return nil, err
	}

	var output MongoResources_Master

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	// here, not use job.post
	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		KPost(prefix+"/services", &input.svc1, &output.svc1).
		KPost(prefix+"/services", &input.svc2, &output.svc2).
		KPost(prefix+"/services", &input.svc3, &output.svc3).
		KPost(prefix+"/replicationcontrollers", &input.rc1, &output.rc1).
		KPost(prefix+"/replicationcontrollers", &input.rc2, &output.rc2).
		KPost(prefix+"/replicationcontrollers", &input.rc3, &output.rc3)

	if osr.Err != nil {
		logger.Error("createMongoResources_Master", osr.Err)
	}

	return &output, osr.Err
}

func GetMongoResources_Master(instanceId, serviceBrokerNamespace, mongoUser, mongoPassword string, volumes []oshandler.Volume) (*MongoResources_Master, error) {
	var output MongoResources_Master

	var input MongoResources_Master
	err := loadMongoResources_Master(instanceId, serviceBrokerNamespace, mongoUser, mongoPassword, volumes, &input)
	if err != nil {
		return &output, err
	}

	err = getMongoResources_Master(serviceBrokerNamespace, &input, &output)
	return &output, err
}

func getMongoResources_Master(serviceBrokerNamespace string, input, output *MongoResources_Master) error {
	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		KGet(prefix+"/services/"+input.svc1.Name, &output.svc1).
		KGet(prefix+"/services/"+input.svc2.Name, &output.svc2).
		KGet(prefix+"/services/"+input.svc3.Name, &output.svc3).
		KGet(prefix+"/replicationcontrollers/"+input.rc1.Name, &output.rc1).
		KGet(prefix+"/replicationcontrollers/"+input.rc2.Name, &output.rc2).
		KGet(prefix+"/replicationcontrollers/"+input.rc3.Name, &output.rc3)

	if osr.Err != nil {
		logger.Error("getMongoResources_Master", osr.Err)
	}

	return osr.Err
}

func DestroyMongoResources_Master(masterRes *MongoResources_Master, serviceBrokerNamespace string) {
	// todo: add to retry queue on fail

	go func() { kdel(serviceBrokerNamespace, "services", masterRes.svc1.Name) }()
	go func() { kdel(serviceBrokerNamespace, "services", masterRes.svc2.Name) }()
	go func() { kdel(serviceBrokerNamespace, "services", masterRes.svc3.Name) }()
	go func() { kdel_rc(serviceBrokerNamespace, &masterRes.rc1) }()
	go func() { kdel_rc(serviceBrokerNamespace, &masterRes.rc2) }()
	go func() { kdel_rc(serviceBrokerNamespace, &masterRes.rc3) }()
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
				logger.Error("watch HA mongo rc error", status.Err)
				close(cancel)
				return
			} else {
				//logger.Debug("watch mongo HA rc, status.Info: " + string(status.Info))
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

// https://hub.docker.com/r/mbabineau/mongo-exhibitor/
// https://hub.docker.com/r/netflixoss/exhibitor/

// todo:
// set ACL: https://godoc.org/github.com/samuel/go-mongo/zk#Conn.SetACL
// github.com/samuel/go-mongo/zk

/*
bin/zkCli.sh 127.0.0.1:2181
bin/zkCli.sh -server sb-instanceid-zk:2181

echo conf|nc localhost 2181
echo cons|nc localhost 2181
echo ruok|nc localhost 2181
echo srst|nc localhost 2181
echo crst|nc localhost 2181
echo dump|nc localhost 2181
echo srvr|nc localhost 2181
echo stat|nc localhost 2181
echo mntr|nc localhost 2181
*/

/* need this?

# zoo.cfg

# Enable regular purging of old data and transaction logs every 24 hours
autopurge.purgeInterval=24
autopurge.snapRetainCount=5

The last two autopurge.* settings are very important for production systems.
They instruct Mongo to regularly remove (old) data and transaction logs.
The default Mongo configuration does not do this on its own,
and if you do not set up regular purging Mongo will quickly run out of disk space.

*/
