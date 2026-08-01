package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

type ActorType string

const (
	ActorTypeUser   ActorType = "USER"
	ActorTypeSystem ActorType = "SYSTEM"
)

type StatusHistorySource string

const (
	StatusHistorySourceShipmentService StatusHistorySource = "SHIPMENT_SERVICE"
)

const (
	StatusHistoryWarningPartial = "SHIPMENT_STATUS_HISTORY_PARTIAL"
)

type ShipmentStatusHistory struct {
	ID              uuid.UUID
	TenantID        uuid.UUID
	ShipmentID      uuid.UUID
	ShipmentVersion int

	FromStatus    *string
	ToStatus      string
	ReasonCode    *string
	Source        string
	ActorType     ActorType
	ActorID       *uuid.UUID
	CorrelationID *string
	OccurredAt    time.Time
	RecordedAt    time.Time
}

type StatusTransitionContext struct {
	ActorType     ActorType
	ActorID       *uuid.UUID
	CorrelationID *string
	Source        StatusHistorySource
	OccurredAt    time.Time
	ReasonCode    *string
}

func NewUserTransitionContext(userID uuid.UUID, correlationID *string, occurredAt time.Time) StatusTransitionContext {
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	return StatusTransitionContext{
		ActorType:     ActorTypeUser,
		ActorID:       &userID,
		CorrelationID: correlationID,
		Source:        StatusHistorySourceShipmentService,
		OccurredAt:    occurredAt.UTC(),
	}
}

func NewSystemTransitionContext(source StatusHistorySource, correlationID *string, occurredAt time.Time) StatusTransitionContext {
	if source == "" {
		source = StatusHistorySourceShipmentService
	}
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	return StatusTransitionContext{
		ActorType:     ActorTypeSystem,
		ActorID:       nil,
		CorrelationID: correlationID,
		Source:        source,
		OccurredAt:    occurredAt.UTC(),
	}
}

func ValidateStatusTransitionContext(ctx StatusTransitionContext) error {
	switch ctx.ActorType {
	case ActorTypeUser:
		if ctx.ActorID == nil || *ctx.ActorID == uuid.Nil {
			return apperrors.Validation("actor_id is required for USER transitions", map[string]any{"field": "actor_id"})
		}
	case ActorTypeSystem:
		if ctx.ActorID != nil {
			return apperrors.Validation("actor_id must be omitted for SYSTEM transitions", map[string]any{"field": "actor_id"})
		}
	default:
		return apperrors.Validation("invalid actor_type", map[string]any{"field": "actor_type", "value": ctx.ActorType})
	}
	if ctx.Source != StatusHistorySourceShipmentService {
		return apperrors.Validation("invalid status history source", map[string]any{"field": "source", "value": ctx.Source})
	}
	if ctx.OccurredAt.IsZero() {
		return apperrors.Validation("occurred_at is required", map[string]any{"field": "occurred_at"})
	}
	return nil
}

type ListStatusHistoryFilter struct {
	TenantID   uuid.UUID
	ShipmentID uuid.UUID
	Page       int
	Limit      int
	Order      string
}

func ValidateListStatusHistoryFilter(f ListStatusHistoryFilter) error {
	if f.TenantID == uuid.Nil {
		return apperrors.Unauthorized("tenant context is required")
	}
	if f.ShipmentID == uuid.Nil {
		return apperrors.Validation("shipment_id is required", map[string]any{"field": "shipment_id"})
	}
	if f.Page <= 0 {
		f.Page = 1
	}
	if f.Limit <= 0 {
		f.Limit = 50
	}
	if f.Limit > 200 {
		return apperrors.Validation("limit must be at most 200", map[string]any{"field": "limit"})
	}
	order := strings.ToLower(strings.TrimSpace(f.Order))
	if order != "" && order != "asc" && order != "desc" {
		return apperrors.Validation("order must be asc or desc", map[string]any{"field": "order"})
	}
	return nil
}

func StatusHistoryIsComplete(items []ShipmentStatusHistory) bool {
	for i := range items {
		if items[i].FromStatus == nil {
			return true
		}
	}
	return false
}
