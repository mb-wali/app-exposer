package main

import (
	"fmt"
	"net/url"
	"strconv"

	"gopkg.in/cyverse-de/model.v4"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	resourcev1 "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// analyisCommand returns a []string containing the command to fire up the VICE analysis.
func analysisCommand(step *model.Step) []string {
	output := []string{}
	if step.Component.Container.EntryPoint != "" {
		output = append(output, step.Component.Container.EntryPoint)
	}
	if len(step.Arguments()) != 0 {
		output = append(output, step.Arguments()...)
	}
	return output
}

// analysisPorts returns a list of container ports needed by the VICE analysis.
func analysisPorts(step *model.Step) []apiv1.ContainerPort {
	ports := []apiv1.ContainerPort{}

	for i, p := range step.Component.Container.Ports {
		ports = append(ports, apiv1.ContainerPort{
			ContainerPort: int32(p.ContainerPort),
			Name:          fmt.Sprintf("tcp-a-%d", i),
			Protocol:      apiv1.ProtocolTCP,
		})
	}

	return ports
}

// deploymentVolumes returns the Volume objects needed for the VICE analyis
// Deployment. This does NOT call the k8s API to actually create the Volumes,
// it returns the objects that can be included in the Deployment object that
// will get passed to the k8s API later. Also not that these are the Volumes,
// not the container-specific VolumeMounts.
func deploymentVolumes(job *model.Job) []apiv1.Volume {
	output := []apiv1.Volume{}

	if len(job.FilterInputsWithoutTickets()) > 0 {
		output = append(output, apiv1.Volume{
			Name: inputPathListVolumeName,
			VolumeSource: apiv1.VolumeSource{
				ConfigMap: &apiv1.ConfigMapVolumeSource{
					LocalObjectReference: apiv1.LocalObjectReference{
						Name: inputPathListConfigMapName(job),
					},
				},
			},
		})
	}

	output = append(output,
		apiv1.Volume{
			Name: fileTransfersVolumeName,
			VolumeSource: apiv1.VolumeSource{
				EmptyDir: &apiv1.EmptyDirVolumeSource{},
			},
		},
		apiv1.Volume{
			Name: porklockConfigVolumeName,
			VolumeSource: apiv1.VolumeSource{
				Secret: &apiv1.SecretVolumeSource{
					SecretName: porklockConfigSecretName,
				},
			},
		},
		apiv1.Volume{
			Name: excludesVolumeName,
			VolumeSource: apiv1.VolumeSource{
				ConfigMap: &apiv1.ConfigMapVolumeSource{
					LocalObjectReference: apiv1.LocalObjectReference{
						Name: excludesConfigMapName(job),
					},
				},
			},
		},
	)

	return output
}

func (e *ExposerApp) getFrontendURL(job *model.Job) *url.URL {
	// This should be parsed in main(), so we shouldn't worry about it here.
	frontURL, _ := url.Parse(e.FrontendBaseURL)
	frontURL.Host = fmt.Sprintf("%s.%s", IngressName(job.UserID, job.InvocationID), frontURL.Host)
	return frontURL
}

func (e *ExposerApp) viceProxyCommand(job *model.Job) []string {
	frontURL := e.getFrontendURL(job)
	backendURL := fmt.Sprintf("http://localhost:%s", strconv.Itoa(job.Steps[0].Component.Container.Ports[0].ContainerPort))

	// websocketURL := fmt.Sprintf("ws://localhost:%s", strconv.Itoa(job.Steps[0].Component.Container.Ports[0].ContainerPort))

	output := []string{
		"vice-proxy",
		"--listen-addr", fmt.Sprintf("0.0.0.0:%d", viceProxyPort),
		"--backend-url", backendURL,
		"--ws-backend-url", backendURL,
		"--cas-base-url", e.CASBaseURL,
		"--cas-validate", "validate",
		"--frontend-url", frontURL.String(),
		"--external-id", job.InvocationID,
		"--get-analysis-id-base", fmt.Sprintf("http://%s.%s", e.GetAnalysisIDService, e.VICEBackendNamespace),
		"--check-resource-access-base", fmt.Sprintf("http://%s.%s", e.CheckResourceAccessService, e.VICEBackendNamespace),
	}

	return output
}

func cpuResourceLimit(job *model.Job) float32 {
	if job.Steps[0].Component.Container.MaxCPUCores != 0 {
		return job.Steps[0].Component.Container.MaxCPUCores
	}
	return 4
}

func memResourceLimit(job *model.Job) int64 {
	if job.Steps[0].Component.Container.MemoryLimit != 0 {
		return job.Steps[0].Component.Container.MemoryLimit
	}
	return 8589934592 // 8 GB in bytes
}

var (
	defaultCPUResourceRequest, _ = resourcev1.ParseQuantity("1000m")
	defaultMemResourceRequest, _ = resourcev1.ParseQuantity("2Gi")
	defaultCPUResourceLimit, _   = resourcev1.ParseQuantity("4000m")
	defaultMemResourceLimit, _   = resourcev1.ParseQuantity("32Gi")
)

// initContainers returns a []apiv1.Container used for the InitContainers in
// the VICE app Deployment resource.
func (e *ExposerApp) initContainers(job *model.Job) []apiv1.Container {
	return []apiv1.Container{
		apiv1.Container{
			Name:            fileTransfersInitContainerName,
			Image:           fmt.Sprintf("%s:%s", e.PorklockImage, e.PorklockTag),
			Command:         append(fileTransferCommand(job), "--no-service"),
			ImagePullPolicy: apiv1.PullPolicy(apiv1.PullAlways),
			WorkingDir:      inputPathListMountPath,
			VolumeMounts:    e.fileTransfersVolumeMounts(job),
			Ports: []apiv1.ContainerPort{
				{
					Name:          fileTransfersPortName,
					ContainerPort: fileTransfersPort,
					Protocol:      apiv1.Protocol("TCP"),
				},
			},
			SecurityContext: &apiv1.SecurityContext{
				RunAsUser:  int64Ptr(int64(job.Steps[0].Component.Container.UID)),
				RunAsGroup: int64Ptr(int64(job.Steps[0].Component.Container.UID)),
				Capabilities: &apiv1.Capabilities{
					Drop: []apiv1.Capability{
						"SETPCAP",
						"AUDIT_WRITE",
						"KILL",
						"SETGID",
						"SETUID",
						"NET_BIND_SERVICE",
						"SYS_CHROOT",
						"SETFCAP",
						"FSETID",
						"NET_RAW",
						"MKNOD",
					},
				},
			},
		},
	}
}

// deploymentContainers returns the Containers needed for the VICE analysis
// Deployment. It does not call the k8s API.
func (e *ExposerApp) deploymentContainers(job *model.Job) []apiv1.Container {
	cpuLimit, err := resourcev1.ParseQuantity(fmt.Sprintf("%fm", cpuResourceLimit(job)*1000))
	if err != nil {
		log.Warn(err)
		cpuLimit = defaultCPUResourceLimit
	}

	memLimit, err := resourcev1.ParseQuantity(fmt.Sprintf("%d", memResourceLimit(job)))
	if err != nil {
		log.Warn(err)
		memLimit = defaultMemResourceLimit
	}

	analysisEnvironment := []apiv1.EnvVar{}
	for envKey, envVal := range job.Steps[0].Environment {
		analysisEnvironment = append(
			analysisEnvironment,
			apiv1.EnvVar{
				Name:  envKey,
				Value: envVal,
			},
		)
	}

	analysisEnvironment = append(
		analysisEnvironment,
		apiv1.EnvVar{
			Name:  "REDIRECT_URL",
			Value: e.getFrontendURL(job).String(),
		},
		apiv1.EnvVar{
			Name:  "IPLANT_USER",
			Value: job.Submitter,
		},
		apiv1.EnvVar{
			Name:  "IPLANT_EXECUTION_ID",
			Value: job.InvocationID,
		},
	)

	return []apiv1.Container{
		apiv1.Container{
			Name:            viceProxyContainerName,
			Image:           e.ViceProxyImage,
			Command:         e.viceProxyCommand(job),
			ImagePullPolicy: apiv1.PullPolicy(apiv1.PullAlways),
			Ports: []apiv1.ContainerPort{
				{
					Name:          viceProxyPortName,
					ContainerPort: viceProxyPort,
					Protocol:      apiv1.Protocol("TCP"),
				},
			},
			SecurityContext: &apiv1.SecurityContext{
				RunAsUser:  int64Ptr(int64(job.Steps[0].Component.Container.UID)),
				RunAsGroup: int64Ptr(int64(job.Steps[0].Component.Container.UID)),
				Capabilities: &apiv1.Capabilities{
					Drop: []apiv1.Capability{
						"SETPCAP",
						"AUDIT_WRITE",
						"KILL",
						"SETGID",
						"SETUID",
						"SYS_CHROOT",
						"SETFCAP",
						"FSETID",
						"NET_RAW",
						"MKNOD",
					},
				},
			},
			ReadinessProbe: &apiv1.Probe{
				Handler: apiv1.Handler{
					HTTPGet: &apiv1.HTTPGetAction{
						Port:   intstr.FromInt(int(viceProxyPort)),
						Scheme: apiv1.URISchemeHTTP,
						Path:   "/",
					},
				},
			},
		},
		apiv1.Container{
			Name:            fileTransfersContainerName,
			Image:           fmt.Sprintf("%s:%s", e.PorklockImage, e.PorklockTag),
			Command:         fileTransferCommand(job),
			ImagePullPolicy: apiv1.PullPolicy(apiv1.PullAlways),
			WorkingDir:      inputPathListMountPath,
			VolumeMounts:    e.fileTransfersVolumeMounts(job),
			Ports: []apiv1.ContainerPort{
				{
					Name:          fileTransfersPortName,
					ContainerPort: fileTransfersPort,
					Protocol:      apiv1.Protocol("TCP"),
				},
			},
			SecurityContext: &apiv1.SecurityContext{
				RunAsUser:  int64Ptr(int64(job.Steps[0].Component.Container.UID)),
				RunAsGroup: int64Ptr(int64(job.Steps[0].Component.Container.UID)),
				Capabilities: &apiv1.Capabilities{
					Drop: []apiv1.Capability{
						"SETPCAP",
						"AUDIT_WRITE",
						"KILL",
						"SETGID",
						"SETUID",
						"NET_BIND_SERVICE",
						"SYS_CHROOT",
						"SETFCAP",
						"FSETID",
						"NET_RAW",
						"MKNOD",
					},
				},
			},
			ReadinessProbe: &apiv1.Probe{
				Handler: apiv1.Handler{
					HTTPGet: &apiv1.HTTPGetAction{
						Port:   intstr.FromInt(int(fileTransfersPort)),
						Scheme: apiv1.URISchemeHTTP,
						Path:   "/",
					},
				},
			},
		},
		apiv1.Container{
			Name: analysisContainerName,
			Image: fmt.Sprintf(
				"%s:%s",
				job.Steps[0].Component.Container.Image.Name,
				job.Steps[0].Component.Container.Image.Tag,
			),
			ImagePullPolicy: apiv1.PullPolicy(apiv1.PullAlways),
			Command:         analysisCommand(&job.Steps[0]),
			Env:             analysisEnvironment,
			Resources: apiv1.ResourceRequirements{
				Limits: apiv1.ResourceList{
					apiv1.ResourceCPU:    cpuLimit, //job contains # cores
					apiv1.ResourceMemory: memLimit, // job contains # bytes mem
				},
				Requests: apiv1.ResourceList{
					apiv1.ResourceCPU:    defaultCPUResourceRequest,
					apiv1.ResourceMemory: defaultMemResourceRequest,
				},
			},
			VolumeMounts: []apiv1.VolumeMount{
				{
					Name:      fileTransfersVolumeName,
					MountPath: fileTransfersMountPath(job),
					ReadOnly:  false,
				},
			},
			Ports: analysisPorts(&job.Steps[0]),
			SecurityContext: &apiv1.SecurityContext{
				RunAsUser:  int64Ptr(int64(job.Steps[0].Component.Container.UID)),
				RunAsGroup: int64Ptr(int64(job.Steps[0].Component.Container.UID)),
				// Capabilities: &apiv1.Capabilities{
				// 	Drop: []apiv1.Capability{
				// 		"SETPCAP",
				// 		"AUDIT_WRITE",
				// 		"KILL",
				// 		//"SETGID",
				// 		//"SETUID",
				// 		"SYS_CHROOT",
				// 		"SETFCAP",
				// 		"FSETID",
				// 		//"MKNOD",
				// 	},
				// },
			},
			ReadinessProbe: &apiv1.Probe{
				InitialDelaySeconds: 0,
				TimeoutSeconds:      30,
				SuccessThreshold:    1,
				FailureThreshold:    10,
				PeriodSeconds:       31,
				Handler: apiv1.Handler{
					HTTPGet: &apiv1.HTTPGetAction{
						Port:   intstr.FromInt(job.Steps[0].Component.Container.Ports[0].ContainerPort),
						Scheme: apiv1.URISchemeHTTP,
						Path:   "/",
					},
				},
			},
		},
	}
}

// getDeployment assembles and returns the Deployment for the VICE analysis. It does
// not call the k8s API.
func (e *ExposerApp) getDeployment(job *model.Job) (*appsv1.Deployment, error) {
	labels := labelsFromJob(job)
	autoMount := false

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:   job.InvocationID,
			Labels: labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"external-id": job.InvocationID,
				},
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: apiv1.PodSpec{
					RestartPolicy:                apiv1.RestartPolicy("Always"),
					Volumes:                      deploymentVolumes(job),
					InitContainers:               e.initContainers(job),
					Containers:                   e.deploymentContainers(job),
					AutomountServiceAccountToken: &autoMount,
					SecurityContext: &apiv1.PodSecurityContext{
						RunAsUser:  int64Ptr(int64(job.Steps[0].Component.Container.UID)),
						RunAsGroup: int64Ptr(int64(job.Steps[0].Component.Container.UID)),
						FSGroup:    int64Ptr(int64(job.Steps[0].Component.Container.UID)),
					},
					Tolerations: []apiv1.Toleration{
						{
							Key:      viceTolerationKey,
							Operator: apiv1.TolerationOperator(viceTolerationOperator),
							Value:    viceTolerationValue,
							Effect:   apiv1.TaintEffect(viceTolerationEffect),
						},
					},
					Affinity: &apiv1.Affinity{
						NodeAffinity: &apiv1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &apiv1.NodeSelector{
								NodeSelectorTerms: []apiv1.NodeSelectorTerm{
									{
										MatchExpressions: []apiv1.NodeSelectorRequirement{
											{
												Key:      viceAffinityKey,
												Operator: apiv1.NodeSelectorOperator(viceAffinityOperator),
												Values: []string{
													viceAffinityValue,
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	return deployment, nil
}
