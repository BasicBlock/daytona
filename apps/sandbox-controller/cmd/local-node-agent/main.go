package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"time"

	computev1 "github.com/daytonaio/sandbox-controller/api/v1alpha1"
	"github.com/daytonaio/sandbox-controller/internal/localrunsc"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

func main() {
	var nodeName string
	var httpListen string
	var metricsAddr string
	var probeAddr string
	var runscPath string
	var runscRoot string
	var artifactRoot string

	flag.StringVar(&nodeName, "node-name", os.Getenv("NODE_NAME"), "Kubernetes node name handled by this agent.")
	flag.StringVar(&httpListen, "http-listen", ":2281", "HTTP listen address for raw local runsc smoke endpoints.")
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.StringVar(&runscPath, "runsc-path", localrunsc.DefaultRunscPath, "Path to the runsc binary.")
	flag.StringVar(&runscRoot, "runsc-root", "", "Path to the runsc root directory that contains runtime state.")
	flag.StringVar(&artifactRoot, "artifact-root", localrunsc.DefaultArtifactRoot, "Filesystem root for local checkpoint artifacts.")
	opts := zap.Options{Development: true}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	if nodeName == "" {
		ctrl.Log.Error(nil, "node name is required")
		os.Exit(1)
	}

	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(computev1.AddToScheme(scheme))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         false,
	})
	if err != nil {
		ctrl.Log.Error(err, "unable to start manager")
		os.Exit(1)
	}

	runtime := localrunsc.NewRuntime(runscPath, artifactRoot, localrunsc.ExecRunner{})
	runtime.RunscRoot = runscRoot
	if err := (&localrunsc.SnapshotReconciler{
		Client:   mgr.GetClient(),
		Runtime:  runtime,
		NodeName: nodeName,
	}).SetupWithManager(mgr); err != nil {
		ctrl.Log.Error(err, "unable to create LocalRunscSnapshot controller")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		ctrl.Log.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		ctrl.Log.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	server := &http.Server{
		Addr:              httpListen,
		Handler:           localrunsc.NewHTTPServer(runtime),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		ctrl.Log.Info("starting local runsc HTTP server", "listen", httpListen)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			ctrl.Log.Error(err, "local runsc HTTP server exited")
			os.Exit(1)
		}
	}()

	ctx := ctrl.SetupSignalHandler()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	ctrl.Log.Info("starting local runsc node agent", "node", nodeName)
	if err := mgr.Start(ctx); err != nil {
		ctrl.Log.Error(err, "manager exited")
		os.Exit(1)
	}
}
