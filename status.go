package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/apimachinery/pkg/watch"
	apiv1 "k8s.io/api/core/v1"
	"github.com/cyverse-de/messaging"
	"github.com/pkg/errors"
)

// AnalysisStatusPublisher is the interface for types that need to publish a job
// update.
type AnalysisStatusPublisher interface {
	Fail(jobID, msg string) error
	Success(jobID, msg string) error
	Running(jobID, msg string) error
}

// JSLPublisher is a concrete implementation of AnalysisStatusPublisher that
// posts status updates to the job-status-listener service.
type JSLPublisher struct {
	statusURL string
}

// AnalysisStatus contains the data needed to post a status update to the
// notification-agent service.
type AnalysisStatus struct {
	Host    string
	State   messaging.JobState
	Message string
}

func (j *JSLPublisher) postStatus(jobID, msg string, jobState messaging.JobState) error {
	status := &AnalysisStatus{
		Host:    hostname(),
		State:   jobState,
		Message: msg,
	}

	u, err := url.Parse(j.statusURL)
	if err != nil {
		return errors.Wrapf(
			err,
			"error parsing URL %s for job %s before posting %s status",
			j,
			jobID,
			jobState,
		)
	}
	u.Path = path.Join(jobID, "status")

	js, err := json.Marshal(status)
	if err != nil {
		return errors.Wrapf(
			err,
			"error marshalling JSON for analysis %s before posting %s status",
			jobID,
			jobState,
		)

	}
	response, err := http.Post(u.String(), "application/json", bytes.NewReader(js))
	if err != nil {
		return errors.Wrapf(
			err,
			"error returned posting %s status for job %s to %s",
			jobState,
			jobID,
			u.String(),
		)
	}
	if response.StatusCode < 200 || response.StatusCode > 399 {
		return errors.Wrapf(
			err,
			"error status code %d returned after posting %s status for job %s to %s: %s",
			response.StatusCode,
			jobState,
			jobID,
			u.String(),
			response.Body,
		)
	}
	return nil
}

// Fail sends an analysis failure update with the provided message via the AMQP
// broker. Should be sent once.
func (j *JSLPublisher) Fail(jobID, msg string) error {
	return j.postStatus(jobID, msg, messaging.FailedState)
}

// Success sends a success update via the AMQP broker. Should be sent once.
func (j *JSLPublisher) Success(jobID, msg string) error {
	return j.postStatus(jobID, msg, messaging.SucceededState)
}

// Running sends an analysis running status update with the provided message via the
// AMQP broker. May be sent multiple times, preferably with different messages.
func (j *JSLPublisher) Running(jobID, msg string) error {
	return j.postStatus(jobID, msg, messaging.RunningState)
}

// MonitorVICEEvents fires up a goroutine that forwards events from the cluster
// to the status receiving service (probably job-status-listener).
func (e *ExposerApp) MonitorVICEEvents() {
	go func(namespace string, clientset kubernetes.Interface) {
		for {
			log.Debug("beginning to monitor k8s events")
			set := labels.Set(map[string]string{
				"app-type": "interactive",
			})

			listoptions := metav1.ListOptions{
				LabelSelector: set.AsSelector().String(),
			}

			podwatcher, err := clientset.CoreV1().Pods(namespace).Watch(listoptions)
			if err != nil {
				log.Error(err)
				return
			}
			podchan := podwatcher.ResultChan()

			depwatcher, err := clientset.AppsV1().Deployments(e.viceNamespace).Watch(listoptions)
			if err != nil {
				log.Error(err)
				return
			}
			depchan := depwatcher.ResultChan()

			svcwatcher, err := clientset.CoreV1().Services(e.viceNamespace).Watch(listoptions)
			if err != nil {
				log.Error(err)
				return
			}
			svcchan := svcwatcher.ResultChan()

			ingwatcher, err := clientset.ExtensionsV1beta1().Ingresses(e.viceNamespace).Watch(listoptions)
			if err != nil {
				log.Error(err)
				return
			}
			ingchan := ingwatcher.ResultChan()

			for {
				select {
				case event := <-podchan:

				case event := <-depchan:
				case event := <-svcchan:
				case event := <-ingchan:
				}
			}
			log.Debug("stopped monitoring k8s events, probably restarting")
		}
	}(e.namespace, e.clientset)
}

// processPodEvent will send out notifications based on pod events. Partially
// adapted/inspired by code at https://github.com/dtan4/k8s-pod-notifier/blob/master/kubernetes/client.go.
func (e *ExposerApp) processPodEvent(event *watch.Event) error {
	var err error

	obj, ok := event.Object.(*apiv1.Pod)
	if !ok {
		return errors.New("unexpected type for pod object")
	}

	jobID, ok := obj.Labels["external-id"]
	if !ok {
		return errors.New("pod is missing external-id label")
	}

	switch event.Type {
	case watch.Added:
		if err = e.statusPublisher.Running(
			jobID,
			fmt.Sprintf("pod %s has started for analysis %s", obj.Name, obj.Labels["analysis-name"]),
		); err != nil {
			return err
		}
		break
	case watch.Modified:
		if err = e.statusPublisher.Running(
			jobID,
			fmt.Sprintf("pod %s has been modified for analysis %s", obj.Name, obj.Labels["analysis-name"]),
		); err != nil {
			return err
		}
		break
	case watch.Deleted:
		// Deletions are a success. Crashes are a failure. Those usually pop up as a Modified event.
		if err = e.statusPublisher.Success(
			jobID,
			fmt.Sprintf("pod %s has been deleted for analysis %s", obj.Name, obj.Labels["analysis-name"]),
		); err != nil {
			return err
		}
		break
	default:
		for _, containerStatus := range obj.Status.ContainerStatuses {
			if containerStatus.State.Terminated == nil {
				continue
			}

			ii err = e.statusPublisher.Fail(
				jobID,
				fmt.Sprintf(
					"pod %s for analysis %s failed: %s", 
					obj.Name, 
					obj.Labels["analysis-name"], 
					containerStatus.State.Terminated.Reason
				),
			); err != nil {
				return err
			}
		}
	}

	return nil
}

func hostname() string {
	h, err := os.Hostname()
	if err != nil {
		log.Errorf("Couldn't get the hostname: %s", err.Error())
		return ""
	}
	return h
}
