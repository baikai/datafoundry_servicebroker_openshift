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
}

type Volume struct {
	Volume_size int    `json:"volume_size"`
	Volume_name string `json:"volume_name"`
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

func getenv(env string) string {
	env_value := os.Getenv(env)
	if env_value == "" {
		fmt.Println("FATAL: NEED ENV", env)
		fmt.Println("Exit...........")
		os.Exit(2)
	}
	fmt.Println("ENV:", env, env_value)
	return env_value
}

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
var mariadbImage string
var prometheusMysqldExporterImage string
var phpMyAdminImage string

// added by Jared
var esclusterImage string

func init() {
	theOC = newOpenshiftClient(
		getenv("OPENSHIFTADDR"),
		getenv("OPENSHIFTUSER"),
		getenv("OPENSHIFTPASS"),
		getenv("SBNAMESPACE"),
	)

	svcDomainSuffix = os.Getenv("SERVICEDOMAINSUFFIX")
	if svcDomainSuffix == "" {
		svcDomainSuffix = "svc.cluster.local"
	}
	svcDomainSuffixWithDot = "." + svcDomainSuffix

	endpointSuffix = getenv("ENDPOINTSUFFIX")

	storageClassName = getenv("STORAGECLASSNAME")
	dfProxyApiPrefix = os.Getenv("DATAFOUNDRYPROXYADDR")
	if dfProxyApiPrefix == "" {
		logger.Error("int dfProxyApiPrefix error:", errors.New("DATAFOUNDRYPROXYADDR env is not set"))
	}
	dfProxyApiPrefix = "http://" + dfProxyApiPrefix + "/lapi/v1"

	dnsmasqServer = getenv("DNSMASQ_SERVER")

	nodeAddresses = strings.Split(getenv("NODE_ADDRESSES"), ",")
	nodeDemains = strings.Split(getenv("NODE_DOMAINS"), ",")
	externalZookeeperServers = strings.Split(getenv("EXTERNALZOOKEEPERSERVERS"), ",")

	ocspOcm = getenv("OCSP_OCM")
	ocspOcmPort = getenv("OCSP_OCM_PORT")
	ocspHdpVersion = getenv("OCSP_HDP_VERSION")

	etcdImage = getenv("ETCDIMAGE")
	etcdbootImage = getenv("ETCDBOOTIMAGE")
	zookeeperImage = getenv("ZOOKEEPERIMAGE")
	zookeeperexhibitorImage = getenv("ZOOKEEPEREXHIBITORIMAGE")
	redisImage = getenv("REDISIMAGE")
	redis32Image = getenv("REDIS32IMAGE")
	redisClusterImage = getenv("REDISCLUSTERIMAGE")
	redisStatImage = getenv("REDISSTATIMAGE")
	//redisClusterTribImage = getenv("REDISCLUSTERTRIBIMAGE")
	redisphpadminImage = getenv("REDISPHPADMINIMAGE")
	kafkaImage = getenv("KAFKAIMAGE")
	stormImage = getenv("STORMIMAGE")
	cassandraImage = getenv("CASSANDRAIMAGE")
	tensorflowImage = getenv("TENSORFLOWIMAGE")
	nifiImage = getenv("NIFIIMAGE")
	kettleImage = getenv("KETTLEIMAGE")
	simplefileuplaoderImage = getenv("SIMPLEFILEUPLOADERIMAGE")
	rabbitmqImage = getenv("RABBITMQIMAGE")
	sparkImage = getenv("SPARKIMAGE")
	zepplinImage = getenv("ZEPPLINIMAGE")
	pyspiderImage = getenv("PYSPIDERIMAGE")
	etcdVolumeImage = getenv("ETCDVOLUMEIMAGE")
	elasticsearchVolumeImage = getenv("ELASTICSEARCHVOLUMEIMAGE")
	mongoVolumeImage = getenv("MONGOVOLUMEIMAGE")
	kafkaVolumeImage = getenv("KAFKAVOLUMEIMAGE")
	neo4jVolumeImage = getenv("NEO4JVOLUMEIMAGE")
	stormExternalImage = getenv("STORMEXTERNALIMAGE")
	ocspImage = getenv("OCSPIMAGE")
	dataikuImage = getenv("DATAIKUIMAGE")
	anacondaImage = getenv("ANACONDAIMAGE")
	{
		mariadbGaleraHostPathNodeLabels = map[string]string{}
		nodeLabels := strings.Split(getenv("MARIADBGALERAHOSTPATHNODELABELS"), ",")
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
	hostPathServiceAccount = getenv("HOSTPATHSERVICEACCOUNT")
	mariadbGaleraHostPathDataPath = getenv("MARIADBGALERAHOSTPATHDATAPATH")
	mariadbImage = getenv("MARIADBIMAGE")
	prometheusMysqldExporterImage = getenv("PROMETHEUSMYSQLEXPORTERIMAGE")
	phpMyAdminImage = getenv("PHPMYADMINIMAGE")
	

	esclusterImage = getenv("ESCLUSTERIMAGE")
}
