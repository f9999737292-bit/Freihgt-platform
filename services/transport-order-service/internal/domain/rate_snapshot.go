package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	apperrors "github.com/freight-platform/transport-order-service/internal/platform/errors"
)

const (
	PricingModelVersionSnapshotV1 = "SNAPSHOT_V1"
	ResolvedServiceContractRate   = "contract-rate-service"
	ResolverVersionSnapshotV1     = "v2.0C"
	MoneyScale                    = 2
)

type PricingContext struct {
	CarrierCompanyID   uuid.UUID  `json:"carrier_company_id"`
	AwardLinkID        *uuid.UUID `json:"award_link_id,omitempty"`
	AwardScopeEventID  *uuid.UUID `json:"award_scope_event_id,omitempty"`
	AwardScopeLotID    *uuid.UUID `json:"award_scope_lot_id,omitempty"`
	BidID              *uuid.UUID `json:"bid_id,omitempty"`
	ManualSpotAmount   *string    `json:"manual_spot_amount,omitempty"`
	ManualSpotCurrency *string    `json:"manual_spot_currency,omitempty"`
	PricingSource      *string    `json:"pricing_source,omitempty"`
}

type InternalActor struct {
	TenantID  uuid.UUID
	UserID    uuid.UUID
	CompanyID uuid.UUID
	ActorKind string
}

type CreatePricedTransportOrderInput struct {
	CreateTransportOrderInput
	Actor           InternalActor
	PricingContext  PricingContext
	IdempotencyKey  string
}

type CreateFromAwardScopeInput struct {
	TenantID              uuid.UUID
	ActorUserID           uuid.UUID
	ActorCompanyID        uuid.UUID
	IdempotencyKey        string
	RfxEventID            uuid.UUID
	RfxLotID              *uuid.UUID
	OrderNumber           string
	ShipperCompanyID      uuid.UUID
	ConsigneeCompanyID    uuid.UUID
	OriginLocationID      uuid.UUID
	DestinationLocationID uuid.UUID
	CargoID               uuid.UUID
	TransportMode         string
	EquipmentType         string
	CarrierCompanyID      uuid.UUID
	SourceSystem          *string
	ExternalReference     *string
	CreatedBy             *uuid.UUID
}

type RateSnapshot struct {
	ID                       uuid.UUID
	TenantID                 uuid.UUID
	TransportOrderID         uuid.UUID
	BuyerCompanyID           uuid.UUID
	CarrierCompanyID         uuid.UUID
	PricingSource            string
	AwardLinkID              *uuid.UUID
	RfxEventID               *uuid.UUID
	RfxLotID                 *uuid.UUID
	BidID                    *uuid.UUID
	ManualSpotAuditID        *uuid.UUID
	ContractID               *uuid.UUID
	RateCardID               *uuid.UUID
	RateVersionID            *uuid.UUID
	RateLineID               *uuid.UUID
	ContractNumber           *string
	RateCardName             *string
	RateVersionNumber        *int
	OriginLocationID         uuid.UUID
	DestinationLocationID    uuid.UUID
	EquipmentType            string
	TransportMode            string
	CurrencyCode             string
	ComponentBreakdownStatus string
	Components               json.RawMessage
	AccessorialRules         json.RawMessage
	BaseAmount               *decimal.Decimal
	TotalAmount              decimal.Decimal
	PricingDate              time.Time
	ResolvedAt               time.Time
	ResolvedByService        string
	ResolverVersion          string
	ResolutionRequestHash    string
	CreatedAt                time.Time
}

type PricedTransportOrderResult struct {
	Order          *TransportOrder
	RateSnapshot   *RateSnapshot
	RateSnapshotID uuid.UUID
}

type ResolveRateResult struct {
	Status                   string          `json:"status"`
	PricingSource            string          `json:"pricing_source,omitempty"`
	ContractID               *uuid.UUID      `json:"contract_id,omitempty"`
	RateCardID               *uuid.UUID      `json:"rate_card_id,omitempty"`
	RateVersionID            *uuid.UUID      `json:"rate_version_id,omitempty"`
	RateLineID               *uuid.UUID      `json:"rate_line_id,omitempty"`
	ContractNumber           *string         `json:"contract_number,omitempty"`
	RateCardName             *string         `json:"rate_card_name,omitempty"`
	VersionNumber            *int            `json:"version_number,omitempty"`
	CurrencyCode             *string         `json:"currency_code,omitempty"`
	ComponentBreakdownStatus string          `json:"component_breakdown_status,omitempty"`
	BaseAmount               *string         `json:"base_amount,omitempty"`
	TotalAmount              *string         `json:"total_amount,omitempty"`
	Components               json.RawMessage `json:"components,omitempty"`
	AccessorialRules         json.RawMessage `json:"accessorial_rules,omitempty"`
	PricingDate              string          `json:"pricing_date,omitempty"`
	ResolvedAt               time.Time       `json:"resolved_at"`
	ResolverVersion          string          `json:"resolver_version"`
	ManualSpotAuditID        *uuid.UUID      `json:"manual_spot_audit_id,omitempty"`
	AwardLinkID              *uuid.UUID      `json:"award_link_id,omitempty"`
	BidID                    *uuid.UUID      `json:"bid_id,omitempty"`
	RfxEventID               *uuid.UUID      `json:"rfx_event_id,omitempty"`
	RfxLotID                 *uuid.UUID      `json:"rfx_lot_id,omitempty"`
	CarrierCompanyID         *uuid.UUID      `json:"carrier_company_id,omitempty"`
	BuyerCompanyID           *uuid.UUID      `json:"buyer_company_id,omitempty"`
}

type CreateIdempotencyRecord struct {
	RequestHash      string
	TransportOrderID uuid.UUID
	RateSnapshotID   uuid.UUID
}

type createRequestHashPayload struct {
	TenantID              string  `json:"tenant_id"`
	ActorCompanyID        string  `json:"actor_company_id"`
	OrderNumber           string  `json:"order_number"`
	ShipperCompanyID      string  `json:"shipper_company_id"`
	ConsigneeCompanyID    string  `json:"consignee_company_id"`
	CarrierCompanyID      *string `json:"carrier_company_id,omitempty"`
	OriginLocationID      string  `json:"origin_location_id"`
	DestinationLocationID string  `json:"destination_location_id"`
	CargoID               string  `json:"cargo_id"`
	RequestedPickupDate   *string `json:"requested_pickup_date,omitempty"`
	RequestedDeliveryDate *string `json:"requested_delivery_date,omitempty"`
	TransportMode         string  `json:"transport_mode"`
	EquipmentType         string  `json:"equipment_type"`
	SourceSystem          *string `json:"source_system,omitempty"`
	ExternalReference     *string `json:"external_reference,omitempty"`
	AwardLinkID           *string `json:"award_link_id,omitempty"`
	AwardScopeEventID     *string `json:"award_scope_event_id,omitempty"`
	AwardScopeLotID       *string `json:"award_scope_lot_id,omitempty"`
	BidID                 *string `json:"bid_id,omitempty"`
	ManualSpotAmount      *string `json:"manual_spot_amount,omitempty"`
	ManualSpotCurrency    *string `json:"manual_spot_currency,omitempty"`
	PricingSource         *string `json:"pricing_source,omitempty"`
}

type resolutionHashPayload struct {
	TenantID              string  `json:"tenant_id"`
	BuyerCompanyID        string  `json:"buyer_company_id"`
	CarrierCompanyID      string  `json:"carrier_company_id"`
	OriginLocationID      string  `json:"origin_location_id"`
	DestinationLocationID string  `json:"destination_location_id"`
	EquipmentType         string  `json:"equipment_type"`
	TransportMode         string  `json:"transport_mode"`
	PricingDate           string  `json:"pricing_date"`
	AwardLinkID           *string `json:"award_link_id,omitempty"`
	AwardScopeEventID     *string `json:"award_scope_event_id,omitempty"`
	AwardScopeLotID       *string `json:"award_scope_lot_id,omitempty"`
	BidID                 *string `json:"bid_id,omitempty"`
	ManualSpotAmount      *string `json:"manual_spot_amount,omitempty"`
	ManualSpotCurrency    *string `json:"manual_spot_currency,omitempty"`
	PricingSource         *string `json:"pricing_source,omitempty"`
}

func ValidatePricingContext(ctx PricingContext) error {
	hasAward := ctx.AwardLinkID != nil || ctx.AwardScopeEventID != nil
	hasBid := ctx.BidID != nil
	hasManual := ctx.ManualSpotAmount != nil
	if hasAward && hasBid {
		return apperrors.Validation("award and bid pricing identifiers are mutually exclusive", map[string]any{"field": "pricing_context"})
	}
	if hasManual && (hasAward || hasBid) {
		return apperrors.Validation("manual spot cannot be combined with award or bid references", map[string]any{"field": "pricing_context"})
	}
	if !hasAward && !hasBid && ctx.CarrierCompanyID == uuid.Nil {
		return apperrors.Validation("carrier_company_id is required in pricing_context", map[string]any{"field": "pricing_context.carrier_company_id"})
	}
	if hasManual && trimOptionalString(ctx.ManualSpotCurrency) == nil {
		return apperrors.Validation("manual_spot_currency is required with manual_spot_amount", map[string]any{"field": "pricing_context.manual_spot_currency"})
	}
	return nil
}

func ValidateCreatePricedTransportOrderInput(in CreatePricedTransportOrderInput) error {
	if err := ValidateCreateTransportOrderInput(in.CreateTransportOrderInput); err != nil {
		return err
	}
	if strings.TrimSpace(in.IdempotencyKey) == "" {
		return apperrors.Validation("idempotency_key is required for priced transport order create", map[string]any{"field": "idempotency_key"})
	}
	if in.Actor.TenantID == uuid.Nil || in.Actor.UserID == uuid.Nil || in.Actor.CompanyID == uuid.Nil {
		return apperrors.Validation("actor context is required", map[string]any{"field": "actor"})
	}
	if strings.TrimSpace(in.Actor.ActorKind) == "" {
		return apperrors.Validation("actor_kind is required", map[string]any{"field": "actor_kind"})
	}
	if in.EquipmentType == nil || strings.TrimSpace(*in.EquipmentType) == "" {
		return apperrors.Validation("equipment_type is required for priced transport order create", map[string]any{"field": "equipment_type"})
	}
	return ValidatePricingContext(in.PricingContext)
}

func ValidateCreateFromAwardScopeInput(in CreateFromAwardScopeInput) error {
	if in.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if strings.TrimSpace(in.IdempotencyKey) == "" {
		return apperrors.Validation("idempotency_key is required", map[string]any{"field": "idempotency_key"})
	}
	if in.RfxEventID == uuid.Nil {
		return apperrors.Validation("rfx_event_id is required", map[string]any{"field": "rfx_event_id"})
	}
	if strings.TrimSpace(in.OrderNumber) == "" {
		return apperrors.Validation("order_number is required", map[string]any{"field": "order_number"})
	}
	if in.ShipperCompanyID == uuid.Nil || in.ConsigneeCompanyID == uuid.Nil {
		return apperrors.Validation("shipper and consignee company ids are required", nil)
	}
	if in.OriginLocationID == uuid.Nil || in.DestinationLocationID == uuid.Nil {
		return apperrors.Validation("origin and destination location ids are required", nil)
	}
	if in.CargoID == uuid.Nil {
		return apperrors.Validation("cargo_id is required", map[string]any{"field": "cargo_id"})
	}
	if strings.TrimSpace(in.EquipmentType) == "" {
		return apperrors.Validation("equipment_type is required", map[string]any{"field": "equipment_type"})
	}
	if in.CarrierCompanyID == uuid.Nil {
		return apperrors.Validation("carrier_company_id is required", map[string]any{"field": "carrier_company_id"})
	}
	if err := validateTransportMode(in.TransportMode); err != nil {
		return err
	}
	return nil
}

func ComputeCreateRequestHash(in CreatePricedTransportOrderInput) (string, error) {
	payload := createRequestHashPayload{
		TenantID:              in.TenantID.String(),
		ActorCompanyID:        in.Actor.CompanyID.String(),
		OrderNumber:           strings.TrimSpace(in.OrderNumber),
		ShipperCompanyID:      in.ShipperCompanyID.String(),
		ConsigneeCompanyID:    in.ConsigneeCompanyID.String(),
		CarrierCompanyID:      uuidPtrString(nonNilUUIDPtr(in.PricingContext.CarrierCompanyID)),
		OriginLocationID:      in.OriginLocationID.String(),
		DestinationLocationID: in.DestinationLocationID.String(),
		CargoID:               in.CargoID.String(),
		RequestedPickupDate:   formatDatePtr(in.RequestedPickupDate),
		RequestedDeliveryDate: formatDatePtr(in.RequestedDeliveryDate),
		TransportMode:         NormalizeTransportMode(in.TransportMode),
		EquipmentType:         strings.ToUpper(strings.TrimSpace(derefString(in.EquipmentType))),
		SourceSystem:          in.SourceSystem,
		ExternalReference:     in.ExternalReference,
		AwardLinkID:           uuidPtrString(in.PricingContext.AwardLinkID),
		AwardScopeEventID:     uuidPtrString(in.PricingContext.AwardScopeEventID),
		AwardScopeLotID:       uuidPtrString(in.PricingContext.AwardScopeLotID),
		BidID:                 uuidPtrString(in.PricingContext.BidID),
		ManualSpotAmount:      trimOptionalString(in.PricingContext.ManualSpotAmount),
		ManualSpotCurrency:    trimOptionalString(in.PricingContext.ManualSpotCurrency),
		PricingSource:         trimOptionalString(in.PricingContext.PricingSource),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", apperrors.Internal("marshal create request hash payload", err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func nonNilUUIDPtr(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}
	return &id
}

func formatDatePtr(value *time.Time) *string {
	if value == nil {
		return nil
	}
	s := value.Format("2006-01-02")
	return &s
}

func ComputeResolutionRequestHash(in CreatePricedTransportOrderInput, pricingDate time.Time) (string, error) {
	payload := resolutionHashPayload{
		TenantID:              in.TenantID.String(),
		BuyerCompanyID:        in.ShipperCompanyID.String(),
		CarrierCompanyID:      in.PricingContext.CarrierCompanyID.String(),
		OriginLocationID:      in.OriginLocationID.String(),
		DestinationLocationID: in.DestinationLocationID.String(),
		EquipmentType:         strings.ToUpper(strings.TrimSpace(derefString(in.EquipmentType))),
		TransportMode:         NormalizeTransportMode(in.TransportMode),
		PricingDate:           pricingDate.Format("2006-01-02"),
		AwardLinkID:           uuidPtrString(in.PricingContext.AwardLinkID),
		AwardScopeEventID:     uuidPtrString(in.PricingContext.AwardScopeEventID),
		AwardScopeLotID:       uuidPtrString(in.PricingContext.AwardScopeLotID),
		BidID:                 uuidPtrString(in.PricingContext.BidID),
		ManualSpotAmount:      trimOptionalString(in.PricingContext.ManualSpotAmount),
		ManualSpotCurrency:    trimOptionalString(in.PricingContext.ManualSpotCurrency),
		PricingSource:         trimOptionalString(in.PricingContext.PricingSource),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", apperrors.Internal("marshal resolution hash payload", err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func PricingDateForOrder(in CreateTransportOrderInput) time.Time {
	if in.RequestedPickupDate != nil {
		return in.RequestedPickupDate.UTC().Truncate(24 * time.Hour)
	}
	return time.Now().UTC().Truncate(24 * time.Hour)
}

func ValidateUpdateWithSnapshot(current *TransportOrder, in UpdateTransportOrderInput) error {
	if current.PricingModelVersion == nil || *current.PricingModelVersion != PricingModelVersionSnapshotV1 {
		return nil
	}
	if in.EquipmentType != nil {
		return apperrors.Validation("equipment_type cannot be changed when rate snapshot exists", map[string]any{"field": "equipment_type"})
	}
	if in.TransportMode != nil {
		return apperrors.Validation("transport_mode cannot be changed when rate snapshot exists", map[string]any{"field": "transport_mode"})
	}
	if in.RequestedPickupDate != nil {
		return apperrors.Validation("requested_pickup_date cannot be changed when rate snapshot exists", map[string]any{"field": "requested_pickup_date"})
	}
	return nil
}

func EmptyJSONArray() json.RawMessage {
	return json.RawMessage("[]")
}

func NormalizePricingSource(source string) string {
	return strings.ToUpper(strings.TrimSpace(source))
}

func derefString(v *string) string {
	return DerefString(v)
}

func DerefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func uuidPtrString(v *uuid.UUID) *string {
	if v == nil || *v == uuid.Nil {
		return nil
	}
	s := v.String()
	return &s
}

func trimOptionalString(v *string) *string {
	if v == nil {
		return nil
	}
	s := strings.TrimSpace(*v)
	if s == "" {
		return nil
	}
	return &s
}

func SortedComponentJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return EmptyJSONArray()
	}
	var items []map[string]any
	if err := json.Unmarshal(raw, &items); err != nil {
		return raw
	}
	sort.Slice(items, func(i, j int) bool {
		a, _ := items[i]["component_type"].(string)
		b, _ := items[j]["component_type"].(string)
		return a < b
	})
	out, err := json.Marshal(items)
	if err != nil {
		return raw
	}
	return out
}
