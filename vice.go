package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"
	"strings"

	jobtmpl "github.com/cyverse-de/job-templates"
	"gopkg.in/cyverse-de/model.v4"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	analysisContainerName = "analysis"

	porklockConfigVolumeName = "porklock-config"
	porklockConfigSecretName = "porklock-config"
	porklockConfigMountPath  = "/etc/porklock"

	fileTransfersVolumeName      = "input-files"
	fileTransfersContainerName   = "input-files"
	fileTransfersInputsMountPath = "/input-files"

	excludesMountPath  = "/excludes"
	excludesFileName   = "excludes-file"
	excludesVolumeName = "excludes-file"

	inputPathListMountPath  = "/input-paths"
	inputPathListFileName   = "input-path-list"
	inputPathListVolumeName = "input-path-list"

	irodsConfigFilePath = "/etc/porklock/irods-config.properties"

	fileTransfersPortName = "tcp-input"
	fileTransfersPort     = int32(60001)
)

func int32Ptr(i int32) *int32 { return &i }
func int64Ptr(i int64) *int64 { return &i }

func labelsFromJob(job *model.Job) map[string]string {
	return map[string]string{
		"app":      job.InvocationID,
		"app-name": job.AppName,
		"app-id":   job.AppID,
		"username": job.Submitter,
		"user-id":  job.UserID,
	}
}

func fileTransferCommand(job *model.Job) []string {
	retval := []string{
		"/vice-file-transfers",
		"--listen-port", "60001",
		"--user", job.Submitter,
		"--excludes-file", path.Join(excludesMountPath, excludesFileName),
		"--path-list-file", path.Join(inputPathListMountPath, inputPathListFileName),
		"--upload-destination", job.OutputDirectory(),
		"--irods-config", irodsConfigFilePath,
		"--invocation-id", job.InvocationID,
	}
	for _, fm := range job.FileMetadata {
		retval = append(retval, fm.Argument()...)
	}
	return retval
}

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

func fileTransfersMountPath(job *model.Job) string {
	return job.Steps[0].Component.Container.WorkingDirectory()
}

func excludesConfigMapName(job *model.Job) string {
	return fmt.Sprintf("excludes-file-%s", job.InvocationID)
}

func excludesConfigMap(job *model.Job) apiv1.ConfigMap {
	labels := labelsFromJob(job)

	return apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:   excludesConfigMapName(job),
			Labels: labels,
		},
		Data: map[string]string{
			excludesFileName: jobtmpl.ExcludesFileContents(job).String(),
		},
	}
}

func inputPathListConfigMapName(job *model.Job) string {
	return fmt.Sprintf("input-path-list-%s", job.InvocationID)
}

func IngressName(userID, invocationID string) string {
	return fmt.Sprintf("a%x", sha256.Sum256([]byte(fmt.Sprintf("%s%s", userID, invocationID))))[0:9]
}

func (e *ExposerApp) inputPathListConfigMap(job *model.Job) (*apiv1.ConfigMap, error) {
	labels := labelsFromJob(job)

	fileContents, err := jobtmpl.InputPathListContents(job, e.InputPathListIdentifier, e.TicketInputPathListIdentifier)
	if err != nil {
		return nil, err
	}

	return &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:   inputPathListConfigMapName(job),
			Labels: labels,
		},
		Data: map[string]string{
			inputPathListFileName: fileContents.String(),
		},
	}, nil
}

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

func (e *ExposerApp) fileTransfersVolumeMounts(job *model.Job) []apiv1.VolumeMount {
	retval := []apiv1.VolumeMount{
		{
			Name:      porklockConfigVolumeName,
			MountPath: porklockConfigMountPath,
			ReadOnly:  true,
		},
		{
			Name:      fileTransfersVolumeName,
			MountPath: fileTransfersInputsMountPath,
			ReadOnly:  false,
		},
		{
			Name:      excludesVolumeName,
			MountPath: excludesMountPath,
			ReadOnly:  true,
		},
	}

	if len(job.FilterInputsWithoutTickets()) > 0 {
		retval = append(retval, apiv1.VolumeMount{
			Name:      inputPathListVolumeName,
			MountPath: inputPathListMountPath,
			ReadOnly:  true,
		})
	}

	return retval
}

func (e *ExposerApp) deploymentContainers(job *model.Job) []apiv1.Container {
	return []apiv1.Container{
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
		},
		apiv1.Container{
			Name: analysisContainerName,
			Image: fmt.Sprintf(
				"%s:%s",
				job.Steps[0].Component.Container.Image.Name,
				job.Steps[0].Component.Container.Image.Tag,
			),
			Command: analysisCommand(&job.Steps[0]),
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
						"MKNOD",
					},
				},
			},
		},
	}
}

func (e *ExposerApp) getDeployment(job *model.Job) (*appsv1.Deployment, error) {
	labels := labelsFromJob(job)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:   job.InvocationID,
			Labels: labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": job.InvocationID,
				},
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: apiv1.PodSpec{
					RestartPolicy: apiv1.RestartPolicy("Always"),
					Volumes:       deploymentVolumes(job),
					Containers:    e.deploymentContainers(job),
					SecurityContext: &apiv1.PodSecurityContext{
						RunAsUser:  int64Ptr(int64(job.Steps[0].Component.Container.UID)),
						RunAsGroup: int64Ptr(int64(job.Steps[0].Component.Container.UID)),
						FSGroup:    int64Ptr(int64(job.Steps[0].Component.Container.UID)),
					},
				},
			},
		},
	}

	b, _ := json.Marshal(deployment)
	log.Info(string(b))

	return deployment, nil
}

func (e *ExposerApp) getService(job *model.Job, deployment *appsv1.Deployment) apiv1.Service {
	labels := labelsFromJob(job)

	svc := apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:   fmt.Sprintf("vice-%s", job.InvocationID),
			Labels: labels,
		},
		Spec: apiv1.ServiceSpec{
			Selector: map[string]string{
				"app": job.InvocationID,
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

func (e *ExposerApp) UpsertExcludesConfigMap(job *model.Job) error {
	excludesCM := excludesConfigMap(job)

	cmclient := e.clientset.CoreV1().ConfigMaps(e.viceNamespace)

	_, err := cmclient.Get(excludesConfigMapName(job), metav1.GetOptions{})
	if err != nil {
		fmt.Println(err)
		_, err = cmclient.Create(&excludesCM)
		if err != nil {
			return err
		}
	} else {
		_, err = cmclient.Update(&excludesCM)
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *ExposerApp) UpsertInputPathListConfigMap(job *model.Job) error {
	inputCM, err := e.inputPathListConfigMap(job)
	if err != nil {
		return err
	}

	cmclient := e.clientset.CoreV1().ConfigMaps(e.viceNamespace)

	_, err = cmclient.Get(inputPathListConfigMapName(job), metav1.GetOptions{})
	if err != nil {
		_, err = cmclient.Create(inputCM)
		if err != nil {
			return err
		}
	} else {
		_, err = cmclient.Update(inputCM)
		if err != nil {
			return err
		}
	}

	return nil
}

func (e *ExposerApp) UpsertDeployment(job *model.Job) error {
	deployment, err := e.getDeployment(job)
	if err != nil {
		return err
	}

	depclient := e.clientset.AppsV1().Deployments(e.viceNamespace)
	_, err = depclient.Get(job.InvocationID, metav1.GetOptions{})
	if err != nil {
		_, err = depclient.Create(deployment)
		if err != nil {
			return err
		}
	} else {
		_, err = depclient.Update(deployment)
		if err != nil {
			return err
		}
	}

	// Create the service for the job.
	svc := e.getService(job, deployment)
	svcclient := e.clientset.CoreV1().Services(e.viceNamespace)
	_, err = svcclient.Get(job.InvocationID, metav1.GetOptions{})
	if err != nil {
		_, err = svcclient.Create(&svc)
		if err != nil {
			return err
		}
	}

	// Create the ingress for the job
	ingress, err := e.getIngress(job, &svc)
	if err != nil {
		return err
	}

	ingressclient := e.clientset.ExtensionsV1beta1().Ingresses(e.viceNamespace)
	_, err = ingressclient.Get(ingress.Name, metav1.GetOptions{})
	if err != nil {
		_, err = ingressclient.Create(ingress)
		if err != nil {
			return err
		}
	}

	return nil
}

func (e *ExposerApp) LaunchApp(writer http.ResponseWriter, request *http.Request) {
	job := &model.Job{}

	buf, err := ioutil.ReadAll(request.Body)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}

	if err = json.Unmarshal(buf, job); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}

	if strings.ToLower(job.ExecutionTarget) != "interapps" {
		http.Error(
			writer,
			fmt.Errorf("job type %s is not supported by this service", job.Type).Error(),
			http.StatusBadRequest,
		)
		return
	}

	// Create the excludes file ConfigMap for the job.
	if err = e.UpsertExcludesConfigMap(job); err != nil {
		if err != nil {
			http.Error(
				writer,
				err.Error(),
				http.StatusInternalServerError,
			)
			return
		}
	}

	// Create the input path list config map
	if err = e.UpsertInputPathListConfigMap(job); err != nil {
		if err != nil {
			http.Error(
				writer,
				err.Error(),
				http.StatusInternalServerError,
			)
			return
		}
	}

	// Create the deployment for the job.
	if err = e.UpsertDeployment(job); err != nil {
		if err != nil {
			http.Error(
				writer,
				err.Error(),
				http.StatusInternalServerError,
			)
			return
		}
	}
}
