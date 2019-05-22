package main

import (
	"crypto/sha256"
	"fmt"

	"gopkg.in/cyverse-de/model.v4"
	apiv1 "k8s.io/api/core/v1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// IngressName returns the name of the ingress created for the running VICE
// analysis. This should match the name created in the apps service.
func IngressName(userID, invocationID string) string {
	return fmt.Sprintf("a%x", sha256.Sum256([]byte(fmt.Sprintf("%s%s", userID, invocationID))))[0:9]
}

// getIngress assembles and returns the Ingress needed for the VICE analysis.
// It does not call the k8s API.
func (e *ExposerApp) getIngress(job *model.Job, svc *apiv1.Service) (*extv1beta1.Ingress, error) {
	var (
		rules        []extv1beta1.IngressRule
		port80found  bool
		port443found bool
		firstPort    int32
		defaultPort  int32
	)

	labels := labelsFromJob(job)
	ingressName := IngressName(job.UserID, job.InvocationID)

	for _, svcport := range svc.Spec.Ports {
		if svcport.Name != fileTransfersPortName {
			if svcport.Port == 80 {
				port80found = true
			}
			if svcport.Port == 443 {
				port443found = true
			}
			if firstPort == 0 { // 0 is firstPort's default value
				firstPort = svcport.Port
			}
		}

		rules = append(rules, extv1beta1.IngressRule{
			Host: fmt.Sprintf("%s-port%d", ingressName, svcport.Port),
			IngressRuleValue: extv1beta1.IngressRuleValue{
				HTTP: &extv1beta1.HTTPIngressRuleValue{
					Paths: []extv1beta1.HTTPIngressPath{
						{
							Backend: extv1beta1.IngressBackend{
								ServiceName: svc.Name,
								ServicePort: intstr.FromInt(int(svcport.Port)),
							},
						},
					},
				},
			},
		})
	}

	if port80found {
		defaultPort = 80 // if any of the ports are 80, then that's the default.
	} else if port443found {
		defaultPort = 443 // if not, then any of the ports listed as 443 are the default.
	} else {
		defaultPort = firstPort // if not, then the first listed port is the default.
	}

	if defaultPort == 0 {
		return nil, fmt.Errorf("default port cannot be 0 for invocation %s", job.InvocationID)
	}

	backend := &extv1beta1.IngressBackend{
		ServiceName: svc.Name,
		ServicePort: intstr.FromInt(int(defaultPort)),
	}

	// Make sure the default backend is also included
	rules = append(rules, extv1beta1.IngressRule{
		Host: ingressName,
		IngressRuleValue: extv1beta1.IngressRuleValue{
			HTTP: &extv1beta1.HTTPIngressRuleValue{
				Paths: []extv1beta1.HTTPIngressPath{
					{
						Backend: *backend,
					},
				},
			},
		},
	})

	return &extv1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name: job.InvocationID,
			Annotations: map[string]string{
				"kubernetes.io/ingress.class": "nginx",
			},
			Labels: labels,
		},
		Spec: extv1beta1.IngressSpec{
			Backend: backend, // default backend
			Rules:   rules,
		},
	}, nil
}
