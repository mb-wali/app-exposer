package main

import (
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
	ept typed_corev1.EndpointsInterface
}

// Create uses the Kubernetes API to add a new Endpoint to the indicated
// namespace.
func (e *Endpointer) Create(name, namespace, ip string, port int32) (*v1.Endpoints, error) {
	return e.ept.Create(&v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Subsets: []v1.EndpointSubset{
			{
				Addresses: []v1.EndpointAddress{{IP: ip}},
				Ports:     []v1.EndpointPort{{Port: port}},
			},
		},
	})
}

// Get returns a *v1.Endpoints for an existing Endpoints configuration in K8s.
func (e *Endpointer) Get(name string) (*v1.Endpoints, error) {
	return e.ept.Get(name, metav1.GetOptions{})
}

// Update applies updates to an existing set of Endpoints in K8s.
func (e *Endpointer) Update(name, namespace, ip string, port int32) (*v1.Endpoints, error) {
	return e.ept.Update(&v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Subsets: []v1.EndpointSubset{
			{
				Addresses: []v1.EndpointAddress{{IP: ip}},
				Ports:     []v1.EndpointPort{{Port: port}},
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

// Create uses the Kubernetes API add a new Ingress to the indicated namespace.
func (i *Ingresser) Create(name, namespace, serviceName string, servicePort int) (*extv1beta1.Ingress, error) {
	return i.ing.Create(&extv1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: extv1beta1.IngressSpec{
			Backend: &extv1beta1.IngressBackend{
				ServiceName: serviceName,
				ServicePort: intstr.FromInt(servicePort),
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
func (i *Ingresser) Update(name, namespace, serviceName string, servicePort int) (*extv1beta1.Ingress, error) {
	return i.ing.Update(&extv1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: extv1beta1.IngressSpec{
			Backend: &extv1beta1.IngressBackend{
				ServiceName: serviceName,
				ServicePort: intstr.FromInt(servicePort),
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

func main() {

}
