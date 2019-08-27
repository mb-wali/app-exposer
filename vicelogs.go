package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// VICEStep contains information about an analysis step associated with a running
// VICE job.
type VICEStep struct {
	Name          string `json:"name"`
	ExternalID    string `json:"external_id"`
	StartDate     string `json:"startdate"`
	EndDate       string `json:"enddate"`
	Status        string `json:"status"`
	AppStepNumber int    `json:"app_step_number"`
	StepType      string `json:"step_type"`
}

// VICEAnalysis contains information about an analysis associated with a running
// VICE job.
type VICEAnalysis struct {
	AnalysisID string     `json:"analysis_id"`
	Steps      []VICEStep `json:"steps"`
	Timestamp  string     `json:"timestamp"`
	Total      int        `json:"total"`
}

func (e *ExposerApp) getExternalIDs(user, analysisID string) ([]string, error) {
	var (
		err               error
		analysisLookupURL *url.URL
	)

	analysisLookupURL, err = url.Parse(e.AppsServiceBaseURL)
	if err != nil {
		return nil, errors.Wrapf(err, "error parsing url %s", e.AppsServiceBaseURL)
	}
	analysisLookupURL.Path = path.Join("/analyses", analysisID, "steps")
	q := analysisLookupURL.Query()
	q.Set("user", user)
	analysisLookupURL.RawQuery = q.Encode()

	resp, err := http.Get(analysisLookupURL.String())
	if err != nil {
		return nil, errors.Wrapf(err, "error from GET %s", analysisLookupURL.String())
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrapf(err, "error reading response body from %s", analysisLookupURL.String())
	}

	parsedResponse := &VICEAnalysis{
		Steps: []VICEStep{},
	}

	if err = json.Unmarshal(body, parsedResponse); err != nil {
		return nil, errors.Wrapf(err, "error unmarshalling JSON from %s", analysisLookupURL.String())
	}

	retval := []string{}

	for _, step := range parsedResponse.Steps {
		retval = append(retval, step.ExternalID)
	}

	return retval, nil
}

// VICELogEntry contains the data returned for each log request.
type VICELogEntry struct {
	SinceTime string   `json:"since_time"`
	Lines     [][]byte `json:"lines"`
}

// VICELogs handles requests to access the analysis container logs for a pod in a running
// VICE app. Needs the 'id' and 'pod-name' mux Vars.
//
// Query Parameters:
//   previous - Converted to a boolean, should be either true or false. Return previously
//              terminated container logs.
//   since - Converted to a int64. The number of seconds before the current time at which
//           to begin showing logs. Yeah, that's a sentence.
//   since-time - Converted to an int64. The number of seconds since the epoch for the time at
//               which to begin showing logs.
//   tail-lines - Converted to an int64. The number of lines from the end of the log to show.
//   timestamps - Converted to a boolean, should be either true or false. Whether or not to
//                display timestamps at the beginning of each log line.
//   container - String containing the name of the container to display logs from. Defaults
//               the value 'analysis', since this is VICE-specific.
func (e *ExposerApp) VICELogs(writer http.ResponseWriter, request *http.Request) {
	var (
		err        error
		id         string
		since      int64
		sinceTime  int64
		podName    string
		container  string
		previous   bool
		tailLines  int64
		timestamps bool
		found      bool
		users      []string
		user       string
		logOpts    *apiv1.PodLogOptions
	)

	// id is required
	if id, found = mux.Vars(request)["analysis-id"]; !found {
		http.Error(writer, errors.New("id parameter is empty").Error(), http.StatusBadRequest)
		return
	}

	//podName is required
	if podName, found = mux.Vars(request)["pod"]; !found {
		http.Error(writer, errors.New("pod parameter is empty").Error(), http.StatusBadRequest)
		return
	}

	// user is required
	if users, found = request.URL.Query()["user"]; !found {
		http.Error(writer, "user is not set", http.StatusForbidden)
		return
	}
	user = users[0]

	externalIDs, err := e.getExternalIDs(user, id)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	if len(externalIDs) < 1 {
		http.Error(writer, fmt.Errorf("no external-ids found for analysis-id %s", id).Error(), http.StatusInternalServerError)
		return
	}

	//Just use the first external-id for now.
	externalID := externalIDs[0]

	logOpts = &apiv1.PodLogOptions{}
	queryParams := request.URL.Query()

	// previous is optional
	if queryParams.Get("previous") != "" {
		if previous, err = strconv.ParseBool(queryParams.Get("previous")); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}

		logOpts.Previous = previous
	}

	// since is optional
	if queryParams.Get("since") != "" {
		if since, err = strconv.ParseInt(queryParams.Get("since"), 10, 64); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}

		logOpts.SinceSeconds = &since
	}

	if queryParams.Get("since-time") != "" {
		if sinceTime, err = strconv.ParseInt(queryParams.Get("since-time"), 10, 64); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}

		convertedSinceTime := metav1.Unix(sinceTime, 0)
		logOpts.SinceTime = &convertedSinceTime
	}

	// tail-lines is optional
	if queryParams.Get("tail-lines") != "" {
		if tailLines, err = strconv.ParseInt(queryParams.Get("tail-lines"), 10, 64); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}

		logOpts.TailLines = &tailLines
	}

	logOpts.Follow = false

	// timestamps is optional
	if queryParams.Get("timestamps") != "" {
		if timestamps, err = strconv.ParseBool(queryParams.Get("timestamps")); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}

		logOpts.Timestamps = timestamps
	}

	// Make sure that the pod is actually part of the job with the provided external-id.
	pod, err := e.clientset.CoreV1().Pods(e.viceNamespace).Get(podName, metav1.GetOptions{})
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	if _, ok := pod.Labels["external-id"]; !ok {
		http.Error(writer, errors.New("pod missing external-id label").Error(), http.StatusInternalServerError)
		return
	}

	if pod.Labels["external-id"] != externalID {
		http.Error(writer, fmt.Errorf("pod's external-id label was not set to %s", id).Error(), http.StatusInternalServerError)
		return
	}

	// container is optional, but should have a default value of "analysis"
	if queryParams.Get("container") != "" {
		container = queryParams.Get("container")
	} else {
		container = "analysis"
	}

	logOpts.Container = container

	// Finally, actually get the logs and write the response out
	podLogs := e.clientset.CoreV1().Pods(e.viceNamespace).GetLogs(podName, logOpts)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	logReadCloser, err := podLogs.Stream()
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	defer logReadCloser.Close()

	bodyBytes, err := ioutil.ReadAll(logReadCloser)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	bodyLines := bytes.Split(bodyBytes, []byte("\n"))
	newSinceTime := fmt.Sprintf("%d", time.Now().Unix())

	retval := &VICELogEntry{
		SinceTime: newSinceTime,
		Lines:     bodyLines,
	}

	outBytes, err := json.Marshal(retval)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(writer, outBytes)

}

// Contains information about pods returned by the VICEPods handler.
type retPod struct {
	Name string `json:"name"`
}

// VICEPods lists the k8s pods associated with the provided external-id. For now
// just returns pod info in the format `{"pods" : [{}]}`
func (e *ExposerApp) VICEPods(writer http.ResponseWriter, request *http.Request) {
	analysisID := mux.Vars(request)["analysis-id"]
	users, found := request.URL.Query()["user"]

	if !found || len(users) < 1 {
		http.Error(writer, "user not set", http.StatusForbidden)
		return
	}

	user := users[0]

	externalIDs, err := e.getExternalIDs(user, analysisID)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	if len(externalIDs) == 0 {
		http.Error(writer, fmt.Errorf("no external-id found for analysis-id %s", analysisID).Error(), http.StatusInternalServerError)
		return
	}

	externalID := externalIDs[0]

	// For now, just use the first external ID
	set := labels.Set(map[string]string{
		"external-id": externalID,
	})

	listoptions := metav1.ListOptions{
		LabelSelector: set.AsSelector().String(),
	}

	returnedPods := []retPod{}

	podlist, err := e.clientset.CoreV1().Pods(e.viceNamespace).List(listoptions)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	for _, p := range podlist.Items {
		returnedPods = append(returnedPods, retPod{Name: p.Name})
	}

	if err = json.NewEncoder(writer).Encode(
		map[string][]retPod{
			"pods": returnedPods,
		},
	); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}

}
