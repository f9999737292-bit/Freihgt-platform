package dto

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/freight-cost-service/internal/domain"
	apperrors "github.com/freight-platform/freight-cost-service/internal/platform/errors"
	"github.com/freight-platform/freight-cost-service/internal/service"
)

type SourceEventRequest struct {
	EventID                string         `json:"event_id"`
	EventType              string         `json:"event_type"`
	SchemaVersion          int            `json:"schema_version"`
	TenantID               string         `json:"tenant_id"`
	TransportOrderID       string         `json:"transport_order_id"`
	ShipmentID             *string        `json:"shipment_id"`
	BuyerCompanyID         string         `json:"buyer_company_id"`
	CarrierCompanyID       string         `json:"carrier_company_id"`
	EntryKind              string         `json:"entry_kind"`
	SourceService          string         `json:"source_service"`
	SourceType             string         `json:"source_type"`
	SourceID               string         `json:"source_id"`
	SourceRevision         int64          `json:"source_revision"`
	SourceRevisionSemantic *string        `json:"source_revision_semantic"`
	CurrencyCode           string         `json:"currency_code"`
	TaxBasis               string         `json:"tax_basis"`
	AmountAvailability     string         `json:"amount_availability"`
	Amount                 *string        `json:"amount"`
	OccurredAt             string         `json:"occurred_at"`
	EventOrigin            string         `json:"event_origin"`
	SettlementStatus       string         `json:"settlement_status"`
	OpenDisputeCount       int            `json:"open_dispute_count"`
	Metadata               map[string]any `json:"metadata"`
}

type SourceEventResponse struct {
	Outcome     string  `json:"outcome"`
	CostEntryID *string `json:"cost_entry_id,omitempty"`
}

func ToSourceEventInput(req SourceEventRequest, headerTenantID uuid.UUID) (service.SourceEventInput, error) {
	eventID, err := parseRequiredUUID("event_id", req.EventID)
	if err != nil {
		return service.SourceEventInput{}, err
	}
	tenantID, err := parseRequiredUUID("tenant_id", req.TenantID)
	if err != nil {
		return service.SourceEventInput{}, err
	}
	if tenantID != headerTenantID {
		return service.SourceEventInput{}, apperrors.Validation("tenant mismatch", map[string]any{"field": "tenant_id"})
	}
	transportOrderID, err := parseRequiredUUID("transport_order_id", req.TransportOrderID)
	if err != nil {
		return service.SourceEventInput{}, err
	}
	buyerCompanyID, err := parseRequiredUUID("buyer_company_id", req.BuyerCompanyID)
	if err != nil {
		return service.SourceEventInput{}, err
	}
	carrierCompanyID, err := parseRequiredUUID("carrier_company_id", req.CarrierCompanyID)
	if err != nil {
		return service.SourceEventInput{}, err
	}
	sourceID, err := parseRequiredUUID("source_id", req.SourceID)
	if err != nil {
		return service.SourceEventInput{}, err
	}
	var shipmentID *uuid.UUID
	if req.ShipmentID != nil && strings.TrimSpace(*req.ShipmentID) != "" {
		id, err := parseRequiredUUID("shipment_id", *req.ShipmentID)
		if err != nil {
			return service.SourceEventInput{}, err
		}
		shipmentID = &id
	}
	occurredAt, err := time.Parse(time.RFC3339, strings.TrimSpace(req.OccurredAt))
	if err != nil {
		return service.SourceEventInput{}, apperrors.Validation("invalid occurred_at", map[string]any{"field": "occurred_at"})
	}
	var amount *decimal.Decimal
	if req.Amount != nil {
		parsed, err := domain.ParseMoneyAmount(*req.Amount)
		if err != nil {
			return service.SourceEventInput{}, apperrors.Validation("invalid amount", map[string]any{"field": "amount"})
		}
		amount = &parsed
	}
	var revisionSemantic string
	if req.SourceRevisionSemantic != nil {
		revisionSemantic = strings.TrimSpace(*req.SourceRevisionSemantic)
	}
	origin := strings.TrimSpace(req.EventOrigin)
	if origin == "" {
		origin = domain.EventOriginLiveOutbox
	}
	return service.SourceEventInput{
		EventID:                eventID,
		EventType:              strings.TrimSpace(req.EventType),
		SchemaVersion:          req.SchemaVersion,
		TenantID:               tenantID,
		TransportOrderID:       transportOrderID,
		ShipmentID:             shipmentID,
		BuyerCompanyID:         buyerCompanyID,
		CarrierCompanyID:       carrierCompanyID,
		EntryKind:              strings.TrimSpace(req.EntryKind),
		SourceService:          strings.TrimSpace(req.SourceService),
		SourceType:             strings.TrimSpace(req.SourceType),
		SourceID:               sourceID,
		SourceRevision:         req.SourceRevision,
		SourceRevisionSemantic: revisionSemantic,
		CurrencyCode:           strings.TrimSpace(req.CurrencyCode),
		TaxBasis:               domain.TaxBasis(strings.TrimSpace(req.TaxBasis)),
		AmountAvailability:     strings.TrimSpace(req.AmountAvailability),
		Amount:                 amount,
		OccurredAt:             occurredAt.UTC(),
		EventOrigin:            origin,
		SettlementStatus:       strings.TrimSpace(req.SettlementStatus),
		OpenDisputeCount:       req.OpenDisputeCount,
		Metadata:               req.Metadata,
	}, nil
}

func ToSourceEventResponse(result service.IngestResult) SourceEventResponse {
	resp := SourceEventResponse{Outcome: result.Outcome}
	if result.CostEntryID != nil {
		value := result.CostEntryID.String()
		resp.CostEntryID = &value
	}
	return resp
}

type RebuildResponse struct {
	FactsProcessed int      `json:"facts_processed"`
	Outcomes       []string `json:"outcomes"`
}

func ToRebuildResponse(result service.RebuildResult) RebuildResponse {
	outcomes := make([]string, 0, len(result.Outcomes))
	for _, item := range result.Outcomes {
		outcomes = append(outcomes, item.Outcome)
	}
	return RebuildResponse{
		FactsProcessed: result.FactsProcessed,
		Outcomes:       outcomes,
	}
}

func parseRequiredUUID(field, raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(strings.TrimSpace(raw))
	if err != nil || id == uuid.Nil {
		return uuid.Nil, apperrors.Validation("invalid "+field, map[string]any{"field": field})
	}
	return id, nil
}
