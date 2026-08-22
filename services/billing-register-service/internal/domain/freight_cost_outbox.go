package domain

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

const (
	FreightCostOutboxStatusPending   = "PENDING"
	FreightCostOutboxStatusPublished = "PUBLISHED"
	FreightCostOutboxStatusFailed    = "FAILED"

	FreightCostServiceName = "billing-register-service"

	EventFreightSettlementAccrualSnapshot       = "freight_settlement.accrual_snapshot.v1"
	EventFreightSettlementCurrentActualSnapshot = "freight_settlement.current_actual_snapshot.v1"
	EventFreightSettlementFinalActualSnapshot   = "freight_settlement.final_actual_snapshot.v1"
	EventBillingRegisterSettlementBillingLink   = "billing_register.settlement_billing_link_snapshot.v1"
	EventBillingRegisterPayableSnapshot         = "billing_register.payable_snapshot.v1"

	AggregateFreightSettlement            = "FREIGHT_SETTLEMENT"
	AggregateFreightSettlementBillingLink = "FREIGHT_SETTLEMENT_BILLING_LINK"
	AggregateBillingRegister              = "BILLING_REGISTER"

	EntryKindAccrualCostSnapshot       = "ACCRUAL_COST_SNAPSHOT"
	EntryKindCurrentActualCostSnapshot = "CURRENT_ACTUAL_COST_SNAPSHOT"
	EntryKindFinalActualCostSnapshot   = "FINAL_ACTUAL_COST_SNAPSHOT"
	EntryKindBilledCostSnapshot        = "BILLED_COST_SNAPSHOT"
	EntryKindPayableAmountSnapshot     = "PAYABLE_AMOUNT_SNAPSHOT"

	TaxBasisExVAT     = "EX_VAT"
	TaxBasisWithVAT   = "WITH_VAT"
	AmountAvailable   = "AVAILABLE"
	AmountUnavailable = "UNAVAILABLE"

	BillingLinkStateLinked   = "LINKED"
	BillingLinkStateUnlinked = "UNLINKED"

	FreightCostOutboxSchemaVersion = 1
	MoneyScale                     = 2
)

var ErrFreightCostOutboxPublishStateConflict = errors.New("freight cost outbox publish state conflict")

var AllSettlementSnapshotEventTypes = []string{
	EventFreightSettlementAccrualSnapshot,
	EventFreightSettlementCurrentActualSnapshot,
	EventFreightSettlementFinalActualSnapshot,
}

type FreightCostOutboxEvent struct {
	ID             uuid.UUID
	TenantID       uuid.UUID
	AggregateType  string
	AggregateID    uuid.UUID
	SourceRevision int64
	EventType      string
	SchemaVersion  int
	Payload        json.RawMessage
	Status         string
	Attempts       int
	AvailableAt    time.Time
	LockedAt       *time.Time
	LockedBy       *string
	PublishedAt    *time.Time
	LastErrorCode  *string
	CreatedAt      time.Time
}

type settlementSnapshotPayload struct {
	EventID            string  `json:"event_id"`
	EventType          string  `json:"event_type"`
	SchemaVersion      int     `json:"schema_version"`
	TenantID           string  `json:"tenant_id"`
	TransportOrderID   string  `json:"transport_order_id"`
	ShipmentID         string  `json:"shipment_id,omitempty"`
	SourceService      string  `json:"source_service"`
	SourceType         string  `json:"source_type"`
	SourceID           string  `json:"source_id"`
	SourceRevision     int64   `json:"source_revision"`
	OccurredAt         string  `json:"occurred_at"`
	CurrencyCode       string  `json:"currency_code"`
	TaxBasis           string  `json:"tax_basis"`
	EntryKind          string  `json:"entry_kind"`
	AmountAvailability string  `json:"amount_availability"`
	Amount             *string `json:"amount"`
	SettlementStatus   string  `json:"settlement_status"`
	OpenDisputeCount   int     `json:"open_dispute_count"`
}

type billingLinkSnapshotPayload struct {
	EventID               string  `json:"event_id"`
	EventType             string  `json:"event_type"`
	SchemaVersion         int     `json:"schema_version"`
	TenantID              string  `json:"tenant_id"`
	TransportOrderID      string  `json:"transport_order_id"`
	ShipmentID            string  `json:"shipment_id,omitempty"`
	SourceService         string  `json:"source_service"`
	SourceType            string  `json:"source_type"`
	SourceID              string  `json:"source_id"`
	SourceRevision        int64   `json:"source_revision"`
	OccurredAt            string  `json:"occurred_at"`
	CurrencyCode          string  `json:"currency_code"`
	TaxBasis              string  `json:"tax_basis"`
	EntryKind             string  `json:"entry_kind"`
	AmountAvailability    string  `json:"amount_availability"`
	Amount                *string `json:"amount"`
	BillingLinkState      string  `json:"billing_link_state"`
	BillingRegisterID     *string `json:"billing_register_id,omitempty"`
	BillingRegisterItemID *string `json:"billing_register_item_id,omitempty"`
}

type payableSnapshotPayload struct {
	EventID            string `json:"event_id"`
	EventType          string `json:"event_type"`
	SchemaVersion      int    `json:"schema_version"`
	TenantID           string `json:"tenant_id"`
	SourceService      string `json:"source_service"`
	SourceType         string `json:"source_type"`
	SourceID           string `json:"source_id"`
	SourceRevision     int64  `json:"source_revision"`
	OccurredAt         string `json:"occurred_at"`
	CurrencyCode       string `json:"currency_code"`
	TaxBasis           string `json:"tax_basis"`
	EntryKind          string `json:"entry_kind"`
	AmountAvailability string `json:"amount_availability"`
	Amount             string `json:"amount"`
	RegisterStatus     string `json:"register_status"`
}

func CurrentActualAvailable(status string, openDisputeCount int) bool {
	if openDisputeCount > 0 {
		return false
	}
	switch status {
	case SettlementStatusApproved, SettlementStatusDocumentsReady, SettlementStatusReadyForPayment:
		return true
	default:
		return false
	}
}

func FinalActualAvailable(status string, openDisputeCount int) bool {
	return openDisputeCount == 0 && status == SettlementStatusReadyForPayment
}

func BuildSettlementSnapshotPayloads(
	eventTypes []string,
	settlement *FreightSettlement,
	openDisputeCount int,
	occurredAt time.Time,
) ([]uuid.UUID, []json.RawMessage, error) {
	if settlement == nil {
		return nil, nil, errors.New("settlement is required")
	}
	accrualAmount := FormatMoneyFloat(settlement.TotalWithoutVAT)
	var currentAmount, finalAmount *string
	currentAvail := AmountUnavailable
	finalAvail := AmountUnavailable
	if CurrentActualAvailable(settlement.Status, openDisputeCount) {
		currentAvail = AmountAvailable
		currentAmount = &accrualAmount
	}
	if FinalActualAvailable(settlement.Status, openDisputeCount) {
		finalAvail = AmountAvailable
		finalAmount = &accrualAmount
	}

	base := settlementSnapshotPayload{
		SchemaVersion:    FreightCostOutboxSchemaVersion,
		TenantID:         settlement.TenantID.String(),
		TransportOrderID: settlement.TransportOrderID.String(),
		ShipmentID:       settlement.ShipmentID.String(),
		SourceService:    FreightCostServiceName,
		SourceType:       AggregateFreightSettlement,
		SourceID:         settlement.ID.String(),
		SourceRevision:   int64(settlement.Version),
		OccurredAt:       occurredAt.UTC().Format(time.RFC3339Nano),
		CurrencyCode:     settlement.CurrencyCode,
		TaxBasis:         TaxBasisExVAT,
		SettlementStatus: settlement.Status,
		OpenDisputeCount: openDisputeCount,
	}

	ids := make([]uuid.UUID, 0, len(eventTypes))
	payloads := make([]json.RawMessage, 0, len(eventTypes))

	for _, eventType := range eventTypes {
		eventID := uuid.New()
		p := base
		p.EventID = eventID.String()
		p.EventType = eventType
		switch eventType {
		case EventFreightSettlementAccrualSnapshot:
			p.EntryKind = EntryKindAccrualCostSnapshot
			p.AmountAvailability = AmountAvailable
			p.Amount = &accrualAmount
		case EventFreightSettlementCurrentActualSnapshot:
			p.EntryKind = EntryKindCurrentActualCostSnapshot
			p.AmountAvailability = currentAvail
			p.Amount = currentAmount
		case EventFreightSettlementFinalActualSnapshot:
			p.EntryKind = EntryKindFinalActualCostSnapshot
			p.AmountAvailability = finalAvail
			p.Amount = finalAmount
		default:
			return nil, nil, errors.New("unsupported settlement snapshot event type")
		}
		raw, err := json.Marshal(p)
		if err != nil {
			return nil, nil, err
		}
		ids = append(ids, eventID)
		payloads = append(payloads, raw)
	}
	return ids, payloads, nil
}

func BuildBillingLinkSnapshotPayload(
	settlement *FreightSettlement,
	linkState string,
	amountExVAT *string,
	registerID, registerItemID *uuid.UUID,
	occurredAt time.Time,
) (uuid.UUID, json.RawMessage, error) {
	eventID := uuid.New()
	avail := AmountUnavailable
	var amount *string
	var regID, itemID *string
	if linkState == BillingLinkStateLinked && amountExVAT != nil {
		avail = AmountAvailable
		amount = amountExVAT
	}
	if registerID != nil {
		s := registerID.String()
		regID = &s
	}
	if registerItemID != nil {
		s := registerItemID.String()
		itemID = &s
	}
	payload := billingLinkSnapshotPayload{
		EventID:               eventID.String(),
		EventType:             EventBillingRegisterSettlementBillingLink,
		SchemaVersion:         FreightCostOutboxSchemaVersion,
		TenantID:              settlement.TenantID.String(),
		TransportOrderID:      settlement.TransportOrderID.String(),
		ShipmentID:            settlement.ShipmentID.String(),
		SourceService:         FreightCostServiceName,
		SourceType:            AggregateFreightSettlementBillingLink,
		SourceID:              settlement.ID.String(),
		SourceRevision:        settlement.BillingLinkRevision,
		OccurredAt:            occurredAt.UTC().Format(time.RFC3339Nano),
		CurrencyCode:          settlement.CurrencyCode,
		TaxBasis:              TaxBasisExVAT,
		EntryKind:             EntryKindBilledCostSnapshot,
		AmountAvailability:    avail,
		Amount:                amount,
		BillingLinkState:      linkState,
		BillingRegisterID:     regID,
		BillingRegisterItemID: itemID,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return uuid.Nil, nil, err
	}
	return eventID, raw, nil
}

func BuildPayableSnapshotPayload(register *BillingRegister, occurredAt time.Time) (uuid.UUID, json.RawMessage, error) {
	eventID := uuid.New()
	amount := FormatMoneyFloat(register.TotalWithVAT)
	payload := payableSnapshotPayload{
		EventID:            eventID.String(),
		EventType:          EventBillingRegisterPayableSnapshot,
		SchemaVersion:      FreightCostOutboxSchemaVersion,
		TenantID:           register.TenantID.String(),
		SourceService:      FreightCostServiceName,
		SourceType:         AggregateBillingRegister,
		SourceID:           register.ID.String(),
		SourceRevision:     int64(register.Version),
		OccurredAt:         occurredAt.UTC().Format(time.RFC3339Nano),
		CurrencyCode:       register.CurrencyCode,
		TaxBasis:           TaxBasisWithVAT,
		EntryKind:          EntryKindPayableAmountSnapshot,
		AmountAvailability: AmountAvailable,
		Amount:             amount,
		RegisterStatus:     register.Status,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return uuid.Nil, nil, err
	}
	return eventID, raw, nil
}

func FormatMoneyFloat(v float64) string {
	return decimal.NewFromFloat(v).StringFixed(MoneyScale)
}

func FormatMoneyDecimal(d decimal.Decimal) string {
	return d.StringFixed(MoneyScale)
}
