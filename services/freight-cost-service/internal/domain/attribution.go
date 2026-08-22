package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

var NamespaceFreightCostVarianceAttribution = uuid.MustParse("f0a1b2c3-d4e5-6789-abcd-ef0123456791")

const (
	SemanticClassVarianceDriver            = "VARIANCE_DRIVER"
	SemanticClassVarianceAvailabilityReason = "VARIANCE_AVAILABILITY_REASON"

	VarianceKindCurrent = "CURRENT"
	VarianceKindFinal   = "FINAL"

	ReasonAccessorial      = "ACCESSORIAL"
	ReasonFuel             = "FUEL"
	ReasonDetention        = "DETENTION"
	ReasonWaiting          = "WAITING"
	ReasonLegacyPricing    = "LEGACY_PRICING"
	ReasonUnattributed     = "UNATTRIBUTED"
	ReasonManualAdjustment = "MANUAL_ADJUSTMENT"
	ReasonOther            = "OTHER"

	ReasonOpenDispute      = "OPEN_DISPUTE"
	ReasonCancelled        = "CANCELLED"
	ReasonMissingActual    = "MISSING_ACTUAL"
	ReasonMissingPlanned   = "MISSING_PLANNED"
	ReasonCurrencyMismatch = "CURRENCY_MISMATCH"
	ReasonTaxBasisMismatch = "TAX_BASIS_MISMATCH"
)

type AttributionInput struct {
	TenantID           uuid.UUID
	TransportOrderID   uuid.UUID
	VarianceKind       string
	SemanticClass      string
	ReasonCode         string
	EvidenceFingerprint string
	MappingVersion     int64
	ProjectionRevision int64
}

type VarianceAttribution struct {
	TenantID           uuid.UUID
	TransportOrderID   uuid.UUID
	AttributionFactID  uuid.UUID
	SemanticClass      string
	VarianceKind       string
	ReasonCode         string
	EvidenceJSON       map[string]any
	MappingVersion     int64
	ProjectionRevision int64
	IsCurrent          bool
}

func DeriveAttributionFactID(input AttributionInput) uuid.UUID {
	key := strings.Join([]string{
		input.TenantID.String(),
		input.TransportOrderID.String(),
		input.VarianceKind,
		input.SemanticClass,
		fmt.Sprintf("%d", input.ProjectionRevision),
		input.ReasonCode,
		input.EvidenceFingerprint,
		fmt.Sprintf("%d", input.MappingVersion),
	}, "|")
	return uuid.NewSHA1(NamespaceFreightCostVarianceAttribution, []byte(key))
}

func EvidenceFingerprint(parts ...string) string {
	h := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(h[:8])
}

func BuildAvailabilityReasons(projection *CostSummaryProjection, varianceKind string) []VarianceAttribution {
	if projection == nil {
		return nil
	}
	var varianceAmount *decimal.Decimal
	if varianceKind == VarianceKindCurrent {
		varianceAmount = projection.CurrentVarianceAmount
	} else {
		varianceAmount = projection.FinalVarianceAmount
	}
	if varianceAmount != nil {
		return nil
	}
	var reasons []string
	if projection.PlannedAmount == nil {
		reasons = append(reasons, ReasonMissingPlanned)
	}
	if projection.OpenDisputeCount > 0 {
		reasons = append(reasons, ReasonOpenDispute)
	}
	if strings.EqualFold(projection.SettlementStatus, "CANCELLED") {
		reasons = append(reasons, ReasonCancelled)
	}
	if varianceKind == VarianceKindCurrent && projection.CurrentActualAmount == nil && projection.PlannedAmount != nil {
		reasons = append(reasons, ReasonMissingActual)
	}
	if varianceKind == VarianceKindFinal && projection.FinalActualAmount == nil && projection.PlannedAmount != nil {
		reasons = append(reasons, ReasonMissingActual)
	}
	if len(reasons) == 0 {
		return nil
	}
	var out []VarianceAttribution
	for _, code := range reasons {
		fp := EvidenceFingerprint(code, varianceKind)
		input := AttributionInput{
			TenantID:            projection.TenantID,
			TransportOrderID:    projection.TransportOrderID,
			VarianceKind:        varianceKind,
			SemanticClass:       SemanticClassVarianceAvailabilityReason,
			ReasonCode:          code,
			EvidenceFingerprint: fp,
			MappingVersion:      0,
			ProjectionRevision:  projection.ProjectionRevision,
		}
		out = append(out, VarianceAttribution{
			TenantID:           input.TenantID,
			TransportOrderID:   input.TransportOrderID,
			AttributionFactID:  DeriveAttributionFactID(input),
			SemanticClass:      input.SemanticClass,
			VarianceKind:       input.VarianceKind,
			ReasonCode:         input.ReasonCode,
			EvidenceJSON:       map[string]any{"reason": code},
			MappingVersion:     0,
			ProjectionRevision: input.ProjectionRevision,
			IsCurrent:          true,
		})
	}
	return out
}

type ApprovedAccessorialEvidence struct {
	AccessorialID uuid.UUID
	ChargeCode    string
	Amount        decimal.Decimal
}

type DriverAttributionContext struct {
	ApprovedAccessorials []ApprovedAccessorialEvidence
	BaseFreightAmount    *decimal.Decimal
	PlatformMappings     []ChargeCodeMapping
	TenantMappings       []ChargeCodeMapping
	MappingVersion       int64
}

func BuildVarianceDrivers(
	projection *CostSummaryProjection,
	varianceKind string,
	ctx DriverAttributionContext,
) []VarianceAttribution {
	if projection == nil {
		return nil
	}
	var varianceAmount *decimal.Decimal
	if varianceKind == VarianceKindCurrent {
		varianceAmount = projection.CurrentVarianceAmount
	} else {
		varianceAmount = projection.FinalVarianceAmount
	}
	if varianceAmount == nil {
		return nil
	}

	var drivers []VarianceAttribution
	matched := false

	if ctx.BaseFreightAmount != nil && projection.PlannedAmount != nil &&
		!ctx.BaseFreightAmount.Round(MoneyScale).Equal(projection.PlannedAmount.Round(MoneyScale)) {
		fp := EvidenceFingerprint("legacy_pricing", ctx.BaseFreightAmount.StringFixed(MoneyScale), projection.PlannedAmount.StringFixed(MoneyScale))
		drivers = append(drivers, buildDriverAttribution(projection, varianceKind, ReasonLegacyPricing, fp, ctx.MappingVersion, map[string]any{
			"base_freight_amount": ctx.BaseFreightAmount.StringFixed(MoneyScale),
			"planned_amount":      projection.PlannedAmount.StringFixed(MoneyScale),
		}))
		matched = true
	}

	for _, accessorial := range ctx.ApprovedAccessorials {
		category := ResolveChargeCategory(accessorial.ChargeCode, ctx.PlatformMappings, ctx.TenantMappings)
		reason := ReasonAccessorial
		switch category {
		case "FUEL":
			reason = ReasonFuel
		case "DETENTION":
			reason = ReasonDetention
		case "WAITING":
			reason = ReasonWaiting
		}
		fp := EvidenceFingerprint(accessorial.AccessorialID.String(), accessorial.ChargeCode, accessorial.Amount.StringFixed(MoneyScale))
		drivers = append(drivers, buildDriverAttribution(projection, varianceKind, reason, fp, ctx.MappingVersion, map[string]any{
			"accessorial_id": accessorial.AccessorialID.String(),
			"charge_code":    accessorial.ChargeCode,
			"amount":         accessorial.Amount.StringFixed(MoneyScale),
			"category":       category,
		}))
		matched = true
	}

	if !matched {
		fp := EvidenceFingerprint("unattributed", varianceKind, varianceAmount.StringFixed(MoneyScale))
		drivers = append(drivers, buildDriverAttribution(projection, varianceKind, ReasonUnattributed, fp, ctx.MappingVersion, map[string]any{
			"variance_amount": varianceAmount.StringFixed(MoneyScale),
		}))
	}
	return drivers
}

func buildDriverAttribution(
	projection *CostSummaryProjection,
	varianceKind, reasonCode, fingerprint string,
	mappingVersion int64,
	evidence map[string]any,
) VarianceAttribution {
	input := AttributionInput{
		TenantID:            projection.TenantID,
		TransportOrderID:    projection.TransportOrderID,
		VarianceKind:        varianceKind,
		SemanticClass:       SemanticClassVarianceDriver,
		ReasonCode:          reasonCode,
		EvidenceFingerprint: fingerprint,
		MappingVersion:      mappingVersion,
		ProjectionRevision:  projection.ProjectionRevision,
	}
	return VarianceAttribution{
		TenantID:           input.TenantID,
		TransportOrderID:   input.TransportOrderID,
		AttributionFactID:  DeriveAttributionFactID(input),
		SemanticClass:      input.SemanticClass,
		VarianceKind:       input.VarianceKind,
		ReasonCode:         input.ReasonCode,
		EvidenceJSON:       evidence,
		MappingVersion:     mappingVersion,
		ProjectionRevision: input.ProjectionRevision,
		IsCurrent:          true,
	}
}
