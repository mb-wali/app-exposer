package main

import (
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// EndpointCrudder defines the interface for objects that allow CRUD operations
// on Kubernetes Endpoints. Mostly needed to facilitate testing.
type EndpointCrudder interface {
	Create(name, namespace string, ip string, ports string) (*v1.Endpoints, error)
	Get(name string) (*v1.Endpoints, error)
	Update(name, namespace, IPs []string, ports []string) (*v1.Endpoints, error)
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

// Servicer is a concrete implementation of a ServiceCrudder.
type Servicer struct {
	svc typedcorev1.ServiceInterface
}

// NewServicer returns a newly instantiated *Servicer.
func NewServicer(s typedcorev1.ServiceInterface) *Servicer {
	return &Servicer{s}
}

// Create uses the Kubernetes API to add a new Service to the indicated
// namespace. Yes, I know that using an int for targetPort and an int32 for
// listenPort is weird, but that weirdness comes from the underlying K8s API.
// I'm letting the weirdness percolate up the stack until I get annoyed enough
// to deal with it.
func (s *Servicer) Create(name, namespace string, targetPort int, listenPort int32) (*v1.Service, error) {
	return s.svc.Create(&v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{{TargetPort: intstr.FromInt(targetPort), Port: listenPort}},
		},
	})
}

// Get returns a *v1.Service for an existing Service.
func (s *Servicer) Get(name string) (*v1.Service, error) {
	return s.svc.Get(name, metav1.GetOptions{})
}

// Update applies updates to an existing Service.
func (s *Servicer) Update(name, namespace string, targetPort int, listenPort int32) (*v1.Service, error) {
	return s.svc.Update(&v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{{TargetPort: intstr.FromInt(targetPort), Port: listenPort}},
		},
	})
}

// Delete removes a Service from Kubernetes.
func (s *Servicer) Delete(name string) error {
	return s.svc.Delete(name, &metav1.DeleteOptions{})
}

// Endpointer is a concreate implementation of a EndpointCrudder.
type Endpointer struct {
	ept typedcorev1.EndpointsInterface
}

// Create uses the Kubernetes API to add a new Endpoint to the indicated
// namespace.
func (e *Endpointer) Create(name, namespace string, IP string, port int32) (*v1.Endpoints, error) {
	return e.ept.Create(&v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Subsets: []v1.EndpointSubset{
			{
				Addresses: []v1.EndpointAddress{{IP: IP}},
				Ports:     []v1.EndpointPort{{Port: port}},
			},
		},
	})
}

// NewEndpointer returns a newly instantiated *Endpointer.
func NewEndpointer(e typedcorev1.EndpointsInterface) *Endpointer {
	return &Endpointer{e}
}

func main() {

}
