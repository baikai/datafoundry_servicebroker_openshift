package handler

import (
	"fmt"
	"errors"
	//marathon "github.com/gambol99/go-marathon"
	//"github.com/pivotal-cf/brokerapi"
	"bufio"
	"bytes"
	"strings"
	"time"
	"io"
	"io/ioutil"
	"os"
	"sync/atomic"
	//"crypto/tls"
	"encoding/json"
	"net/http"
	neturl "net/url"
	//"golang.org/x/build/kubernetes"
	//"golang.org/x/oauth2"

	kclient "k8s.io/kubernetes/pkg/client/unversioned"

	"github.com/openshift/origin/pkg/cmd/util/tokencmd"

	kapi "k8s.io/kubernetes/pkg/api/v1"
	//"github.com/ghodss/yaml"
	"github.com/pivotal-golang/lager"
)

var _ = fmt.Print

//==============================================================
//
//==============================================================

func init() {
	logger = lager.NewLogger("OpenShift")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))
}

var logger lager.Logger

//==============================================================
//
//==============================================================

type E string

func (e E) Error() string {
	return string(e)
}

const (
	NotFound = E("not found")
)

//==============================================================
//
//==============================================================

type OpenshiftClient struct {
	host string
	//authUrl string
	oapiUrl string
	kapiUrl string

	// url for v1beta1 api
	kapiV1B1Url string

	namespace string
	username  string
	password  string
	//bearerToken string
	bearerToken atomic.Value
}

func (oc *OpenshiftClient) Namespace() string {
	return oc.namespace
}

func (oc *OpenshiftClient) BearerToken() string {
	//return oc.bearerToken
	return oc.bearerToken.Load().(string)
}

func (oc *OpenshiftClient) setBearerToken(token string) {
	oc.bearerToken.Store(token)
}

func newOpenshiftClient(host, username, password, defaultNamespace string) *OpenshiftClient {
	switch {
	default: host = "https://" + host
	case strings.HasPrefix(host, "https://"):
	case strings.HasPrefix(host, "http://"):
	}
	
	oc := &OpenshiftClient{
		host: host,
		//authUrl: host + "/oauth/authorize?response_type=token&client_id=openshift-challenging-client",
		oapiUrl:     host + "/oapi/v1",
		kapiUrl:     host + "/api/v1",
		kapiV1B1Url: host + "/apis/apps/v1beta1",

		namespace: defaultNamespace,
		username:  username,
		password:  password,
	}
	oc.bearerToken.Store("")

	go oc.updateBearerToken()

	return oc
}

func (oc *OpenshiftClient) updateBearerToken() {
	if ServiceAccountToken() != "" {
		oc.setBearerToken(ServiceAccountToken())

		println("Use ServiceAccountToken(). Length=", len(ServiceAccountToken()))
		
		return
	}
	
	for {
		clientConfig := &kclient.Config{}
		clientConfig.Host = oc.host
		clientConfig.Insecure = true
		//clientConfig.Version =

		token, err := tokencmd.RequestToken(clientConfig, nil, oc.username, oc.password)
		if err != nil {
			println("RequestToken error: ", err.Error())

			time.Sleep(15 * time.Second)
		} else {
			//clientConfig.BearerToken = token
			//oc.bearerToken = token
			oc.setBearerToken(token)

			println("RequestToken token: ", token)

			time.Sleep(3 * time.Hour)
		}
	}
}

func (oc *OpenshiftClient) request(method string, url string, body []byte, timeout time.Duration) (*http.Response, error) {
	//token := oc.bearerToken
	token := oc.BearerToken()
	if token == "" {
		return nil, errors.New("token is blank")
	}

	return request(timeout, method, url, token, body)
}

type WatchStatus struct {
	Info []byte
	Err  error
}

func (oc *OpenshiftClient) doWatch(url string) (<-chan WatchStatus, chan<- struct{}, error) {
	//res, err := oc.request("GET", url, nil, 0)
	//if err != nil {
	//	return nil, nil, err
	//}
	////if res.Body == nil {
	////	return nil, nil, errors.New("response.body is nil")
	////}

	statuses := make(chan WatchStatus, 5)
	canceled := make(chan struct{}, 1)

	go func() {
		defer close(statuses)
		
		for range [100]struct{}{} { // most 99 retries on ErrUnexpectedEOF
			needRetry := func() bool {
				res, err := oc.request("GET", url, nil, 0)
				if err != nil {
					//return nil, nil, err
					println("doWatch, oc.request. error:", err.Error(), ", ", err == io.ErrUnexpectedEOF)
					
					if err == io.ErrUnexpectedEOF {
						return true
					}
					
					statuses <- WatchStatus{nil, err}
					return false
				}
				//if res.Body == nil {
				
				defer func() {
					res.Body.Close()
				}()

				reader := bufio.NewReader(res.Body)
				for {
					select {
					case <-canceled:
						return false
					default:
					}

					line, err := reader.ReadBytes('\n')
					if err != nil {
						println("doWatch, reader.ReadBytes. error:", err.Error(), ", ", err == io.ErrUnexpectedEOF)
						if err == io.ErrUnexpectedEOF {
							return true
						}
						
						statuses <- WatchStatus{line, err}
						return false
					}

					statuses <- WatchStatus{line, nil}
				}
			}()
			
			if needRetry {
				time.Sleep(time.Second * 5)
			} else {
				return
			}
		}
		
		statuses <- WatchStatus{nil, errors.New("too many tires")}
		
		return
	}()

	return statuses, canceled, nil
}

func (oc *OpenshiftClient) OWatch(uri string) (<-chan WatchStatus, chan<- struct{}, error) {
	return oc.doWatch(oc.oapiUrl + "/watch" + uri)
}

func (oc *OpenshiftClient) KWatch(uri string) (<-chan WatchStatus, chan<- struct{}, error) {
	return oc.doWatch(oc.kapiUrl + "/watch" + uri)
}

const GeneralRequestTimeout = time.Duration(30) * time.Second

/*
func (oc *OpenshiftClient) doRequest (method, url string, body []byte) ([]byte, error) {
	res, err := oc.request(method, url, body, GeneralRequestTimeout)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	return ioutil.ReadAll(res.Body)
}

func (oc *OpenshiftClient) ORequest (method, uri string, body []byte) ([]byte, error) {
	return oc.doRequest(method, oc.oapiUrl + uri, body)
}

func (oc *OpenshiftClient) KRequest (method, uri string, body []byte) ([]byte, error) {
	return oc.doRequest(method, oc.kapiUrl + uri, body)
}
*/

type OpenshiftREST struct {
	oc  *OpenshiftClient
	Err error
}

func NewOpenshiftREST(oc *OpenshiftClient) *OpenshiftREST {
	return &OpenshiftREST{oc: oc}
}

func (osr *OpenshiftREST) doRequest(returnIfAlreadyError bool, method, url string, bodyParams interface{}, into interface{}) *OpenshiftREST {
	if returnIfAlreadyError && osr.Err != nil {
		return osr
	}

	err := func() error {
		var body []byte
		if bodyParams != nil {
			var err error
			body, err = json.Marshal(bodyParams)
			if err != nil {
				return err
			}
		}

		//res, osr.Err := oc.request(method, url, body, GeneralRequestTimeout) // non-name error
		res, err := osr.oc.request(method, url, body, GeneralRequestTimeout)
		if err != nil {
			return err
		}
		defer res.Body.Close()

		data, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return err
		}

		//println("22222 len(data) = ", len(data), " , res.StatusCode = ", res.StatusCode)

		if res.StatusCode == 404 {
			return NotFound
		} else if res.StatusCode < 200 || res.StatusCode >= 400 {
			return errors.New(string(data))
		} else if into != nil {
			//println("into data = ", string(data), "\n")

			return json.Unmarshal(data, into)
		}

		return nil
	}()

	if osr.Err == nil {
		osr.Err = err
	}

	return osr
}

func buildUriWithSelector(uri string, selector map[string]string) string {
	var buf bytes.Buffer
	for k, v := range selector {
		if buf.Len() > 0 {
			buf.WriteByte(',')
		}
		buf.WriteString(k)
		buf.WriteByte('=')
		buf.WriteString(v)
	}

	if buf.Len() == 0 {
		return uri
	}

	values := neturl.Values{}
	values.Set("labelSelector", buf.String())

	if strings.IndexByte(uri, '?') < 0 {
		uri = uri + "?"
	}

	println("\n uri=", uri+values.Encode(), "\n")

	return uri + values.Encode()
}

// o

func (osr *OpenshiftREST) OList(uri string, selector map[string]string, into interface{}) *OpenshiftREST {

	return osr.doRequest(false, "GET", osr.oc.oapiUrl+buildUriWithSelector(uri, selector), nil, into)
}

func (osr *OpenshiftREST) OGet(uri string, into interface{}) *OpenshiftREST {
	return osr.doRequest(false, "GET", osr.oc.oapiUrl+uri, nil, into)
}

func (osr *OpenshiftREST) ODelete(uri string, into interface{}) *OpenshiftREST {
	return osr.doRequest(false, "DELETE", osr.oc.oapiUrl+uri, &kapi.DeleteOptions{}, into)
}

func (osr *OpenshiftREST) OPost(uri string, body interface{}, into interface{}) *OpenshiftREST {
	return osr.doRequest(true, "POST", osr.oc.oapiUrl+uri, body, into)
}

func (osr *OpenshiftREST) OPut(uri string, body interface{}, into interface{}) *OpenshiftREST {
	return osr.doRequest(true, "PUT", osr.oc.oapiUrl+uri, body, into)
}

// k

func (osr *OpenshiftREST) KList(uri string, selector map[string]string, into interface{}) *OpenshiftREST {
	return osr.doRequest(false, "GET", osr.oc.kapiUrl+buildUriWithSelector(uri, selector), nil, into)
}

func (osr *OpenshiftREST) KGet(uri string, into interface{}) *OpenshiftREST {
	return osr.doRequest(false, "GET", osr.oc.kapiUrl+uri, nil, into)
}

func (osr *OpenshiftREST) KDelete(uri string, into interface{}) *OpenshiftREST {
	return osr.doRequest(false, "DELETE", osr.oc.kapiUrl+uri, &kapi.DeleteOptions{}, into)
}

func (osr *OpenshiftREST) KDeleteByLabels(uri string, selector map[string]string, into interface{}) *OpenshiftREST {
	return osr.doRequest(false, "DELETE", osr.oc.kapiUrl+buildUriWithSelector(uri, selector), &kapi.DeleteOptions{}, into)
}

func (osr *OpenshiftREST) KPost(uri string, body interface{}, into interface{}) *OpenshiftREST {
	return osr.doRequest(true, "POST", osr.oc.kapiUrl+uri, body, into)
}

func (osr *OpenshiftREST) KPut(uri string, body interface{}, into interface{}) *OpenshiftREST {
	return osr.doRequest(true, "PUT", osr.oc.kapiUrl+uri, body, into)
}

// Kv1b1Get --- api for retrieving information according to v1beta1 spec
func (osr *OpenshiftREST) Kv1b1Get(uri string, into interface{}) *OpenshiftREST {
	return osr.doRequest(false, "GET", osr.oc.kapiV1B1Url+uri, nil, into)
}

// Kv1b1Delete --- api for delete objects according to v1beta1 spec
func (osr *OpenshiftREST) Kv1b1Delete(uri string, into interface{}) *OpenshiftREST {
	return osr.doRequest(false, "DELETE", osr.oc.kapiV1B1Url+uri, &kapi.DeleteOptions{}, into)
}

// Kv1b1Post --- api for create objects according to v1beta1 spec
func (osr *OpenshiftREST) Kv1b1Post(uri string, body interface{}, into interface{}) *OpenshiftREST {
	return osr.doRequest(true, "POST", osr.oc.kapiV1B1Url+uri, body, into)
}

// Kv1b1Put --- api for scale objects according to v1beta1 spec
func (osr *OpenshiftREST) Kv1b1Put(uri string, body interface{}, into interface{}) *OpenshiftREST {
	return osr.doRequest(true, "PUT", osr.oc.kapiV1B1Url+uri, body, into)
}

// Kv1b1Watch --- api for watching objects defined in v1beta1
func (oc *OpenshiftClient) Kv1b1Watch(uri string) (<-chan WatchStatus, chan<- struct{}, error) {
	return oc.doWatch(oc.kapiV1B1Url + "/watch" + uri)
}


// custom api group

func (osr *OpenshiftREST) List(uri string, selector map[string]string, into interface{}, apiGroup string) *OpenshiftREST {
	return osr.doRequest(false, "GET", osr.oc.host + apiGroup + buildUriWithSelector(uri, selector), nil, into)
}

func (osr *OpenshiftREST) Get(uri string, into interface{}, apiGroup string) *OpenshiftREST {
	return osr.doRequest(false, "GET", osr.oc.host + apiGroup + uri, nil, into)
}

func (osr *OpenshiftREST) Delete(uri string, into interface{}, apiGroup string, opt *kapi.DeleteOptions) *OpenshiftREST {
	//fmt.Println(">>>>>>> opt=", *opt)
	if opt == nil {
		opt = &kapi.DeleteOptions{}
	}
	return osr.doRequest(false, "DELETE", osr.oc.host + apiGroup + uri, opt, into)
}

func (osr *OpenshiftREST) Post(uri string, body interface{}, into interface{}, apiGroup string) *OpenshiftREST {
	return osr.doRequest(true, "POST", osr.oc.host + apiGroup + uri, body, into)
}

func (osr *OpenshiftREST) Put(uri string, body interface{}, into interface{}, apiGroup string) *OpenshiftREST {
	return osr.doRequest(true, "PUT", osr.oc.host + apiGroup + uri, body, into)
}

//===============================================================
//
//===============================================================

func GetServicePortByName(service *kapi.Service, name string) *kapi.ServicePort {
	if service != nil {
		for i := range service.Spec.Ports {
			port := &service.Spec.Ports[i]
			if port.Name == name {
				return port
			}
		}
	}

	return nil
}

func GetPodPortByName(pod *kapi.Pod, name string) *kapi.ContainerPort {
	if pod != nil {
		for i := range pod.Spec.Containers {
			c := &pod.Spec.Containers[i]
			for j := range c.Ports {
				port := &c.Ports[j]
				if port.Name == name {
					return port
				}
			}
		}
	}

	return nil
}

func GetReplicationControllersByLabels(serviceBrokerNamespace string, labels map[string]string) ([]kapi.ReplicationController, error) {

	println("to list pods in", serviceBrokerNamespace)

	uri := "/namespaces/" + serviceBrokerNamespace + "/pods"

	rcs := kapi.ReplicationControllerList{}

	osr := NewOpenshiftREST(OC()).KList(uri, labels, &rcs)
	if osr.Err != nil {
		return nil, osr.Err
	}

	return rcs.Items, osr.Err
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
