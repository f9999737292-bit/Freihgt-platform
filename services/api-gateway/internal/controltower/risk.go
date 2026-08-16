package controltower

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/freight-platform/api-gateway/internal/controltower/risk"
	"github.com/freight-platform/api-gateway/internal/controltowerreadmodel"
	apperrors "github.com/freight-platform/api-gateway/internal/platform/errors"
)

type ControlTowerRiskSignal struct {
	SignalCode     string         `json:"signalCode"`
	Severity       string         `json:"severity"`
	Weight         int            `json:"weight"`
	ObservedAt     time.Time      `json:"observedAt"`
	Source         string         `json:"source"`
	Value          map[string]any `json:"value,omitempty"`
	ExplanationKey string         `json:"explanationKey"`
}

type ControlTowerShipmentRisk struct {
	RiskID                 string                   `json:"riskId"`
	ShipmentID             string                   `json:"shipmentId"`
	ShipmentNumber         string                   `json:"shipmentNumber"`
	PredictedExceptionType string                   `json:"predictedExceptionType"`
	Score                  int                      `json:"score"`
	Level                  string                   `json:"level"`
	Status                 string                   `json:"status"`
	Signals                []ControlTowerRiskSignal `json:"signals"`
	FirstDetectedAt        time.Time                `json:"firstDetectedAt"`
	EvaluatedAt            time.Time                `json:"evaluatedAt"`
	NextEvaluationAt       *time.Time               `json:"nextEvaluationAt,omitempty"`
	ThreatenedDeadlineAt   *time.Time               `json:"threatenedDeadlineAt,omitempty"`
	MitigationCode         *string                  `json:"mitigationCode,omitempty"`
	MitigationComment      *string                  `json:"mitigationComment,omitempty"`
	ActualEventID          *string                  `json:"actualEventId,omitempty"`
	MaterializedAt         *time.Time               `json:"materializedAt,omitempty"`
	ClearedAt              *time.Time               `json:"clearedAt,omitempty"`
	ClearReason            *string                  `json:"clearReason,omitempty"`
	LeadTimeSeconds        *int64                   `json:"leadTimeSeconds,omitempty"`
	Actions                []ControlTowerRiskAction `json:"actions,omitempty"`
}

type ControlTowerRiskAction struct {
	ActionType  string         `json:"actionType"`
	ActorUserID *string        `json:"actorUserId,omitempty"`
	OccurredAt  time.Time      `json:"occurredAt"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type RiskKPI struct {
	ActiveRisks                  int `json:"activeRisks"`
	CriticalRisks                int `json:"criticalRisks"`
	HighRisks                    int `json:"highRisks"`
	DeliveryDelayRisks           int `json:"deliveryDelayRisks"`
	PickupDelayRisks             int `json:"pickupDelayRisks"`
	SlotMissRisks                int `json:"slotMissRisks"`
	MitigatingRisks              int `json:"mitigatingRisks"`
	RisksMaterialized            int `json:"risksMaterialized"`
	RisksCleared                 int `json:"risksCleared"`
	ClearedBeforeMaterialization int `json:"clearedBeforeMaterialization"`
}

type RisksListResponse struct {
	Items []ControlTowerShipmentRisk `json:"items"`
}

type mitigateRiskRequestPayload struct {
	MitigationCode string  `json:"mitigationCode"`
	Comment        *string `json:"comment,omitempty"`
}

func (s *Service) evaluateAndPersistRisks(
	ctx context.Context,
	reqCtx RequestContext,
	allRows []ControlTowerShipment,
	rawByID map[string]rawShipment,
	criticalEvents []ControlTowerEvent,
	now time.Time,
) {
	if !s.readModelCfg.Mode.Enabled() || s.readModel == nil {
		return
	}

	inputs := buildRiskShipmentInputs(allRows, rawByID)
	evaluator := risk.NewEvaluator(risk.Thresholds{
		AtRiskMinutes:        s.thresholds.AtRiskMinutes,
		StaleWarningMinutes:  s.thresholds.StaleWarningMinutes,
		StaleCriticalMinutes: s.thresholds.StaleCriticalMinutes,
	}, now)
	assessments := evaluator.EvaluateAll(inputs)

	syncEvaluations := make([]controltowerreadmodel.SyncRiskEvaluation, 0, len(assessments))
	for _, item := range assessments {
		signals := make([]controltowerreadmodel.SyncRiskSignal, 0, len(item.Signals))
		for _, sig := range item.Signals {
			signals = append(signals, controltowerreadmodel.SyncRiskSignal{
				Code:           sig.Code,
				Severity:       sig.Severity,
				Weight:         sig.Weight,
				ObservedAt:     controltowerreadmodel.FormatRiskTime(sig.ObservedAt),
				Source:         sig.Source,
				Value:          sig.Value,
				ExplanationKey: sig.ExplanationKey,
			})
		}
		syncEvaluations = append(syncEvaluations, controltowerreadmodel.SyncRiskEvaluation{
			RiskKey:                item.RiskID,
			ShipmentID:             item.ShipmentID,
			PredictedExceptionType: item.PredictedExceptionType,
			Score:                  item.Score,
			RiskLevel:              item.Level,
			EvaluatedAt:            controltowerreadmodel.FormatRiskTime(item.EvaluatedAt),
			NextEvaluationAt:       controltowerreadmodel.FormatRiskTime(item.NextEvaluationAt),
			ThreatenedDeadlineAt:   controltowerreadmodel.FormatRiskTimePtr(item.ThreatenedDeadlineAt),
			SignalsHash:            risk.SignalsHash(item.Signals),
			Signals:                signals,
		})
	}

	materializations := buildRiskMaterializations(criticalEvents, now)

	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.readModelCfg.Timeout)
	defer cancel()
	_ = s.readModel.SyncRisks(rmCtx, controltowerreadmodel.SyncRisksInput{
		TenantID:         reqCtx.TenantID,
		RequestID:        reqCtx.RequestID,
		Evaluations:      syncEvaluations,
		Materializations: materializations,
	})
	s.triggerDomainAutomation(reqCtx, allRows, assessments)
}

func buildRiskShipmentInputs(rows []ControlTowerShipment, rawByID map[string]rawShipment) []risk.ShipmentInput {
	inputs := make([]risk.ShipmentInput, 0)
	for _, row := range rows {
		if !IsActiveShipmentStatus(row.Status) {
			continue
		}
		raw, ok := rawByID[row.ID]
		if !ok {
			continue
		}
		inputs = append(inputs, risk.ShipmentInput{
			ID:                row.ID,
			ShipmentNumber:    row.ShipmentNumber,
			Status:            row.Status,
			PlannedPickupAt:   row.PlannedPickupAt,
			PlannedDeliveryAt: row.PlannedDeliveryAt,
			ActualPickupAt:    row.ActualPickupAt,
			ActualDeliveryAt:  row.ActualDeliveryAt,
			LastUpdatedAt:     row.LastUpdatedAt,
			DriverID:          raw.DriverID,
			VehicleID:         raw.VehicleID,
			DocumentsComplete: row.DocumentsComplete,
			SLAStatus:         string(row.SLAStatus),
			SLAReason:         row.SLAReason,
			Telemetry:         telemetryContextFromShipment(row),
			ETA:               etaContextFromShipment(row),
			Slot:              slotContextFromShipment(row),
		})
	}
	return inputs
}

func buildRiskMaterializations(events []ControlTowerEvent, now time.Time) []controltowerreadmodel.MaterializeRiskInput {
	out := make([]controltowerreadmodel.MaterializeRiskInput, 0)
	seen := map[string]struct{}{}
	for _, event := range events {
		predictedType, ok := risk.MaterializationMap[event.Type]
		if !ok {
			continue
		}
		riskKey := risk.DeterministicRiskID(event.ShipmentID, predictedType)
		if _, exists := seen[riskKey]; exists {
			continue
		}
		seen[riskKey] = struct{}{}
		materializedAt := event.OccurredAt
		if materializedAt.IsZero() {
			materializedAt = now
		}
		out = append(out, controltowerreadmodel.MaterializeRiskInput{
			RiskKey:        riskKey,
			ShipmentID:     event.ShipmentID,
			PredictedType:  predictedType,
			ActualEventID:  event.ID,
			MaterializedAt: controltowerreadmodel.FormatRiskTime(materializedAt),
		})
	}
	return out
}

func (s *Service) loadShipmentRisks(
	ctx context.Context,
	reqCtx RequestContext,
	query ListQuery,
	shipmentNumbers map[string]string,
) ([]ControlTowerShipmentRisk, RiskKPI) {
	if !s.readModelCfg.Mode.Enabled() || s.readModel == nil {
		return nil, RiskKPI{}
	}

	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.readModelCfg.Timeout)
	defer cancel()

	filter := controltowerreadmodel.ListRisksFilter{
		Level:             query.RiskLevel,
		Status:            query.RiskStatus,
		PredictedType:     query.RiskPredictedType,
		ShipmentID:        query.RiskShipmentID,
		ActiveOnly:        query.RiskActiveOnly,
		MitigatingOnly:    query.RiskMitigatingOnly,
		NonMitigatingOnly: query.RiskNonMitigatingOnly,
	}
	if filter.Level == "" && filter.Status == "" && filter.PredictedType == "" &&
		filter.ShipmentID == "" && !filter.MitigatingOnly && !filter.NonMitigatingOnly {
		filter.ActiveOnly = true
	}

	remoteItems, depErr := s.readModel.ListRisks(rmCtx, reqCtx.TenantID, reqCtx.RequestID, filter)
	if depErr != nil {
		return nil, RiskKPI{}
	}

	kpiRemote, _ := s.readModel.GetRiskKPI(rmCtx, reqCtx.TenantID, reqCtx.RequestID)
	kpi := mapRemoteRiskKPI(kpiRemote)

	items := make([]ControlTowerShipmentRisk, 0, len(remoteItems))
	for _, remote := range remoteItems {
		items = append(items, mapRemoteShipmentRisk(remote, shipmentNumbers))
	}
	SortShipmentRisks(items)
	return items, kpi
}

func mapRemoteRiskKPI(remote *controltowerreadmodel.RemoteRiskKPI) RiskKPI {
	if remote == nil {
		return RiskKPI{}
	}
	cleared := int(remote.RisksCleared)
	return RiskKPI{
		ActiveRisks:                  int(remote.ActiveRisks),
		CriticalRisks:                int(remote.CriticalRisks),
		HighRisks:                    int(remote.HighRisks),
		DeliveryDelayRisks:           int(remote.DeliveryDelayRisks),
		PickupDelayRisks:             int(remote.PickupDelayRisks),
		SlotMissRisks:                int(remote.SlotMissRisks),
		MitigatingRisks:              int(remote.MitigatingRisks),
		RisksMaterialized:            int(remote.RisksMaterialized),
		RisksCleared:                 cleared,
		ClearedBeforeMaterialization: cleared,
	}
}

func mapRemoteShipmentRisk(remote controltowerreadmodel.RemoteShipmentRisk, shipmentNumbers map[string]string) ControlTowerShipmentRisk {
	item := ControlTowerShipmentRisk{
		RiskID:                 remote.RiskID,
		ShipmentID:             remote.ShipmentID,
		ShipmentNumber:         shipmentNumbers[remote.ShipmentID],
		PredictedExceptionType: remote.PredictedExceptionType,
		Score:                  remote.Score,
		Level:                  remote.Level,
		Status:                 remote.Status,
		MitigationCode:         remote.MitigationCode,
		MitigationComment:      remote.MitigationComment,
		ActualEventID:          remote.ActualEventID,
		ClearReason:            remote.ClearReason,
	}
	if item.ShipmentNumber == "" {
		item.ShipmentNumber = remote.ShipmentID
	}
	if t, err := time.Parse(time.RFC3339, remote.FirstDetectedAt); err == nil {
		item.FirstDetectedAt = t.UTC()
	}
	if t, err := time.Parse(time.RFC3339, remote.EvaluatedAt); err == nil {
		item.EvaluatedAt = t.UTC()
	}
	item.NextEvaluationAt = parseOptionalTime(remote.NextEvaluationAt)
	item.ThreatenedDeadlineAt = parseOptionalTime(remote.ThreatenedDeadlineAt)
	item.ClearedAt = parseOptionalTime(remote.ClearedAt)
	item.MaterializedAt = parseOptionalTime(remote.MaterializedAt)
	if item.MaterializedAt != nil && !item.FirstDetectedAt.IsZero() {
		seconds := int64(item.MaterializedAt.Sub(item.FirstDetectedAt).Seconds())
		if seconds >= 0 {
			item.LeadTimeSeconds = &seconds
		}
	}
	for _, sig := range remote.Signals {
		observedAt, _ := time.Parse(time.RFC3339, sig.ObservedAt)
		item.Signals = append(item.Signals, ControlTowerRiskSignal{
			SignalCode:     sig.SignalCode,
			Severity:       sig.Severity,
			Weight:         sig.Weight,
			ObservedAt:     observedAt.UTC(),
			Source:         sig.Source,
			Value:          sig.Value,
			ExplanationKey: sig.ExplanationKey,
		})
	}
	sort.SliceStable(item.Signals, func(i, j int) bool {
		return item.Signals[i].Weight > item.Signals[j].Weight
	})
	for _, action := range remote.Actions {
		occurredAt, _ := time.Parse(time.RFC3339, action.OccurredAt)
		item.Actions = append(item.Actions, ControlTowerRiskAction{
			ActionType:  action.ActionType,
			ActorUserID: action.ActorUserID,
			OccurredAt:  occurredAt.UTC(),
			Metadata:    action.Metadata,
		})
	}
	return item
}

func parseOptionalTime(raw *string) *time.Time {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, *raw)
	if err != nil {
		return nil
	}
	utc := parsed.UTC()
	return &utc
}

// SortShipmentRisks orders risks: critical > high > medium > low, then deadline proximity, score, stable id.
func SortShipmentRisks(items []ControlTowerShipmentRisk) {
	sort.SliceStable(items, func(i, j int) bool {
		lr := riskLevelRank(items[i].Level) - riskLevelRank(items[j].Level)
		if lr != 0 {
			return lr < 0
		}
		aDeadline := deadlineUnix(items[i].ThreatenedDeadlineAt)
		bDeadline := deadlineUnix(items[j].ThreatenedDeadlineAt)
		if aDeadline != bDeadline {
			return aDeadline < bDeadline
		}
		if items[i].Score != items[j].Score {
			return items[i].Score > items[j].Score
		}
		return items[i].RiskID < items[j].RiskID
	})
}

func riskLevelRank(level string) int {
	switch level {
	case risk.LevelCritical:
		return 1
	case risk.LevelHigh:
		return 2
	case risk.LevelMedium:
		return 3
	case risk.LevelLow:
		return 4
	default:
		return 5
	}
}

func deadlineUnix(t *time.Time) int64 {
	if t == nil {
		return 1 << 62
	}
	return t.UTC().Unix()
}

func (s *Service) ListRisks(ctx context.Context, reqCtx RequestContext, query ListQuery) (RisksListResponse, error) {
	shipmentsRaw, _, err := s.client.FetchShipments(ctx, reqCtx)
	if err != nil {
		return RisksListResponse{}, apperrors.ControlTowerShipmentsUnavailable("required shipment data is temporarily unavailable")
	}
	shipmentNumbers := map[string]string{}
	for _, raw := range shipmentsRaw {
		shipmentNumbers[raw.ID] = raw.ShipmentNumber
	}
	items, _ := s.loadShipmentRisks(ctx, reqCtx, query, shipmentNumbers)
	return RisksListResponse{Items: items}, nil
}

func (s *Service) GetRisk(ctx context.Context, reqCtx RequestContext, riskID string) (ControlTowerShipmentRisk, error) {
	if !s.readModelCfg.Mode.Enabled() || s.readModel == nil {
		return ControlTowerShipmentRisk{}, apperrors.NotFound("risk not found")
	}

	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.readModelCfg.Timeout)
	defer cancel()

	remote, depErr := s.readModel.GetRisk(rmCtx, reqCtx.TenantID, reqCtx.RequestID, riskID)
	if depErr != nil {
		if depErr.Status == http.StatusNotFound {
			return ControlTowerShipmentRisk{}, apperrors.NotFound("risk not found")
		}
		return ControlTowerShipmentRisk{}, mapRiskDependencyError(depErr)
	}

	shipmentNumbers := map[string]string{}
	if shipmentsRaw, _, err := s.client.FetchShipments(ctx, reqCtx); err == nil {
		for _, raw := range shipmentsRaw {
			shipmentNumbers[raw.ID] = raw.ShipmentNumber
		}
	}
	return mapRemoteShipmentRisk(*remote, shipmentNumbers), nil
}

func (s *Service) AcknowledgeRisk(ctx context.Context, reqCtx RequestContext, riskID string) (ControlTowerShipmentRisk, error) {
	if !s.readModelCfg.Mode.Enabled() || s.readModel == nil {
		return ControlTowerShipmentRisk{}, apperrors.ControlTowerReadModelUnavailable("risk service is temporarily unavailable")
	}

	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.readModelCfg.Timeout)
	defer cancel()

	remote, depErr := s.readModel.AcknowledgeRisk(rmCtx, controltowerreadmodel.AcknowledgeRiskInput{
		TenantID:  reqCtx.TenantID,
		UserID:    reqCtx.UserID,
		RequestID: reqCtx.RequestID,
		RiskKey:   riskID,
	})
	if depErr != nil {
		return ControlTowerShipmentRisk{}, mapRiskDependencyError(depErr)
	}
	return mapRemoteShipmentRisk(*remote, nil), nil
}

func (s *Service) MitigateRisk(ctx context.Context, reqCtx RequestContext, riskID string, rawBody []byte) (ControlTowerShipmentRisk, error) {
	if !s.readModelCfg.Mode.Enabled() || s.readModel == nil {
		return ControlTowerShipmentRisk{}, apperrors.ControlTowerReadModelUnavailable("risk service is temporarily unavailable")
	}

	var payload mitigateRiskRequestPayload
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return ControlTowerShipmentRisk{}, apperrors.Validation("invalid request body", map[string]any{"field": "body"})
	}
	code := strings.TrimSpace(payload.MitigationCode)
	if code == "" {
		return ControlTowerShipmentRisk{}, apperrors.Validation("mitigationCode is required", map[string]any{"field": "mitigationCode"})
	}
	if !risk.ValidMitigationCode(code) {
		return ControlTowerShipmentRisk{}, apperrors.Validation("unknown mitigationCode", map[string]any{"field": "mitigationCode"})
	}

	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.readModelCfg.Timeout)
	defer cancel()

	remote, depErr := s.readModel.MitigateRisk(rmCtx, controltowerreadmodel.MitigateRiskInput{
		TenantID:          reqCtx.TenantID,
		UserID:            reqCtx.UserID,
		RequestID:         reqCtx.RequestID,
		RiskKey:           riskID,
		MitigationCode:    code,
		MitigationComment: payload.Comment,
	})
	if depErr != nil {
		return ControlTowerShipmentRisk{}, mapRiskDependencyError(depErr)
	}
	return mapRemoteShipmentRisk(*remote, nil), nil
}

func readRiskRequestBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	defer r.Body.Close()
	return io.ReadAll(io.LimitReader(r.Body, 16*1024))
}

func mapRiskDependencyError(depErr *controltowerreadmodel.DependencyError) error {
	if depErr == nil {
		return apperrors.Internal("risk service error", nil)
	}
	if depErr.Status == http.StatusNotFound {
		return apperrors.NotFound("risk not found")
	}
	if depErr.Status == http.StatusConflict {
		return apperrors.Conflict("invalid risk transition", map[string]any{})
	}
	return apperrors.ControlTowerReadModelUnavailable("risk service is temporarily unavailable")
}
