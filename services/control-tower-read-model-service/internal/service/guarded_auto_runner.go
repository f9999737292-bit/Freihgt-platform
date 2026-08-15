package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	"github.com/freight-platform/control-tower-read-model-service/internal/repository"
)

type GuardedAutoRunner struct {
	automation *repository.AutomationRepository
	guarded    *GuardedActionService
}

func NewGuardedAutoRunner(automation *repository.AutomationRepository, guarded *GuardedActionService) *GuardedAutoRunner {
	return &GuardedAutoRunner{automation: automation, guarded: guarded}
}

func (r *GuardedAutoRunner) RunRecommendation(ctx context.Context, tenantID uuid.UUID, rec domain.AutomationRecommendation, trigger domain.AutomationTrigger) error {
	if r.guarded == nil {
		return nil
	}
	rule, err := r.automation.GetRule(ctx, tenantID, rec.RuleID)
	if err != nil {
		return err
	}
	actorID := rule.CreatedByUserID
	if actorID == uuid.Nil {
		actorID = uuid.New()
	}
	_, exec, err := r.automation.AcceptRecommendation(ctx, tenantID, actorID, rec.ID)
	if err != nil {
		return err
	}
	exec, err = r.automation.StartExecution(ctx, tenantID, actorID, exec.ID)
	if err != nil {
		return err
	}
	dispatched := false
	for _, step := range exec.Steps {
		if step.StepType != domain.StepTypeSystemAction {
			continue
		}
		dispatched = true
		_, err := r.guarded.DispatchSystemStep(ctx, DispatchGuardedStepInput{
			TenantID: tenantID, Execution: exec, Step: step, Trigger: trigger, ActorUserID: &actorID,
		})
		if err != nil {
			return err
		}
	}
	if !dispatched {
		return fmt.Errorf("no system_action steps found for execution %s", exec.ID)
	}
	return nil
}
