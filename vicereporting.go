package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extv1b1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func getListOptions(customLabels map[string]string) metav1.ListOptions {
	allLabels := map[string]string{
		"app-type": "interactive",
	}

	for k, v := range customLabels {
		allLabels[k] = v
	}

	set := labels.Set(allLabels)

	return metav1.ListOptions{
		LabelSelector: set.AsSelector().String(),
	}
}

func (e *ExposerApp) deploymentList(namespace string, customLabels map[string]string) (*v1.DeploymentList, error) {
	listOptions := getListOptions(customLabels)

	depList, err := e.clientset.AppsV1().Deployments(namespace).List(listOptions)
	if err != nil {
		return nil, err
	}

	return depList, nil
}

func (e *ExposerApp) configmapsList(namespace string, customLabels map[string]string) (*corev1.ConfigMapList, error) {
	listOptions := getListOptions(customLabels)

	cfgList, err := e.clientset.CoreV1().ConfigMaps(namespace).List(listOptions)
	if err != nil {
		return nil, err
	}

	return cfgList, nil
}

func (e *ExposerApp) serviceList(namespace string, customLabels map[string]string) (*corev1.ServiceList, error) {
	listOptions := getListOptions(customLabels)

	svcList, err := e.clientset.CoreV1().Services(namespace).List(listOptions)
	if err != nil {
		return nil, err
	}

	return svcList, nil
}

func (e *ExposerApp) ingressList(namespace string, customLabels map[string]string) (*extv1b1.IngressList, error) {
	listOptions := getListOptions(customLabels)

	ingList, err := e.clientset.ExtensionsV1beta1().Ingresses(namespace).List(listOptions)
	if err != nil {
		return nil, err
	}

	return ingList, nil
}

func filterMap(values url.Values) map[string]string {
	q := map[string]string{}

	for k, v := range values {
		q[k] = v[0]
	}

	return q
}

// DeploymentInfo contains information returned about a Deployment.
type DeploymentInfo struct {
	Name              string   `json:"name"`
	Namespace         string   `json:"namespace"`
	AnalysisName      string   `json:"analysis_name"`
	AppName           string   `json:"app_name"`
	AppID             string   `json:"app_id"`
	ExternalID        string   `json:"external_id"`
	UserID            string   `json:"user_id"`
	Username          string   `json:"username"`
	CreationTimestamp string   `json:"creation_timestamp"`
	Image             string   `json:"image"`
	Command           []string `json:"command"`
	Port              int32    `json:"port"`
	User              int64    `json:"user"`
	Group             int64    `json:"group"`
}

func deploymentInfo(deployment *v1.Deployment) *DeploymentInfo {
	var (
		user    int64
		group   int64
		image   string
		port    int32
		command []string
	)

	labels := deployment.GetObjectMeta().GetLabels()
	containers := deployment.Spec.Template.Spec.Containers

	for _, container := range containers {
		if container.Name == "analysis" {
			image = container.Image
			command = container.Command
			port = container.Ports[0].ContainerPort
			user = *container.SecurityContext.RunAsUser
			group = *container.SecurityContext.RunAsGroup
		}
	}

	return &DeploymentInfo{
		Name:              deployment.GetName(),
		Namespace:         deployment.GetNamespace(),
		AnalysisName:      labels["analysis-name"],
		AppName:           labels["app-name"],
		AppID:             labels["app-id"],
		ExternalID:        labels["external-id"],
		UserID:            labels["user-id"],
		Username:          labels["username"],
		CreationTimestamp: deployment.GetCreationTimestamp().String(),
		Image:             image,
		Command:           command,
		Port:              port,
		User:              user,
		Group:             group,
	}
}

// FilterableDeployments lists all of the deployments.
func (e *ExposerApp) FilterableDeployments(writer http.ResponseWriter, request *http.Request) {
	defer request.Body.Close()

	filter := filterMap(request.URL.Query())

	depList, err := e.deploymentList(e.viceNamespace, filter)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	deployments := []DeploymentInfo{}

	for _, dep := range depList.Items {
		info := deploymentInfo(&dep)
		deployments = append(deployments, *info)
	}

	buf, err := json.Marshal(map[string][]DeploymentInfo{
		"deployments": deployments,
	})
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	writer.Header().Add("Content-Type", "application/json")
	fmt.Fprintf(writer, string(buf))
}

// FilterableConfigMaps lists configmaps in use by VICE apps.
func (e *ExposerApp) FilterableConfigMaps(writer http.ResponseWriter, request *http.Request) {
	defer request.Body.Close()

	filter := filterMap(request.URL.Query())

	cmList, err := e.configmapsList(e.viceNamespace, filter)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	buf, err := json.Marshal(cmList)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	writer.Header().Add("Content-Type", "application/json")
	fmt.Fprintf(writer, string(buf))
}

// FilterableServices lists services in use by VICE apps.
func (e *ExposerApp) FilterableServices(writer http.ResponseWriter, request *http.Request) {
	defer request.Body.Close()

	filter := filterMap(request.URL.Query())

	svcList, err := e.serviceList(e.viceNamespace, filter)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	buf, err := json.Marshal(svcList)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	writer.Header().Add("Content-Type", "application/json")
	fmt.Fprintf(writer, string(buf))
}

//FilterableIngresses lists ingresses in use by VICE apps.
func (e *ExposerApp) FilterableIngresses(writer http.ResponseWriter, request *http.Request) {
	defer request.Body.Close()

	filter := filterMap(request.URL.Query())

	ingList, err := e.ingressList(e.viceNamespace, filter)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	buf, err := json.Marshal(ingList)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	writer.Header().Add("Content-Type", "application/json")
	fmt.Fprintf(writer, string(buf))
}
