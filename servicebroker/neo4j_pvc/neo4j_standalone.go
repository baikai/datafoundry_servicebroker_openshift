package neo4j_pvc

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
	"net"
	"strconv"
	"strings"
	"time"

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

const Neo4jServcieBrokerName_Standalone = "Neo4j_volumes_standalone"

func init() {
	oshandler.Register(Neo4jServcieBrokerName_Standalone, &Neo4j_freeHandler{})

	logger = lager.NewLogger(Neo4jServcieBrokerName_Standalone)
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))
}

var logger lager.Logger

//==============================================================
//
//==============================================================

type Neo4j_freeHandler struct{}

func (handler *Neo4j_freeHandler) DoProvision(etcdSaveResult chan error, instanceID string, details brokerapi.ProvisionDetails, planInfo oshandler.PlanInfo, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, oshandler.ServiceInfo, error) {
	return newNeo4jHandler().DoProvision(etcdSaveResult, instanceID, details, planInfo, asyncAllowed)
}

func (handler *Neo4j_freeHandler) DoLastOperation(myServiceInfo *oshandler.ServiceInfo) (brokerapi.LastOperation, error) {
	return newNeo4jHandler().DoLastOperation(myServiceInfo)
}

func (handler *Neo4j_freeHandler) DoUpdate(myServiceInfo *oshandler.ServiceInfo, planInfo oshandler.PlanInfo, callbackSaveNewInfo func(*oshandler.ServiceInfo) error, asyncAllowed bool) error {
	return newNeo4jHandler().DoUpdate(myServiceInfo, planInfo, callbackSaveNewInfo, asyncAllowed)
}

func (handler *Neo4j_freeHandler) DoDeprovision(myServiceInfo *oshandler.ServiceInfo, asyncAllowed bool) (brokerapi.IsAsync, error) {
	return newNeo4jHandler().DoDeprovision(myServiceInfo, asyncAllowed)
}

func (handler *Neo4j_freeHandler) DoBind(myServiceInfo *oshandler.ServiceInfo, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, oshandler.Credentials, error) {
	return newNeo4jHandler().DoBind(myServiceInfo, bindingID, details)
}

func (handler *Neo4j_freeHandler) DoUnbind(myServiceInfo *oshandler.ServiceInfo, mycredentials *oshandler.Credentials) error {
	return newNeo4jHandler().DoUnbind(myServiceInfo, mycredentials)
}

//==============================================================
//
//==============================================================

// version 1:
//   one peer volume,

func volumeBaseName(instanceId string) string {
	return "neo-" + instanceId
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

type Neo4j_Handler struct {
}

func newNeo4jHandler() *Neo4j_Handler {
	return &Neo4j_Handler{}
}

func (handler *Neo4j_Handler) DoProvision(etcdSaveResult chan error, instanceID string, details brokerapi.ProvisionDetails, planInfo oshandler.PlanInfo, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, oshandler.ServiceInfo, error) {
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
	neo4jUser := "neo4j"
	neo4jPassword := oshandler.GenGUID()

	/*
		finalVolumeSize, err := getVolumeSize(details, planInfo)
		if err != nil {
			return serviceSpec, serviceInfo, err
		}*/

	volumeBaseName := volumeBaseName(instanceIdInTempalte)
	volumes := []oshandler.Volume{
		// one peer volume
		{
			Volume_size: planInfo.Volume_size,
			Volume_name: volumeBaseName + "-0",
		},
	}

	println()
	println("instanceIdInTempalte = ", instanceIdInTempalte)
	println("serviceBrokerNamespace = ", serviceBrokerNamespace)
	println()

	// master neo4j

	serviceInfo.Url = instanceIdInTempalte
	serviceInfo.Database = serviceBrokerNamespace // may be not needed
	serviceInfo.User = neo4jUser
	serviceInfo.Password = neo4jPassword //NEO4J_AUTH

	serviceInfo.Volumes = volumes

	//>> may be not optimized
	var template neo4jResources_Master
	err := loadNeo4jResources_Master(
		serviceInfo.Url,
		serviceInfo.User,
		serviceInfo.Password,
		serviceInfo.Volumes,
		&template)
	if err != nil {
		return serviceSpec, oshandler.ServiceInfo{}, err
	}
	//<<

	nodePort, err := createNeo4jResources_NodePort(
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
			logger.Error("neo4j create volume", err)
			go func() { kdel(serviceBrokerNamespace, "services", nodePort.servicebolt.Name) }()
			oshandler.DeleteVolumns(serviceInfo.Database, serviceInfo.Volumes)
			return
		}

		println("createNeo4jResources_Master ...")

		// create master res

		output, err := createNeo4jResources_Master(
			serviceInfo.Url,
			serviceInfo.Database,
			serviceInfo.User,
			serviceInfo.Password,
			serviceInfo.Volumes,
		)
		if err != nil {
			println(" neo4j createNeo4jResources_Master error: ", err)
			logger.Error("neo4j createNeo4jResources_Master error", err)

			destroyNeo4jResources_Master(output, serviceBrokerNamespace)
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

/*func getVolumeSize(details brokerapi.ProvisionDetails, planInfo oshandler.PlanInfo) (finalVolumeSize int, err error) {
	if planInfo.Customize == nil {
		finalVolumeSize = planInfo.Volume_size
	} else if cus, ok := planInfo.Customize[G_VolumeSize]; ok {
		if details.Parameters == nil {
			finalVolumeSize = int(cus.Default)
			return
		}
		if _, ok := details.Parameters[G_VolumeSize]; !ok {
			err = errors.New("getVolumeSize:idetails.Parameters[volumeSize] not exist")
			println(err)
			return
		}
		sSize, ok := details.Parameters[G_VolumeSize].(string)
		if !ok {
			err = errors.New("getVolumeSize:idetails.Parameters[volumeSize] cannot be converted to string")
			println(err)
			return
		}
		fSize, e := strconv.ParseFloat(sSize, 64)
		if e != nil {
			println("getVolumeSize: input parameter volumeSize :", sSize, e)
			err = e
			return
		}
		if fSize > cus.Max {
			finalVolumeSize = int(cus.Default)
		} else {
			finalVolumeSize = int(cus.Default + cus.Step*math.Ceil((fSize-cus.Default)/cus.Step))
		}
	} else {
		finalVolumeSize = planInfo.Volume_size
	}

	return
}
*/

func (handler *Neo4j_Handler) DoLastOperation(myServiceInfo *oshandler.ServiceInfo) (brokerapi.LastOperation, error) {

	// assume in provisioning

	volumeJob := oshandler.GetCreatePvcVolumnJob(volumeBaseName(myServiceInfo.Url))
	if volumeJob != nil {
		return brokerapi.LastOperation{
			State:       brokerapi.InProgress,
			Description: "in progress.",
		}, nil
	}

	// the job may be finished or interrupted or running in another instance.

	master_res, _ := getNeo4jResources_Master(
		myServiceInfo.Url,
		myServiceInfo.Database,
		myServiceInfo.User,
		myServiceInfo.Password,
		myServiceInfo.Volumes,
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

func (handler *Neo4j_Handler) DoUpdate(myServiceInfo *oshandler.ServiceInfo, planInfo oshandler.PlanInfo, callbackSaveNewInfo func(*oshandler.ServiceInfo) error, asyncAllowed bool) error {
	go func() {
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
			logger.Error("neo4j expand volume error", err)
			return
		}

		println("neo4j expand volumens done")

		for i := range myServiceInfo.Volumes {
			myServiceInfo.Volumes[i].Volume_size = planInfo.Volume_size
		}
		err = callbackSaveNewInfo(myServiceInfo)
		if err != nil {
			logger.Error("neo4j expand volume succeeded but save info error", err)
		}
	}()
	return nil
}

func (handler *Neo4j_Handler) DoDeprovision(myServiceInfo *oshandler.ServiceInfo, asyncAllowed bool) (brokerapi.IsAsync, error) {
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

		master_res, _ := getNeo4jResources_Master(
			myServiceInfo.Url,
			myServiceInfo.Database,
			myServiceInfo.User,
			myServiceInfo.Password,
			myServiceInfo.Volumes,
		)
		destroyNeo4jResources_Master(master_res, myServiceInfo.Database)

		// ...

		println("to destroy volumes:", myServiceInfo.Volumes)

		oshandler.DeleteVolumns(myServiceInfo.Database, myServiceInfo.Volumes)
	}()

	return brokerapi.IsAsync(false), nil
}

// please note: the bsi may be still not fully initialized when calling the function.
func getCredentialsOnPrivision(myServiceInfo *oshandler.ServiceInfo, nodePort *neo4jResources_Master) oshandler.Credentials {
	var master_res neo4jResources_Master
	err := loadNeo4jResources_Master(myServiceInfo.Url, myServiceInfo.User, myServiceInfo.Password, myServiceInfo.Volumes, &master_res)
	if err != nil {
		return oshandler.Credentials{}
	}

	http_port := oshandler.GetServicePortByName(&master_res.service, "neo4j-http-port")
	if http_port == nil {
		return oshandler.Credentials{}
	}

	//host := fmt.Sprintf("%s.%s.%s", master_res.service.Name, myServiceInfo.Database, oshandler.ServiceDomainSuffix(false))
	//port := strconv.Itoa(http_port.Port)
	//host := master_res.routeMQ.Spec.Host
	//port := "80"
	host := oshandler.RandomNodeAddress()
	var port string = ""
	if nodePort != nil && len(nodePort.servicebolt.Spec.Ports) > 0 {
		port = strconv.Itoa(nodePort.servicebolt.Spec.Ports[0].NodePort)
	}

	return oshandler.Credentials{
		Uri:      fmt.Sprintf("bolt://%s:%s@%s:%s", myServiceInfo.User, myServiceInfo.Password, host, port),
		Hostname: host,
		Port:     port,
		Username: myServiceInfo.User,
		Password: myServiceInfo.Password,
		//Vhost:    master_res.routeAdmin.Spec.Host,
		Vhost: fmt.Sprintf("%s.%s.%s", master_res.service.Name, myServiceInfo.Database, oshandler.ServiceDomainSuffix(false)),
	}
}

func (handler *Neo4j_Handler) DoBind(myServiceInfo *oshandler.ServiceInfo, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, oshandler.Credentials, error) {
	// todo: handle errors

	master_res, err := getNeo4jResources_Master(
		myServiceInfo.Url,
		myServiceInfo.Database,
		myServiceInfo.User,
		myServiceInfo.Password,
		myServiceInfo.Volumes,
	)
	if err != nil {
		return brokerapi.Binding{}, oshandler.Credentials{}, err
	}

	http_port := oshandler.GetServicePortByName(&master_res.service, "neo4j-http-port")
	if http_port == nil {
		return brokerapi.Binding{}, oshandler.Credentials{}, errors.New("neo4j-http-port not found")
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

func (handler *Neo4j_Handler) DoUnbind(myServiceInfo *oshandler.ServiceInfo, mycredentials *oshandler.Credentials) error {
	// do nothing

	return nil
}

//=======================================================================
//
//=======================================================================

var Neo4jTemplateData_Master []byte = nil

func loadNeo4jResources_Master(instanceID, neo4jUser, neo4jPassword string, volumes []oshandler.Volume, res *neo4jResources_Master) error {
	if Neo4jTemplateData_Master == nil {
		f, err := os.Open("neo4j-pvc.yaml")
		if err != nil {
			return err
		}
		Neo4jTemplateData_Master, err = ioutil.ReadAll(f)
		if err != nil {
			return err
		}
		neo4jVolumeImage := oshandler.Neo4jVolumeImage()
		neo4jVolumeImage = strings.TrimSpace(neo4jVolumeImage)
		if len(neo4jVolumeImage) > 0 {
			Neo4jTemplateData_Master = bytes.Replace(
				Neo4jTemplateData_Master,
				[]byte("http://neo4j-image-place-holder/neo4j-openshift-orchestration"),
				[]byte(neo4jVolumeImage),
				-1)
		}
		endpoint_postfix := oshandler.EndPointSuffix()
		endpoint_postfix = strings.TrimSpace(endpoint_postfix)
		if len(endpoint_postfix) > 0 {
			Neo4jTemplateData_Master = bytes.Replace(
				Neo4jTemplateData_Master,
				[]byte("endpoint-postfix-place-holder"),
				[]byte(endpoint_postfix),
				-1)
		}
	}

	// ...
	peerPvcName0 := peerPvcName0(volumes)

	yamlTemplates := Neo4jTemplateData_Master

	yamlTemplates = bytes.Replace(yamlTemplates, []byte("instanceid"), []byte(instanceID), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("user*****"), []byte(neo4jUser), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("pass*****"), []byte(neo4jPassword), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("neo4jauth"),
		[]byte(neo4jUser+"/"+neo4jPassword), -1)

	yamlTemplates = bytes.Replace(yamlTemplates, []byte("pvcname*****node"), []byte(peerPvcName0), -1)

	//println("========= Boot yamlTemplates ===========")
	//println(string(yamlTemplates))
	//println()

	decoder := oshandler.NewYamlDecoder(yamlTemplates)
	decoder.
		Decode(&res.rc).
		Decode(&res.routeAdmin).
		//Decode(&res.routeMQ).
		Decode(&res.service).
		Decode(&res.servicebolt)

	return decoder.Err
}

type neo4jResources_Master struct {
	rc         kapi.ReplicationController
	routeAdmin routeapi.Route
	//routeMQ    routeapi.Route
	service     kapi.Service
	servicebolt kapi.Service
}

func createNeo4jResources_Master(instanceId, serviceBrokerNamespace, neo4jUser, neo4jPassword string, volumes []oshandler.Volume) (*neo4jResources_Master, error) {
	var input neo4jResources_Master
	err := loadNeo4jResources_Master(instanceId, neo4jUser, neo4jPassword, volumes, &input)
	if err != nil {
		return nil, err
	}

	var output neo4jResources_Master

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	// here, not use job.post
	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		KPost(prefix+"/replicationcontrollers", &input.rc, &output.rc).
		OPost(prefix+"/routes", &input.routeAdmin, &output.routeAdmin).
		//OPost(prefix + "/routes", &input.routeMQ, &output.routeMQ).
		KPost(prefix+"/services", &input.service, &output.service)
		//KPost(prefix+"/services", &input.servicebolt, &output.servicebolt)

	if osr.Err != nil {
		logger.Error("createNeo4jResources_Master", osr.Err)
	}

	return &output, osr.Err
}

func createNeo4jResources_NodePort(input *neo4jResources_Master, serviceBrokerNamespace string) (*neo4jResources_Master, error) {
	var output neo4jResources_Master

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	// here, not use job.post
	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.KPost(prefix+"/services", &input.servicebolt, &output.servicebolt)

	if osr.Err != nil {
		logger.Error("createNeo4jResources_NodePort", osr.Err)
	}

	return &output, osr.Err
}

func getNeo4jResources_Master(instanceId, serviceBrokerNamespace, neo4jUser, neo4jPassword string, volumes []oshandler.Volume) (*neo4jResources_Master, error) {
	var output neo4jResources_Master

	var input neo4jResources_Master
	err := loadNeo4jResources_Master(instanceId, neo4jUser, neo4jPassword, volumes, &input)
	if err != nil {
		return &output, err
	}

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		KGet(prefix+"/replicationcontrollers/"+input.rc.Name, &output.rc).
		OGet(prefix+"/routes/"+input.routeAdmin.Name, &output.routeAdmin).
		//OGet(prefix + "/routes/" + input.routeMQ.Name, &output.routeMQ).
		KGet(prefix+"/services/"+input.service.Name, &output.service).
		KGet(prefix+"/services/"+input.servicebolt.Name, &output.servicebolt)

	if osr.Err != nil {
		logger.Error("getNeo4jResources_Master", osr.Err)
	}

	return &output, osr.Err
}

func destroyNeo4jResources_Master(masterRes *neo4jResources_Master, serviceBrokerNamespace string) {
	// todo: add to retry queue on fail

	//go func() { kdel_rc(serviceBrokerNamespace, &masterRes.rc) }()
	//go func() { odel(serviceBrokerNamespace, "routes", masterRes.routeAdmin.Name) }()
	////go func() {odel (serviceBrokerNamespace, "routes", masterRes.routeMQ.Name)}()
	//go func() { kdel(serviceBrokerNamespace, "services", masterRes.service.Name) }()
	//go func() { kdel(serviceBrokerNamespace, "services", masterRes.servicebolt.Name) }()

	// change to synced to avoid deleting behind volumes.
	kdel_rc(serviceBrokerNamespace, &masterRes.rc)
	odel(serviceBrokerNamespace, "routes", masterRes.routeAdmin.Name)
	//odel (serviceBrokerNamespace, "routes", masterRes.routeMQ.Name)
	kdel(serviceBrokerNamespace, "services", masterRes.service.Name)
	kdel(serviceBrokerNamespace, "services", masterRes.servicebolt.Name)
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
				logger.Error("watch HA neo4j rc error", status.Err)
				close(cancel)
				return
			} else {
				//logger.Debug("watch neo4j HA rc, status.Info: " + string(status.Info))
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
