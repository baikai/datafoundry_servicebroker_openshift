package handler

import (
	//"errors"
	//"encoding/json"
	"time"
	//"fmt"
	"bytes"
	"crypto/tls"
	"net/http"
	//"github.com/pivotal-cf/brokerapi"
	//kapi "k8s.io/kubernetes/pkg/api/v1"
)


var httpClientA = &http.Client{
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	},
	Timeout:   0,
}
var httpClientB = &http.Client{
	Transport: httpClientA.Transport,
	Timeout:   DfRequestTimeout,
}
var httpClientC = &http.Client{
	Transport: httpClientA.Transport,
	Timeout:   GeneralRequestTimeout,
}

func request(timeout time.Duration, method, url, bearerToken string, body []byte) (*http.Response, error) {
	var req *http.Request
	var err error
	if len(body) == 0 {
		req, err = http.NewRequest(method, url, nil)
	} else {
		req, err = http.NewRequest(method, url, bytes.NewReader(body))
	}

	if err != nil {
		return nil, err
	}

	//for k, v := range headers {
	//	req.Header.Add(k, v)
	//}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+bearerToken)

	//transCfg := &http.Transport{
	//	TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	//}
	//client := &http.Client{
	//	Transport: transCfg,
	//	Timeout:   timeout,
	//}

	switch timeout {
	case GeneralRequestTimeout:
		return httpClientC.Do(req)
	case DfRequestTimeout:
		return httpClientB.Do(req)
	case 0:
		return httpClientA.Do(req)
	default:
		c := &http.Client{
			Transport: httpClientA.Transport,
			Timeout:   timeout,
		}
		return c.Do(req)
	}
}

//===================================================

// now this function is moved to each service broker folder
//func InstancePvcName(instanceId string) string {
//	return "v" + instanceId // DON'T CHANGE
//}

//===================================================

/*

type watchPodStatus struct {
	// The type of watch update contained in the message
	Type string `json:"type"`
	// Pod details
	Object kapi.Pod `json:"object"`
}

func WaitUntilPodIsRunning(pod *kapi.Pod, stopWatching <-chan struct{}) error {
	select {
	case <- stopWatching:
		return errors.New("cancelled by calleer")
	default:
	}

	uri := "/namespaces/" + pod.Namespace + "/pods/" + pod.Name
	statuses, cancel, err := OC().KWatch (uri)
	if err != nil {
		return err
	}
	defer close(cancel)

	getPodChan := make(chan *kapi.Pod, 1)
	go func() {
		// the pod may be already running initially.
		// so simulate this get request result as a new watch event.

		interval := 2 * time.Second
		for {
			select {
			case <- stopWatching:
				return
			case <- time.After(interval):
				interval = 15 * time.Second
			}

			pod := &kapi.Pod{}
			osr := NewOpenshiftREST(OC()).KGet(uri, pod)
			if osr.Err == nil {
				getPodChan <- pod
			}
		}
	}()

	for {
		var pod *kapi.Pod
		select {
		case <- stopWatching:
			return errors.New("cancelled by calleer")
		case pod = <- getPodChan:
		case status, _ := <- statuses:
			if status.Err != nil {
				return status.Err
			}
			//println("watch etcd pod, status.Info: " + string(status.Info))

			var wps watchPodStatus
			if err := json.Unmarshal(status.Info, &wps); err != nil {
				return err
			}

			pod = &wps.Object
		}

		if pod.Status.Phase != kapi.PodPending {
			//println("watch pod phase: ", pod.Status.Phase)

			if pod.Status.Phase != kapi.PodRunning {
				return errors.New("pod phase is neither pending nor running: " + string(pod.Status.Phase))
			}

			break
		}
	}

	return nil
}

func WaitUntilPodIsReachable(pod *kapi.Pod, stopChecking <-chan struct{}, reachableFunc func(pod *kapi.Pod) bool, checkingInterval time.Duration) error {
	for {
		select {
		case <- time.After(checkingInterval):
			break
		case <- stopChecking:
			return errors.New("cancelled by calleer")
		}

		reached := reachableFunc(pod)
		if reached {
			break
		}
	}

	return nil
}

func WaitUntilPodsAreReachable(pods []*kapi.Pod, stopChecking <-chan struct{}, reachableFunc func(pod *kapi.Pod) bool, checkingInterval time.Duration) error {
	startIndex := 0
	num := len(pods)
	for {
		select {
		case <- time.After(checkingInterval):
			break
		case <- stopChecking:
			return errors.New("cancelled by calleer")
		}

		reached := true
		i := 0
		for ; i < num; i++ {
			pod := pods[(i + startIndex) % num]
			if ! reachableFunc(pod) {
				reached = false
				break
			}
		}

		if reached {
			break
		} else {
			startIndex = i
		}
	}

	return nil
}

type watchReplicationControllerStatus struct {
	// The type of watch update contained in the message
	Type string `json:"type"`
	// RC details
	Object kapi.ReplicationController `json:"object"`
}

func QueryPodsByLabels(serviceBrokerNamespace string, labels map[string]string) ([]*kapi.Pod, error) {

	//println("to list pods in", serviceBrokerNamespace)

	uri := "/namespaces/" + serviceBrokerNamespace + "/pods"

	pods := kapi.PodList{}

	osr := NewOpenshiftREST(OC()).KList(uri, labels, &pods)
	if osr.Err != nil {
		return nil, osr.Err
	}

	returnedPods := make([]*kapi.Pod, len(pods.Items))
	for i := range pods.Items {
		returnedPods[i] = &pods.Items[i]
	}

	return returnedPods, osr.Err
}

func QueryRunningPodsByLabels(serviceBrokerNamespace string, labels map[string]string) ([]*kapi.Pod, error) {

	pods, err := QueryPodsByLabels(serviceBrokerNamespace, labels)
	if err != nil {
		return pods, err
	}

	num := 0
	for i := range pods {
		pod := pods[i]

		//println("\n pods.Items[", i, "].Status.Phase =", pod.Status.Phase, "\n")

		if pod != nil && pod.Status.Phase == kapi.PodRunning {
			pods[num], pods[i] = pod, pods[num]
			num ++
		}
	}

	return pods[:num], nil
}

func GetReachablePodsByLabels(pods []*kapi.Pod, reachableFunc func(pod *kapi.Pod) bool) ([]*kapi.Pod, error) {
	num := 0
	for i := range pods {
		pod := pods[i]

		if pod != nil && pod.Status.Phase == kapi.PodRunning && reachableFunc(pod) {
			pods[num], pods[i] = pod, pods[num]
			num ++
		}
	}

	return pods[:num], nil
}

func DeleteReplicationController (serviceBrokerNamespace string, rc *kapi.ReplicationController) {
	// looks pods will be auto deleted when rc is deleted.

	if rc == nil || rc.Name == "" {
		return
	}

	defer KDelWithRetries(serviceBrokerNamespace, "replicationcontrollers", rc.Name)

	println("to delete pods on replicationcontroller", rc.Name)

	uri := "/namespaces/" + serviceBrokerNamespace + "/replicationcontrollers/" + rc.Name

	// modfiy rc replicas to 0

	zero := 0
	rc.Spec.Replicas = &zero
	osr := NewOpenshiftREST(OC()).KPut(uri, rc, nil)
	if osr.Err != nil {
		logger.Error("modify rc.Spec.Replicas => 0", osr.Err)
		return
	}

	// start watching rc status

	statuses, cancel, err := OC().KWatch (uri)
	if err != nil {
		logger.Error("start watching rc", err)
		return
	}
	defer close(cancel)

	for {
		status, _ := <- statuses

		if status.Err != nil {
			logger.Error("watch rc error", status.Err)
			return
		} else {
			//logger.Debug("watch tensorflow HA rc, status.Info: " + string(status.Info))
		}

		var wrcs watchReplicationControllerStatus
		if err := json.Unmarshal(status.Info, &wrcs); err != nil {
			logger.Error("parse master HA rc status", err)
			return
		}

		if wrcs.Object.Status.Replicas <= 0 {
			break
		}
	}
}

//===============================================================
// post and delete with retries
//===============================================================

func KPostWithRetries (serviceBrokerNamespace, typeName string, body interface{}, into interface{}) error {
	println("to create ", typeName)

	uri := fmt.Sprintf("/namespaces/%s/%s", serviceBrokerNamespace, typeName)
	i, n := 0, 5
RETRY:

	osr := NewOpenshiftREST(OC()).KPost(uri, body, into)
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

func OPostWithRetries (serviceBrokerNamespace, typeName string, body interface{}, into interface{}) error {
	println("to create ", typeName)

	uri := fmt.Sprintf("/namespaces/%s/%s", serviceBrokerNamespace, typeName)
	i, n := 0, 5
RETRY:

	osr := NewOpenshiftREST(OC()).OPost(uri, body, into)
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

func KDelWithRetries (serviceBrokerNamespace, typeName, resName string) error {
	if resName == "" {
		return nil
	}

	println("to delete ", typeName, "/", resName)

	uri := fmt.Sprintf("/namespaces/%s/%s/%s", serviceBrokerNamespace, typeName, resName)
	i, n := 0, 5
RETRY:
	osr := NewOpenshiftREST(OC()).KDelete(uri, nil)
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

func ODelWithRetries (serviceBrokerNamespace, typeName, resName string) error {
	if resName == "" {
		return nil
	}

	println("to delete ", typeName, "/", resName)

	uri := fmt.Sprintf("/namespaces/%s/%s/%s", serviceBrokerNamespace, typeName, resName)
	i, n := 0, 5
RETRY:
	osr := NewOpenshiftREST(OC()).ODelete(uri, nil)
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
*/
