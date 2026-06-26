package main

import (
	"flag"
	"os"
	"time"

	computev1 "github.com/daytonaio/sandbox-controller/api/v1alpha1"
	"github.com/daytonaio/sandbox-controller/internal/controller"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

func main() {
	var metricsAddr string
	var probeAddr string
	var leaderElection bool
	var defaultToolboxImage string
	var enableLocalPodSnapshotShim bool
	var localStorageEndpoint string
	var localStorageBucket string
	var localStoragePrefix string
	var localStoragePath string
	var staleSandboxDeleteTimeout time.Duration
	var staleSnapshotTimeout time.Duration

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&leaderElection, "leader-elect", false, "Enable leader election for controller manager.")
	flag.StringVar(&defaultToolboxImage, "default-toolbox-image", "", "Default toolbox sidecar image.")
	flag.BoolVar(&enableLocalPodSnapshotShim, "enable-local-podsnapshot-shim", false, "Enable the local k3s PodSnapshot shim backed by LocalRunscSnapshot.")
	flag.StringVar(&localStorageEndpoint, "local-podsnapshot-storage-endpoint", "", "S3-compatible endpoint used by the local PodSnapshot shim.")
	flag.StringVar(&localStorageBucket, "local-podsnapshot-storage-bucket", "", "S3 bucket used by the local PodSnapshot shim.")
	flag.StringVar(&localStoragePrefix, "local-podsnapshot-storage-prefix", "", "S3 prefix used by the local PodSnapshot shim.")
	flag.StringVar(&localStoragePath, "local-podsnapshot-storage-path", "", "Node-local artifact path used by the local PodSnapshot shim.")
	flag.DurationVar(&staleSandboxDeleteTimeout, "stale-sandbox-delete-timeout", 10*time.Minute, "Best-effort cleanup timeout before releasing a deleting Sandbox finalizer.")
	flag.DurationVar(&staleSnapshotTimeout, "stale-snapshot-timeout", 30*time.Minute, "Timeout before failing a pending or triggering SandboxSnapshot and cleaning provider artifacts.")
	opts := zap.Options{Development: true}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(networkingv1.AddToScheme(scheme))
	utilruntime.Must(computev1.AddToScheme(scheme))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         leaderElection,
		LeaderElectionID:       "sandbox-controller.compute.daytona.io",
	})
	if err != nil {
		ctrl.Log.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err := (&controller.SandboxReconciler{
		Client:              mgr.GetClient(),
		Scheme:              mgr.GetScheme(),
		DefaultToolboxImage: defaultToolboxImage,
		Recorder:            mgr.GetEventRecorderFor("sandbox-controller"),
		StaleDeleteTimeout:  staleSandboxDeleteTimeout,
	}).SetupWithManager(mgr); err != nil {
		ctrl.Log.Error(err, "unable to create Sandbox controller")
		os.Exit(1)
	}

	if err := (&controller.SandboxSnapshotReconciler{
		Client:       mgr.GetClient(),
		Scheme:       mgr.GetScheme(),
		Recorder:     mgr.GetEventRecorderFor("sandbox-snapshot-controller"),
		StaleTimeout: staleSnapshotTimeout,
	}).SetupWithManager(mgr); err != nil {
		ctrl.Log.Error(err, "unable to create SandboxSnapshot controller")
		os.Exit(1)
	}

	if err := (&controller.SandboxTemplateReconciler{
		Client: mgr.GetClient(),
	}).SetupWithManager(mgr); err != nil {
		ctrl.Log.Error(err, "unable to create SandboxTemplate controller")
		os.Exit(1)
	}

	if enableLocalPodSnapshotShim {
		if err := (&controller.LocalPodSnapshotShimReconciler{
			Client: mgr.GetClient(),
			Scheme: mgr.GetScheme(),
			DefaultStorage: computev1.LocalRunscStorageSpec{
				Mode:     "s3",
				Endpoint: localStorageEndpoint,
				Bucket:   localStorageBucket,
				Prefix:   localStoragePrefix,
				Path:     localStoragePath,
			},
		}).SetupWithManager(mgr); err != nil {
			ctrl.Log.Error(err, "unable to create local PodSnapshot shim controller")
			os.Exit(1)
		}
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		ctrl.Log.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		ctrl.Log.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	ctrl.Log.Info("starting sandbox controller")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		ctrl.Log.Error(err, "manager exited")
		os.Exit(1)
	}
}
