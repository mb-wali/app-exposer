package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"

	"github.com/cyverse-de/messaging"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
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

			for {
				select {
				case event := <-podchan:
					if err := e.processPodEvent(&event); err != nil {
						log.Error(err)
					}
					break
				case event := <-depchan:
					if err = e.processDeploymentEvent(&event); err != nil {
						log.Error(err)
					}
					break
				}
			}
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
		err = e.statusPublisher.Running(
			jobID,
			fmt.Sprintf("pod %s has started for analysis %s", obj.Name, obj.Labels["analysis-name"]),
		)
		break
	case watch.Modified:
		err = e.eventPodModified(obj, jobID)
		break
	case watch.Deleted:
		// Deletions are a success. Crashes are a failure. Those usually pop up as a Modified event.
		err = e.statusPublisher.Success(
			jobID,
			fmt.Sprintf("pod %s has been deleted for analysis %s", obj.Name, obj.Labels["analysis-name"]),
		)
		break
	default:
		err = e.statusPublisher.Fail(
			jobID,
			fmt.Sprintf("pod %s is in an unknown state for analysis %s", obj.Name, obj.Labels["analysis-name"]),
		)
	}

	return err
}

// eventPodModified handles emitting job status updates when the pod for the
// VICE analysis generates a modified event from k8s.
func (e *ExposerApp) eventPodModified(pod *apiv1.Pod, jobID string) error {
	var err error

	analysisName := pod.Labels["analysis-name"]

	if pod.DeletionTimestamp != nil {
		// Pod was deleted at some point, don't do anything now.
		return nil
	}

	switch pod.Status.Phase {
	case apiv1.PodSucceeded: // unlikely, but we should handle it.
		err = e.statusPublisher.Success(
			jobID,
			fmt.Sprintf("pod %s marked Completed for analysis %s", pod.Name, analysisName),
		)
		break
	case apiv1.PodRunning:
		err = e.statusPublisher.Running(
			jobID,
			fmt.Sprintf("pod %s of analysis %s changed. Reason: %s", pod.Name, analysisName, pod.Status.Reason),
		)
		break
	case apiv1.PodFailed:
		err = e.statusPublisher.Fail(
			jobID,
			fmt.Sprintf("pod %s of analysis %s failed. Reason: %s", pod.Name, analysisName, pod.Status.Reason),
		)
		break
	case apiv1.PodPending:
		err = e.statusPublisher.Running(
			jobID,
			fmt.Sprintf("pod %s of analysis %s is pending", pod.Name, analysisName),
		)
		break
	default:
		err = e.statusPublisher.Fail(
			jobID,
			fmt.Sprintf("pod %s of analysis %s is in an unknown state. Marking as failed. Reason: %s", pod.Name, analysisName, pod.Status.Reason),
		)
		break
	}

	return err
}

// processDeploymentEvent sends out notifications based on deployment events.
// All notifications will be in the Running state since the app is technically
// still up while the pods are up.
func (e *ExposerApp) processDeploymentEvent(event *watch.Event) error {
	var err error

	obj, ok := event.Object.(*appsv1.Deployment)
	if !ok {
		return errors.New("unexpected type for pod object")
	}

	jobID, ok := obj.Labels["external-id"]
	if !ok {
		return errors.New("pod is missing external-id label")
	}

	switch event.Type {
	case watch.Added:
		err = e.statusPublisher.Running(
			jobID,
			fmt.Sprintf("deployment %s has started for analysis %s", obj.Name, obj.Labels["analysis-name"]),
		)
		break
	case watch.Modified:
		err = e.eventDeploymentModified(obj, jobID)
		break
	case watch.Deleted:
		err = e.statusPublisher.Running(
			jobID,
			fmt.Sprintf("deployment %s has been deleted for analysis %s", obj.Name, obj.Labels["analysis-name"]),
		)
		break
	default:
		err = e.statusPublisher.Running(
			jobID,
			fmt.Sprintf("deployment %s is in an unknown state for analysis %s", obj.Name, obj.Labels["analysis-name"]),
		)
	}

	return err
}

// eventDeploymentModified handles emitting job status updates when the pod for the
// VICE analysis generates a modified event from k8s.
func (e *ExposerApp) eventDeploymentModified(deployment *appsv1.Deployment, jobID string) error {
	var err error

	analysisName := deployment.Labels["analysis-name"]

	if deployment.DeletionTimestamp != nil {
		// Pod was deleted at some point, don't do anything now.
		return nil
	}

	err = e.statusPublisher.Running(
		jobID,
		fmt.Sprintf(
			"deployment %s for analysis %s summary: \n replicas: %d ready replicas: %d \n available replicas: %d \n unavailable replicas: %d",
			deployment.Name,
			analysisName,
			deployment.Status.Replicas,
			deployment.Status.ReadyReplicas,
			deployment.Status.AvailableReplicas,
			deployment.Status.UnavailableReplicas,
		),
	)

	return err
}

func hostname() string {
	h, err := os.Hostname()
	if err != nil {
		log.Errorf("Couldn't get the hostname: %s", err.Error())
		return ""
	}
	return h
}
