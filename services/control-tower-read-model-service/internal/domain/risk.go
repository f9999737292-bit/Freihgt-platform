package domain

import (
	"time"

	"github.com/google/uuid"
)

const (
	RiskLevelNone     = "none"
	RiskLevelLow      = "low"
	RiskLevelMedium   = "medium"
	RiskLevelHigh     = "high"
	RiskLevelCritical = "critical"

	RiskStatusActive       = "active"
	RiskStatusAcknowledged = "acknowledged"
	RiskStatusMitigating   = "mitigating"
	RiskStatusCleared      = "cleared"
	RiskStatusMaterialized = "materialized"

	ActionRiskAcknowledged  = "risk_acknowledged"
	ActionMitigationStarted = "mitigation_started"
	ActionMitigationUpdated = "mitigation_updated"
	ActionRiskCleared       = "risk_cleared"
	ActionRiskMaterialized  = "risk_materialized"

	SystemActorUUID = "00000000-0000-0000-0000-000000000001"
)

type RiskSignal struct {
	Code           string
	Severity       string
	Weight         int
	ObservedAt     time.Time
	Source         string
	ValueJSON      map[string]any
	ExplanationKey string
}

type ShipmentRisk struct {
	ID                     uuid.UUID
	TenantID               uuid.UUID
	RiskKey                string
	ShipmentID             uuid.UUID
	PredictedExceptionType string
	Score                  int
	RiskLevel              string
	Status                 string
	FirstDetectedAt        time.Time
	EvaluatedAt            time.Time
	NextEvaluationAt       *time.Time
	ThreatenedDeadlineAt   *time.Time
	ClearedAt              *time.Time
	ClearReason            *string
	MaterializedAt         *time.Time
	ActualEventID          *string
	MitigationCode         *string
	MitigationComment      *string
	AcknowledgedAt         *time.Time
	AcknowledgedByUserID   *uuid.UUID
	MitigatingAt           *time.Time
	MitigatingByUserID     *uuid.UUID
	Version                int
}

type RiskAssessmentSnapshot struct {
	ID                     uuid.UUID
	ShipmentRiskID         uuid.UUID
	ShipmentID             uuid.UUID
	PredictedExceptionType string
	Score                  int
	RiskLevel              string
	Status                 string
	EvaluatedAt            time.Time
	SignalsHash            string
	Signals                []RiskSignal
}

type RiskAction struct {
	ActionType  string
	ActorUserID *uuid.UUID
	OccurredAt  time.Time
	Metadata    map[string]any
}

type SyncRiskEvaluation struct {
	RiskKey                string
	ShipmentID             string
	PredictedExceptionType string
	Score                  int
	RiskLevel              string
	EvaluatedAt            time.Time
	NextEvaluationAt       time.Time
	ThreatenedDeadlineAt   *time.Time
	SignalsHash            string
	Signals                []RiskSignal
}

type MaterializeRiskInput struct {
	RiskKey        string
	ShipmentID     string
	PredictedType  string
	ActualEventID  string
	MaterializedAt time.Time
}

type ClearRiskInput struct {
	RiskKey     string
	ShipmentID  string
	ClearReason string
	ClearedAt   time.Time
}

type AcknowledgeRiskInput struct {
	TenantID    uuid.UUID
	ActorUserID uuid.UUID
	RiskKey     string
}

type MitigateRiskInput struct {
	TenantID          uuid.UUID
	ActorUserID       uuid.UUID
	RiskKey           string
	MitigationCode    string
	MitigationComment *string
}

func ScoreBand(level string) string {
	return level
}

func MeaningfulRiskChange(prev ShipmentRisk, eval SyncRiskEvaluation) bool {
	if prev.RiskLevel != eval.RiskLevel {
		return true
	}
	if prev.PredictedExceptionType != eval.PredictedExceptionType {
		return true
	}
	if abs(prev.Score-eval.Score) >= 10 {
		return true
	}
	return false
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
