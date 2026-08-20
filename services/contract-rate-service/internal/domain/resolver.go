package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	apperrors "github.com/freight-platform/contract-rate-service/internal/platform/errors"
)

type ResolveRateRequest struct {
	TenantID              uuid.UUID
	BuyerCompanyID        uuid.UUID
	CarrierCompanyID      uuid.UUID
	OriginLocationID      uuid.UUID
	DestinationLocationID uuid.UUID
	EquipmentType         string
	TransportMode         string
	PricingDate           time.Time
	CurrencyCode          *string
	ManualSpotAmount      *decimal.Decimal
	ManualSpotCurrency    *string
	PricingSource         *string
	AwardLinkID           *uuid.UUID
	BidID                 *uuid.UUID
	Actor                 ActorInput
}

type AccessorialRule struct {
	ComponentType     string `json:"component_type"`
	CalculationMethod string `json:"calculation_method"`
	UnitAmount        string `json:"unit_amount"`
	UnitCode          string `json:"unit_code"`
	CurrencyCode      string `json:"currency_code"`
}

type ResolvedComponent struct {
	ComponentType     string  `json:"component_type"`
	CalculationMethod string  `json:"calculation_method"`
	Amount            *string `json:"amount,omitempty"`
	PercentValue      *string `json:"percent_value,omitempty"`
	UnitCode          *string `json:"unit_code,omitempty"`
}

type ResolveRateResult struct {
	Status                    string              `json:"status"`
	PricingSource             string              `json:"pricing_source,omitempty"`
	ContractID                *uuid.UUID          `json:"contract_id,omitempty"`
	RateCardID                *uuid.UUID          `json:"rate_card_id,omitempty"`
	RateVersionID             *uuid.UUID          `json:"rate_version_id,omitempty"`
	RateLineID                *uuid.UUID          `json:"rate_line_id,omitempty"`
	ContractNumber            *string             `json:"contract_number,omitempty"`
	RateCardName              *string             `json:"rate_card_name,omitempty"`
	VersionNumber             *int                `json:"version_number,omitempty"`
	OriginLocationID          *uuid.UUID          `json:"origin_location_id,omitempty"`
	DestinationLocationID     *uuid.UUID          `json:"destination_location_id,omitempty"`
	EquipmentType             *string             `json:"equipment_type,omitempty"`
	TransportMode             *string             `json:"transport_mode,omitempty"`
	CurrencyCode              *string             `json:"currency_code,omitempty"`
	ComponentBreakdownStatus  string              `json:"component_breakdown_status,omitempty"`
	BaseAmount                *string             `json:"base_amount,omitempty"`
	TotalAmount               *string             `json:"total_amount,omitempty"`
	Components                []ResolvedComponent `json:"components,omitempty"`
	AccessorialRules          []AccessorialRule   `json:"accessorial_rules,omitempty"`
	PricingDate               string              `json:"pricing_date,omitempty"`
	ResolvedAt                time.Time           `json:"resolved_at"`
	ResolverVersion           string              `json:"resolver_version"`
	ReasonCode                *string             `json:"reason_code,omitempty"`
}

type RateCandidate struct {
	ContractID            uuid.UUID
	ContractNumber        string
	ContractValidFrom     time.Time
	ContractValidTo       *time.Time
	ContractStatus        string
	ContractCurrency      string
	BuyerCompanyID        uuid.UUID
	CarrierCompanyID      uuid.UUID
	RateCardID            uuid.UUID
	RateCardName          string
	RateVersionID         uuid.UUID
	VersionNumber         int
	VersionValidFrom      time.Time
	VersionValidTo        *time.Time
	RateLineID            uuid.UUID
	OriginLocationID      uuid.UUID
	DestinationLocationID uuid.UUID
	EquipmentType         string
	TransportMode         string
	Components            []RateComponent
}

func ValidateResolveRateRequest(req ResolveRateRequest) (ResolveRateRequest, error) {
	if req.TenantID == uuid.Nil {
		return req, apperrors.Unauthorized("tenant context is required")
	}
	if req.BuyerCompanyID == uuid.Nil || req.CarrierCompanyID == uuid.Nil {
		return req, apperrors.Validation("buyer_company_id and carrier_company_id are required", nil)
	}
	if req.OriginLocationID == uuid.Nil || req.DestinationLocationID == uuid.Nil {
		return req, apperrors.Validation("origin and destination location ids are required", nil)
	}
	equipment, err := NormalizeEquipmentType(req.EquipmentType)
	if err != nil {
		return req, err
	}
	mode, err := NormalizeTransportMode(req.TransportMode)
	if err != nil {
		return req, err
	}
	req.EquipmentType = equipment
	req.TransportMode = mode
	if req.PricingDate.IsZero() {
		req.PricingDate = time.Now().UTC()
	}
	if req.PricingSource != nil {
		src := strings.ToUpper(strings.TrimSpace(*req.PricingSource))
		switch src {
		case PricingSourceRFQAward, PricingSourceSpotBid:
			return req, apperrors.Validation("explicit RFx pricing source is not available in v2.0B", map[string]any{"code": ReasonPricingSourceNotAvail, "pricing_source": src})
		}
	}
	if req.AwardLinkID != nil || req.BidID != nil {
		return req, apperrors.Validation("explicit RFx pricing identifiers are not available in v2.0B", map[string]any{"code": ReasonPricingSourceNotAvail})
	}
	if req.CurrencyCode != nil {
		if err := ValidateCurrencyCode(*req.CurrencyCode); err != nil {
			return req, err
		}
		normalized := NormalizeCurrencyCode(*req.CurrencyCode)
		req.CurrencyCode = &normalized
	}
	if req.ManualSpotAmount != nil {
		if err := ValidateMoneyScale(*req.ManualSpotAmount, "manual_spot_amount"); err != nil {
			return req, err
		}
	}
	if req.ManualSpotCurrency != nil {
		if err := ValidateCurrencyCode(*req.ManualSpotCurrency); err != nil {
			return req, err
		}
		n := NormalizeCurrencyCode(*req.ManualSpotCurrency)
		req.ManualSpotCurrency = &n
	}
	return req, nil
}

func ResolveRateCandidates(req ResolveRateRequest, candidates []RateCandidate) ResolveRateResult {
	now := time.Now().UTC()
	eligible := make([]RateCandidate, 0, len(candidates))
	for _, c := range candidates {
		if c.ContractStatus != ContractStatusActive {
			continue
		}
		if !DateWithinInclusive(req.PricingDate, c.ContractValidFrom, c.ContractValidTo) {
			continue
		}
		if !DateWithinInclusive(req.PricingDate, c.VersionValidFrom, c.VersionValidTo) {
			continue
		}
		if req.CurrencyCode != nil && !strings.EqualFold(c.ContractCurrency, *req.CurrencyCode) {
			continue
		}
		eligible = append(eligible, c)
	}

	switch len(eligible) {
	case 0:
		return ResolveRateResult{Status: ResolveStatusNoMatch, ResolvedAt: now, ResolverVersion: ResolverVersion}
	case 1:
		return buildContractMatch(eligible[0], req.PricingDate, now)
	default:
		code := ReasonRateAmbiguous
		return ResolveRateResult{Status: ResolveStatusAmbiguous, ResolvedAt: now, ResolverVersion: ResolverVersion, ReasonCode: &code}
	}
}

func ApplyManualSpotFallback(req ResolveRateRequest, base ResolveRateResult, authorized bool) (ResolveRateResult, error) {
	if base.Status == ResolveStatusMatched || base.Status == ResolveStatusAmbiguous {
		return base, nil
	}
	if base.Status != ResolveStatusNoMatch {
		return base, nil
	}
	if req.ManualSpotAmount == nil {
		return base, nil
	}
	if !authorized {
		return ResolveRateResult{}, apperrors.Forbidden("manual spot pricing authorization required", map[string]any{"code": ReasonManualSpotForbidden})
	}
	currency := req.ManualSpotCurrency
	if currency == nil && req.CurrencyCode != nil {
		currency = req.CurrencyCode
	}
	if currency == nil {
		return ResolveRateResult{}, apperrors.Validation("manual_spot_currency is required", map[string]any{"field": "manual_spot_currency"})
	}
	amount := RoundMoney(*req.ManualSpotAmount)
	amountStr := amount.StringFixed(MoneyScale)
	cur := *currency
	now := time.Now().UTC()
	return ResolveRateResult{
		Status:                   ResolveStatusMatched,
		PricingSource:            PricingSourceManualSpot,
		CurrencyCode:             &cur,
		ComponentBreakdownStatus: "UNKNOWN",
		TotalAmount:              &amountStr,
		PricingDate:              req.PricingDate.Format("2006-01-02"),
		ResolvedAt:               now,
		ResolverVersion:          ResolverVersion,
	}, nil
}

func buildContractMatch(c RateCandidate, pricingDate, resolvedAt time.Time) ResolveRateResult {
	calc, err := CalculatePreExecutionTotal(c.Components, c.ContractCurrency)
	if err != nil {
		code := ReasonInvalidRateComponent
		return ResolveRateResult{Status: ResolveStatusNoMatch, ResolvedAt: resolvedAt, ResolverVersion: ResolverVersion, ReasonCode: &code}
	}
	baseStr := calc.BaseAmount.StringFixed(MoneyScale)
	totalStr := calc.TotalAmount.StringFixed(MoneyScale)
	vn := c.VersionNumber
	return ResolveRateResult{
		Status:                   ResolveStatusMatched,
		PricingSource:            PricingSourceContractRate,
		ContractID:               &c.ContractID,
		RateCardID:               &c.RateCardID,
		RateVersionID:            &c.RateVersionID,
		RateLineID:               &c.RateLineID,
		ContractNumber:           &c.ContractNumber,
		RateCardName:             &c.RateCardName,
		VersionNumber:            &vn,
		OriginLocationID:         &c.OriginLocationID,
		DestinationLocationID:    &c.DestinationLocationID,
		EquipmentType:            &c.EquipmentType,
		TransportMode:            &c.TransportMode,
		CurrencyCode:             &c.ContractCurrency,
		ComponentBreakdownStatus: "AVAILABLE",
		BaseAmount:               &baseStr,
		TotalAmount:              &totalStr,
		Components:               calc.Components,
		AccessorialRules:         calc.AccessorialRules,
		PricingDate:              pricingDate.Format("2006-01-02"),
		ResolvedAt:               resolvedAt,
		ResolverVersion:          ResolverVersion,
	}
}

type PreExecutionCalculation struct {
	BaseAmount       decimal.Decimal
	TotalAmount      decimal.Decimal
	Components       []ResolvedComponent
	AccessorialRules []AccessorialRule
}

func CalculatePreExecutionTotal(components []RateComponent, currencyCode string) (PreExecutionCalculation, error) {
	if err := ValidateActivatableLineComponents(components); err != nil {
		return PreExecutionCalculation{}, err
	}
	var base decimal.Decimal
	var fuelPercent *decimal.Decimal
	outComponents := make([]ResolvedComponent, 0, 2)
	accessorials := make([]AccessorialRule, 0, 2)
	for _, c := range components {
		switch c.ComponentType {
		case ComponentTypeBaseFreight:
			base = RoundMoney(*c.Amount)
			amount := base.StringFixed(MoneyScale)
			outComponents = append(outComponents, ResolvedComponent{
				ComponentType: c.ComponentType, CalculationMethod: c.CalculationMethod, Amount: &amount,
			})
		case ComponentTypeFuelSurcharge:
			fuelPercent = c.PercentValue
			pct := fuelPercent.String()
			outComponents = append(outComponents, ResolvedComponent{
				ComponentType: c.ComponentType, CalculationMethod: c.CalculationMethod, PercentValue: &pct,
			})
		case ComponentTypeWaiting, ComponentTypeDetention:
			unitAmt := RoundMoney(*c.Amount).StringFixed(MoneyScale)
			unitCode := strings.TrimSpace(*c.UnitCode)
			accessorials = append(accessorials, AccessorialRule{
				ComponentType: c.ComponentType, CalculationMethod: c.CalculationMethod,
				UnitAmount: unitAmt, UnitCode: unitCode, CurrencyCode: currencyCode,
			})
		}
	}
	total := base
	if fuelPercent != nil {
		fuel := RoundMoney(base.Mul(*fuelPercent).Div(decimal.NewFromInt(100)))
		amount := fuel.StringFixed(MoneyScale)
		for i := range outComponents {
			if outComponents[i].ComponentType == ComponentTypeFuelSurcharge {
				outComponents[i].Amount = &amount
			}
		}
		total = RoundMoney(base.Add(fuel))
	}
	return PreExecutionCalculation{
		BaseAmount: base, TotalAmount: total, Components: outComponents, AccessorialRules: accessorials,
	}, nil
}

func DecimalString(v decimal.Decimal) string {
	return v.StringFixed(MoneyScale)
}
