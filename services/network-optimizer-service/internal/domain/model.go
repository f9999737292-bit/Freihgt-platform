package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	LoadDraft     = "DRAFT"
	LoadPublished = "PUBLISHED"
	LoadWithdrawn = "WITHDRAWN"

	CapacityAvailable = "AVAILABLE"
	CapacityWithdrawn = "WITHDRAWN"
	CapacityPredicted = "PREDICTED"

	VisPrivate     = "PRIVATE"
	VisInvited     = "INVITED_CARRIERS"
	VisMarketplace = "MARKETPLACE"
	VisAnonymized  = "ANONYMIZED_MARKETPLACE"
	VisNetworkOnly = "NETWORK_OPTIMIZATION_ONLY"

	CapVisPrivate        = "PRIVATE"
	CapVisShipperNetwork = "SHIPPER_NETWORK"
	CapVisMarketplace    = "MARKETPLACE"
	CapVisAnonymized     = "ANONYMIZED"

	SourceTransportOrder            = "TRANSPORT_ORDER"
	SourceShipment                  = "SHIPMENT"
	SourceManual                    = "MANUAL"
	SourceCurrentShipmentPrediction = "CURRENT_SHIPMENT_PREDICTION"

	EventLoadPublished     = "network.load_opportunity.published"
	EventLoadUpdated       = "network.load_opportunity.updated"
	EventLoadWithdrawn     = "network.load_opportunity.withdrawn"
	EventCapacityPublished = "network.capacity.published"
	EventCapacityUpdated   = "network.capacity.updated"
	EventCapacityWithdrawn = "network.capacity.withdrawn"
	EventCapacityPredicted = "network.capacity.predicted"
)

type Place struct {
	LocationID  *uuid.UUID `json:"location_id,omitempty"`
	Label       string     `json:"label,omitempty"`
	Latitude    *float64   `json:"latitude,omitempty"`
	Longitude   *float64   `json:"longitude,omitempty"`
	CountryCode string     `json:"country_code,omitempty"`
	Region      string     `json:"region,omitempty"`
	City        string     `json:"city,omitempty"`
}

type TimeWindow struct {
	Start *time.Time `json:"start,omitempty"`
	End   *time.Time `json:"end,omitempty"`
}

func (c CargoConstraints) empty() bool {
	return c.TemperatureMinC == nil && c.TemperatureMaxC == nil && c.PreferredTemperatureSetpointC == nil &&
		c.TemperatureRequired == nil && c.Dangerous == nil && len(c.RequiredBodyTypes) == 0 &&
		len(c.RequiredLoadingAccess) == 0 && len(c.RequiredUnloadingAccess) == 0 &&
		len(c.AllowedLoadingAccess) == 0 && len(c.AllowedUnloadingAccess) == 0 &&
		c.CargoTypeCode == nil && c.PalletCount == nil && c.PalletTypeCode == nil && c.LinearMeters == nil &&
		c.MaxLoadedHeightMM == nil && c.Stackable == nil && c.Fragile == nil && c.PackagingTypeCode == nil &&
		c.FoodGradeRequired == nil && c.OdorEmissionClass == nil && c.OdorSensitive == nil &&
		c.ContaminationClass == nil && len(c.HazardClasses) == 0
}

type CargoConstraints struct {
	TemperatureMinC               *float64 `json:"temperature_min_c,omitempty"`
	TemperatureMaxC               *float64 `json:"temperature_max_c,omitempty"`
	PreferredTemperatureSetpointC *float64 `json:"preferred_temperature_setpoint_c,omitempty"`
	TemperatureRequired           *bool    `json:"temperature_required,omitempty"`
	RequiredBodyTypes             []string `json:"required_body_types,omitempty"`
	RequiredLoadingAccess         []string `json:"required_loading_access,omitempty"`
	RequiredUnloadingAccess       []string `json:"required_unloading_access,omitempty"`
	AllowedLoadingAccess          []string `json:"allowed_loading_access,omitempty"`
	AllowedUnloadingAccess        []string `json:"allowed_unloading_access,omitempty"`
	Dangerous                     *bool    `json:"dangerous,omitempty"`
	CargoTypeCode                 *string  `json:"cargo_type_code,omitempty"`
	PalletCount                   *int     `json:"pallet_count,omitempty"`
	PalletTypeCode                *string  `json:"pallet_type_code,omitempty"`
	LinearMeters                  *float64 `json:"linear_meters,omitempty"`
	MaxLoadedHeightMM             *int     `json:"max_loaded_height_mm,omitempty"`
	Stackable                     *bool    `json:"stackable,omitempty"`
	Fragile                       *bool    `json:"fragile,omitempty"`
	PackagingTypeCode             *string  `json:"packaging_type_code,omitempty"`
	FoodGradeRequired             *bool    `json:"food_grade_required,omitempty"`
	OdorEmissionClass             *string  `json:"odor_emission_class,omitempty"`
	OdorSensitive                 *bool    `json:"odor_sensitive,omitempty"`
	ContaminationClass            *string  `json:"contamination_class,omitempty"`
	HazardClasses                 []string `json:"hazard_classes,omitempty"`
}

type Commercial struct {
	Mode     string   `json:"mode,omitempty"`
	Amount   *float64 `json:"amount,omitempty"`
	Currency string   `json:"currency,omitempty"`
}

type LoadOpportunity struct {
	ID                       uuid.UUID        `json:"id"`
	OwnerTenantID            uuid.UUID        `json:"owner_tenant_id"`
	SourceType               string           `json:"source_type"`
	SourceID                 uuid.UUID        `json:"source_id"`
	Pickup                   Place            `json:"pickup"`
	PickupWindow             TimeWindow       `json:"pickup_window"`
	Delivery                 Place            `json:"delivery"`
	DeliveryWindow           TimeWindow       `json:"delivery_window"`
	WeightKg                 *float64         `json:"weight_kg,omitempty"`
	VolumeM3                 *float64         `json:"volume_m3,omitempty"`
	BodyType                 string           `json:"body_type,omitempty"`
	Equipment                []string         `json:"equipment,omitempty"`
	Cargo                    CargoConstraints `json:"cargo,omitempty"`
	Commercial               Commercial       `json:"commercial,omitempty"`
	VisibilityScope                   string           `json:"visibility_scope"`
	InvitedCarrierCompanyIDs          []uuid.UUID      `json:"invited_carrier_company_ids,omitempty"`
	ConsolidationAllowed              bool             `json:"consolidation_allowed"`
	CrossShipperConsolidationAllowed  bool             `json:"cross_shipper_consolidation_allowed"`
	Status                            string           `json:"status"`
	Version                  int              `json:"version"`
	CreatedAt                time.Time        `json:"created_at"`
	UpdatedAt                time.Time        `json:"updated_at"`
}

type MarketplaceLoad struct {
	ID              uuid.UUID        `json:"id"`
	OwnerTenantID   *uuid.UUID       `json:"owner_tenant_id,omitempty"`
	Pickup          Place            `json:"pickup"`
	PickupWindow    TimeWindow       `json:"pickup_window"`
	Delivery        Place            `json:"delivery"`
	DeliveryWindow  TimeWindow       `json:"delivery_window"`
	WeightKg        *float64         `json:"weight_kg,omitempty"`
	VolumeM3        *float64         `json:"volume_m3,omitempty"`
	BodyType        string           `json:"body_type,omitempty"`
	Equipment       []string         `json:"equipment,omitempty"`
	Cargo           CargoConstraints `json:"cargo,omitempty"`
	Commercial      *Commercial      `json:"commercial,omitempty"`
	VisibilityScope string           `json:"visibility_scope"`
	Status          string           `json:"status"`
	Version         int              `json:"version"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
}

type Capacity struct {
	ID                 uuid.UUID   `json:"id"`
	OwnerTenantID      uuid.UUID   `json:"owner_tenant_id"`
	CarrierCompanyID   *uuid.UUID  `json:"carrier_company_id,omitempty"`
	VehicleID          *uuid.UUID  `json:"vehicle_id,omitempty"`
	LocationID         *uuid.UUID  `json:"location_id,omitempty"`
	LocationLabel      string      `json:"location_label,omitempty"`
	Latitude           *float64    `json:"latitude,omitempty"`
	Longitude          *float64    `json:"longitude,omitempty"`
	CountryCode        string      `json:"country_code,omitempty"`
	Region             string      `json:"region,omitempty"`
	City               string      `json:"city,omitempty"`
	AvailableFrom      time.Time   `json:"available_from"`
	AvailableUntil     time.Time   `json:"available_until"`
	Source             string      `json:"source"`
	BodyType           string      `json:"body_type,omitempty"`
	Equipment          []string    `json:"equipment,omitempty"`
	PayloadRemainingKg *float64    `json:"payload_remaining_kg,omitempty"`
	VolumeRemainingM3  *float64    `json:"volume_remaining_m3,omitempty"`
	VisibilityScope    string      `json:"visibility_scope"`
	AudienceTenantIDs  []uuid.UUID `json:"audience_tenant_ids,omitempty"`
	Status             string      `json:"status"`
	Version            int         `json:"version"`
	CreatedAt          time.Time   `json:"created_at"`
	UpdatedAt          time.Time   `json:"updated_at"`
}

type MarketplaceCapacity struct {
	ID                 uuid.UUID  `json:"id"`
	OwnerTenantID      *uuid.UUID `json:"owner_tenant_id,omitempty"`
	CarrierCompanyID   *uuid.UUID `json:"carrier_company_id,omitempty"`
	VehicleID          *uuid.UUID `json:"vehicle_id,omitempty"`
	LocationID         *uuid.UUID `json:"location_id,omitempty"`
	LocationLabel      string     `json:"location_label,omitempty"`
	Latitude           *float64   `json:"latitude,omitempty"`
	Longitude          *float64   `json:"longitude,omitempty"`
	CountryCode        string     `json:"country_code,omitempty"`
	Region             string     `json:"region,omitempty"`
	City               string     `json:"city,omitempty"`
	AvailableFrom      time.Time  `json:"available_from"`
	AvailableUntil     time.Time  `json:"available_until"`
	Source             string     `json:"source"`
	BodyType           string     `json:"body_type,omitempty"`
	Equipment          []string   `json:"equipment,omitempty"`
	PayloadRemainingKg *float64   `json:"payload_remaining_kg,omitempty"`
	VolumeRemainingM3  *float64   `json:"volume_remaining_m3,omitempty"`
	VisibilityScope    string     `json:"visibility_scope"`
	Status             string     `json:"status"`
	Version            int        `json:"version"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

func (l LoadOpportunity) MarketplaceView() MarketplaceLoad {
	view := MarketplaceLoad{
		ID: l.ID, PickupWindow: l.PickupWindow, DeliveryWindow: l.DeliveryWindow,
		WeightKg: l.WeightKg, VolumeM3: l.VolumeM3, BodyType: l.BodyType,
		Equipment: append([]string(nil), l.Equipment...), Cargo: l.Cargo,
		VisibilityScope: l.VisibilityScope, Status: l.Status, Version: l.Version,
		CreatedAt: l.CreatedAt, UpdatedAt: l.UpdatedAt,
	}
	// Anonymized display keeps only known coarse geography. Exact search
	// geography stays on the stored load and is not copied into this view.
	if l.VisibilityScope != VisAnonymized {
		view.Pickup = l.Pickup
		view.Delivery = l.Delivery
		owner := l.OwnerTenantID
		view.OwnerTenantID = &owner
	} else {
		view.Pickup = l.Pickup.CoarseDisplay()
		view.Delivery = l.Delivery.CoarseDisplay()
	}
	if commercial := l.Commercial; commercial.Mode != "" || commercial.Amount != nil {
		copy := commercial
		view.Commercial = &copy
	}
	return view
}

func (c Capacity) MarketplaceView() MarketplaceCapacity {
	view := MarketplaceCapacity{
		ID: c.ID, AvailableFrom: c.AvailableFrom, AvailableUntil: c.AvailableUntil, Source: c.Source,
		BodyType: c.BodyType, Equipment: append([]string(nil), c.Equipment...),
		PayloadRemainingKg: c.PayloadRemainingKg, VolumeRemainingM3: c.VolumeRemainingM3,
		VisibilityScope: c.VisibilityScope, Status: c.Status, Version: c.Version,
		CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
	}
	if c.VisibilityScope != CapVisAnonymized {
		view.LocationID = c.LocationID
		view.LocationLabel = c.LocationLabel
		view.Latitude = c.Latitude
		view.Longitude = c.Longitude
		view.CountryCode = c.CountryCode
		view.Region = c.Region
		view.City = c.City
		owner := c.OwnerTenantID
		view.OwnerTenantID = &owner
		view.CarrierCompanyID = c.CarrierCompanyID
		view.VehicleID = c.VehicleID
	} else {
		view.CountryCode = c.CountryCode
		view.Region = c.Region
		view.City = c.City
	}
	return view
}

func LoadVisible(l LoadOpportunity, viewer uuid.UUID, company *uuid.UUID) (bool, bool) {
	if l.OwnerTenantID == viewer || l.Status != LoadPublished {
		return false, false
	}
	switch l.VisibilityScope {
	case VisMarketplace:
		return true, false
	case VisAnonymized:
		return true, true
	case VisInvited:
		if company == nil {
			return false, false
		}
		for _, id := range l.InvitedCarrierCompanyIDs {
			if id == *company {
				return true, false
			}
		}
		return false, false
	default:
		return false, false
	}
}

func CapacityVisible(c Capacity, viewer uuid.UUID) bool {
	if c.OwnerTenantID == viewer || c.Status != CapacityAvailable {
		return false
	}
	switch c.VisibilityScope {
	case CapVisMarketplace, CapVisAnonymized:
		return true
	case CapVisShipperNetwork:
		for _, id := range c.AudienceTenantIDs {
			if id == viewer {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func ValidateLoad(l LoadOpportunity) error {
	switch l.SourceType {
	case SourceTransportOrder, SourceShipment:
	default:
		return fmt.Errorf("unsupported source_type")
	}
	if l.SourceID == uuid.Nil {
		return fmt.Errorf("source_id is required")
	}
	if err := validatePlace("pickup", l.Pickup); err != nil {
		return err
	}
	if err := validatePlace("delivery", l.Delivery); err != nil {
		return err
	}
	if err := validateWindow("pickup_window", l.PickupWindow); err != nil {
		return err
	}
	if err := validateWindow("delivery_window", l.DeliveryWindow); err != nil {
		return err
	}
	if err := positiveOptional("weight_kg", l.WeightKg); err != nil {
		return err
	}
	if err := positiveOptional("volume_m3", l.VolumeM3); err != nil {
		return err
	}
	if err := validateEquipment(l.Equipment); err != nil {
		return err
	}
	if err := validateCargo(l.Cargo); err != nil {
		return err
	}
	if err := validateCommercial(l.Commercial); err != nil {
		return err
	}
	return validateLoadVisibility(l.VisibilityScope, l.InvitedCarrierCompanyIDs)
}

func ValidateCapacity(c Capacity) error {
	if c.Source != SourceManual {
		return fmt.Errorf("only MANUAL capacity can be created in this release")
	}
	if c.LocationID == nil && strings.TrimSpace(c.LocationLabel) == "" && c.Latitude == nil && c.Longitude == nil {
		return fmt.Errorf("location_id, location_label, or coordinates are required")
	}
	if err := validateGeo(c.Latitude, c.Longitude); err != nil {
		return err
	}
	if err := validateCoarse("location", c.CountryCode, c.Region, c.City); err != nil {
		return err
	}
	if !c.AvailableUntil.After(c.AvailableFrom) || c.AvailableFrom.IsZero() || c.AvailableUntil.IsZero() {
		return fmt.Errorf("available_until must be after available_from")
	}
	if err := positiveOptional("payload_remaining_kg", c.PayloadRemainingKg); err != nil {
		return err
	}
	if err := positiveOptional("volume_remaining_m3", c.VolumeRemainingM3); err != nil {
		return err
	}
	if err := validateEquipment(c.Equipment); err != nil {
		return err
	}
	if len(c.BodyType) > 64 {
		return fmt.Errorf("body_type is too long")
	}
	return validateCapacityVisibility(c.VisibilityScope, c.AudienceTenantIDs)
}

func CanUpdateLoad(status string) bool {
	return status == LoadDraft || status == LoadPublished
}

func CanPublishLoad(status string) bool { return status == LoadDraft }

func CanWithdrawLoad(status string) bool {
	return status == LoadDraft || status == LoadPublished
}

func CanUpdateCapacity(status string) bool { return status == CapacityAvailable }

func CanWithdrawCapacity(status string) bool { return status == CapacityAvailable }

func validateLoadVisibility(scope string, invited []uuid.UUID) error {
	switch scope {
	case VisPrivate, VisMarketplace, VisAnonymized, VisNetworkOnly:
		if len(invited) > 0 {
			return fmt.Errorf("invited_carrier_company_ids are only valid for INVITED_CARRIERS")
		}
		return nil
	case VisInvited:
		if len(invited) == 0 {
			return fmt.Errorf("invited_carrier_company_ids are required for INVITED_CARRIERS")
		}
		return uniqueUUIDs("invited_carrier_company_ids", invited)
	default:
		return fmt.Errorf("unsupported visibility_scope")
	}
}

func validateCapacityVisibility(scope string, audience []uuid.UUID) error {
	switch scope {
	case CapVisPrivate, CapVisMarketplace, CapVisAnonymized:
		if len(audience) > 0 {
			return fmt.Errorf("audience_tenant_ids are only valid for SHIPPER_NETWORK")
		}
		return nil
	case CapVisShipperNetwork:
		if len(audience) == 0 {
			return fmt.Errorf("audience_tenant_ids are required for SHIPPER_NETWORK")
		}
		return uniqueUUIDs("audience_tenant_ids", audience)
	default:
		return fmt.Errorf("unsupported visibility_scope")
	}
}

func validatePlace(name string, p Place) error {
	if p.LocationID == nil && strings.TrimSpace(p.Label) == "" {
		return fmt.Errorf("%s requires location_id or label", name)
	}
	if len(p.Label) > 160 {
		return fmt.Errorf("%s label is too long", name)
	}
	if err := validateCoarse(name, p.CountryCode, p.Region, p.City); err != nil {
		return err
	}
	return validateGeo(p.Latitude, p.Longitude)
}

func validateCoarse(name, country, region, city string) error {
	if country != "" && len(country) != 2 {
		return fmt.Errorf("%s country_code must be 2 characters when provided", name)
	}
	if len(region) > 120 || len(city) > 120 {
		return fmt.Errorf("%s region or city is too long", name)
	}
	return nil
}

func validateGeo(lat, lon *float64) error {
	if lat == nil && lon == nil {
		return nil
	}
	if lat == nil || lon == nil {
		return fmt.Errorf("latitude and longitude must both be set")
	}
	if *lat < -90 || *lat > 90 || *lon < -180 || *lon > 180 {
		return fmt.Errorf("latitude or longitude is out of range")
	}
	return nil
}

func validateWindow(name string, w TimeWindow) error {
	if w.Start == nil && w.End == nil {
		return nil
	}
	if w.Start == nil || w.End == nil || !w.End.After(*w.Start) {
		return fmt.Errorf("%s end must be after start", name)
	}
	return nil
}

func positiveOptional(name string, v *float64) error {
	if v == nil {
		return nil
	}
	if *v <= 0 {
		return fmt.Errorf("%s must be greater than zero when provided; omit the field when unknown", name)
	}
	return nil
}

func validateEquipment(items []string) error {
	if len(items) > 16 {
		return fmt.Errorf("equipment has too many entries")
	}
	for _, item := range items {
		if strings.TrimSpace(item) == "" || len(item) > 64 {
			return fmt.Errorf("equipment entry is invalid")
		}
	}
	return nil
}

func validateCargo(c CargoConstraints) error {
	if err := finiteOptional("temperature_min_c", c.TemperatureMinC); err != nil {
		return err
	}
	if err := finiteOptional("temperature_max_c", c.TemperatureMaxC); err != nil {
		return err
	}
	if c.TemperatureMinC != nil && c.TemperatureMaxC != nil && *c.TemperatureMaxC < *c.TemperatureMinC {
		return fmt.Errorf("temperature_max_c must be greater than or equal to temperature_min_c")
	}
	if err := finiteOptional("preferred_temperature_setpoint_c", c.PreferredTemperatureSetpointC); err != nil {
		return err
	}
	if err := validateBodyTokens(c.RequiredBodyTypes); err != nil {
		return err
	}
	if err := validateAccessTokens("required_loading_access", c.RequiredLoadingAccess); err != nil {
		return err
	}
	if err := validateAccessTokens("required_unloading_access", c.RequiredUnloadingAccess); err != nil {
		return err
	}
	if err := validateAccessTokens("allowed_loading_access", c.AllowedLoadingAccess); err != nil {
		return err
	}
	if err := validateAccessTokens("allowed_unloading_access", c.AllowedUnloadingAccess); err != nil {
		return err
	}
	if c.PalletCount != nil && *c.PalletCount <= 0 {
		return fmt.Errorf("pallet_count must be greater than zero when known")
	}
	if c.LinearMeters != nil && *c.LinearMeters <= 0 {
		return fmt.Errorf("linear_meters must be greater than zero when known")
	}
	if c.MaxLoadedHeightMM != nil && *c.MaxLoadedHeightMM <= 0 {
		return fmt.Errorf("max_loaded_height_mm must be greater than zero when known")
	}
	return nil
}

func finiteOptional(name string, v *float64) error {
	if v == nil {
		return nil
	}
	if *v < -273.15 || *v > 1000 {
		return fmt.Errorf("%s is out of range", name)
	}
	return nil
}

func validateCommercial(c Commercial) error {
	switch c.Mode {
	case "":
		if c.Amount != nil || c.Currency != "" {
			return fmt.Errorf("commercial.mode is required when commercial fields are set")
		}
		return nil
	case "OPEN":
		if c.Amount != nil || c.Currency != "" {
			return fmt.Errorf("OPEN commercial mode cannot include an amount")
		}
		return nil
	case "FIXED_OFFER":
		if c.Amount == nil || *c.Amount <= 0 || len(c.Currency) != 3 {
			return fmt.Errorf("FIXED_OFFER requires a positive amount and a 3-letter currency")
		}
		return nil
	default:
		return fmt.Errorf("unsupported commercial.mode")
	}
}

func uniqueUUIDs(name string, ids []uuid.UUID) error {
	seen := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		if id == uuid.Nil {
			return fmt.Errorf("%s contains an empty id", name)
		}
		if _, ok := seen[id]; ok {
			return fmt.Errorf("%s contains a duplicate id", name)
		}
		seen[id] = struct{}{}
	}
	return nil
}
