package metrics

import (
	"net/http"
	"strconv"
	"sync"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	httpRequestsTotal     *prometheus.CounterVec
	sourceRequestsTotal   *prometheus.CounterVec
	sourceErrorsTotal     *prometheus.CounterVec
	currencyMismatchTotal *prometheus.CounterVec
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
		}
		prometheus.MustRegister(
			shared.httpRequestsTotal,
			shared.sourceRequestsTotal,
			shared.sourceErrorsTotal,
			shared.currencyMismatchTotal,
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
