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

// MetaInfo contains useful information provided by multiple resource types.
type MetaInfo struct {
	Name              string `json:"name"`
	Namespace         string `json:"namespace"`
	AnalysisName      string `json:"analysis_name"`
	AppName           string `json:"app_name"`
	AppID             string `json:"app_id"`
	ExternalID        string `json:"external_id"`
	UserID            string `json:"user_id"`
	Username          string `json:"username"`
	CreationTimestamp string `json:"creation_timestamp"`
}

// DeploymentInfo contains information returned about a Deployment.
type DeploymentInfo struct {
	MetaInfo
	Image   string   `json:"image"`
	Command []string `json:"command"`
	Port    int32    `json:"port"`
	User    int64    `json:"user"`
	Group   int64    `json:"group"`
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
		MetaInfo: MetaInfo{
			Name:              deployment.GetName(),
			Namespace:         deployment.GetNamespace(),
			AnalysisName:      labels["analysis-name"],
			AppName:           labels["app-name"],
			AppID:             labels["app-id"],
			ExternalID:        labels["external-id"],
			UserID:            labels["user-id"],
			Username:          labels["username"],
			CreationTimestamp: deployment.GetCreationTimestamp().String(),
		},

		Image:   image,
		Command: command,
		Port:    port,
		User:    user,
		Group:   group,
	}
}

// ConfigMapInfo contains useful info about a config map.
type ConfigMapInfo struct {
	MetaInfo
	Data map[string]string `json:"data"`
}

func configMapInfo(cm *corev1.ConfigMap) *ConfigMapInfo {
	labels := cm.GetObjectMeta().GetLabels()

	return &ConfigMapInfo{
		MetaInfo: MetaInfo{
			Name:              cm.GetName(),
			Namespace:         cm.GetNamespace(),
			AnalysisName:      labels["analysis-name"],
			AppName:           labels["app-name"],
			AppID:             labels["app-id"],
			ExternalID:        labels["external-id"],
			UserID:            labels["user-id"],
			Username:          labels["username"],
			CreationTimestamp: cm.GetCreationTimestamp().String(),
		},
		Data: cm.Data,
	}
}

// ServiceInfoPort contains information about a service's Port.
type ServiceInfoPort struct {
	Name           string `json:"name"`
	NodePort       int32  `json:"node_port"`
	TargetPort     int32  `json:"target_port"`
	TargetPortName string `json:"target_port_name"`
	Port           int32  `json:"port"`
	Protocol       string `json:"protocol"`
}

//ServiceInfo contains info about a service
type ServiceInfo struct {
	MetaInfo
	Ports []ServiceInfoPort `json:"ports"`
}

func serviceInfo(svc *corev1.Service) *ServiceInfo {
	labels := svc.GetObjectMeta().GetLabels()

	ports := svc.Spec.Ports
	svcInfoPorts := []ServiceInfoPort{}

	for _, port := range ports {
		svcInfoPorts = append(svcInfoPorts, ServiceInfoPort{
			Name:           port.Name,
			NodePort:       port.NodePort,
			TargetPort:     port.TargetPort.IntVal,
			TargetPortName: port.TargetPort.String(),
			Port:           port.Port,
			Protocol:       string(port.Protocol),
		})
	}

	return &ServiceInfo{
		MetaInfo: MetaInfo{
			Name:              svc.GetName(),
			Namespace:         svc.GetNamespace(),
			AnalysisName:      labels["analysis-name"],
			AppName:           labels["app-name"],
			AppID:             labels["app-id"],
			ExternalID:        labels["external-id"],
			UserID:            labels["user-id"],
			Username:          labels["username"],
			CreationTimestamp: svc.GetCreationTimestamp().String(),
		},

		Ports: svcInfoPorts,
	}
}

// IngressInfo contains useful Ingress VICE info.
type IngressInfo struct {
	MetaInfo
	DefaultBackend string                `json:"default_backend"`
	Rules          []extv1b1.IngressRule `json:"rules"`
}

func ingressInfo(ingress *extv1b1.Ingress) *IngressInfo {
	labels := ingress.GetObjectMeta().GetLabels()

	return &IngressInfo{
		MetaInfo: MetaInfo{
			Name:              ingress.GetName(),
			Namespace:         ingress.GetNamespace(),
			AnalysisName:      labels["analysis-name"],
			AppName:           labels["app-name"],
			AppID:             labels["app-id"],
			ExternalID:        labels["external-id"],
			UserID:            labels["user-id"],
			Username:          labels["username"],
			CreationTimestamp: ingress.GetCreationTimestamp().String(),
		},
		Rules: ingress.Spec.Rules,
		DefaultBackend: fmt.Sprintf(
			"%s:%d",
			ingress.Spec.Backend.ServiceName,
			ingress.Spec.Backend.ServicePort.IntValue(),
		),
	}
}

func (e *ExposerApp) getFilteredDeployments(filter map[string]string) ([]DeploymentInfo, error) {
	depList, err := e.deploymentList(e.viceNamespace, filter)
	if err != nil {
		return nil, err
	}

	deployments := []DeploymentInfo{}

	for _, dep := range depList.Items {
		info := deploymentInfo(&dep)
		deployments = append(deployments, *info)
	}

	return deployments, nil
}

// FilterableDeployments lists all of the deployments.
func (e *ExposerApp) FilterableDeployments(writer http.ResponseWriter, request *http.Request) {
	defer request.Body.Close()

	filter := filterMap(request.URL.Query())

	deployments, err := e.getFilteredDeployments(filter)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
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

func (e *ExposerApp) getFilteredConfigMaps(filter map[string]string) ([]ConfigMapInfo, error) {
	cmList, err := e.configmapsList(e.viceNamespace, filter)
	if err != nil {
		return nil, err
	}

	cms := []ConfigMapInfo{}

	for _, cm := range cmList.Items {
		info := configMapInfo(&cm)
		cms = append(cms, *info)
	}

	return cms, nil
}

// FilterableConfigMaps lists configmaps in use by VICE apps.
func (e *ExposerApp) FilterableConfigMaps(writer http.ResponseWriter, request *http.Request) {
	defer request.Body.Close()

	filter := filterMap(request.URL.Query())

	cms, err := e.getFilteredConfigMaps(filter)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	buf, err := json.Marshal(map[string][]ConfigMapInfo{
		"configmaps": cms,
	})
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	writer.Header().Add("Content-Type", "application/json")
	fmt.Fprintf(writer, string(buf))
}

func (e *ExposerApp) getFilteredServices(filter map[string]string) ([]ServiceInfo, error) {
	svcList, err := e.serviceList(e.viceNamespace, filter)
	if err != nil {
		return nil, err
	}

	svcs := []ServiceInfo{}

	for _, svc := range svcList.Items {
		info := serviceInfo(&svc)
		svcs = append(svcs, *info)
	}

	return svcs, nil
}

// FilterableServices lists services in use by VICE apps.
func (e *ExposerApp) FilterableServices(writer http.ResponseWriter, request *http.Request) {
	defer request.Body.Close()

	filter := filterMap(request.URL.Query())

	svcs, err := e.getFilteredServices(filter)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	buf, err := json.Marshal(map[string][]ServiceInfo{
		"services": svcs,
	})
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	writer.Header().Add("Content-Type", "application/json")
	fmt.Fprintf(writer, string(buf))
}

func (e *ExposerApp) getFilteredIngresses(filter map[string]string) ([]IngressInfo, error) {
	ingList, err := e.ingressList(e.viceNamespace, filter)
	if err != nil {
		return nil, err
	}

	ingresses := []IngressInfo{}

	for _, ingress := range ingList.Items {
		info := ingressInfo(&ingress)
		ingresses = append(ingresses, *info)
	}

	return ingresses, nil
}

//FilterableIngresses lists ingresses in use by VICE apps.
func (e *ExposerApp) FilterableIngresses(writer http.ResponseWriter, request *http.Request) {
	defer request.Body.Close()

	filter := filterMap(request.URL.Query())

	ingresses, err := e.getFilteredIngresses(filter)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	buf, err := json.Marshal(map[string][]IngressInfo{
		"ingresses": ingresses,
	})
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	writer.Header().Add("Content-Type", "application/json")
	fmt.Fprintf(writer, string(buf))
}

// ResourceInfo contains all of the k8s resource information about a running VICE analysis
// that we know of and care about.
type ResourceInfo struct {
	Deployments []DeploymentInfo `json:"deployments"`
	ConfigMaps  []ConfigMapInfo  `json:"config_maps"`
	Services    []ServiceInfo    `json:"services"`
	Ingresses   []IngressInfo    `json:"ingresses"`
}

// FilterableResources returns all of the k8s resources associated with a VICE analysis.
func (e *ExposerApp) FilterableResources(writer http.ResponseWriter, request *http.Request) {
	defer request.Body.Close()

	filter := filterMap(request.URL.Query())

	deployments, err := e.getFilteredDeployments(filter)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	cms, err := e.getFilteredConfigMaps(filter)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	svcs, err := e.getFilteredServices(filter)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	ingresses, err := e.getFilteredIngresses(filter)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	buf, err := json.Marshal(ResourceInfo{
		Deployments: deployments,
		ConfigMaps:  cms,
		Services:    svcs,
		Ingresses:   ingresses,
	})
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	writer.Header().Add("Content-Type", "application/json")
	fmt.Fprintf(writer, string(buf))
}
