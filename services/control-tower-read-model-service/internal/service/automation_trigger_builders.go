package service

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
)

func BuildCaseCreatedTrigger(tenantID uuid.UUID, c domain.OperationalCase, shipmentIDs []uuid.UUID, causationID string) domain.AutomationTrigger {
	stateVersion := fmt.Sprintf("%s:%s:%d", c.ID.String(), c.Status, c.Version)
	trigger := domain.AutomationTrigger{
		TenantID:      tenantID,
		TriggerType:   "case_created",
		TriggerID:     fmt.Sprintf("case:%s:%s", c.ID.String(), stateVersion),
		CaseID:        &c.ID,
		WorkItemType:  "case",
		WorkItemID:    c.ID.String(),
		CorrelationID: fmt.Sprintf("case-create:%s", c.ID.String()),
		CausationID:   causationID,
		OccurredAt:    time.Now().UTC(),
		Attributes: domain.TriggerAttributes{
			CaseStatus:   c.Status,
			CaseSeverity: c.EffectiveSeverity,
			StateVersion: stateVersion,
		},
	}
	if len(shipmentIDs) > 0 {
		trigger.ShipmentID = &shipmentIDs[0]
	}
	return trigger
}

func BuildExceptionCreatedTrigger(tenantID uuid.UUID, seed domain.EnsureExceptionSeed) domain.AutomationTrigger {
	stateVersion := fmt.Sprintf("%s:%s:%s", seed.EventID, seed.EventType, seed.Severity)
	shipmentID, _ := uuid.Parse(seed.ShipmentID)
	var shipmentPtr *uuid.UUID
	if shipmentID != uuid.Nil {
		shipmentPtr = &shipmentID
	}
	return domain.AutomationTrigger{
		TenantID:      tenantID,
		TriggerType:   "exception_created",
		TriggerID:     fmt.Sprintf("exception:%s:%s", seed.EventID, stateVersion),
		ShipmentID:    shipmentPtr,
		ExceptionID:   seed.EventID,
		WorkItemType:  "exception",
		WorkItemID:    seed.EventID,
		CorrelationID: fmt.Sprintf("exception-ensure:%s", seed.EventID),
		OccurredAt:    seed.OccurredAt.UTC(),
		Attributes: domain.TriggerAttributes{
			ExceptionCategory: seed.EventType,
			Priority:          seed.Severity,
			StateVersion:      stateVersion,
		},
	}
}

func BuildExceptionSLATrigger(tenantID uuid.UUID, workflow domain.CriticalEventWorkflow, sla domain.SLAEvaluation) domain.AutomationTrigger {
	stateVersion := fmt.Sprintf("%s:%s:%s", workflow.EventID, sla.Phase, sla.Status)
	return domain.AutomationTrigger{
		TenantID:      tenantID,
		TriggerType:   "sla_breached",
		TriggerID:     fmt.Sprintf("exception-sla:%s:%s", workflow.EventID, stateVersion),
		ShipmentID:    &workflow.ShipmentID,
		ExceptionID:   workflow.EventID,
		WorkItemType:  "exception",
		WorkItemID:    workflow.EventID,
		CorrelationID: fmt.Sprintf("exception-sla:%s", workflow.EventID),
		OccurredAt:    time.Now().UTC(),
		Attributes: domain.TriggerAttributes{
			SLAStatus:       sla.Status,
			WorkflowStatus:  workflow.Status,
			EscalationLevel: workflow.EscalationLevel,
			Priority:        workflow.Priority,
			StateVersion:    stateVersion,
		},
	}
}

func BuildEtaAtRiskTrigger(tenantID uuid.UUID, shipmentID uuid.UUID, triggerID, correlationID string, projectedDelaySeconds int64) domain.AutomationTrigger {
	stateVersion := fmt.Sprintf("%s:%d", shipmentID.String(), projectedDelaySeconds)
	return domain.AutomationTrigger{
		TenantID:      tenantID,
		TriggerType:   "eta_at_risk",
		TriggerID:     triggerID,
		ShipmentID:    &shipmentID,
		CorrelationID: correlationID,
		OccurredAt:    time.Now().UTC(),
		Attributes: domain.TriggerAttributes{
			ETAStatus:             "at_risk",
			ProjectedDelaySeconds: &projectedDelaySeconds,
			StateVersion:          stateVersion,
		},
	}
}
