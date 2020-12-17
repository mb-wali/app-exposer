package main

import (
	"net/http"

	"github.com/cyverse-de/app-exposer/common"
	"github.com/cyverse-de/app-exposer/external"
	"github.com/cyverse-de/app-exposer/instantlaunches"
	"github.com/cyverse-de/app-exposer/internal"
	"github.com/jmoiron/sqlx"
	"k8s.io/client-go/kubernetes"

	"github.com/labstack/echo/v4"
)

// ExposerApp encapsulates the overall application-logic, tying together the
// REST-like API with the underlying Kubernetes API. All of the HTTP handlers
// are methods for an ExposerApp instance.
type ExposerApp struct {
	external        *external.External
	internal        *internal.Internal
	namespace       string
	clientset       kubernetes.Interface
	router          *echo.Echo
	db              *sqlx.DB
	instantlaunches *instantlaunches.App
}

// ExposerAppInit contains configuration settings for creating a new ExposerApp.
type ExposerAppInit struct {
	Namespace                     string // The namespace that the Ingress settings are added to.
	ViceNamespace                 string // The namespace containing the running VICE apps.
	PorklockImage                 string // The image containing the porklock tool
	PorklockTag                   string // The docker tag for the image containing the porklock tool
	UseCSIDriver                  bool   // Yes to use CSI Driver for data input/output, No to use Vice-file-transfer
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
	db                            *sqlx.DB
	UserSuffix                    string
}

// NewExposerApp creates and returns a newly instantiated *ExposerApp.
func NewExposerApp(init *ExposerAppInit, ingressClass string, cs kubernetes.Interface) *ExposerApp {
	internalInit := &internal.Init{
		ViceNamespace:                 init.ViceNamespace,
		PorklockImage:                 init.PorklockImage,
		PorklockTag:                   init.PorklockTag,
		UseCSIDriver:                  init.UseCSIDriver,
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
		UserSuffix:                    init.UserSuffix,
	}

	app := &ExposerApp{
		external:  external.New(cs, init.Namespace, ingressClass),
		internal:  internal.New(internalInit, init.db, cs),
		namespace: init.Namespace,
		clientset: cs,
		router:    echo.New(),
		db:        init.db,
	}

	app.router.HTTPErrorHandler = func(err error, c echo.Context) {
		code := http.StatusInternalServerError

		if echoErr, ok := err.(*echo.HTTPError); ok {
			code = echoErr.Code
		}

		c.JSON(code, common.NewErrorResponse(err))
	}

	app.router.GET("/", app.Greeting).Name = "greeting"
	app.router.Static("/docs", "./docs")

	vice := app.router.Group("/vice")
	vice.POST("/launch", app.internal.VICELaunchApp)
	vice.POST("/apply-labels", app.internal.ApplyAsyncLabelsHandler)
	vice.GET("/async-data", app.internal.GetAsyncData)
	vice.GET("/listing", app.internal.FilterableResources)
	vice.POST("/:id/download-input-files", app.internal.VICETriggerDownloads)
	vice.POST("/:id/save-output-files", app.internal.VICETriggerUploads)
	vice.POST("/:id/exit", app.internal.VICEExit)
	vice.POST("/:id/save-and-exit", app.internal.VICESaveAndExit)
	vice.GET("/:analysis-id/pods", app.internal.VICEPods)
	vice.GET("/:analysis-id/logs", app.internal.VICELogs)
	vice.POST("/:analysis-id/time-limit", app.internal.VICETimeLimitUpdate)
	vice.GET("/:analysis-id/time-limit", app.internal.VICEGetTimeLimit)
	vice.GET("/:host/url-ready", app.internal.VICEStatus)

	vicelisting := vice.Group("/listing")
	vicelisting.GET("/", app.internal.FilterableResources)
	vicelisting.GET("/deployments", app.internal.FilterableDeployments)
	vicelisting.GET("/pods", app.internal.FilterablePods)
	vicelisting.GET("/configmaps", app.internal.FilterableConfigMaps)
	vicelisting.GET("/services", app.internal.FilterableServices)
	vicelisting.GET("/ingresses", app.internal.FilterableIngresses)

	viceadmin := vice.Group("/admin/analyses")
	viceadmin.POST("/:analysis-id/download-input-files", app.internal.VICEAdminTriggerDownloads)
	viceadmin.POST("/:analysis-id/save-output-files", app.internal.VICEAdminTriggerUploads)
	viceadmin.POST("/:analysis-id/exit", app.internal.VICEAdminExit)
	viceadmin.POST("/:analysis-id/save-and-exit", app.internal.VICEAdminSaveAndExit)
	viceadmin.GET("/:analysis-id/time-limit", app.internal.VICEAdminGetTimeLimit)
	viceadmin.POST("/:analysis-id/time-limit", app.internal.VICEAdminTimeLimitUpdate)
	viceadmin.GET("/:analysis-id/external-id", app.internal.VICEAdminGetExternalID)

	svc := app.router.Group("/service")
	svc.POST("/:name", app.external.CreateService)
	svc.PUT("/:name", app.external.UpdateService)
	svc.GET("/:name", app.external.GetService)
	svc.DELETE("/:name", app.external.DeleteService)

	endpoint := app.router.Group("/endpoint")
	endpoint.POST("/:name", app.external.CreateEndpoint)
	endpoint.PUT("/:name", app.external.UpdateEndpoint)
	endpoint.GET("/:name", app.external.GetEndpoint)
	endpoint.DELETE("/:name", app.external.DeleteEndpoint)

	ingress := app.router.Group("/ingress")
	ingress.POST("/:name", app.external.CreateIngress)
	ingress.PUT("/:name", app.external.UpdateIngress)
	ingress.GET("/:name", app.external.GetIngress)
	ingress.DELETE("/:name", app.external.DeleteIngress)

	ilgroup := app.router.Group("/instantlaunches")
	app.instantlaunches = instantlaunches.New(app.db, ilgroup, init.UserSuffix)

	return app
}

// Greeting lets the caller know that the service is up and should be receiving
// requests.
func (e *ExposerApp) Greeting(context echo.Context) error {
	context.String(http.StatusOK, "Hello from app-exposer.")
	return nil
}
