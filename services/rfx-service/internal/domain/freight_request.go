package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

const (
	FreightRequestStatusDraft          = "DRAFT"
	FreightRequestStatusPublished      = "PUBLISHED"
	FreightRequestStatusResponsesOpen  = "RESPONSES_OPEN"
	FreightRequestStatusAwarded        = "AWARDED"

	TransportOrderStatusReadyForSourcing   = "READY_FOR_SOURCING"
	TransportOrderStatusSourcingInProgress = "SOURCING_IN_PROGRESS"
)

var allowedFreightRequestTypes = map[string]struct{}{
	"SPOT": {}, "MINI_TENDER": {}, "LANE_TENDER": {}, "CONTRACT_TENDER": {},
	"SEASONAL_TENDER": {}, "PROJECT_TENDER": {},
}

type FreightRequest struct {
	ID                   uuid.UUID
	TenantID             uuid.UUID
	FreightRequestNumber string
	TransportOrderID     *uuid.UUID
	RequestType          string
	ShipperCompanyID     uuid.UUID
	Status               string
	ResponseDeadline     *time.Time
	CurrencyCode         *string
	CreatedAt            time.Time
	UpdatedAt            time.Time
	Version              int
}

type CreateFreightRequestFromOrderInput struct {
	TenantID             uuid.UUID
	TransportOrderID     uuid.UUID
	FreightRequestNumber string
	RequestType          string
	ShipperCompanyID     uuid.UUID
	ResponseDeadline     *time.Time
	CurrencyCode         *string
}

type ListFreightRequestsFilter struct {
	TenantID         uuid.UUID
	RequestType      *string
	Status           *string
	ShipperCompanyID *uuid.UUID
	Limit            int
	Offset           int
}

func ValidateCreateFreightRequestInput(in CreateFreightRequestFromOrderInput) error {
	if in.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if in.TransportOrderID == uuid.Nil {
		return apperrors.Validation("transport_order_id is required", map[string]any{"field": "transport_order_id"})
	}
	if strings.TrimSpace(in.FreightRequestNumber) == "" {
		return apperrors.Validation("freight_request_number is required", map[string]any{"field": "freight_request_number"})
	}
	if err := validateFreightRequestType(in.RequestType); err != nil {
		return err
	}
	if in.ShipperCompanyID == uuid.Nil {
		return apperrors.Validation("shipper_company_id is required", map[string]any{"field": "shipper_company_id"})
	}
	return ValidateFutureDeadline(in.ResponseDeadline, "response_deadline")
}

func ValidatePublishFreightRequest(status string) error {
	if status != FreightRequestStatusDraft {
		return apperrors.Validation("freight request can only be published from DRAFT status", map[string]any{"field": "status", "status": status})
	}
	return nil
}

func ValidateTransportOrderForFreightRequest(status string) error {
	if status != TransportOrderStatusReadyForSourcing && status != TransportOrderStatusSourcingInProgress {
		return apperrors.Validation("transport order must be READY_FOR_SOURCING or SOURCING_IN_PROGRESS", map[string]any{"field": "transport_order_id", "status": status})
	}
	return nil
}

func ValidateListFreightRequestsFilter(f ListFreightRequestsFilter) error {
	if f.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if f.Limit <= 0 || f.Limit > 100 {
		return apperrors.Validation("invalid limit", map[string]any{"field": "limit"})
	}
	if f.Offset < 0 {
		return apperrors.Validation("offset must be >= 0", map[string]any{"field": "offset"})
	}
	return nil
}

func validateFreightRequestType(value string) error {
	value = strings.TrimSpace(value)
	if _, ok := allowedFreightRequestTypes[value]; !ok {
		return apperrors.Validation("invalid request_type", map[string]any{"field": "request_type", "value": value})
	}
	return nil
}

func ValidateFreightRequestForBid(status string) error {
	if status != FreightRequestStatusPublished && status != FreightRequestStatusResponsesOpen {
		return apperrors.Validation("freight request must be PUBLISHED or RESPONSES_OPEN to accept bids", map[string]any{"field": "status", "status": status})
	}
	return nil
}
