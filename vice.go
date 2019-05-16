package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	jobtmpl "github.com/cyverse-de/job-templates"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"gopkg.in/cyverse-de/model.v4"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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

	downloadBasePath = "/download"
	uploadBasePath   = "/upload"
	downloadKind     = "download"
	uploadKind       = "upload"
)

func int32Ptr(i int32) *int32 { return &i }
func int64Ptr(i int64) *int64 { return &i }

// labelsFromJob returns a map[string]string that can be used as labels for K8s resources.
func labelsFromJob(job *model.Job) map[string]string {
	name := []rune(job.Name)

	var stringmax int
	if len(name) >= 63 {
		stringmax = 62
	} else {
		stringmax = len(name) - 1
	}

	return map[string]string{
		"external-id":   job.InvocationID,
		"app-name":      job.AppName,
		"app-id":        job.AppID,
		"username":      job.Submitter,
		"user-id":       job.UserID,
		"analysis-name": string(name[:stringmax]),
		"app-type":      "interactive",
	}
}

// fileTransferCommand returns a []string containing the command to fire up the vice-file-transfers service.
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

// fileTransferMountPath returns the path to the directory containing file inputs.
func fileTransfersMountPath(job *model.Job) string {
	return job.Steps[0].Component.Container.WorkingDirectory()
}

// excludesConfigMapName returns the name of the ConfigMap containing the list
// of paths that should be excluded from file uploads to iRODS by porklock.
func excludesConfigMapName(job *model.Job) string {
	return fmt.Sprintf("excludes-file-%s", job.InvocationID)
}

// excludesConfigMap returns the ConfigMap containing the list of paths
// that should be excluded from file uploads to iRODS by porklock. This does NOT
// call the k8s API to actually create the ConfigMap, just returns the object
// that can be passed to the API.
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

// inputPathListConfigMapName returns the name of the ConfigMap containing
// the list of paths that should be downloaded from iRODS by porklock
// as input files for the VICE analysis.
func inputPathListConfigMapName(job *model.Job) string {
	return fmt.Sprintf("input-path-list-%s", job.InvocationID)
}

// IngressName returns the name of the ingress created for the running VICE
// analysis. This should match the name created in the apps service.
func IngressName(userID, invocationID string) string {
	return fmt.Sprintf("a%x", sha256.Sum256([]byte(fmt.Sprintf("%s%s", userID, invocationID))))[0:9]
}

// inputPathListConfigMap returns the ConfigMap object containing the the
// list of paths that should be downloaded from iRODS by porklock as input
// files for the VICE analysis. This does NOT call the k8s API to actually
// create the ConfigMap, just returns the object that can be passed to the API.
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

// fileTransferVolumeMounts returns the list of VolumeMounts needed by the fileTransfer
// container in the VICE analysis pod. Each VolumeMount should correspond to one of the
// Volumes returned by the deploymentVolumes() function. This does not call the k8s API.
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

// deploymentContainers returns the Containers needed for the VICE analysis
// Deployment. It does not call the k8s API.
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

// getDeployment assembles and returns the Deployment for the VICE analysis. It does
// not call the k8s API.
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
					"external-id": job.InvocationID,
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

	return deployment, nil
}

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

// UpsertExcludesConfigMap uses the Job passed in to assemble the ConfigMap
// containing the files that should not be uploaded to iRODS. It then calls
// the k8s API to create the ConfigMap if it does not already exist or to
// update it if it does.
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

// UpsertInputPathListConfigMap uses the Job passed in to assemble the ConfigMap
// containing the path list of files to download from iRODS for the VICE analysis.
// It then uses the k8s API to create the ConfigMap if it does not already exist or to
// update it if it does.
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

// UpsertDeployment uses the Job passed in to assemble a Deployment for the
// VICE analysis. If then uses the k8s API to create the Deployment if it does
// not already exist or to update it if it does.
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

// VICELaunchApp is the HTTP handler that orchestrates the launching of a VICE analysis inside
// the k8s cluster. This get passed to the router to be associated with a route. The Job
// is passed in as the body of the request.
func (e *ExposerApp) VICELaunchApp(writer http.ResponseWriter, request *http.Request) {
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

type transferResponse struct {
	UUID   string `json:"uuid"`
	Status string `json:"status"`
	Kind   string `json:"kind"`
}

func requestTransfer(svc apiv1.Service, reqpath string) (*transferResponse, error) {
	var (
		bodybytes []byte
		bodyerr   error
		jsonerr   error
		err       error
	)

	xferresp := &transferResponse{}
	svcurl := url.URL{}

	svcurl.Scheme = "http"
	svcurl.Host = fmt.Sprintf("%s.%s:%d", svc.Name, svc.Namespace, fileTransfersPort)
	svcurl.Path = reqpath

	resp, posterr := http.Post(svcurl.String(), "", nil)
	if posterr != nil {
		return nil, errors.Wrapf(posterr, "error POSTing to %s", svcurl.String())
	}
	if resp == nil {
		return nil, fmt.Errorf("response from %s was nil", svcurl.String())
	}

	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 399 {
		return nil, errors.Wrapf(posterr, "download request to %s returned %d", svcurl.String(), resp.StatusCode)
	}

	if bodybytes, bodyerr = ioutil.ReadAll(resp.Body); err != nil {
		return nil, errors.Wrapf(bodyerr, "reading body from %s failed", svcurl.String())
	}

	if jsonerr = json.Unmarshal(bodybytes, xferresp); jsonerr != nil {
		return nil, errors.Wrapf(jsonerr, "error unmarshalling json from %s", svcurl.String())
	}

	return xferresp, nil
}

func getTransferDetails(id string, svc apiv1.Service, reqpath string) (*transferResponse, error) {
	var (
		bodybytes []byte
		bodyerr   error
		jsonerr   error
		err       error
	)

	xferresp := &transferResponse{}
	svcurl := url.URL{}

	svcurl.Scheme = "http"
	svcurl.Host = fmt.Sprintf("%s.%s:%d", svc.Name, svc.Namespace, fileTransfersPort)
	svcurl.Path = reqpath

	resp, posterr := http.Get(svcurl.String())
	if posterr != nil {
		return nil, errors.Wrapf(posterr, "error on GET %s", svcurl.String())
	}
	if resp == nil {
		return nil, fmt.Errorf("response from GET %s was nil", svcurl.String())
	}

	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 399 {
		return nil, errors.Wrapf(posterr, "status request to %s returned %d", svcurl.String(), resp.StatusCode)
	}

	if bodybytes, bodyerr = ioutil.ReadAll(resp.Body); err != nil {
		return nil, errors.Wrapf(bodyerr, "reading body from %s failed", svcurl.String())
	}

	if jsonerr = json.Unmarshal(bodybytes, xferresp); jsonerr != nil {
		return nil, errors.Wrapf(jsonerr, "error unmarshalling json from %s", svcurl.String())
	}

	return xferresp, nil
}

const (
	// RequestedStatus means the the transfer has been requested but hasn't started
	RequestedStatus = "requested"

	// DownloadingStatus means that a downloading request is running
	DownloadingStatus = "downloading"

	// UploadingStatus means that an uploading request is running
	UploadingStatus = "uploading"

	// FailedStatus means that the transfer request failed
	FailedStatus = "failed"

	//CompletedStatus means that the transfer request succeeded
	CompletedStatus = "completed"
)

func isFinished(status string) bool {
	switch status {
	case FailedStatus:
		return true
	case CompletedStatus:
		return true
	default:
		return false
	}
}

// doFileTransfer handles requests to initial file transfers for a VICE
// analysis. We only need the ID of the job, nothing is required in the
// body of the request.
func (e *ExposerApp) doFileTransfer(request *http.Request, reqpath, kind string) error {
	id := mux.Vars(request)["id"]

	log.Infof("starting %s transfers for job %s", kind, id)

	// Make sure that the list of services only comes from the VICE namespace.
	svcclient := e.clientset.CoreV1().Services(e.viceNamespace)

	// Filter the list of services so only those tagged with an external-id are
	// returned. external-id is the job ID assigned by the apps service and is
	// not the same as the analysis ID.
	set := labels.Set(map[string]string{
		"external-id": id,
	})

	svclist, err := svcclient.List(metav1.ListOptions{
		LabelSelector: set.AsSelector().String(),
	})
	if err != nil {
		return err
	}

	if len(svclist.Items) < 1 {
		return fmt.Errorf("no services with a label of 'external-id=%s' were found", id)
	}

	// It's technically possibly for multiple services to provide file transfer services,
	// so we should block until all of them are complete. We're using a WaitGroup to
	// coordinate the file transfers, since they occur in separate goroutines.
	var wg sync.WaitGroup

	for _, svc := range svclist.Items {
		wg.Add(1)

		go func(svc apiv1.Service) {
			defer wg.Done()

			log.Infof("%s transfer for %s", kind, id)

			transferObj, xfererr := requestTransfer(svc, reqpath)
			if xfererr != nil {
				log.Error(xfererr)
				err = xfererr
				return
			}

			currentStatus := transferObj.Status

			for !isFinished(currentStatus) {
				// Set it again here to catch the new values set farther down.
				currentStatus = transferObj.Status

				switch currentStatus {
				case FailedStatus:
					msg := fmt.Sprintf("%s failed for job %s", kind, id)

					err = errors.New(msg)

					log.Error(err)

					if failerr := e.statusPublisher.Running(id, msg); failerr != nil {
						log.Error(failerr)
					}

					return
				case CompletedStatus:
					msg := fmt.Sprintf("%s succeeded for job %s", kind, id)

					log.Info(msg)

					if successerr := e.statusPublisher.Running(id, msg); successerr != nil {
						log.Error(successerr)
					}

					return
				case RequestedStatus:
					msg := fmt.Sprintf("%s requested for job %s", kind, id)

					log.Error(err)

					if requestederr := e.statusPublisher.Running(id, msg); requestederr != nil {
						log.Error(err)
					}

					break
				case UploadingStatus:
					msg := fmt.Sprintf("%s is in progress for job %s", kind, id)

					log.Info(msg)

					if uploadingerr := e.statusPublisher.Running(id, msg); uploadingerr != nil {
						log.Error(err)
					}

					break
				case DownloadingStatus:
					msg := fmt.Sprintf("%s is in progress for job %s", kind, id)

					log.Info(msg)

					if downloadingerr := e.statusPublisher.Running(id, msg); downloadingerr != nil {
						log.Error(err)
					}

					break
				default:
					err = fmt.Errorf("unknown status from %s: %s", svc.Spec.ClusterIP, transferObj.Status)

					log.Error(err)

					return // return and not break because we want to fail out
				}

				fullreqpath := path.Join(reqpath, transferObj.UUID)

				transferObj, xfererr = getTransferDetails(transferObj.UUID, svc, fullreqpath)
				if xfererr != nil {
					log.Error(xfererr)
					err = xfererr
					return
				}

				if transferObj == nil {
					log.Error("transferObj is nil")
					return
				}

				time.Sleep(5 * time.Second)
			}
		}(svc)
	}

	// Block until all of the file transfers are complete. There usually will only
	// be a single goroutine to wait for, but we should support more.
	wg.Wait()

	return err
}

// VICETriggerDownloads handles requests to trigger file downloads.
func (e *ExposerApp) VICETriggerDownloads(writer http.ResponseWriter, request *http.Request) {
	var err error
	if err = e.doFileTransfer(request, downloadBasePath, downloadKind); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}

// VICETriggerUploads handles requests to trigger file uploads.
func (e *ExposerApp) VICETriggerUploads(writer http.ResponseWriter, request *http.Request) {
	var err error
	if err = e.doFileTransfer(request, uploadBasePath, uploadKind); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}

// VICEExit terminates the VICE analysis deployment and cleans up
// resources asscociated with it. Does not save outputs first. Uses
// the external-id label to find all of the objects in the configured
// namespace associated with the job. Deletes the following objects:
// ingresses, services, deployments, and configmaps.
func (e *ExposerApp) VICEExit(writer http.ResponseWriter, request *http.Request) {
	id := mux.Vars(request)["id"]

	set := labels.Set(map[string]string{
		"external-id": id,
	})

	listoptions := metav1.ListOptions{
		LabelSelector: set.AsSelector().String(),
	}

	// Delete the ingress
	ingressclient := e.clientset.ExtensionsV1beta1().Ingresses(e.viceNamespace)
	ingresslist, err := ingressclient.List(listoptions)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	for _, ingress := range ingresslist.Items {
		if err = ingressclient.Delete(ingress.Name, &metav1.DeleteOptions{}); err != nil {
			log.Error(err)
		}
	}

	// Delete the service
	svcclient := e.clientset.CoreV1().Services(e.viceNamespace)
	svclist, err := svcclient.List(listoptions)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	for _, svc := range svclist.Items {
		if err = svcclient.Delete(svc.Name, &metav1.DeleteOptions{}); err != nil {
			log.Error(err)
		}
	}

	// Delete the deployment
	depclient := e.clientset.AppsV1().Deployments(e.viceNamespace)
	deplist, err := depclient.List(listoptions)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	for _, dep := range deplist.Items {
		if err = depclient.Delete(dep.Name, &metav1.DeleteOptions{}); err != nil {
			log.Error(err)
		}
	}

	// Delete the input files list and the excludes list config maps
	cmclient := e.clientset.CoreV1().ConfigMaps(e.viceNamespace)
	cmlist, err := cmclient.List(listoptions)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	for _, cm := range cmlist.Items {
		if err = cmclient.Delete(cm.Name, &metav1.DeleteOptions{}); err != nil {
			log.Error(err)
		}
	}
}

// VICESaveAndExit handles requests to save the output files in iRODS and then exit.
// The exit portion will only occur if the save operation succeeds. The operation is
// performed inside of a goroutine so that the caller isn't waiting for hours/days for
// output file transfers to complete.
func (e *ExposerApp) VICESaveAndExit(writer http.ResponseWriter, request *http.Request) {
	log.Info("save and exit called")

	// Since file transfers can take a while, we should do this asynchronously by default.
	go func(writer http.ResponseWriter, request *http.Request) {
		var err error

		log.Info("calling doFileTransfer")

		// Trigger a blocking output file transfer request.
		if err = e.doFileTransfer(request, uploadBasePath, uploadKind); err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			log.Error(err)
			return
		}

		log.Info("calling VICEExit")

		// Only tell the deployment to halt if the save worked.
		e.VICEExit(writer, request)

		log.Info("after VICEExit")
	}(writer, request)

	log.Info("leaving save and exit")
}
