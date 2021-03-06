package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	kapi "k8s.io/kubernetes/pkg/api/v1"
	//"fmt"
	"sync"
)

var dfProxyApiPrefix string

func DfProxyApiPrefix() string {
	if dfProxyApiPrefix == "" {
		addr := os.Getenv("DATAFOUNDRYPROXYADDR")
		if addr == "" {
			logger.Error("int dfProxyApiPrefix error:", errors.New("DATAFOUNDRYPROXYADDR env is not set"))
		}

		dfProxyApiPrefix = "http://" + addr + "/lapi/v1"
	}
	return dfProxyApiPrefix
}

const DfRequestTimeout = time.Duration(8) * time.Second

func dfRequest(method, url, bearerToken string, bodyParams interface{}, into interface{}) (err error) {
	var body []byte
	if bodyParams != nil {
		body, err = json.Marshal(bodyParams)
		if err != nil {
			return
		}
	}

	res, err := request(DfRequestTimeout, method, url, bearerToken, body)
	if err != nil {
		return
	}
	defer res.Body.Close()

	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return
	}

	//println("22222 len(data) = ", len(data), " , res.StatusCode = ", res.StatusCode)

	if res.StatusCode < 200 || res.StatusCode >= 400 {
		err = errors.New(string(data))
	} else {
		if into != nil {
			//println("into data = ", string(data), "\n")

			err = json.Unmarshal(data, into)
		}
	}

	return
}

type VolumnCreateOptions struct {
	Name            string `json:"name,omitempty"`
	Size            int    `json:"size,omitempty"`
	kapi.ObjectMeta `json:"metadata,omitempty"`
}

type VolumnUpdateOptions struct {
	Name    string `json:"name,omitempty"`
	OldSize int    `json:"old-size,omitempty"`
	NewSize int    `json:"new-size,omitempty"`
}

func CreateVolumn(namespace, volumnName string, size int) error {
	oc := OC()

	url := DfProxyApiPrefix() + "/namespaces/" + namespace + "/volumes"

	options := &VolumnCreateOptions{
		volumnName,
		size,
		kapi.ObjectMeta{
			Annotations: map[string]string{
				"dadafoundry.io/create-by": oc.username,
			},
		},
	}

	err := dfRequest("POST", url, oc.BearerToken(), options, nil)

	return err
}

func DeleteVolumn(namespace, volumnName string) error {
	oc := OC()

	url := DfProxyApiPrefix() + "/namespaces/" + namespace + "/volumes/" + volumnName

	err := dfRequest("DELETE", url, oc.BearerToken(), nil, nil)

	return err
}

// oldSize is not used here.
func ExpandVolumn(namespace, volumnName string, oldSize int, newSize int) error {
	oc := OC()

	url := DfProxyApiPrefix() + "/namespaces/" + namespace + "/volumes"

	options := &VolumnUpdateOptions{
		Name:    volumnName,
		OldSize: oldSize,
		NewSize: newSize,
	}

	err := dfRequest("PUT", url, oc.BearerToken(), options, nil)

	return err
}

//=======================================================================
//
//=======================================================================

// todo: need improving
func DeleteVolumns(namespace string, volumes []Volume) <-chan error {
	println("DeleteVolumns", volumes, "...")

	for _, vol := range volumes {
		go DeleteVolumn(namespace, vol.Volume_name)
	}

	return nil
}

//=======================================================================
//
//=======================================================================

// todo: it is best to save jobs in mysql firstly, ...
// now, when the server instance is terminated, jobs are lost.

// todo: Volumn -> volume

var pvcVolumnCreatingJobs = map[string]*CreatePvcVolumnJob{}
var pvcVolumnCreatingJobsMutex sync.Mutex

func GetCreatePvcVolumnJob(jobName string) *CreatePvcVolumnJob {
	pvcVolumnCreatingJobsMutex.Lock()
	job := pvcVolumnCreatingJobs[jobName]
	pvcVolumnCreatingJobsMutex.Unlock()

	return job
}

func StartCreatePvcVolumnJob(
	jobName string,
	namespace string,
	volumes []Volume,
) <-chan error {

	job := &CreatePvcVolumnJob{
		cancelChan: make(chan struct{}),

		namespace: namespace,
		volumes:   volumes,
	}

	c := make(chan error, 1)

	pvcVolumnCreatingJobsMutex.Lock()
	defer pvcVolumnCreatingJobsMutex.Unlock()

	if pvcVolumnCreatingJobs[jobName] == nil {
		pvcVolumnCreatingJobs[jobName] = job
		go func() {
			job.run(c)

			pvcVolumnCreatingJobsMutex.Lock()
			delete(pvcVolumnCreatingJobs, jobName)
			pvcVolumnCreatingJobsMutex.Unlock()
		}()
	} else {
		c <- errors.New("one volume creating job (" + jobName + ") is ongoing")
	}

	return c
}

type CreatePvcVolumnJob struct {
	cancelled   bool
	cancelChan  chan struct{}
	cancelMetex sync.Mutex

	namespace string
	volumes   []Volume
}

func (job *CreatePvcVolumnJob) Cancel() {
	job.cancelMetex.Lock()
	defer job.cancelMetex.Unlock()

	if !job.cancelled {
		job.cancelled = true
		close(job.cancelChan)
	}
}

func (job *CreatePvcVolumnJob) run(c chan<- error) {
	println("startCreatePvcVolumnJob ...")

	println("CreateVolumns", job.volumes, "...")

	errChan := make(chan error, len(job.volumes))

	var wg sync.WaitGroup
	wg.Add(len(job.volumes))
	for _, vol := range job.volumes {
		go func(name string, size int) {

			defer wg.Done()

			// ...

			println("CreateVolumn: name=", name, ", size=", size)

			err := CreateVolumn(job.namespace, name, size)
			if err != nil {
				println("CreateVolumn error:", err.Error())

				errChan <- err
				return
			}

			// ...

			println("WaitUntilPvcIsBound: name=", name)

			err = WaitUntilPvcIsBound(job.namespace, name, job.cancelChan)
			if err != nil {
				println("WaitUntilPvcIsBound", job.namespace, name, "error: ", err.Error())

				// caller will do the deletions.
				//println("DeleteVolumn", name, "...")/
				// todo: on error
				//DeleteVolumn(name)

				c <- fmt.Errorf("WaitUntilPvcIsBound (%s, %s), error: %s", job.namespace, name, err)
				return
			}

			// ...

			println("CreateVolumn succeeded: name=", name, ", size=", size)

		}(vol.Volume_name, vol.Volume_size)
	}
	wg.Wait()
	close(errChan)

	if len(errChan) == 0 {
		c <- nil
		return
	}

	errs := make([]string, 0, len(job.volumes))
	for err := range errChan {
		errs = append(errs, err.Error())
	}
	c <- errors.New(strings.Join(errs, "\n"))

	/*
		err := CreateVolumn(job.volumeName, job.volumeSize)
		if err != nil {
			println("CreateVolumn", job.volumeName, "esrror: ", err)
			c <- fmt.Errorf("CreateVolumn error: ", err)
			return
		}

		println("WaitUntilPvcIsBound", job.volumeName, "...")

		// watch pvc until bound

		err = WaitUntilPvcIsBound(namespace, job.volumeName, job.cancelChan)
		if err != nil {
			println("WaitUntilPvcIsBound", job.volumeName, "error: ", err)

			println("DeleteVolumn", job.volumeName, "...")

			// todo: on error
			DeleteVolumn(job.volumeName)

			c <- fmt.Errorf("WaitUntilPvcIsBound", job.volumeName, "error: ", err)

			return
		}
	*/

	c <- nil
}

//=======================================================================
//
//=======================================================================

// todo: it is best to save jobs in mysql firstly, ...
// now, when the server instance is terminated, jobs are lost.

// todo: Volumn -> volume

func StartExpandPvcVolumnJob(
	jobName string,
	namespace string,
	volumes []Volume,
	newVolumeSize int,
) <-chan error {

	job := &ExpandPvcVolumnJob{
		CreatePvcVolumnJob{
			cancelChan: make(chan struct{}),

			namespace: namespace,
			volumes:   volumes,
		},
		newVolumeSize,
	}

	c := make(chan error, 1)

	pvcVolumnCreatingJobsMutex.Lock()
	defer pvcVolumnCreatingJobsMutex.Unlock()

	if pvcVolumnCreatingJobs[jobName] == nil {
		pvcVolumnCreatingJobs[jobName] = &job.CreatePvcVolumnJob
		go func() {
			job.run(c)

			pvcVolumnCreatingJobsMutex.Lock()
			delete(pvcVolumnCreatingJobs, jobName)
			pvcVolumnCreatingJobsMutex.Unlock()
		}()
	} else {
		c <- errors.New("one volume creating/expanding job (" + jobName + ") is ongoing")
	}

	return c
}

type ExpandPvcVolumnJob struct {
	CreatePvcVolumnJob
	newVolueSize int
}

func (job *ExpandPvcVolumnJob) run(c chan<- error) {
	println("startExpandPvcVolumnJob ...")

	println("ExpandVolumns", job.volumes, "...")

	errChan := make(chan error, len(job.volumes))

	var wg sync.WaitGroup
	wg.Add(len(job.volumes))
	for _, vol := range job.volumes {
		go func(name string, oldSize, newSize int) {

			defer wg.Done()

			// ...

			println("ExpandVolumn: name=", name, ", oldSize=", oldSize, ", newSize =", newSize)

			err := ExpandVolumn(job.namespace, name, oldSize, newSize)
			if err != nil {
				println("ExpandVolumn error:", err.Error())

				errChan <- err
				return
			}

			// ...

			println("ExpandVolumn succeeded: name=", name, ", oldSize=", oldSize, ", newSize =", newSize)

		}(vol.Volume_name, vol.Volume_size, job.newVolueSize)
	}
	wg.Wait()
	close(errChan)

	if len(errChan) == 0 {
		c <- nil
		return
	}

	errs := make([]string, 0, len(job.volumes))
	for err := range errChan {
		errs = append(errs, err.Error())
	}
	c <- errors.New(strings.Join(errs, "\n"))

	/*
		err := ExpandVolumn(job.volumeName, job.volumeSize)
		if err != nil {
			println("ExpandVolumn", job.volumeName, "esrror: ", err)
			c <- fmt.Errorf("ExpandVolumn error: ", err)
			return
		}

		println("WaitUntilPvcIsBound", job.volumeName, "...")

		// watch pvc until bound

		err = WaitUntilPvcIsBound(namespace, job.volumeName, job.cancelChan)
		if err != nil {
			println("WaitUntilPvcIsBound", job.volumeName, "error: ", err)

			println("DeleteVolumn", job.volumeName, "...")

			// todo: on error
			DeleteVolumn(job.volumeName)

			c <- fmt.Errorf("WaitUntilPvcIsBound", job.volumeName, "error: ", err)

			return
		}
	*/

	c <- nil
}

//====================

//====================

type watchPvcStatus struct {
	// The type of watch update contained in the message
	Type string `json:"type"`
	// Pod details
	Object kapi.PersistentVolumeClaim `json:"object"`
}

func WaitUntilPvcIsBound(namespace, pvcName string, stopWatching <-chan struct{}) error {
	select {
	case <-stopWatching:
		return errors.New("cancelled by calleer")
	default:
	}

	uri := "/namespaces/" + namespace + "/persistentvolumeclaims/" + pvcName
	statuses, cancel, err := OC().KWatch(uri)
	if err != nil {
		return err
	}
	defer close(cancel)

	getPvcChan := make(chan *kapi.PersistentVolumeClaim, 1)
	go func() {
		// the pvc may be already bound initially.
		// so simulate this get request result as a new watch event.

		select {
		case <-stopWatching:
			return
		case <-time.After(3 * time.Second):
			pvc := &kapi.PersistentVolumeClaim{}
			osr := NewOpenshiftREST(OC()).KGet(uri, pvc)
			//fmt.Println("WaitUntilPvcIsBound, get pvc, osr.Err=", osr.Err)
			if osr.Err == nil {
				getPvcChan <- pvc
			} else {
				getPvcChan <- nil
			}
		}
	}()

	// avoid waiting too long time
	timer := time.NewTimer(1 * time.Hour)
	defer timer.Stop()

	for {
		var pvc *kapi.PersistentVolumeClaim
		select {
		case <-timer.C:
			return errors.New("create volume" + pvcName + "expired")
		case <-stopWatching:
			return errors.New("cancelled by calleer")
		case pvc = <-getPvcChan:
		case status, _ := <-statuses:
			if status.Err != nil {
				return status.Err
			}

			var wps watchPvcStatus
			if err := json.Unmarshal(status.Info, &wps); err != nil {
				return err
			}

			pvc = &wps.Object
		}

		if pvc == nil {
			// get return 404 from above goroutine
			return errors.New("pvc not found")
		}

		// assert pvc != nil

		//fmt.Println("WaitUntilPvcIsBound, pvc.Phase=", pvc.Status.Phase, ", pvc=", *pvc)

		if pvc.Status.Phase != kapi.ClaimPending {
			//println("watch pvc phase: ", pvc.Status.Phase)

			if pvc.Status.Phase != kapi.ClaimBound {
				return errors.New("pvc phase is neither pending nor bound: " + string(pvc.Status.Phase))
			}

			break
		}
	}

	return nil
}
