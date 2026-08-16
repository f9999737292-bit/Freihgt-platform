package controltower

import (
	"context"
	"strings"
)

func (s *Service) mergeDriverCriticalEvents(
	ctx context.Context,
	reqCtx RequestContext,
	events *[]ControlTowerEvent,
	shipmentByID map[string]ControlTowerShipment,
) {
	if events == nil || !s.readModelCfg.Mode.Enabled() || s.readModel == nil {
		return
	}

	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.readModelCfg.Timeout)
	defer cancel()

	workflows, depErr := s.readModel.ListOpenWorkflowsBySource(rmCtx, reqCtx.TenantID, reqCtx.RequestID, EventSourceDriver)
	if depErr != nil || len(workflows) == 0 {
		return
	}

	existing := make(map[string]struct{}, len(*events))
	for _, event := range *events {
		existing[event.ID] = struct{}{}
	}

	for _, workflow := range workflows {
		if _, ok := existing[workflow.EventID]; ok {
			continue
		}
		shipmentNumber := ""
		if shipment, ok := shipmentByID[workflow.ShipmentID]; ok {
			shipmentNumber = shipment.ShipmentNumber
		}
		*events = append(*events, ControlTowerEvent{
			ID:                workflow.EventID,
			ShipmentID:        workflow.ShipmentID,
			ShipmentNumber:    shipmentNumber,
			Type:              workflow.EventType,
			Severity:          severityForWorkflowPriority(workflow.Priority),
			OccurredAt:        workflow.OccurredAt,
			Source:            EventSourceDriver,
			Status:            workflow.Status,
			Priority:          workflow.Priority,
			ExceptionCategory: workflow.ExceptionCategory,
			BusinessImpact:    workflow.BusinessImpact,
		})
		existing[workflow.EventID] = struct{}{}
	}
}

func severityForWorkflowPriority(priority string) string {
	switch strings.ToLower(strings.TrimSpace(priority)) {
	case "p1", "critical", "high":
		return EventSeverityCritical
	case "p2", "medium", "warning":
		return EventSeverityWarning
	default:
		return EventSeverityInfo
	}
}
