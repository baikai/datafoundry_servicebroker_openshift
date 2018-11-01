package kafka_pvc

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	//"github.com/pivotal-golang/lager"
	logger "github.com/golang/glog"

	oshandler "github.com/asiainfoLDP/datafoundry_servicebroker_openshift/handler"
	dcapi "github.com/openshift/origin/deploy/api/v1"
	kapi "k8s.io/kubernetes/pkg/api/v1"
)

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

func peerPvcName3(volumes []oshandler.Volume) string {
	if len(volumes) > 3 {
		return volumes[3].Volume_name
	}
	return ""
}

func peerPvcName4(volumes []oshandler.Volume) string {
	if len(volumes) > 4 {
		return volumes[4].Volume_name
	}
	return ""
}

func watchZookeeperOrchestration(instanceId, serviceBrokerNamespace string, volumes []oshandler.Volume) (result <-chan bool, cancel chan<- struct{}, err error) {
	var input ZookeeperResources_Master
	err = loadZookeeperResources_Master(instanceId, volumes, &input)
	if err != nil {
		logger.Infoln("loadZookeeperResources_Master err:", err)
		return
	}

	var output ZookeeperResources_Master
	err = getZookeeperResources_Master(serviceBrokerNamespace, &input, &output)
	if err != nil {
		//close_all()
		return
	}

	dc1 := &output.dc1
	dc2 := &output.dc2
	dc3 := &output.dc3
	logger.Infoln("output.dc1:", dc1.Spec.Template.Labels)
	logger.Infoln("output.dc2:", dc2.Spec.Template.Labels)
	logger.Infoln("output.dc3:", dc3.Spec.Template.Labels)

	theresult := make(chan bool)
	result = theresult
	cancelled := make(chan struct{})
	cancel = cancelled

	go func() {
		ok := func(dc *dcapi.DeploymentConfig) bool {
			podCount, err := statRunningPodsByLabels(serviceBrokerNamespace, dc.Spec.Template.Labels)
			logger.Infoln("podCount:", podCount)
			if err != nil {
				logger.Infoln("statRunningPodsByLabels err:", err)
				return false
			}
			if dc == nil || dc.Name == "" || dc.Spec.Replicas == 0 || podCount < dc.Spec.Replicas {
				return false
			}
			n, _ := statRunningPodsByLabels(serviceBrokerNamespace, dc.Labels)
			return n >= dc.Spec.Replicas
		}

		for {
			if ok(dc1) && ok(dc2) && ok(dc3) {
				theresult <- true

				//close_all()
				return
			}

			//var status oshandler.WatchStatus
			var valid bool
			//var rc **kapi.ReplicationController
			select {
			case <-cancelled:
				valid = false
			//case status, valid = <- statuses1:
			//	//rc = &rc1
			//	break
			//case status, valid = <- statuses2:
			//	//rc = &rc2
			//	break
			//case status, valid = <- statuses3:
			//	//rc = &rc3
			//	break
			case <-time.After(15 * time.Second):
				// bug: pod phase change will not trigger rc status change.
				// so need this case
				continue
			}

			/*
				if valid {
					if status.Err != nil {
						valid = false
						logger.Error("watch master rcs error", status.Err)
					} else {
						var wrcs watchReplicationControllerStatus
						if err := json.Unmarshal(status.Info, &wrcs); err != nil {
							valid = false
							logger.Error("parse master rc status", err)
						//} else {
						//	*rc = &wrcs.Object
						}
					}
				}

				logger.Infoln("> WatchZookeeperOrchestration valid:", valid)
			*/

			if !valid {
				theresult <- false

				//close_all()
				return
			}
		}
	}()

	return
}

//=======================================================================
// the zookeeper functions may be called by outer packages
//=======================================================================

var ZookeeperTemplateData_Master []byte = nil

func loadZookeeperResources_Master(instanceID string, volumes []oshandler.Volume, res *ZookeeperResources_Master) error {

	if ZookeeperTemplateData_Master == nil {

		f, err := os.Open("zk-before-kafka-pvc.yaml")
		if err != nil {
			return err
		}
		ZookeeperTemplateData_Master, err = ioutil.ReadAll(f)
		if err != nil {
			return err
		}

		zookeeper_image := oshandler.KafkaVolumeImage()
		zookeeper_image = strings.TrimSpace(zookeeper_image)
		if len(zookeeper_image) > 0 {
			ZookeeperTemplateData_Master = bytes.Replace(
				ZookeeperTemplateData_Master,
				[]byte("http://kafka-image-place-holder/kafka-openshift-orchestration"),
				[]byte(zookeeper_image),
				-1)
		}
	}

	// ...

	// ...

	peerPvcName0 := peerPvcName0(volumes)
	peerPvcName1 := peerPvcName1(volumes)
	peerPvcName2 := peerPvcName2(volumes)

	// invalid operation sha1.Sum(([]byte)(zookeeperPassword))[:] (slice of unaddressable value)
	//sum := (sha1.Sum([]byte(zookeeperPassword)))[:]
	//zoo_password := zookeeperUser + ":" + base64.StdEncoding.EncodeToString (sum)

	yamlTemplates := ZookeeperTemplateData_Master

	yamlTemplates = bytes.Replace(yamlTemplates, []byte("instanceid"), []byte(instanceID), -1)

	yamlTemplates = bytes.Replace(yamlTemplates, []byte("zk-pvc-name-replace1"), []byte(peerPvcName0), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("zk-pvc-name-replace2"), []byte(peerPvcName1), -1)
	yamlTemplates = bytes.Replace(yamlTemplates, []byte("zk-pvc-name-replace3"), []byte(peerPvcName2), -1)

	//logger.Infoln("========= Boot yamlTemplates ===========")
	//logger.Infoln(string(yamlTemplates))
	//logger.Infoln()

	decoder := oshandler.NewYamlDecoder(yamlTemplates)
	decoder.
		Decode(&res.dc1).
		Decode(&res.dc2).
		Decode(&res.dc3).
		Decode(&res.svc1).
		Decode(&res.svc2).
		Decode(&res.svc3).
		Decode(&res.svc4) //.
		//Decode(&res.serviceNodePort)

	return decoder.Err
}

type ZookeeperResources_Master struct {
	dc1 dcapi.DeploymentConfig
	dc2 dcapi.DeploymentConfig
	dc3 dcapi.DeploymentConfig

	svc1 kapi.Service
	svc2 kapi.Service
	svc3 kapi.Service
	svc4 kapi.Service

	//serviceNodePort kapi.Service
}

func (masterRes *ZookeeperResources_Master) ServiceHostPort(serviceBrokerNamespace string) (string, string, error) {

	client_port := oshandler.GetServicePortByName(&masterRes.svc4, "2181-tcp")
	if client_port == nil {
		return "", "", errors.New("client port not found")
	}

	host := fmt.Sprintf("%s.%s.%s", masterRes.svc4.Name, serviceBrokerNamespace, oshandler.ServiceDomainSuffix(false))
	port := strconv.Itoa(client_port.Port)

	return host, port, nil
}

func createZookeeperResources_Master(instanceId, serviceBrokerNamespace string, volumes []oshandler.Volume) (*ZookeeperResources_Master, error) {
	var input ZookeeperResources_Master
	err := loadZookeeperResources_Master(instanceId, volumes, &input)
	if err != nil {
		return nil, err
	}

	var output ZookeeperResources_Master

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	// here, not use job.post
	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		OPost(prefix+"/deploymentconfigs", &input.dc1, &output.dc1).
		OPost(prefix+"/deploymentconfigs", &input.dc2, &output.dc2).
		OPost(prefix+"/deploymentconfigs", &input.dc3, &output.dc3).
		KPost(prefix+"/services", &input.svc1, &output.svc1).
		KPost(prefix+"/services", &input.svc2, &output.svc2).
		KPost(prefix+"/services", &input.svc3, &output.svc3).
		KPost(prefix+"/services", &input.svc4, &output.svc4)

	if osr.Err != nil {
		logger.Error("createZookeeperResources_Master", osr.Err)
	}

	return &output, osr.Err
}

/*
func createZookeeperResources_NodePort(input *ZookeeperResources_Master, serviceBrokerNamespace string) (*ZookeeperResources_Master, error) {
	var output ZookeeperResources_Master

	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	// here, not use job.post
	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.KPost(prefix+"/services", &input.serviceNodePort, &output.serviceNodePort)

	if osr.Err != nil {
		logger.Error("createZookeeperResources_NodePort", osr.Err)
	}

	return &output, osr.Err
}
*/

func GetZookeeperResources_Master(instanceId, serviceBrokerNamespace string, volumes []oshandler.Volume) (*ZookeeperResources_Master, error) {
	var output ZookeeperResources_Master

	var input ZookeeperResources_Master
	err := loadZookeeperResources_Master(instanceId, volumes, &input)
	if err != nil {
		return &output, err
	}

	err = getZookeeperResources_Master(serviceBrokerNamespace, &input, &output)
	return &output, err
}

func getZookeeperResources_Master(serviceBrokerNamespace string, input, output *ZookeeperResources_Master) error {
	osr := oshandler.NewOpenshiftREST(oshandler.OC())

	prefix := "/namespaces/" + serviceBrokerNamespace
	osr.
		OGet(prefix+"/deploymentconfigs/"+input.dc1.Name, &output.dc1).
		OGet(prefix+"/deploymentconfigs/"+input.dc2.Name, &output.dc2).
		OGet(prefix+"/deploymentconfigs/"+input.dc3.Name, &output.dc3).
		KGet(prefix+"/services/"+input.svc1.Name, &output.svc1).
		KGet(prefix+"/services/"+input.svc2.Name, &output.svc2).
		KGet(prefix+"/services/"+input.svc3.Name, &output.svc3).
		KGet(prefix+"/services/"+input.svc4.Name, &output.svc4) //.
		//KGet(prefix+"/services/"+input.serviceNodePort.Name, &output.serviceNodePort)

	if osr.Err != nil {
		logger.Error("getZookeeperResources_Master", osr.Err)
	}

	return osr.Err
}

func destroyZookeeperResources_Master(masterRes *ZookeeperResources_Master, serviceBrokerNamespace string) {
	// todo: add to retry queue on fail

	// All are changed to synced to avoid deleting them behind volumes.

	//go func() { odel(serviceBrokerNamespace, "deploymentconfigs", masterRes.dc1.Name) }()
	//go func() { odel(serviceBrokerNamespace, "deploymentconfigs", masterRes.dc2.Name) }()
	//go func() { odel(serviceBrokerNamespace, "deploymentconfigs", masterRes.dc3.Name) }()
	//go func() { kdel(serviceBrokerNamespace, "services", masterRes.svc1.Name) }()
	//go func() { kdel(serviceBrokerNamespace, "services", masterRes.svc2.Name) }()
	//go func() { kdel(serviceBrokerNamespace, "services", masterRes.svc3.Name) }()
	//go func() { kdel(serviceBrokerNamespace, "services", masterRes.svc4.Name) }()
	////go func() { kdel(serviceBrokerNamespace, "services", masterRes.serviceNodePort.Name) }()

	odel(serviceBrokerNamespace, "deploymentconfigs", masterRes.dc1.Name)
	odel(serviceBrokerNamespace, "deploymentconfigs", masterRes.dc2.Name)
	odel(serviceBrokerNamespace, "deploymentconfigs", masterRes.dc3.Name)
	kdel(serviceBrokerNamespace, "services", masterRes.svc1.Name)
	kdel(serviceBrokerNamespace, "services", masterRes.svc2.Name)
	kdel(serviceBrokerNamespace, "services", masterRes.svc3.Name)
	kdel(serviceBrokerNamespace, "services", masterRes.svc4.Name)
	//kdel(serviceBrokerNamespace, "services", masterRes.serviceNodePort.Name)

	logger.Infoln("zookeeper dc1 lables:", masterRes.dc1.Labels)
	rcs, _ := statRunningRCByLabels(serviceBrokerNamespace, masterRes.dc1.Labels)
	for _, rc := range rcs {
		//go func() { kdel_rc(serviceBrokerNamespace, &rc) }()
		kdel_rc(serviceBrokerNamespace, &rc)
	}

	logger.Infoln("zookeeper dc1 lables:", masterRes.dc2.Labels)
	rcs, _ = statRunningRCByLabels(serviceBrokerNamespace, masterRes.dc2.Labels)
	for _, rc := range rcs {
		//go func() { kdel_rc(serviceBrokerNamespace, &rc) }()
		kdel_rc(serviceBrokerNamespace, &rc)
	}

	logger.Infoln("zookeeper dc1 lables:", masterRes.dc3.Labels)
	rcs, _ = statRunningRCByLabels(serviceBrokerNamespace, masterRes.dc3.Labels)
	for _, rc := range rcs {
		//go func() { kdel_rc(serviceBrokerNamespace, &rc) }()
		kdel_rc(serviceBrokerNamespace, &rc)
	}
}
