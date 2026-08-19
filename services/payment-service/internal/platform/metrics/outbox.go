package metrics

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	outboxMetricsOnce sync.Once

	outboxClaimedTotal          *prometheus.CounterVec
	outboxPublishedTotal        *prometheus.CounterVec
	outboxPublishFailedTotal    *prometheus.CounterVec
	outboxMarkedFailedTotal     *prometheus.CounterVec
	outboxPublishDuration       *prometheus.HistogramVec
	outboxPendingCount          prometheus.Gauge
	outboxFailedCount           prometheus.Gauge
	outboxOldestPendingAgeGauge prometheus.Gauge
)

func initOutboxMetrics() {
	outboxMetricsOnce.Do(func() {
		outboxClaimedTotal = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "payment_outbox_claimed_total",
				Help: "Total payment outbox events claimed for publishing",
			},
			[]string{"event_type"},
		)
		outboxPublishedTotal = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "payment_outbox_published_total",
				Help: "Total payment outbox events published successfully",
			},
			[]string{"event_type", "result"},
		)
		outboxPublishFailedTotal = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "payment_outbox_publish_failed_total",
				Help: "Total payment outbox publish attempts that failed and will retry",
			},
			[]string{"event_type", "error_code"},
		)
		outboxMarkedFailedTotal = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "payment_outbox_marked_failed_total",
				Help: "Total payment outbox events marked permanently failed",
			},
			[]string{"event_type", "error_code"},
		)
		outboxPublishDuration = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "payment_outbox_publish_duration_seconds",
				Help:    "Payment outbox publish duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"event_type", "result"},
		)
		outboxPendingCount = prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "payment_outbox_pending_count",
			Help: "Current count of pending payment outbox events",
		})
		outboxFailedCount = prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "payment_outbox_failed_count",
			Help: "Current count of permanently failed payment outbox events",
		})
		outboxOldestPendingAgeGauge = prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "payment_outbox_oldest_pending_age_seconds",
			Help: "Age in seconds of the oldest pending payment outbox event",
		})
		prometheus.MustRegister(
			outboxClaimedTotal,
			outboxPublishedTotal,
			outboxPublishFailedTotal,
			outboxMarkedFailedTotal,
			outboxPublishDuration,
			outboxPendingCount,
			outboxFailedCount,
			outboxOldestPendingAgeGauge,
		)
	})
}

func ObserveOutboxClaimed(eventType string) {
	initOutboxMetrics()
	outboxClaimedTotal.WithLabelValues(eventType).Inc()
}

func ObserveOutboxPublished(eventType, result string, duration time.Duration) {
	initOutboxMetrics()
	outboxPublishedTotal.WithLabelValues(eventType, result).Inc()
	outboxPublishDuration.WithLabelValues(eventType, result).Observe(duration.Seconds())
}

func ObserveOutboxPublishFailed(eventType, errorCode string) {
	initOutboxMetrics()
	outboxPublishFailedTotal.WithLabelValues(eventType, errorCode).Inc()
}

func ObserveOutboxMarkedFailed(eventType, errorCode string) {
	initOutboxMetrics()
	outboxMarkedFailedTotal.WithLabelValues(eventType, errorCode).Inc()
}

func SetOutboxGaugeSnapshot(pending, failed int64, oldestPendingAgeSeconds float64) {
	initOutboxMetrics()
	outboxPendingCount.Set(float64(pending))
	outboxFailedCount.Set(float64(failed))
	outboxOldestPendingAgeGauge.Set(oldestPendingAgeSeconds)
}
