package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	requests     *prometheus.CounterVec
	duration     *prometheus.HistogramVec
	sourceErrors *prometheus.CounterVec
	handler      http.Handler
}

func New() *Metrics {
	registry := prometheus.NewRegistry()
	requests := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "analytics_requests_total",
		Help: "Analytics HTTP requests. Labels stay low cardinality.",
	}, []string{"operation", "result", "kpi_id"})
	duration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "analytics_request_duration_seconds",
		Help:    "Analytics HTTP request duration in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"operation", "result", "kpi_id"})
	sourceErrors := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "analytics_source_errors_total",
		Help: "Shipment source contract failures. No tenant or business identifiers.",
	}, []string{"reason"})
	registry.MustRegister(requests, duration, sourceErrors)
	return &Metrics{
		requests:     requests,
		duration:     duration,
		sourceErrors: sourceErrors,
		handler:      promhttp.HandlerFor(registry, promhttp.HandlerOpts{}),
	}
}

func (m *Metrics) Handler() http.Handler {
	if m == nil || m.handler == nil {
		return http.NotFoundHandler()
	}
	return m.handler
}

func (m *Metrics) Observe(operation, result, kpiID string, started time.Time) {
	if m == nil {
		return
	}
	kpiID = boundedKPI(kpiID)
	elapsed := time.Since(started).Seconds()
	m.requests.WithLabelValues(operation, result, kpiID).Inc()
	m.duration.WithLabelValues(operation, result, kpiID).Observe(elapsed)
}

func (m *Metrics) SourceError(reason string) {
	if m == nil {
		return
	}
	switch reason {
	case "unreachable", "timeout", "http_5xx", "http_status", "malformed", "wrong_tenant", "inconsistent":
	default:
		reason = "other"
	}
	m.sourceErrors.WithLabelValues(reason).Inc()
}

func boundedKPI(id string) string {
	switch id {
	case "OPS_SHIPMENTS_TOTAL", "OPS_ON_TIME_DELIVERY", "OPS_ON_TIME_DELIVERY_RATE", "OPS_RETURN_CASES", "OPS_REDIRECT_CASES",
		"OPS_ON_TIME_PICKUP", "OPS_ON_TIME_PICKUP_RATE", "OPS_LATE_PICKUP", "OPS_LATE_DELIVERY",
		"CAR_ON_TIME_PICKUP_RATE", "CAR_ON_TIME_DELIVERY_RATE":
		return id
	case "":
		return "none"
	default:
		return "unsupported"
	}
}

func StatusResult(status int) string {
	switch status {
	case http.StatusOK:
		return "ok"
	case http.StatusBadRequest:
		return "validation"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusServiceUnavailable:
		return "unavailable"
	default:
		return "status_" + strconv.Itoa(status)
	}
}
