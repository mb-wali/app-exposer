// Contains code for managing VICE applciations running
// outside of the k8s cluster, namely in HTCondor

package external

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/client-go/kubernetes"
)

var log = logrus.WithFields(logrus.Fields{
	"service": "app-exposer",
	"art-id":  "app-exposer",
	"group":   "org.cyverse",
})

// External contains the support for running VICE apps outside of k8s.
type External struct {
	namespace          string
	clientset          kubernetes.Interface
	ServiceController  ServiceCrudder
	EndpointController EndpointCrudder
	IngressController  IngressCrudder
}

// New returns a new *External.
func New(cs kubernetes.Interface, namespace, ingressClass string) *External {
	return &External{
		clientset:          cs,
		namespace:          namespace,
		ServiceController:  NewServicer(cs.CoreV1().Services(namespace)),
		EndpointController: NewEndpointer(cs.CoreV1().Endpoints(namespace)),
		IngressController:  NewIngresser(cs.ExtensionsV1beta1().Ingresses(namespace), ingressClass),
	}
}

// WriteService uses the provided writer to write a version of the provided
// *v1.Services object out as JSON in the response body.
func WriteService(svc *v1.Service, writer http.ResponseWriter) {
	returnOpts := &ServiceOptions{
		Name:       svc.Name,
		Namespace:  svc.Namespace,
		ListenPort: svc.Spec.Ports[0].Port,
		TargetPort: svc.Spec.Ports[0].TargetPort.IntValue(),
	}

	outbuf, err := json.Marshal(returnOpts)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	writer.Write(outbuf)
}

// CreateService is an http handler for creating a Service object in a k8s cluster.
//
// Expects JSON in the request body in the following format:
// 	{
// 		"target_port" : integer,
// 		"listen_port" : integer
// 	}
//
// The name of the Service comes from the URL the request is sent to and the
// namespace is a daemon-wide configuration setting.
func (e *External) CreateService(writer http.ResponseWriter, request *http.Request) {
	defer request.Body.Close()

	var (
		service string
		ok      bool
		v       = mux.Vars(request)
	)

	if service, ok = v["name"]; !ok {
		http.Error(writer, "missing service name in the URL", http.StatusBadRequest)
		return
	}

	log.Printf("CreateService: creating a service named %s", service)

	buf, err := ioutil.ReadAll(request.Body)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	opts := &ServiceOptions{}

	err = json.Unmarshal(buf, opts)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}

	if opts.TargetPort == 0 {
		http.Error(writer, "TargetPort was either not set or set to 0", http.StatusBadRequest)
		return
	}
	log.Printf("CreateService: target port for service %s will be %d", service, opts.TargetPort)

	if opts.ListenPort == 0 {
		http.Error(writer, "ListenPort was either not set or set to 0", http.StatusBadRequest)
		return
	}
	log.Printf("CreateService: listen port for service %s will be %d", service, opts.ListenPort)

	opts.Name = service
	opts.Namespace = e.namespace

	log.Printf("CreateService: namespace for service %s will be %s", service, opts.Namespace)

	svc, err := e.ServiceController.Create(opts)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("CreateService: finished creating service %s", service)

	WriteService(svc, writer)

	log.Printf("CreateService: done writing response for creating service %s", service)
}

// UpdateService is an http handler for updating a Service object in a k8s cluster.
//
// Expects JSON in the request body in the following format:
// 	{
// 		"target_port" : integer,
// 		"listen_port" : integer
// 	}
//
// The name of the Service comes from the URL the request is sent to and the
// namespace is a daemon-wide configuration setting.
func (e *External) UpdateService(writer http.ResponseWriter, request *http.Request) {
	defer request.Body.Close()

	var (
		service string
		ok      bool
		v       = mux.Vars(request)
	)

	if service, ok = v["name"]; !ok {
		http.Error(writer, "missing service name in the URL", http.StatusBadRequest)
		return
	}

	log.Printf("UpdateService: updating service %s", service)

	buf, err := ioutil.ReadAll(request.Body)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	opts := &ServiceOptions{}

	err = json.Unmarshal(buf, opts)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}

	if opts.TargetPort == 0 {
		http.Error(writer, "TargetPort was either not set or set to 0", http.StatusBadRequest)
		return
	}
	log.Printf("UpdateService: target port for %s should be %d", service, opts.TargetPort)

	if opts.ListenPort == 0 {
		http.Error(writer, "ListenPort was either not set or set to 0", http.StatusBadRequest)
		return
	}
	log.Printf("UpdateService: listen port for %s should be %d", service, opts.ListenPort)

	opts.Name = service
	opts.Namespace = e.namespace

	log.Printf("UpdateService: namespace for %s will be %s", service, opts.Namespace)

	svc, err := e.ServiceController.Update(opts)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("UpdateService: finished updating service %s", service)

	WriteService(svc, writer)

	log.Printf("UpdateService: done writing response for updating service %s", service)
}

// GetService is an http handler for getting information about a Service object from
// a k8s cluster.
//
// Expects no body in the requests and will return a JSON encoded body in the
// response in the following format:
// 	{
// 		"name" : "The name of the service as a string.",
// 		"namespace" : "The namespace that the service is in, as a string",
// 		"target_port" : integer,
// 		"listen_port" : integer
// 	}
//
// The namespace of the Service comes from the daemon configuration setting.
func (e *External) GetService(writer http.ResponseWriter, request *http.Request) {
	var (
		service string
		ok      bool
		v       = mux.Vars(request)
	)

	if service, ok = v["name"]; !ok {
		http.Error(writer, "missing service name in the URL", http.StatusBadRequest)
		return
	}

	log.Printf("GetService: getting info for service %s", service)

	svc, err := e.ServiceController.Get(service)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("GetService: finished getting info for service %s", service)

	WriteService(svc, writer)

	log.Printf("GetService: done writing response for getting service %s", service)
}

// DeleteService is an http handler for deleting a Service object in a k8s cluster.
//
// Expects no body in the request and returns no body in the response. Returns
// a 200 status if you try to delete a Service that doesn't exist.
func (e *External) DeleteService(writer http.ResponseWriter, request *http.Request) {
	var (
		service string
		ok      bool
		v       = mux.Vars(request)
	)

	if service, ok = v["name"]; !ok {
		http.Error(writer, "missing service name in the URL", http.StatusBadRequest)
		return
	}

	log.Printf("DeleteService: deleting service %s", service)

	if err := e.ServiceController.Delete(service); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("DeleteService: finished deleting service %s", service)
}

// WriteEndpoint uses the provided writer to write a version of the provided
// *v1.Endpoints object out as JSON in the response body.
func WriteEndpoint(ept *v1.Endpoints, writer http.ResponseWriter) {
	returnOpts := &EndpointOptions{
		Name:      ept.Name,
		Namespace: ept.Namespace,
		IP:        ept.Subsets[0].Addresses[0].IP,
		Port:      ept.Subsets[0].Ports[0].Port,
	}

	outbuf, err := json.Marshal(returnOpts)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	writer.Write(outbuf)
}

// CreateEndpoint is an http handler for creating an Endpoints object in a k8s cluster.
//
// Expects JSON in the request body in the following format:
// 	{
// 		"ip" : "IP address of the external process as a string.",
// 		"port" : The target port of the external process as an integer
// 	}
//
// The name of the Endpoint is derived from the URL the request was sent to and
// the namespace comes from the daemon-wide configuration value.
func (e *External) CreateEndpoint(writer http.ResponseWriter, request *http.Request) {
	defer request.Body.Close()

	var (
		endpoint string
		ok       bool
		v        = mux.Vars(request)
	)

	if endpoint, ok = v["name"]; !ok {
		http.Error(writer, "missing endpoint name in the URL", http.StatusBadRequest)
		return
	}

	log.Printf("CreateEndpoint: creating an endpoint named %s", endpoint)

	buf, err := ioutil.ReadAll(request.Body)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	opts := &EndpointOptions{}

	if err = json.Unmarshal(buf, opts); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	if opts.IP == "" {
		http.Error(writer, "IP field is blank", http.StatusBadRequest)
		return
	}
	log.Printf("CreateEndpoint: ip for endpoint %s will be %s", endpoint, opts.IP)

	if opts.Port == 0 {
		http.Error(writer, "Port field is blank", http.StatusBadRequest)
		return
	}
	log.Printf("CreateEndpoint: port for endpoint %s will be %d", endpoint, opts.Port)

	opts.Name = endpoint
	opts.Namespace = e.namespace

	log.Printf("CreateEndpoint: namespace for endpoint %s will be %s", endpoint, opts.Namespace)

	ept, err := e.EndpointController.Create(opts)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("CreateEndpoint: finished creating endpoint %s", endpoint)

	WriteEndpoint(ept, writer)

	log.Printf("CreateEndpoint: done writing response for creating endpoint %s", endpoint)
}

// UpdateEndpoint is an http handler for updating an Endpoints object in a k8s cluster.
//
// Expects JSON in the request body in the following format:
// 	{
// 		"ip" : "IP address of the external process as a string.",
// 		"port" : The target port of the external process as an integer
// 	}
//
// The name of the Endpoint is derived from the URL the request was sent to and
// the namespace comes from the daemon-wide configuration value.
func (e *External) UpdateEndpoint(writer http.ResponseWriter, request *http.Request) {
	defer request.Body.Close()

	var (
		endpoint string
		ok       bool
		v        = mux.Vars(request)
	)

	if endpoint, ok = v["name"]; !ok {
		http.Error(writer, "missing endpoint name in the URL", http.StatusBadRequest)
		return
	}

	log.Printf("UpdateEndpoint: updating endpoint %s", endpoint)

	buf, err := ioutil.ReadAll(request.Body)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	opts := &EndpointOptions{}

	if err = json.Unmarshal(buf, opts); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	if opts.IP == "" {
		http.Error(writer, "IP field is blank", http.StatusBadRequest)
		return
	}
	log.Printf("UpdateEndpoint: ip for endpoint %s should be %s", endpoint, opts.IP)

	if opts.Port == 0 {
		http.Error(writer, "Port field is blank", http.StatusBadRequest)
		return
	}
	log.Printf("UpdateEndpoint: port for endpoint %s should be %d", endpoint, opts.Port)

	opts.Name = endpoint
	opts.Namespace = e.namespace

	log.Printf("UpdateEndpoint: namespace for endpoint %s should be %s", endpoint, opts.Namespace)

	ept, err := e.EndpointController.Update(opts)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("UpdateEndpoint: finished updating endpoint %s", endpoint)

	WriteEndpoint(ept, writer)

	log.Printf("UpdateEndpoint: done writing response for updating endpoint %s", endpoint)
}

// GetEndpoint is an http handler for getting an Endpoints object from a k8s cluster.
//
// Expects no body in the request and returns JSON in the response body in the
// following format:
// 	{
// 		"name" : "The name of the Endpoints object in Kubernetes, as a string.",
// 		"namespace" : "The namespace of the Endpoints object in Kubernetes, as a string.",
// 		"ip" : "IP address of the external process as a string.",
// 		"port" : The target port of the external process as an integer
// 	}
//
// The name of the Endpoint is derived from the URL the request was sent to and
// the namespace comes from the daemon-wide configuration value.
func (e *External) GetEndpoint(writer http.ResponseWriter, request *http.Request) {
	var (
		endpoint string
		ok       bool
		v        = mux.Vars(request)
	)

	if endpoint, ok = v["name"]; !ok {
		http.Error(writer, "missing endpoint name in the URL", http.StatusBadRequest)
		return
	}

	log.Printf("GetEndpoint: getting info on endpoint %s", endpoint)

	ept, err := e.EndpointController.Get(endpoint)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("GetEndpoint: done getting info on endpoint %s", endpoint)

	WriteEndpoint(ept, writer)

	log.Printf("GetEndpoint: done writing response for getting endpoint %s", endpoint)
}

// DeleteEndpoint is an http handler for deleting an Endpoints object from a k8s cluster.
//
// Expects no request body and returns no body in the response. Returns a 200
// if you attempt to delete an Endpoints object that doesn't exist.
func (e *External) DeleteEndpoint(writer http.ResponseWriter, request *http.Request) {
	var (
		endpoint string
		ok       bool
		v        = mux.Vars(request)
	)

	if endpoint, ok = v["name"]; !ok {
		http.Error(writer, "missing endpoint name in the URL", http.StatusBadRequest)
		return
	}

	log.Printf("DeleteEndpoint: deleting endpoint %s", endpoint)

	err := e.EndpointController.Delete(endpoint)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("DeleteEndpoint: done deleting endpoint %s", endpoint)
}

// WriteIngress uses the provided writer to write a version of the provided
// *typed_extv1beta1.Ingress object out as JSON in the response body.
func WriteIngress(ing *extv1beta1.Ingress, writer http.ResponseWriter) {
	returnOpts := &IngressOptions{
		Name:      ing.Name,
		Namespace: ing.Namespace,
		Service:   ing.Spec.Backend.ServiceName,
		Port:      ing.Spec.Backend.ServicePort.IntValue(),
	}

	outbuf, err := json.Marshal(returnOpts)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	writer.Write(outbuf)
}

// CreateIngress is an http handler for creating an Ingress object in a k8s cluster.
//
// Expects a JSON encoded request body in the following format:
// 	{
// 		"service" : "The name of the Service that the Ingress is configured for, as a string.",
// 		"port" : The port of the Service that the Ingress is configured for, as an integer
// 	}
//
// The name of the Ingress is extracted from the URL that the request is sent to.
// The namespace for the Ingress object comes from the daemon configuration setting.
func (e *External) CreateIngress(writer http.ResponseWriter, request *http.Request) {
	defer request.Body.Close()

	var (
		ingress string
		ok      bool
		v       = mux.Vars(request)
	)

	if ingress, ok = v["name"]; !ok {
		http.Error(writer, "missing ingress name in the URL", http.StatusBadRequest)
		return
	}

	log.Printf("CreateIngress: create an ingress named %s", ingress)

	buf, err := ioutil.ReadAll(request.Body)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	opts := &IngressOptions{}

	if err = json.Unmarshal(buf, opts); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	if opts.Service == "" {
		http.Error(writer, "missing service from the ingress JSON", http.StatusBadRequest)
		return
	}
	log.Printf("CreateIngress: service name for ingress %s will be %s", ingress, opts.Service)

	if opts.Port == 0 {
		http.Error(writer, "Port was either not set or set to 0", http.StatusBadRequest)
		return
	}
	log.Printf("CreateIngress: port for ingress %s will be %d", ingress, opts.Port)

	opts.Name = ingress
	opts.Namespace = e.namespace

	log.Printf("CreateIngress: namespace for ingress %s will be %s", ingress, opts.Namespace)

	ing, err := e.IngressController.Create(opts)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("CreateIngress: done creating ingress %s", ingress)

	WriteIngress(ing, writer)

	log.Printf("CreateIngress: done writing response for creating ingress %s", ingress)
}

// UpdateIngress is an http handler for updating an Ingress object in a k8s cluster.
//
// Expects a JSON encoded request body in the following format:
// 	{
// 		"service" : "The name of the Service that the Ingress is configured for, as a string.",
// 		"port" : The port of the Service that the Ingress is configured for, as an integer
// 	}
//
// The name of the Ingress is extracted from the URL that the request is sent to.
// The namespace for the Ingress object comes from the daemon configuration setting.
func (e *External) UpdateIngress(writer http.ResponseWriter, request *http.Request) {
	defer request.Body.Close()

	var (
		ingress string
		ok      bool
		v       = mux.Vars(request)
	)

	if ingress, ok = v["name"]; !ok {
		http.Error(writer, "missing ingress name in the URL", http.StatusBadRequest)
		return
	}

	log.Printf("UpdateIngress: updating ingress %s", ingress)

	buf, err := ioutil.ReadAll(request.Body)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	opts := &IngressOptions{}

	if err = json.Unmarshal(buf, opts); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	if opts.Service == "" {
		http.Error(writer, "missing service from the ingress JSON", http.StatusBadRequest)
		return
	}
	log.Printf("UpdateIngress: service for ingress %s should be %s", ingress, opts.Service)

	if opts.Port == 0 {
		http.Error(writer, "Port was either not set or set to 0", http.StatusBadRequest)
		return
	}
	log.Printf("UpdateIngress: port for ingress %s should be %d", ingress, opts.Port)

	opts.Name = ingress
	opts.Namespace = e.namespace

	log.Printf("UpdateIngress: namespace for ingress %s should be %s", ingress, opts.Namespace)

	ing, err := e.IngressController.Update(opts)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("UpdateIngress: finished updating ingress %s", ingress)

	WriteIngress(ing, writer)

	log.Printf("UpdateIngress: done writing response for updating ingress %s", ingress)
}

// GetIngress is an http handler for getting an Ingress object from a k8s cluster.
//
// Expects no request body and returns a JSON-encoded body in the response in the
// following format:
// 	{
// 		"name" : "The name of the Ingress, as a string.",
// 		"namespace" : "The Kubernetes namespace that the Ingress exists in, as a string.",
// 		"service" : "The name of the Service that the Ingress is configured for, as a string.",
// 		"port" : The port of the Service that the Ingress is configured for, as an integer
// 	}
func (e *External) GetIngress(writer http.ResponseWriter, request *http.Request) {
	var (
		ingress string
		ok      bool
		v       = mux.Vars(request)
	)

	if ingress, ok = v["name"]; !ok {
		http.Error(writer, "missing ingress name in the URL", http.StatusBadRequest)
		return
	}

	log.Printf("GetIngress: getting ingress %s", ingress)

	ing, err := e.IngressController.Get(ingress)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("GetIngress: done getting ingress %s", ingress)

	WriteIngress(ing, writer)

	log.Printf("GetIngress: done writing response for getting ingress %s", ingress)
}

// DeleteIngress is an http handler for deleting an Ingress object from a k8s cluster.
//
// Expects no request body and returns no body in the response. Returns a 200
// if you attempt to delete an Endpoints object that doesn't exist.
func (e *External) DeleteIngress(writer http.ResponseWriter, request *http.Request) {
	var (
		ingress string
		ok      bool
		v       = mux.Vars(request)
	)

	if ingress, ok = v["name"]; !ok {
		http.Error(writer, "missing ingress name in the URL", http.StatusBadRequest)
		return
	}

	log.Printf("DeleteIngress: deleting ingress %s", ingress)

	err := e.IngressController.Delete(ingress)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("DeleteIngress: done deleting ingress %s", ingress)
}
