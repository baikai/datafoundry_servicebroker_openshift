package etcd

import (
	"fmt"
	"net"
	"time"

	"bytes"
	"encoding/json"
	dcapi "github.com/openshift/origin/deploy/api/v1"
	routeapi "github.com/openshift/origin/route/api/v1"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-golang/lager"
	"io/ioutil"
	kapi "k8s.io/kubernetes/pkg/api/v1"
	"os"
	"strings"

	oshandler "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/handler"
)

//==============================================================
//
//==============================================================

const EtcdServcieBrokerName_Volume_Standalone = "ETCD_volumes_standalone"

func init() {
	oshandler.Register(EtcdServcieBrokerName_Volume_Standalone, &Etcd_sampleHandler{})

	logger = lager.NewLogger(EtcdServcieBrokerName_Volume_Standalone)
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))
}

var logger lager.Logger

func volumeBaseName(instanceId string) string {
	return "etcd-" + instanceId
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

type Etcd_sampleHandler struct{}

func (handler *Etcd_sampleHandler) DoProvision(etcdSaveResult chan error, instanceID string, details brokerapi.ProvisionDetails, planInfo oshandler.PlanInfo, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, oshandler.ServiceInfo, error) {
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
	rootPassword := oshandler.GenGUID()
	//etcduser := EtcdGeneralUser //oshandler.NewElevenLengthID() // oshandler.GenGUID()[:16]
	//etcdPassword := oshandler.GenGUID()

	volumeBaseName := volumeBaseName(instanceIdInTempalte)
	volumes := []oshandler.Volume{
		// one master volume
		{
			Volume_size: planInfo.Volume_size,
			Volume_name: volumeBaseName + "-1",
		},
		// two slave volumes
		{
			Volume_size: planInfo.Volume_size,
			Volume_name: volumeBaseName + "-2",
		},
		{
			Volume_size: planInfo.Volume_size,
			Volume_name: volumeBaseName + "-3",
		},
	}

	println()
	println("instanceIdInTempalte = ", instanceIdInTempalte)
	println("serviceBrokerNamespace = ", serviceBrokerNamespace)
	println()

	// boot etcd

	serviceInfo.Url = instanceIdInTempalte
	serviceInfo.Database = serviceBrokerNamespace // may be not needed
	serviceInfo.Admin_user = "root"
	serviceInfo.Admin_password = rootPassword
	//serviceInfo.User = etcduser
	//serviceInfo.Password = etcdPassword

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
			logger.Error("etcd create volume (to delete created ones)", err)
			handler.DoDeprovision(&serviceInfo, true)
			return
		}

		println("create etcd Resources ...")

		// todo: consider if DoDeprovision is called now, ...

		// create master res

		output, err := createEtcdResources_HA(
			instanceIdInTempalte, serviceBrokerNamespace,
			rootPassword, volumes)

		if err != nil {
			println("etcd createEtcdResources_HA error: ", err)
			logger.Error("etcd createEtcdResources_HA error", err)

			//destroyEtcdResources_HA(output, serviceBrokerNamespace)
			//oshandler.DeleteVolumns(serviceInfo.Database, volumes)
			_ = output

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

func (handler *Etcd_sampleHandler) DoLastOperation(myServiceInfo *oshandler.ServiceInfo) (brokerapi.LastOperation, error) {

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

	ha_res, _ := getEtcdResources_HA(
		myServiceInfo.Url, myServiceInfo.Database,
		myServiceInfo.Admin_password, myServiceInfo.User, myServiceInfo.Password, myServiceInfo.Volumes)

	if ok(&ha_res.etcddc1) && ok(&ha_res.etcddc2) && ok(&ha_res.etcddc3) {
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
			logger.Error("etcd expand volume error", err)
			return
		}

		println("etcd expand volumens done")

		for i := range myServiceInfo.Volumes {
			myServiceInfo.Volumes[i].Volume_size = planInfo.Volume_size
		}
		err = callbackSaveNewInfo(myServiceInfo)
		if err != nil {
			logger.Error("etcd expand volume succeeded but save info error", err)
		}
	}()

	return nil
}

func (handler *Etcd_sampleHandler) DoDeprovision(myServiceInfo *oshandler.ServiceInfo, asyncAllowed bool) (brokerapi.IsAsync, error) {

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

		ha_res, _ := getEtcdResources_HA(
			myServiceInfo.Url, myServiceInfo.Database,
			myServiceInfo.Admin_password, myServiceInfo.User, myServiceInfo.Password, myServiceInfo.Volumes)
		// under current frame, it is not a good idea to return here
		//if err != nil {
		//	return brokerapi.IsAsync(false), err
		//}
		destroyEtcdResources_HA(ha_res, myServiceInfo.Database)
		println("destroy master resources done")

		println("to destroy volumes:", myServiceInfo.Volumes)

		oshandler.DeleteVolumns(myServiceInfo.Database, myServiceInfo.Volumes)
		println("to destroy volumes done")

	}()

	return brokerapi.IsAsync(false), nil
}

// please note: the bsi may be still not fully initialized when calling the function.
func getCredentialsOnPrivision(myServiceInfo *oshandler.ServiceInfo) oshandler.Credentials {
	var ha_res etcdResources_HA
	err := loadEtcdResources_HA(myServiceInfo.Url, myServiceInfo.Admin_password, myServiceInfo.Volumes, &ha_res)
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
		Username: myServiceInfo.Admin_user,
		Password: myServiceInfo.Admin_password,
	}
}

func (handler *Etcd_sampleHandler) DoBind(myServiceInfo *oshandler.ServiceInfo, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, oshandler.Credentials, error) {
	// output.route.Spec.Host

	ha_res, err := getEtcdResources_HA(
		myServiceInfo.Url, myServiceInfo.Database,
		myServiceInfo.Admin_password, myServiceInfo.User, myServiceInfo.Password, myServiceInfo.Volumes)

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
		Username: myServiceInfo.Admin_user,
		Password: myServiceInfo.Admin_password,
	}

	myBinding := brokerapi.Binding{Credentials: mycredentials}

	return myBinding, mycredentials, nil
}

func (handler *Etcd_sampleHandler) DoUnbind(myServiceInfo *oshandler.ServiceInfo, mycredentials *oshandler.Credentials) error {

	return nil
}

//===============================================================
//
//===============================================================

// The function name is for history reason?
func initEtcdRootPassword(namespasce string, input etcdResources_HA) bool {

	ok := func(dc *dcapi.DeploymentConfig) bool {
		podCount, err := statRunningPodsByLabels(namespasce, dc.Labels)
		fmt.Println("running pod:", podCount)
		if err != nil {
			fmt.Println("statRunningPodsByLabels err:", err)
			return false
		}
		if dc == nil || dc.Name == "" || dc.Spec.Replicas == 0 || podCount < dc.Spec.Replicas {
			return false
		}
		n, _ := statRunningPodsByLabels(namespasce, dc.Labels)
		return n >= dc.Spec.Replicas
	}

	var output etcdResources_HA
	var i = 0
	const MaxTries = 9
	for i < MaxTries {
		time.Sleep(7 * time.Second)
		if ok(&input.etcddc1) && ok(&input.etcddc2) && ok(&input.etcddc3) {
			err := kpost(namespasce, "pods", &input.pod, &output.pod)
			if err != nil {
				fmt.Println("cteate init password pod err:", err)
				return false
			}
			break
		}
		i++
	}
	if i == MaxTries {
		fmt.Println("cteate init password pod err: max tries reached")
		return false
	}
	return true
}

var EtcdTemplateData_HA []byte = nil

func loadEtcdResources_HA(instanceID, rootPassword string, volumes []oshandler.Volume, res *etcdResources_HA) error {
	if EtcdTemplateData_HA == nil {
		f, err := os.Open("etcd-pvc.yaml")
		if err != nil {
			return err
		}
		EtcdTemplateData_HA, err = ioutil.ReadAll(f)
		if err != nil {
			return err
		}

		etcd_image := oshandler.EtcdVolumeImage()
		etcd_image = strings.TrimSpace(etcd_image)
		if len(etcd_image) > 0 {
			EtcdTemplateData_HA = bytes.Replace(
				EtcdTemplateData_HA,
				[]byte("http://etcd-image-place-holder/etcd-product-openshift-orchestration"),
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

	peerPvcName0 := peerPvcName0(volumes)
	peerPvcName1 := peerPvcName1(volumes)
	peerPvcName2 := peerPvcName2(volumes)

	yamlTemplates := EtcdTemplateData_HA

	yamlTemplates = bytes.Replace(yamlTemplates, []byte("instanceid"), []byte(instanceID), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("#ETCDROOTPASSWORD#"), []byte(rootPassword), -1)

	yamlTemplates = bytes.Replace(yamlTemplates, []byte("pvc-name-replace1"), []byte(peerPvcName0), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("pvc-name-replace2"), []byte(peerPvcName1), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("pvc-name-replace3"), []byte(peerPvcName2), -1)

	//println("========= HA yamlTemplates ===========")
	//println(string(yamlTemplates))
	//println()

	decoder := oshandler.NewYamlDecoder(yamlTemplates)
	decoder.
		Decode(&res.etcddc1).
		Decode(&res.etcddc2).
		Decode(&res.etcddc3).
		Decode(&res.etcdsvc1).
		Decode(&res.etcdsvc2).
		Decode(&res.etcdsvc3).
		Decode(&res.etcdsvc0).
		Decode(&res.route).
		Decode(&res.pod)

	return decoder.Err
}

type etcdResources_HA struct {
	etcddc1 dcapi.DeploymentConfig
	etcddc2 dcapi.DeploymentConfig
	etcddc3 dcapi.DeploymentConfig

	etcdsvc1 kapi.Service
	etcdsvc2 kapi.Service
	etcdsvc3 kapi.Service
	etcdsvc0 kapi.Service

	route routeapi.Route

	pod kapi.Pod
}

func (haRes *etcdResources_HA) endpoint() (string, string, string) {
	port := "80" // strconv.Itoa(bootRes.service.Spec.Ports[0].Port)
	host := haRes.route.Spec.Host
	return "http://" + net.JoinHostPort(host, port), host, port
}

func createEtcdResources_HA(instanceId, serviceBrokerNamespace, rootPassword string, volumes []oshandler.Volume) (*etcdResources_HA, error) {
	var input etcdResources_HA
	err := loadEtcdResources_HA(instanceId, rootPassword, volumes, &input)
	if err != nil {
		return nil, err
	}

	var output etcdResources_HA

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		OPost(prefix+"/deploymentconfigs", &input.etcddc1, &output.etcddc1).
		OPost(prefix+"/deploymentconfigs", &input.etcddc2, &output.etcddc2).
		OPost(prefix+"/deploymentconfigs", &input.etcddc3, &output.etcddc3).
		KPost(prefix+"/services", &input.etcdsvc1, &output.etcdsvc1).
		KPost(prefix+"/services", &input.etcdsvc2, &output.etcdsvc2).
		KPost(prefix+"/services", &input.etcdsvc3, &output.etcdsvc3).
		KPost(prefix+"/services", &input.etcdsvc0, &output.etcdsvc0).
		OPost(prefix+"/routes", &input.route, &output.route)

	if osr.Err != nil {
		logger.Error("createEtcdResources_HA", osr.Err)
	}

	ok := initEtcdRootPassword(serviceBrokerNamespace, input)
	if !ok {
		fmt.Println("init password faild")
	}

	return &output, nil
}

func getEtcdResources_HA(instanceId, serviceBrokerNamespace, rootPassword, user, password string, volumes []oshandler.Volume) (*etcdResources_HA, error) {
	var output etcdResources_HA

	var input etcdResources_HA
	err := loadEtcdResources_HA(instanceId, rootPassword, volumes, &input)
	if err != nil {
		return &output, err
	}

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		OGet(prefix+"/deploymentconfigs/"+input.etcddc1.Name, &output.etcddc1).
		OGet(prefix+"/deploymentconfigs/"+input.etcddc2.Name, &output.etcddc2).
		OGet(prefix+"/deploymentconfigs/"+input.etcddc3.Name, &output.etcddc3).
		KGet(prefix+"/services/"+input.etcdsvc1.Name, &output.etcdsvc1).
		KGet(prefix+"/services/"+input.etcdsvc2.Name, &output.etcdsvc2).
		KGet(prefix+"/services/"+input.etcdsvc3.Name, &output.etcdsvc3).
		KGet(prefix+"/services/"+input.etcdsvc0.Name, &output.etcdsvc0).
		OGet(prefix+"/routes/"+input.route.Name, &output.route)

	if osr.Err != nil {
		logger.Error("getEtcdResources_HA", osr.Err)
	}

	return &output, osr.Err
}

func destroyEtcdResources_HA(haRes *etcdResources_HA, serviceBrokerNamespace string) {
	// todo: add to retry queue on fail

	go func() { odel(serviceBrokerNamespace, "deploymentconfigs", haRes.etcddc1.Name) }()
	go func() { odel(serviceBrokerNamespace, "deploymentconfigs", haRes.etcddc2.Name) }()
	go func() { odel(serviceBrokerNamespace, "deploymentconfigs", haRes.etcddc3.Name) }()

	go func() { kdel(serviceBrokerNamespace, "services", haRes.etcdsvc1.Name) }()
	go func() { kdel(serviceBrokerNamespace, "services", haRes.etcdsvc2.Name) }()
	go func() { kdel(serviceBrokerNamespace, "services", haRes.etcdsvc3.Name) }()
	go func() { kdel(serviceBrokerNamespace, "services", haRes.etcdsvc0.Name) }()

	go func() { odel(serviceBrokerNamespace, "routes", haRes.route.Name) }()

	go func() { kdel(serviceBrokerNamespace, "pods", haRes.pod.Name) }()

	rcs, _ := statRunningRCByLabels(serviceBrokerNamespace, haRes.etcddc1.Labels)
	for _, rc := range rcs {
		go func() { kdel_rc(serviceBrokerNamespace, &rc) }()
	}

	rcs, _ = statRunningRCByLabels(serviceBrokerNamespace, haRes.etcddc2.Labels)
	for _, rc := range rcs {
		go func() { kdel_rc(serviceBrokerNamespace, &rc) }()
	}

	rcs, _ = statRunningRCByLabels(serviceBrokerNamespace, haRes.etcddc3.Labels)
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
