package cassandra

import (
	"errors"
	"fmt"
	"bytes"
	"encoding/json"
	"github.com/pivotal-cf/brokerapi"
	"strconv"
	"strings"
	"time"
	"io/ioutil"
	"os"
	"sync"
	cassandra "github.com/gocql/gocql"
	"github.com/pivotal-golang/lager"
	kapi "k8s.io/kubernetes/pkg/api/v1"
	oshandler "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/handler"
)

//==============================================================
//
//==============================================================

const CassandraServcieBrokerName_Standalone = "Cassandra_standalone"

func init() {
	oshandler.Register(CassandraServcieBrokerName_Standalone, &Cassandra_sampleHandler{})

	logger = lager.NewLogger(CassandraServcieBrokerName_Standalone)
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))
}

var logger lager.Logger

//==============================================================
//
//==============================================================

type Cassandra_sampleHandler struct{}

func (handler *Cassandra_sampleHandler) DoProvision(etcdSaveResult chan error, instanceID string, details brokerapi.ProvisionDetails, planInfo oshandler.PlanInfo, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, oshandler.ServiceInfo, error) {
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
	serviceInfo.Database = serviceBrokerNamespace      // may be not needed
	serviceInfo.User = oshandler.NewThirteenLengthID() // NewElevenLengthID()
	serviceInfo.Password = oshandler.GenGUID()

	// todo: improve watch. Pod may be already running before watching!
	go func() {
		// boot cassandra

		err := <-etcdSaveResult
		if err != nil {
			return
		}

		output, err := createCassandraResources_Boot(instanceIdInTempalte, serviceBrokerNamespace)

		if err != nil {
			destroyCassandraResources_Boot(output, serviceBrokerNamespace)

			return
		}

		startCassandraOrchestrationJob(&cassandraOrchestrationJob{
			cancelled:  false,
			cancelChan: make(chan struct{}),

			isProvisioning: true,
			serviceInfo:    &serviceInfo,
			bootResources:  output,
			//haResources:    nil,
		})

	}()

	serviceSpec.DashboardURL = ""

	//>>>
	// the credentials are created on binding cassandra
	// serviceSpec.Credentials = getCredentialsOnPrivision(&serviceInfo)
	//<<<

	return serviceSpec, serviceInfo, nil
}

func (handler *Cassandra_sampleHandler) DoLastOperation(myServiceInfo *oshandler.ServiceInfo) (brokerapi.LastOperation, error) {
	// try to get state from running job
	job := getCassandraOrchestrationJob(myServiceInfo.Url)
	if job != nil {
		return brokerapi.LastOperation{
			State:       brokerapi.InProgress,
			Description: "In progress .",
		}, nil
	}

	// assume in provisioning

	// the job may be finished or interrupted or running in another instance.

	// check boot route, if it doesn't exist, return failed
	//boot_res, _ := getCassandraResources_Boot (myServiceInfo.Url, myServiceInfo.Database)
	//if boot_res.route.Name == "" {
	//	return brokerapi.LastOperation{
	//		State:       brokerapi.Failed,
	//		Description: "Failed!",
	//	}, nil
	//}

	// only check the statuses of 3 ReplicationControllers. The cassandra pods may be not running well.

	ok := func(rc *kapi.ReplicationController) bool {
		if rc == nil || rc.Name == "" || rc.Spec.Replicas == nil || rc.Status.Replicas < *rc.Spec.Replicas {
			return false
		}
		n, _ := statRunningPodsByLabels(myServiceInfo.Database, rc.Labels)

		println("n = ", n, ", *rc.Spec.Replicas = ", *rc.Spec.Replicas)

		return n >= *rc.Spec.Replicas
	}

	ha_res, _ := getCassandraResources_HA(myServiceInfo.Url, myServiceInfo.Database)

	//println("num_ok_rcs = ", num_ok_rcs)

	if !ok(&ha_res.rc) {
		return brokerapi.LastOperation{
			State:       brokerapi.InProgress,
			Description: "In progress.",
		}, nil
	}

	inited, err := checkIfCassandraPodsFullyInited(
		myServiceInfo.Database, ha_res.rc.Labels,
		"", myServiceInfo.User, myServiceInfo.Password)
	if err != nil {
		inited = false

		logger.Error("DoLastOperation/checkIfCassandraPodsFullyInited", err)
	}

	if inited {
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

func (handler *Cassandra_sampleHandler) DoUpdate(myServiceInfo *oshandler.ServiceInfo, planInfo oshandler.PlanInfo, callbackSaveNewInfo func(*oshandler.ServiceInfo) error, asyncAllowed bool) error {
	return nil
}

func (handler *Cassandra_sampleHandler) DoDeprovision(myServiceInfo *oshandler.ServiceInfo, asyncAllowed bool) (brokerapi.IsAsync, error) {
	go func() {
		job := getCassandraOrchestrationJob(myServiceInfo.Url)
		if job != nil {
			job.cancel()

			// wait job to exit
			for {
				time.Sleep(7 * time.Second)
				if nil == getCassandraOrchestrationJob(myServiceInfo.Url) {
					break
				}
			}
		}

		// ...

		println("to destroy resources")

		// must before destroying boot_rs
		ha_res, _ := getCassandraResources_HA(myServiceInfo.Url, myServiceInfo.Database)
		destroyCassandraResources_HA(ha_res, myServiceInfo.Database)

		boot_res, _ := getCassandraResources_Boot(myServiceInfo.Url, myServiceInfo.Database)
		destroyCassandraResources_Boot(boot_res, myServiceInfo.Database)
	}()

	return brokerapi.IsAsync(false), nil
}

func (handler *Cassandra_sampleHandler) DoBind(myServiceInfo *oshandler.ServiceInfo, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, oshandler.Credentials, error) {
	// output.route.Spec.Host

	job := getCassandraOrchestrationJob(myServiceInfo.Url)
	if job != nil {
		return brokerapi.Binding{}, oshandler.Credentials{}, errors.New("not fully initialized yet")
	}

	ha_res, err := getCassandraResources_HA(myServiceInfo.Url, myServiceInfo.Database)
	if err != nil {
		return brokerapi.Binding{}, oshandler.Credentials{}, err
	}

	host, port, err := ha_res.ServiceHostPort(myServiceInfo.Database)
	if err != nil {
		return brokerapi.Binding{}, oshandler.Credentials{}, err
	}

	// ...

	newusername := oshandler.NewThirteenLengthID() // NewElevenLengthID() // oshandler.GenGUID()[:16]
	newpassword := oshandler.GenGUID()

	println("super user:", myServiceInfo.User, ", super password:", myServiceInfo.Password)
	println("new user:", newusername, ", new password:", newpassword)

	tries := 5

RETRY: // maybe not needed now

	println("tries:", tries)

	cassandra_session, err := newAuthrizedCassandraSession([]string{host}, port, "", myServiceInfo.User, myServiceInfo.Password)
	//cassandra_session, err := newAuthrizedCassandraSession ([]string{host}, port, "", "cassandra", "cassandra")
	if err != nil {
		logger.Error("create cassandra authrized session", err)

		if tries > 0 {
			tries--
			goto RETRY
		}

		return brokerapi.Binding{}, oshandler.Credentials{}, err
	}
	defer cassandra_session.Close()

	if err := cassandra_session.Query(
		//`CREATE USER ? WITH PASSWORD '?' SUPERUSER`, newusername, newpassword).Exec(); err != nil {
		fmt.Sprintf(`CREATE USER '%s' WITH PASSWORD '%s' SUPERUSER;`, newusername, newpassword)).Exec(); err != nil {
		logger.Error("create new cassandra user", err)
		return brokerapi.Binding{}, oshandler.Credentials{}, err
	}

	// ...

	mycredentials := oshandler.Credentials{
		Uri:      "",
		Hostname: host,
		Port:     strconv.Itoa(port),
		Username: newusername,
		Password: newpassword,
	}

	myBinding := brokerapi.Binding{Credentials: mycredentials}

	return myBinding, mycredentials, nil
}

func (handler *Cassandra_sampleHandler) DoUnbind(myServiceInfo *oshandler.ServiceInfo, mycredentials *oshandler.Credentials) error {
	ha_res, err := getCassandraResources_HA(myServiceInfo.Url, myServiceInfo.Database)
	if err != nil {
		return err
	}

	host, port, err := ha_res.ServiceHostPort(myServiceInfo.Database)
	if err != nil {
		return err
	}

	// ...

	tries := 5

RETRY: // maybe not needed now

	println("tries:", tries)

	cassandra_session, err := newAuthrizedCassandraSession([]string{host}, port, "", myServiceInfo.User, myServiceInfo.Password)
	//cassandra_session, err := newAuthrizedCassandraSession ([]string{host}, port, "", "cassandra", "cassandra")
	if err != nil {
		logger.Error("create cassandra authrized session", err)

		if tries > 0 {
			tries--
			goto RETRY
		}

		return err
	}
	defer cassandra_session.Close()

	println("to delete user: ", mycredentials.Username)

	if err := cassandra_session.Query(
		//`DROP USER ?`, mycredentials.Username).Exec(); err != nil {
		fmt.Sprintf(`DROP USER '%s';`, mycredentials.Username)).Exec(); err != nil {
		logger.Error("delete cassandra user", err)
		return err
	}

	// ...

	return nil
}

//==============================================================
//
//==============================================================

var cassandraOrchestrationJobs = map[string]*cassandraOrchestrationJob{}
var cassandraOrchestrationJobsMutex sync.Mutex

func getCassandraOrchestrationJob(instanceId string) *cassandraOrchestrationJob {
	cassandraOrchestrationJobsMutex.Lock()
	defer cassandraOrchestrationJobsMutex.Unlock()

	return cassandraOrchestrationJobs[instanceId]
}

func startCassandraOrchestrationJob(job *cassandraOrchestrationJob) {
	cassandraOrchestrationJobsMutex.Lock()
	defer cassandraOrchestrationJobsMutex.Unlock()

	if cassandraOrchestrationJobs[job.serviceInfo.Url] == nil {
		cassandraOrchestrationJobs[job.serviceInfo.Url] = job
		go func() {
			job.run()

			cassandraOrchestrationJobsMutex.Lock()
			delete(cassandraOrchestrationJobs, job.serviceInfo.Url)
			cassandraOrchestrationJobsMutex.Unlock()
		}()
	}
}

type cassandraOrchestrationJob struct {
	//instanceId string // use serviceInfo.

	cancelled   bool
	cancelChan  chan struct{}
	cancelMetex sync.Mutex

	isProvisioning bool // false for deprovisionings

	serviceInfo *oshandler.ServiceInfo

	bootResources *cassandraResources_Boot
	//haResources   *cassandraResources_HA
}

func (job *cassandraOrchestrationJob) cancel() {
	job.cancelMetex.Lock()
	defer job.cancelMetex.Unlock()

	if !job.cancelled {
		job.isProvisioning = false
		job.cancelled = true
		close(job.cancelChan)
	}
}

type watchPodStatus struct {
	// The type of watch update contained in the message
	Type string `json:"type"`
	// Pod details
	Object kapi.Pod `json:"object"`
}

func (job *cassandraOrchestrationJob) run() {
	start_time := time.Now()

	default_root_user := "cassandra"
	default_root_password := "cassandra"

	serviceInfo := job.serviceInfo

	/*
		uri := "/namespaces/" + serviceInfo.Database + "/pods/" + job.bootResources.pod.Name
		statuses, cancel, err := oshandler.OC().KWatch (uri)
		if err != nil {
			logger.Error("start watching boot pod", err)
			job.isProvisioning = false
			destroyCassandraResources_Boot (job.bootResources, serviceInfo.Database)
			return
		}

		for {
			var status oshandler.WatchStatus
			select {
			case <- job.cancelChan:
				close(cancel)
				return
			case status, _ = <- statuses:
				break
			}

			if status.Err != nil {
				close(cancel)

				logger.Error("watch boot pod error", status.Err)
				job.isProvisioning = false
				destroyCassandraResources_Boot (job.bootResources, serviceInfo.Database)
				return
			} else {
				//logger.Debug("watch cassandra pod, status.Info: " + string(status.Info))
			}

			var wps watchPodStatus
			if err := json.Unmarshal(status.Info, &wps); err != nil {
				close(cancel)

				logger.Error("parse boot pod status", err)
				job.isProvisioning = false
				destroyCassandraResources_Boot (job.bootResources, serviceInfo.Database)
				return
			}

			if wps.Object.Status.Phase != kapi.PodPending {
				println("watch pod phase: ", wps.Object.Status.Phase)

				if wps.Object.Status.Phase != kapi.PodRunning {
					close(cancel)

					logger.Debug("pod phase is neither pending nor running")
					job.isProvisioning = false
					destroyCassandraResources_Boot (job.bootResources, serviceInfo.Database)
					return
				}

				// running now, to create HA resources
				close(cancel)
				break
			}
		}
	*/

	if job.bootResources.rc.Spec.Replicas == nil { // shouldn't
		return
	}

CHECK_POD_STATE_0:

	if job.cancelled {
		return
	}

	{
		n, _ := statRunningPodsByLabels(serviceInfo.Database, job.bootResources.rc.Labels)

		println("n = ", n, ", *job.bootResources.rc.Spec.Replicas = ", *job.bootResources.rc.Spec.Replicas)

		if n < *job.bootResources.rc.Spec.Replicas {
			time.Sleep(20 * time.Second)
			goto CHECK_POD_STATE_0
		}
	}

	println("seed pod is running now")

	// todo: check if seed pod is running

CHECK_POD_STATE_1:

	if job.cancelled {
		return
	}

	//time.Sleep(10 * time.Minute) // wait seed pod fully initialized
	{
		inited, err := checkIfCassandraPodsFullyInited(
			serviceInfo.Database, job.bootResources.service.Spec.Selector,
			"", default_root_user, default_root_password)
		if err != nil {
			logger.Error("checkIfCassandraPodsFullyInited 1", err)
			return
		}
		if !inited {
			time.Sleep(30 * time.Second)
			goto CHECK_POD_STATE_1
		}
	}

	if job.cancelled {
		return
	}

	// ...

	println("to create HA resources")

	// create HA resources

	ha_res, err := job.createCassandraResources_HA(serviceInfo.Url, serviceInfo.Database)
	// todo: if err != nil

	if ha_res.rc.Spec.Replicas == nil { // shouldn't
		return
	}

CHECK_POD_STATE_2:

	time.Sleep(10 * time.Second)

	if job.cancelled {
		return
	}

	{
		n, _ := statRunningPodsByLabels(serviceInfo.Database, ha_res.rc.Labels)

		println("n = ", n, ", *ha_res.rc.Spec.Replicas = ", *ha_res.rc.Spec.Replicas)

		if n < *ha_res.rc.Spec.Replicas {
			goto CHECK_POD_STATE_2
		}
	}

	if job.cancelled {
		return
	}

	println("cassandra ha pods are all running now")

CHECK_POD_STATE_3:

	if job.cancelled {
		return
	}

	//time.Sleep(15 * time.Minute) // wait ha pods fully initialized
	{
		inited, err := checkIfCassandraPodsFullyInited(
			serviceInfo.Database, ha_res.service.Spec.Selector,
			"", default_root_user, default_root_password)
		if err != nil {
			logger.Error("checkIfCassandraPodsFullyInited 2", err)
			return
		}
		if !inited {
			time.Sleep(30 * time.Second)
			goto CHECK_POD_STATE_3
		}
	}

	if job.cancelled {
		return
	}

	// create users

	time.Sleep(5 * time.Minute) // wait cluster fully formed.

RETRY_CREATE_NEW_USER:

	if job.cancelled {
		return
	}

	// ...

	host, port, err := ha_res.ServiceHostPort(serviceInfo.Database)
	if err != nil {
		logger.Error("get ServiceHostPort", err)
		return
	}

	println("to create new super user")

	f1 := func() bool {
		cassandra_session, err := newAuthrizedCassandraSession([]string{host}, port, "", default_root_user, default_root_password)
		//cassandra_session, err := newUnauthrizedCassandraSession ([]string{host}, port, "")
		if err != nil {
			logger.Error("create cassandra authrized session", err)
			return false
		}
		defer cassandra_session.Close()

		if err := cassandra_session.Query(
			//`CREATE USER '?' WITH PASSWORD '?' SUPERUSER;`, serviceInfo.User, serviceInfo.Password).Exec(); err != nil {
			fmt.Sprintf(`CREATE USER '%s' WITH PASSWORD '%s' SUPERUSER;`, serviceInfo.User, serviceInfo.Password)).Exec(); err != nil {
			logger.Error("create new cassandra super user ("+serviceInfo.User+","+serviceInfo.Password+")", err)
			return false
		}

		return true
	}

	if f1() == false {
		//return
		time.Sleep(30 * time.Second)
		goto RETRY_CREATE_NEW_USER
	}

RETRY_DELETE_DEFAULT_USER:

	time.Sleep(30 * time.Second)

	println("to delete user cassandra")

	f2 := func() bool {
		cassandra_session, err := newAuthrizedCassandraSession([]string{host}, port, "", serviceInfo.User, serviceInfo.Password)
		if err != nil {
			logger.Error("create cassandra authrized session.", err)
			return false
		}
		defer cassandra_session.Close()

		if err := cassandra_session.Query(
			//`DROP USER '?';`, default_root_user).Exec(); err != nil {
			fmt.Sprintf(`DROP USER '%s';`, default_root_user)).Exec(); err != nil {
			logger.Error("drop user cassandra", err)
			return false
		}

		return true
	}

	if f2() == false {
		//return
		goto RETRY_DELETE_DEFAULT_USER
	}

	end_time := time.Now()
	println("cassandra cluster inited fully. Used", end_time.Sub(start_time).String())
}

func newCassandraClusterConfig(cassandraEndPoints []string, port int, initialKeyspace string) *cassandra.ClusterConfig {
	cluster := cassandra.NewCluster(cassandraEndPoints...)
	cluster.Port = port
	cluster.Keyspace = initialKeyspace
	//cluster.Consistency = cassandra.One // Quorum
	cluster.CQLVersion = "3.4.0"
	cluster.ProtoVersion = 4
	cluster.Timeout = 30 * time.Second

	return cluster
}

func newUnauthrizedCassandraSession(cassandraEndPoints []string, port int, initialKeyspace string) (*cassandra.Session, error) {
	cluster := newCassandraClusterConfig(cassandraEndPoints, port, initialKeyspace)
	return cluster.CreateSession()
}

func newAuthrizedCassandraSession(cassandraEndPoints []string, port int, initialKeyspace string, cassandraUser, cassandraPassword string) (*cassandra.Session, error) {
	cluster := newCassandraClusterConfig(cassandraEndPoints, port, initialKeyspace)
	cluster.Authenticator = cassandra.PasswordAuthenticator{Username: cassandraUser, Password: cassandraPassword}
	return cluster.CreateSession()
}

func checkIfCassandraPodsFullyInited(serviceBrokerNamespace string, labels map[string]string,
	initialKeyspace string, cassandraUser, cassandraPassword string) (bool, error) {

	inited := true
	po := PodObserver(func(pod *kapi.Pod) {
		if inited {
			port := oshandler.GetPodPortByName(pod, "cql")
			if port == nil {
				inited = false
				return
			}

			cassandra_session, err := newAuthrizedCassandraSession(
				[]string{pod.Status.PodIP}, port.ContainerPort,
				initialKeyspace, cassandraUser, cassandraPassword)
			if err != nil {
				println("pod ", pod.Name, " has not inited yet")

				inited = false
			} else {
				cassandra_session.Close()

				println("pod ", pod.Name, " fully inited already")
			}
		}
	})

	err := po.ObserveRunningPodsByLabels(serviceBrokerNamespace, labels)

	return inited, err
}

//===============================================================
//
//===============================================================

var CassandraTemplateData_Boot []byte = nil

func loadCassandraResources_Boot(instanceID, serviceBrokerNamespace string, res *cassandraResources_Boot) error {
	if CassandraTemplateData_Boot == nil {
		f, err := os.Open("cassandra-boot.yaml")
		if err != nil {
			return err
		}
		CassandraTemplateData_Boot, err = ioutil.ReadAll(f)
		if err != nil {
			return err
		}

		cassandra_image := oshandler.CassandraImage()
		cassandra_image = strings.TrimSpace(cassandra_image)
		if len(cassandra_image) > 0 {
			CassandraTemplateData_Boot = bytes.Replace(
				CassandraTemplateData_Boot,
				[]byte("http://cassandra-image-place-holder/cassandra-openshift-orchestration"),
				[]byte(cassandra_image),
				-1)
		}
		endpoint_postfix := oshandler.EndPointSuffix()
		endpoint_postfix = strings.TrimSpace(endpoint_postfix)
		if len(endpoint_postfix) > 0 {
			CassandraTemplateData_Boot = bytes.Replace(
				CassandraTemplateData_Boot,
				[]byte("endpoint-postfix-place-holder"),
				[]byte(endpoint_postfix),
				-1)
		}
	}

	// todo: max length of res names in kubernetes is 24

	yamlTemplates := CassandraTemplateData_Boot

	yamlTemplates = bytes.Replace(yamlTemplates, []byte("instanceid"), []byte(instanceID), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("local-service-postfix-place-holder"),
		[]byte(serviceBrokerNamespace + oshandler.ServiceDomainSuffix(true)), -1)

	//println("========= Boot yamlTemplates ===========")
	//println(string(yamlTemplates))
	//println()

	decoder := oshandler.NewYamlDecoder(yamlTemplates)
	decoder.
		Decode(&res.rc).
		Decode(&res.service) //.
		//Decode(&res.route)

	return decoder.Err
}

var CassandraTemplateData_HA []byte = nil

func loadCassandraResources_HA(instanceID, serviceBrokerNamespace string, res *cassandraResources_HA) error {
	if CassandraTemplateData_HA == nil {
		f, err := os.Open("cassandra-ha.yaml")
		if err != nil {
			return err
		}
		CassandraTemplateData_HA, err = ioutil.ReadAll(f)
		if err != nil {
			return err
		}

		cassandra_image := oshandler.CassandraImage()
		cassandra_image = strings.TrimSpace(cassandra_image)
		if len(cassandra_image) > 0 {
			CassandraTemplateData_HA = bytes.Replace(
				CassandraTemplateData_HA,
				[]byte("http://cassandra-image-place-holder/cassandra-openshift-orchestration"),
				[]byte(cassandra_image),
				-1)
		}
	}

	yamlTemplates := CassandraTemplateData_HA

	yamlTemplates = bytes.Replace(yamlTemplates, []byte("instanceid"), []byte(instanceID), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("local-service-postfix-place-holder"),
		[]byte(serviceBrokerNamespace + oshandler.ServiceDomainSuffix(true)), -1)

	//println("========= HA yamlTemplates ===========")
	//println(string(yamlTemplates))
	//println()

	decoder := oshandler.NewYamlDecoder(yamlTemplates)
	decoder.
		Decode(&res.rc).
		Decode(&res.service)

	return decoder.Err
}

type cassandraResources_Boot struct {
	//pod     kapi.Pod
	rc      kapi.ReplicationController
	service kapi.Service
	//route   routeapi.Route
}

type cassandraResources_HA struct {
	rc      kapi.ReplicationController
	service kapi.Service
}

//func (bootRes *cassandraResources_Boot) endpoint() (string, string, string) {
//	//port := "80" // strconv.Itoa(bootRes.service.Spec.Ports[0].Port)
//	//host := bootRes.route.Spec.Host
//	//return "http://" + net.JoinHostPort(host, port), host, port
//}

func (bootRes *cassandraResources_HA) ServiceHostPort(serviceBrokerNamespace string) (string, int, error) {

	//client_port := oshandler.GetServicePortByName(&masterRes.service, "client")
	//if client_port == nil {
	//	return "", "", errors.New("client port not found")
	//}

	client_port := &bootRes.service.Spec.Ports[0]

	host := fmt.Sprintf("%s.%s.%s", bootRes.service.Name, serviceBrokerNamespace, oshandler.ServiceDomainSuffix(false))
	port := client_port.Port

	return host, port, nil
}

func createCassandraResources_Boot(instanceId, serviceBrokerNamespace string) (*cassandraResources_Boot, error) {
	var input cassandraResources_Boot
	err := loadCassandraResources_Boot(instanceId, serviceBrokerNamespace, &input)
	if err != nil {
		return nil, err
	}

	var output cassandraResources_Boot

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	// here, not use job.post
	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		KPost(prefix+"/replicationcontrollers", &input.rc, &output.rc).
		KPost(prefix+"/services", &input.service, &output.service) //.
		//OPost(prefix + "/routes", &input.route, &output.route)

	if osr.Err != nil {
		logger.Error("createCassandraResources_Boot", osr.Err)
	}

	return &output, osr.Err
}

func getCassandraResources_Boot(instanceId, serviceBrokerNamespace string) (*cassandraResources_Boot, error) {
	var output cassandraResources_Boot

	var input cassandraResources_Boot
	err := loadCassandraResources_Boot(instanceId, serviceBrokerNamespace, &input)
	if err != nil {
		return &output, err
	}

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		KGet(prefix+"/replicationcontrollers/"+input.rc.Name, &output.rc).
		KGet(prefix+"/services/"+input.service.Name, &output.service) //.
		//OGet(prefix + "/routes/" + input.route.Name, &output.route)

	if osr.Err != nil {
		logger.Error("getCassandraResources_Boot", osr.Err)
	}

	return &output, osr.Err
}

func destroyCassandraResources_Boot(bootRes *cassandraResources_Boot, serviceBrokerNamespace string) {
	// todo: add to retry queue on fail

	//go func() {odel (serviceBrokerNamespace, "routes", bootRes.route.Name)}()

	//go func() {kdel (serviceBrokerNamespace, "services", bootRes.service.Name)}()
	//go func() {kdel (serviceBrokerNamespace, "pods", bootRes.pod.Name)}()
	kdel(serviceBrokerNamespace, "services", bootRes.service.Name)
	//kdel (serviceBrokerNamespace, "pods", bootRes.pod.Name)
	kdel_rc(serviceBrokerNamespace, &bootRes.rc)
}

func (job *cassandraOrchestrationJob) createCassandraResources_HA(instanceId, serviceBrokerNamespace string) (*cassandraResources_HA, error) {
	var input cassandraResources_HA
	err := loadCassandraResources_HA(instanceId, serviceBrokerNamespace, &input)
	if err != nil {
		return nil, err
	}

	var output cassandraResources_HA

	/*
		osr := oshandler.NewOpenshiftREST(oshandler.OC())

		prefix := "/namespaces/" + serviceBrokerNamespace
		osr.
			KPost(prefix + "/replicationcontrollers", &input.rc, &output.rc)

		if osr.Err != nil {
			logger.Error("createCassandraResources_HA", osr.Err)
		}

		return osr.Err
	*/
	go func() {
		if err := job.kpost(serviceBrokerNamespace, "replicationcontrollers", &input.rc, &output.rc); err != nil {
			return
		}
		if err := job.kpost(serviceBrokerNamespace, "services", &input.service, &output.service); err != nil {
			return
		}
	}()

	return &input, nil
}

func getCassandraResources_HA(instanceId, serviceBrokerNamespace string) (*cassandraResources_HA, error) {
	var output cassandraResources_HA

	var input cassandraResources_HA
	err := loadCassandraResources_HA(instanceId, serviceBrokerNamespace, &input)
	if err != nil {
		return &output, err
	}

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		KGet(prefix+"/replicationcontrollers/"+input.rc.Name, &output.rc).
		KGet(prefix+"/services/"+input.service.Name, &output.service)

	if osr.Err != nil {
		logger.Error("getCassandraResources_HA", osr.Err)
	}

	return &output, osr.Err
}

func destroyCassandraResources_HA(haRes *cassandraResources_HA, serviceBrokerNamespace string) {
	// todo: add to retry queue on fail

	kdel(serviceBrokerNamespace, "services", haRes.service.Name)
	//go func() {kdel_rc (serviceBrokerNamespace, &haRes.rc)}()
	kdel_rc(serviceBrokerNamespace, &haRes.rc)
}

//===============================================================
//
//===============================================================

func (job *cassandraOrchestrationJob) kpost(serviceBrokerNamespace, typeName string, body interface{}, into interface{}) error {
	println("to create ", typeName)

	uri := fmt.Sprintf("/namespaces/%s/%s", serviceBrokerNamespace, typeName)
	i, n := 0, 5
RETRY:
	if job.cancelled {
		return nil
	}

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

func (job *cassandraOrchestrationJob) opost(serviceBrokerNamespace, typeName string, body interface{}, into interface{}) error {
	println("to create ", typeName)

	uri := fmt.Sprintf("/namespaces/%s/%s", serviceBrokerNamespace, typeName)
	i, n := 0, 5
RETRY:
	if job.cancelled {
		return nil
	}

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

	println("to delete rc, uri:", uri)

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
				logger.Error("watch HA cassandra rc error", status.Err)
				close(cancel)
				return
			} else {
				//logger.Debug("watch cassandra HA rc, status.Info: " + string(status.Info))
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

type PodObserver func(pod *kapi.Pod)

func (podObserver PodObserver) ObserveRunningPodsByLabels(serviceBrokerNamespace string, labels map[string]string) error {

	println("to list pods in", serviceBrokerNamespace)

	uri := "/namespaces/" + serviceBrokerNamespace + "/pods"

	pods := kapi.PodList{}

	osr := oshandler.NewOpenshiftREST(oshandler.OC()).KList(uri, labels, &pods)
	if osr.Err != nil {
		return osr.Err
	}

	for i := range pods.Items {
		pod := &pods.Items[i]
		podObserver(pod)
	}

	return nil
}

func statRunningPodsByLabels(serviceBrokerNamespace string, labels map[string]string) (int, error) {
	total := 0
	po := PodObserver(func(pod *kapi.Pod) {
		total++
	})

	err := po.ObserveRunningPodsByLabels(serviceBrokerNamespace, labels)

	return total, err
}

/*
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
			nrunnings ++
		}
	}

	return nrunnings, nil
}
*/
