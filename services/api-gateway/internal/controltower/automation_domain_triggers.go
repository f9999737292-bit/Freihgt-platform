package controltower

import (
	"encoding/json"
	"fmt"

	"github.com/freight-platform/api-gateway/internal/controltower/risk"
	"github.com/freight-platform/api-gateway/internal/controltowerreadmodel"
)

// triggerDomainAutomation fires normalized automation triggers derived from risk assessments
// and enriched shipment context (ETA, slot, tracking, SLA signals).
func (s *Service) triggerDomainAutomation(reqCtx RequestContext, rows []ControlTowerShipment, assessments []risk.Assessment) {
	shipmentByID := make(map[string]ControlTowerShipment, len(rows))
	for _, row := range rows {
		shipmentByID[row.ID] = row
	}

	seen := map[string]struct{}{}
	for _, item := range assessments {
		row, hasShipment := shipmentByID[item.ShipmentID]

		if item.Level == "high" || item.Level == "critical" {
			s.fireDomainTrigger(reqCtx, buildRiskCreatedTrigger(item), seen)
		}

		for _, sig := range item.Signals {
			if tr := mapSignalToTrigger(item, sig, row, hasShipment); tr != nil {
				s.fireDomainTrigger(reqCtx, tr, seen)
			}
		}

		if hasShipment {
			if tr := buildShipmentSLATrigger(row); tr != nil {
				s.fireDomainTrigger(reqCtx, tr, seen)
			}
		}
	}
}

func (s *Service) fireDomainTrigger(reqCtx RequestContext, payload map[string]any, seen map[string]struct{}) {
	triggerType, _ := payload["triggerType"].(string)
	triggerID, _ := payload["triggerId"].(string)
	key := triggerType + "|" + triggerID
	if _, ok := seen[key]; ok {
		return
	}
	seen[key] = struct{}{}
	body, _ := json.Marshal(payload)
	s.fireAutomationTrigger(nil, reqCtx, body)
}

func buildRiskCreatedTrigger(item risk.Assessment) map[string]any {
	stateVersion := fmt.Sprintf("%s:%s:%d", item.RiskID, item.Level, item.Score)
	return map[string]any{
		"triggerType":   "risk_created",
		"triggerId":     fmt.Sprintf("risk:%s:%s", item.RiskID, stateVersion),
		"shipmentId":    item.ShipmentID,
		"riskId":        item.RiskID,
		"workItemType":  "risk",
		"workItemId":    item.RiskID,
		"correlationId": fmt.Sprintf("risk-sync:%s", item.RiskID),
		"attributes": map[string]any{
			"riskLevel":              item.Level,
			"predictedExceptionType": item.PredictedExceptionType,
			"stateVersion":           stateVersion,
		},
		"persist": true,
	}
}

func mapSignalToTrigger(item risk.Assessment, sig risk.Signal, row ControlTowerShipment, hasShipment bool) map[string]any {
	triggerType := signalToTriggerType(sig.Code)
	if triggerType == "" {
		return nil
	}

	stateVersion := fmt.Sprintf("%s:%s:%d", item.ShipmentID, sig.Code, sig.Weight)
	attrs := map[string]any{
		"stateVersion":           stateVersion,
		"predictedExceptionType": item.PredictedExceptionType,
		"riskLevel":              item.Level,
	}

	if hasShipment {
		mergeShipmentAttributes(attrs, row, triggerType)
	}

	triggerID := fmt.Sprintf("%s:%s:%s", triggerType, item.ShipmentID, stateVersion)
	payload := map[string]any{
		"triggerType":   triggerType,
		"triggerId":     triggerID,
		"shipmentId":    item.ShipmentID,
		"riskId":        item.RiskID,
		"workItemType":  "risk",
		"workItemId":    item.RiskID,
		"correlationId": fmt.Sprintf("risk-signal:%s:%s", item.ShipmentID, sig.Code),
		"attributes":    attrs,
		"persist":       true,
	}
	return payload
}

func signalToTriggerType(code string) string {
	switch code {
	case "eta_delivery_at_risk", "delivery_at_risk_sla", "delivery_window_approaching":
		return "eta_at_risk"
	case "eta_after_planned_delivery", "delivery_deadline_imminent", "delivery_time_near_without_completion":
		return "eta_projected_late"
	case "slot_eta_at_risk", "slot_window_near", "slot_eta_stale":
		return "slot_at_risk"
	case "slot_projected_miss":
		return "slot_projected_miss"
	case "slot_actual_missed":
		return "slot_actual_missed"
	case "telemetry_stale":
		return "tracking_stale"
	case "telemetry_lost":
		return "tracking_lost"
	default:
		return ""
	}
}

func mergeShipmentAttributes(attrs map[string]any, row ControlTowerShipment, triggerType string) {
	if row.ETAStatus != nil {
		attrs["etaStatus"] = *row.ETAStatus
	}
	if row.ArrivalProjection != nil {
		attrs["arrivalProjection"] = *row.ArrivalProjection
	}
	if row.ProjectedDelaySeconds != nil {
		attrs["projectedDelaySeconds"] = *row.ProjectedDelaySeconds
	}
	if row.TrackingStatus != nil {
		attrs["trackingStatus"] = *row.TrackingStatus
	}
	if row.TrackingQuality != nil {
		attrs["trackingQuality"] = *row.TrackingQuality
	}
	if row.SLAStatus != "" {
		attrs["slaStatus"] = string(row.SLAStatus)
	}

	switch triggerType {
	case "slot_at_risk", "slot_projected_miss", "slot_actual_missed":
		if row.DeliverySlotArrivalProjection != nil {
			attrs["slotType"] = "delivery"
			attrs["slotArrivalProjection"] = *row.DeliverySlotArrivalProjection
			if row.DeliverySlotProjectedLateSeconds != nil {
				attrs["slotProjectedLateSeconds"] = *row.DeliverySlotProjectedLateSeconds
			}
		} else if row.PickupSlotArrivalProjection != nil {
			attrs["slotType"] = "pickup"
			attrs["slotArrivalProjection"] = *row.PickupSlotArrivalProjection
			if row.PickupSlotProjectedLateSeconds != nil {
				attrs["slotProjectedLateSeconds"] = *row.PickupSlotProjectedLateSeconds
			}
		}
	}
}

func buildShipmentSLATrigger(row ControlTowerShipment) map[string]any {
	slaStatus := string(row.SLAStatus)
	var triggerType string
	switch slaStatus {
	case string(SLAStatusAtRisk):
		triggerType = "sla_warning"
	case string(SLAStatusCritical), string(SLAStatusDelayed):
		triggerType = "sla_breached"
	default:
		return nil
	}

	stateVersion := fmt.Sprintf("%s:%s", row.ID, slaStatus)
	if row.SLAReason != nil {
		stateVersion = fmt.Sprintf("%s:%s:%s", row.ID, slaStatus, *row.SLAReason)
	}

	attrs := map[string]any{
		"slaStatus":    slaStatus,
		"stateVersion": stateVersion,
	}
	if row.SLAReason != nil {
		attrs["businessImpact"] = *row.SLAReason
	}

	return map[string]any{
		"triggerType":   triggerType,
		"triggerId":     fmt.Sprintf("%s:%s:%s", triggerType, row.ID, stateVersion),
		"shipmentId":    row.ID,
		"correlationId": fmt.Sprintf("shipment-sla:%s", row.ID),
		"attributes":    attrs,
		"persist":       true,
	}
}

// triggerExceptionAutomation fires exception_created for newly ensured critical event workflows.
func (s *Service) triggerExceptionAutomation(reqCtx RequestContext, seeds []controltowerreadmodel.EnsureExceptionSeed, createdEventIDs []string) {
	created := map[string]struct{}{}
	for _, id := range createdEventIDs {
		created[id] = struct{}{}
	}
	for _, seed := range seeds {
		if _, ok := created[seed.EventID]; !ok {
			continue
		}
		stateVersion := fmt.Sprintf("%s:%s:%s", seed.EventID, seed.EventType, seed.Severity)
		body, _ := json.Marshal(map[string]any{
			"triggerType":   "exception_created",
			"triggerId":     fmt.Sprintf("exception:%s:%s", seed.EventID, stateVersion),
			"shipmentId":    seed.ShipmentID,
			"exceptionId":   seed.EventID,
			"workItemType":  "exception",
			"workItemId":    seed.EventID,
			"correlationId": fmt.Sprintf("exception-ensure:%s", seed.EventID),
			"attributes": map[string]any{
				"exceptionCategory": seed.EventType,
				"priority":          seed.Severity,
				"stateVersion":      stateVersion,
			},
			"persist": true,
		})
		s.fireAutomationTrigger(nil, reqCtx, body)
	}
}

// triggerExceptionSLAAutomation fires sla_breached when exception workflow SLA is breached.
func (s *Service) triggerExceptionSLAAutomation(reqCtx RequestContext, event ControlTowerEvent) {
	if event.SLA == nil || event.SLA.Status != "breached" {
		return
	}
	sla := event.SLA
	escalationLevel := ""
	if event.Escalation != nil {
		escalationLevel = event.Escalation.Level
	}
	stateVersion := fmt.Sprintf("%s:%s:%s", event.ID, sla.Phase, sla.Status)
	body, _ := json.Marshal(map[string]any{
		"triggerType":   "sla_breached",
		"triggerId":     fmt.Sprintf("exception-sla:%s:%s", event.ID, stateVersion),
		"shipmentId":    event.ShipmentID,
		"exceptionId":   event.ID,
		"workItemType":  "exception",
		"workItemId":    event.ID,
		"correlationId": fmt.Sprintf("exception-sla:%s", event.ID),
		"attributes": map[string]any{
			"slaStatus":       sla.Status,
			"workflowStatus":  event.Status,
			"escalationLevel": escalationLevel,
			"stateVersion":    stateVersion,
		},
		"persist": true,
	})
	s.fireAutomationTrigger(nil, reqCtx, body)
}
