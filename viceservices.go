package main

import (
	"fmt"
	"gopkg.in/cyverse-de/model.v4"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// getService assembles and returns the Service needed for the VICE analysis.
// It does not call the k8s API.
func (e *ExposerApp) getService(job *model.Job, deployment *appsv1.Deployment) apiv1.Service {
	labels := labelsFromJob(job)

	svc := apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:   fmt.Sprintf("vice-%s", job.InvocationID),
			Labels: labels,
		},
		Spec: apiv1.ServiceSpec{
			Selector: map[string]string{
				"external-id": job.InvocationID,
			},
			Ports: []apiv1.ServicePort{
				apiv1.ServicePort{
					Name:       fileTransfersPortName,
					Protocol:   apiv1.ProtocolTCP,
					Port:       fileTransfersPort,
					TargetPort: intstr.FromString(fileTransfersPortName),
				},
			},
		},
	}

	var analysisContainer apiv1.Container
	for _, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == analysisContainerName {
			analysisContainer = container
		}
	}

	for _, port := range analysisContainer.Ports {
		svc.Spec.Ports = append(svc.Spec.Ports, apiv1.ServicePort{
			Name:       port.Name,
			Protocol:   port.Protocol,
			Port:       port.ContainerPort,
			TargetPort: intstr.FromString(port.Name),
		})
	}

	return svc
}
