package metrics

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type AutomationMetrics struct {
	triggersTotal            *prometheus.CounterVec
	triggerDuplicatesTotal   prometheus.Counter
	ruleEvaluationsTotal     *prometheus.CounterVec
	ruleMatchesTotal         *prometheus.CounterVec
	executionsTotal          *prometheus.CounterVec
	executionFailuresTotal   *prometheus.CounterVec
	actionsTotal             *prometheus.CounterVec
	actionFailuresTotal      *prometheus.CounterVec
	executionDurationSeconds *prometheus.HistogramVec
}

var automationRegisterOnce sync.Once

func NewAutomationMetrics() *AutomationMetrics {
	m := &AutomationMetrics{
		triggersTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "automation_triggers_total",
			Help: "Automation triggers received by type and outcome.",
		}, []string{"trigger_type", "outcome"}),
		triggerDuplicatesTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "automation_trigger_duplicates_total",
			Help: "Duplicate automation trigger deliveries deduplicated.",
		}),
		ruleEvaluationsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "automation_rule_evaluations_total",
			Help: "Automation rule evaluations by outcome.",
		}, []string{"outcome"}),
		ruleMatchesTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "automation_rule_matches_total",
			Help: "Automation rules matched by trigger type.",
		}, []string{"trigger_type"}),
		executionsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "automation_executions_total",
			Help: "Automation recommendation/execution outcomes.",
		}, []string{"outcome"}),
		executionFailuresTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "automation_execution_failures_total",
			Help: "Automation execution failures by reason.",
		}, []string{"reason"}),
		actionsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "automation_actions_total",
			Help: "Automation playbook action attempts by outcome.",
		}, []string{"outcome"}),
		actionFailuresTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "automation_action_failures_total",
			Help: "Automation action failures by reason.",
		}, []string{"reason"}),
		executionDurationSeconds: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "automation_execution_duration_seconds",
			Help:    "Duration of automation trigger processing.",
			Buckets: prometheus.DefBuckets,
		}, []string{"trigger_type", "outcome"}),
	}
	automationRegisterOnce.Do(func() {
		prometheus.MustRegister(
			m.triggersTotal,
			m.triggerDuplicatesTotal,
			m.ruleEvaluationsTotal,
			m.ruleMatchesTotal,
			m.executionsTotal,
			m.executionFailuresTotal,
			m.actionsTotal,
			m.actionFailuresTotal,
			m.executionDurationSeconds,
		)
	})
	return m
}

func (m *AutomationMetrics) ObserveTrigger(triggerType, outcome string, duration time.Duration, deduplicated int) {
	m.triggersTotal.WithLabelValues(triggerType, outcome).Inc()
	m.executionDurationSeconds.WithLabelValues(triggerType, outcome).Observe(duration.Seconds())
	if deduplicated > 0 {
		m.triggerDuplicatesTotal.Add(float64(deduplicated))
	}
}

func (m *AutomationMetrics) ObserveRuleEvaluation(outcome string) {
	m.ruleEvaluationsTotal.WithLabelValues(outcome).Inc()
}

func (m *AutomationMetrics) ObserveRuleMatch(triggerType string, count int) {
	if count > 0 {
		m.ruleMatchesTotal.WithLabelValues(triggerType).Add(float64(count))
	}
}

func (m *AutomationMetrics) ObserveRecommendation(created int) {
	if created > 0 {
		m.executionsTotal.WithLabelValues("recommendation_created").Add(float64(created))
	}
}

func (m *AutomationMetrics) ObserveSkipped(reason string) {
	m.triggersTotal.WithLabelValues("unknown", "skipped_"+reason).Inc()
}
