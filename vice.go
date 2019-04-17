package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"
	"strings"

	jobtmpl "gopkg.in/cyverse-de/job-templates.v5"
	"gopkg.in/cyverse-de/model.v4"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	analysisContainerName = "analysis"

	porklockConfigVolumeName = "porklock-config"
	porklockConfigSecretName = "porklock-config"
	porklockConfigMountPath  = "/etc/porklock"

	inputFilesVolumeName    = "input-files"
	inputFilesContainerName = "input-files"

	excludesMountPath  = "/excludes"
	excludesFileName   = "excludes-file"
	excludesVolumeName = "excludes-file"

	inputPathListMountPath  = "/input-paths"
	inputPathListFileName   = "input-path-list"
	inputPathListVolumeName = "input-path-list"

	outputFilesPortName = "tcp-trigger-output"
	inputFilesPortName  = "tcp-trigger-input"
	outputFilesPort     = int32(60000)
	inputFilesPort      = int32(60001)
)

func int32Ptr(i int32) *int32 { return &i }
func int64Ptr(i int64) *int64 { return &i }

func labelsFromJob(job *model.Job) map[string]string {
	return map[string]string{
		"app":         job.ID,
		"app-name":    job.AppName,
		"app-id":      job.AppID,
		"username":    job.Submitter,
		"user-id":     job.UserID,
		"description": job.Description,
		"output-dir":  job.OutputDirectory(),
	}
}

func inputCommand(job *model.Job) []string {
	prependArgs := []string{
		"nc",
		"-lk",
		"-p", "60001",
		"-e",
		"porklock", "-jar", "/usr/src/app/porklock-standalone.jar",
	}
	appendArgs := []string{
		"-z", "/etc/porklock/irods-config.json",
	}
	args := job.InputSourceListArguments(path.Join(inputPathListMountPath, inputPathListFileName))
	return append(append(prependArgs, args...), appendArgs...)
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
			Name:          fmt.Sprintf("tcp-analysis-port-%d", i),
			Protocol:      apiv1.ProtocolTCP,
		})
	}

	return ports
}

func inputFilesMountPath(job *model.Job) string {
	return job.Steps[0].Component.Container.WorkingDirectory()
}

func excludesConfigMapName(job *model.Job) string {
	return fmt.Sprintf("excludes-file-%s", job.ID)
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
	return fmt.Sprintf("includes-%s", job.ID)
}

func inputPathListConfigMap(job *model.Job) (*apiv1.ConfigMap, error) {
	labels := labelsFromJob(job)

	fileContents, err := jobtmpl.InputPathListContents(job)
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

func outputCommand(job *model.Job) []string {
	prependArgs := []string{
		"nc", "-lk", "-p", "60000", "-e", "porklock", "-jar", "/usr/src/app/porklock-standalone.jar",
	}
	appendArgs := []string{
		"-z", "/etc/porklock/irods-config.json",
	}
	args := job.FinalOutputArguments(path.Join(excludesMountPath, excludesFileName))
	return append(append(prependArgs, args...), appendArgs...)
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
			Name: inputFilesVolumeName,
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

func (e *ExposerApp) deploymentContainers(job *model.Job) []apiv1.Container {
	output := []apiv1.Container{}

	if len(job.FilterInputsWithoutTickets()) > 0 {
		output = append(output, apiv1.Container{
			Name:       inputFilesContainerName,
			Image:      fmt.Sprintf("%s:%s", e.PorklockImage, e.PorklockTag),
			Command:    inputCommand(job),
			WorkingDir: inputPathListMountPath,
			VolumeMounts: []apiv1.VolumeMount{
				{
					Name:      porklockConfigVolumeName,
					MountPath: porklockConfigMountPath,
				},
				{
					Name:      inputFilesVolumeName,
					MountPath: inputFilesMountPath(job),
				},
			},
			Ports: []apiv1.ContainerPort{
				{
					Name:          inputFilesPortName,
					ContainerPort: inputFilesPort,
					Protocol:      apiv1.Protocol("TCP"),
				},
			},
			SecurityContext: &apiv1.SecurityContext{
				RunAsUser: int64Ptr(int64(job.Steps[0].Component.Container.UID)),
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
		})
	}

	output = append(output, apiv1.Container{
		Name: analysisContainerName,
		Image: fmt.Sprintf(
			"%s:%s",
			job.Steps[0].Component.Container.Image.Name,
			job.Steps[0].Component.Container.Image.Tag,
		),
		Command: analysisCommand(&job.Steps[0]),
		VolumeMounts: []apiv1.VolumeMount{
			{
				Name:      inputFilesVolumeName,
				MountPath: inputFilesMountPath(job),
			},
		},
		Ports: analysisPorts(&job.Steps[0]),
		SecurityContext: &apiv1.SecurityContext{
			RunAsUser: int64Ptr(int64(job.Steps[0].Component.Container.UID)),
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
	})

	output = append(output, apiv1.Container{
		Name: "output-files",
		Image: fmt.Sprintf(
			"%s:%s",
			e.PorklockImage,
			e.PorklockTag,
		),
		Command: outputCommand(job),
		VolumeMounts: []apiv1.VolumeMount{
			{
				Name:      porklockConfigVolumeName,
				MountPath: porklockConfigMountPath,
			},
			{
				Name:      inputFilesVolumeName,
				MountPath: inputFilesMountPath(job),
			},
			{
				Name:      excludesVolumeName,
				MountPath: excludesMountPath,
			},
		},
		Ports: []apiv1.ContainerPort{
			apiv1.ContainerPort{
				Name:          outputFilesPortName,
				ContainerPort: outputFilesPort,
			},
		},
		SecurityContext: &apiv1.SecurityContext{
			RunAsUser: int64Ptr(int64(job.Steps[0].Component.Container.UID)),
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
	})

	return output
}

func (e *ExposerApp) getDeployment(job *model.Job) (*appsv1.Deployment, error) {
	labels := labelsFromJob(job)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:   job.ID,
			Labels: labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(2),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": job.ID,
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
				},
			},
		},
	}

	return deployment, nil
}

func (e *ExposerApp) createService(job *model.Job, deployment *appsv1.Deployment) apiv1.Service {
	labels := labelsFromJob(job)

	svc := apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:   job.ID,
			Labels: labels,
		},
		Spec: apiv1.ServiceSpec{
			Selector: map[string]string{
				"app": job.ID,
			},
			Ports: []apiv1.ServicePort{
				apiv1.ServicePort{
					Name:       outputFilesPortName,
					Protocol:   apiv1.ProtocolTCP,
					Port:       outputFilesPort,
					TargetPort: intstr.FromString(outputFilesPortName),
				},
				apiv1.ServicePort{
					Name:       inputFilesPortName,
					Protocol:   apiv1.ProtocolTCP,
					Port:       inputFilesPort,
					TargetPort: intstr.FromString(inputFilesPortName),
				},
			},
		},
	}

	var analysisContainer *apiv1.Container
	for _, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == analysisContainerName {
			analysisContainer = &container
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

func (e *ExposerApp) LaunchApp(writer http.ResponseWriter, request *http.Request) {
	// Parse DE Job JSON
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

	if strings.ToLower(job.Type) != "interactive" {
		http.Error(
			writer,
			fmt.Errorf("job type %s is not supported by this service", job.Type).Error(),
			http.StatusBadRequest,
		)
		return
	}

	deployment, err := e.getDeployment(job)
	if err != nil {
		http.Error(
			writer,
			fmt.Errorf("error creating deployment for job : %s", job.InvocationID).Error(),
			http.StatusInternalServerError,
		)
		return
	}

	svc := e.createService(job, deployment)
	excludesCM := excludesConfigMap(job)
	inputCM, err := inputPathListConfigMap(job)
	if err != nil {
		http.Error(
			writer,
			err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	// Create the excludes file ConfigMap for the job.
	cmclient := e.clientset.CoreV1().ConfigMaps(e.viceNamespace)
	_, err = cmclient.Create(&excludesCM)
	if err != nil {
		http.Error(
			writer,
			err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	// Create the includes file ConfigMap for the job.
	_, err = cmclient.Create(inputCM)
	if err != nil {
		http.Error(
			writer,
			err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	// Create the deployment for the job.
	depclient := e.clientset.AppsV1().Deployments(e.viceNamespace)
	_, err = depclient.Create(deployment)
	if err != nil {
		http.Error(
			writer,
			err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	// Create the service for the job.
	svcclient := e.clientset.CoreV1().Services(e.viceNamespace)
	_, err = svcclient.Create(&svc)
	if err != nil {
		http.Error(
			writer,
			err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

}
