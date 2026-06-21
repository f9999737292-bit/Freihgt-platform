package domain

import (
	"strings"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

type RfxLot struct {
	ID            uuid.UUID
	TenantID      uuid.UUID
	RfxEventID    uuid.UUID
	LotNumber     string
	Name          string
	Description   *string
	Category      *string
	EstimatedValue *float64
	CurrencyCode  *string
	Status        string
}

type CreateRfxLotInput struct {
	TenantID       uuid.UUID
	RfxEventID     uuid.UUID
	LotNumber      string
	Name           string
	Description    *string
	Category       *string
	EstimatedValue *float64
	CurrencyCode   *string
}

type RfxLane struct {
	ID                   uuid.UUID
	TenantID             uuid.UUID
	RfxLotID             uuid.UUID
	OriginLocationID     *uuid.UUID
	DestinationLocationID *uuid.UUID
	TransportMode        string
	EquipmentType        *string
	EstimatedVolume      *float64
	VolumeUnit           *string
	RequiredServiceLevel *string
}

type CreateRfxLaneInput struct {
	TenantID              uuid.UUID
	RfxLotID              uuid.UUID
	OriginLocationID      uuid.UUID
	DestinationLocationID uuid.UUID
	TransportMode         string
	EquipmentType         *string
	EstimatedVolume       *float64
	VolumeUnit            *string
	RequiredServiceLevel  *string
}

func ValidateCreateRfxLotInput(in CreateRfxLotInput) error {
	if in.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if in.RfxEventID == uuid.Nil {
		return apperrors.Validation("rfx_event_id is required", map[string]any{"field": "rfx_event_id"})
	}
	if strings.TrimSpace(in.LotNumber) == "" {
		return apperrors.Validation("lot_number is required", map[string]any{"field": "lot_number"})
	}
	if strings.TrimSpace(in.Name) == "" {
		return apperrors.Validation("name is required", map[string]any{"field": "name"})
	}
	if in.EstimatedValue != nil {
		return ValidateNonNegativeAmount(*in.EstimatedValue, "estimated_value")
	}
	return nil
}

func ValidateCreateRfxLaneInput(in CreateRfxLaneInput) error {
	if in.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if in.RfxLotID == uuid.Nil {
		return apperrors.Validation("rfx_lot_id is required", map[string]any{"field": "rfx_lot_id"})
	}
	if in.OriginLocationID == uuid.Nil {
		return apperrors.Validation("origin_location_id is required", map[string]any{"field": "origin_location_id"})
	}
	if in.DestinationLocationID == uuid.Nil {
		return apperrors.Validation("destination_location_id is required", map[string]any{"field": "destination_location_id"})
	}
	return ValidateTransportMode(in.TransportMode)
}
