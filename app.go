package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/gorilla/mux"
	v1 "k8s.io/api/core/v1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/client-go/kubernetes"
)

// ExposerApp encapsulates the overall application-logic, tying together the
// REST-like API with the underlying Kubernetes API. All of the HTTP handlers
// are methods for an ExposerApp instance.
type ExposerApp struct {
	namespace                     string
	clientset                     kubernetes.Interface
	viceNamespace                 string
	ServiceController             ServiceCrudder
	EndpointController            EndpointCrudder
	IngressController             IngressCrudder
	PorklockImage                 string
	PorklockTag                   string
	InputPathListIdentifier       string
	TicketInputPathListIdentifier string
	router                        *mux.Router
	statusPublisher               AnalysisStatusPublisher
	ViceProxyImage                string
	CASBaseURL                    string
	FrontendBaseURL               string
	IngressBaseURL                string
	AnalysisHeader                string
	AccessHeader                  string
	ViceDefaultBackendService     string
	ViceDefaultBackendServicePort int
	GetAnalysisIDService          string
	CheckResourceAccessService    string
	VICEBackendNamespace          string
}

// ExposerAppInit contains configuration settings for creating a new ExposerApp.
type ExposerAppInit struct {
	Namespace                     string // The namespace that the Ingress settings are added to.
	ViceNamespace                 string // The namespace containing the running VICE apps.
	PorklockImage                 string // The image containing the porklock tool
	PorklockTag                   string // The docker tag for the image containing the porklock tool
	InputPathListIdentifier       string // Header line for input path lists
	TicketInputPathListIdentifier string // Header line for ticket input path lists
	statusPublisher               AnalysisStatusPublisher
	ViceProxyImage                string
	CASBaseURL                    string
	FrontendBaseURL               string
	IngressBaseURL                string
	AnalysisHeader                string
	AccessHeader                  string
	ViceDefaultBackendService     string
	ViceDefaultBackendServicePort int
	GetAnalysisIDService          string
	CheckResourceAccessService    string
	VICEBackendNamespace          string
}

// NewExposerApp creates and returns a newly instantiated *ExposerApp.
func NewExposerApp(init *ExposerAppInit, ingressClass string, cs kubernetes.Interface) *ExposerApp {
	app := &ExposerApp{
		namespace:                     init.Namespace,
		viceNamespace:                 init.ViceNamespace,
		PorklockImage:                 init.PorklockImage,
		PorklockTag:                   init.PorklockTag,
		InputPathListIdentifier:       init.InputPathListIdentifier,
		TicketInputPathListIdentifier: init.TicketInputPathListIdentifier,
		clientset:                     cs,
		ServiceController:             NewServicer(cs.CoreV1().Services(init.Namespace)),
		EndpointController:            NewEndpointer(cs.CoreV1().Endpoints(init.Namespace)),
		IngressController:             NewIngresser(cs.ExtensionsV1beta1().Ingresses(init.Namespace), ingressClass),
		router:                        mux.NewRouter(),
		statusPublisher:               init.statusPublisher,
		IngressBaseURL:                init.IngressBaseURL,
		ViceProxyImage:                init.ViceProxyImage,
		CASBaseURL:                    init.CASBaseURL,
		FrontendBaseURL:               init.FrontendBaseURL,
		AnalysisHeader:                init.AnalysisHeader,
		AccessHeader:                  init.AccessHeader,
		ViceDefaultBackendService:     init.ViceDefaultBackendService,
		ViceDefaultBackendServicePort: init.ViceDefaultBackendServicePort,
		GetAnalysisIDService:          init.GetAnalysisIDService,
		CheckResourceAccessService:    init.CheckResourceAccessService,
		VICEBackendNamespace:          init.VICEBackendNamespace,
	}
	app.router.HandleFunc("/", app.Greeting).Methods("GET")
	app.router.HandleFunc("/vice/launch", app.VICELaunchApp).Methods("POST")
	app.router.HandleFunc("/vice/{id}/download-input-files", app.VICETriggerDownloads).Methods("POST")
	app.router.HandleFunc("/vice/{id}/save-output-files", app.VICETriggerUploads).Methods("POST")
	app.router.HandleFunc("/vice/{id}/exit", app.VICEExit).Methods("POST")
	app.router.HandleFunc("/vice/{id}/save-and-exit", app.VICESaveAndExit).Methods("POST")
	app.router.HandleFunc("/vice/{id}/pods", app.VICEPods).Methods("GET")
	app.router.HandleFunc("/vice/{id}/pods/{pod}/logs", app.VICELogs).Methods("GET")
	app.router.HandleFunc("/vice/{host}/url-ready", app.VICEStatus).Methods("GET")
	app.router.HandleFunc("/service/{name}", app.CreateService).Methods("POST")
	app.router.HandleFunc("/service/{name}", app.UpdateService).Methods("PUT")
	app.router.HandleFunc("/service/{name}", app.GetService).Methods("GET")
	app.router.HandleFunc("/service/{name}", app.DeleteService).Methods("DELETE")
	app.router.HandleFunc("/endpoint/{name}", app.CreateEndpoint).Methods("POST")
	app.router.HandleFunc("/endpoint/{name}", app.UpdateEndpoint).Methods("PUT")
	app.router.HandleFunc("/endpoint/{name}", app.GetEndpoint).Methods("GET")
	app.router.HandleFunc("/endpoint/{name}", app.DeleteEndpoint).Methods("DELETE")
	app.router.HandleFunc("/ingress/{name}", app.CreateIngress).Methods("POST")
	app.router.HandleFunc("/ingress/{name}", app.UpdateIngress).Methods("PUT")
	app.router.HandleFunc("/ingress/{name}", app.GetIngress).Methods("GET")
	app.router.HandleFunc("/ingress/{name}", app.DeleteIngress).Methods("DELETE")
	return app
}

// Greeting lets the caller know that the service is up and should be receiving
// requests.
func (e *ExposerApp) Greeting(writer http.ResponseWriter, request *http.Request) {
	fmt.Fprintf(writer, "Hello from app-exposer.")
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
func (e *ExposerApp) CreateService(writer http.ResponseWriter, request *http.Request) {
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
func (e *ExposerApp) UpdateService(writer http.ResponseWriter, request *http.Request) {
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
func (e *ExposerApp) GetService(writer http.ResponseWriter, request *http.Request) {
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
func (e *ExposerApp) DeleteService(writer http.ResponseWriter, request *http.Request) {
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
func (e *ExposerApp) CreateEndpoint(writer http.ResponseWriter, request *http.Request) {
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
func (e *ExposerApp) UpdateEndpoint(writer http.ResponseWriter, request *http.Request) {
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
func (e *ExposerApp) GetEndpoint(writer http.ResponseWriter, request *http.Request) {
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
func (e *ExposerApp) DeleteEndpoint(writer http.ResponseWriter, request *http.Request) {
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
func (e *ExposerApp) CreateIngress(writer http.ResponseWriter, request *http.Request) {
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
func (e *ExposerApp) UpdateIngress(writer http.ResponseWriter, request *http.Request) {
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
func (e *ExposerApp) GetIngress(writer http.ResponseWriter, request *http.Request) {
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
func (e *ExposerApp) DeleteIngress(writer http.ResponseWriter, request *http.Request) {
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
