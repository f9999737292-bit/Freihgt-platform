package outbox

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Outbox metrics intentionally use bounded labels only (event_type, result, error_code).
// Never add tenant_id, shipment_id, event_id, or worker_id labels.
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
				Name: "shipment_outbox_claimed_total",
				Help: "Total shipment outbox events claimed for publishing",
			},
			[]string{"event_type"},
		)
		outboxPublishedTotal = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "shipment_outbox_published_total",
				Help: "Total shipment outbox events published successfully",
			},
			[]string{"event_type", "result"},
		)
		outboxPublishFailedTotal = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "shipment_outbox_publish_failed_total",
				Help: "Total shipment outbox publish attempts that failed and will retry",
			},
			[]string{"event_type", "error_code"},
		)
		outboxMarkedFailedTotal = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "shipment_outbox_marked_failed_total",
				Help: "Total shipment outbox events marked permanently failed",
			},
			[]string{"event_type", "error_code"},
		)
		outboxPublishDuration = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "shipment_outbox_publish_duration_seconds",
				Help:    "Shipment outbox publish duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"event_type", "result"},
		)
		outboxPendingCount = prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "shipment_outbox_pending_count",
			Help: "Current count of pending shipment outbox events",
		})
		outboxFailedCount = prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "shipment_outbox_failed_count",
			Help: "Current count of permanently failed shipment outbox events",
		})
		outboxOldestPendingAgeGauge = prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "shipment_outbox_oldest_pending_age_seconds",
			Help: "Age in seconds of the oldest pending shipment outbox event",
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

var (
	kafkaMetricsOnce sync.Once

	kafkaPublishTotal    *prometheus.CounterVec
	kafkaPublishDuration *prometheus.HistogramVec
	kafkaPublishErrors   *prometheus.CounterVec
)

func initKafkaMetrics() {
	kafkaMetricsOnce.Do(func() {
		kafkaPublishTotal = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "shipment_outbox_kafka_publish_total",
				Help: "Total Kafka publish attempts from shipment outbox worker transport",
			},
			[]string{"event_type", "result"},
		)
		kafkaPublishDuration = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "shipment_outbox_kafka_publish_duration_seconds",
				Help:    "Kafka publish duration in seconds from shipment outbox transport",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"event_type", "result"},
		)
		kafkaPublishErrors = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "shipment_outbox_kafka_publish_errors_total",
				Help: "Total Kafka publish errors from shipment outbox transport",
			},
			[]string{"event_type", "error_code"},
		)
		prometheus.MustRegister(kafkaPublishTotal, kafkaPublishDuration, kafkaPublishErrors)
	})
}

func ObserveKafkaPublish(eventType, result, errorCode string, duration time.Duration) {
	initKafkaMetrics()
	kafkaPublishTotal.WithLabelValues(eventType, result).Inc()
	kafkaPublishDuration.WithLabelValues(eventType, result).Observe(duration.Seconds())
	if result != "success" && errorCode != "" {
		kafkaPublishErrors.WithLabelValues(eventType, errorCode).Inc()
	}
}
