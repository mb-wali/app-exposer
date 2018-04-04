package main

// Ingresser is a concrete implementation of IngressCrudder
import (
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	typed_extv1beta1 "k8s.io/client-go/kubernetes/typed/extensions/v1beta1"
)

// IngressOptions contains the settings needed to create or update an Ingress
// for an interactive app.
type IngressOptions struct {
	Name      string
	Namespace string
	Service   string
	Port      int
}

// IngressCrudder defines the interface for objects that allow CRUD operations
// on Kubernetes Ingresses. Mostly needed to facilitate testing.
type IngressCrudder interface {
	Create(opts *IngressOptions) (*extv1beta1.Ingress, error)
	Get(name string) (*extv1beta1.Ingress, error)
	Update(opts *IngressOptions) (*extv1beta1.Ingress, error)
	Delete(name string) error
}

// Ingresser is a concrete implementation of an IngressCrudder.
type Ingresser struct {
	ing   typed_extv1beta1.IngressInterface
	class string
}

// Create uses the Kubernetes API add a new Ingress to the indicated namespace.
func (i *Ingresser) Create(opts *IngressOptions) (*extv1beta1.Ingress, error) {
	backend := &extv1beta1.IngressBackend{
		ServiceName: opts.Service,
		ServicePort: intstr.FromInt(opts.Port),
	}
	return i.ing.Create(&extv1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.Name,
			Namespace: opts.Namespace,
			Annotations: map[string]string{
				"kubernetes.io/ingress.class": i.class,
			},
		},
		Spec: extv1beta1.IngressSpec{
			Backend: backend,
			Rules: []extv1beta1.IngressRule{
				{
					Host: opts.Name, // For interactive apps, this is the job ID.
					IngressRuleValue: extv1beta1.IngressRuleValue{
						HTTP: &extv1beta1.HTTPIngressRuleValue{
							Paths: []extv1beta1.HTTPIngressPath{
								{
									Backend: *backend,
								},
							},
						},
					},
				},
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
func (i *Ingresser) Update(opts *IngressOptions) (*extv1beta1.Ingress, error) {
	backend := &extv1beta1.IngressBackend{
		ServiceName: opts.Service,
		ServicePort: intstr.FromInt(opts.Port),
	}
	return i.ing.Update(&extv1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.Name,
			Namespace: opts.Namespace,
			Annotations: map[string]string{
				"kubernetes.io/ingress.class": i.class,
			},
		},
		Spec: extv1beta1.IngressSpec{
			Backend: backend,
			Rules: []extv1beta1.IngressRule{
				{
					Host: opts.Name,
					IngressRuleValue: extv1beta1.IngressRuleValue{
						HTTP: &extv1beta1.HTTPIngressRuleValue{
							Paths: []extv1beta1.HTTPIngressPath{
								{
									Backend: *backend,
								},
							},
						},
					},
				},
			},
		},
	})
}

// Delete removes the specified Ingress from Kubernetes.
func (i *Ingresser) Delete(name string) error {
	return i.ing.Delete(name, &metav1.DeleteOptions{})
}

// NewIngresser returns a newly instantiated *Ingresser.
func NewIngresser(i typed_extv1beta1.IngressInterface, class string) *Ingresser {
	return &Ingresser{i, class}
}
