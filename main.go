package main

import (
	"bufio"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/etcd/client"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-golang/lager"
	"golang.org/x/net/context"

	oshandler "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/handler"
	bsiapi "github.com/openshift/origin/backingserviceinstance/api/v1"

	"github.com/asiainfoLDP/datafoundry_servicebroker_openshift/handler"

	_ "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/servicebroker/cassandra"
	_ "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/servicebroker/etcd"
	_ "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/servicebroker/kafka"
	_ "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/servicebroker/kettle"
	_ "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/servicebroker/nifi"
	_ "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/servicebroker/ocsp"
	_ "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/servicebroker/pyspider"
	_ "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/servicebroker/rabbitmq"
	_ "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/servicebroker/redis"
	_ "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/servicebroker/spark"
	_ "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/servicebroker/storm"
	_ "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/servicebroker/tensorflow"
	_ "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/servicebroker/zookeeper"

	_ "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/servicebroker/elasticsearch_pvc"
	_ "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/servicebroker/etcd-pvc"
	_ "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/servicebroker/kafka_pvc"
	_ "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/servicebroker/mongo_pvc"
	_ "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/servicebroker/neo4j_pvc"
	_ "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/servicebroker/rabbitmq_pvc"
	_ "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/servicebroker/redis_pvc"
	_ "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/servicebroker/storm_pvc"
	_ "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/servicebroker/zookeeper_pvc"

	_ "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/servicebroker/redissingle_pvc"
	_ "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/servicebroker/storm_external"
	_ "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/servicebroker/dataiku"
	_ "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/servicebroker/rediscluster_pvc"
	_ "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/servicebroker/anaconda3"
	_ "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/servicebroker/zeppelin"
	_ "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/servicebroker/rediscluster_with_replicas_pvc"
	_ "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/servicebroker/mysql_galera_pvc"
	_ "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/servicebroker/mysql_galera_hostpath"
	_ "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/servicebroker/elasticsearch"
	_ "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/servicebroker/dataiku_pvc"
	_ "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/servicebroker/anaconda3_pvc"
)

func initETCD() {
	_, err := etcdapi.Get(context.Background(), "/servicebroker", &client.GetOptions{Recursive: false})
	if err == nil {
		return
	}

	etcdRegInitInfo := handler.EtcdRegistryInitInfo()
	if etcdRegInitInfo == "" {
		return
	}
	
	// format: init_script_file:api_user:api_password
	
	params := strings.SplitN(etcdRegInitInfo, ":", 3)
	
	etcdInitScriptFile, apiUser, apiPassword := params[0], "", ""
	if len(params) > 1 {
		apiUser = params[1]
	}
	if len(params) > 2 {
		apiPassword = params[2]
	}
	
	f, err := os.Open(etcdInitScriptFile)
	if err != nil {
		logger.Error("error opening etcdInitScriptFile: " + etcdInitScriptFile, err)
		os.Exit(1)
	}
	
	r := bufio.NewReader(f)
	for {
		line, err := handler.Readln(r)
		if err != nil {
			if err == io.EOF {
				break
			}
			logger.Error("error readline etcdInitScriptFile", err)
			os.Exit(1)
		}
		
		const etcdCmd = "$ETCDCTL"
		if !strings.HasPrefix(line, etcdCmd) {
			continue
		}

		line = strings.TrimLeft(line[len(etcdCmd):], " ")
		index := strings.Index(line, " ")
		if index <= 0 {
			continue
		}

		subCmd := line[:index]
		remaining := line[index+1:]
		keyvalue := strings.SplitN(remaining, " ", 2)
		key, value := strings.TrimSpace(keyvalue[0]), ""
		if len(keyvalue) > 1 {
			const cutset = ` '"`
			value = strings.Trim(keyvalue[1], cutset)
		}

		fmt.Println("etcd", subCmd, keyvalue)

		switch subCmd {
		case "mkdir":
			if key != "" {
				_, err = etcdapi.Set(context.Background(), key, "", &client.SetOptions{Dir: true})
				err = nil // tolerate mkdir errors
			}
		case "set":
			switch key {
			case "":
			case "/servicebroker/"+servcieBrokerName+"/username":
				if apiUser != "" {
					value = apiUser
				}
				_, err = etcdapi.Set(context.Background(), key, value, nil)
			case "/servicebroker/"+servcieBrokerName+"/password":
				if apiPassword != "" {
					value = apiPassword
				}
				_, err = etcdapi.Set(context.Background(), key, value, nil)
			default:
				if value != "" {
					_, err = etcdapi.Set(context.Background(), key, value, nil)
				}
			}
		}

		if err != nil {
			logger.Error("etcd " + subCmd + " " + key + " error:", err)
			os.Exit(1)
		}
	}
}

type myServiceBroker struct {
}

type serviceInfo struct {
	Service_name   string `json:"service_name"`
	Plan_name      string `json:"plan_name"`
	Url            string `json:"url"`
	Admin_user     string `json:"admin_user,omitempty"`
	Admin_password string `json:"admin_password,omitempty"`
	Database       string `json:"database,omitempty"`
	User           string `json:"user"`
	Password       string `json:"password"`
}

type myCredentials struct {
	Uri      string `json:"uri"`
	Hostname string `json:"host"`
	Port     string `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	Name     string `json:"name,omitempty"`
}

// Services returns the information of all available backing services.
func (myBroker *myServiceBroker) Services() []brokerapi.Service {
	/*
		//free := true
		paid := false
		return []brokerapi.Service{
			brokerapi.Service{
				ID              : "5E397661-1385-464A-8DB7-9C4DF8CC0662",
				Name            : "etcd_openshift",
				Description     : "etcd service",
				Bindable        : true,
				Tags            : []string{"etcd"},
				PlanUpdatable   : false,
				Plans           : []brokerapi.ServicePlan{
					brokerapi.ServicePlan{
						ID          : "204F8288-F8D9-4806-8661-EB48D94504B3",
						Name        : "standalone",
						Description : "each user has a standalone etcd cluster",
						Free        : &paid,
						Metadata    : &brokerapi.ServicePlanMetadata{
							DisplayName : "Big Bunny",
							Bullets     : []string{"20 GB of Disk","20 connections"},
							Costs       : []brokerapi.ServiceCost{
								brokerapi.ServiceCost{
									Amount : map[string]float64{"usd":99.0,"eur":49.0},
									Unit   : "MONTHLY",
								},
							},
						},
					},
				},
				Metadata        : &brokerapi.ServiceMetadata{
					DisplayName         : "etcd",
					ImageUrl            : "https://coreos.com/assets/images/media/etcd2-0.png",
					LongDescription     : "Managed, highly available etcd clusters in the cloud",
					ProviderDisplayName : "Asiainfo BDX LDP",
					DocumentationUrl    : "https://coreos.com/etcd/docs/latest",
					SupportUrl          : "https://coreos.com/",
				},
				DashboardClient : &brokerapi.ServiceDashboardClient{},
			},
		}
	*/

	//todo还需要考虑对于service和plan的隐藏参数，status，比如可以用，不可用，已经删除等。删除应该是软删除，后两者不予以显示，前者表示还有数据
	//获取catalog信息
	resp, err := etcdapi.Get(context.Background(), "/servicebroker/"+servcieBrokerName+"/catalog", &client.GetOptions{Recursive: true}) //改为环境变量
	if err != nil {
		logger.Error("Can not get catalog information from etcd", err) //所有这些出错消息最好命名为常量，放到开始的时候
		return []brokerapi.Service{}
	} else {
		logger.Debug("Successful get catalog information from etcd. NodeInfo is " + resp.Node.Key)
	}

	myServices := []brokerapi.Service{}
	for i := 0; i < len(resp.Node.Nodes); i++ {
		logger.Debug("Start to Parse Service " + resp.Node.Nodes[i].Key)
		//在下一级循环外设置id，因为他是目录名字，注意，如果按照这个逻辑，id一定要是uuid，中间一定不能有目录符号"/"
		myService := brokerapi.Service{}
		myService.ID = strings.Split(resp.Node.Nodes[i].Key, "/")[len(strings.Split(resp.Node.Nodes[i].Key, "/"))-1]
		//开始取service级别除了ID以外的其他参数
		for j := 0; j < len(resp.Node.Nodes[i].Nodes); j++ {
			if !resp.Node.Nodes[i].Nodes[j].Dir {
				lowerkey := strings.ToLower(resp.Node.Nodes[i].Key)
				switch strings.ToLower(resp.Node.Nodes[i].Nodes[j].Key) {
				case lowerkey + "/name":
					myService.Name = resp.Node.Nodes[i].Nodes[j].Value
				case lowerkey + "/description":
					myService.Description = resp.Node.Nodes[i].Nodes[j].Value
				case lowerkey + "/bindable":
					myService.Bindable, _ = strconv.ParseBool(resp.Node.Nodes[i].Nodes[j].Value)
				case lowerkey + "/tags":
					myService.Tags = strings.Split(resp.Node.Nodes[i].Nodes[j].Value, ",")
				case lowerkey + "/planupdatable":
					myService.PlanUpdatable, _ = strconv.ParseBool(resp.Node.Nodes[i].Nodes[j].Value)
				case lowerkey + "/metadata":
					json.Unmarshal([]byte(resp.Node.Nodes[i].Nodes[j].Value), &myService.Metadata)
				}
			} else if strings.HasSuffix(strings.ToLower(resp.Node.Nodes[i].Nodes[j].Key), "plan") {
				//开始解析套餐目录中的套餐计划plan。上述判断也不是太严谨，比如有目录如果是xxxxplan怎么办？
				myPlans := []brokerapi.ServicePlan{}
				for k := 0; k < len(resp.Node.Nodes[i].Nodes[j].Nodes); k++ {
					logger.Debug("Start to Parse Plan " + resp.Node.Nodes[i].Nodes[j].Nodes[k].Key)
					myPlan := brokerapi.ServicePlan{}
					myPlan.ID = strings.Split(resp.Node.Nodes[i].Nodes[j].Nodes[k].Key, "/")[len(strings.Split(resp.Node.Nodes[i].Nodes[j].Nodes[k].Key, "/"))-1]
					for n := 0; n < len(resp.Node.Nodes[i].Nodes[j].Nodes[k].Nodes); n++ {
						lowernodekey := strings.ToLower(resp.Node.Nodes[i].Nodes[j].Nodes[k].Key)
						switch strings.ToLower(resp.Node.Nodes[i].Nodes[j].Nodes[k].Nodes[n].Key) {
						case lowernodekey + "/name":
							myPlan.Name = resp.Node.Nodes[i].Nodes[j].Nodes[k].Nodes[n].Value
						case lowernodekey + "/description":
							myPlan.Description = resp.Node.Nodes[i].Nodes[j].Nodes[k].Nodes[n].Value
						case lowernodekey + "/free":
							//这里没有搞懂为什么brokerapi里面的这个bool要定义为传指针的模式
							myPlanfree, _ := strconv.ParseBool(resp.Node.Nodes[i].Nodes[j].Nodes[k].Nodes[n].Value)
							myPlan.Free = brokerapi.FreeValue(myPlanfree)
						case lowernodekey + "/metadata":
							json.Unmarshal([]byte(resp.Node.Nodes[i].Nodes[j].Nodes[k].Nodes[n].Value), &myPlan.Metadata)
						case lowernodekey + "/schemas":
							json.Unmarshal([]byte(resp.Node.Nodes[i].Nodes[j].Nodes[k].Nodes[n].Value), &myPlan.Schemas)
						}
					}
					//装配plan需要返回的值，按照有多少个plan往里面装
					myPlans = append(myPlans, myPlan)
				}
				//将装配好的Plan对象赋值给Service
				myService.Plans = myPlans

			}
		}

		//装配catalog需要返回的值，按照有多少个服务往里面装
		myServices = append(myServices, myService)
	}

	return myServices

}

// Provision tries to create a new backing service instance according the settings provided by client.
func (myBroker *myServiceBroker) Provision(
	instanceID string,
	details brokerapi.ProvisionDetails,
	asyncAllowed bool,
) (brokerapi.ProvisionedServiceSpec, error) {

	var provsiondetail brokerapi.ProvisionedServiceSpec
	var myServiceInfo handler.ServiceInfo

	//判断实例是否已经存在，如果存在就报错
	resp, err := etcdget("/servicebroker/" + servcieBrokerName + "/instance") //改为环境变量

	if err != nil {
		logger.Error("Can't connet to etcd", err)
		return brokerapi.ProvisionedServiceSpec{}, errors.New("Can't connet to etcd")
	}

	for i := 0; i < len(resp.Node.Nodes); i++ {
		if resp.Node.Nodes[i].Dir && strings.HasSuffix(resp.Node.Nodes[i].Key, instanceID) {
			logger.Info("ErrInstanceAlreadyExists")
			return brokerapi.ProvisionedServiceSpec{}, brokerapi.ErrInstanceAlreadyExists
		}
	}

	//判断servcie_id和plan_id是否正确
	service_name := findServiceNameInCatalog(details.ServiceID)
	plan_name := findServicePlanNameInCatalog(details.ServiceID, details.PlanID)
	//todo 应该修改service broker添加一个用户输入出错的返回，而不是500
	if service_name == "" || plan_name == "" {
		msg := "Service_id (" + service_name + ":" +
			details.ServiceID + ") or plan (" + plan_name + ":" +
			details.PlanID + ")_id not correct!!"
		logger.Info(msg)
		return brokerapi.ProvisionedServiceSpec{}, errors.New(msg)
	}
	//是否要检查service和plan的status是否允许创建 todo

	//生成具体的handler对象
	myHandler, err := handler.New(service_name + "_" + plan_name)

	//没有找到具体的handler，这里如果没有找到具体的handler不是由于用户输入的，是不对的，报500错误
	if err != nil {
		logger.Error("Can not found handler for service "+service_name+" plan "+plan_name, err)
		return brokerapi.ProvisionedServiceSpec{}, errors.New("Internal Error!!")
	}

	volumeSize, connections, customization, err := findServicePlanInfo(
		details.ServiceID, details.PlanID, details.Parameters, false)
	if err != nil {
		logger.Error("findServicePlanInfo service "+service_name+" plan "+plan_name, err)
		return brokerapi.ProvisionedServiceSpec{}, errors.New("Internal Error!!")
	}

	planInfo := handler.PlanInfo{
		Volume_size: volumeSize,
		Connections: connections,

		MoreParameters:    details.Parameters,
		ParameterSettings: customization,
	}

	//volumeSize, err = getVolumeSize(details, planInfo)
	//if err != nil {
	//	logger.Error("getVolumeSize service "+service_name+" plan "+plan_name, err)
	//	return brokerapi.ProvisionedServiceSpec{}, errors.New("Internal Error!!")
	//} else {
	//	planInfo.Volume_size = volumeSize
	//}

	etcdSaveResult := make(chan error, 1)

	//执行handler中的命令
	provsiondetail, myServiceInfo, err = myHandler.DoProvision(etcdSaveResult, instanceID, details, planInfo, asyncAllowed)
	if err != nil {
		etcdSaveResult <- errors.New("DoProvision Error!")
		logger.Error("Error do handler for service "+service_name+" plan "+plan_name, err)
		return brokerapi.ProvisionedServiceSpec{}, err ///errors.New("Internal Error!!")
	}

	//为隐藏属性添加上必要的变量
	myServiceInfo.Service_name = service_name
	myServiceInfo.Plan_name = plan_name

	//写入etcd 话说如果这个时候写入失败，那不就出现数据不一致的情况了么！todo
	//先创建instanceid目录
	_, err = etcdapi.Set(context.Background(), 
		"/servicebroker/"+servcieBrokerName+"/instance/"+instanceID,
		"",
		&client.SetOptions{Dir: true})
	if err != nil {
		etcdSaveResult <- errors.New("etcdapi.Set instance Error!")
		logger.Error("Can not create instance "+instanceID+" in etcd", err) //todo都应该改为日志key
		return brokerapi.ProvisionedServiceSpec{}, err
	} else {
		logger.Debug("Successful create instance "+instanceID+" in etcd", nil)
	}
	//然后创建一系列属性
	etcdset("/servicebroker/"+servcieBrokerName+"/instance/"+instanceID+"/organization_guid", details.OrganizationGUID)
	etcdset("/servicebroker/"+servcieBrokerName+"/instance/"+instanceID+"/space_guid", details.SpaceGUID)
	etcdset("/servicebroker/"+servcieBrokerName+"/instance/"+instanceID+"/service_id", details.ServiceID)
	etcdset("/servicebroker/"+servcieBrokerName+"/instance/"+instanceID+"/plan_id", details.PlanID)
	tmpval, _ := json.Marshal(details.Parameters)
	etcdset("/servicebroker/"+servcieBrokerName+"/instance/"+instanceID+"/parameters", string(tmpval))
	etcdset("/servicebroker/"+servcieBrokerName+"/instance/"+instanceID+"/dashboardurl", provsiondetail.DashboardURL)
	//存储隐藏信息_info
	tmpval, _ = json.Marshal(myServiceInfo)
	etcdset("/servicebroker/"+servcieBrokerName+"/instance/"+instanceID+"/_info", string(tmpval))

	//创建绑定目录
	_, err = etcdapi.Set(context.Background(),
		"/servicebroker/"+servcieBrokerName+"/instance/"+instanceID+"/binding",
		"",
		&client.SetOptions{Dir: true})
	if err != nil {
		etcdSaveResult <- errors.New("etcdapi.Set binding Error!")
		logger.Error("Can not create banding directory of  "+instanceID+" in etcd", err) //todo都应该改为日志key
		return brokerapi.ProvisionedServiceSpec{}, err
	}
	logger.Debug("Successful create banding directory of  "+instanceID+" in etcd", nil)

	//完成所有操作后，返回DashboardURL和是否异步的标志
	logger.Info("Successful create instance " + instanceID)
	etcdSaveResult <- nil

	if asyncResult := myServiceInfo.AsyncResult(); asyncResult != nil {
		go func() {
			if err := <-asyncResult; err != nil {			
				myServiceInfo.ProvisionFailureInfo = err.Error()
				tmpval, _ = json.Marshal(myServiceInfo)
				etcdset("/servicebroker/"+servcieBrokerName+"/instance/"+instanceID+"/_info", string(tmpval))
			}
		}()
	}
	
	return provsiondetail, nil
}

// Update tries to modify a new backing service instance according the settings provided by client.
// For volume size modification request, Update only support volume expanding now.
func (myBroker *myServiceBroker) Update(
	instanceID string,
	details brokerapi.UpdateDetails,
	asyncAllowed bool,
) (brokerapi.IsAsync, error) {

	var myServiceInfo handler.ServiceInfo

	//判断实例是否已经存在，如果不存在就报错
	resp, err := etcdapi.Get(context.Background(),
		"/servicebroker/"+servcieBrokerName+"/instance/"+instanceID,
		&client.GetOptions{Recursive: true})
	if err != nil || !resp.Node.Dir {
		logger.Error("Can not get instance information from etcd", err)
		return brokerapi.IsAsync(false), brokerapi.ErrInstanceDoesNotExist
	} else {
		logger.Debug("Successful get instance information from etcd. NodeInfo is " + resp.Node.Key)
	}

	var servcie_id, plan_id string
	//从etcd中取得参数。
	for i := 0; i < len(resp.Node.Nodes); i++ {
		if !resp.Node.Nodes[i].Dir {
			switch strings.ToLower(resp.Node.Nodes[i].Key) {
			case strings.ToLower(resp.Node.Key) + "/service_id":
				servcie_id = resp.Node.Nodes[i].Value
			case strings.ToLower(resp.Node.Key) + "/plan_id":
				plan_id = resp.Node.Nodes[i].Value
			}
		}
	}

	// todo: maybe
	// if servcie_id != details.PreviousValues.ServiceID || plan_id != details.PreviousValues.PlanID {
	// }

	//并且要核对一下detail里面的service_id和plan_id。出错消息现在是500，需要更改一下源代码，以便更改出错代码
	if servcie_id != details.ServiceID || plan_id != details.PlanID {
		logger.Info("ServiceID or PlanID not correct!!")
		return brokerapi.IsAsync(false), errors.New("ServiceID or PlanID not correct!! instanceID " + instanceID)
	}
	//是否要判断里面有没有绑定啊？todo

	//判断servcie_id和plan_id是否正确
	service_name := findServiceNameInCatalog(details.ServiceID)
	plan_name := findServicePlanNameInCatalog(details.ServiceID, details.PlanID)
	//todo 应该修改service broker添加一个用户输入出错的返回，而不是500
	if service_name == "" || plan_name == "" {
		logger.Info("Service_id or plan_id not correct!!")
		return false, errors.New("Service_id or plan_id not correct!!")
	}
	//是否要检查service和plan的status是否允许创建 todo

	//根据存储在etcd中的service_name和plan_name来确定到底调用那一段处理。注意这个时候不能像Provision一样去catalog里面读取了。
	//因为这个时候的数据不一定和创建的时候一样，plan等都有可能变化。同样的道理，url，用户名，密码都应该从_info中解码出来

	//隐藏属性不得不单独获取
	resp, err = etcdget("/servicebroker/" + servcieBrokerName + "/instance/" + instanceID + "/_info")
	if err != nil {
		logger.Error("etcdget", err)
		return brokerapi.IsAsync(false), err
	}
	err = json.Unmarshal([]byte(resp.Node.Value), &myServiceInfo)
	if err != nil {
		logger.Error("Unmarshal", err)
		return brokerapi.IsAsync(false), err
	}

	//生成具体的handler对象
	myHandler, err := handler.New(myServiceInfo.Service_name + "_" + myServiceInfo.Plan_name)

	//没有找到具体的handler，这里如果没有找到具体的handler不是由于用户输入的，是不对的，报500错误
	if err != nil {
		logger.Error("Can not found handler for service "+myServiceInfo.Service_name+" plan "+myServiceInfo.Plan_name, err)
		return brokerapi.IsAsync(false), errors.New("Internal Error!!")
	}

	// ...
	hasVolumes := len(myServiceInfo.Volumes) > 0

	volumeSize, connections, customization, err := findServicePlanInfo(
		details.ServiceID, details.PlanID, details.Parameters, hasVolumes)
	if err != nil {
		//logger.Error("findServicePlanInfo service "+service_name+" plan "+plan_name, err)
		//return false, errors.New("Internal Error!!")
		logger.Info(fmt.Sprint("findServicePlanInfo service "+service_name+" plan "+plan_name, err))
		//} else {
		//	volumeSize = 0
		//	connections = 0
	}

	logger.Info(fmt.Sprint("volumeSize =", volumeSize, ", connections=", connections))

	//if len(myServiceInfo.Volumes) == 0 {
	//	reason := "can not get volume info from the old plan."
	//	logger.Info(reason)
	//	return false, errors.New(reason)
	//}
	if hasVolumes {
		//if volumeSize == myServiceInfo.Volumes[0].Volume_size {
		//	return false, nil
		//}

		if volumeSize < myServiceInfo.Volumes[0].Volume_size {
			reason := fmt.Sprintf(
				"new volume size %d must be larger than old sizes %d",
				volumeSize, myServiceInfo.Volumes[0].Volume_size,
			)
			logger.Info(reason)
			return false, errors.New(reason)
		}
	}

	// to update plan ...

	planInfo := handler.PlanInfo{
		Volume_size: volumeSize,
		Connections: connections,

		MoreParameters:    details.Parameters,
		ParameterSettings: customization,
	}

	callbackSaveNewInfo := func(serviceInfo *handler.ServiceInfo) error {
		//存储隐藏信息_info
		tmpval, _ := json.Marshal(myServiceInfo)
		_, err := etcdset("/servicebroker/"+servcieBrokerName+"/instance/"+instanceID+"/_info", string(tmpval))
		return err
	}

	//执行handler中的命令
	err = myHandler.DoUpdate(&myServiceInfo, planInfo, callbackSaveNewInfo, asyncAllowed)
	if err != nil {
		logger.Error("Error do handler for service "+service_name+" plan "+plan_name, err)
		return false, errors.New("Internal Error!!")
	}

	return true, nil
}

// LastOperation returns the progress of a creation of backing service instance.
func (myBroker *myServiceBroker) LastOperation(instanceID string) (brokerapi.LastOperation, error) {
	// If the broker provisions asynchronously, the Cloud Controller will poll this endpoint
	// for the status of the provisioning operation.

	var myServiceInfo handler.ServiceInfo
	var lastOperation brokerapi.LastOperation
	//判断实例是否已经存在，如果不存在就报错
	resp, err := etcdapi.Get(context.Background(),
		"/servicebroker/"+servcieBrokerName+"/instance/"+instanceID,
		&client.GetOptions{Recursive: true})
	if err != nil || !resp.Node.Dir {
		logger.Error("Can not get instance information from etcd", err)
		return brokerapi.LastOperation{}, brokerapi.ErrInstanceDoesNotExist
	} else {
		logger.Debug("Successful get instance information from etcd. NodeInfo is " + resp.Node.Key)
	}

	//隐藏属性不得不单独获取
	resp, err = etcdget("/servicebroker/" + servcieBrokerName + "/instance/" + instanceID + "/_info")
	if err != nil {
		logger.Error("etcdget", err)
		return brokerapi.LastOperation{}, err
	}
	err = json.Unmarshal([]byte(resp.Node.Value), &myServiceInfo)
	if err != nil {
		logger.Error("Unmarshal", err)
		return brokerapi.LastOperation{}, err
	}

	//如果没有找到具体的handler，这里如果没有找到具体的handler不是由于用户输入的，是不对的，报500错误
	myHandler, err := handler.New(myServiceInfo.Service_name + "_" + myServiceInfo.Plan_name)
	if err != nil {
		logger.Error("Can not found handler for service "+myServiceInfo.Service_name+" plan "+myServiceInfo.Plan_name, err)
		return brokerapi.LastOperation{}, errors.New("Internal Error!!")
	}

	//执行handler中的命令
	lastOperation, err = myHandler.DoLastOperation(&myServiceInfo)
	if err != nil {
		logger.Error("Error do handler for service "+myServiceInfo.Service_name+" plan "+myServiceInfo.Plan_name, err)
		return brokerapi.LastOperation{}, errors.New("Internal Error!!")
	}

	//如果一切正常，返回结果
	logger.Info("Successful query last operation for service instance" + instanceID)
	return lastOperation, nil
}

// Deprovision destroys a backing service instance.
func (myBroker *myServiceBroker) Deprovision(instanceID string, details brokerapi.DeprovisionDetails, asyncAllowed bool) (brokerapi.IsAsync, error) {

	var myServiceInfo handler.ServiceInfo

	//判断实例是否已经存在，如果不存在就报错
	resp, err := etcdapi.Get(context.Background(),
		"/servicebroker/"+servcieBrokerName+"/instance/"+instanceID,
		&client.GetOptions{Recursive: true})
	if err != nil || !resp.Node.Dir {
		logger.Error("Can not get instance information from etcd", err)
		return brokerapi.IsAsync(false), brokerapi.ErrInstanceDoesNotExist
	} else {
		logger.Debug("Successful get instance information from etcd. NodeInfo is " + resp.Node.Key)
	}

	var service_id, plan_id string
	//从etcd中取得参数。
	for i := 0; i < len(resp.Node.Nodes); i++ {
		if !resp.Node.Nodes[i].Dir {
			switch strings.ToLower(resp.Node.Nodes[i].Key) {
			case strings.ToLower(resp.Node.Key) + "/service_id":
				service_id = resp.Node.Nodes[i].Value
			case strings.ToLower(resp.Node.Key) + "/plan_id":
				plan_id = resp.Node.Nodes[i].Value
			}
		}
	}

	//并且要核对一下detail里面的service_id和plan_id。出错消息现在是500，需要更改一下源代码，以便更改出错代码
	if service_id != details.ServiceID || plan_id != details.PlanID {
		logger.Info("ServiceID or PlanID not correct!!")
		return brokerapi.IsAsync(false), errors.New("ServiceID (" + service_id + ") or PlanID (" + plan_id + ") not correct!! instanceID " + instanceID)
	}
	//是否要判断里面有没有绑定啊？todo

	//根据存储在etcd中的service_name和plan_name来确定到底调用那一段处理。注意这个时候不能像Provision一样去catalog里面读取了。
	//因为这个时候的数据不一定和创建的时候一样，plan等都有可能变化。同样的道理，url，用户名，密码都应该从_info中解码出来

	//隐藏属性不得不单独获取
	resp, err = etcdget("/servicebroker/" + servcieBrokerName + "/instance/" + instanceID + "/_info")
	if err != nil {
		logger.Error("etcdget", err)
		return brokerapi.IsAsync(false), err
	}
	err = json.Unmarshal([]byte(resp.Node.Value), &myServiceInfo)
	if err != nil {
		logger.Error("Unmarshal", err)
		return brokerapi.IsAsync(false), err
	}

	//生成具体的handler对象
	myHandler, err := handler.New(myServiceInfo.Service_name + "_" + myServiceInfo.Plan_name)

	//没有找到具体的handler，这里如果没有找到具体的handler不是由于用户输入的，是不对的，报500错误
	if err != nil {
		logger.Error("Can not found handler for service "+myServiceInfo.Service_name+" plan "+myServiceInfo.Plan_name, err)
		return brokerapi.IsAsync(false), errors.New("Internal Error!!")
	}

	//执行handler中的命令
	isasync, err := myHandler.DoDeprovision(&myServiceInfo, asyncAllowed)
	if err != nil {
		logger.Error("Error do handler for service "+myServiceInfo.Service_name+" plan "+myServiceInfo.Plan_name, err)
		return brokerapi.IsAsync(false), errors.New("Internal Error!!")
	}

	//然后删除etcd里面的纪录，这里也有可能有不一致的情况
	_, err = etcdapi.Delete(context.Background(),
		"/servicebroker/"+servcieBrokerName+"/instance/"+instanceID,
		&client.DeleteOptions{Recursive: true, Dir: true})
	if err != nil {
		logger.Error("Can not delete instance "+instanceID+" in etcd", err) //todo都应该改为日志key
		return brokerapi.IsAsync(false), errors.New("Internal Error!!")
	} else {
		logger.Debug("Successful delete instance " + instanceID + " in etcd")
	}

	logger.Info("Successful Deprovision instance " + instanceID)
	return isasync, nil
}

// Bind adds a binding information for a backing service instance.
func (myBroker *myServiceBroker) Bind(instanceID, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, error) {
	var mycredentials handler.Credentials
	var myBinding brokerapi.Binding
	//判断实例是否已经存在，如果不存在就报错
	resp, err := etcdget("/servicebroker/" + servcieBrokerName + "/instance/" + instanceID)
	if err != nil || !resp.Node.Dir {
		logger.Error("Can not get instance information from etcd", err) //所有这些出错消息最好命名为常量，放到开始的时候
		return brokerapi.Binding{}, brokerapi.ErrInstanceDoesNotExist
	} else {
		logger.Debug("Successful get instance information from etcd. NodeInfo is " + resp.Node.Key)
	}

	//判断绑定是否存在，如果存在就报错
	resp, err = etcdget("/servicebroker/" + servcieBrokerName + "/instance/" + instanceID + "/binding")
	for i := 0; i < len(resp.Node.Nodes); i++ {
		if resp.Node.Nodes[i].Dir && strings.HasSuffix(resp.Node.Nodes[i].Key, bindingID) {
			logger.Info("ErrBindingAlreadyExists " + instanceID)
			return brokerapi.Binding{}, brokerapi.ErrBindingAlreadyExists
		}
	}

	//对于参数中的service_id和plan_id仅做校验，不再在binding中存储
	var servcie_id, plan_id string

	//从etcd中取得参数。
	resp, err = etcdapi.Get(context.Background(),
		"/servicebroker/"+servcieBrokerName+"/instance/"+instanceID,
		&client.GetOptions{Recursive: true})
	if err != nil {
		logger.Error("Can not get instance information from etcd", err) //所有这些出错消息最好命名为常量，放到开始的时候
		return brokerapi.Binding{}, brokerapi.ErrInstanceDoesNotExist
	} else {
		logger.Debug("Successful get instance information from etcd.")
	}
	for i := 0; i < len(resp.Node.Nodes); i++ {
		if !resp.Node.Nodes[i].Dir {
			switch strings.ToLower(resp.Node.Nodes[i].Key) {
			case strings.ToLower(resp.Node.Key) + "/service_id":
				servcie_id = resp.Node.Nodes[i].Value
			case strings.ToLower(resp.Node.Key) + "/plan_id":
				plan_id = resp.Node.Nodes[i].Value
			}
		}
	}

	//并且要核对一下detail里面的service_id和plan_id。出错消息现在是500，需要更改一下源代码，以便更改出错代码
	if servcie_id != details.ServiceID || plan_id != details.PlanID {
		logger.Info("ServiceID or PlanID not correct!!")
		return brokerapi.Binding{}, errors.New("ServiceID or PlanID not correct!! instanceID " + instanceID)
	}

	//隐藏属性不得不单独获取。取得当时绑定服务得到信息
	var myServiceInfo handler.ServiceInfo
	resp, err = etcdget("/servicebroker/" + servcieBrokerName + "/instance/" + instanceID + "/_info")
	if err != nil {
		logger.Error("etcdget", err)
		return brokerapi.Binding{}, err
	}
	err = json.Unmarshal([]byte(resp.Node.Value), &myServiceInfo)
	if err != nil {
		logger.Error("Unmarshal", err)
		return brokerapi.Binding{}, err
	}

	//如果没有找到具体的handler，这里如果没有找到具体的handler不是由于用户输入的，是不对的，报500错误
	myHandler, err := handler.New(myServiceInfo.Service_name + "_" + myServiceInfo.Plan_name)
	if err != nil {
		logger.Error("Can not found handler for service "+myServiceInfo.Service_name+" plan "+myServiceInfo.Plan_name, err)
		return brokerapi.Binding{}, errors.New("Internal Error!!")
	}

	//执行handler中的命令
	myBinding, mycredentials, err = myHandler.DoBind(&myServiceInfo, bindingID, details)
	if err != nil {
		logger.Error("Error do handler for service "+myServiceInfo.Service_name+" plan "+myServiceInfo.Plan_name, err)
		return brokerapi.Binding{}, err
	}

	//把信息存储到etcd里面，同样这里有同步性的问题 todo怎么解决呢？
	//先创建bindingID目录
	_, err = etcdapi.Set(context.Background(),
		"/servicebroker/"+servcieBrokerName+"/instance/"+instanceID+"/binding/"+bindingID,
		"",
		&client.SetOptions{Dir: true})
	if err != nil {
		logger.Error("Can not create binding "+bindingID+" in etcd", err) //todo都应该改为日志key
		return brokerapi.Binding{}, err
	} else {
		logger.Debug("Successful create binding "+bindingID+" in etcd", nil)
	}
	//然后创建一系列属性
	etcdset("/servicebroker/"+servcieBrokerName+"/instance/"+instanceID+"/binding/"+bindingID+"/app_guid", details.AppGUID)
	tmpval, _ := json.Marshal(details.Parameters)
	etcdset("/servicebroker/"+servcieBrokerName+"/instance/"+instanceID+"/binding/"+bindingID+"/parameters", string(tmpval))
	tmpval, _ = json.Marshal(myBinding)
	etcdset("/servicebroker/"+servcieBrokerName+"/instance/"+instanceID+"/binding/"+bindingID+"/binding", string(tmpval))
	//存储隐藏信息_info
	tmpval, _ = json.Marshal(mycredentials)
	etcdset("/servicebroker/"+servcieBrokerName+"/instance/"+instanceID+"/binding/"+bindingID+"/_info", string(tmpval))

	logger.Info("Successful create binding " + bindingID)
	return myBinding, nil
}

// Unbind removes a binding information for a backing service instance.
func (myBroker *myServiceBroker) Unbind(instanceID, bindingID string, details brokerapi.UnbindDetails) error {

	var mycredentials handler.Credentials
	var myServiceInfo handler.ServiceInfo
	//判断实例是否已经存在，如果不存在就报错
	resp, err := etcdapi.Get(context.Background(),
		"/servicebroker/"+servcieBrokerName+"/instance/"+instanceID,
		&client.GetOptions{Recursive: true})
	if err != nil || !resp.Node.Dir {
		logger.Error("Can not get instance information from etcd", err)
		return brokerapi.ErrInstanceDoesNotExist //这几个错误返回为空，是detele操作的要求吗？
	} else {
		logger.Debug("Successful get instance information from etcd. NodeInfo is " + resp.Node.Key)
	}

	var servcie_id, plan_id string

	//从etcd中取得参数。
	for i := 0; i < len(resp.Node.Nodes); i++ {
		if !resp.Node.Nodes[i].Dir {
			switch strings.ToLower(resp.Node.Nodes[i].Key) {
			case strings.ToLower(resp.Node.Key) + "/service_id":
				servcie_id = resp.Node.Nodes[i].Value
			case strings.ToLower(resp.Node.Key) + "/plan_id":
				plan_id = resp.Node.Nodes[i].Value
			}
		}
	}

	//并且要核对一下detail里面的service_id和plan_id。出错消息现在是500，需要更改一下源代码，以便更改出错代码
	if servcie_id != details.ServiceID || plan_id != details.PlanID {
		logger.Info("ServiceID or PlanID not correct!!")
		return errors.New("ServiceID or PlanID not correct!! instanceID " + instanceID)
	}

	//判断绑定是否存在，如果不存在就报错
	resp, err = etcdget("/servicebroker/" + servcieBrokerName + "/instance/" + instanceID + "/binding/" + bindingID)
	if err != nil || !resp.Node.Dir {
		logger.Error("Can not get binding information from etcd", err)
		return brokerapi.ErrBindingDoesNotExist //这几个错误返回为空，是detele操作的要求吗？
	} else {
		logger.Debug("Successful get bingding information from etcd. NodeInfo is " + resp.Node.Key)
	}

	//根据存储在etcd中的service_name和plan_name来确定到底调用那一段处理。注意这个时候不能像Provision一样去catalog里面读取了。
	//因为这个时候的数据不一定和创建的时候一样，plan等都有可能变化。同样的道理，url，用户名，密码都应该从_info中解码出来

	//隐藏属性不得不单独获取
	resp, err = etcdget("/servicebroker/" + servcieBrokerName + "/instance/" + instanceID + "/_info")
	if err != nil {
		logger.Error("etcdget", err)
		return err
	}
	err = json.Unmarshal([]byte(resp.Node.Value), &myServiceInfo)
	if err != nil {
		logger.Error("etcdget", err)
		return err
	}

	//隐藏属性不得不单独获取
	resp, err = etcdget("/servicebroker/" + servcieBrokerName + "/instance/" + instanceID + "/binding/" + bindingID + "/_info")
	if err != nil {
		logger.Error("etcdget", err)
		return err
	}
	err = json.Unmarshal([]byte(resp.Node.Value), &mycredentials)
	if err != nil {
		logger.Error("etcdget", err)
		return err
	}

	//没有找到具体的handler，这里如果没有找到具体的handler不是由于用户输入的，是不对的，报500错误
	myHandler, err := handler.New(myServiceInfo.Service_name + "_" + myServiceInfo.Plan_name)
	if err != nil {
		logger.Error("Can not found handler for service "+myServiceInfo.Service_name+" plan "+myServiceInfo.Plan_name, err)
		return errors.New("Internal Error!!")
	}

	//执行handler中的命令
	err = myHandler.DoUnbind(&myServiceInfo, &mycredentials)
	if err != nil {
		logger.Error("Error do handler for service "+myServiceInfo.Service_name+" plan "+myServiceInfo.Plan_name, err)
		return err
	}

	//然后删除etcd里面的纪录，这里也有可能有不一致的情况
	_, err = etcdapi.Delete(context.Background(),
		"/servicebroker/"+servcieBrokerName+"/instance/"+instanceID+"/binding/"+bindingID,
		&client.DeleteOptions{Recursive: true, Dir: true})
	if err != nil {
		logger.Error("Can not delete binding "+bindingID+" in etcd", err) //todo都应该改为日志key
		return errors.New("Can not delete binding " + bindingID + " in etcd")
	} else {
		logger.Debug("Successful delete binding "+bindingID+" in etcd", nil)
	}

	logger.Info("Successful delete binding "+bindingID, nil)
	return nil
}

// A robust way to get an item from ETCD.
func etcdget(key string) (*client.Response, error) {
	n := 5

RETRY:
	resp, err := etcdapi.Get(context.Background(), key, nil)
	if err != nil {
		logger.Error("Can not get "+key+" from etcd", err)
		n--
		if n > 0 {
			goto RETRY
		}

		return nil, err
	} else {
		logger.Debug("Successful get " + key + " from etcd. value is " + resp.Node.Value)
		return resp, nil
	}
}

// A robust way to query an item from ETCD.
func etcdset(key string, value string) (*client.Response, error) {
	n := 5

RETRY:
	resp, err := etcdapi.Set(context.Background(), key, value, nil)
	if err != nil {
		logger.Error("Can not set "+key+" from etcd", err)
		n--
		if n > 0 {
			goto RETRY
		}

		return nil, err
	} else {
		logger.Debug("Successful set " + key + " from etcd. value is " + value)
		return resp, nil
	}
}

func findServiceNameInCatalog(service_id string) string {
	resp, err := etcdget("/servicebroker/" + servcieBrokerName + "/catalog/" + service_id + "/name")
	if err != nil {
		return ""
	}
	return resp.Node.Value
}

func findServicePlanNameInCatalog(service_id, plan_id string) string {
	resp, err := etcdget("/servicebroker/" + servcieBrokerName + "/catalog/" + service_id + "/plan/" + plan_id + "/name")
	if err != nil {
		return ""
	}
	return resp.Node.Value
}

func findServicePlanInfo(service_id, plan_id string, parameters map[string]interface{}, requireVolumeParameter bool) (volumeSize, connections int, customization map[string]oshandler.CustomParams, err error) {
	vsize, conns, customization, err :=
		findServicePlanInfoInBulletsAndCustomizeSettings(service_id, plan_id)
	if err != nil {
		return
	}

	// default size is the value in etcd bullets.
	fVolumeSize := float64(vsize)

	// If input parameter also specifies volume size, then use it.
	// For Update, volume size must be specified in input parameter.
	// For Create, volume size is not required in input parameter,
	// but if it is not specified in input parameter, it must be present in plan.
	if interSize, ok := parameters[handler.VolumeSize]; !ok {
		//if requireVolumeParameter {
		//	err = errors.New(handler.VolumeSize + " parameter is not provided.")
		//	return
		//}
	} else {
		if sSize, ok := interSize.(string); !ok {
			err = errors.New(handler.VolumeSize + " is not string.")
			return
		} else if fSize, e := strconv.ParseFloat(sSize, 64); e != nil {
			err = e
			return
		} else {
			fVolumeSize = math.Floor(fSize + 0.5)
		}

		// todo: use this instead
		// fVolumeSize, err = handler.ParseFloat64(interSize)
		// if err != nil {
		//	return
		// }
	}

	// try to validate volume size
	if cus, ok := customization[handler.VolumeSize]; ok {
		// todo: use fVolumeSize = cus.Validate(fVolumeSize) instead
		fVolumeSize = cus.Default + cus.Step*math.Ceil((fVolumeSize-cus.Default)/cus.Step)
		if fVolumeSize > cus.Max {
			fVolumeSize = cus.Default // cus.Max
		} else if fVolumeSize < cus.Default {
			fVolumeSize = cus.Default
		}
	}

	// ...
	volumeSize = int(fVolumeSize)
	connections = conns

	return
}

func findServicePlanInfoInBulletsAndCustomizeSettings(service_id, plan_id string) (volumeSize, connections int, customization map[string]oshandler.CustomParams, err error) {
	resp, err := etcdget("/servicebroker/" + servcieBrokerName + "/catalog/" + service_id + "/plan/" + plan_id + "/metadata")
	if err != nil {
		return
	}

	type PlanMetaData struct {
		Bullets   []string                          `json:"bullets,omitempty"`
		Customize map[string]oshandler.CustomParams `json:"customize,omitempty"`
	}
	// metadata '{"bullets":["20 GB of Disk","20 connections"],"displayName":"Shared and Free" }'

	var meta PlanMetaData
	err = json.Unmarshal([]byte(resp.Node.Value), &meta)
	if err != nil {
		return
	}

	customization = meta.Customize

	for _, info := range meta.Bullets {
		info = strings.ToLower(info)
		if index := strings.Index(info, " gb of disk"); index > 0 {
			volumeSize, err = strconv.Atoi(info[:index])
			if err != nil {
				return
			}
		} else if index := strings.Index(info, " connection"); index > 0 {
			connections, err = strconv.Atoi(info[:index])
			if err != nil {
				return
			}
		}
	}

	return
}

/*
func getVolumeSize(details brokerapi.ProvisionDetails, planInfo oshandler.PlanInfo) (finalVolumeSize int, err error) {
	if planInfo.Customize == nil {
		//如果没有Customize, finalVolumeSize默认值 planInfo.Volume_size
		finalVolumeSize = planInfo.Volume_size
	} else if cus, ok := planInfo.Customize[handler.VolumeSize]; ok {
		if details.Parameters == nil {
			finalVolumeSize = int(cus.Default)
			return
		}
		if _, ok := details.Parameters[handler.VolumeSize]; !ok {
			err = errors.New("getVolumeSize:idetails.Parameters[volumeSize] not exist")
			println(err)
			return
		}
		sSize, ok := details.Parameters[handler.VolumeSize].(string)
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

func getmd5string(s string) string {
	h := md5.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

func getguid() string {
	b := make([]byte, 48)

	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return ""
	}
	return getmd5string(base64.URLEncoding.EncodeToString(b))
}

//func Getenv_must(env string) string {
//	env_value := os.Getenv(env)
//	if env_value == "" {
//		fmt.Println("FATAL: NEED ENV", env)
//		fmt.Println("Exit...........")
//		os.Exit(1)
//	}
//	fmt.Println("ENV:", env, env_value)
//	return env_value
//}

//定义日志和etcd的全局变量，以及其他变量
var logger lager.Logger
var etcdapi client.KeysAPI
var servcieBrokerName string = "openshift" // also used in init-etcd.sh
var etcdEndPoint, etcdUser, etcdPassword string
var serviceBrokerPort string
var brokerCredentials brokerapi.BrokerCredentials




func main() {

	//需要以下环境变量
	etcdEndPoint = handler.Getenv_must("ETCDENDPOINT") //etcd的路径
	etcdUser = handler.Getenv_opitional("ETCDUSER")
	etcdPassword = handler.Getenv_opitional("ETCDPASSWORD")
	serviceBrokerPort = handler.Getenv_must("BROKERPORT") //监听的端口

	//初始化日志对象，日志输出到stdout
	logger = lager.NewLogger(servcieBrokerName)
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.INFO)) //默认日志级别

	//初始化etcd客户端
	cfg := client.Config{
		Endpoints: []string{etcdEndPoint},
		Transport: client.DefaultTransport,
		// set timeout per request to fail fast when the target endpoint is unavailable
		HeaderTimeoutPerRequest: time.Second * 5,
		Username:                etcdUser,
		Password:                etcdPassword,
	}
	c, err := client.New(cfg)
	if err != nil {
		logger.Error("Can not init ectd client", err)
	}
	etcdapi = client.NewKeysAPI(c)
	
	initETCD()

	//初始化serviceborker对象
	serviceBroker := &myServiceBroker{}

	//取得用户名和密码
	var username, password string
	
	resp, err := etcdget("/servicebroker/" + servcieBrokerName + "/username")
	if err != nil {
		logger.Error("Can not init username,Progrom Exit!", err)
		os.Exit(1)
	} else {
		username = resp.Node.Value
	}

	resp, err = etcdget("/servicebroker/" + servcieBrokerName + "/password")
	if err != nil {
		logger.Error("Can not init password,Progrom Exit!", err)
		os.Exit(1)
	} else {
		password = resp.Node.Value
	}

	brokerCredentials := brokerapi.BrokerCredentials{
		Username: username,
		Password: password,
	}

	fmt.Println("Start service broker with the following services provided", servcieBrokerName)
	handler.ListHandler()
	brokerAPI := brokerapi.New(serviceBroker, logger, brokerCredentials)
	http.HandleFunc("/bsiinfo", getBsiInfo)
	http.Handle("/", brokerAPI)
	fmt.Println(http.ListenAndServe(":"+serviceBrokerPort, nil))
}

//============================

// infos
//    wild bsi pods/svcs/... which have no bsis related.
func getBsiInfo(w http.ResponseWriter, r *http.Request) {
	if user, pass, ok := r.BasicAuth(); !ok {
		w.Write([]byte("need auth"))
		return
	} else if user != etcdUser || pass != etcdPassword {
		w.Write([]byte("wrong auth, " + user + ", " + pass + ", " + etcdUser + ", " + etcdPassword))
		return
	}

	// steps:
	// 1. list all registed bsi under "/servicebroker/"+servcieBrokerName+"/instance/"
	// 2. list all bsi in df clusters (north1, north2, ...)
	// 3. list unregistered BSIs (expected none)
	// 4. list registered but no real BSI corresponded (expected many)
	// 5. create curl command list to release above resources
	//    create

	// ...

	println("========================== get BSIs in all namesapces")

	{
		bsis := bsiapi.BackingServiceInstanceList{}

		uri := "/backingserviceinstances"
		osr := oshandler.NewOpenshiftREST(oshandler.OC()).OGet(uri, &bsis)
		if osr.Err != nil {
			w.Write([]byte("get bsi list error: " + osr.Err.Error()))
			return
		}

		for i := range bsis.Items {
			bsi := &bsis.Items[i]
			println(bsi.Namespace, bsi.Name, bsi.Spec.InstanceID)
		}
	}

	// ...

	println("========================== get all registered instances")

	{
		instancesPrefix := "/servicebroker/" + servcieBrokerName + "/instance/"

		resp, err := etcdapi.Get(context.Background(),
			instancesPrefix[:len(instancesPrefix)-1],
			&client.GetOptions{Recursive: true})
		if err != nil {
			w.Write([]byte("list instanceid error: " + err.Error()))
			return
		}

		for i := range resp.Node.Nodes {
			node := resp.Node.Nodes[i]
			instanceID := strings.TrimPrefix(node.Key, instancesPrefix)
			println(instanceID)
		}
	}
}

