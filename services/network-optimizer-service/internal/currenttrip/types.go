package currenttrip

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound    = errors.New("current trip source not found")
	ErrUnavailable = errors.New("current trip source unavailable")
)

const (
	EvidenceConfirmedOnboard = "CONFIRMED_ONBOARD"
	EvidenceUnloaded         = "UNLOADED"
	EvidencePlanned          = "PLANNED"
	ResolutionUnproven       = "ONBOARD_CARGO_UNPROVEN"

	DimensionKnown   = "KNOWN"
	DimensionUnknown = "UNKNOWN"
	DimensionExceeds = "CURRENT_OCCUPANCY_EXCEEDS_CAPACITY"

	HeightKnownOK       = "KNOWN_OK"
	HeightKnownExceeded = "KNOWN_EXCEEDED"
	HeightUnknown       = "UNKNOWN"

	ReasonUnproven          = "ONBOARD_CARGO_UNPROVEN"
	ReasonVehicleUnknown    = "VEHICLE_TOTAL_UNKNOWN"
	ReasonOccupancyUnknown  = "OCCUPANCY_UNKNOWN"
	ReasonPalletEquivalence = "PALLET_EQUIVALENCE_UNPROVEN"
	ReasonPalletType        = "PALLET_TYPE_UNKNOWN"
	ReasonPalletCount       = "PALLET_COUNT_UNKNOWN"
	MultiZoneRequired       = "MULTI_ZONE_ALLOCATION_REQUIRED"

	ProvenanceAsset   = "ASSET_CONFIRMED"
	ProvenanceOnboard = "CONFIRMED_ONBOARD"
	ProvenanceUnknown = "UNKNOWN"
)

type ShipmentExecution struct {
	ShipmentID            uuid.UUID
	TenantID              uuid.UUID
	ShipmentVersion       int
	ShipmentStatus        string
	VehicleID             *uuid.UUID
	DriverID              *uuid.UUID
	CarrierCompanyID      *uuid.UUID
	OriginLocationID      uuid.UUID
	DestinationLocationID uuid.UUID
	CargoID               *uuid.UUID
	PlannedDeliveryAt     *time.Time
	ActualPickupAt        *time.Time
	ActualDeliveryAt      *time.Time
}

type OnboardEvidenceItem struct {
	CargoID         uuid.UUID
	State           string
	StateVersion    int
	OccurredAt      time.Time
	Source          string
	SourceEventType string
	DriverID        *uuid.UUID
}

type OnboardCargo struct {
	ShipmentID      uuid.UUID
	ShipmentVersion int
	Resolution      string
	Items           []OnboardEvidenceItem
}

type CargoProfile struct {
	ID                            uuid.UUID
	Version                       int
	CargoTypeCode                 *string
	WeightKg                      *float64
	VolumeM3                      *float64
	PalletCount                   *int
	PalletTypeCode                *string
	LinearMeters                  *float64
	MaxLoadedHeightMM             *int
	Stackable                     *bool
	Fragile                       *bool
	PackagingTypeCode             *string
	FoodGradeRequired             *bool
	TemperatureRequired           *bool
	TemperatureMinC               *float64
	TemperatureMaxC               *float64
	PreferredTemperatureSetpointC *float64
	DangerousGoods                *bool
	HazardClasses                 []string
	OdorEmissionClass             *string
	OdorSensitive                 *bool
	ContaminationClass            *string
}

type VehicleCapability struct {
	ID                            uuid.UUID
	TenantID                      uuid.UUID
	Version                       int
	EquipmentType                 *string
	CombinationType               *string
	BodyType                      *string
	EquipmentUnitKind             *string
	LoadingAccess                 []string
	UnloadingAccess               []string
	CapacityWeight                *float64
	CapacityVolume                *float64
	PalletPositions               *int
	UsableLinearMeters            *float64
	InternalLengthMM              *int
	InternalWidthMM               *int
	InternalHeightMM              *int
	TemperatureControlMode        *string
	TemperatureCapabilityMinC     *float64
	TemperatureCapabilityMaxC     *float64
	TemperatureZoneCount          *int
	IndependentTemperatureControl *bool
	FoodGradeCapability           *bool
	ADRCapability                 *bool
	ContainerSize                 *string
}

type TrackingPosition struct {
	TrackingStatus string
	Freshness      string
	AgeSeconds     *int
	Latitude       *float64
	Longitude      *float64
	RecordedAt     *time.Time
}

type ETA struct {
	Status             string
	FreshnessStatus    string
	AgeSeconds         *int
	EstimatedArrivalAt *time.Time
	SourceObservedAt   *time.Time
	SourceType         string
	Provider           string
}

type PalletEquivalence struct {
	FromCode        string
	BasisCode       string
	PositionsEach   float64
	OwnershipProven bool
}

type OnboardCargoUnit struct {
	CargoID              uuid.UUID
	CargoVersion         int
	EvidenceState        string
	EvidenceStateVersion int
	EvidenceOccurredAt   time.Time
	EvidenceSource       string
	Profile              CargoProfile
}

type SubtractiveDimension struct {
	Total      *float64
	Occupied   *float64
	Remaining  *float64
	Status     string
	Provenance string
	Reason     string
}

type HeightCheck struct {
	VehicleInternalHeightMM *int
	MaxOnboardHeightMM      *int
	Status                  string
	Provenance              string
}

type ResidualCapacitySnapshot struct {
	Payload               SubtractiveDimension
	Volume                SubtractiveDimension
	PalletPositions       SubtractiveDimension
	LinearMeters          SubtractiveDimension
	Height                HeightCheck
	TemperatureAllocation string
}

type CurrentTripContext struct {
	ShipmentID              uuid.UUID
	ShipmentVersion         int
	ShipmentStatus          string
	VehicleID               *uuid.UUID
	VehicleVersion          int
	DriverID                *uuid.UUID
	CarrierCompanyID        *uuid.UUID
	OriginLocationID        uuid.UUID
	DestinationLocationID   uuid.UUID
	CurrentPosition         *TrackingPosition
	PositionFreshnessStatus string
	PositionObservedAt      *time.Time
	ETA                     *ETA
	ETAFreshnessStatus      string
	ETAObservedAt           *time.Time
	OnboardCargoUnits       []OnboardCargoUnit
	VehicleCapability       *VehicleCapability
	ResidualCapacity        ResidualCapacitySnapshot
	InputFingerprint        string
	BuiltAt                 time.Time
}
