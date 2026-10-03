package storage

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	metricsOnce sync.Once

	putTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "object_storage_put_total",
		Help: "Object storage put attempts.",
	})
	getTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "object_storage_get_total",
		Help: "Object storage get attempts.",
	})
	errorsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "object_storage_errors_total",
		Help: "Object storage operation errors.",
	})
	latencySeconds = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "object_storage_latency_seconds",
		Help:    "Object storage operation latency.",
		Buckets: prometheus.DefBuckets,
	})
	integrityFailures = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "integrity_check_failures_total",
		Help: "Stored object digest mismatches.",
	})
)

func ensureMetrics() {
	metricsOnce.Do(func() {
		prometheus.MustRegister(putTotal, getTotal, errorsTotal, latencySeconds, integrityFailures)
	})
}

func observe(operation string, started time.Time, err error) {
	ensureMetrics()
	latencySeconds.Observe(time.Since(started).Seconds())
	switch operation {
	case "put":
		putTotal.Inc()
	case "get":
		getTotal.Inc()
	}
	if err != nil {
		errorsTotal.Inc()
	}
}

// RecordIntegrityFailure counts a digest mismatch. It has no tenant, key, or filename labels.
func RecordIntegrityFailure() {
	ensureMetrics()
	integrityFailures.Inc()
}
