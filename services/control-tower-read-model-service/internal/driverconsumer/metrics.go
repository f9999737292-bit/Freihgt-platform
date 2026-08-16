package driverconsumer

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	consumedTotal   *prometheus.CounterVec
	failedTotal     *prometheus.CounterVec
	duplicateTotal  prometheus.Counter
	ruleMatches     *prometheus.CounterVec
}

var registerOnce sync.Once

func NewMetrics() *Metrics {
	m := &Metrics{
		consumedTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "control_tower_driver_events_consumed_total",
			Help: "Driver domain events consumed by Control Tower.",
		}, []string{"event_type"}),
		failedTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "control_tower_driver_events_failed_total",
			Help: "Failed driver domain event consumptions.",
		}, []string{"error_code"}),
		duplicateTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "control_tower_driver_events_duplicate_total",
			Help: "Duplicate driver domain events ignored.",
		}),
		ruleMatches: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "control_tower_driver_rule_matches_total",
			Help: "Driver event rule matches.",
		}, []string{"trigger_type"}),
	}
	registerOnce.Do(func() {
		prometheus.MustRegister(m.consumedTotal, m.failedTotal, m.duplicateTotal, m.ruleMatches)
	})
	return m
}

func (m *Metrics) IncConsumed(eventType string) {
	m.consumedTotal.WithLabelValues(eventType).Inc()
}

func (m *Metrics) IncFailed(code string) {
	m.failedTotal.WithLabelValues(code).Inc()
}

func (m *Metrics) IncDuplicate(outcome string) {
	m.duplicateTotal.Inc()
}

func (m *Metrics) IncRuleMatch(triggerType string) {
	m.ruleMatches.WithLabelValues(triggerType).Inc()
}
