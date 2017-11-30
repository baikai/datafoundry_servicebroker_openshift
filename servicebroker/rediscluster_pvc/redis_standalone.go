package rediscluster_pvc

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
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/pivotal-cf/brokerapi"
	//"crypto/sha1"
	//"encoding/base64"
	//"io"

	"os"
	//"sync"

	"github.com/pivotal-golang/lager"

	dcapi "github.com/openshift/origin/deploy/api/v1"
	//"k8s.io/kubernetes/pkg/util/yaml"
	kapi "k8s.io/kubernetes/pkg/api/v1"
	//routeapi "github.com/openshift/origin/route/api/v1"

	oshandler "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/handler"
)

//==============================================================
//
//==============================================================

const RedisClusterServcieBrokerName_Standalone = "Redis_volumes_cluster"

const DefaultNumNodes = 3 // 3 masters

func init() {
	oshandler.Register(RedisClusterServcieBrokerName_Standalone, &RedisCluster_freeHandler{})

	logger = lager.NewLogger(RedisClusterServcieBrokerName_Standalone)
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))
}

var logger lager.Logger

//==============================================================
//
//==============================================================

type RedisCluster_freeHandler struct{}

func (handler *RedisCluster_freeHandler) DoProvision(etcdSaveResult chan error, instanceID string, details brokerapi.ProvisionDetails, planInfo oshandler.PlanInfo, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, oshandler.ServiceInfo, error) {
	return newRedisClusterHandler().DoProvision(etcdSaveResult, instanceID, details, planInfo, asyncAllowed)
}

func (handler *RedisCluster_freeHandler) DoLastOperation(myServiceInfo *oshandler.ServiceInfo) (brokerapi.LastOperation, error) {
	return newRedisClusterHandler().DoLastOperation(myServiceInfo)
}

func (handler *RedisCluster_freeHandler) DoUpdate(myServiceInfo *oshandler.ServiceInfo, planInfo oshandler.PlanInfo, callbackSaveNewInfo func(*oshandler.ServiceInfo) error, asyncAllowed bool) error {
	return newRedisClusterHandler().DoUpdate(myServiceInfo, planInfo, callbackSaveNewInfo, asyncAllowed)
}

func (handler *RedisCluster_freeHandler) DoDeprovision(myServiceInfo *oshandler.ServiceInfo, asyncAllowed bool) (brokerapi.IsAsync, error) {
	return newRedisClusterHandler().DoDeprovision(myServiceInfo, asyncAllowed)
}

func (handler *RedisCluster_freeHandler) DoBind(myServiceInfo *oshandler.ServiceInfo, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, oshandler.Credentials, error) {
	return newRedisClusterHandler().DoBind(myServiceInfo, bindingID, details)
}

func (handler *RedisCluster_freeHandler) DoUnbind(myServiceInfo *oshandler.ServiceInfo, mycredentials *oshandler.Credentials) error {
	return newRedisClusterHandler().DoUnbind(myServiceInfo, mycredentials)
}

//==============================================================
//
//==============================================================

func volumeBaseName(instanceId string) string {
	return "rdsclstr-" + instanceId
}

//==============================================================
//
//==============================================================

type RedisCluster_Handler struct {
}

func newRedisClusterHandler() *RedisCluster_Handler {
	return &RedisCluster_Handler{}
}

func (handler *RedisCluster_Handler) DoProvision(etcdSaveResult chan error, instanceID string, details brokerapi.ProvisionDetails, planInfo oshandler.PlanInfo, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, oshandler.ServiceInfo, error) {
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
	//redisUser := oshandler.NewElevenLengthID()
	//redisPassword := oshandler.GenGUID() // redis cluster doesn't support password

	volumeBaseName := volumeBaseName(instanceIdInTempalte)
	volumes := make([]oshandler.Volume, DefaultNumNodes)
	for i := 0; i < DefaultNumNodes; i++ {
		volumes[i] = oshandler.Volume{
			Volume_size: planInfo.Volume_size,
			Volume_name: volumeBaseName + "-" + strconv.Itoa(i),
		}
	}

	println()
	println("instanceIdInTempalte = ", instanceIdInTempalte)
	println("serviceBrokerNamespace = ", serviceBrokerNamespace)
	println()

	// ...

	serviceInfo.Url = instanceIdInTempalte
	serviceInfo.Database = serviceBrokerNamespace // may be not needed
	//serviceInfo.User = redisUser
	//serviceInfo.Password = redisPassword

	serviceInfo.Volumes = volumes

	//>> may be not optimized
	var templates = make([]redisResources_Peer, DefaultNumNodes)
	err := loadRedisClusterResources_Peers(
		serviceInfo.Url,
		//serviceInfo.Password,
		serviceInfo.Volumes,
		nil, // nonsense for the to-be-created nodeport service
		templates,
	)
	if err != nil {
		return serviceSpec, oshandler.ServiceInfo{}, err
	}
	//<<

	nodePorts, err := createRedisClusterResources_NodePorts(
		templates,
		serviceInfo.Database,
	)
	if err != nil {
		peers := make([]*redisResources_Peer, len(templates))
		for i := range templates {
			peers[i] = &templates[i]
		}
		destroyRedisClusterResources_Peers(peers, serviceInfo.Database)
		return serviceSpec, oshandler.ServiceInfo{}, err
	}

	announceInfos := collectAnnounceInfos(nodePorts)

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
			logger.Error("redis cluster create volume", err)
			handler.DoDeprovision(&serviceInfo, true)
			return
		}

		println("createRedisClusterResources_Peer ...")

		// create master res

		outputs, err := createRedisClusterResources_Peers(
			serviceInfo.Database,
			serviceInfo.Url,
			//serviceInfo.Password,
			serviceInfo.Volumes,
			announceInfos,
		)
		if err != nil {
			println(" redis createRedisClusterResources_Peer error: ", err)
			logger.Error("redis createRedisClusterResources_Peer error", err)

			destroyRedisClusterResources_Peers(outputs, serviceInfo.Database)
			oshandler.DeleteVolumns(serviceInfo.Database, volumes)

			return
		}

		// run redis-trib.rb: create cluster
		err = initRedisMasterSlots(serviceInfo.Database, serviceInfo.Url, announceInfos)
		if err != nil {
			println(" redis initRedisMasterSlots error: ", err)
			logger.Error("redis initRedisMasterSlots error", err)
			return
		}
		println("redis cluster", serviceInfo.Database, "created.")
	}()

	// ...

	serviceSpec.DashboardURL = ""

	//>>>
	serviceSpec.Credentials = getCredentialsOnPrivision(&serviceInfo, announceInfos) //nodePort)
	//<<<

	return serviceSpec, serviceInfo, nil
}

func (handler *RedisCluster_Handler) DoLastOperation(myServiceInfo *oshandler.ServiceInfo) (brokerapi.LastOperation, error) {

	volumeJob := oshandler.GetCreatePvcVolumnJob(volumeBaseName(myServiceInfo.Url))
	if volumeJob != nil {
		return brokerapi.LastOperation{
			State:       brokerapi.InProgress,
			Description: "in progress.",
		}, nil
	}

	master_reses, err := getRedisClusterResources_Peers(
		myServiceInfo.Database,
		myServiceInfo.Url,
		//myServiceInfo.Password,
		myServiceInfo.Volumes,
	)
	//if err == oshandler.NotFound {
	//	return brokerapi.LastOperation{
	//		State:       brokerapi.InProgress,
	//		Description: "In progress .",
	//	}, nil
	//} else if err != nil {
	//	return return brokerapi.LastOperation{}, err
	//}
	if err != nil {
		return brokerapi.LastOperation{
			State:       brokerapi.Failed,
			Description: "In progress .",
		}, err
	}

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

	for _, res := range master_reses {
		if !ok(&res.dc) {
			return brokerapi.LastOperation{
				State:       brokerapi.InProgress,
				Description: "In progress.",
			}, nil
		}
	}
	return brokerapi.LastOperation{
		State:       brokerapi.Succeeded,
		Description: "Succeeded!",
	}, nil
}

func (handler *RedisCluster_Handler) DoUpdate(myServiceInfo *oshandler.ServiceInfo, planInfo oshandler.PlanInfo, callbackSaveNewInfo func(*oshandler.ServiceInfo) error, asyncAllowed bool) error {
	return errors.New("not implemented")
}

func (handler *RedisCluster_Handler) DoDeprovision(myServiceInfo *oshandler.ServiceInfo, asyncAllowed bool) (brokerapi.IsAsync, error) {
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

		master_reses, _ := getRedisClusterResources_Peers(
			myServiceInfo.Database,
			myServiceInfo.Url,
			//myServiceInfo.Password,
			myServiceInfo.Volumes,
		)
		destroyRedisClusterResources_Peers(master_reses, myServiceInfo.Database)

		//>> ...
		go func() { kdel(myServiceInfo.Database, "pod", "redis-trib-"+myServiceInfo.Url) }()
		//<<

		// ...

		fmt.Println("to destroy volumes:", myServiceInfo.Volumes)

		oshandler.DeleteVolumns(myServiceInfo.Database, myServiceInfo.Volumes)
	}()

	return brokerapi.IsAsync(false), nil
}

/*
// please note: the bsi may be still not fully initialized when calling the function.
func getCredentialsOnPrivision(myServiceInfo *oshandler.ServiceInfo, nodePort *redisResources_Peer) oshandler.Credentials {
	//var master_res redisResources_Peer
	//err := loadRedisClusterResources_Peer(myServiceInfo.Url, myServiceInfo.Password, myServiceInfo.Volumes, &master_res)
	//if err != nil {
	//	return oshandler.Credentials{}
	//}

	ndhost := oshandler.RandomNodeAddress()
	var svchost, svcport, ndport string
	if nodePort != nil && len(nodePort.serviceNodePort.Spec.Ports) > 0 {
		port := &nodePort.serviceNodePort.Spec.Ports[0]
		ndport = strconv.Itoa(port.NodePort)
		svchost = fmt.Sprintf("%s.%s.%s", nodePort.serviceNodePort.Name, myServiceInfo.Database, oshandler.ServiceDomainSuffix(false))
		svcport = strconv.Itoa(port.Port)
	}

	return oshandler.Credentials{
		Uri:      fmt.Sprintf("internal address: %s:%s", svchost, svcport),
		Hostname: ndhost,
		Port:     ndport,
		//Username: myServiceInfo.User,
		Password: myServiceInfo.Password,
		//Name:     cluser_name,
	}
}
*/

// please note: the bsi may be still not fully initialized when calling the function.
func getCredentialsOnPrivision(myServiceInfo *oshandler.ServiceInfo, announces []redisAnnounceInfo) oshandler.Credentials {

	infos := make([]string, len(announces))
	for i, announce := range announces {
		infos[i] = fmt.Sprintf("%s:%s", announce.IP, announce.Port)
	}

	return oshandler.Credentials{
		Uri: strings.Join(infos, ", "),
		//Password: myServiceInfo.Password,
	}
}

func (handler *RedisCluster_Handler) DoBind(myServiceInfo *oshandler.ServiceInfo, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, oshandler.Credentials, error) {
	// todo: handle errors

	master_reses, err := getRedisClusterResources_Peers(
		myServiceInfo.Database,
		myServiceInfo.Url,
		//myServiceInfo.Password,
		myServiceInfo.Volumes,
	)
	if err != nil {
		return brokerapi.Binding{}, oshandler.Credentials{}, err
	}

	mycredentials := getCredentialsOnPrivision(myServiceInfo, collectAnnounceInfos(master_reses))

	myBinding := brokerapi.Binding{Credentials: mycredentials}

	return myBinding, mycredentials, nil
}

func (handler *RedisCluster_Handler) DoUnbind(myServiceInfo *oshandler.ServiceInfo, mycredentials *oshandler.Credentials) error {
	// do nothing

	return nil
}

//=======================================================================
//
//=======================================================================

type redisAnnounceInfo struct {
	IP, Port, BusPort string
}

func collectAnnounceInfos(nodePorts []*redisResources_Peer) []redisAnnounceInfo {
	hostip := oshandler.RandomNodeAddress()

	announces := make([]redisAnnounceInfo, len(nodePorts))
	for i, res := range nodePorts {
		announces[i] = redisAnnounceInfo{
			IP:      hostip,
			Port:    strconv.Itoa(res.serviceNodePort.Spec.Ports[0].NodePort),
			BusPort: strconv.Itoa(res.serviceNodePort.Spec.Ports[1].NodePort),
		}
	}
	return announces
}

var redisTribYamlTemplate = template.Must(template.ParseFiles("redis-cluster-trib.yaml"))

func runRedisTrib(serviceBrokerNamespace, instanceId, command string, args []string) error {

	var params = map[string]interface{}{
		"InstanceID":       instanceId,
		"RedisTribCommand": command,
		"RedisTribImage":   oshandler.RedisClusterTribImage(),
		"Arguments":        args,
	}

	var buf bytes.Buffer
	err := redisTribYamlTemplate.Execute(&buf, params)
	if err != nil {
		return err
	}

	var pod kapi.Pod
	oshandler.NewYamlDecoder(buf.Bytes()).Decode(&pod)

	println(string(buf.Bytes()))

	return kpost(serviceBrokerNamespace, "pods", &pod, nil)
}

func initRedisMasterSlots(serviceBrokerNamespace, instanceId string, announces []redisAnnounceInfo) error {
	args := make([]string, 0, 100)
	args = append(args, "create")
	//lines = append(lines, "--replicas 1")
	for _, announce := range announces {
		args = append(args, announce.IP+":"+announce.Port)
	}
	for i := len(args); i < cap(args); i++ {
		args = append(args, "")
	}
	return runRedisTrib(serviceBrokerNamespace, instanceId, "redis-trib.rb", args)
}

//=======================================================================
//
//=======================================================================

var redisClusterYamlTemplate = template.Must(template.ParseFiles("redis-cluster-pvc.yaml"))

func loadRedisClusterResources_Peers(instanceID /*, redisPassword*/ string, volumes []oshandler.Volume,
	announces []redisAnnounceInfo, res []redisResources_Peer) error {

	if announces == nil { // for get, announces is allowed to be nil
		announces = make([]redisAnnounceInfo, len(res))
	}
	if len(announces) < len(res) {
		return fmt.Errorf("loadRedisClusterResources_Peers len(announces) < len(res): %d, %d", len(announces), len(res))
	}
	if len(volumes) < len(res) {
		return fmt.Errorf("loadRedisClusterResources_Peers len(volumes) < len(res): %d, %d", len(volumes), len(res))
	}

	for i := range res {
		err := loadRedisClusterResources_Peer(
			instanceID, strconv.Itoa(i) /*, redisPassword*/, volumes[i].Volume_name,
			announces[i],
			&res[i],
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func loadRedisClusterResources_Peer(instanceID, peerID /*, redisPassword*/, pvcName string,
	announce redisAnnounceInfo, res *redisResources_Peer) error {

	var params = map[string]interface{}{
		"InstanceID":             instanceID,
		"NodeID":                 peerID,
		"DataVolumePVC":          pvcName,
		"ClusterAnnounceIP":      announce.IP,
		"ClusterAnnouncePort":    announce.Port,
		"ClusterAnnounceBusPort": announce.BusPort,
		"RedisImage":             oshandler.RedisClusterImage(),
		//"Password":               redisPassword,
	}

	var buf bytes.Buffer
	err := redisClusterYamlTemplate.Execute(&buf, params)
	if err != nil {
		return err
	}

	//println("========= Boot yamlTemplates ===========")
	//println(string(yamlTemplates))
	//println()

	decoder := oshandler.NewYamlDecoder(buf.Bytes())
	decoder.
		//Decode(&res.service).
		Decode(&res.serviceNodePort).
		Decode(&res.dc)

	return decoder.Err
}

type redisResources_Peer struct {
	//service         kapi.Service
	serviceNodePort kapi.Service
	dc              dcapi.DeploymentConfig
}

func createRedisClusterResources_Peers(serviceBrokerNamespace string,
	instanceID /*, redisPassword*/ string, volumes []oshandler.Volume,
	announces []redisAnnounceInfo) ([]*redisResources_Peer, error) {

	if len(announces) < len(volumes) {
		return nil, fmt.Errorf("createRedisClusterResources_Peers len(announces) < numberPeers: %d, %d", len(announces), len(volumes))
	}

	var outputs = make([]*redisResources_Peer, len(volumes))
	for i := range outputs {
		o, err := createRedisClusterResources_Peer(serviceBrokerNamespace,
			instanceID, strconv.Itoa(i) /*, redisPassword*/, volumes[i].Volume_name,
			announces[i])
		if err != nil {
			return nil, err
		}
		outputs[i] = o
	}
	return outputs, nil
}

func createRedisClusterResources_Peer(serviceBrokerNamespace string,
	instanceID, peerID /*, redisPassword*/, pvcName string,
	announce redisAnnounceInfo) (*redisResources_Peer, error) {

	var input redisResources_Peer
	err := loadRedisClusterResources_Peer(instanceID, peerID /*, redisPassword*/, pvcName,
		announce, &input)
	if err != nil {
		return nil, err
	}

	var output redisResources_Peer

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	// here, not use job.post
	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		//KPost(prefix+"/services", &input.service, &output.service).
		OPost(prefix+"/deploymentconfigs", &input.dc, &output.dc)

	if osr.Err != nil {
		logger.Error("createRedisClusterResources_Peer error", osr.Err)
	}

	return &output, osr.Err
}

func createRedisClusterResources_NodePorts(inputs []redisResources_Peer, serviceBrokerNamespace string) ([]*redisResources_Peer, error) {
	var outputs = make([]*redisResources_Peer, len(inputs))
	for i := range inputs {
		o, err := createRedisClusterResources_NodePort(&inputs[i], serviceBrokerNamespace)
		if err != nil {
			return nil, err
		}
		outputs[i] = o
	}
	return outputs, nil
}

func createRedisClusterResources_NodePort(input *redisResources_Peer, serviceBrokerNamespace string) (*redisResources_Peer, error) {
	var output redisResources_Peer

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	// here, not use job.post
	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.KPost(prefix+"/services", &input.serviceNodePort, &output.serviceNodePort)

	if osr.Err != nil {
		logger.Error("createRedisClusterResources_NodePort", osr.Err)
	}

	return &output, osr.Err
}

func getRedisClusterResources_Peers(serviceBrokerNamespace string,
	instanceID /*, redisPassword*/ string, volumes []oshandler.Volume) ([]*redisResources_Peer, error) {

	var outputs = make([]*redisResources_Peer, len(volumes))
	for i := range outputs {
		o, err := getRedisClusterResources_Peer(serviceBrokerNamespace,
			instanceID, strconv.Itoa(i) /*, redisPassword*/, volumes[i].Volume_name)
		if err != nil {
			return nil, err
		}
		outputs[i] = o
	}
	return outputs, nil
}

func getRedisClusterResources_Peer(serviceBrokerNamespace string,
	instanceID, peerID /*, redisPassword*/, pvcName string) (*redisResources_Peer, error) {

	var output redisResources_Peer

	var input redisResources_Peer
	err := loadRedisClusterResources_Peer(
		instanceID, peerID /*, redisPassword*/, pvcName,
		redisAnnounceInfo{},
		&input)
	if err != nil {
		return &output, err
	}

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		//KGet(prefix+"/services/"+input.service.Name, &output.service).
		KGet(prefix+"/services/"+input.serviceNodePort.Name, &output.serviceNodePort).
		OGet(prefix+"/deploymentconfigs/"+input.dc.Name, &output.dc)

	if osr.Err != nil {
		logger.Error("getRedisClusterResources_Peer", osr.Err)
	}

	return &output, osr.Err
}

func destroyRedisClusterResources_Peers(masterReses []*redisResources_Peer, serviceBrokerNamespace string) {
	for _, res := range masterReses {
		destroyRedisClusterResources_Peer(res, serviceBrokerNamespace)
	}
}

func destroyRedisClusterResources_Peer(masterRes *redisResources_Peer, serviceBrokerNamespace string) {
	// todo: add to retry queue on fail

	go func() { odel(serviceBrokerNamespace, "deploymentconfigs", masterRes.dc.Name) }()
	go func() { kdel(serviceBrokerNamespace, "services", masterRes.serviceNodePort.Name) }()
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
				logger.Error("watch HA redis rc error", status.Err)
				close(cancel)
				return
			} else {
				//logger.Debug("watch redis HA rc, status.Info: " + string(status.Info))
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
