package risk

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	LevelNone     = "none"
	LevelLow      = "low"
	LevelMedium   = "medium"
	LevelHigh     = "high"
	LevelCritical = "critical"

	StatusActive       = "active"
	StatusAcknowledged = "acknowledged"
	StatusMitigating   = "mitigating"
	StatusCleared      = "cleared"
	StatusMaterialized = "materialized"

	TypePickupDelayRisk       = "pickup_delay_risk"
	TypeDeliveryDelayRisk     = "delivery_delay_risk"
	TypeSlotMissRisk          = "slot_miss_risk"
	TypeTrackingLossRisk      = "tracking_loss_risk"
	TypeDocumentReadinessRisk = "document_readiness_risk"
	TypeVehicleAssignmentRisk = "vehicle_assignment_risk"
	TypeDriverAssignmentRisk  = "driver_assignment_risk"

	SourceControlTower = "control-tower"
)

var validMitigationCodes = map[string]struct{}{
	"contact_carrier":   {},
	"contact_driver":    {},
	"reschedule_slot":   {},
	"request_documents": {},
	"reassign_driver":   {},
	"reassign_vehicle":  {},
	"adjust_plan":       {},
	"monitor":           {},
	"other":             {},
}

func ValidMitigationCode(code string) bool {
	_, ok := validMitigationCodes[strings.TrimSpace(code)]
	return ok
}

type Signal struct {
	Code           string         `json:"signalCode"`
	Severity       string         `json:"severity"`
	Weight         int            `json:"weight"`
	ObservedAt     time.Time      `json:"observedAt"`
	Source         string         `json:"source"`
	Value          map[string]any `json:"value,omitempty"`
	ExplanationKey string         `json:"explanationKey"`
}

type Assessment struct {
	RiskID                 string     `json:"riskId"`
	ShipmentID             string     `json:"shipmentId"`
	ShipmentNumber         string     `json:"shipmentNumber"`
	PredictedExceptionType string     `json:"predictedExceptionType"`
	Score                  int        `json:"score"`
	Level                  string     `json:"level"`
	Signals                []Signal   `json:"signals"`
	EvaluatedAt            time.Time  `json:"evaluatedAt"`
	NextEvaluationAt       time.Time  `json:"nextEvaluationAt"`
	ThreatenedDeadlineAt   *time.Time `json:"threatenedDeadlineAt,omitempty"`
}

type ShipmentInput struct {
	ID                string
	ShipmentNumber    string
	Status            string
	PlannedPickupAt   *time.Time
	PlannedDeliveryAt *time.Time
	ActualPickupAt    *time.Time
	ActualDeliveryAt  *time.Time
	LastUpdatedAt     *time.Time
	DriverID          *string
	VehicleID         *string
	DocumentsComplete bool
	SLAStatus         string
	SLAReason         *string
	Telemetry         *TelemetryContext
}

type TelemetryContext struct {
	HasBinding       bool
	TrackingStatus   string
	LastRecordedAt   *time.Time
	FreshnessStatus  string
	QualityStatus    string
	TelemetryAgeSecs *int64
}

type Thresholds struct {
	AtRiskMinutes        int
	StaleWarningMinutes  int
	StaleCriticalMinutes int
}

type Evaluator struct {
	Thresholds Thresholds
	Now        time.Time
}

func NewEvaluator(thresholds Thresholds, now time.Time) Evaluator {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if thresholds.AtRiskMinutes <= 0 {
		thresholds.AtRiskMinutes = 120
	}
	if thresholds.StaleWarningMinutes <= 0 {
		thresholds.StaleWarningMinutes = 120
	}
	if thresholds.StaleCriticalMinutes <= 0 {
		thresholds.StaleCriticalMinutes = 360
	}
	return Evaluator{Thresholds: thresholds, Now: now.UTC()}
}

func (e Evaluator) EvaluateShipment(shipment ShipmentInput) []Assessment {
	if shipment.Status == "CANCELLED" || shipment.Status == "FINANCIALLY_CLOSED" {
		return nil
	}

	candidates := []Assessment{
		e.evaluatePickupDelayRisk(shipment),
		e.evaluateDeliveryDelayRisk(shipment),
		e.evaluateSlotMissRisk(shipment),
		e.evaluateTrackingLossRisk(shipment),
		e.evaluateDriverAssignmentRisk(shipment),
		e.evaluateVehicleAssignmentRisk(shipment),
	}

	out := make([]Assessment, 0, len(candidates))
	for _, item := range candidates {
		if item.Score >= 20 {
			out = append(out, item)
		}
	}
	return out
}

func (e Evaluator) EvaluateAll(shipments []ShipmentInput) []Assessment {
	all := make([]Assessment, 0)
	for _, shipment := range shipments {
		all = append(all, e.EvaluateShipment(shipment)...)
	}
	return all
}

func LevelFromScore(score int) string {
	switch {
	case score >= 80:
		return LevelCritical
	case score >= 60:
		return LevelHigh
	case score >= 40:
		return LevelMedium
	case score >= 20:
		return LevelLow
	default:
		return LevelNone
	}
}

func DeterministicRiskID(shipmentID, predictedType string) string {
	raw := fmt.Sprintf("%s:%s", strings.ToLower(shipmentID), predictedType)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:16])
}

func SignalsHash(signals []Signal) string {
	codes := make([]string, 0, len(signals))
	for _, s := range signals {
		codes = append(codes, fmt.Sprintf("%s:%d", s.Code, s.Weight))
	}
	sort.Strings(codes)
	sum := sha256.Sum256([]byte(strings.Join(codes, "|")))
	return hex.EncodeToString(sum[:8])
}

func buildAssessment(shipment ShipmentInput, predictedType string, signals []Signal, deadline *time.Time, now time.Time) Assessment {
	score := sumSignalWeights(signals)
	if score > 100 {
		score = 100
	}
	evaluatedAt := now.UTC()
	nextEval := evaluatedAt.Add(5 * time.Minute)
	return Assessment{
		RiskID:                 DeterministicRiskID(shipment.ID, predictedType),
		ShipmentID:             shipment.ID,
		ShipmentNumber:         shipment.ShipmentNumber,
		PredictedExceptionType: predictedType,
		Score:                  score,
		Level:                  LevelFromScore(score),
		Signals:                signals,
		EvaluatedAt:            evaluatedAt,
		NextEvaluationAt:       nextEval,
		ThreatenedDeadlineAt:   deadline,
	}
}

func sumSignalWeights(signals []Signal) int {
	total := 0
	for _, s := range signals {
		total += s.Weight
	}
	return total
}

func minutesUntil(deadline, now time.Time) int64 {
	if deadline.Before(now) {
		return 0
	}
	return int64(deadline.Sub(now).Minutes())
}

func isPrePickupStatus(status string) bool {
	switch status {
	case "CARRIER_ASSIGNED", "ACCEPTED_BY_CARRIER", "VEHICLE_ASSIGNED", "DRIVER_ASSIGNED":
		return true
	default:
		return false
	}
}

func isPreTransitPickup(status string) bool {
	return isPrePickupStatus(status) || status == "PICKUP_SLOT_BOOKED"
}

func isInTransit(status string) bool {
	switch status {
	case "IN_PICKUP", "LOADED", "IN_TRANSIT", "ARRIVED_AT_CONSIGNEE", "UNLOADING":
		return true
	default:
		return false
	}
}

func isDelivered(status string) bool {
	switch status {
	case "DELIVERED", "DELIVERY_CONFIRMED", "DOCUMENTS_COMPLETED", "READY_FOR_BILLING",
		"INCLUDED_IN_BILLING_REGISTER", "FINANCIALLY_CLOSED":
		return true
	default:
		return false
	}
}

func requiresDriver(status string) bool {
	switch status {
	case "DRIVER_ASSIGNED", "PICKUP_SLOT_BOOKED", "IN_PICKUP", "LOADED", "IN_TRANSIT",
		"ARRIVED_AT_CONSIGNEE", "UNLOADING", "DELIVERED", "DELIVERY_CONFIRMED",
		"DOCUMENTS_COMPLETED", "READY_FOR_BILLING", "INCLUDED_IN_BILLING_REGISTER", "FINANCIALLY_CLOSED":
		return true
	default:
		return false
	}
}

func requiresVehicle(status string) bool {
	switch status {
	case "VEHICLE_ASSIGNED", "DRIVER_ASSIGNED", "PICKUP_SLOT_BOOKED", "IN_PICKUP", "LOADED",
		"IN_TRANSIT", "ARRIVED_AT_CONSIGNEE", "UNLOADING", "DELIVERED", "DELIVERY_CONFIRMED",
		"DOCUMENTS_COMPLETED", "READY_FOR_BILLING", "INCLUDED_IN_BILLING_REGISTER", "FINANCIALLY_CLOSED":
		return true
	default:
		return false
	}
}

func (e Evaluator) evaluatePickupDelayRisk(s ShipmentInput) Assessment {
	signals := make([]Signal, 0)
	if s.ActualPickupAt != nil || s.PlannedPickupAt == nil {
		return buildAssessment(s, TypePickupDelayRisk, signals, s.PlannedPickupAt, e.Now)
	}
	if s.SLAReason != nil && *s.SLAReason == "PICKUP_OVERDUE" {
		return buildAssessment(s, TypePickupDelayRisk, signals, s.PlannedPickupAt, e.Now)
	}
	if !isPreTransitPickup(s.Status) && s.Status != "IN_PICKUP" {
		return buildAssessment(s, TypePickupDelayRisk, signals, s.PlannedPickupAt, e.Now)
	}
	if s.ActualPickupAt == nil && isPreTransitPickup(s.Status) {
		mins := minutesUntil(*s.PlannedPickupAt, e.Now)
		if mins <= 30 {
			signals = append(signals, Signal{
				Code: "pickup_deadline_imminent", Severity: "high", Weight: 55,
				ObservedAt: e.Now, Source: SourceControlTower,
				Value:          map[string]any{"minutesRemaining": mins},
				ExplanationKey: "controlTower.risk.signals.pickup_deadline_imminent",
			})
		} else if mins <= 60 {
			signals = append(signals, Signal{
				Code: "pickup_time_near_without_started_state", Severity: "high", Weight: 45,
				ObservedAt: e.Now, Source: SourceControlTower,
				Value:          map[string]any{"minutesRemaining": mins},
				ExplanationKey: "controlTower.risk.signals.pickup_time_near_without_started_state",
			})
		} else if mins <= 120 {
			signals = append(signals, Signal{
				Code: "pickup_window_approaching", Severity: "medium", Weight: 30,
				ObservedAt: e.Now, Source: SourceControlTower,
				Value:          map[string]any{"minutesRemaining": mins},
				ExplanationKey: "controlTower.risk.signals.pickup_window_approaching",
			})
		}
		if s.SLAReason != nil && *s.SLAReason == "PICKUP_AT_RISK" && len(signals) == 0 {
			signals = append(signals, Signal{
				Code: "pickup_at_risk_sla", Severity: "medium", Weight: 35,
				ObservedAt: e.Now, Source: SourceControlTower,
				ExplanationKey: "controlTower.risk.signals.pickup_at_risk_sla",
			})
		}
	}
	return buildAssessment(s, TypePickupDelayRisk, signals, s.PlannedPickupAt, e.Now)
}

func (e Evaluator) evaluateDeliveryDelayRisk(s ShipmentInput) Assessment {
	signals := make([]Signal, 0)
	if s.ActualDeliveryAt != nil || s.PlannedDeliveryAt == nil || isDelivered(s.Status) {
		return buildAssessment(s, TypeDeliveryDelayRisk, signals, s.PlannedDeliveryAt, e.Now)
	}
	if s.SLAReason != nil && *s.SLAReason == "DELIVERY_OVERDUE" {
		return buildAssessment(s, TypeDeliveryDelayRisk, signals, s.PlannedDeliveryAt, e.Now)
	}
	mins := minutesUntil(*s.PlannedDeliveryAt, e.Now)
	if mins <= 30 && isInTransit(s.Status) {
		signals = append(signals, Signal{
			Code: "delivery_deadline_imminent", Severity: "high", Weight: 50,
			ObservedAt: e.Now, Source: SourceControlTower,
			Value:          map[string]any{"minutesRemaining": mins},
			ExplanationKey: "controlTower.risk.signals.delivery_deadline_imminent",
		})
	} else if mins <= 60 && isInTransit(s.Status) {
		signals = append(signals, Signal{
			Code: "delivery_time_near_without_completion", Severity: "high", Weight: 40,
			ObservedAt: e.Now, Source: SourceControlTower,
			Value:          map[string]any{"minutesRemaining": mins},
			ExplanationKey: "controlTower.risk.signals.delivery_time_near_without_completion",
		})
	} else if mins <= 120 && (isInTransit(s.Status) || s.Status == "LOADED") {
		signals = append(signals, Signal{
			Code: "delivery_window_approaching", Severity: "medium", Weight: 28,
			ObservedAt: e.Now, Source: SourceControlTower,
			Value:          map[string]any{"minutesRemaining": mins},
			ExplanationKey: "controlTower.risk.signals.delivery_window_approaching",
		})
	}
	if s.SLAReason != nil && *s.SLAReason == "DELIVERY_AT_RISK" && len(signals) == 0 {
		signals = append(signals, Signal{
			Code: "delivery_at_risk_sla", Severity: "medium", Weight: 32,
			ObservedAt: e.Now, Source: SourceControlTower,
			ExplanationKey: "controlTower.risk.signals.delivery_at_risk_sla",
		})
	}
	return buildAssessment(s, TypeDeliveryDelayRisk, signals, s.PlannedDeliveryAt, e.Now)
}

func (e Evaluator) evaluateSlotMissRisk(s ShipmentInput) Assessment {
	signals := make([]Signal, 0)
	if s.PlannedPickupAt != nil && isPrePickupStatus(s.Status) && s.Status != "PICKUP_SLOT_BOOKED" {
		mins := minutesUntil(*s.PlannedPickupAt, e.Now)
		if mins > 0 && mins <= 120 {
			signals = append(signals, Signal{
				Code: "slot_window_near", Severity: "medium", Weight: 35,
				ObservedAt: e.Now, Source: SourceControlTower,
				Value:          map[string]any{"minutesRemaining": mins, "slotPhase": "pickup"},
				ExplanationKey: "controlTower.risk.signals.pickup_slot_not_booked",
			})
		}
	}
	if s.PlannedDeliveryAt != nil && isInTransit(s.Status) && s.Status != "DELIVERY_SLOT_BOOKED" {
		mins := minutesUntil(*s.PlannedDeliveryAt, e.Now)
		if mins > 0 && mins <= 120 {
			signals = append(signals, Signal{
				Code: "slot_window_near", Severity: "medium", Weight: 30,
				ObservedAt: e.Now, Source: SourceControlTower,
				Value:          map[string]any{"minutesRemaining": mins, "slotPhase": "delivery"},
				ExplanationKey: "controlTower.risk.signals.delivery_slot_not_booked",
			})
		}
	}
	deadline := s.PlannedPickupAt
	if s.PlannedDeliveryAt != nil && (deadline == nil || s.PlannedDeliveryAt.Before(*deadline)) {
		deadline = s.PlannedDeliveryAt
	}
	return buildAssessment(s, TypeSlotMissRisk, signals, deadline, e.Now)
}

func (e Evaluator) evaluateTrackingLossRisk(s ShipmentInput) Assessment {
	signals := make([]Signal, 0)
	if !isInTransit(s.Status) {
		return buildAssessment(s, TypeTrackingLossRisk, signals, nil, e.Now)
	}

	if s.Telemetry == nil || !s.Telemetry.HasBinding {
		return buildAssessment(s, TypeTrackingLossRisk, signals, nil, e.Now)
	}

	telemetry := *s.Telemetry
	switch telemetry.TrackingStatus {
	case TrackingStatusNotConfigured, TrackingStatusEnded:
		return buildAssessment(s, TypeTrackingLossRisk, signals, nil, e.Now)
	case TrackingStatusAwaitingData:
		if isInTransit(s.Status) {
			signals = append(signals, Signal{
				Code: "telemetry_awaiting_data", Severity: "low", Weight: 15,
				ObservedAt: e.Now, Source: SourceControlTower,
				ExplanationKey: "controlTower.risk.signals.telemetry_awaiting_data",
			})
		}
	case TrackingStatusStale:
		age := int64(0)
		if telemetry.TelemetryAgeSecs != nil {
			age = *telemetry.TelemetryAgeSecs
		}
		signals = append(signals, Signal{
			Code: "telemetry_stale", Severity: "medium", Weight: 30,
			ObservedAt: e.Now, Source: SourceControlTower,
			Value:          map[string]any{"telemetryAgeSeconds": age},
			ExplanationKey: "controlTower.risk.signals.telemetry_stale",
		})
	case TrackingStatusLost:
		age := int64(0)
		if telemetry.TelemetryAgeSecs != nil {
			age = *telemetry.TelemetryAgeSecs
		}
		signals = append(signals, Signal{
			Code: "telemetry_lost", Severity: "high", Weight: 45,
			ObservedAt: e.Now, Source: SourceControlTower,
			Value:          map[string]any{"telemetryAgeSeconds": age},
			ExplanationKey: "controlTower.risk.signals.telemetry_lost",
		})
	}

	if telemetry.QualityStatus == "degraded" || telemetry.QualityStatus == "poor" {
		signals = append(signals, Signal{
			Code: "telemetry_quality_degraded", Severity: "medium", Weight: 20,
			ObservedAt: e.Now, Source: SourceControlTower,
			Value:          map[string]any{"qualityStatus": telemetry.QualityStatus},
			ExplanationKey: "controlTower.risk.signals.telemetry_quality_degraded",
		})
	}

	return buildAssessment(s, TypeTrackingLossRisk, signals, nil, e.Now)
}

const (
	TrackingStatusNotConfigured = "not_configured"
	TrackingStatusAwaitingData  = "awaiting_data"
	TrackingStatusActive        = "active"
	TrackingStatusStale         = "stale"
	TrackingStatusLost          = "lost"
	TrackingStatusEnded         = "ended"
)

func (e Evaluator) evaluateDriverAssignmentRisk(s ShipmentInput) Assessment {
	signals := make([]Signal, 0)
	if s.DriverID != nil && *s.DriverID != "" {
		return buildAssessment(s, TypeDriverAssignmentRisk, signals, s.PlannedPickupAt, e.Now)
	}
	if !requiresDriver(s.Status) && !isPrePickupStatus(s.Status) {
		return buildAssessment(s, TypeDriverAssignmentRisk, signals, s.PlannedPickupAt, e.Now)
	}
	deadline := s.PlannedPickupAt
	if deadline == nil {
		return buildAssessment(s, TypeDriverAssignmentRisk, signals, deadline, e.Now)
	}
	mins := minutesUntil(*deadline, e.Now)
	if mins > 0 && mins <= 240 {
		weight := 25
		if mins <= 60 {
			weight = 45
		} else if mins <= 120 {
			weight = 35
		}
		signals = append(signals, Signal{
			Code: "missing_driver", Severity: "high", Weight: weight,
			ObservedAt: e.Now, Source: SourceControlTower,
			Value:          map[string]any{"minutesUntilPickup": mins},
			ExplanationKey: "controlTower.risk.signals.missing_driver",
		})
	}
	return buildAssessment(s, TypeDriverAssignmentRisk, signals, deadline, e.Now)
}

func (e Evaluator) evaluateVehicleAssignmentRisk(s ShipmentInput) Assessment {
	signals := make([]Signal, 0)
	if s.VehicleID != nil && *s.VehicleID != "" {
		return buildAssessment(s, TypeVehicleAssignmentRisk, signals, s.PlannedPickupAt, e.Now)
	}
	if !requiresVehicle(s.Status) && s.Status != "CARRIER_ASSIGNED" && s.Status != "ACCEPTED_BY_CARRIER" {
		return buildAssessment(s, TypeVehicleAssignmentRisk, signals, s.PlannedPickupAt, e.Now)
	}
	deadline := s.PlannedPickupAt
	if deadline == nil {
		return buildAssessment(s, TypeVehicleAssignmentRisk, signals, deadline, e.Now)
	}
	mins := minutesUntil(*deadline, e.Now)
	if mins > 0 && mins <= 240 {
		weight := 22
		if mins <= 60 {
			weight = 42
		} else if mins <= 120 {
			weight = 32
		}
		signals = append(signals, Signal{
			Code: "missing_vehicle", Severity: "high", Weight: weight,
			ObservedAt: e.Now, Source: SourceControlTower,
			Value:          map[string]any{"minutesUntilPickup": mins},
			ExplanationKey: "controlTower.risk.signals.missing_vehicle",
		})
	}
	return buildAssessment(s, TypeVehicleAssignmentRisk, signals, deadline, e.Now)
}

// MaterializationMap links actual critical event types to predictive risk types.
var MaterializationMap = map[string]string{
	"PICKUP_DELAY":      TypePickupDelayRisk,
	"DELIVERY_DELAY":    TypeDeliveryDelayRisk,
	"STALE_UPDATES":     TypeTrackingLossRisk,
	"MISSING_DOCUMENTS": TypeDocumentReadinessRisk,
}

func SortAssessments(items []Assessment) {
	sort.SliceStable(items, func(i, j int) bool {
		lr := levelRank(items[i].Level) - levelRank(items[j].Level)
		if lr != 0 {
			return lr < 0
		}
		if items[i].Score != items[j].Score {
			return items[i].Score > items[j].Score
		}
		aDeadline := deadlineUnix(items[i].ThreatenedDeadlineAt)
		bDeadline := deadlineUnix(items[j].ThreatenedDeadlineAt)
		if aDeadline != bDeadline {
			return aDeadline < bDeadline
		}
		return items[i].RiskID < items[j].RiskID
	})
}

func levelRank(level string) int {
	switch level {
	case LevelCritical:
		return 1
	case LevelHigh:
		return 2
	case LevelMedium:
		return 3
	case LevelLow:
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
