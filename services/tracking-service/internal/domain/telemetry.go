package domain

import (
	"time"

	"github.com/google/uuid"
)

const (
	TrackingStatusNotConfigured = "not_configured"
	TrackingStatusAwaitingData    = "awaiting_data"
	TrackingStatusActive          = "active"
	TrackingStatusStale           = "stale"
	TrackingStatusLost            = "lost"
	TrackingStatusEnded           = "ended"

	FreshnessUnknown = "unknown"
	FreshnessFresh   = "fresh"
	FreshnessStale   = "stale"
	FreshnessLost    = "lost"

	QualityUnknown   = "unknown"
	QualityGood      = "good"
	QualityDegraded  = "degraded"
	QualityPoor      = "poor"

	BindingStatusActive   = "active"
	BindingStatusInactive = "inactive"
	BindingStatusRevoked  = "revoked"

	SourceVehicleTelematics = "vehicle_telematics"
	SourceDriverMobile      = "driver_mobile"
	SourceCarrierAPI        = "carrier_api"
	SourceManual            = "manual"
	SourceSystemImport      = "system_import"

	ProviderGeneric = "generic"

	TransitionTrackingStarted      = "tracking_started"
	TransitionTrackingBecameStale  = "tracking_became_stale"
	TransitionTrackingLost         = "tracking_lost"
	TransitionTrackingRestored     = "tracking_restored"
	TransitionTrackingEnded        = "tracking_ended"
	TransitionBindingChanged       = "binding_changed"

	MaxLocationHistoryLimit = 500
)

type LocationEvent struct {
	ID               uuid.UUID
	TenantID         uuid.UUID
	ShipmentID       uuid.UUID
	VehicleID        *uuid.UUID
	DriverID         *uuid.UUID
	ProviderCode     string
	ProviderDeviceID string
	ProviderEventID  *string
	DedupKey         string
	Latitude         float64
	Longitude        float64
	RecordedAt       time.Time
	ReceivedAt       time.Time
	SpeedKph         *float64
	HeadingDegrees   *float64
	AccuracyMeters   *float64
	AltitudeMeters   *float64
	SourceType       string
	QualityStatus    string
	QualityReason    *string
	CreatedAt        time.Time
}

type ShipmentTrackingBinding struct {
	ID               uuid.UUID
	TenantID         uuid.UUID
	ShipmentID       uuid.UUID
	VehicleID        *uuid.UUID
	DriverID         *uuid.UUID
	ProviderCode     string
	ProviderDeviceID string
	Status           string
	ActiveFrom       time.Time
	ActiveTo         *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type ShipmentTrackingState struct {
	TenantID             uuid.UUID
	ShipmentID           uuid.UUID
	TrackingStatus       string
	ProviderCode         *string
	LastLatitude         *float64
	LastLongitude        *float64
	LastRecordedAt       *time.Time
	LastReceivedAt       *time.Time
	LastSpeedKph         *float64
	LastHeadingDegrees   *float64
	FreshnessStatus      string
	QualityStatus        string
	AgeSeconds           *int64
	DeliveryDelaySeconds *int64
	UpdatedAt            time.Time
}

type LastKnownPosition struct {
	Latitude   float64
	Longitude  float64
	RecordedAt time.Time
	AgeSeconds int64
}

type TrackingSummary struct {
	ShipmentID         uuid.UUID
	TrackingStatus     string
	Provider           *string
	LastKnownPosition  *LastKnownPosition
	Freshness          FreshnessSummary
	Quality            QualitySummary
	LastRecordedAt     *time.Time
	LastReceivedAt     *time.Time
	SpeedKph           *float64
	HeadingDegrees     *float64
	DeliveryDelaySeconds *int64
}

type FreshnessSummary struct {
	Status     string
	AgeSeconds *int64
}

type QualitySummary struct {
	Status string
	Reason *string
}

type StateTransition struct {
	ID             uuid.UUID
	TenantID       uuid.UUID
	ShipmentID     uuid.UUID
	TransitionType string
	FromStatus     *string
	ToStatus       string
	Metadata       map[string]any
	OccurredAt     time.Time
}
