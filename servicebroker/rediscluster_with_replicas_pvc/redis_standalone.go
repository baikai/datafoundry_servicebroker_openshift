package rediscluster_with_replicas_pvc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	oshandler "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/handler"
	routeapi "github.com/openshift/origin/route/api/v1"
	dcapi "github.com/openshift/origin/deploy/api/v1"
	"github.com/pivotal-cf/brokerapi"
	//"github.com/pivotal-golang/lager"
	logger "github.com/golang/glog"
	kapi "k8s.io/kubernetes/pkg/api/v1"
	//"os"
	"strconv"
	"strings"
	"text/template"
	"time"
)

//==============================================================
//初始化Log
//==============================================================

const RedisClusterServcieBrokerName_Standalone = "Redis_volumes_cluster_with_replicas"

const DefaultNumNodes = 3    // 3 masters
const DefaultNumReplicas = 1 // zero slaves per master

const Key_EnableAuth = "ATTR_enable_auth"

func init() {
	oshandler.Register(RedisClusterServcieBrokerName_Standalone, &RedisCluster_freeHandler{})

	//logger = lager.NewLogger(RedisClusterServcieBrokerName_Standalone)
	//logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))
}

//var logger lager.Logger

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
//挂卷配置
//==============================================================

func volumeBaseName(instanceId string) string {
	return "rdsclstr-" + instanceId
}

//==============================================================
//
//==============================================================

func retrieveNumNodesFromPlanInfo(planInfo oshandler.PlanInfo, defaultNodes int) (numNodes int, err error) {
	nodesSettings, ok := planInfo.ParameterSettings[oshandler.Nodes]
	if !ok {
		err = errors.New(oshandler.Nodes + " settings not found")
		numNodes = defaultNodes
		return
	}

	nodes64, err := oshandler.ParseInt64(planInfo.MoreParameters[oshandler.Nodes])
	if err != nil {
		numNodes = defaultNodes
		return
	}
	numNodes = int(nodes64)

	if float64(numNodes) > nodesSettings.Max {
		err = fmt.Errorf("too many nodes specfied: %d > %f", numNodes, nodesSettings.Max)
	}

	if float64(numNodes) < nodesSettings.Default {
		err = fmt.Errorf("too few nodes specfied: %d < %f", numNodes, nodesSettings.Default)
	}

	numNodes = int(nodesSettings.Validate(float64(numNodes)))

	return
}

func retrieveNodeMemoryFromPlanInfo(planInfo oshandler.PlanInfo, defaultMemory int) (nodeMemory int, err error) {
	memorySettings, ok := planInfo.ParameterSettings[oshandler.Memory]
	if !ok {
		err = errors.New(oshandler.Memory + " settings not found")
		nodeMemory = defaultMemory
		return
	}

	fMemory, err := oshandler.ParseFloat64(planInfo.MoreParameters[oshandler.Memory])
	if err != nil {
		nodeMemory = defaultMemory
		return
	}

	if float64(fMemory) > memorySettings.Max {
		err = fmt.Errorf("too large memory specfied: %f > %f", fMemory, memorySettings.Max)
	}

	if float64(fMemory) < memorySettings.Default {
		err = fmt.Errorf("too small memory specfied: %f < %f", fMemory, memorySettings.Default)
	}

	fMemory = memorySettings.Validate(fMemory)
	nodeMemory = int(1000 * fMemory)

	return
}

func retrieveNumReplicasFromPlanInfo(planInfo oshandler.PlanInfo, defaultReplicas int) (numReplicas int, err error) {
	replicasSettings, ok := planInfo.ParameterSettings[oshandler.Replicas]
	if !ok {
		err = errors.New(oshandler.Replicas + " settings not found")
		numReplicas = defaultReplicas
		return
	}

	replicas64, err := oshandler.ParseInt64(planInfo.MoreParameters[oshandler.Replicas])
	if err != nil {
		numReplicas = defaultReplicas
		return
	}
	numReplicas = int(replicas64)

	if float64(numReplicas) > replicasSettings.Max {
		err = fmt.Errorf("too many replicas specfied: %d > %f", numReplicas, replicasSettings.Max)
	}

	if float64(numReplicas) < replicasSettings.Default {
		err = fmt.Errorf("too few replicas specfied: %d < %f", numReplicas, replicasSettings.Default)
	}

	numReplicas = int(replicasSettings.Validate(float64(numReplicas)))

	return
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

	// ...
	params := planInfo.MoreParameters // same as details.Parameters
	enableAuthParam, _ := oshandler.ParseString(params[Key_EnableAuth])
	enableAuthParam = strings.ToLower(enableAuthParam)
	enableAuth := enableAuthParam == "1" || enableAuthParam == "yes" || enableAuthParam == "true"

	// ...
	numMasters, err := retrieveNumNodesFromPlanInfo(planInfo, DefaultNumNodes)
	if err != nil {
		logger.Infoln("retrieveNumNodesFromPlanInfo error: ", err.Error())
	}

	containerMemory, err := retrieveNodeMemoryFromPlanInfo(planInfo, 500) // Mi
	if err != nil {
		logger.Infoln("retrieveNodeMemoryFromPlanInfo error: ", err.Error())
	}

	numReplicas, err := retrieveNumReplicasFromPlanInfo(planInfo, DefaultNumReplicas)
	if err != nil {
		logger.Infoln("retrieveNumReplicasFromPlanInfo error: ", err.Error())
	}

	logger.Infoln("new redis cluster parameters: numMasters=", numMasters, ", numReplicas=", numReplicas, ", containerMemory=", containerMemory, "Mi, enableAuth=", enableAuth)

	serviceSpec.IsAsync = true

	instanceIdInTempalte := strings.ToLower(oshandler.NewTenLengthID()) // for openshift 1.2
	serviceBrokerNamespace := oshandler.OC().Namespace()

	var redisPassword string
	if enableAuth {
		redisPassword = oshandler.GenGUID() // redis cluster doesn't support password officially
	}

	numNodePeers := numMasters * (numReplicas + 1)
	volumeBaseName := volumeBaseName(instanceIdInTempalte)
	volumes := make([]oshandler.Volume, numNodePeers)
	for i := range volumes {
		volumes[i] = oshandler.Volume{
			Volume_size: planInfo.Volume_size,
			Volume_name: volumeBaseName + "-" + strconv.Itoa(i),
		}
	}

	logger.Info("Redis Creating ...", map[string]interface{}{"instanceIdInTempalte": instanceIdInTempalte, "serviceBrokerNamespace": serviceBrokerNamespace})

	// ...

	serviceInfo.Url = instanceIdInTempalte
	serviceInfo.Database = serviceBrokerNamespace // may be not needed
	serviceInfo.Password = redisPassword

	serviceInfo.Volumes = volumes
	serviceInfo.Miscs = map[string]string{}
	serviceInfo.Miscs[oshandler.Nodes] = strconv.Itoa(numMasters)
	serviceInfo.Miscs[oshandler.Memory] = strconv.Itoa(containerMemory)
	serviceInfo.Miscs[oshandler.Replicas] = strconv.Itoa(numReplicas) // absent means 0

	// create dashboard
	
	_, stat, err := createRedisClusterResources_Stat(
		serviceInfo.Database,
		serviceInfo.Url,
		serviceInfo.Password,
		nil,
	)
	if err != nil {
		destroyRedisClusterResources_Stat(stat, serviceInfo.Database)
		return serviceSpec, oshandler.ServiceInfo{}, err
	}

	//>> may be not optimized
	var templates = make([]redisResources_Peer, numNodePeers)
	err = loadRedisClusterResources_Peers(
		serviceInfo.Url,
		serviceInfo.Password,
		serviceInfo.Miscs[oshandler.Memory],
		serviceInfo.Volumes,
		nil, // nonsense for the to-be-created nodeport service
		templates,
	)
	if err != nil {
		destroyRedisClusterResources_Stat(stat, serviceInfo.Database)
		return serviceSpec, oshandler.ServiceInfo{}, err
	}
	//<<

	peers := make([]*redisResources_Peer, len(templates))
	for i := range templates {
		peers[i] = &templates[i]
	}

	nodePorts, err := createRedisClusterResources_NodePorts(
		templates,
		serviceInfo.Database,
	)
	if err != nil {
		logger.Error("createRedisClusterResources_NodePorts error", err)
		destroyRedisClusterResources_Stat(stat, serviceInfo.Database)
		destroyRedisClusterResources_Peers(peers, serviceInfo.Database)
		return serviceSpec, oshandler.ServiceInfo{}, err
	}

	// All node port infos, including masters and slaves, are collected.
	// Clients must use the "CLUSTER NODES" command to check which nodes are masters.
	// https://redis.io/commands/cluster-nodes
	announceInfos := collectAnnounceInfos(nodePorts)

	asyncResultChan := serviceInfo.MakeAsyncResult()
	
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

			asyncResultChan <- err
			return
		}

		logger.Infoln("createRedisClusterResources_Peer ...")

		// create master res

		outputs, err := createRedisClusterResources_Peers(
			serviceInfo.Database,
			serviceInfo.Url,
			serviceInfo.Password,
			containerMemory, // serviceInfo.Miscs[oshandler.Memory],
			serviceInfo.Volumes,
			announceInfos,
		)
		if err != nil {
			logger.Error("redis createRedisClusterResources_Peer error", err)

			destroyRedisClusterResources_Stat(stat, serviceInfo.Database)
			//destroyRedisClusterResources_Peers(outputs, serviceInfo.Database)
			destroyRedisClusterResources_Peers(peers, serviceInfo.Database)
			oshandler.DeleteVolumns(serviceInfo.Database, volumes)
			
			asyncResultChan <- err

			return
		}

		err = waitAllRedisPodsAreReady(nodePorts, outputs)
		if err != nil {
			logger.Error("redis waitAllRedisPodsAreReady error", err)
			
			asyncResultChan <- err
			return
		}

		// run redis-trib.rb: create cluster
		serverIPs, err := initRedisMasterSlots(serviceInfo.Database, serviceInfo.Url, nodePorts, numMasters, numReplicas, serviceInfo.Password)
		if err != nil {
			logger.Error("redis initRedisMasterSlots error", err)
			
			asyncResultChan <- err
			return
		}
		
		_, _, err = createRedisClusterResources_Stat(
			serviceInfo.Database,
			serviceInfo.Url,
			serviceInfo.Password,
			serverIPs,
		)
		if err != nil {
			logger.Error("redis createRedisClusterResources_Stat (rc) error", err)
			//asyncResultChan <- err
			return
		}
		
		logger.Infoln("redis cluster", serviceInfo.Database, "created.")
		asyncResultChan <- nil
	}()

	// ...

	serviceSpec.DashboardURL = "http://" + stat.route.Spec.Host

	//>>>
	serviceSpec.Credentials = getCredentialsOnPrivision(&serviceInfo, announceInfos) //nodePort)
	//<<<

	return serviceSpec, serviceInfo, nil
}

func (handler *RedisCluster_Handler) DoLastOperation(myServiceInfo *oshandler.ServiceInfo) (brokerapi.LastOperation, error) {
	
	if myServiceInfo.ProvisionFailureInfo != "" {
		return brokerapi.LastOperation{
			State:       brokerapi.Failed,
			Description: myServiceInfo.ProvisionFailureInfo,
		}, nil
	}
	
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
		myServiceInfo.Password,
		myServiceInfo.Volumes,
	)

	if err != nil {
		return brokerapi.LastOperation{
			State:       brokerapi.Failed,
			Description: "In progress .",
		}, err
	}

	ok := func(dc *dcapi.DeploymentConfig) bool {
		podCount, err := statRunningPodsByLabels(myServiceInfo.Database, dc.Labels)
		if err != nil {
			logger.Infoln("statRunningPodsByLabels err:", err)
			return false
		}
		if dc == nil || dc.Name == "" || dc.Spec.Replicas == 0 || podCount < dc.Spec.Replicas {
			return false
		}
		// todo: why call it again?
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
	// planInfo.Volume_size // volume update is not supported now.

	namespace := myServiceInfo.Database
	instanceId := myServiceInfo.Url

	params := planInfo.MoreParameters
	enableAuthParam, _ := oshandler.ParseString(params[Key_EnableAuth])
	enableAuth := enableAuthParam == "1" || enableAuthParam == "yes" || enableAuthParam == "true"
	if myServiceInfo.Password == "" && enableAuth {
		return errors.New("auth must be enabled on creating")
	}

	logger.Infoln("[DoUpdate] redis cluster ...")
	go func() (finalError error) {
		defer func() {
			if finalError != nil {
				logger.Infoln("[DoUpdate] redis cluster done with error:", finalError.Error())
			}

			logger.Infoln("[DoUpdate] redis cluster. Updated exit.")
		}()

		// get old peer 0
		if len(myServiceInfo.Volumes) == 0 {
			return errors.New("[DoUpdate] old number of nodes is zero?!")
		}
		oldPeers, err := getRedisClusterResources_Peers(namespace, instanceId,
			myServiceInfo.Password, myServiceInfo.Volumes)

		if err != nil {
			return err
		}
		if myServiceInfo.Volumes[0].Volume_size != planInfo.Volume_size {
			return errors.New("volume size update is not supported now.")
		}
		newVolumeSize := myServiceInfo.Volumes[0].Volume_size // use volume size of old nodes for new nodes
		hostip := func(p *redisResources_Peer) string {
			args := p.dc.Spec.Template.Spec.Containers[0].Args
			for i := range args {
				if args[i] == "--cluster-announce-ip" {
					if i+1 < len(args) {
						return args[i+1]
					}
				}
			}
			return ""
		}(oldPeers[0])
		if hostip == "" {
			return errors.New("cluster-announce-ip is not found in old peer.")
		}

		// get new number of nodes (masters)
		oldNumMasters, err := oshandler.ParseInt64(myServiceInfo.Miscs[oshandler.Nodes])
		if err != nil {
			return err
		}
		newNumMasters, err := retrieveNumNodesFromPlanInfo(planInfo, int(oldNumMasters))
		if err != nil {
			return err
		}
		if newNumMasters < int(oldNumMasters) {
			return errors.New("number of masters can not decrease.")
		}

		// get new node memory
		nMemory, err := oshandler.ParseInt64(myServiceInfo.Miscs[oshandler.Memory])
		if err != nil {
			return err
		}
		oldNodeMemory := int(nMemory)
		newNodeMemory, err := retrieveNodeMemoryFromPlanInfo(planInfo, oldNodeMemory) // Mi
		if err != nil {
			return err
		}
		if newNodeMemory != oldNodeMemory {
			return errors.New("memory update is not supported now.")
		}

		// get new number of replicas (slaves)
		oldNumReplicas, err := oshandler.ParseInt64(myServiceInfo.Miscs[oshandler.Replicas])
		if err != nil {
			if _, ok := myServiceInfo.Miscs[oshandler.Replicas]; ok {
				return err
			} else {
				oldNumReplicas = 0
			}
		}
		newNumReplicas, err := retrieveNumReplicasFromPlanInfo(planInfo, int(oldNumReplicas))
		if err != nil {
			return err
		}
		if newNumReplicas < int(oldNumReplicas) {
			return errors.New("number of replicas can not decrease.")
		}

		// ...
		oldNumNodePeers := oldNumMasters * (oldNumReplicas + 1) // == len(myServiceInfo.Volumes)
		newNumNodePeers := newNumMasters * (newNumReplicas + 1)
		if newNumNodePeers <= int(oldNumNodePeers) {
			return errors.New("number of nodes can only be increased.")
		}

		volumeBaseName := volumeBaseName(instanceId)
		newVolumes := make([]oshandler.Volume, newNumNodePeers-int(oldNumNodePeers))
		for i := int(oldNumNodePeers); i < newNumNodePeers; i++ {
			newVolumes[i-int(oldNumNodePeers)] = oshandler.Volume{
				Volume_size: newVolumeSize,
				Volume_name: volumeBaseName + "-" + strconv.Itoa(i),
			}
		}

		logger.Infoln("[DoUpdate] new redis cluster parameters: newNumMasters=", newNumMasters, ", newNumReplicas=", newNumReplicas, ", newNodeMemory=", newNodeMemory)

		//===========================================================================

		succeeded := false

		// delete old trib pod

		deleteRedisTribPod(namespace, instanceId, int(oldNumMasters), int(oldNumReplicas))

		// create node ports

		var templates = make([]redisResources_Peer, newNumNodePeers-int(oldNumNodePeers))
		for i := range templates {
			err := loadRedisClusterResources_Peer(
				instanceId, strconv.Itoa(int(oldNumNodePeers)+i), myServiceInfo.Password,
				newNodeMemory,
				newVolumes[i].Volume_name,
				redisAnnounceInfo{}, // nonsense
				&templates[i],
			)
			if err != nil {
				return err
			}
		}

		defer func() {
			if !succeeded {
				peers := make([]*redisResources_Peer, len(templates))
				for i := range templates {
					peers[i] = &templates[i]
				}
				destroyRedisClusterResources_Peers(peers, namespace)
			}
		}()

		nodePorts, err := createRedisClusterResources_NodePorts(
			templates,
			namespace,
		)
		if err != nil {
			return err
		}

		logger.Infoln("[DoUpdate] redis cluster. NodePort svcs created done")

		// create new volumes

		defer func() {
			if !succeeded {
				oshandler.DeleteVolumns(namespace, newVolumes)
			}
		}()

		result := oshandler.StartCreatePvcVolumnJob(
			volumeBaseName,
			namespace,
			newVolumes,
		)
		err = <-result
		if err != nil {
			logger.Error("DoUpdate: redis cluster create volume", err)
			return err
		}

		// create dc

		var outputs = make([]*redisResources_Peer, len(newVolumes))

		defer func() {
			if !succeeded {
				destroyRedisClusterResources_Peers(outputs, namespace)
			}
		}()

		for i, p := range nodePorts {
			o, err := createRedisClusterResources_Peer(namespace,
				instanceId, strconv.Itoa(int(oldNumNodePeers)+i), myServiceInfo.Password,
				newNodeMemory,
				newVolumes[i].Volume_name,
				redisAnnounceInfo{
					IP:      hostip,
					Port:    strconv.Itoa(p.serviceNodePort.Spec.Ports[0].NodePort),
					BusPort: strconv.Itoa(p.serviceNodePort.Spec.Ports[1].NodePort),
				})
			if err != nil {
				// destroyRedisClusterResources_Peers(newPeers, namespace)
				return err
			}
			outputs[i] = o
		}

		logger.Infoln("[DoUpdate] redis cluster. new dcs are created.")

		err = waitAllRedisPodsAreReady(nodePorts, outputs)
		if err != nil {
			logger.Error("DoUpdate: redis waitAllRedisPodsAreReady error", err)
			return err
		}

		logger.Infoln("[DoUpdate] redis cluster. new pods are running.")

		// add new nodes to cluster and rebalance

		serverIPs, err := addRedisNewPeersAndRebalance(namespace, instanceId,
			nodePorts, oldPeers,
			int(oldNumMasters), newNumMasters, int(oldNumReplicas), newNumReplicas)
		if err != nil {
			logger.Error("DoUpdate: redis addRedisNewPeersAndRebalance error", err)
			return err
		}

		// save info (todo: improve the flow)

		myServiceInfo.Miscs[oshandler.Nodes] = strconv.Itoa(newNumMasters)
		myServiceInfo.Miscs[oshandler.Memory] = strconv.Itoa(newNodeMemory)
		myServiceInfo.Miscs[oshandler.Replicas] = strconv.Itoa(newNumReplicas)
		myServiceInfo.Volumes = append(myServiceInfo.Volumes, newVolumes...)

		err = callbackSaveNewInfo(myServiceInfo)
		if err != nil {
			logger.Error("redis cluster add nodes succeeded but save info error", err)
			// todo: how to rollback redis cluster config?
			return err
		}

		// update dashboard rc
		
		_, err = updateRedisClusterResources_Stat(
			namespace,
			instanceId,
			myServiceInfo.Password,
			serverIPs,
		)
		if err != nil {
			logger.Error("DoUpdate: redis updateRedisClusterResources_Stat error", err)
			// todo: how to rollback redis cluster config?
			// return // better to still succeed, maybe.
		}
		
		// ...
		
		logger.Infoln("[DoUpdate] redis cluster. updated info saved.")

		// ...
		succeeded = true

		return nil
	}()

	return nil
}

func (handler *RedisCluster_Handler) DoDeprovision(myServiceInfo *oshandler.ServiceInfo, asyncAllowed bool) (brokerapi.IsAsync, error) {
	go func() {
		// ...
		volumeJob := oshandler.GetCreatePvcVolumnJob(volumeBaseName(myServiceInfo.Url))
		if volumeJob != nil {
			volumeJob.Cancel()

			// wait job to exit
			for {
				logger.Infoln("wait CreatePvcVolumnJob done")
				time.Sleep(7 * time.Second)
				if nil == oshandler.GetCreatePvcVolumnJob(volumeBaseName(myServiceInfo.Url)) {
					break
				}
			}
		}
		
		stat, _ := getRedisClusterResources_Stat(
			myServiceInfo.Database,
			myServiceInfo.Url,
			myServiceInfo.Password,
			nil,
		)
		destroyRedisClusterResources_Stat(stat, myServiceInfo.Database)

		master_reses, _ := getRedisClusterResources_Peers(
			myServiceInfo.Database,
			myServiceInfo.Url,
			myServiceInfo.Password,
			myServiceInfo.Volumes,
		)
		destroyRedisClusterResources_Peers(master_reses, myServiceInfo.Database)

		// delete redis trib pods
		//>> ...
		go func() {
			for i := 1; i <= len(myServiceInfo.Volumes); i++ {
				for j := 0; j < len(myServiceInfo.Volumes); j++ {
					if i*(j+1) <= len(myServiceInfo.Volumes) {
						deleteRedisTribPod(myServiceInfo.Database, myServiceInfo.Url, i, j)
					}
				}

				kdel(myServiceInfo.Database, "pods", "redis-trib-"+myServiceInfo.Url+"-"+strconv.Itoa(i)) // for compatibility
			}
		}()
		//<<

		// ...

		logger.Infoln("to destroy volumes:", myServiceInfo.Volumes)

		oshandler.DeleteVolumns(myServiceInfo.Database, myServiceInfo.Volumes)
	}()

	return brokerapi.IsAsync(false), nil
}

// please note: the bsi may be still not fully initialized when calling the function.
func getCredentialsOnPrivision(myServiceInfo *oshandler.ServiceInfo, announces []redisAnnounceInfo) oshandler.Credentials {

	infos := make([]string, len(announces))
	for i, announce := range announces {
		infos[i] = fmt.Sprintf("%s:%s", announce.IP, announce.Port)
	}

	return oshandler.Credentials{
		Uri:      strings.Join(infos, ", "),
		Password: myServiceInfo.Password,
	}
}

func (handler *RedisCluster_Handler) DoBind(myServiceInfo *oshandler.ServiceInfo, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, oshandler.Credentials, error) {
	// todo: handle errors

	master_reses, err := getRedisClusterResources_Peers(
		myServiceInfo.Database,
		myServiceInfo.Url,
		myServiceInfo.Password,
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

func redisTribPodNameSuffix(numMasters, numReplias int) string {
	return fmt.Sprintf("-%d-%d", numMasters, numReplias)
}

func deleteRedisTribPod(serviceBrokerNamespace, instanceId string, numMasters, numReplias int) {
	kdel(serviceBrokerNamespace, "pods", "redis-trib-"+instanceId+redisTribPodNameSuffix(numMasters, numReplias))
}

var redisTribYamlTemplate = template.Must(template.ParseFiles("redis-cluster-trib.yaml"))

func runRedisTrib(serviceBrokerNamespace, instanceId, command string, args []string, customScript string, newNumMasters, newNumReplicas int) error {

	var params = map[string]interface{}{
		"InstanceID":    instanceId,
		"Image":         oshandler.RedisClusterImage(), // oshandler.RedisClusterTribImage(),
		"Command":       command,
		"Arguments":     args,
		"ScriptContent": customScript,
		"PodNameSuffix": redisTribPodNameSuffix(newNumMasters, newNumReplicas),
	}

	var buf bytes.Buffer
	err := redisTribYamlTemplate.Execute(&buf, params)
	if err != nil {
		return err
	}

	var pod kapi.Pod
	oshandler.NewYamlDecoder(buf.Bytes()).Decode(&pod)

	return kpost(serviceBrokerNamespace, "pods", &pod, nil)
}

func getPeerAddr(peer *redisResources_Peer) (string, string) {
	ip := peer.serviceNodePort.Spec.ClusterIP
	port := strconv.Itoa(peer.serviceNodePort.Spec.Ports[0].Port)
	// res.serviceNodePort.Name is not ok, but ip is ok. Don't know why.
	return ip + ":" + port, ip
}

func initRedisMasterSlots(serviceBrokerNamespace, instanceId string, peers []*redisResources_Peer, numMasters, numReplicas int, password string) ([]string, error) {
	cmd := "ruby"
	args := make([]string, 0, 100)
	if password != "" {
		args = append(args, "/usr/local/bin/redis-trib-2.rb")
		args = append(args, "create")
		args = append(args, "--password")
		args = append(args, password)
	} else {
		args = append(args, "/usr/local/bin/redis-trib.rb")
		args = append(args, "create")
	}
	if numReplicas > 0 {
		args = append(args, "--replicas")
		args = append(args, strconv.Itoa(numReplicas))
	}
	
	serverIPs := make([]string, 0, len(peers))
	for _, res := range peers {
		addr, ip := getPeerAddr(res)
		args = append(args, addr)
		serverIPs = append(serverIPs, ip)
	}
	
	err := runRedisTrib(serviceBrokerNamespace, instanceId, cmd, args, "", numMasters, numReplicas)
	return serverIPs, err
}

func addRedisNewPeersAndRebalance(serviceBrokerNamespace, instanceId string,
	newPeers []*redisResources_Peer, oldPeers []*redisResources_Peer,
	oldNumMasters, newNumMasters, oldNumReplicas, newNumReplicas int) ([]string, error) {

	numNewMasters := newNumMasters - oldNumMasters
	
	serverIPs := make([]string, 0, len(newPeers) + len(oldPeers))
	for _, oldPeer := range oldPeers {
		_ , ip := getPeerAddr(oldPeer)
		serverIPs = append(serverIPs, ip)
	}

	var oldPeerAddr, newPeerAddr string
	peers := oldPeers
	script := ""

	for n, newPeer := range newPeers {
		newPeerAddr, newPeerIp := getPeerAddr(newPeer)
		serverIPs = append(serverIPs, newPeerIp)
		for k := len(peers) - 1; k >= 0; k-- {
			peer := peers[k]
			oldPeerAddr, _ = getPeerAddr(peer)
			if n < numNewMasters {
				script += ">&2 echo ============== add new master: " + newPeerAddr + " for " + oldPeerAddr + " ==============\n\n"
				script += ">&2 ruby /usr/local/bin/redis-trib.rb add-node " + newPeerAddr + " " + oldPeerAddr + "\n\n"
			} else {
				script += ">&2 echo ============== add new replica: " + newPeerAddr + " for " + oldPeerAddr + " ==============\n\n"
				script += ">&2 ruby /usr/local/bin/redis-trib.rb add-node --slave " + newPeerAddr + " " + oldPeerAddr + "\n\n"
			}
		}
		peers = append(peers, newPeer)
	}

	if oldPeerAddr == "" {
		oldPeerAddr = newPeerAddr
	}
	script += ">&2 echo ============== sleep for awhile ... ==============\n\n"
	script += "sleep 3\n\n"
	script += ">&2 echo ============== rebalance started: " + oldPeerAddr + "... ==============\n\n"
	script += ">&2 ruby /usr/local/bin/redis-trib.rb rebalance --threshold 1 --use-empty-masters " + " " + oldPeerAddr + "\n\n"
	script += ">&2 echo ============== rebalance done. ==============\n\n"

	cmd := "/usr/local/bin/run-custom-script.sh"
	err := runRedisTrib(serviceBrokerNamespace, instanceId, cmd, nil, script, newNumMasters, newNumReplicas)
	return serverIPs, err
}

func waitAllRedisPodsAreReady(nodeports []*redisResources_Peer, dcs []*redisResources_Peer) error {
	time.Sleep(time.Second)
	for {
		logger.Infoln("===== check redis pod status ...")
		for i, res := range nodeports {
			svc := res.serviceNodePort
			osr := oshandler.NewOpenshiftREST(oshandler.OC())
			osr.KGet("/namespaces/"+svc.Namespace+"/services/"+svc.Name, nil)
			if osr.Err == oshandler.NotFound {
				return osr.Err
			}

			dc := dcs[i].dc
			n, _ := statRunningPodsByLabels(dc.Namespace, dc.Spec.Selector)
			if n < dc.Spec.Replicas {
				logger.Infoln(dc.Name, " is not ready")
				goto CheckAgain
			}

			logger.Infoln(dc.Name, " is ready")
			// todo: PING redis pod
		}
		break

	CheckAgain:
		time.Sleep(time.Second * 3)
	}
	time.Sleep(time.Second)
	return nil
}

//=======================================================================
//
//=======================================================================

var redisClusterYamlTemplate = template.Must(template.ParseFiles("redis-cluster-pvc.yaml"))

func loadRedisClusterResources_Peers(instanceID, redisPassword string, containerMemory string, volumes []oshandler.Volume,
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

	memory, err := strconv.Atoi(containerMemory)
	if err != nil {
		return err
	}

	for i := range res {
		err := loadRedisClusterResources_Peer(
			instanceID, strconv.Itoa(i), redisPassword,
			memory,
			volumes[i].Volume_name,
			announces[i],
			&res[i],
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func loadRedisClusterResources_Peer(instanceID, peerID, redisPassword string, containerMemory int, pvcName string,
	announce redisAnnounceInfo, res *redisResources_Peer) error {

	args := make([]string, 0, 100)
	args = append(args, "/usr/local/etc/redis.conf")
	args = append(args, "--cluster-announce-ip")
	args = append(args, announce.IP)
	args = append(args, "--cluster-announce-port")
	args = append(args, announce.Port)
	args = append(args, "--cluster-announce-bus-port")
	args = append(args, announce.BusPort)
	if redisPassword != "" {
		args = append(args, "--masterauth")
		args = append(args, redisPassword)
		args = append(args, "--requirepass")
		args = append(args, redisPassword)
	}

	var params = map[string]interface{}{
		"InstanceID":      instanceID,
		"NodeID":          peerID,
		"DataVolumePVC":   pvcName,
		"Arguments":       args,
		"RedisImage":      oshandler.RedisClusterImage(),
		"ContainerMemory": containerMemory, // "Mi"
	}

	var buf bytes.Buffer
	err := redisClusterYamlTemplate.Execute(&buf, params)
	if err != nil {
		return err
	}

	decoder := oshandler.NewYamlDecoder(buf.Bytes())
	decoder.
		Decode(&res.serviceNodePort).
		Decode(&res.dc)

	return decoder.Err
}

var redisStatYamlTemplate = template.Must(template.ParseFiles("redis-stat.yaml"))

func loadRedisClusterResources_Stat(instanceID, redisPassword string, serverIPs []string, res *redisResources_Stat) error {

	var params = map[string]interface{}{
		"InstanceID":     instanceID,
		"RedisPassword":  redisPassword,
		"ServerIPs":      serverIPs,
		"EndPointSuffix": oshandler.EndPointSuffix(),
		"Image":          oshandler.RedisStatImage(),
	}

	var buf bytes.Buffer
	err := redisStatYamlTemplate.Execute(&buf, params)
	if err != nil {
		return err
	}

	decoder := oshandler.NewYamlDecoder(buf.Bytes())
	decoder.
		Decode(&res.rc).
		Decode(&res.service).
		Decode(&res.route)

	return decoder.Err
}

type redisResources_Peer struct {
	serviceNodePort kapi.Service
	dc              dcapi.DeploymentConfig
}

type redisResources_Stat struct {
	rc      kapi.ReplicationController
	service kapi.Service
	route   routeapi.Route
}

func createRedisClusterResources_Peers(serviceBrokerNamespace string,
	instanceID, redisPassword string, memory int, volumes []oshandler.Volume,
	announces []redisAnnounceInfo) ([]*redisResources_Peer, error) {

	if len(announces) < len(volumes) {
		return nil, fmt.Errorf("createRedisClusterResources_Peers len(announces) < numberPeers: %d, %d", len(announces), len(volumes))
	}

	var outputs = make([]*redisResources_Peer, len(volumes))
	for i := range outputs {
		o, err := createRedisClusterResources_Peer(serviceBrokerNamespace,
			instanceID, strconv.Itoa(i), redisPassword,
			memory,
			volumes[i].Volume_name,
			announces[i])
		if err != nil {
			return nil, err
		}
		outputs[i] = o
	}
	return outputs, nil
}

func createRedisClusterResources_Peer(serviceBrokerNamespace string,
	instanceID, peerID, redisPassword string, memory int, pvcName string,
	announce redisAnnounceInfo) (*redisResources_Peer, error) {

	var input redisResources_Peer
	err := loadRedisClusterResources_Peer(instanceID, peerID, redisPassword, memory, pvcName,
		announce, &input)
	if err != nil {
		return nil, err
	}

	var output redisResources_Peer

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	// here, not use job.post
	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
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
	instanceID, redisPassword string, volumes []oshandler.Volume) ([]*redisResources_Peer, error) {

	var err error
	var outputs = make([]*redisResources_Peer, len(volumes))
	for i := range outputs {
		o, err2 := getRedisClusterResources_Peer(serviceBrokerNamespace,
			instanceID, strconv.Itoa(i), redisPassword, volumes[i].Volume_name)
		if err == nil {
			err = err2 // not perfect, only the first error is recorded.
		}
		outputs[i] = o
	}
	return outputs, err
}

func getRedisClusterResources_Peer(serviceBrokerNamespace string,
	instanceID, peerID, redisPassword, pvcName string) (*redisResources_Peer, error) {

	var output redisResources_Peer

	var input redisResources_Peer
	err := loadRedisClusterResources_Peer(
		instanceID, peerID, redisPassword,
		100,                 // Gi memory, the value is nonsense here.
		pvcName,             // the pvc name is nonsense here
		redisAnnounceInfo{}, //the value is nonsense
		&input)
	if err != nil {
		return &output, err
	}

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
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
	if masterRes == nil {
		return
	}
	//go func() { odel(serviceBrokerNamespace, "deploymentconfigs", masterRes.dc.Name) }()
	//go func() { kdel(serviceBrokerNamespace, "services", masterRes.serviceNodePort.Name) }()

	// Change to synced to avoid being deleted behind bolumes.
	odel(serviceBrokerNamespace, "deploymentconfigs", masterRes.dc.Name)
	kdel(serviceBrokerNamespace, "services", masterRes.serviceNodePort.Name)
}





func createRedisClusterResources_Stat(serviceBrokerNamespace, instanceID, redisPassword string, serverIPs []string) (*redisResources_Stat, *redisResources_Stat, error) {
	var input redisResources_Stat
	err := loadRedisClusterResources_Stat(instanceID, redisPassword, serverIPs, &input)
	if err != nil {
		return nil, nil, err
	}

	var output redisResources_Stat

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	prefix := "/namespaces/" + serviceBrokerNamespace
	
	if serverIPs == nil {
		osr.
			KPost(prefix+"/services", &input.service, &output.service).
			OPost(prefix+"/routes", &input.route, &output.route)
	} else {
		osr.KPost(prefix+"/replicationcontrollers", &input.rc, &output.rc)
	}

	if osr.Err != nil {
		logger.Error("createRedisClusterResources_Stat error", osr.Err)
	}

	return &output, &input, osr.Err
}

func updateRedisClusterResources_Stat(serviceBrokerNamespace, instanceID, redisPassword string, serverIPs []string) (*redisResources_Stat, error) {
	var input redisResources_Stat
	err := loadRedisClusterResources_Stat(instanceID, redisPassword, serverIPs, &input)
	if err != nil {
		return nil, err
	}
	
	rc := input.rc
	kdel_rc(serviceBrokerNamespace, &rc)
	
	//>> ensure to finish the deleting before the following re-creating.
	kdel(serviceBrokerNamespace, "replicationcontrollers", input.rc.Name)
	time.Sleep(time.Second * 5)
	//<<

	stat, _ := getRedisClusterResources_Stat(serviceBrokerNamespace, instanceID, redisPassword, serverIPs)
	if stat.service.Name == "" && stat.route.Name == "" {
		return nil, errors.New("reids cluster " + instanceID + " has already been deprovisioned.")
	}
	
	var output redisResources_Stat
	osr := oshandler.NewOpenshiftREST(oshandler.OC())
	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.KPost(prefix+"/replicationcontrollers", &input.rc, &output.rc)
	if osr.Err == nil {
		// n, _ := deleteCreatedPodsByLabels(serviceBrokerNamespace, output.rc.Labels)
		n, _ := deleteCreatedPodsByLabels(serviceBrokerNamespace, output.rc.Spec.Selector)
		logger.Infoln("updateRedisClusterResources_Stat:", n, "pods are deleted.")
	}

	return &output, osr.Err
}

func getRedisClusterResources_Stat(serviceBrokerNamespace, instanceID, redisPassword string, serverIPs []string) (*redisResources_Stat, error) {

	var output redisResources_Stat

	var input redisResources_Stat
	err := loadRedisClusterResources_Stat(instanceID, redisPassword, serverIPs, &input)
	if err != nil {
		return &output, err
	}

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		KGet(prefix+"/replicationcontrollers/"+input.rc.Name, &output.rc).
		KGet(prefix+"/services/"+input.service.Name, &output.service).
		OGet(prefix+"/routes/"+input.route.Name, &output.route)

	if osr.Err != nil {
		logger.Error("getRedisClusterResources_Stat", osr.Err)
	}

	return &output, osr.Err
}

func destroyRedisClusterResources_Stat(statRes *redisResources_Stat, serviceBrokerNamespace string) {
	// todo: add to retry queue on fail
	if statRes == nil {
		return
	}
	//go func() { kdel_rc(serviceBrokerNamespace, &statRes.rc) }()
	//go func() { kdel(serviceBrokerNamespace, "services", statRes.service.Name) }()
	//go func() { odel(serviceBrokerNamespace, "routes", statRes.route.Name) }()

	// Change to synced to avoid being deleted behind bolumes.
	kdel_rc(serviceBrokerNamespace, &statRes.rc)
	kdel(serviceBrokerNamespace, "services", statRes.service.Name)
	odel(serviceBrokerNamespace, "routes", statRes.route.Name)
}

//===============================================================
//
//===============================================================

func kpost(serviceBrokerNamespace, typeName string, body interface{}, into interface{}) error {
	logger.Infoln("to create ", typeName)

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

func kdel(serviceBrokerNamespace, typeName, resName string) error {
	if resName == "" {
		return nil
	}

	logger.Infoln("to delete ", typeName, "/", resName)

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

func kdel_rc(serviceBrokerNamespace string, rc *kapi.ReplicationController) {
	// looks pods will be auto deleted when rc is deleted.

	if rc == nil || rc.Name == "" {
		return
	}

	logger.Infoln("to delete pods on replicationcontroller", rc.Name)

	uri := "/namespaces/" + serviceBrokerNamespace + "/replicationcontrollers/" + rc.Name

	// modfiy rc replicas to 0

	zero := 0
	rc.Spec.Replicas = &zero
	osr := oshandler.NewOpenshiftREST(oshandler.OC()).KPut(uri, rc, nil)
	if osr.Err != nil {
		logger.Error("Modify Tensorflow rc", osr.Err)
		return
	}

	// start watching rc status

	statuses, cancel, err := oshandler.OC().KWatch(uri)
	if err != nil {
		logger.Error("Start Watching Tensorflow rc", err)
		return
	}

	go func() {
		for {
			status, _ := <-statuses

			if status.Err != nil {
				logger.Error("Watch Tensorflow rc error", status.Err)
				close(cancel)
				return
			} else {
				//logger.Debug("watch tensorflow HA rc, status.Info: " + string(status.Info))
			}

			var wrcs watchReplicationControllerStatus
			if err := json.Unmarshal(status.Info, &wrcs); err != nil {
				logger.Error("Parse Master Tensorflow rc status", err)
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

func odel(serviceBrokerNamespace, typeName, resName string) error {
	if resName == "" {
		return nil
	}

	logger.Infoln("to delete ", typeName, "/", resName)

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

func statRunningPodsByLabels(serviceBrokerNamespace string, labels map[string]string) (int, error) {

	logger.Infoln("to list pods in", serviceBrokerNamespace)

	uri := "/namespaces/" + serviceBrokerNamespace + "/pods"

	pods := kapi.PodList{}

	osr := oshandler.NewOpenshiftREST(oshandler.OC()).KList(uri, labels, &pods)
	if osr.Err != nil {
		return 0, osr.Err
	}

	nrunnings := 0

	for i := range pods.Items {
		pod := &pods.Items[i]

		logger.Infoln("\n pods.Items[", i, "].Status.Phase =", pod.Status.Phase, "\n")

		if pod.Status.Phase == kapi.PodRunning {
			nrunnings++
		}
	}

	return nrunnings, nil
}

func deleteCreatedPodsByLabels(serviceBrokerNamespace string, labels map[string]string) (int, error) {

	logger.Infoln("to delete created pods in", serviceBrokerNamespace)
	if len(labels) == 0 {
		return 0, errors.New("labels can't be blank in deleteCreatedPodsByLabels")
	}

	uri := "/namespaces/" + serviceBrokerNamespace + "/pods"

	pods := kapi.PodList{}

	osr := oshandler.NewOpenshiftREST(oshandler.OC()).KList(uri, labels, &pods)
	if osr.Err != nil {
		return 0, osr.Err
	}

	ndeleted := 0

	for i := range pods.Items {
		pod := &pods.Items[i]

		logger.Infoln("\n pods.Items[", i, "].Status.Phase =", pod.Status.Phase, "\n")

		if pod.Status.Phase != kapi.PodSucceeded {
			ndeleted++
			kdel(serviceBrokerNamespace, "pods", pod.Name)
		}
	}

	return ndeleted, nil
}
