package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	"github.com/freight-platform/control-tower-read-model-service/internal/repository"
)

type GuardEvaluationInput struct {
	TenantID   uuid.UUID
	Trigger    domain.AutomationTrigger
	Rule       domain.AutomationRule
	ActionType string
	ShipmentID *uuid.UUID
	DriverID   *uuid.UUID
}

type GuardEvaluationResult struct {
	Decision string
	Reason   string
}

type GuardEvaluator struct {
	shipments *repository.ShipmentLookupRepository
	actions   *repository.GuardedActionRepository
}

func NewGuardEvaluator(shipments *repository.ShipmentLookupRepository, actions *repository.GuardedActionRepository) *GuardEvaluator {
	return &GuardEvaluator{shipments: shipments, actions: actions}
}

func (g *GuardEvaluator) EvaluateAction(ctx context.Context, in GuardEvaluationInput) (GuardEvaluationResult, error) {
	actionType := domain.NormalizeGuardedActionCode(in.ActionType)
	if domain.IsForbiddenAutomationAction(actionType) {
		return GuardEvaluationResult{Decision: domain.GuardDecisionDeny, Reason: "FORBIDDEN_ACTION"}, nil
	}
	spec, ok := domain.LookupGuardedActionSpec(actionType)
	if !ok {
		return GuardEvaluationResult{Decision: domain.GuardDecisionDeny, Reason: "UNKNOWN_ACTION"}, nil
	}
	policy, err := g.actions.GetTenantActionPolicy(ctx, in.TenantID, actionType)
	if err != nil {
		return GuardEvaluationResult{}, err
	}
	if policy != nil && !policy.Enabled {
		return GuardEvaluationResult{Decision: domain.GuardDecisionDeny, Reason: "TENANT_ACTION_DISABLED"}, nil
	}
	if spec.RequiresShipment {
		if in.ShipmentID == nil {
			return GuardEvaluationResult{Decision: domain.GuardDecisionDeny, Reason: "MISSING_SHIPMENT"}, nil
		}
		assignment, err := g.shipments.GetAssignedDriver(ctx, in.TenantID, *in.ShipmentID)
		if err != nil {
			return GuardEvaluationResult{Decision: domain.GuardDecisionDeny, Reason: "SHIPMENT_LOOKUP_FAILED"}, nil
		}
		if assignment.TenantID != in.TenantID {
			return GuardEvaluationResult{Decision: domain.GuardDecisionDeny, Reason: "TENANT_MISMATCH"}, nil
		}
		if assignment.DriverID == nil {
			return GuardEvaluationResult{Decision: domain.GuardDecisionDeny, Reason: "NO_ASSIGNED_DRIVER"}, nil
		}
		if in.DriverID != nil && *in.DriverID != *assignment.DriverID {
			return GuardEvaluationResult{Decision: domain.GuardDecisionDeny, Reason: "STALE_DRIVER_ASSIGNMENT"}, nil
		}
		if err := g.shipments.ValidateDriverTenant(ctx, in.TenantID, *assignment.DriverID); err != nil {
			return GuardEvaluationResult{Decision: domain.GuardDecisionDeny, Reason: "DRIVER_TENANT_MISMATCH"}, nil
		}
	}
	if spec.RequiresDriver && in.ShipmentID == nil && in.DriverID == nil {
		return GuardEvaluationResult{Decision: domain.GuardDecisionDeny, Reason: "MISSING_DRIVER"}, nil
	}
	if in.DriverID != nil {
		if err := g.shipments.ValidateDriverTenant(ctx, in.TenantID, *in.DriverID); err != nil {
			return GuardEvaluationResult{Decision: domain.GuardDecisionDeny, Reason: "DRIVER_TENANT_MISMATCH"}, nil
		}
	}
	if depth := causationDepth(in.Trigger.CausationID); depth >= domain.MaxAutomationCausationDepth {
		return GuardEvaluationResult{Decision: domain.GuardDecisionDeny, Reason: "MAX_CAUSATION_DEPTH"}, nil
	}
	level := domain.EffectiveApprovalLevel(actionType, policy)
	if level != domain.ApprovalLevelNone {
		return GuardEvaluationResult{Decision: domain.GuardDecisionRequireApproval, Reason: fmt.Sprintf("APPROVAL_%s", strings.ToUpper(level))}, nil
	}
	return GuardEvaluationResult{Decision: domain.GuardDecisionAllow, Reason: "ALLOWED"}, nil
}

func causationDepth(causationID string) int {
	parts := strings.Split(strings.TrimSpace(causationID), ":")
	return len(parts)
}
