package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gorilla/mux"

	"k8s.io/api/core/v1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// ServiceOptions contains the settings needed to create or update a Service for
// an interactive app.
type ServiceOptions struct {
	Name       string
	Namespace  string
	TargetPort int   `json:"target_port"`
	ListenPort int32 `json:"listen_port"`
}

// EndpointOptions contains the settings needed to create or update an
// Endpoint for an interactive app.
type EndpointOptions struct {
	Name      string
	Namespace string
	IP        string
	Port      int32
}

// IngressOptions contains the settings needed to create or update an Ingress
// for an interactive app.
type IngressOptions struct {
	Name      string
	Namespace string
	Service   string
	Port      int
}

// EndpointCrudder defines the interface for objects that allow CRUD operations
// on Kubernetes Endpoints. Mostly needed to facilitate testing.
type EndpointCrudder interface {
	Create(opts *EndpointOptions) (*v1.Endpoints, error)
	Get(name string) (*v1.Endpoints, error)
	Update(opts *EndpointOptions) (*v1.Endpoints, error)
	Delete(name string) error
}

// ServiceCrudder defines the interface for objects that allow CRUD operation
// on Kubernetes Services. Mostly needed to facilitate testing.
type ServiceCrudder interface {
	Create(opts *ServiceOptions) (*v1.Service, error)
	Get(name string) (*v1.Service, error)
	Update(opts *ServiceOptions) (*v1.Service, error)
	Delete(name string) error
}

// IngressCrudder defines the interface for objects that allow CRUD operations
// on Kubernetes Ingresses. Mostly needed to facilitate testing.
type IngressCrudder interface {
	Create(opts *IngressOptions) (*extv1beta1.Ingress, error)
	Get(name string) (*extv1beta1.Ingress, error)
	Update(opts *IngressOptions) (*extv1beta1.Ingress, error)
	Delete(name string) error
}

// HTTPObjectInterface defines the functions for the HTTP request handlers that
// deal with CRUD operations on k8s objects.
type HTTPObjectInterface interface {
	GetRequest(http.ResponseWriter, *http.Request)
	PutRequest(http.ResponseWriter, *http.Request)
	PostRequest(http.ResponseWriter, *http.Request)
	DeleteRequest(http.ResponseWriter, *http.Request)
}

// ExposerApp encapsulates the overall application-logic, tying together the
// REST-like API with the underlying Kubernetes API. All of the HTTP handlers
// are methods for an ExposerApp instance.
type ExposerApp struct {
	namespace          string
	ServiceController  ServiceCrudder
	EndpointController EndpointCrudder
	IngressController  IngressCrudder
	router             *mux.Router
}

// NewExposerApp creates and returns a newly instantiated *ExposerApp.
func NewExposerApp(ns string, cs *kubernetes.Clientset) *ExposerApp {
	app := &ExposerApp{
		ns,
		NewServicer(cs.CoreV1().Services(ns)),
		NewEndpointer(cs.CoreV1().Endpoints(ns)),
		NewIngresser(cs.ExtensionsV1beta1().Ingresses(ns)),
		mux.NewRouter(),
	}
	app.router.HandleFunc("/", app.Greeting).Methods("GET")
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

	if opts.ListenPort == 0 {
		http.Error(writer, "ListenPort was either not set or set to 0", http.StatusBadRequest)
		return
	}

	opts.Name = service
	opts.Namespace = e.namespace

	// Call e.ServiceController.Create(*ServicerOptions)
	svc, err := e.ServiceController.Create(opts)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	WriteService(svc, writer)
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

	if opts.ListenPort == 0 {
		http.Error(writer, "ListenPort was either not set or set to 0", http.StatusBadRequest)
		return
	}

	opts.Name = service
	opts.Namespace = e.namespace

	svc, err := e.ServiceController.Update(opts)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	WriteService(svc, writer)
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

	svc, err := e.ServiceController.Get(service)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}

	WriteService(svc, writer)
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

	if err := e.ServiceController.Delete(service); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
}

// WriteEndpoint uses the provided writer to write a version of the provided
// *v1.Endpoints object out as JSON in the response body.
func WriteEndpoint(ept *v1.Endpoints, writer http.ResponseWriter) {
	returnOpts := &EndpointOptions{
		IP:   ept.Subsets[0].Addresses[0].IP,
		Port: ept.Subsets[0].Ports[0].Port,
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

	if opts.Port == 0 {
		http.Error(writer, "Port field is blank", http.StatusBadRequest)
		return
	}

	opts.Name = endpoint
	opts.Namespace = e.namespace

	ept, err := e.EndpointController.Create(opts)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	WriteEndpoint(ept, writer)
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

	if opts.Port == 0 {
		http.Error(writer, "Port field is blank", http.StatusBadRequest)
		return
	}

	opts.Name = endpoint
	opts.Namespace = e.namespace

	ept, err := e.EndpointController.Update(opts)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	WriteEndpoint(ept, writer)
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

	ept, err := e.EndpointController.Get(endpoint)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	WriteEndpoint(ept, writer)
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

	err := e.EndpointController.Delete(endpoint)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
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
func (e *ExposerApp) CreateIngress(writer http.ResponseWriter, request *http.Request) {}

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
func (e *ExposerApp) UpdateIngress(writer http.ResponseWriter, request *http.Request) {}

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
func (e *ExposerApp) GetIngress(writer http.ResponseWriter, request *http.Request) {}

// DeleteIngress is an http handler for deleting an Ingress object from a k8s cluster.
//
// Expects no request body and returns no body in the response. Returns a 200
// if you attempt to delete an Endpoints object that doesn't exist.
func (e *ExposerApp) DeleteIngress(writer http.ResponseWriter, request *http.Request) {}

func homeDir() string {
	return os.Getenv("HOME")
}

func main() {
	var (
		err        error
		kubeconfig *string
		namespace  *string
		listenPort *int
	)
	if home := homeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	namespace = flag.String("namespace", "default", "The namespace scope this process operates on")
	listenPort = flag.Int("port", 60000, "(optional) The port to listen on")
	flag.Parse()

	var config *rest.Config
	if *kubeconfig != "" {
		config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
		if err != nil {
			panic(err.Error())
		}
	} else {
		config, err = rest.InClusterConfig()
		if err != nil {
			panic(err.Error())
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	app := NewExposerApp(*namespace, clientset)
	log.Fatal(http.ListenAndServe(strconv.Itoa(*listenPort), app.router))
}
