package main

import (
	"flag"
	"net/http"
	"os"

	computev1 "github.com/daytonaio/sandbox-controller/api/v1alpha1"
	"github.com/daytonaio/sandbox-controller/internal/apiserver"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func main() {
	var listenAddr string
	var namespace string
	var publicBaseURL string
	flag.StringVar(&listenAddr, "listen", ":8090", "HTTP listen address.")
	flag.StringVar(&namespace, "namespace", "sandboxes", "Sandbox namespace.")
	flag.StringVar(&publicBaseURL, "public-base-url", "", "Tailnet-reachable public base URL returned for sandbox access.")
	opts := zap.Options{Development: true}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(networkingv1.AddToScheme(scheme))
	utilruntime.Must(computev1.AddToScheme(scheme))

	config := ctrl.GetConfigOrDie()
	k8sClient, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		ctrl.Log.Error(err, "unable to create Kubernetes client")
		os.Exit(1)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		ctrl.Log.Error(err, "unable to create Kubernetes clientset")
		os.Exit(1)
	}

	ctrl.Log.Info("starting sandbox API", "listen", listenAddr, "namespace", namespace)
	options := []apiserver.Option{apiserver.WithPodLogStreamer(apiserver.NewKubernetesPodLogStreamer(clientset))}
	if publicBaseURL != "" {
		options = append(options, apiserver.WithPublicBaseURL(publicBaseURL))
	}
	if err := http.ListenAndServe(listenAddr, apiserver.New(k8sClient, namespace, options...)); err != nil {
		ctrl.Log.Error(err, "sandbox API exited")
		os.Exit(1)
	}
}
