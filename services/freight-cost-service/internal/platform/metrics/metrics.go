package metrics

import (
	"net/http"
	"strconv"
	"sync"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	httpRequestsTotal        *prometheus.CounterVec
	sourceRequestsTotal      *prometheus.CounterVec
	sourceErrorsTotal        *prometheus.CounterVec
	currencyMismatchTotal    *prometheus.CounterVec
	eventsAppliedTotal       *prometheus.CounterVec
	projectionUpdatesTotal   *prometheus.CounterVec
	outOfOrderTotal          *prometheus.CounterVec
	rebuildTotal             *prometheus.CounterVec
}

var (
	registerOnce sync.Once
	shared       *Metrics
)

func New() *Metrics {
	registerOnce.Do(func() {
		shared = &Metrics{
			httpRequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
				Name: "freight_cost_http_requests_total",
				Help: "Freight cost HTTP requests by method, path, and status",
			}, []string{"method", "path", "status"}),
			sourceRequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
				Name: "freight_cost_source_requests_total",
				Help: "Downstream source requests by service, operation, and result",
			}, []string{"source_service", "operation", "result"}),
			sourceErrorsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
				Name: "freight_cost_source_errors_total",
				Help: "Downstream source errors by service and error code",
			}, []string{"source_service", "error_code"}),
			currencyMismatchTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
				Name: "freight_cost_currency_mismatch_total",
				Help: "Currency mismatch validation failures by operation",
			}, []string{"operation"}),
			eventsAppliedTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
				Name: "freight_cost_events_applied_total",
				Help: "Freight cost ingest outcomes by entry kind and result",
			}, []string{"entry_kind", "result"}),
			projectionUpdatesTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
				Name: "freight_cost_projection_updates_total",
				Help: "Freight cost projection updates by entry kind",
			}, []string{"entry_kind"}),
			outOfOrderTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
				Name: "freight_cost_events_out_of_order_total",
				Help: "Freight cost out-of-order journal events by entry kind",
			}, []string{"entry_kind"}),
			rebuildTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
				Name: "freight_cost_rebuild_total",
				Help: "Freight cost rebuild operations by result",
			}, []string{"result"}),
		}
		prometheus.MustRegister(
			shared.httpRequestsTotal,
			shared.sourceRequestsTotal,
			shared.sourceErrorsTotal,
			shared.currencyMismatchTotal,
			shared.eventsAppliedTotal,
			shared.projectionUpdatesTotal,
			shared.outOfOrderTotal,
			shared.rebuildTotal,
		)
	})
	return shared
}

func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		status := strconv.Itoa(ww.Status())
		if status == "0" {
			status = "200"
		}
		m.httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, status).Inc()
	})
}

func (m *Metrics) ObserveSourceRequest(sourceService, operation, result string) {
	m.sourceRequestsTotal.WithLabelValues(sourceService, operation, result).Inc()
}

func (m *Metrics) ObserveSourceError(sourceService, errorCode string) {
	m.sourceErrorsTotal.WithLabelValues(sourceService, errorCode).Inc()
}

func (m *Metrics) ObserveCurrencyMismatch(operation string) {
	m.currencyMismatchTotal.WithLabelValues(operation).Inc()
}

func (m *Metrics) ObserveEventApplied(entryKind, result string) {
	m.eventsAppliedTotal.WithLabelValues(entryKind, result).Inc()
}

func (m *Metrics) ObserveProjectionUpdate(entryKind string) {
	m.projectionUpdatesTotal.WithLabelValues(entryKind).Inc()
}

func (m *Metrics) ObserveOutOfOrder(entryKind string) {
	m.outOfOrderTotal.WithLabelValues(entryKind).Inc()
}

func (m *Metrics) ObserveRebuild(result string) {
	m.rebuildTotal.WithLabelValues(result).Inc()
}
