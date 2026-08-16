package service

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	ctmetrics "github.com/freight-platform/control-tower-read-model-service/internal/platform/metrics"
)

const AutomationCausationPrefix = "automation:"

type AutomationTriggerIngress struct {
	svc     *AutomationService
	metrics *ctmetrics.AutomationMetrics
	log     *slog.Logger
}

func NewAutomationTriggerIngress(svc *AutomationService, metrics *ctmetrics.AutomationMetrics, log *slog.Logger) *AutomationTriggerIngress {
	return &AutomationTriggerIngress{svc: svc, metrics: metrics, log: log}
}

func IsAutomationOrigin(causationID string) bool {
	return strings.HasPrefix(strings.TrimSpace(causationID), AutomationCausationPrefix)
}

func (i *AutomationTriggerIngress) HandleTrigger(ctx context.Context, tenantID uuid.UUID, trigger domain.AutomationTrigger, persist bool) (EvaluateOutcome, error) {
	start := time.Now()
	triggerType := strings.TrimSpace(trigger.TriggerType)

	if IsAutomationOrigin(trigger.CausationID) {
		if i.metrics != nil {
			i.metrics.ObserveSkipped("automation_origin")
		}
		return EvaluateOutcome{}, nil
	}
	if trigger.SourceOrigin == "automation" {
		if i.metrics != nil {
			i.metrics.ObserveSkipped("automation_source")
		}
		return EvaluateOutcome{}, nil
	}

	var out EvaluateOutcome
	outcome := "no_match"
	defer func() {
		if i.metrics != nil {
			i.metrics.ObserveTrigger(triggerType, outcome, time.Since(start), out.Deduplicated)
		}
	}()

	var err error
	out, err = i.svc.EvaluateTrigger(ctx, tenantID, trigger, persist)
	if err != nil {
		outcome = "error"
		if i.metrics != nil {
			i.metrics.ObserveRuleEvaluation("error")
		}
		return out, err
	}

	if len(out.Matches) > 0 {
		if i.metrics != nil {
			i.metrics.ObserveRuleMatch(triggerType, len(out.Matches))
			i.metrics.ObserveRuleEvaluation("matched")
		}
		outcome = "matched"
	} else if i.metrics != nil {
		i.metrics.ObserveRuleEvaluation("not_matched")
	}
	if len(out.Recommendations) > 0 {
		if i.metrics != nil {
			i.metrics.ObserveRecommendation(len(out.Recommendations))
		}
		outcome = "recommendation_created"
	}
	if out.Deduplicated > 0 {
		outcome = "deduplicated"
	}

	if i.log != nil {
		i.log.Info("automation trigger processed",
			slog.String("tenant_id", tenantID.String()),
			slog.String("trigger_type", triggerType),
			slog.String("trigger_id", trigger.TriggerID),
			slog.String("correlation_id", trigger.CorrelationID),
			slog.Int("matches", len(out.Matches)),
			slog.Int("recommendations", len(out.Recommendations)),
			slog.Int("deduplicated", out.Deduplicated),
		)
	}
	return out, nil
}
