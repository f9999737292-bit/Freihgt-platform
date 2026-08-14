package domain

import (
	"time"

	"github.com/google/uuid"
)

const (
	TargetPickup   = "pickup"
	TargetDelivery = "delivery"

	ETASourceProviderETA    = "provider_eta"
	ETASourceCarrierETA     = "carrier_eta"
	ETASourceDriverETA      = "driver_eta"
	ETASourceManualOperator = "manual_operator"
	ETASourceCalculated     = "calculated"

	ETAStatusUnavailable = "unavailable"
	ETAStatusAvailable   = "available"
	ETAStatusStale       = "stale"
	ETAStatusExpired     = "expired"
	ETAStatusCompleted   = "completed"

	ETAFreshnessUnknown = "unknown"
	ETAFreshnessFresh   = "fresh"
	ETAFreshnessStale   = "stale"
	ETAFreshnessExpired = "expired"

	ETAQualityUnknown  = "unknown"
	ETAQualityGood     = "good"
	ETAQualityDegraded = "degraded"
	ETAQualityPoor     = "poor"

	ArrivalEarly    = "early"
	ArrivalOnTime   = "on_time"
	ArrivalAtRisk   = "at_risk"
	ArrivalLate     = "late"
	ArrivalUnknown  = "unknown"

	MaxETAHistoryLimit = 200

	TransitionETABecameAvailable = "eta_became_available"
	TransitionETABecameStale     = "eta_became_stale"
	TransitionETAExpired         = "eta_expired"
	TransitionETARestored        = "eta_restored"
	TransitionETACompleted       = "eta_completed"
	TransitionETASourceChanged   = "eta_source_changed"
)

type ETAObservation struct {
	ID                  uuid.UUID
	TenantID            uuid.UUID
	ShipmentID          uuid.UUID
	TargetType          string
	TargetReference     *string
	EstimatedArrivalAt  time.Time
	SourceType          string
	ProviderCode        *string
	ProviderEventID     *string
	DedupKey            string
	SourceObservedAt    time.Time
	ReceivedAt          time.Time
	QualityStatus       string
	QualityReasons      []string
	ProviderConfidence  *float64
	CreatedAt           time.Time
}

type ShipmentETAState struct {
	TenantID           uuid.UUID
	ShipmentID         uuid.UUID
	TargetType         string
	Status             string
	EstimatedArrivalAt *time.Time
	SourceType         *string
	ProviderCode       *string
	SourceObservedAt   *time.Time
	ReceivedAt         *time.Time
	FreshnessStatus    string
	QualityStatus      string
	AgeSeconds         *int64
	DeliveryLagSeconds *int64
	Version            int64
	UpdatedAt          time.Time
}

type ETATargetSummary struct {
	Status                  string
	EstimatedArrivalAt      *time.Time
	SourceType              *string
	Provider                *string
	SourceObservedAt        *time.Time
	ReceivedAt              *time.Time
	AgeSeconds              *int64
	FreshnessStatus         string
	QualityStatus           string
	QualityReasons          []string
	ProviderConfidence      *float64
	DeliveryLagSeconds      *int64
	PlannedArrivalAt        *time.Time
	ProjectedDeviationSeconds *int64
	ArrivalProjection       string
}

type ShipmentETASummary struct {
	ShipmentID uuid.UUID
	Delivery   *ETATargetSummary
	Pickup     *ETATargetSummary
}

type ETAStateTransition struct {
	ID             uuid.UUID
	TenantID       uuid.UUID
	ShipmentID     uuid.UUID
	TargetType     string
	TransitionType string
	FromStatus     *string
	ToStatus       string
	Metadata       map[string]any
	OccurredAt     time.Time
}

func SourcePriority(sourceType string) int {
	switch sourceType {
	case ETASourceProviderETA:
		return 100
	case ETASourceCarrierETA:
		return 90
	case ETASourceDriverETA:
		return 80
	case ETASourceManualOperator:
		return 70
	case ETASourceCalculated:
		return 60
	default:
		return 0
	}
}

func IsEnabledSourceType(sourceType string) bool {
	switch sourceType {
	case ETASourceProviderETA, ETASourceCarrierETA, ETASourceDriverETA, ETASourceManualOperator:
		return true
	default:
		return false
	}
}
