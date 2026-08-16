package domain

import (
	"time"

	"github.com/google/uuid"
)

const (
	SlotTypePickup   = "pickup"
	SlotTypeDelivery = "delivery"

	SlotSourceInternalBooking = "internal_booking"
	SlotSourceWarehouseAPI    = "warehouse_api"
	SlotSourceCarrierAPI      = "carrier_api"
	SlotSourceShipperAPI      = "shipper_api"
	SlotSourceManualOperator  = "manual_operator"
	SlotSourceSystemImport    = "system_import"

	SlotStatusProposed  = "proposed"
	SlotStatusBooked    = "booked"
	SlotStatusConfirmed = "confirmed"
	SlotStatusCancelled = "cancelled"
	SlotStatusCompleted = "completed"
	SlotStatusMissed    = "missed"

	SlotWindowUnavailable = "unavailable"
	SlotWindowAvailable   = "available"

	SlotQualityUnknown  = "unknown"
	SlotQualityGood     = "good"
	SlotQualityDegraded = "degraded"

	SlotArrivalUnknown       = "unknown"
	SlotArrivalEarly         = "early"
	SlotArrivalOnTime        = "on_time"
	SlotArrivalAtRisk        = "at_risk"
	SlotArrivalProjectedMiss = "projected_miss"
	SlotArrivalMissed        = "missed"
	SlotArrivalCompleted     = "completed"

	MaxSlotHistoryLimit = 200

	TransitionSlotBecameAvailable   = "slot_became_available"
	TransitionSlotRescheduled         = "slot_rescheduled"
	TransitionSlotCancelled           = "slot_cancelled"
	TransitionSlotCompleted           = "slot_completed"
	TransitionSlotMissed              = "slot_missed"
	TransitionSlotProjectionAtRisk    = "slot_projection_at_risk"
	TransitionSlotProjectionMiss      = "slot_projection_miss"
	TransitionSlotProjectionRestored  = "slot_projection_restored"
)

type SlotRevision struct {
	ID               uuid.UUID
	TenantID         uuid.UUID
	ShipmentID       uuid.UUID
	SlotType         string
	FacilityID       *uuid.UUID
	LocationID       *uuid.UUID
	WindowStart      time.Time
	WindowEnd        time.Time
	Timezone         *string
	SlotStatus       string
	SourceType       string
	ProviderCode     *string
	ProviderSlotID   *string
	ProviderVersion  *string
	DedupKey         string
	SourceObservedAt time.Time
	ReceivedAt       time.Time
	QualityStatus    string
	QualityReasons   []string
	BookedAt         *time.Time
	ConfirmedAt      *time.Time
	CancelledAt      *time.Time
	CreatedAt        time.Time
}

type ShipmentSlotState struct {
	TenantID         uuid.UUID
	ShipmentID       uuid.UUID
	SlotType         string
	WindowStatus     string
	SlotStatus       *string
	WindowStart      *time.Time
	WindowEnd        *time.Time
	Timezone         *string
	FacilityID       *uuid.UUID
	LocationID       *uuid.UUID
	SourceType       *string
	ProviderCode     *string
	ProviderSlotID   *string
	SourceObservedAt *time.Time
	ReceivedAt       *time.Time
	QualityStatus    string
	BookedAt         *time.Time
	ConfirmedAt      *time.Time
	Version          int64
	UpdatedAt        time.Time
}

type SlotTargetSummary struct {
	WindowStatus           string
	SlotStatus             *string
	WindowStart            *time.Time
	WindowEnd              *time.Time
	Timezone               *string
	SourceType             *string
	Provider               *string
	ProviderSlotID         *string
	SourceObservedAt       *time.Time
	QualityStatus          string
	BookedAt               *time.Time
	ConfirmedAt            *time.Time
	ArrivalProjection      string
	ProjectedLateBySeconds *int64
	EarlyBySeconds         *int64
	MarginSeconds          *int64
	ETARelation            string
}

type ShipmentSlotSummary struct {
	ShipmentID uuid.UUID
	Pickup     *SlotTargetSummary
	Delivery   *SlotTargetSummary
}

type SlotStateTransition struct {
	ID             uuid.UUID
	TenantID       uuid.UUID
	ShipmentID     uuid.UUID
	SlotType       string
	TransitionType string
	FromStatus     *string
	ToStatus       string
	Metadata       map[string]any
	OccurredAt     time.Time
}

type ETASnapshot struct {
	HasUsableETA       bool
	Status             string
	FreshnessStatus    string
	QualityStatus      string
	EstimatedArrivalAt *time.Time
}

func SlotSourcePriority(sourceType string) int {
	switch sourceType {
	case SlotSourceWarehouseAPI:
		return 100
	case SlotSourceInternalBooking:
		return 90
	case SlotSourceShipperAPI:
		return 80
	case SlotSourceCarrierAPI:
		return 70
	case SlotSourceManualOperator:
		return 60
	case SlotSourceSystemImport:
		return 50
	default:
		return 0
	}
}

func IsEnabledSlotSourceType(sourceType string) bool {
	switch sourceType {
	case SlotSourceInternalBooking, SlotSourceWarehouseAPI, SlotSourceCarrierAPI,
		SlotSourceShipperAPI, SlotSourceManualOperator, SlotSourceSystemImport:
		return true
	default:
		return false
	}
}

func IsActiveSlotStatus(status string) bool {
	switch status {
	case SlotStatusProposed, SlotStatusBooked, SlotStatusConfirmed:
		return true
	default:
		return false
	}
}

func ShouldReplaceSlotState(current, candidate ShipmentSlotState) bool {
	if current.WindowStatus != SlotWindowAvailable {
		return candidate.WindowStatus == SlotWindowAvailable
	}
	if candidate.WindowStatus != SlotWindowAvailable {
		return false
	}
	curStatus := ""
	if current.SlotStatus != nil {
		curStatus = *current.SlotStatus
	}
	candStatus := ""
	if candidate.SlotStatus != nil {
		candStatus = *candidate.SlotStatus
	}
	curActive := IsActiveSlotStatus(curStatus)
	candActive := IsActiveSlotStatus(candStatus)
	if candActive && !curActive {
		return true
	}
	if !candActive && curActive {
		return false
	}
	curSource := ""
	if current.SourceType != nil {
		curSource = *current.SourceType
	}
	candSource := ""
	if candidate.SourceType != nil {
		candSource = *candidate.SourceType
	}
	if SlotSourcePriority(candSource) > SlotSourcePriority(curSource) {
		return true
	}
	if SlotSourcePriority(candSource) < SlotSourcePriority(curSource) {
		return false
	}
	if candidate.SourceObservedAt != nil && current.SourceObservedAt != nil {
		return candidate.SourceObservedAt.After(*current.SourceObservedAt)
	}
	if candidate.SourceObservedAt != nil && current.SourceObservedAt == nil {
		return true
	}
	return false
}
