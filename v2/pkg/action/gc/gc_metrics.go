package gc

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

//nolint:gochecknoglobals // Metrics are process-wide Prometheus collectors.
var (
	GCyclesTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "action_gc_cycles_total",
		Help: "Number of GC cycles.",
	})
	GDeletedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "action_gc_deleted_total",
		Help: "Number of resources deleted by GC.",
	})
	registerMetricsOnce sync.Once
)

// RegisterMetrics registers GC metrics with controller-runtime's registry.
func RegisterMetrics() {
	registerMetricsOnce.Do(func() {
		metrics.Registry.MustRegister(GCyclesTotal, GDeletedTotal)
	})
}
