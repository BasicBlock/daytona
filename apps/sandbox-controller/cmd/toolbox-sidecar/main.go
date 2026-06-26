package main

import (
	"flag"
	"net/http"
	"os"
	"time"

	"github.com/daytonaio/sandbox-controller/internal/toolbox"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func main() {
	var listenAddr string
	flag.StringVar(&listenAddr, "listen", ":2280", "HTTP listen address.")
	opts := zap.Options{Development: true}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	server := &http.Server{
		Addr:              listenAddr,
		Handler:           toolbox.New(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	ctrl.Log.Info("starting toolbox sidecar", "listen", listenAddr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		ctrl.Log.Error(err, "toolbox sidecar exited")
		os.Exit(1)
	}
}
