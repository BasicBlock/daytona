package observability

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	SandboxPhase = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "daytona_sandbox_phase",
		Help: "Sandbox phase by namespace, name, and phase.",
	}, []string{"namespace", "name", "phase"})
	SnapshotPhase = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "daytona_sandbox_snapshot_phase",
		Help: "SandboxSnapshot phase by namespace, name, and phase.",
	}, []string{"namespace", "name", "phase"})
	ReconcileErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "daytona_sandbox_reconcile_errors_total",
		Help: "Controller reconcile errors by controller.",
	}, []string{"controller"})
	ReconcileDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "daytona_sandbox_reconcile_duration_seconds",
		Help:    "Controller reconcile duration by controller.",
		Buckets: prometheus.DefBuckets,
	}, []string{"controller"})
)

func init() {
	metrics.Registry.MustRegister(SandboxPhase, SnapshotPhase, ReconcileErrors, ReconcileDuration)
}

func ObserveReconcile(controller string, started time.Time, err error) {
	ReconcileDuration.WithLabelValues(controller).Observe(time.Since(started).Seconds())
	if err != nil {
		ReconcileErrors.WithLabelValues(controller).Inc()
	}
}
