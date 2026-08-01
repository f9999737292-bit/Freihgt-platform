package outbox

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type StatusSummaryMetrics struct {
	requestsTotal *prometheus.CounterVec
	queryDuration *prometheus.HistogramVec
	errorsTotal   *prometheus.CounterVec
}

var statusSummaryRegisterOnce sync.Once

func NewStatusSummaryMetrics() *StatusSummaryMetrics {
	m := &StatusSummaryMetrics{
		requestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "shipment_status_summary_requests_total",
			Help: "Shipment status summary internal endpoint requests.",
		}, []string{"result", "error_code"}),
		queryDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "shipment_status_summary_query_duration_seconds",
			Help:    "Shipment status summary aggregate query duration.",
			Buckets: prometheus.DefBuckets,
		}, []string{"result", "error_code"}),
		errorsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "shipment_status_summary_errors_total",
			Help: "Shipment status summary handler errors.",
		}, []string{"result", "error_code"}),
	}
	statusSummaryRegisterOnce.Do(func() {
		prometheus.MustRegister(m.requestsTotal, m.queryDuration, m.errorsTotal)
	})
	return m
}

func (m *StatusSummaryMetrics) Observe(result, errorCode string, duration time.Duration) {
	if errorCode == "" {
		errorCode = "NONE"
	}
	m.requestsTotal.WithLabelValues(result, errorCode).Inc()
	m.queryDuration.WithLabelValues(result, errorCode).Observe(duration.Seconds())
	if result == "error" {
		m.errorsTotal.WithLabelValues(result, errorCode).Inc()
	}
}
