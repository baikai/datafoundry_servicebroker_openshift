package etcd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	oshandler "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/handler"
	dcapi "github.com/openshift/origin/deploy/api/v1"
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
//
//==============================================================

const EtcdServcieBrokerName_Volume_Standalone = "Elasticsearch_volumes_standalone"

func init() {
	oshandler.Register(EtcdServcieBrokerName_Volume_Standalone, &Elasticsearch_handler{})

	logger = lager.NewLogger(EtcdServcieBrokerName_Volume_Standalone)
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))
}

var logger lager.Logger

func volumeBaseName(instanceId string) string {
	return "elasticsearch-" + instanceId
}

func peerPvcName0(volumes []oshandler.Volume) string {
	if len(volumes) > 0 {
		return volumes[0].Volume_name
	}
	return ""
}

func peerPvcName1(volumes []oshandler.Volume) string {
	if len(volumes) > 1 {
		return volumes[1].Volume_name
	}
	return ""
}

func peerPvcName2(volumes []oshandler.Volume) string {
	if len(volumes) > 2 {
		return volumes[2].Volume_name
	}
	return ""
}

//==============================================================
//
//==============================================================

//const ServiceBrokerNamespace = "default" // use oshandler.OC().Namespace instead

type Elasticsearch_handler struct{}

func (handler *Elasticsearch_handler) DoProvision(etcdSaveResult chan error, instanceID string, details brokerapi.ProvisionDetails, planInfo oshandler.PlanInfo, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, oshandler.ServiceInfo, error) {
	//初始化到openshift的链接

	serviceSpec := brokerapi.ProvisionedServiceSpec{IsAsync: asyncAllowed}
	serviceInfo := oshandler.ServiceInfo{}

	//if asyncAllowed == false {
	//	return serviceSpec, serviceInfo, errors.New("Sync mode is not supported")
	//}
	serviceSpec.IsAsync = true

	instanceIdInTempalte := strings.ToLower(oshandler.NewThirteenLengthID())
	serviceBrokerNamespace := oshandler.OC().Namespace()

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

	// boot etcd

	serviceInfo.Url = instanceIdInTempalte
	serviceInfo.Database = serviceBrokerNamespace // may be not needed

	serviceInfo.Volumes = volumes

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
			logger.Error("elasticsearch create volume", err)
			oshandler.DeleteVolumns(serviceInfo.Database, serviceInfo.Volumes)
			return
		}

		println("create Elasticsearch Resources ...")

		// todo: consider if DoDeprovision is called now, ...

		// create master res

		output, err := createESResources_HA(
			instanceIdInTempalte, serviceBrokerNamespace, volumes)

		if err != nil {
			println("etcd createESResources_HA error: ", err)
			logger.Error("etcd createESResources_HA error", err)

			destroyESResources_HA(output, serviceBrokerNamespace)
			oshandler.DeleteVolumns(serviceInfo.Database, volumes)

			return
		}

		println("create etcd Resources done")

	}()

	serviceSpec.DashboardURL = ""

	//>>>
	serviceSpec.Credentials = getCredentialsOnPrivision(&serviceInfo)
	//<<<

	return serviceSpec, serviceInfo, nil
}

func (handler *Elasticsearch_handler) DoLastOperation(myServiceInfo *oshandler.ServiceInfo) (brokerapi.LastOperation, error) {

	volumeJob := oshandler.GetCreatePvcVolumnJob(volumeBaseName(myServiceInfo.Url))
	if volumeJob != nil {
		return brokerapi.LastOperation{
			State:       brokerapi.InProgress,
			Description: "in progress.",
		}, nil
	}

	// only check the statuses of 3 ReplicationControllers. The etcd pods may be not running well.
	ok := func(dc *dcapi.DeploymentConfig) bool {
		podCount, err := statRunningPodsByLabels(myServiceInfo.Database, dc.Labels)
		if err != nil {
			fmt.Println("statRunningPodsByLabels err:", err)
			return false
		}
		if dc == nil || dc.Name == "" || dc.Spec.Replicas == 0 || podCount < dc.Spec.Replicas {
			return false
		}
		n, _ := statRunningPodsByLabels(myServiceInfo.Database, dc.Labels)
		return n >= dc.Spec.Replicas
	}

	ha_res, _ := getESResources_HA(
		myServiceInfo.Url, myServiceInfo.Database,
		myServiceInfo.Admin_password, myServiceInfo.User, myServiceInfo.Password, myServiceInfo.Volumes)

	//println("num_ok_rcs = ", num_ok_rcs)

	if ok(&ha_res.dc1) && ok(&ha_res.dc2) && ok(&ha_res.dc3) {
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

func (handler *Elasticsearch_handler) DoUpdate(myServiceInfo *oshandler.ServiceInfo, planInfo oshandler.PlanInfo, callbackSaveNewInfo func(*oshandler.ServiceInfo) error, asyncAllowed bool) error {
	return errors.New("not implemented")
}

func (handler *Elasticsearch_handler) DoDeprovision(myServiceInfo *oshandler.ServiceInfo, asyncAllowed bool) (brokerapi.IsAsync, error) {

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

		ha_res, _ := getESResources_HA(
			myServiceInfo.Url, myServiceInfo.Database,
			myServiceInfo.Admin_password, myServiceInfo.User, myServiceInfo.Password, myServiceInfo.Volumes)
		// under current frame, it is not a good idea to return here
		//if err != nil {
		//	return brokerapi.IsAsync(false), err
		//}
		destroyESResources_HA(ha_res, myServiceInfo.Database)
		println("destroy master resources done")

		println("to destroy volumes:", myServiceInfo.Volumes)

		oshandler.DeleteVolumns(myServiceInfo.Database, myServiceInfo.Volumes)
		println("to destroy volumes done")

	}()

	return brokerapi.IsAsync(false), nil
}

// please note: the bsi may be still not fully initialized when calling the function.
func getCredentialsOnPrivision(myServiceInfo *oshandler.ServiceInfo) oshandler.Credentials {
	var ha_res esResources_HA
	err := loadESResources_HA(myServiceInfo.Url, myServiceInfo.Volumes, &ha_res)
	if err != nil {
		return oshandler.Credentials{}
	}

	es_host, es_port, err := ha_res.ServiceHostPort(myServiceInfo.Database)
	if err != nil {
		return oshandler.Credentials{}
	}
	es_uri := fmt.Sprintf("http://%s:%s", es_host, es_port)

	return oshandler.Credentials{
		Uri:      es_uri,
		Hostname: es_host,
		Port:     es_port,
	}
}

func (handler *Elasticsearch_handler) DoBind(myServiceInfo *oshandler.ServiceInfo, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, oshandler.Credentials, error) {

	ha_res, err := getESResources_HA(
		myServiceInfo.Url, myServiceInfo.Database,
		myServiceInfo.Admin_password, myServiceInfo.User, myServiceInfo.Password, myServiceInfo.Volumes)
	if err != nil {
		return brokerapi.Binding{}, oshandler.Credentials{}, err
	}

	es_host, es_port, err := ha_res.ServiceHostPort(myServiceInfo.Database)
	if err != nil {
		return brokerapi.Binding{}, oshandler.Credentials{}, err
	}
	es_uri := fmt.Sprintf("http://%s:%s", es_host, es_port)

	mycredentials := oshandler.Credentials{
		Uri:      es_uri,
		Hostname: es_host,
		Port:     es_port,
	}

	myBinding := brokerapi.Binding{Credentials: mycredentials}

	return myBinding, mycredentials, nil
}

func (handler *Elasticsearch_handler) DoUnbind(myServiceInfo *oshandler.ServiceInfo, mycredentials *oshandler.Credentials) error {

	return nil
}

//===============================================================
//
//===============================================================

var ESTemplateData_HA []byte = nil

func loadESResources_HA(instanceID string, volumes []oshandler.Volume, res *esResources_HA) error {
	if ESTemplateData_HA == nil {
		f, err := os.Open("elasticsearch-pvc.yaml")
		if err != nil {
			return err
		}
		ESTemplateData_HA, err = ioutil.ReadAll(f)
		if err != nil {
			return err
		}

		ES_image := oshandler.ElasticsearchVolumeImage()
		ES_image = strings.TrimSpace(ES_image)
		if len(ES_image) > 0 {
			ESTemplateData_HA = bytes.Replace(
				ESTemplateData_HA,
				[]byte("http://elasticsearch-image-place-holder/elasticsearch-openshift-orchestration"),
				[]byte(ES_image),
				-1)
		}
		//endpoint_postfix := oshandler.EndPointSuffix()
		//endpoint_postfix = strings.TrimSpace(endpoint_postfix)
		//if len(endpoint_postfix) > 0 {
		//	EtcdTemplateData_HA = bytes.Replace(
		//		EtcdTemplateData_HA,
		//		[]byte("endpoint-postfix-place-holder"),
		//		[]byte(endpoint_postfix),
		//		-1)
		//}
	}

	peerPvcName0 := peerPvcName0(volumes)
	peerPvcName1 := peerPvcName1(volumes)
	peerPvcName2 := peerPvcName2(volumes)

	yamlTemplates := ESTemplateData_HA

	yamlTemplates = bytes.Replace(yamlTemplates, []byte("instanceid"), []byte(instanceID), -1)

	yamlTemplates = bytes.Replace(yamlTemplates, []byte("pvc-name-replace0"), []byte(peerPvcName0), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("pvc-name-replace1"), []byte(peerPvcName1), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("pvc-name-replace2"), []byte(peerPvcName2), -1)

	//println("========= HA yamlTemplates ===========")
	//println(string(yamlTemplates))
	//println()

	decoder := oshandler.NewYamlDecoder(yamlTemplates)
	decoder.
		Decode(&res.dc1).
		Decode(&res.dc2).
		Decode(&res.dc3).
		Decode(&res.svc1).
		Decode(&res.svc2).
		Decode(&res.svc3).
		Decode(&res.svc)
	return decoder.Err
}

type esResources_HA struct {
	dc1 dcapi.DeploymentConfig
	dc2 dcapi.DeploymentConfig
	dc3 dcapi.DeploymentConfig

	svc1 kapi.Service
	svc2 kapi.Service
	svc3 kapi.Service
	svc  kapi.Service
}

func (esResources_HA *esResources_HA) ServiceHostPort(serviceBrokerNamespace string) (string, string, error) {

	client_port := oshandler.GetServicePortByName(&esResources_HA.svc, "port-9200")
	if client_port == nil {
		return "", "", errors.New("client port not found")
	}

	host := fmt.Sprintf("%s.%s.%s", esResources_HA.svc.Name, serviceBrokerNamespace, oshandler.ServiceDomainSuffix(false))
	port := strconv.Itoa(client_port.Port)

	return host, port, nil
}

func createESResources_HA(instanceId, serviceBrokerNamespace string, volumes []oshandler.Volume) (*esResources_HA, error) {
	var input esResources_HA
	err := loadESResources_HA(instanceId, volumes, &input)
	if err != nil {
		return nil, err
	}

	var output esResources_HA

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		OPost(prefix+"/deploymentconfigs", &input.dc1, &output.dc1).
		OPost(prefix+"/deploymentconfigs", &input.dc2, &output.dc2).
		OPost(prefix+"/deploymentconfigs", &input.dc3, &output.dc3).
		KPost(prefix+"/services", &input.svc1, &output.svc1).
		KPost(prefix+"/services", &input.svc2, &output.svc2).
		KPost(prefix+"/services", &input.svc3, &output.svc3).
		KPost(prefix+"/services", &input.svc, &output.svc)

	if osr.Err != nil {
		logger.Error("createESResources_HA", osr.Err)
	}

	return &output, nil
}

func getESResources_HA(instanceId, serviceBrokerNamespace, rootPassword, user, password string, volumes []oshandler.Volume) (*esResources_HA, error) {
	var output esResources_HA

	var input esResources_HA
	err := loadESResources_HA(instanceId, volumes, &input)
	if err != nil {
		return &output, err
	}

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		OGet(prefix+"/deploymentconfigs/"+input.dc1.Name, &output.dc1).
		OGet(prefix+"/deploymentconfigs/"+input.dc2.Name, &output.dc2).
		OGet(prefix+"/deploymentconfigs/"+input.dc3.Name, &output.dc3).
		KGet(prefix+"/services/"+input.svc1.Name, &output.svc1).
		KGet(prefix+"/services/"+input.svc2.Name, &output.svc2).
		KGet(prefix+"/services/"+input.svc3.Name, &output.svc3).
		KGet(prefix+"/services/"+input.svc.Name, &output.svc)

	if osr.Err != nil {
		logger.Error("getEtcdResources_HA", osr.Err)
	}

	return &output, osr.Err
}

func destroyESResources_HA(haRes *esResources_HA, serviceBrokerNamespace string) {
	// todo: add to retry queue on fail

	go func() { odel(serviceBrokerNamespace, "deploymentconfigs", haRes.dc1.Name) }()
	go func() { odel(serviceBrokerNamespace, "deploymentconfigs", haRes.dc2.Name) }()
	go func() { odel(serviceBrokerNamespace, "deploymentconfigs", haRes.dc3.Name) }()

	go func() { kdel(serviceBrokerNamespace, "services", haRes.svc1.Name) }()
	go func() { kdel(serviceBrokerNamespace, "services", haRes.svc2.Name) }()
	go func() { kdel(serviceBrokerNamespace, "services", haRes.svc3.Name) }()
	go func() { kdel(serviceBrokerNamespace, "services", haRes.svc.Name) }()

	rcs, _ := statRunningRCByLabels(serviceBrokerNamespace, haRes.dc1.Labels)
	for _, rc := range rcs {
		go func() { kdel_rc(serviceBrokerNamespace, &rc) }()
	}

	rcs, _ = statRunningRCByLabels(serviceBrokerNamespace, haRes.dc2.Labels)
	for _, rc := range rcs {
		go func() { kdel_rc(serviceBrokerNamespace, &rc) }()
	}

	rcs, _ = statRunningRCByLabels(serviceBrokerNamespace, haRes.dc3.Labels)
	for _, rc := range rcs {
		go func() { kdel_rc(serviceBrokerNamespace, &rc) }()
	}
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
				logger.Error("watch HA etcd rc error", status.Err)
				close(cancel)
				return
			} else {
				//logger.Debug("watch etcd HA rc, status.Info: " + string(status.Info))
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

func statRunningRCByLabels(serviceBrokerNamespace string, labels map[string]string) ([]kapi.ReplicationController, error) {
	println("to list RC in", serviceBrokerNamespace)

	uri := "/namespaces/" + serviceBrokerNamespace + "/replicationcontrollers"

	rcs := kapi.ReplicationControllerList{}

	osr := oshandler.NewOpenshiftREST(oshandler.OC()).KList(uri, labels, &rcs)
	if osr.Err != nil {
		fmt.Println("get rc list err:", osr.Err)
		return nil, osr.Err
	}

	rcNames := make([]string, 0)
	for _, rc := range rcs.Items {
		rcNames = append(rcNames, rc.Name)

	}

	fmt.Println("-------->rcnames:", rcNames)
	return rcs.Items, nil
}

// todo:
//   use etcd clientv3 instead, which is able to close a client.
