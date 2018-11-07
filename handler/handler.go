package handler

import (
	//"crypto/md5"
	//"crypto/rand"
	//"encoding/base64"
	//"encoding/hex"
	"errors"
	"fmt"
	//"io"
	//"math"
	mathrand "math/rand"
	"os"
	//"strconv"
	"strings"
	"time"

	"github.com/pivotal-cf/brokerapi"
)

func init() {
	mathrand.Seed(time.Now().UnixNano())
}

//const (
//	VolumeType_EmptyDir = ""    // DON'T change
//	VolumeType_PVC      = "pvc" // DON'T change
//)

// Some service common parameters.
const (
	// pvc plans
	VolumeSize = "volumeSize"
	// ... never used
	Connections = "connections"
	// redis cluster, ...
	Nodes = "nodes"
	// redis cluster, storem external, ...
	Memory = "memory"
	// redis cluster, ...
	Replicas = "replicas"
	// zeppelin cpu
	CPU = "cpu"
)

type Volume struct {
	Volume_size int    `json:"volume_size"`
	Volume_name string `json:"volume_name"`
}

type ServiceInfo struct {
	Service_name   string `json:"service_name"`
	Plan_name      string `json:"plan_name"`
	Url            string `json:"url"`
	Admin_user     string `json:"admin_user,omitempty"`
	Admin_password string `json:"admin_password,omitempty"`
	Database       string `json:"database,omitempty"`
	User           string `json:"user"`
	Password       string `json:"password"`

	// following fileds
	//Volume_type    string   `json:"volume_type"` // "" | "pvc"
	//Volume_size    int      `json:"volume_size"`
	//
	// will be replaced by
	Volumes []Volume `json:"volumes,omitempty"`

	// for different bs, the meaning is different
	Miscs map[string]string `json:"miscs,omitempty"`
	
	// ...
	ProvisionFailureInfo string `json:"provision_failure_info,omitempty"`
	
	// The following fields are used by main.go and will not save into ectd.
	asyncResult chan error
}

func (info *ServiceInfo) MakeAsyncResult() chan<- error {
	info.asyncResult = make(chan error, 1)
	return info.asyncResult
}

func (info *ServiceInfo) AsyncResult() <-chan error {
	return info.asyncResult
}

//==================

type PlanInfo struct {
	Volume_size int `json:"volume_type"`
	Connections int `json:"connections"`
	//Customize   map[string]CustomParams `json:"customize"`

	MoreParameters    map[string]interface{}
	ParameterSettings map[string]CustomParams
}

type Credentials struct {
	Uri      string `json:"uri"`
	Hostname string `json:"host"`
	Port     string `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	Name     string `json:"name"`
	Vhost    string `json:"vhost"`
}

type CustomParams struct {
	Default float64 `json:"default"`
	Max     float64 `json:"max"`
	Price   float64 `json:"price"`
	Unit    string  `json:"unit"`
	Step    float64 `json:"step"`
	Desc    string  `json:"desc"`
}

type HandlerDriver interface {
	DoProvision(etcdSaveResult chan error, instanceID string, details brokerapi.ProvisionDetails, planInfo PlanInfo, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, ServiceInfo, error)
	DoUpdate(myServiceInfo *ServiceInfo, planInfo PlanInfo, callbackSaveNewInfo func(*ServiceInfo) error, asyncAllowed bool) error
	DoLastOperation(myServiceInfo *ServiceInfo) (brokerapi.LastOperation, error)
	DoDeprovision(myServiceInfo *ServiceInfo, asyncAllowed bool) (brokerapi.IsAsync, error)
	DoBind(myServiceInfo *ServiceInfo, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, Credentials, error)
	DoUnbind(myServiceInfo *ServiceInfo, mycredentials *Credentials) error
}

type Handler struct {
	driver HandlerDriver
}

var handlers = make(map[string]HandlerDriver)

func Register(name string, handler HandlerDriver) {
	if handler == nil {
		panic("handler: Register handler is nil")
	}
	if _, dup := handlers[name]; dup {
		panic("handler: Register called twice for handler " + name)
	}
	handlers[name] = handler
}

func ListHandler() {
	for k, _ := range handlers {
		fmt.Println(k)
	}
}

func New(name string) (*Handler, error) {
	handler, ok := handlers[name]
	if !ok {
		return nil, fmt.Errorf("Can't find handler %s", name)
	}
	return &Handler{driver: handler}, nil
}

func (handler *Handler) DoProvision(etcdSaveResult chan error, instanceID string, details brokerapi.ProvisionDetails, planInfo PlanInfo, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, ServiceInfo, error) {
	return handler.driver.DoProvision(etcdSaveResult, instanceID, details, planInfo, asyncAllowed)
}

func (handler *Handler) DoUpdate(myServiceInfo *ServiceInfo, planInfo PlanInfo, callbackSaveNewInfo func(*ServiceInfo) error, asyncAllowed bool) error {
	return handler.driver.DoUpdate(myServiceInfo, planInfo, callbackSaveNewInfo, asyncAllowed)
}

func (handler *Handler) DoLastOperation(myServiceInfo *ServiceInfo) (brokerapi.LastOperation, error) {
	return handler.driver.DoLastOperation(myServiceInfo)
}

func (handler *Handler) DoDeprovision(myServiceInfo *ServiceInfo, asyncAllowed bool) (brokerapi.IsAsync, error) {
	return handler.driver.DoDeprovision(myServiceInfo, asyncAllowed)
}

func (handler *Handler) DoBind(myServiceInfo *ServiceInfo, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, Credentials, error) {
	return handler.driver.DoBind(myServiceInfo, bindingID, details)
}

func (handler *Handler) DoUnbind(myServiceInfo *ServiceInfo, mycredentials *Credentials) error {
	return handler.driver.DoUnbind(myServiceInfo, mycredentials)
}

//=========================================================

func OC() *OpenshiftClient {
	return theOC
}

func ServiceDomainSuffix(prefixedWithDot bool) string {
	if prefixedWithDot {
		return svcDomainSuffixWithDot
	}
	return svcDomainSuffix
}

func EndPointSuffix() string {
	return endpointSuffix
}

func StorageClassName() string {
	return storageClassName
}

func DfProxyApiPrefix() string {
	return dfProxyApiPrefix
}

func DnsmasqServer() string {
	return dnsmasqServer
}

func RandomNodeAddress() string {
	if len(nodeAddresses) == 0 {
		return ""
	}
	return nodeAddresses[mathrand.Intn(len(nodeAddresses))]
}

func RandomNodeDomain() string {
	if len(nodeDemains) == 0 {
		return ""
	}
	return nodeDemains[mathrand.Intn(len(nodeDemains))]
}

func NodeDomain(n int) string {
	if len(nodeDemains) == 0 {
		return ""
	}
	if n < 0 || n >= len(nodeDemains) {
		n = 0
	}
	return nodeDemains[n]
}

func ExternalZookeeperServer(n int) string {
	if len(externalZookeeperServers) == 0 {
		return ""
	}
	if n < 0 || n >= len(externalZookeeperServers) {
		n = 0
	}
	return externalZookeeperServers[n]
}

func EtcdRegistryInitInfo() string {
	return etcdRegistryInitInfo
}

func ServiceAccountToken() string {
	return serviceAccountToken
}

func EtcdImage() string {
	return etcdImage
}

func EtcdVolumeImage() string {
	return etcdVolumeImage
}

func EtcdbootImage() string {
	return etcdbootImage
}

func ZookeeperImage() string {
	return zookeeperImage
}

func ZookeeperExhibitorImage() string {
	return zookeeperexhibitorImage
}

func RedisImage() string {
	return redisImage
}

func RedisPhpAdminImage() string {
	return redisphpadminImage
}

func Redis32Image() string {
	return redis32Image
}

func RedisClusterImage() string {
	return redisClusterImage
}

func RedisStatImage() string {
	return redisStatImage
}

//func RedisClusterTribImage() string {
//	return redisClusterTribImage
//}

func KafkaImage() string {
	return kafkaImage
}

func StormImage() string {
	return stormImage
}

func CassandraImage() string {
	return cassandraImage
}

func TensorFlowImage() string {
	return tensorflowImage
}

func NiFiImage() string {
	return nifiImage
}

func KettleImage() string {
	return kettleImage
}

func SimpleFileUplaoderImage() string {
	return simplefileuplaoderImage
}

func RabbitmqImage() string {
	return rabbitmqImage
}

func SparkImage() string {
	return sparkImage
}

func ZepplinImage() string {
	return zepplinImage
}

func PySpiderImage() string {
	return pyspiderImage
}

func ElasticsearchVolumeImage() string {
	return elasticsearchVolumeImage
}

func MongoVolumeImage() string {
	return mongoVolumeImage
}

func KafkaVolumeImage() string {
	return kafkaVolumeImage
}

func Neo4jVolumeImage() string {
	return neo4jVolumeImage
}

func StormExternalImage() string {
	return stormExternalImage
}

func DataikuImage() string {
	return dataikuImage
}

func OcspImage() string {
	return ocspImage
}

func OcspOcm() string {
	return ocspOcm
}

func OcspOcmPort() string {
	return ocspOcmPort
}
func OcspHdpVersion() string {
	return ocspHdpVersion
}

func AnacodaImage() string {
	return anacondaImage
}

func HostPathServiceAccount() string {
	return hostPathServiceAccount
}

func MariadbGaleraHostPathNodeLabels() map[string]string {
	return mariadbGaleraHostPathNodeLabels
}

func MariadbGaleraHostPathDataPath() string {
	return mariadbGaleraHostPathDataPath
}

func MariadbImageForHostPath() string {
	return mariadbImageForHostPath
}

func MariadbImage() string {
	return mariadbImage
}

func PrometheusMysqldExporterImage() string {
	return prometheusMysqldExporterImage
}

func PhpMyAdminImage() string {
	return phpMyAdminImage
}

// EsclusterImage return image name for elastic search cluster
func EsclusterImage() string {
	return esclusterImage
}

//func DfExternalIPs() string {
//	return externalIPs
//}

var theOC *OpenshiftClient

var svcDomainSuffix string
var endpointSuffix string
var svcDomainSuffixWithDot string

var storageClassName string
var dfProxyApiPrefix string

var dnsmasqServer string // may be useless now.

var nodeAddresses []string
var nodeDemains []string
var externalZookeeperServers []string

var etcdRegistryInitInfo string
var serviceAccountToken string

var ocspOcm string
var ocspOcmPort string
var ocspHdpVersion string

var etcdImage string
var etcdVolumeImage string
var etcdbootImage string
var zookeeperImage string
var zookeeperexhibitorImage string
var redisImage string
var redis32Image string
var redisClusterImage string
var redisStatImage string

//var redisClusterTribImage string // merged into redisClusterImage
var redisphpadminImage string // looks useless
var kafkaImage string
var stormImage string
var cassandraImage string
var tensorflowImage string
var nifiImage string
var kettleImage string
var simplefileuplaoderImage string
var rabbitmqImage string
var sparkImage string
var zepplinImage string
var pyspiderImage string
var elasticsearchVolumeImage string
var mongoVolumeImage string
var kafkaVolumeImage string
var neo4jVolumeImage string
var stormExternalImage string
var ocspImage string
var dataikuImage string
var anacondaImage string
var hostPathServiceAccount string
var mariadbGaleraHostPathNodeLabels map[string]string
var mariadbGaleraHostPathDataPath string
var mariadbImageForHostPath string
var mariadbImage string
var prometheusMysqldExporterImage string
var phpMyAdminImage string

// added by Jared
var esclusterImage string

// will be injected at build time
var Gitrev string = "[not set]"

func init() {
	fmt.Println("git revision:", Gitrev)
	theOC = newOpenshiftClient(
		Getenv_must("OPENSHIFTADDR"),
		Getenv_must("OPENSHIFTUSER"),
		Getenv_must("OPENSHIFTPASS"),
		Getenv_must("SBNAMESPACE"),
	)

	svcDomainSuffix = os.Getenv("SERVICEDOMAINSUFFIX")
	if svcDomainSuffix == "" {
		svcDomainSuffix = "svc.cluster.local"
	}
	svcDomainSuffixWithDot = "." + svcDomainSuffix

	endpointSuffix = Getenv_must("ENDPOINTSUFFIX")

	storageClassName = Getenv_must("STORAGECLASSNAME")
	dfProxyApiPrefix = os.Getenv("DATAFOUNDRYPROXYADDR")
	if dfProxyApiPrefix == "" {
		logger.Error("int dfProxyApiPrefix error:", errors.New("DATAFOUNDRYPROXYADDR env is not set"))
	}
	dfProxyApiPrefix = "http://" + dfProxyApiPrefix + "/lapi/v1"

	dnsmasqServer = Getenv_must("DNSMASQ_SERVER")

	nodeAddresses = strings.Split(Getenv_must("NODE_ADDRESSES"), ",")
	nodeDemains = strings.Split(Getenv_must("NODE_DOMAINS"), ",")
	externalZookeeperServers = strings.Split(Getenv_must("EXTERNALZOOKEEPERSERVERS"), ",")

	etcdRegistryInitInfo = Getenv_opitional("ETCD_REGISTRY_INIT_INFO") // format: init_script_file:api_user:api_password
	serviceAccountToken = Getenv_opitional("SERVICE_ACCOUNT_TOKEN")

	ocspOcm = Getenv_warning("OCSP_OCM")
	ocspOcmPort = Getenv_warning("OCSP_OCM_PORT")
	ocspHdpVersion = Getenv_warning("OCSP_HDP_VERSION")

	etcdImage = Getenv_warning("ETCDIMAGE")
	etcdbootImage = Getenv_warning("ETCDBOOTIMAGE")
	zookeeperImage = Getenv_warning("ZOOKEEPERIMAGE")
	zookeeperexhibitorImage = Getenv_warning("ZOOKEEPEREXHIBITORIMAGE")
	redisImage = Getenv_warning("REDISIMAGE")
	redis32Image = Getenv_warning("REDIS32IMAGE")
	redisClusterImage = Getenv_warning("REDISCLUSTERIMAGE")
	redisStatImage = Getenv_warning("REDISSTATIMAGE")
	//redisClusterTribImage = Getenv_warning("REDISCLUSTERTRIBIMAGE")
	redisphpadminImage = Getenv_warning("REDISPHPADMINIMAGE")
	kafkaImage = Getenv_warning("KAFKAIMAGE")
	stormImage = Getenv_warning("STORMIMAGE")
	cassandraImage = Getenv_warning("CASSANDRAIMAGE")
	tensorflowImage = Getenv_warning("TENSORFLOWIMAGE")
	nifiImage = Getenv_warning("NIFIIMAGE")
	kettleImage = Getenv_warning("KETTLEIMAGE")
	simplefileuplaoderImage = Getenv_warning("SIMPLEFILEUPLOADERIMAGE")
	rabbitmqImage = Getenv_warning("RABBITMQIMAGE")
	sparkImage = Getenv_warning("SPARKIMAGE")
	zepplinImage = Getenv_warning("ZEPPLINIMAGE")
	pyspiderImage = Getenv_warning("PYSPIDERIMAGE")
	etcdVolumeImage = Getenv_warning("ETCDVOLUMEIMAGE")
	elasticsearchVolumeImage = Getenv_warning("ELASTICSEARCHVOLUMEIMAGE")
	mongoVolumeImage = Getenv_warning("MONGOVOLUMEIMAGE")
	kafkaVolumeImage = Getenv_warning("KAFKAVOLUMEIMAGE")
	neo4jVolumeImage = Getenv_warning("NEO4JVOLUMEIMAGE")
	stormExternalImage = Getenv_warning("STORMEXTERNALIMAGE")
	ocspImage = Getenv_warning("OCSPIMAGE")
	dataikuImage = Getenv_warning("DATAIKUIMAGE")
	anacondaImage = Getenv_warning("ANACONDAIMAGE")
	{
		mariadbGaleraHostPathNodeLabels = map[string]string{}
		nodeLabels := strings.Split(Getenv_warning("MARIADBGALERAHOSTPATHNODELABELS"), ",")
		for _, label := range nodeLabels {
			words := strings.SplitN(label, "=", 2)
			if len(words) >= 2 {
				k, v := words[0], words[1]
				if k != "" && v != "" {
					mariadbGaleraHostPathNodeLabels[k] = v
				}
			}
		}
	}
	hostPathServiceAccount = Getenv_warning("HOSTPATHSERVICEACCOUNT")
	mariadbGaleraHostPathDataPath = Getenv_warning("MARIADBGALERAHOSTPATHDATAPATH")
	mariadbImageForHostPath = Getenv_warning("MARIADBIMAGEFORHOSTPATH")
	mariadbImage = Getenv_warning("MARIADBIMAGE")
	prometheusMysqldExporterImage = Getenv_warning("PROMETHEUSMYSQLEXPORTERIMAGE")
	phpMyAdminImage = Getenv_warning("PHPMYADMINIMAGE")

	esclusterImage = Getenv_warning("ESCLUSTERIMAGE")
}
