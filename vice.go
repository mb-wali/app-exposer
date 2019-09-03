package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/gosimple/slug"
	"github.com/pkg/errors"

	"gopkg.in/cyverse-de/model.v4"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func slugString(str string) string {
	slug.MaxLength = 63
	text := slug.Make(str)
	return strings.ReplaceAll(text, "_", "-")
}

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
		"app-name":      slugString(job.AppName),
		"app-id":        job.AppID,
		"username":      slugString(job.Submitter),
		"user-id":       job.UserID,
		"analysis-name": slugString(string(name[:stringmax])),
		"app-type":      "interactive",
	}
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

// VICETriggerDownloads handles requests to trigger file downloads.
func (e *ExposerApp) VICETriggerDownloads(writer http.ResponseWriter, request *http.Request) {
	var err error
	if err = e.doFileTransfer(request, downloadBasePath, downloadKind, true); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}

// VICETriggerUploads handles requests to trigger file uploads.
func (e *ExposerApp) VICETriggerUploads(writer http.ResponseWriter, request *http.Request) {
	var err error
	if err = e.doFileTransfer(request, uploadBasePath, uploadKind, true); err != nil {
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

	log.Infof("number of configmaps to be deleted for %s: %d", id, len(cmlist.Items))

	for _, cm := range cmlist.Items {
		log.Infof("deleting configmap %s for %s", cm.Name, id)
		if err = cmclient.Delete(cm.Name, &metav1.DeleteOptions{}); err != nil {
			log.Error(err)
		}
	}
}

func (e *ExposerApp) getIDFromHost(host string) (string, error) {
	ingressclient := e.clientset.ExtensionsV1beta1().Ingresses(e.viceNamespace)
	ingresslist, err := ingressclient.List(metav1.ListOptions{})
	if err != nil {
		return "", err
	}

	for _, ingress := range ingresslist.Items {
		for _, rule := range ingress.Spec.Rules {
			if rule.Host == host {
				return ingress.Name, nil
			}
		}
	}

	return "", fmt.Errorf("no ingress found for host %s", host)
}

// VICEStatus handles requests to check the status of a running VICE app in K8s.
// This will return an overall status and status for the individual containers in
// the app's pod. Uses the state of the readiness checks in K8s, along with the
// existence of the various resources created for the app.
func (e *ExposerApp) VICEStatus(writer http.ResponseWriter, request *http.Request) {
	var (
		ingressExists bool
		serviceExists bool
		podReady      bool
	)

	host := mux.Vars(request)["host"]

	id, err := e.getIDFromHost(host)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusNotFound)
		return
	}

	// If getIDFromHost returns without an error, then the ingress exists
	// since the ingresses are looked at for the host.
	ingressExists = true

	set := labels.Set(map[string]string{
		"external-id": id,
	})

	listoptions := metav1.ListOptions{
		LabelSelector: set.AsSelector().String(),
	}

	// check the service existence
	svcclient := e.clientset.CoreV1().Services(e.viceNamespace)
	svclist, err := svcclient.List(listoptions)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(svclist.Items) > 0 {
		serviceExists = true
	}

	// Check pod status through the deployment
	depclient := e.clientset.AppsV1().Deployments(e.viceNamespace)
	deplist, err := depclient.List(listoptions)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	for _, dep := range deplist.Items {
		if dep.Status.ReadyReplicas > 0 {
			podReady = true
		}
	}

	data := map[string]bool{
		"ready": ingressExists && serviceExists && podReady,
	}

	body, err := json.Marshal(data)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(writer, string(body))
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
		if err = e.doFileTransfer(request, uploadBasePath, uploadKind, false); err != nil {
			log.Error(err) // Log but don't exit. Possible to cancel a job that hasn't started yet
		}

		log.Info("calling VICEExit")

		e.VICEExit(writer, request)

		log.Info("after VICEExit")
	}(writer, request)

	log.Info("leaving save and exit")
}

const updateTimeLimitSQL = `
	UPDATE ONLY jobs
	   SET planned_end_date = old_value.planned_end_date + interval '72 hours'
	  FROM (SELECT planned_end_date FROM jobs WHERE id = $2) AS old_value
	 WHERE jobs.id = $2
	   AND jobs.user_id = $1
`

const getUserIDSQL = `
	SELECT users.id
	  FROM users
	 WHERE username = $1
`

// VICETimeLimitUpdate handles requests to update the time limit on an already running VICE app.
func (e *ExposerApp) VICETimeLimitUpdate(writer http.ResponseWriter, request *http.Request) {
	log.Info("update time limit called")

	var (
		err    error
		id     string
		users  []string
		user   string
		userID string
		found  bool
	)

	// user is required
	if users, found = request.URL.Query()["user"]; !found {
		http.Error(writer, "user is not set", http.StatusForbidden)
		return
	}
	user = users[0]

	// id is required
	if id, found = mux.Vars(request)["analysis-id"]; !found {
		http.Error(writer, errors.New("id parameter is empty").Error(), http.StatusBadRequest)
		return
	}

	if err = e.db.QueryRow(getUserIDSQL, user).Scan(&userID); err != nil {
		http.Error(writer, errors.Wrapf(err, "error looking user ID for %s", user).Error(), http.StatusBadRequest)
		return
	}

	if _, err = e.db.Exec(updateTimeLimitSQL, userID, id); err != nil {
		http.Error(writer, errors.Wrapf(err, "error extending time limit for user %s on analysis %s", userID, id).Error(), http.StatusBadRequest)
		return
	}

}
