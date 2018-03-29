package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/gorilla/mux"

	"k8s.io/api/core/v1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	typed_corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	typed_extv1beta1 "k8s.io/client-go/kubernetes/typed/extensions/v1beta1"
)

// EndpointCrudder defines the interface for objects that allow CRUD operations
// on Kubernetes Endpoints. Mostly needed to facilitate testing.
type EndpointCrudder interface {
	Create(name, namespace, ip string, port int32) (*v1.Endpoints, error)
	Get(name string) (*v1.Endpoints, error)
	Update(name, namespace, ip string, port int32) (*v1.Endpoints, error)
	Delete(name string) error
}

// ServiceCrudder defines the interface for objects that allow CRUD operation
// on Kubernetes Services. Mostly needed to facilitate testing.
type ServiceCrudder interface {
	Create(name, namespace string, targetPort, listenPort int) (*v1.Service, error)
	Get(name string) (*v1.Service, error)
	Update(name, namespace string, targetPort, listenPort int) (*v1.Service, error)
	Delete(name string) error
}

// IngressCrudder defines the interface for objects that allow CRUD operations
// on Kubernetes Ingresses. Mostly needed to facilitate testing.
type IngressCrudder interface {
	Create(name, namespace, serviceName string, servicePort int32) (*extv1beta1.Ingress, error)
	Get(name string) (*extv1beta1.Ingress, error)
	Update(name, namespace, serviceName string, servicePort int32) (*extv1beta1.Ingress, error)
	Delete(name string) error
}

// Servicer is a concrete implementation of a ServiceCrudder.
type Servicer struct {
	svc typed_corev1.ServiceInterface
}

// NewServicer returns a newly instantiated *Servicer.
func NewServicer(s typed_corev1.ServiceInterface) *Servicer {
	return &Servicer{s}
}

// ServicerOptions contains the settings needed to create or update a Service for
// an interactive app.
type ServicerOptions struct {
	Name       string
	Namespace  string
	TargetPort int
	ListenPort int32
}

// Create uses the Kubernetes API to add a new Service to the indicated
// namespace. Yes, I know that using an int for targetPort and an int32 for
// listenPort is weird, but that weirdness comes from the underlying K8s API.
// I'm letting the weirdness percolate up the stack until I get annoyed enough
// to deal with it.
func (s *Servicer) Create(opts *ServicerOptions) (*v1.Service, error) {
	return s.svc.Create(&v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.Name,
			Namespace: opts.Namespace,
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{{TargetPort: intstr.FromInt(opts.TargetPort), Port: opts.ListenPort}},
		},
	})
}

// Get returns a *v1.Service for an existing Service.
func (s *Servicer) Get(name string) (*v1.Service, error) {
	return s.svc.Get(name, metav1.GetOptions{})
}

// Update applies updates to an existing Service.
func (s *Servicer) Update(opts *ServicerOptions) (*v1.Service, error) {
	return s.svc.Update(&v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.Name,
			Namespace: opts.Namespace,
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{{TargetPort: intstr.FromInt(opts.TargetPort), Port: opts.ListenPort}},
		},
	})
}

// Delete removes a Service from Kubernetes.
func (s *Servicer) Delete(name string) error {
	return s.svc.Delete(name, &metav1.DeleteOptions{})
}

// Endpointer is a concreate implementation of a EndpointCrudder.
type Endpointer struct {
	ept typed_corev1.EndpointsInterface
}

// EndpointerOptions contains the settings needed to create or update an
// Endpoint for an interactive app.
type EndpointerOptions struct {
	Name      string
	Namespace string
	IP        string
	Port      int32
}

// Create uses the Kubernetes API to add a new Endpoint to the indicated
// namespace.
func (e *Endpointer) Create(opts *EndpointerOptions) (*v1.Endpoints, error) {
	return e.ept.Create(&v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.Name,
			Namespace: opts.Namespace,
		},
		Subsets: []v1.EndpointSubset{
			{
				Addresses: []v1.EndpointAddress{{IP: opts.IP}},
				Ports:     []v1.EndpointPort{{Port: opts.Port}},
			},
		},
	})
}

// Get returns a *v1.Endpoints for an existing Endpoints configuration in K8s.
func (e *Endpointer) Get(name string) (*v1.Endpoints, error) {
	return e.ept.Get(name, metav1.GetOptions{})
}

// Update applies updates to an existing set of Endpoints in K8s.
func (e *Endpointer) Update(opts *EndpointerOptions) (*v1.Endpoints, error) {
	return e.ept.Update(&v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.Name,
			Namespace: opts.Namespace,
		},
		Subsets: []v1.EndpointSubset{
			{
				Addresses: []v1.EndpointAddress{{IP: opts.IP}},
				Ports:     []v1.EndpointPort{{Port: opts.Port}},
			},
		},
	})
}

// Delete removes an Endpoints object from K8s.
func (e *Endpointer) Delete(name string) error {
	return e.ept.Delete(name, &metav1.DeleteOptions{})
}

// NewEndpointer returns a newly instantiated *Endpointer.
func NewEndpointer(e typed_corev1.EndpointsInterface) *Endpointer {
	return &Endpointer{e}
}

// Ingresser is a concrete implementation of IngressCrudder
type Ingresser struct {
	ing typed_extv1beta1.IngressInterface
}

// IngresserOptions contains the settings needed to create or update an Ingress
// for an interactive app.
type IngresserOptions struct {
	Name      string
	Namespace string
	Service   string
	Port      int
}

// Create uses the Kubernetes API add a new Ingress to the indicated namespace.
func (i *Ingresser) Create(opts *IngresserOptions) (*extv1beta1.Ingress, error) {
	return i.ing.Create(&extv1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.Name,
			Namespace: opts.Namespace,
		},
		Spec: extv1beta1.IngressSpec{
			Backend: &extv1beta1.IngressBackend{
				ServiceName: opts.Service,
				ServicePort: intstr.FromInt(opts.Port),
			},
		},
	})
}

// Get returns a *extv1beta.Ingress instance for the named Ingress in the K8s
// cluster.
func (i *Ingresser) Get(name string) (*extv1beta1.Ingress, error) {
	return i.ing.Get(name, metav1.GetOptions{})
}

// Update modifies an existing Ingress stored in K8s to match the provided info.
func (i *Ingresser) Update(opts *IngresserOptions) (*extv1beta1.Ingress, error) {
	return i.ing.Update(&extv1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.Name,
			Namespace: opts.Namespace,
		},
		Spec: extv1beta1.IngressSpec{
			Backend: &extv1beta1.IngressBackend{
				ServiceName: opts.Service,
				ServicePort: intstr.FromInt(opts.Port),
			},
		},
	})
}

// Delete removes the specified Ingress from Kubernetes.
func (i *Ingresser) Delete(name string) error {
	return i.ing.Delete(name, &metav1.DeleteOptions{})
}

// NewIngresser returns a newly instantiated *Ingresser.
func NewIngresser(i typed_extv1beta1.IngressInterface) *Ingresser {
	return &Ingresser{i}
}

// HTTPObjectInterface defines the functions for the HTTP request handlers that
// deal with CRUD operations on k8s objects.
type HTTPObjectInterface interface {
	GetRequest(http.ResponseWriter, *http.Request)
	PutRequest(http.ResponseWriter, *http.Request)
	PostRequest(http.ResponseWriter, *http.Request)
	DeleteRequest(http.ResponseWriter, *http.Request)
}

// ExposerApp is the top-level object that orchestrates calls betwen the various
// Controllers and the HTTP router.
type ExposerApp struct {
	ServiceController  ServiceCrudder
	EndpointController EndpointCrudder
	IngressController  IngressCrudder
	router             *mux.Router
}

// NewExposerApp creates a new *ExposerApp and returns it.
func NewExposerApp(svc ServiceCrudder, ept EndpointCrudder, ig IngressCrudder) *ExposerApp {
	app := &ExposerApp{
		svc,
		ept,
		ig,
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

// CreateService is an http handler for creating a Service object in a k8s cluster.
func (e *ExposerApp) CreateService(writer http.ResponseWriter, request *http.Request) {}

// UpdateService is an http handler for updating a Service object in a k8s cluster.
func (e *ExposerApp) UpdateService(writer http.ResponseWriter, request *http.Request) {}

// GetService is an http handler for getting information about a Service object from
// a k8s cluster.
func (e *ExposerApp) GetService(writer http.ResponseWriter, request *http.Request) {}

// DeleteService is an http handler for deleting a Service object in a k8s cluster.
func (e *ExposerApp) DeleteService(writer http.ResponseWriter, request *http.Request) {}

// CreateEndpoint is an http handler for creating an Endpoints object in a k8s cluster.
func (e *ExposerApp) CreateEndpoint(writer http.ResponseWriter, request *http.Request) {}

// UpdateEndpoint is an http handler for updating an Endpoints object in a k8s cluster.
func (e *ExposerApp) UpdateEndpoint(writer http.ResponseWriter, request *http.Request) {}

// GetEndpoint is an http handler for getting an Endpoints object from a k8s cluster.
func (e *ExposerApp) GetEndpoint(writer http.ResponseWriter, request *http.Request) {}

// DeleteEndpoint is an http handler for deleting an Endpoints object from a k8s cluster.
func (e *ExposerApp) DeleteEndpoint(writer http.ResponseWriter, request *http.Request) {}

// CreateIngress is an http handler for creating an Ingress object in a k8s cluster.
func (e *ExposerApp) CreateIngress(writer http.ResponseWriter, request *http.Request) {}

// UpdateIngress is an http handler for updating an Ingress object in a k8s cluster.
func (e *ExposerApp) UpdateIngress(writer http.ResponseWriter, request *http.Request) {}

// GetIngress is an http handler for getting an Ingress object from a k8s cluster.
func (e *ExposerApp) GetIngress(writer http.ResponseWriter, request *http.Request) {}

// DeleteIngress is an http handler for deleting an Ingress object from a k8s cluster.
func (e *ExposerApp) DeleteIngress(writer http.ResponseWriter, request *http.Request) {}

func homeDir() string {
	return os.Getenv("HOME")
}

func main() {

}
