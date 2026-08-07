package metrics

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type ConsumerMetrics struct {
	recordsTotal              *prometheus.CounterVec
	processingDurationSeconds *prometheus.HistogramVec
	errorsTotal               *prometheus.CounterVec
	offsetCommitErrorsTotal   prometheus.Counter
	projectionAppliedTotal    *prometheus.CounterVec
	projectionDuplicateTotal  prometheus.Counter
	projectionStaleTotal      prometheus.Counter
	projectionGapTotal        prometheus.Counter
	deadLetterTotal           *prometheus.CounterVec
	lastRecordTimestamp       prometheus.Gauge
	lastAppliedTimestamp      prometheus.Gauge
}

var registerOnce sync.Once

func NewConsumerMetrics() *ConsumerMetrics {
	m := &ConsumerMetrics{
		recordsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "control_tower_shipment_consumer_records_total",
			Help: "Kafka records polled by the shipment status consumer.",
		}, []string{"event_type", "outcome"}),
		processingDurationSeconds: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "control_tower_shipment_consumer_processing_duration_seconds",
			Help:    "Duration of shipment status event processing.",
			Buckets: prometheus.DefBuckets,
		}, []string{"event_type", "outcome"}),
		errorsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "control_tower_shipment_consumer_errors_total",
			Help: "Consumer processing errors by safe error code.",
		}, []string{"error_code"}),
		offsetCommitErrorsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "control_tower_shipment_consumer_offset_commit_errors_total",
			Help: "Kafka offset commit failures after successful DB commit.",
		}),
		projectionAppliedTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "control_tower_shipment_projection_applied_total",
			Help: "Projection updates applied by outcome.",
		}, []string{"outcome"}),
		projectionDuplicateTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "control_tower_shipment_projection_duplicate_total",
			Help: "Duplicate shipment status events detected via inbox.",
		}),
		projectionStaleTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "control_tower_shipment_projection_stale_total",
			Help: "Stale shipment status events ignored for projection.",
		}),
		projectionGapTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "control_tower_shipment_projection_gap_total",
			Help: "Shipment status events applied with version gap markers.",
		}),
		deadLetterTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "control_tower_shipment_dead_letter_total",
			Help: "Permanent invalid shipment status events stored in dead-letter.",
		}, []string{"error_code"}),
		lastRecordTimestamp: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "control_tower_shipment_consumer_last_record_timestamp_seconds",
			Help: "Unix timestamp of the last Kafka record received.",
		}),
		lastAppliedTimestamp: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "control_tower_shipment_projection_last_applied_timestamp_seconds",
			Help: "Unix timestamp of the last successful projection apply.",
		}),
	}
	registerOnce.Do(func() {
		prometheus.MustRegister(
			m.recordsTotal,
			m.processingDurationSeconds,
			m.errorsTotal,
			m.offsetCommitErrorsTotal,
			m.projectionAppliedTotal,
			m.projectionDuplicateTotal,
			m.projectionStaleTotal,
			m.projectionGapTotal,
			m.deadLetterTotal,
			m.lastRecordTimestamp,
			m.lastAppliedTimestamp,
		)
	})
	return m
}

func (m *ConsumerMetrics) ObserveRecord(eventType, outcome string, duration time.Duration) {
	m.recordsTotal.WithLabelValues(eventType, outcome).Inc()
	m.processingDurationSeconds.WithLabelValues(eventType, outcome).Observe(duration.Seconds())
}

func (m *ConsumerMetrics) ObserveError(errorCode string) {
	m.errorsTotal.WithLabelValues(errorCode).Inc()
}

func (m *ConsumerMetrics) ObserveOffsetCommitError() {
	m.offsetCommitErrorsTotal.Inc()
}

func (m *ConsumerMetrics) ObserveOutcome(outcome, eventType string) {
	switch outcome {
	case "APPLIED", "GAP_APPLIED":
		m.projectionAppliedTotal.WithLabelValues(outcome).Inc()
	case "DUPLICATE":
		m.projectionDuplicateTotal.Inc()
	case "STALE":
		m.projectionStaleTotal.Inc()
	}
	if outcome == "GAP_APPLIED" {
		m.projectionGapTotal.Inc()
	}
}

func (m *ConsumerMetrics) ObserveDeadLetter(errorCode string) {
	m.deadLetterTotal.WithLabelValues(errorCode).Inc()
}

func (m *ConsumerMetrics) SetLastRecordAt(at time.Time) {
	if !at.IsZero() {
		m.lastRecordTimestamp.Set(float64(at.UTC().Unix()))
	}
}

func (m *ConsumerMetrics) SetLastAppliedAt(at time.Time) {
	if !at.IsZero() {
		m.lastAppliedTimestamp.Set(float64(at.UTC().Unix()))
	}
}
