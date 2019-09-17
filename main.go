package main

import (
	"database/sql"
	"flag"
	"fmt"
	_ "github.com/lib/pq"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"

	"github.com/cyverse-de/configurate"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var log = logrus.WithFields(logrus.Fields{
	"service": "app-exposer",
	"art-id":  "app-exposer",
	"group":   "org.cyverse",
})

func main() {
	logrus.SetReportCaller(true)

	var (
		err        error
		kubeconfig *string
		cfg        *viper.Viper
		db         *sql.DB

		configPath                    = flag.String("config", "/etc/iplant/de/jobservices.yml", "Path to the config file")
		namespace                     = flag.String("namespace", "default", "The namespace scope this process operates on for non-VICE calls")
		viceNamespace                 = flag.String("vice-namespace", "vice-apps", "The namepsace that VICE apps are launched within")
		listenPort                    = flag.Int("port", 60000, "(optional) The port to listen on")
		ingressClass                  = flag.String("ingress-class", "nginx", "(optional) the ingress class to use")
		viceProxy                     = flag.String("vice-proxy", "discoenv/vice-proxy", "The image name of the proxy to use for VICE apps. The image tag is set in the config.")
		viceDefaultBackendService     = flag.String("vice-default-backend", "vice-default-backend", "The name of the service to use as the default backend for VICE ingresses")
		viceDefaultBackendServicePort = flag.Int("vice-default-backend-port", 80, "The port for the default backend for VICE ingresses")
		getAnalysisIDService          = flag.String("--get-analysis-id-service", "get-analysis-id", "The service name for the service that provides analysis ID lookups")
		checkResourceAccessService    = flag.String("--check-resource-access-service", "check-resource-access", "The name of the service that validates whether a user can access a resource")
	)

	// if cluster is set, then
	if cluster := os.Getenv("CLUSTER"); cluster != "" {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	} else {
		// If the home directory exists, then assume that the kube config will be read
		// from ~/.kube/config.
		if home := os.Getenv("HOME"); home != "" {
			kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
		} else {
			// If the home directory doesn't exist, then allow the user to specify a path.
			kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
		}
	}

	flag.Parse()

	fmt.Printf("Reading config from %s\n", *configPath)
	if _, err = os.Open(*configPath); err != nil {
		log.Fatal(*configPath)
	}

	cfg, err = configurate.Init(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Done reading config from %s\n", *configPath)

	// Make sure the db.uri URL is parseable
	if _, err = url.Parse(cfg.GetString("db.uri")); err != nil {
		log.Fatal(errors.Wrap(err, "Can't parse db.uri in the config file"))
	}

	// Make sure the frontend base URL is parseable.
	if _, err = url.Parse(cfg.GetString("k8s.frontend.base")); err != nil {
		log.Fatal(errors.Wrap(err, "Can't parse k8s.frontend.base in the config file"))
	}

	// Print error and exit if *kubeconfig is not empty and doesn't actually
	// exist. If *kubeconfig is blank, then the app may be running inside the
	// cluster, so let things proceed.
	if *kubeconfig != "" {
		_, err = os.Stat(*kubeconfig)
		if err != nil {
			if os.IsNotExist(err) {
				log.Fatalf("config %s does not exist", *kubeconfig)
			}
			log.Fatal(errors.Wrapf(err, "error stat'ing the kubeconfig %s", *kubeconfig))
		}
	}

	log.Printf("namespace is set to %s\n", *namespace)
	log.Printf("listen port is set to %d\n", *listenPort)
	log.Printf("kubeconfig is set to '%s', and may be blank", *kubeconfig)

	var config *rest.Config
	if *kubeconfig != "" {
		config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
		if err != nil {
			log.Fatal(errors.Wrapf(err, "error building config from flags using kubeconfig %s", *kubeconfig))
		}
	} else {
		// If the home directory doesn't exist and the user doesn't specify a path,
		// then assume that we're running inside a cluster.
		config, err = rest.InClusterConfig()
		if err != nil {
			log.Fatal(errors.Wrapf(err, "error loading the config inside the cluster"))
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal(errors.Wrap(err, "error creating clientset from config"))
	}

	jobStatusURL := cfg.GetString("vice.job-status.base")
	if jobStatusURL == "" {
		jobStatusURL = "http://job-status-listener"
	}

	appsServiceBaseURL := cfg.GetString("apps.base")
	if appsServiceBaseURL == "" {
		appsServiceBaseURL = "http://apps"
	}

	// Create the JSLPublisher for job status updates
	jsl := &JSLPublisher{
		statusURL: jobStatusURL,
	}

	var proxyImage string
	proxyTag := cfg.GetString("interapps.proxy.tag")
	if proxyTag == "" {
		proxyImage = *viceProxy
	} else {
		proxyImage = fmt.Sprintf("%s:%s", *viceProxy, proxyTag)
	}

	dbURI := cfg.GetString("db.uri")
	// connect to the database. or not, I don't really care.
	db, err = sql.Open("postgres", dbURI)
	if err != nil {
		log.Fatal(errors.Wrapf(err, "error connecting to database %s", dbURI))
	}

	if err = db.Ping(); err != nil {
		log.Fatal(errors.Wrapf(err, "error pinging database %s", dbURI))
	}

	exposerInit := &ExposerAppInit{
		Namespace:                     *namespace,
		ViceNamespace:                 *viceNamespace,
		PorklockImage:                 cfg.GetString("vice.file-transfers.image"),
		PorklockTag:                   cfg.GetString("vice.file-transfers.tag"),
		InputPathListIdentifier:       cfg.GetString("path_list.file_identifier"),
		TicketInputPathListIdentifier: cfg.GetString("tickets_path_list.file_identifier"),
		statusPublisher:               jsl,
		ViceProxyImage:                proxyImage,
		CASBaseURL:                    cfg.GetString("cas.base"),
		FrontendBaseURL:               cfg.GetString("k8s.frontend.base"),
		IngressBaseURL:                cfg.GetString("k8s.app-exposer.base"),
		AnalysisHeader:                cfg.GetString("k8s.get-analysis-id.header"),
		AccessHeader:                  cfg.GetString("k8s.check-resource-access.header"),
		ViceDefaultBackendService:     *viceDefaultBackendService,
		ViceDefaultBackendServicePort: *viceDefaultBackendServicePort,
		GetAnalysisIDService:          *getAnalysisIDService,
		CheckResourceAccessService:    *checkResourceAccessService,
		VICEBackendNamespace:          cfg.GetString("vice.backend-namespace"),
		AppsServiceBaseURL:            appsServiceBaseURL,
		db:                            db,
	}

	app := NewExposerApp(exposerInit, *ingressClass, clientset)
	log.Printf("listening on port %d", *listenPort)
	app.MonitorVICEEvents()
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", strconv.Itoa(*listenPort)), app.router))
}
