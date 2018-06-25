package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	//"os"
	"strconv"
	"strings"
	"time"

	kapi "k8s.io/kubernetes/pkg/api/v1"
	//"fmt"
	"sync"
)

const DfRequestTimeout = time.Duration(8) * time.Second

func dfRequest(method, url, bearerToken string, bodyParams interface{}, into interface{}) (err error) {
	return dfRequestWithTimeout(DfRequestTimeout, method, url, bearerToken, bodyParams, into)
}

func dfRequestWithTimeout(timeout time.Duration, method, url, bearerToken string, bodyParams interface{}, into interface{}) (err error) {
	var body []byte
	if bodyParams != nil {
		body, err = json.Marshal(bodyParams)
		if err != nil {
			logger.Error("dfRequest(), failed to parse body", err)
			return err
		}
	}

	logger.Debug("dfRequest(), method=" + method + ",url=" + url + ", token=" + bearerToken)
	res, err := request(timeout, method, url, bearerToken, body)
	if err != nil {
		logger.Error("dfRequest(), request failed", err)
		return err
	}
	defer res.Body.Close()

	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		logger.Error("dfRequest(), read data from response failed", err)
		return err
	}

	//println("22222 len(data) = ", len(data), " , res.StatusCode = ", res.StatusCode)

	//logger.Debug("dfRequest(), status code=" + strconv.Itoa(res.StatusCode) + ", data=" + string(data))
	if res.StatusCode < 200 || res.StatusCode >= 400 {
		err = errors.New(string(data))
		logger.Error("dfRequest(), unknown status code "+strconv.Itoa(res.StatusCode), err)
		return err
	} else {
		if into != nil {
			//println("into data = ", string(data), "\n")

			err = json.Unmarshal(data, into)
			if err != nil {
				logger.Error("dfRequest(), parse response data failed", err)
			}
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

// CreateVolumn makes an API call to create a volume.
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

// DeleteVolumn makes an API call to delete a volume.
func DeleteVolumn(namespace, volumnName string) error {
	oc := OC()

	url := DfProxyApiPrefix() + "/namespaces/" + namespace + "/volumes/" + volumnName

	err := dfRequest("DELETE", url, oc.BearerToken(), nil, nil)

	return err
}

// ExpandVolumn makes an API call to expend a volume.
func ExpandVolumn(namespace, volumnName string, oldSize int, newSize int) error {
	oc := OC()

	url := DfProxyApiPrefix() + "/namespaces/" + namespace + "/volumes"

	options := &VolumnUpdateOptions{
		Name:    volumnName,
		OldSize: oldSize,
		NewSize: newSize,
	}

	logger.Debug("ExpandVolume(), send request to " + url)
	err := dfRequestWithTimeout(time.Minute*3, "PUT", url, oc.BearerToken(), options, nil)

	if err != nil {
		logger.Error("ExpandVolume(), failed", err)
	}

	return err
}

//=======================================================================
//
//=======================================================================

// DeleteVolumns starts some goroutines to delete some volumes concurrently.
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

// GetCreatePvcVolumnJob returns the ongoing volumn creating (or expending) job with the specified name.
func GetCreatePvcVolumnJob(jobName string) *CreatePvcVolumnJob {
	pvcVolumnCreatingJobsMutex.Lock()
	job := pvcVolumnCreatingJobs[jobName]
	pvcVolumnCreatingJobsMutex.Unlock()

	return job
}

// StartCreatePvcVolumnJob starts some goroutines which create some volumes concurrently.
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

// Cancel cancels a volumn creating job.
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

				errChan <- fmt.Errorf("WaitUntilPvcIsBound (%s, %s), error: %s", job.namespace, name, err)
				return
			}

			// ...

			println("CreateVolumn succeeded: name=", name, ", size=", size)

		}(vol.Volume_name, vol.Volume_size)
	}
	wg.Wait()
	close(errChan)

	println("CreateVolumn done. number errors: ", len(errChan))

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

// HasExpandPvcVolumnJob returns whether or not there is 
// a volume expending (or creating) job with the specified name ongoing.
func HasExpandPvcVolumnJob(jobName string) bool {
	return GetCreatePvcVolumnJob(jobName) != nil
}

// StartExpandPvcVolumnJob creates some goroutines which expend some volumes concurrently.
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

			logger.Debug("ExpandVolume, name=" + name + ", oldsize=" + strconv.Itoa(oldSize) +
				", newSize=" + strconv.Itoa(newSize))
			err := ExpandVolumn(job.namespace, name, oldSize, newSize)
			if err != nil {
				println("ExpandVolumn error:", err.Error())
				logger.Error("ExpandVolume, failed expand volume "+name, err)
				errChan <- err
				return
			}

			// ...

			logger.Debug("ExpandVolume succeed: volume " + name)
			println("ExpandVolumn succeeded: name=", name, ", oldSize=", oldSize, ", newSize =", newSize)

		}(vol.Volume_name, vol.Volume_size, job.newVolueSize)
	}
	wg.Wait()
	close(errChan)

	println("ExpandVolumn done. number erros: ", len(errChan))

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

// WaitUntilPvcIsBound watches the status of a pvc until it is bound or errors happen.
func WaitUntilPvcIsBound(namespace, pvcName string, stopWatching <-chan struct{}) error {
	select {
	case <-stopWatching:
		return errors.New("cancelled by calleer")
	default:
	}

	uri := "/namespaces/" + namespace + "/persistentvolumeclaims/" + pvcName
	statuses, cancel, err := OC().KWatch(uri)
	if err != nil {
		return errors.New("WaitUntilPvcIsBound KWatch error:" + err.Error())
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
				println("WaitUntilPvcIsBound KGet error: " + err.Error())
			}
		}
	}()

	// avoid waiting too long time
	timer := time.NewTimer(25 * time.Hour)
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
				return errors.New("WaitUntilPvcIsBound statuses error: " + status.Err.Error() + ", status.Info=" + string(status.Info))
			}

			var wps watchPvcStatus
			if err := json.Unmarshal(status.Info, &wps); err != nil {
				fmt.Println("WaitUntilPvcIsBound status.Info =", string(status.Info))
				return errors.New("WaitUntilPvcIsBound Unmarshal error: " + err.Error())
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
