package main

import (
	"flag"
	"os"

	"github.com/spf13/pflag"

	"github.com/onrik/logrus/filename"
	"github.com/sirupsen/logrus"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/apis"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/cmd/options"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/controllers/persist"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/clientmgr"
	backendsregistry "github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/registry"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/metrics"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/util/signals"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	// Add filename as one of the fields of the structured log message.
	filenameHook := filename.NewHook()
	filenameHook.Field = "filename"
	logrus.AddHook(filenameHook)
}

func main() {
	var (
		ctrlMetricsAddr      string
		metricsAddr          int
		enableLeaderElection bool
		verbosity            string
	)
	pflag.StringVar(&ctrlMetricsAddr, "controller-metrics-addr", ":8080", "The address the controller metric endpoint binds to.")
	pflag.IntVar(&metricsAddr, "metrics-addr", 8443, "The address the default endpoints binds to.")
	pflag.BoolVar(&enableLeaderElection, "enable-leader-election", true,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	pflag.IntVar(&options.CtrlConfig.MaxConcurrentReconciles, "max-reconciles", 3, "specify the number of max concurrent reconciles of each controller")
	pflag.StringVar(&verbosity, "verbosity", "3", "number for the log level verbosity")

	pflag.Parse()

	if options.CtrlConfig.MaxConcurrentReconciles <= 0 {
		options.CtrlConfig.MaxConcurrentReconciles = 1
	}

	ctrl.SetLogger(zap.Logger(false))

	// init klog
	klogFlags := flag.NewFlagSet("klog", flag.ExitOnError)
	klog.InitFlags(klogFlags)
	klogFlags.Set("v", verbosity)

	klog.Info("klog verbosity :", verbosity)
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: ctrlMetricsAddr,
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   "kubedl-leader-election",
		Port:               9443,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	setupLog.Info("setting up scheme")
	if err := apis.AddToScheme(mgr.GetScheme()); err != nil {
		setupLog.Error(err, "unable to add APIs to scheme")
		os.Exit(1)
	}

	stopCh := signals.GlobalSetupSignal()

	// Setup persist controllers if backends are specified.
	setupLog.Info("setup persist controllers")
	clientmgr.InitFromManager(mgr)
	backendsregistry.RegisterStorageBackends()
	if err = persist.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to setup persist controllers")
		os.Exit(1)
	}

	// Start monitoring for default registry.
	metrics.StartMonitoringForDefaultRegistry(metricsAddr)

	setupLog.Info("starting manager")
	if err := mgr.Start(stopCh); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
