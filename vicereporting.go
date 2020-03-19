package main

import (
	"net/http"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func (e *ExposerApp) deploymentList(namespace string, customLabels map[string]string) ([]byte, error) {
	defaultLabels := map[string]string{
		"app-type": "interactive",
	}

	for k, v := range customLabels {
		defaultLabels[k] = v
	}

	set := labels.Set(customLabels)

	listOptions := metav1.ListOptions{
		LabelSelector: set.AsSelector().String(),
	}

	depList, err := e.clientset.AppsV1().Deployments(namespace).List(listOptions)
	if err != nil {
		return nil, err
	}

	return depList.Marshal()
}

// AllDeployments lists all of the deployments.
func (e *ExposerApp) AllDeployments(writer http.ResponseWriter, request *http.Request) {
	defer request.Body.Close()

	q := map[string]string{}

	for k, v := range request.URL.Query() {
		q[k] = v[0]
	}

	buf, err := e.deploymentList(e.viceNamespace, q)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	writer.Write(buf)
}
