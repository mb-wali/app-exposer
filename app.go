package main

import (
	"database/sql"
	"fmt"
	"net/http"

	"github.com/cyverse-de/app-exposer/external"
	"github.com/cyverse-de/app-exposer/internal"
	"github.com/gorilla/mux"
	"k8s.io/client-go/kubernetes"
)

// ExposerApp encapsulates the overall application-logic, tying together the
// REST-like API with the underlying Kubernetes API. All of the HTTP handlers
// are methods for an ExposerApp instance.
type ExposerApp struct {
	external  *external.External
	internal  *internal.Internal
	namespace string
	clientset kubernetes.Interface
	router    *mux.Router
	db        *sql.DB
}

// ExposerAppInit contains configuration settings for creating a new ExposerApp.
type ExposerAppInit struct {
	Namespace                     string // The namespace that the Ingress settings are added to.
	ViceNamespace                 string // The namespace containing the running VICE apps.
	PorklockImage                 string // The image containing the porklock tool
	PorklockTag                   string // The docker tag for the image containing the porklock tool
	InputPathListIdentifier       string // Header line for input path lists
	TicketInputPathListIdentifier string // Header line for ticket input path lists
	JobStatusURL                  string
	ViceProxyImage                string
	CASBaseURL                    string
	FrontendBaseURL               string
	ViceDefaultBackendService     string
	ViceDefaultBackendServicePort int
	GetAnalysisIDService          string
	CheckResourceAccessService    string
	VICEBackendNamespace          string
	AppsServiceBaseURL            string
	db                            *sql.DB
}

// NewExposerApp creates and returns a newly instantiated *ExposerApp.
func NewExposerApp(init *ExposerAppInit, ingressClass string, cs kubernetes.Interface) *ExposerApp {
	internalInit := &internal.Init{
		ViceNamespace:                 init.ViceNamespace,
		PorklockImage:                 init.PorklockImage,
		PorklockTag:                   init.PorklockTag,
		InputPathListIdentifier:       init.InputPathListIdentifier,
		TicketInputPathListIdentifier: init.TicketInputPathListIdentifier,
		ViceProxyImage:                init.ViceProxyImage,
		CASBaseURL:                    init.CASBaseURL,
		FrontendBaseURL:               init.FrontendBaseURL,
		ViceDefaultBackendService:     init.ViceDefaultBackendService,
		ViceDefaultBackendServicePort: init.ViceDefaultBackendServicePort,
		GetAnalysisIDService:          init.GetAnalysisIDService,
		CheckResourceAccessService:    init.CheckResourceAccessService,
		VICEBackendNamespace:          init.VICEBackendNamespace,
		AppsServiceBaseURL:            init.AppsServiceBaseURL,
		JobStatusURL:                  init.JobStatusURL,
	}

	app := &ExposerApp{
		external:  external.New(cs, init.Namespace, ingressClass),
		internal:  internal.New(internalInit, init.db, cs),
		namespace: init.Namespace,
		clientset: cs,
		router:    mux.NewRouter(),
		db:        init.db,
	}
	app.router.HandleFunc("/", app.Greeting).Methods("GET")
	app.router.HandleFunc("/vice/launch", app.internal.VICELaunchApp).Methods("POST")
	app.router.HandleFunc("/vice/apply-labels", app.internal.ApplyAsyncLabelsHandler).Methods("POST")
	app.router.HandleFunc("/vice/async-data", app.internal.GetAsyncData).Methods("GET")
	app.router.HandleFunc("/vice/listing", app.internal.FilterableResources).Methods("GET")
	app.router.HandleFunc("/vice/listing/deployments", app.internal.FilterableDeployments).Methods("GET")
	app.router.HandleFunc("/vice/listing/pods", app.internal.FilterablePods).Methods("GET")
	app.router.HandleFunc("/vice/listing/configmaps", app.internal.FilterableConfigMaps).Methods("GET")
	app.router.HandleFunc("/vice/listing/services", app.internal.FilterableServices).Methods("GET")
	app.router.HandleFunc("/vice/listing/ingresses", app.internal.FilterableIngresses).Methods("GET")
	app.router.HandleFunc("/vice/{id}/download-input-files", app.internal.VICETriggerDownloads).Methods("POST")
	app.router.HandleFunc("/vice/{id}/save-output-files", app.internal.VICETriggerUploads).Methods("POST")
	app.router.HandleFunc("/vice/{id}/exit", app.internal.VICEExit).Methods("POST")
	app.router.HandleFunc("/vice/{id}/save-and-exit", app.internal.VICESaveAndExit).Methods("POST")
	app.router.HandleFunc("/vice/{analysis-id}/pods", app.internal.VICEPods).Methods("GET")
	app.router.HandleFunc("/vice/{analysis-id}/logs", app.internal.VICELogs).Methods("GET")
	app.router.HandleFunc("/vice/{analysis-id}/time-limit", app.internal.VICETimeLimitUpdate).Methods("POST")
	app.router.HandleFunc("/vice/{analysis-id}/time-limit", app.internal.VICEGetTimeLimit).Methods("GET")
	app.router.HandleFunc("/vice/{host}/url-ready", app.internal.VICEStatus).Methods("GET")

	app.router.HandleFunc("/vice/admin/analyses/{analysis-id}/download-input-files", app.internal.VICEAdminTriggerDownloads).Methods("POST")
	app.router.HandleFunc("/vice/admin/analyses/{analysis-id}/save-output-files", app.internal.VICEAdminTriggerUploads).Methods("POST")
	app.router.HandleFunc("/vice/admin/analyses/{analysis-id}/exit", app.internal.VICEAdminExit).Methods("POST")
	app.router.HandleFunc("/vice/admin/analyses/{analysis-id}/save-and-exit", app.internal.VICEAdminSaveAndExit).Methods("POST")
	app.router.HandleFunc("/vice/admin/analyses/{analysis-id}/time-limit", app.internal.VICEAdminGetTimeLimit).Methods("GET")
	app.router.HandleFunc("/vice/admin/analyses/{analysis-id}/time-limit", app.internal.VICEAdminTimeLimitUpdate).Methods("POST")
	// app.router.HandleFunc("/vice/admin/analyses/{analysis-id}/external-id", app.internal.VICEAdminGetExternalID).Methods("GET")

	app.router.HandleFunc("/service/{name}", app.external.CreateService).Methods("POST")
	app.router.HandleFunc("/service/{name}", app.external.UpdateService).Methods("PUT")
	app.router.HandleFunc("/service/{name}", app.external.GetService).Methods("GET")
	app.router.HandleFunc("/service/{name}", app.external.DeleteService).Methods("DELETE")
	app.router.HandleFunc("/endpoint/{name}", app.external.CreateEndpoint).Methods("POST")
	app.router.HandleFunc("/endpoint/{name}", app.external.UpdateEndpoint).Methods("PUT")
	app.router.HandleFunc("/endpoint/{name}", app.external.GetEndpoint).Methods("GET")
	app.router.HandleFunc("/endpoint/{name}", app.external.DeleteEndpoint).Methods("DELETE")
	app.router.HandleFunc("/ingress/{name}", app.external.CreateIngress).Methods("POST")
	app.router.HandleFunc("/ingress/{name}", app.external.UpdateIngress).Methods("PUT")
	app.router.HandleFunc("/ingress/{name}", app.external.GetIngress).Methods("GET")
	app.router.HandleFunc("/ingress/{name}", app.external.DeleteIngress).Methods("DELETE")
	return app
}

// Greeting lets the caller know that the service is up and should be receiving
// requests.
func (e *ExposerApp) Greeting(writer http.ResponseWriter, request *http.Request) {
	fmt.Fprintf(writer, "Hello from app-exposer.")
}
