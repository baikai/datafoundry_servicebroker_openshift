package mysql_galera_hostpath

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
	"text/template"
	"time"

	"github.com/pivotal-cf/brokerapi"
	//"crypto/sha1"
	//"encoding/base64"
	//"text/template"
	//"io"
	//"io/ioutil"
	"os"
	//"sync"

	"github.com/pivotal-golang/lager"

	//"k8s.io/kubernetes/pkg/util/yaml"
	//routeapi "github.com/openshift/origin/route/api/v1"
	kapi "k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/api/unversioned"
	//kapi_apps_v1beta1 "k8s.io/kubernetes/pkg/apis/apps/v1beta1"
	kapi_apps_v1beta1 "k8s.io/kubernetes/pkg/apis/apps/v1beta1-int32-to-int"

	oshandler "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/handler"
)

//==============================================================
//
//==============================================================

const MysqlServcieBrokerName_Standalone = "MySQL_hostpath_ha_cluster"

func init() {
	oshandler.Register(MysqlServcieBrokerName_Standalone, &Mysql_freeHandler{})

	logger = lager.NewLogger(MysqlServcieBrokerName_Standalone)
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))
}

var logger lager.Logger

//==============================================================
//
//==============================================================

func buildInstancethDataPath(instanceID string) string {
	return oshandler.MariadbGaleraHostPathDataPath() + "/instance-" + instanceID
}

//==============================================================
//
//==============================================================

type Mysql_freeHandler struct{}

func (handler *Mysql_freeHandler) DoProvision(etcdSaveResult chan error, instanceID string, details brokerapi.ProvisionDetails, planInfo oshandler.PlanInfo, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, oshandler.ServiceInfo, error) {
	return newMysqlHandler().DoProvision(etcdSaveResult, instanceID, details, planInfo, asyncAllowed)
}

func (handler *Mysql_freeHandler) DoLastOperation(myServiceInfo *oshandler.ServiceInfo) (brokerapi.LastOperation, error) {
	return newMysqlHandler().DoLastOperation(myServiceInfo)
}

func (handler *Mysql_freeHandler) DoUpdate(myServiceInfo *oshandler.ServiceInfo, planInfo oshandler.PlanInfo, callbackSaveNewInfo func(*oshandler.ServiceInfo) error, asyncAllowed bool) error {
	return newMysqlHandler().DoUpdate(myServiceInfo, planInfo, callbackSaveNewInfo, asyncAllowed)
}

func (handler *Mysql_freeHandler) DoDeprovision(myServiceInfo *oshandler.ServiceInfo, asyncAllowed bool) (brokerapi.IsAsync, error) {
	return newMysqlHandler().DoDeprovision(myServiceInfo, asyncAllowed)
}

func (handler *Mysql_freeHandler) DoBind(myServiceInfo *oshandler.ServiceInfo, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, oshandler.Credentials, error) {
	return newMysqlHandler().DoBind(myServiceInfo, bindingID, details)
}

func (handler *Mysql_freeHandler) DoUnbind(myServiceInfo *oshandler.ServiceInfo, mycredentials *oshandler.Credentials) error {
	return newMysqlHandler().DoUnbind(myServiceInfo, mycredentials)
}

//==============================================================
//
//==============================================================

type Mysql_Handler struct {
}

func newMysqlHandler() *Mysql_Handler {
	return &Mysql_Handler{}
}

func (handler *Mysql_Handler) DoProvision(etcdSaveResult chan error, instanceID string, details brokerapi.ProvisionDetails, planInfo oshandler.PlanInfo, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, oshandler.ServiceInfo, error) {
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
	mysqlUser := "root" // oshandler.NewElevenLengthID()
	mysqlPassword := oshandler.BuildPassword(20)

	println()
	println("instanceIdInTempalte = ", instanceIdInTempalte)
	println("serviceBrokerNamespace = ", serviceBrokerNamespace)
	println()

	// master mysql

	serviceInfo.Url = instanceIdInTempalte
	serviceInfo.Database = serviceBrokerNamespace // may be not needed
	serviceInfo.User = mysqlUser
	serviceInfo.Password = mysqlPassword

	//serviceInfo.Volumes = volumes
	serviceInfo.Miscs = map[string]string{}
	serviceInfo.Miscs[oshandler.VolumeSize] = strconv.Itoa(planInfo.Volume_size)

	//>> may be not optimized
	var template mysqlResources_Master
	err := loadMysqlResources_Master(
		serviceInfo.Url,
		serviceInfo.User,
		serviceInfo.Password,
		planInfo.Volume_size,
		&template)
	if err != nil {
		return serviceSpec, oshandler.ServiceInfo{}, err
	}
	//<<

	nodePort, err := createMysqlResources_NodePort(
		&template,
		serviceInfo.Database,
	)
	if err != nil {
		go destroyMysqlResources_Master(&template, serviceBrokerNamespace)
		return serviceSpec, oshandler.ServiceInfo{}, err
	}

	// ...
	go func() {
		err := <-etcdSaveResult
		if err != nil {
			return
		}

		println("createMysqlResources_Master ...")

		// create master res

		_, input, err := createMysqlResources_Master(
			serviceInfo.Url,
			serviceInfo.Database,
			serviceInfo.User,
			serviceInfo.Password,
			planInfo.Volume_size, // nonsense for this plan
		)
		if err != nil {
			println(" mysql createMysqlResources_Master error: ", err)
			logger.Error("mysql createMysqlResources_Master error", err)
			
			if input != nil {
				destroyMysqlResources_Master(input, serviceBrokerNamespace)
			}
			//oshandler.DeleteVolumns(serviceInfo.Database, volumes)

			return
		}
	}()

	//serviceSpec.DashboardURL = "http://" + template.routePma.Spec.Host

	//>>>
	serviceSpec.Credentials, serviceSpec.DashboardURL = getCredentialsOnPrivision(&serviceInfo, nodePort)
	//<<<

	return serviceSpec, serviceInfo, nil
}

func (handler *Mysql_Handler) DoLastOperation(myServiceInfo *oshandler.ServiceInfo) (brokerapi.LastOperation, error) {

	// assume in provisioning

	/*
	volumeJob := oshandler.GetCreatePvcVolumnJob(volumeBaseName(myServiceInfo.Url))
	if volumeJob != nil {
		return brokerapi.LastOperation{
			State:       brokerapi.InProgress,
			Description: "in progress.",
		}, nil
	}
	*/

	// the job may be finished or interrupted or running in another instance.

	master_res, _ := getMysqlResources_Master(
		myServiceInfo.Url,
		myServiceInfo.Database,
		myServiceInfo.User,
		myServiceInfo.Password,
		// myServiceInfo.Volumes,
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

	if ok(&master_res.rcPma) {
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

func (handler *Mysql_Handler) DoUpdate(myServiceInfo *oshandler.ServiceInfo, planInfo oshandler.PlanInfo, callbackSaveNewInfo func(*oshandler.ServiceInfo) error, asyncAllowed bool) error {
	return errors.New("not implemented")
}

func (handler *Mysql_Handler) DoDeprovision(myServiceInfo *oshandler.ServiceInfo, asyncAllowed bool) (brokerapi.IsAsync, error) {
	go func() {
		// ...
		/*
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
		*/

		// ...

		println("to destroy resources:", myServiceInfo.Url)

		master_res, _ := getMysqlResources_Master(
			myServiceInfo.Url,
			myServiceInfo.Database,
			myServiceInfo.User,
			myServiceInfo.Password,
			// 123,
		)
		destroyMysqlResources_Master(master_res, myServiceInfo.Database)

		// ...

		//println("to destroy volumes:", myServiceInfo.Volumes)
		//
		//oshandler.DeleteVolumns(myServiceInfo.Database, myServiceInfo.Volumes)
	}()

	return brokerapi.IsAsync(false), nil
}

// please note: the bsi may be still not fully initialized when calling the function.
func getCredentialsOnPrivision(myServiceInfo *oshandler.ServiceInfo, nodePort *mysqlResources_Master) (oshandler.Credentials, string) {
	var master_res mysqlResources_Master
	err := loadMysqlResources_Master(myServiceInfo.Url, myServiceInfo.User, myServiceInfo.Password, 123, &master_res)
	if err != nil {
		return oshandler.Credentials{}, "error"
	}

	mariamysql_port := oshandler.GetServicePortByName(&master_res.serviceMaria, "mysql")
	if mariamysql_port == nil {
		return oshandler.Credentials{}, "error"
	}

	svchost := fmt.Sprintf("%s.%s.%s", master_res.serviceMaria.Name, myServiceInfo.Database, oshandler.ServiceDomainSuffix(false))
	svcport := strconv.Itoa(mariamysql_port.Port)

	ndhost := oshandler.RandomNodeAddress()
	var ndport string = ""
	if nodePort != nil && len(nodePort.serviceMysql.Spec.Ports) > 0 {
		ndport = strconv.Itoa(nodePort.serviceMysql.Spec.Ports[0].NodePort)
	}
	// or: ndport := oshandler.GetServicePortByName(&nodePort.serviceMysql, "mysql").NodePort
	var pmaNdport string = ""
	if nodePort != nil && len(nodePort.servicePma.Spec.Ports) > 0 {
		pmaNdport = strconv.Itoa(nodePort.servicePma.Spec.Ports[0].NodePort)
	}
	dashboardUrl := "http://" + ndhost + ":" + pmaNdport

	return oshandler.Credentials{
		Uri:      fmt.Sprintf(
			"external address: %s:%s, internal address: %s:%s, dashboard address: %s",
			ndhost, ndport,
			svchost, svcport,
			dashboardUrl,
		),
		//Hostname: svchost, //ndhost,
		//Port:     svcport, //ndport,
		Username: myServiceInfo.User,
		Password: myServiceInfo.Password,
		//Vhost:    svchost,
	}, dashboardUrl
}

func (handler *Mysql_Handler) DoBind(myServiceInfo *oshandler.ServiceInfo, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, oshandler.Credentials, error) {
	// todo: handle errors
	return brokerapi.Binding{}, oshandler.Credentials{}, nil
	
	/*
	master_res, err := getMysqlResources_Master(
		myServiceInfo.Url,
		myServiceInfo.Database,
		myServiceInfo.User,
		myServiceInfo.Password,
		myServiceInfo.Volumes,
	)
	if err != nil {
		return brokerapi.Binding{}, oshandler.Credentials{}, err
	}

	mq_port := oshandler.GetServicePortByName(&master_res.service, "mq")
	if mq_port == nil {
		return brokerapi.Binding{}, oshandler.Credentials{}, errors.New("mq port not found")
	}

	host := fmt.Sprintf("%s.%s.%s", master_res.service.Name, myServiceInfo.Database, oshandler.ServiceDomainSuffix(false))
	port := strconv.Itoa(mq_port.Port)
	//host := master_res.routeMQ.Spec.Host
	//port := "80"

	// todo: return NodePort?

	mycredentials := oshandler.Credentials{
		Uri:      fmt.Sprintf("amqp://%s:%s@%s:%s", myServiceInfo.User, myServiceInfo.Password, host, port),
		Hostname: host,
		Port:     port,
		Username: myServiceInfo.User,
		Password: myServiceInfo.Password,
	}

	myBinding := brokerapi.Binding{Credentials: mycredentials}

	return myBinding, mycredentials, nil
	*/
}

func (handler *Mysql_Handler) DoUnbind(myServiceInfo *oshandler.ServiceInfo, mycredentials *oshandler.Credentials) error {
	// do nothing

	return nil
}

//=======================================================================
//
//=======================================================================

var MysqlTemplateData_Master []byte = nil

var mysqlYamlTemplate = template.Must(template.ParseFiles("mysql_galera_cluster_hostpath.yaml"))

func loadMysqlResources_Master(instanceID, mysqlUser, mysqlPassword string, volumeSize int, res *mysqlResources_Master) error {

	var params = map[string]interface{}{
		"InstanceID":                    instanceID,
		//"MysqlDataDiskSize":             volumeSize, // Gb
		"RootPassword":                  mysqlPassword,
		"MariadbImage":                  oshandler.MariadbImage(),
		//"PrometheusMysqldExporterImage": oshandler.PrometheusMysqldExporterImage(),
		"PhpMyAdminImage":               oshandler.PhpMyAdminImage(),
		//"StorageClassName":              oshandler.StorageClassName(),
		// "EndPointSuffix":                oshandler.EndPointSuffix(),
		"HostPathServiceAccount": oshandler.HostPathServiceAccount(),
		"NodeSelectorLabels":     oshandler.MariadbGaleraHostPathNodeLabels(),
		"MySqlDataPath":          buildInstancethDataPath(instanceID),
	}

	var buf bytes.Buffer
	err := mysqlYamlTemplate.Execute(&buf, params)
	if err != nil {
		return err
	}
	
	//println("========= Boot yamlTemplates ===========")
	//println(string(buf.Bytes()))
	//println()

	decoder := oshandler.NewYamlDecoder(buf.Bytes())
	decoder.
		Decode(&res.cmConfigD).
		Decode(&res.serviceMaria).
		Decode(&res.serviceMysql).
		Decode(&res.statefulset).
		Decode(&res.servicePma).
		Decode(&res.rcPma)//.
		//Decode(&res.routePma)

	return decoder.Err
}

type mysqlResources_Master struct {
	cmConfigD    kapi.ConfigMap
	serviceMaria kapi.Service
	serviceMysql kapi.Service
	statefulset  kapi_apps_v1beta1.StatefulSet
	
	servicePma kapi.Service
	rcPma      kapi.ReplicationController
	//routePma   routeapi.Route
}

func createMysqlResources_Master(instanceId, serviceBrokerNamespace, mysqlUser, mysqlPassword string, volumeSize int) (*mysqlResources_Master, *mysqlResources_Master, error) {
	var input mysqlResources_Master
	err := loadMysqlResources_Master(instanceId, mysqlUser, mysqlPassword, volumeSize, &input)
	if err != nil {
		return nil, nil, err
	}

	var output mysqlResources_Master

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	// here, not use job.post
	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		KPost(prefix+"/configmaps", &input.cmConfigD, &output.cmConfigD).
		KPost(prefix+"/services", &input.serviceMaria, &output.serviceMaria).
		// KPost(prefix+"/services", &input.serviceMysql, &output.serviceMysql).
		Post(prefix+"/statefulsets", &input.statefulset, &output.statefulset, "/apis/apps/v1beta1").
		// KPost(prefix+"/services", &input.servicePma, &output.servicePma).
		KPost(prefix+"/replicationcontrollers", &input.rcPma, &output.rcPma)//.
		//OPost(prefix+"/routes", &input.routePma, &output.routePma)

	if osr.Err != nil {
		logger.Error("createMysqlHaResources_Master", osr.Err)
	}

	return &output, &input, osr.Err
}

func createMysqlResources_NodePort(input *mysqlResources_Master, serviceBrokerNamespace string) (*mysqlResources_Master, error) {
	var output mysqlResources_Master

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	// here, not use job.post
	prefix := "/namespaces/" + serviceBrokerNamespace

	osr.KPost(prefix+"/services", &input.serviceMysql, &output.serviceMysql)
	if osr.Err != nil {
		logger.Error("createMysqlResources_NodePort (serviceMysql)", osr.Err)
	}

	osr.KPost(prefix+"/services", &input.servicePma, &output.servicePma)
	if osr.Err != nil {
		logger.Error("createMysqlResources_NodePort (servicePma)", osr.Err)
	}

	return &output, osr.Err
}

func getMysqlResources_Master(instanceId, serviceBrokerNamespace, mysqlUser, mysqlPassword string/*, volumes []oshandler.Volume*/) (*mysqlResources_Master, error) {
	var output mysqlResources_Master

	var input mysqlResources_Master
	err := loadMysqlResources_Master(instanceId, mysqlUser, mysqlPassword, 123/*volumes*/, &input)
	if err != nil {
		return &output, err
	}

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		KGet(prefix+"/configmaps/"+input.cmConfigD.Name, &output.cmConfigD).
		KGet(prefix+"/services/"+input.serviceMaria.Name, &output.serviceMaria).
		KGet(prefix+"/services/"+input.serviceMysql.Name, &output.serviceMysql).
		Get(prefix+"/statefulsets/"+input.statefulset.Name, &output.statefulset, "/apis/apps/v1beta1").
		KGet(prefix+"/services/"+input.servicePma.Name, &output.servicePma).
		KGet(prefix+"/replicationcontrollers/"+input.rcPma.Name, &output.rcPma)//.
		//OGet(prefix+"/routes/"+input.routePma.Name, &output.routePma)

	if osr.Err != nil {
		logger.Error("getMysqlResources_Master", osr.Err)
	}

	return &output, osr.Err
}

func destroyMysqlResources_Master(masterRes *mysqlResources_Master, serviceBrokerNamespace string) {
	// todo: add to retry queue on fail
	
	//pvclabels := masterRes.statefulset.Spec.Template.ObjectMeta.Labels
	
	go func() { kdel(serviceBrokerNamespace, "configmaps", masterRes.cmConfigD.Name) }()
	go func() { kdel(serviceBrokerNamespace, "services", masterRes.serviceMaria.Name) }()
	go func() { kdel(serviceBrokerNamespace, "services", masterRes.serviceMysql.Name) }()
	go func() {
		policy := kapi.DeletePropagationForeground
		opt := &kapi.DeleteOptions {
			TypeMeta: unversioned.TypeMeta {
				Kind:       "DeleteOptions",
				APIVersion: "v1",
			},
			PropagationPolicy: &policy,
		}
		del(serviceBrokerNamespace, "statefulsets", masterRes.statefulset.Name, "/apis/apps/v1beta1", opt)
		// sometimes, one pod and the statefulset ifself will not be deleted. Retry?
		time.Sleep(time.Second * 15)
		del(serviceBrokerNamespace, "statefulsets", masterRes.statefulset.Name, "/apis/apps/v1beta1", opt)
	}()
	go func() { kdel(serviceBrokerNamespace, "services", masterRes.servicePma.Name) }()
	go func() { kdel_rc(serviceBrokerNamespace, &masterRes.rcPma) }()
	//go func() { odel(serviceBrokerNamespace, "routes", masterRes.routePma.Name) }()
	//go func() { deletePvcsByLabels(serviceBrokerNamespace, pvclabels) }()
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

func del(serviceBrokerNamespace, typeName, resName string, apiGroup string, opt *kapi.DeleteOptions) error {
	//fmt.Println(">>>>>>> opt=", *opt)
	if resName == "" {
		return nil
	}

	println("to delete ", typeName, "/", resName)

	uri := fmt.Sprintf("/namespaces/%s/%s/%s", serviceBrokerNamespace, typeName, resName)
	i, n := 0, 5
RETRY:
	osr := oshandler.NewOpenshiftREST(oshandler.OC()).Delete(uri, nil, apiGroup, opt)
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
				logger.Error("watch HA mysql rc error", status.Err)
				close(cancel)
				return
			} else {
				//logger.Debug("watch mysql HA rc, status.Info: " + string(status.Info))
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

func deletePvcsByLabels(serviceBrokerNamespace string, labels map[string]string) error {

	println("to delete pvcs in", serviceBrokerNamespace)

	uri := "/namespaces/" + serviceBrokerNamespace + "/persistentvolumeclaims"

	i, n := 0, 5
RETRY:
	var result interface{}
	osr := oshandler.NewOpenshiftREST(oshandler.OC()).KDeleteByLabels(uri, labels, result)
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
