package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	apperrors "github.com/freight-platform/contract-rate-service/internal/platform/errors"
)

const (
	TransportModeRoad = "ROAD"

	ComponentTypeBaseFreight   = "BASE_FREIGHT"
	ComponentTypeFuelSurcharge = "FUEL_SURCHARGE"
	ComponentTypeWaiting       = "WAITING"
	ComponentTypeDetention     = "DETENTION"

	CalcMethodFlat      = "FLAT"
	CalcMethodPercent   = "PERCENT"
	CalcMethodUnitRate  = "UNIT_RATE"

	UnitCodeHour = "HOUR"

	PricingSourceContractRate = "CONTRACT_RATE"
	PricingSourceManualSpot   = "MANUAL_SPOT"
	PricingSourceRFQAward     = "RFQ_AWARD"
	PricingSourceSpotBid      = "SPOT_BID"

	ResolveStatusMatched   = "MATCHED"
	ResolveStatusNoMatch   = "NO_MATCH"
	ResolveStatusAmbiguous = "AMBIGUOUS"

	ResolverVersion = "v2.0B"
)

const (
	ReasonInvalidEquipmentType   = "INVALID_EQUIPMENT_TYPE"
	ReasonInvalidRateComponent   = "INVALID_RATE_COMPONENT"
	ReasonInvalidRateVersion     = "INVALID_RATE_VERSION"
	ReasonInvalidTransportMode   = "INVALID_TRANSPORT_MODE"
	ReasonRateLaneConflict       = "RATE_LANE_CONFLICT"
	ReasonRateNotFound           = "RATE_NOT_FOUND"
	ReasonRateAmbiguous          = "RATE_AMBIGUOUS"
	ReasonCurrencyMismatch       = "CURRENCY_MISMATCH"
	ReasonManualSpotForbidden    = "MANUAL_SPOT_FORBIDDEN"
	ReasonPricingSourceNotAvail  = "PRICING_SOURCE_NOT_AVAILABLE"
)

type RateLine struct {
	ID                    uuid.UUID
	TenantID              uuid.UUID
	RateCardVersionID     uuid.UUID
	OriginLocationID      uuid.UUID
	DestinationLocationID uuid.UUID
	EquipmentType         string
	TransportMode         string
	CreatedAt             time.Time
	CreatedBy             *uuid.UUID
	UpdatedAt             time.Time
	UpdatedBy             *uuid.UUID
}

type RateComponent struct {
	ID                uuid.UUID
	TenantID          uuid.UUID
	RateLineID        uuid.UUID
	ComponentType     string
	CalculationMethod string
	Amount            *decimal.Decimal
	PercentValue      *decimal.Decimal
	UnitCode          *string
	CreatedAt         time.Time
	CreatedBy         *uuid.UUID
	UpdatedAt         time.Time
	UpdatedBy         *uuid.UUID
}

type CreateRateLineInput struct {
	TenantID              uuid.UUID
	RateCardVersionID     uuid.UUID
	OriginLocationID      uuid.UUID
	DestinationLocationID uuid.UUID
	EquipmentType         string
	TransportMode         string
	Actor                 ActorInput
}

type UpdateRateLineInput struct {
	OriginLocationID      *uuid.UUID
	DestinationLocationID *uuid.UUID
	EquipmentType         *string
	TransportMode         *string
	Actor                 ActorInput
}

type CreateRateComponentInput struct {
	TenantID          uuid.UUID
	RateLineID        uuid.UUID
	ComponentType     string
	CalculationMethod string
	Amount            *decimal.Decimal
	PercentValue      *decimal.Decimal
	UnitCode          *string
	Actor             ActorInput
}

type UpdateRateComponentInput struct {
	Amount       *decimal.Decimal
	PercentValue *decimal.Decimal
	UnitCode     *string
	Actor        ActorInput
}

func NormalizeEquipmentType(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", apperrors.Validation("equipment_type is required", map[string]any{"field": "equipment_type", "code": ReasonInvalidEquipmentType})
	}
	return trimmed, nil
}

func NormalizeTransportMode(value string) (string, error) {
	mode := strings.TrimSpace(value)
	if mode == "" {
		mode = TransportModeRoad
	}
	if mode != TransportModeRoad {
		return "", apperrors.Validation("only ROAD transport mode is supported", map[string]any{"field": "transport_mode", "code": ReasonInvalidTransportMode})
	}
	return mode, nil
}

func ValidateDraftVersionMutation(version *RateCardVersion) error {
	if version == nil {
		return apperrors.NotFound("rate version not found")
	}
	if version.Status != RateVersionStatusDraft {
		return apperrors.Validation("rate lines and components are immutable outside DRAFT versions", map[string]any{
			"field": "status", "code": ReasonInvalidRateVersion, "status": version.Status,
		})
	}
	return nil
}

func ValidateCreateRateLineInput(in CreateRateLineInput) error {
	if in.TenantID == uuid.Nil || in.RateCardVersionID == uuid.Nil {
		return apperrors.Validation("tenant_id and rate_card_version_id are required", nil)
	}
	if in.OriginLocationID == uuid.Nil || in.DestinationLocationID == uuid.Nil {
		return apperrors.Validation("origin and destination location ids are required", nil)
	}
	if in.OriginLocationID == in.DestinationLocationID {
		return apperrors.Validation("origin and destination must differ", map[string]any{"field": "destination_location_id"})
	}
	if _, err := NormalizeEquipmentType(in.EquipmentType); err != nil {
		return err
	}
	if _, err := NormalizeTransportMode(in.TransportMode); err != nil {
		return err
	}
	return nil
}

func ValidateCreateRateComponentInput(in CreateRateComponentInput) error {
	if in.TenantID == uuid.Nil || in.RateLineID == uuid.Nil {
		return apperrors.Validation("tenant_id and rate_line_id are required", nil)
	}
	return validateComponentConfig(in.ComponentType, in.CalculationMethod, in.Amount, in.PercentValue, in.UnitCode)
}

func ValidateUpdateRateComponentInput(current *RateComponent, patch UpdateRateComponentInput) error {
	if current == nil {
		return apperrors.NotFound("rate component not found")
	}
	amount := current.Amount
	if patch.Amount != nil {
		amount = patch.Amount
	}
	percent := current.PercentValue
	if patch.PercentValue != nil {
		percent = patch.PercentValue
	}
	unit := current.UnitCode
	if patch.UnitCode != nil {
		unit = patch.UnitCode
	}
	return validateComponentConfig(current.ComponentType, current.CalculationMethod, amount, percent, unit)
}

func validateComponentConfig(componentType, method string, amount, percent *decimal.Decimal, unit *string) error {
	componentType = strings.ToUpper(strings.TrimSpace(componentType))
	method = strings.ToUpper(strings.TrimSpace(method))
	switch componentType {
	case ComponentTypeBaseFreight:
		if method != CalcMethodFlat {
			return componentValidationError("BASE_FREIGHT requires FLAT calculation method")
		}
		if amount == nil {
			return componentValidationError("BASE_FREIGHT amount is required")
		}
		if err := ValidateMoneyScale(*amount, "amount"); err != nil {
			return err
		}
	case ComponentTypeFuelSurcharge:
		if method != CalcMethodPercent {
			return componentValidationError("FUEL_SURCHARGE requires PERCENT calculation method")
		}
		if percent == nil {
			return componentValidationError("FUEL_SURCHARGE percent_value is required")
		}
	case ComponentTypeWaiting, ComponentTypeDetention:
		if method != CalcMethodUnitRate {
			return componentValidationError(componentType + " requires UNIT_RATE calculation method")
		}
		if amount == nil {
			return componentValidationError(componentType + " amount is required")
		}
		if err := ValidateMoneyScale(*amount, "amount"); err != nil {
			return err
		}
		if unit == nil || strings.TrimSpace(*unit) == "" {
			return componentValidationError(componentType + " unit_code is required")
		}
	default:
		return componentValidationError("unsupported component_type")
	}
	if percent != nil && method == CalcMethodPercent {
		if percent.IsNegative() {
			return componentValidationError("percent_value must be non-negative")
		}
	}
	return nil
}

func componentValidationError(message string) error {
	return apperrors.Validation(message, map[string]any{"code": ReasonInvalidRateComponent})
}

func ValidateActivatableLineComponents(components []RateComponent) error {
	if len(components) == 0 {
		return apperrors.Validation("rate line must contain components", map[string]any{"code": ReasonInvalidRateComponent})
	}
	counts := map[string]int{}
	for _, c := range components {
		counts[c.ComponentType]++
		if counts[c.ComponentType] > 1 {
			return componentValidationError("duplicate component_type on rate line")
		}
	}
	if counts[ComponentTypeBaseFreight] != 1 {
		return componentValidationError("exactly one BASE_FREIGHT component is required")
	}
	if counts[ComponentTypeFuelSurcharge] > 1 {
		return componentValidationError("at most one FUEL_SURCHARGE component is allowed")
	}
	if counts[ComponentTypeWaiting] > 1 || counts[ComponentTypeDetention] > 1 {
		return componentValidationError("at most one WAITING and one DETENTION rule is allowed")
	}
	for _, c := range components {
		if err := validateComponentConfig(c.ComponentType, c.CalculationMethod, c.Amount, c.PercentValue, c.UnitCode); err != nil {
			return err
		}
	}
	return nil
}

func DateWithinInclusive(onDate, from time.Time, to *time.Time) bool {
	on := dateOnly(onDate)
	start := dateOnly(from)
	if on.Before(start) {
		return false
	}
	if to == nil {
		return true
	}
	end := dateOnly(*to)
	return !on.After(end)
}

func IntervalsOverlap(aFrom time.Time, aTo *time.Time, bFrom time.Time, bTo *time.Time) bool {
	aStart := dateOnly(aFrom)
	bStart := dateOnly(bFrom)
	aEnd := aStart
	if aTo != nil {
		aEnd = dateOnly(*aTo)
	}
	bEnd := bStart
	if bTo != nil {
		bEnd = dateOnly(*bTo)
	}
	return !aEnd.Before(bStart) && !bEnd.Before(aStart)
}

func LaneKey(origin, destination uuid.UUID, equipmentType, transportMode string) string {
	return origin.String() + "|" + destination.String() + "|" + equipmentType + "|" + transportMode
}

func dateOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func HasManualSpotPriceRole(roleCodes []string) bool {
	for _, code := range roleCodes {
		switch strings.ToUpper(strings.TrimSpace(code)) {
		case RolePlatformAdmin, "SHIPPER_ADMIN", "PROCUREMENT_MANAGER", "FORWARDER_MANAGER":
			return true
		}
	}
	return false
}

func (a ActorInput) RequireManualSpotPrice(roleCodes []string) error {
	if a.IsPlatformAdmin || HasManualSpotPriceRole(roleCodes) {
		return nil
	}
	if a.ActorKind == ActorKindCarrier {
		return apperrors.Forbidden("manual spot pricing is not permitted for carrier actors", map[string]any{"code": ReasonManualSpotForbidden})
	}
	return apperrors.Forbidden("manual spot pricing authorization required", map[string]any{"code": ReasonManualSpotForbidden})
}
