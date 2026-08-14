package controltower

import (
	"encoding/json"
	"fmt"

	"github.com/freight-platform/api-gateway/internal/controltower/risk"
)

func (s *Service) triggerRiskAutomation(reqCtx RequestContext, assessments []risk.Assessment) {
	for _, item := range assessments {
		if item.Level != "high" && item.Level != "critical" {
			continue
		}
		triggerType := "risk_created"
		stateVersion := fmt.Sprintf("%s:%s:%d", item.RiskID, item.Level, item.Score)
		body, _ := json.Marshal(map[string]any{
			"triggerType":  triggerType,
			"triggerId":    fmt.Sprintf("risk:%s:%s", item.RiskID, stateVersion),
			"shipmentId":   item.ShipmentID,
			"riskId":       item.RiskID,
			"workItemType": "risk",
			"workItemId":   item.RiskID,
			"correlationId": fmt.Sprintf("risk-sync:%s", item.RiskID),
			"attributes": map[string]any{
				"riskLevel":              item.Level,
				"predictedExceptionType": item.PredictedExceptionType,
				"stateVersion":           stateVersion,
			},
			"persist": true,
		})
		s.fireAutomationTrigger(nil, reqCtx, body)
	}
}
