package domain

import (
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Frozen v2.1B UUIDv5 namespaces.
var (
	NamespaceFreightCostCanonicalFact  = uuid.MustParse("f0a1b2c3-d4e5-6789-abcd-ef0123456781")
	NamespaceFreightCostRebuildDelivery = uuid.MustParse("f0a1b2c3-d4e5-6789-abcd-ef0123456782")
)

const (
	SourceServiceBillingRegister = "billing-register-service"
	SourceServicePayment         = "payment-service"

	SourceTypeFreightSettlement          = "FREIGHT_SETTLEMENT"
	SourceTypeFreightSettlementBillingLink = "FREIGHT_SETTLEMENT_BILLING_LINK"
	SourceTypeBillingRegister            = "BILLING_REGISTER"
	SourceTypePaymentObligation          = "PAYMENT_OBLIGATION"

	RevisionSemanticImmutable = "IMMUTABLE"

	EntryKindPlannedCostSnapshot       = "PLANNED_COST_SNAPSHOT"
	EntryKindAccrualCostSnapshot       = "ACCRUAL_COST_SNAPSHOT"
	EntryKindCurrentActualCostSnapshot = "CURRENT_ACTUAL_COST_SNAPSHOT"
	EntryKindFinalActualCostSnapshot   = "FINAL_ACTUAL_COST_SNAPSHOT"
	EntryKindBilledCostSnapshot        = "BILLED_COST_SNAPSHOT"
	EntryKindPayableAmountSnapshot     = "PAYABLE_AMOUNT_SNAPSHOT"
	EntryKindPaidAmountSnapshot        = "PAID_AMOUNT_SNAPSHOT"

	TaxBasisExVAT   TaxBasis = "EX_VAT"
	TaxBasisWithVAT TaxBasis = "WITH_VAT"

	AmountAvailabilityAvailable   = "AVAILABLE"
	AmountAvailabilityUnavailable = "UNAVAILABLE"

	EventOriginLiveOutbox       = "LIVE_OUTBOX"
	EventOriginCanonicalRebuild = "CANONICAL_REBUILD"

	BillingLinkStateLinked   = "LINKED"
	BillingLinkStateUnlinked = "UNLINKED"
)

type TaxBasis string

type CostEntry struct {
	ID                 uuid.UUID
	TenantID           uuid.UUID
	TransportOrderID   uuid.UUID
	ShipmentID         *uuid.UUID
	BuyerCompanyID     uuid.UUID
	CarrierCompanyID   uuid.UUID
	EntryKind          string
	Amount             *decimal.Decimal
	CurrencyCode       string
	TaxBasis           TaxBasis
	AmountAvailability string
	SourceService      string
	SourceType         string
	SourceID           uuid.UUID
	SourceRevision     int64
	SourceFactID       uuid.UUID
	SourceEventID      uuid.UUID
	SourceOccurredAt   time.Time
	SupersedesEntryID  *uuid.UUID
	EventOrigin        string
	Metadata           map[string]any
}

type SourceCursorKey struct {
	TenantID         uuid.UUID
	TransportOrderID uuid.UUID
	SourceService    string
	SourceType       string
	SourceID         uuid.UUID
	EntryKind        string
}

type SourceCursor struct {
	SourceCursorKey
	LastSourceRevision int64
	LastSourceEventID  *uuid.UUID
	LastCostEntryID    *uuid.UUID
}

type CostSummaryProjection struct {
	TenantID                    uuid.UUID
	TransportOrderID            uuid.UUID
	BuyerCompanyID              uuid.UUID
	CarrierCompanyID            uuid.UUID
	CurrencyCode                string
	PlannedAmount               *decimal.Decimal
	AccruedAmount               *decimal.Decimal
	CurrentActualAmount         *decimal.Decimal
	FinalActualAmount           *decimal.Decimal
	BillingRegisterAmount       *decimal.Decimal
	PayableAmount               *decimal.Decimal
	PaidAmount                  *decimal.Decimal
	BillingReconciliationStatus BillingReconciliationStatus
	FinancialFinality           FinancialFinality
	DataStage                   DataStage
	SourcesAvailable            []string
	SettlementLinked            bool
	SettlementStatus            string
	OpenDisputeCount            int
	CurrentVarianceAmount       *decimal.Decimal
	FinalVarianceAmount         *decimal.Decimal
	CurrentVariancePercent      *decimal.Decimal
	FinalVariancePercent        *decimal.Decimal
	ForecastExposure            *decimal.Decimal
	AttributionMappingVersion   *int64
	ProjectionRevision          int64
}

func SourceRevisionSemantic(sourceType string, sourceRevision int64) string {
	if strings.EqualFold(sourceType, SourceTypeTORateSnapshot) {
		return RevisionSemanticImmutable
	}
	return strconv.FormatInt(sourceRevision, 10)
}

func DeriveSourceFactID(
	tenantID uuid.UUID,
	sourceService, sourceType string,
	sourceID uuid.UUID,
	sourceRevisionSemantic, entryKind string,
) uuid.UUID {
	key := strings.Join([]string{
		tenantID.String(),
		sourceService,
		sourceType,
		sourceID.String(),
		sourceRevisionSemantic,
		entryKind,
	}, "|")
	return uuid.NewSHA1(NamespaceFreightCostCanonicalFact, []byte(key))
}

func DeriveRebuildDeliveryID(tenantID, sourceFactID uuid.UUID) uuid.UUID {
	key := tenantID.String() + "|" + sourceFactID.String()
	return uuid.NewSHA1(NamespaceFreightCostRebuildDelivery, []byte(key))
}

func CostEntriesSemanticallyEqual(a, b *CostEntry) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.EntryKind == b.EntryKind &&
		a.CurrencyCode == b.CurrencyCode &&
		a.TaxBasis == b.TaxBasis &&
		a.AmountAvailability == b.AmountAvailability &&
		a.SourceService == b.SourceService &&
		a.SourceType == b.SourceType &&
		a.SourceID == b.SourceID &&
		a.SourceRevision == b.SourceRevision &&
		decimalPtrEqual(a.Amount, b.Amount) &&
		a.TransportOrderID == b.TransportOrderID &&
		a.BuyerCompanyID == b.BuyerCompanyID &&
		a.CarrierCompanyID == b.CarrierCompanyID
}

func decimalPtrEqual(a, b *decimal.Decimal) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Round(MoneyScale).Equal(b.Round(MoneyScale))
}
