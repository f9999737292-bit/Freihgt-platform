package metrics

import (
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	dbQueryDuration *prometheus.HistogramVec
	dbMetricsOnce   sync.Once

	slowQueryConfigOnce sync.Once
	slowQueryThresholdMs int64 = 500
)

func initDBMetrics() {
	dbMetricsOnce.Do(func() {
		dbQueryDuration = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: "db_query_duration_seconds",
				Help: "Database query duration in seconds",
				Buckets: []float64{
					0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5,
				},
			},
			[]string{"service", "repository", "operation", "status"},
		)
		prometheus.MustRegister(dbQueryDuration)
	})
}

func initSlowQueryConfig() {
	slowQueryConfigOnce.Do(func() {
		slowQueryThresholdMs = 500
		raw := os.Getenv("DB_SLOW_QUERY_THRESHOLD_MS")
		if raw == "" {
			return
		}
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value < 0 {
			return
		}
		slowQueryThresholdMs = value
	})
}

// SlowQueryThresholdMs returns the configured slow query threshold in milliseconds (0 = disabled).
func SlowQueryThresholdMs() int64 {
	initSlowQueryConfig()
	return slowQueryThresholdMs
}

// ObserveDBQuery records database query duration and logs slow queries.
func ObserveDBQuery(service, repository, operation, status string, duration time.Duration) {
	initDBMetrics()
	initSlowQueryConfig()
	dbQueryDuration.WithLabelValues(service, repository, operation, status).Observe(duration.Seconds())

	thresholdMs := slowQueryThresholdMs
	if thresholdMs <= 0 {
		return
	}
	if duration.Milliseconds() < thresholdMs {
		return
	}

	slog.Warn("slow database query",
		slog.String("level", "warn"),
		slog.String("service", service),
		slog.String("repository", repository),
		slog.String("operation", operation),
		slog.Int64("duration_ms", duration.Milliseconds()),
		slog.Int64("threshold_ms", thresholdMs),
		slog.String("message", "slow database query"),
	)
}

// MeasureDBQuery executes fn and records its duration as a DB query metric.
func MeasureDBQuery(service, repository, operation string, fn func() error) error {
	start := time.Now()
	err := fn()
	status := "success"
	if err != nil {
		status = "error"
	}
	ObserveDBQuery(service, repository, operation, status, time.Since(start))
	return err
}

// Collector exposes Prometheus HTTP metrics for a service.
type Collector struct {
	serviceName string

	requestsTotal   *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
	inFlight        prometheus.Gauge
}

var collectors sync.Map

// New creates and registers Prometheus metrics for the given service.
func New(serviceName string) *Collector {
	if existing, ok := collectors.Load(serviceName); ok {
		return existing.(*Collector)
	}

	c := &Collector{
		serviceName: serviceName,
		requestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"service", "method", "path", "status"},
		),
		requestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "HTTP request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"service", "method", "path"},
		),
		inFlight: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name:        "http_in_flight_requests",
				Help:        "Number of in-flight HTTP requests",
				ConstLabels: prometheus.Labels{"service": serviceName},
			},
		),
	}

	prometheus.MustRegister(c.requestsTotal, c.requestDuration, c.inFlight)
	collectors.Store(serviceName, c)
	return c
}

// Middleware records HTTP request metrics.
func (c *Collector) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.inFlight.Inc()
		defer c.inFlight.Dec()

		start := time.Now()
		ww := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)

		status := strconv.Itoa(ww.Status())
		path := r.URL.Path
		c.requestsTotal.WithLabelValues(c.serviceName, r.Method, path, status).Inc()
		c.requestDuration.WithLabelValues(c.serviceName, r.Method, path).Observe(time.Since(start).Seconds())
	})
}

// Handler returns the Prometheus /metrics endpoint handler.
func (c *Collector) Handler() http.Handler {
	return promhttp.Handler()
}
