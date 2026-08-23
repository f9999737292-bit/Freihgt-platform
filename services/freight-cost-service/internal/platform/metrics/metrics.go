package metrics

import (
	"net/http"
	"strconv"
	"sync"
	"time"

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
	varianceRecomputedTotal  *prometheus.CounterVec
	forecastRecomputedTotal  *prometheus.CounterVec
	forecastProposedUnknown  prometheus.Counter
	reconciliationFindingTotal *prometheus.CounterVec
	chargeCodeUnmappedTotal  prometheus.Counter
	analyticsRebuildTotal    *prometheus.CounterVec
	analyticsRebuildDuration prometheus.Histogram
	analyticsIncrementalTotal *prometheus.CounterVec
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
			varianceRecomputedTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
				Name: "freight_cost_variance_recomputed_total",
				Help: "Freight cost variance recomputations by result",
			}, []string{"result"}),
			forecastRecomputedTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
				Name: "freight_cost_forecast_recomputed_total",
				Help: "Freight cost forecast recomputations by result",
			}, []string{"result"}),
			forecastProposedUnknown: prometheus.NewCounter(prometheus.CounterOpts{
				Name: "freight_cost_forecast_proposed_source_unknown_total",
				Help: "Forecast recompute with unknown proposed accessorial source",
			}),
			reconciliationFindingTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
				Name: "freight_cost_reconciliation_finding_total",
				Help: "Reconciliation findings by kind and severity",
			}, []string{"kind", "severity"}),
			chargeCodeUnmappedTotal: prometheus.NewCounter(prometheus.CounterOpts{
				Name: "freight_cost_charge_code_unmapped_total",
				Help: "Charge codes mapped to OTHER category",
			}),
			analyticsRebuildTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
				Name: "freight_cost_analytics_rebuild_total",
				Help: "Analytics projection rebuild operations by result",
			}, []string{"result"}),
			analyticsRebuildDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
				Name:    "freight_cost_analytics_rebuild_duration_seconds",
				Help:    "Analytics projection tenant rebuild duration",
				Buckets: prometheus.DefBuckets,
			}),
			analyticsIncrementalTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
				Name: "freight_cost_analytics_incremental_total",
				Help: "Analytics incremental dirty processing by result",
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
			shared.varianceRecomputedTotal,
			shared.forecastRecomputedTotal,
			shared.forecastProposedUnknown,
			shared.reconciliationFindingTotal,
			shared.chargeCodeUnmappedTotal,
			shared.analyticsRebuildTotal,
			shared.analyticsRebuildDuration,
			shared.analyticsIncrementalTotal,
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

func (m *Metrics) ObserveVarianceRecomputed(result string) {
	m.varianceRecomputedTotal.WithLabelValues(result).Inc()
}

func (m *Metrics) ObserveForecastRecomputed(result string) {
	m.forecastRecomputedTotal.WithLabelValues(result).Inc()
}

func (m *Metrics) ObserveForecastProposedSourceUnknown() {
	m.forecastProposedUnknown.Inc()
}

func (m *Metrics) ObserveReconciliationFinding(kind, severity string) {
	m.reconciliationFindingTotal.WithLabelValues(kind, severity).Inc()
}

func (m *Metrics) ObserveChargeCodeUnmapped() {
	m.chargeCodeUnmappedTotal.Inc()
}

func (m *Metrics) ObserveAnalyticsRebuild(result string) {
	m.analyticsRebuildTotal.WithLabelValues(result).Inc()
}

func (m *Metrics) ObserveAnalyticsRebuildDuration(d time.Duration) {
	m.analyticsRebuildDuration.Observe(d.Seconds())
}

func (m *Metrics) ObserveAnalyticsIncremental(result string) {
	m.analyticsIncrementalTotal.WithLabelValues(result).Inc()
}
