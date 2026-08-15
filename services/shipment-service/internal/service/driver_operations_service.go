package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
	"github.com/freight-platform/shipment-service/internal/repository"
)

type DriverOperationsService struct {
	drivers    *repository.DriverRepository
	shipments  *repository.ShipmentRepository
	operations *repository.DriverOperationsRepository
}

func NewDriverOperationsService(
	drivers *repository.DriverRepository,
	shipments *repository.ShipmentRepository,
	operations *repository.DriverOperationsRepository,
) *DriverOperationsService {
	return &DriverOperationsService{
		drivers:    drivers,
		shipments:  shipments,
		operations: operations,
	}
}

type ResolvedDriver struct {
	Driver domain.Driver
	UserID uuid.UUID
}

func (s *DriverOperationsService) ResolveDriver(ctx context.Context, tenantID, userID uuid.UUID) (*ResolvedDriver, error) {
	if tenantID == uuid.Nil || userID == uuid.Nil {
		return nil, apperrors.Unauthorized("authentication required")
	}
	driver, err := s.drivers.GetByUserIDAndTenant(ctx, userID, tenantID)
	if err != nil {
		return nil, err
	}
	if driver.Status != domain.DriverStatusActive {
		return nil, apperrors.Unauthorized("driver is not active")
	}
	if driver.UserID == nil || *driver.UserID != userID {
		return nil, apperrors.Unauthorized("driver binding is invalid")
	}
	return &ResolvedDriver{Driver: *driver, UserID: userID}, nil
}

func (s *DriverOperationsService) GetMe(ctx context.Context, tenantID, userID uuid.UUID) (domain.DriverMeView, error) {
	resolved, err := s.ResolveDriver(ctx, tenantID, userID)
	if err != nil {
		return domain.DriverMeView{}, err
	}
	return domain.ToDriverMeView(&resolved.Driver), nil
}

func (s *DriverOperationsService) ListShipments(ctx context.Context, tenantID, userID uuid.UUID, filter domain.ListDriverShipmentsFilter) ([]domain.DriverShipmentSummary, int, error) {
	resolved, err := s.ResolveDriver(ctx, tenantID, userID)
	if err != nil {
		return nil, 0, err
	}
	filter.TenantID = tenantID
	filter.DriverID = resolved.Driver.ID
	if err := domain.ValidateListDriverShipmentsFilter(filter); err != nil {
		return nil, 0, err
	}
	shipments, total, err := s.shipments.ListByDriverID(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	items := make([]domain.DriverShipmentSummary, 0, len(shipments))
	for _, shipment := range shipments {
		items = append(items, domain.ToDriverShipmentSummary(shipment))
	}
	return items, total, nil
}

func (s *DriverOperationsService) GetShipment(ctx context.Context, tenantID, userID, shipmentID uuid.UUID) (domain.DriverShipmentDetail, error) {
	resolved, err := s.ResolveDriver(ctx, tenantID, userID)
	if err != nil {
		return domain.DriverShipmentDetail{}, err
	}
	shipment, err := s.shipments.GetByIDAndDriver(ctx, shipmentID, tenantID, resolved.Driver.ID)
	if err != nil {
		return domain.DriverShipmentDetail{}, err
	}
	return domain.ToDriverShipmentDetail(*shipment), nil
}

type DriverOperationalEventResult struct {
	ShipmentID     uuid.UUID
	EventType      string
	TargetStatus   *string
	ShipmentStatus string
	OccurredAt     time.Time
	ReceivedAt     time.Time
	Replayed       bool
	OutboxEventID  *uuid.UUID
}

func (s *DriverOperationsService) RecordOperationalEvent(
	ctx context.Context,
	tenantID, userID, shipmentID uuid.UUID,
	in domain.DriverOperationalEventInput,
	transition domain.StatusTransitionContext,
) (DriverOperationalEventResult, error) {
	if err := domain.ValidateDriverOperationalEventInput(in); err != nil {
		return DriverOperationalEventResult{}, err
	}
	resolved, err := s.ResolveDriver(ctx, tenantID, userID)
	if err != nil {
		return DriverOperationalEventResult{}, err
	}

	idempotencyKey := strings.TrimSpace(in.IdempotencyKey)
	if existing, err := s.operations.GetIdempotencyRecord(ctx, tenantID, resolved.Driver.ID, domain.DriverOperationTypeStatusEvent, idempotencyKey); err != nil {
		return DriverOperationalEventResult{}, err
	} else if existing != nil {
		var cached DriverOperationalEventResult
		if err := json.Unmarshal(existing.ResponseBody, &cached); err == nil {
			cached.Replayed = true
			return cached, nil
		}
	}

	shipment, err := s.shipments.GetByIDAndDriver(ctx, shipmentID, tenantID, resolved.Driver.ID)
	if err != nil {
		return DriverOperationalEventResult{}, err
	}

	receivedAt := time.Now().UTC()
	occurredAt := receivedAt
	if in.OccurredAt != nil {
		occurredAt = in.OccurredAt.UTC()
	}
	transition.OccurredAt = occurredAt
	reason := strings.TrimSpace(in.Type)
	transition.ReasonCode = &reason

	targetStatus, changesStatus, informational := domain.MapDriverEventToTargetStatus(strings.TrimSpace(in.Type))
	result := DriverOperationalEventResult{
		ShipmentID:     shipment.ID,
		EventType:      strings.TrimSpace(in.Type),
		ShipmentStatus: shipment.Status,
		OccurredAt:     occurredAt,
		ReceivedAt:     receivedAt,
	}

	if informational {
		result.TargetStatus = nil
		if err := s.saveStatusEventIdempotency(ctx, tenantID, resolved.Driver.ID, idempotencyKey, shipmentID, result); err != nil {
			return DriverOperationalEventResult{}, err
		}
		return result, nil
	}

	if !changesStatus {
		return DriverOperationalEventResult{}, apperrors.Validation("unsupported driver event type", map[string]any{"field": "type"})
	}
	result.TargetStatus = &targetStatus

	if err := domain.ValidateStatusTransition(shipment.Status, targetStatus); err != nil {
		return DriverOperationalEventResult{}, err
	}

	var actualPickup, actualDelivery *time.Time
	updateInput := domain.UpdateShipmentStatusInput{Status: targetStatus}
	switch targetStatus {
	case domain.ShipmentStatusLoaded:
		actualPickup = &occurredAt
		updateInput.ActualTime = actualPickup
	case domain.ShipmentStatusDelivered:
		actualDelivery = &occurredAt
		updateInput.ActualTime = actualDelivery
	}

	updated, err := s.shipments.UpdateStatus(ctx, shipment.ID, tenantID, shipment.Status, targetStatus, actualPickup, actualDelivery, shipment.Version, transition)
	if err != nil {
		return DriverOperationalEventResult{}, err
	}
	result.ShipmentStatus = updated.Status

	if err := s.saveStatusEventIdempotency(ctx, tenantID, resolved.Driver.ID, idempotencyKey, shipmentID, result); err != nil {
		return DriverOperationalEventResult{}, err
	}
	return result, nil
}

func (s *DriverOperationsService) saveStatusEventIdempotency(
	ctx context.Context,
	tenantID, driverID uuid.UUID,
	idempotencyKey string,
	shipmentID uuid.UUID,
	result DriverOperationalEventResult,
) error {
	body, err := json.Marshal(result)
	if err != nil {
		return err
	}
	tx, err := s.operations.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	rec := domain.DriverOperationIdempotencyRecord{
		TenantID:           tenantID,
		DriverID:           driverID,
		OperationType:      domain.DriverOperationTypeStatusEvent,
		IdempotencyKey:     idempotencyKey,
		ResourceType:       "shipment",
		ResourceID:         shipmentID,
		ResponseStatusCode: 200,
		ResponseBody:       body,
	}
	if err := s.operations.SaveIdempotencyRecord(ctx, tx, rec); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type DriverExceptionResult struct {
	Exception     domain.DriverReportedException
	OutboxEventID *uuid.UUID
	Replayed      bool
}

func (s *DriverOperationsService) ReportException(
	ctx context.Context,
	tenantID, userID, shipmentID uuid.UUID,
	in domain.DriverExceptionInput,
	correlationID *string,
) (DriverExceptionResult, error) {
	if err := domain.ValidateDriverExceptionInput(in); err != nil {
		return DriverExceptionResult{}, err
	}
	resolved, err := s.ResolveDriver(ctx, tenantID, userID)
	if err != nil {
		return DriverExceptionResult{}, err
	}

	idempotencyKey := strings.TrimSpace(in.IdempotencyKey)
	if existing, err := s.operations.GetIdempotencyRecord(ctx, tenantID, resolved.Driver.ID, domain.DriverOperationTypeException, idempotencyKey); err != nil {
		return DriverExceptionResult{}, err
	} else if existing != nil {
		var cached DriverExceptionResult
		if err := json.Unmarshal(existing.ResponseBody, &cached); err == nil {
			cached.Replayed = true
			return cached, nil
		}
	}

	shipment, err := s.shipments.GetByIDAndDriver(ctx, shipmentID, tenantID, resolved.Driver.ID)
	if err != nil {
		return DriverExceptionResult{}, err
	}

	receivedAt := time.Now().UTC()
	occurredAt := receivedAt
	if in.OccurredAt != nil {
		occurredAt = in.OccurredAt.UTC()
	}
	category := strings.TrimSpace(strings.ToUpper(in.Category))
	excInput := domain.DriverReportedException{
		TenantID:       tenantID,
		ShipmentID:     shipmentID,
		DriverID:       resolved.Driver.ID,
		Category:       category,
		Comment:        domain.SanitizeDriverExceptionComment(in.Comment),
		OccurredAt:     occurredAt,
		ReceivedAt:     receivedAt,
		Source:         domain.DriverExceptionSource,
		IdempotencyKey: idempotencyKey,
	}

	exc, outboxID, err := s.operations.ReportException(ctx, repository.ReportDriverExceptionParams{
		Exception:       excInput,
		ShipmentVersion: shipment.Version,
		CorrelationID:   correlationID,
	})
	if err != nil {
		return DriverExceptionResult{}, err
	}

	result := DriverExceptionResult{Exception: *exc}
	if outboxID != uuid.Nil {
		result.OutboxEventID = &outboxID
	} else {
		result.Replayed = true
	}

	body, err := json.Marshal(result)
	if err != nil {
		return DriverExceptionResult{}, err
	}
	tx, err := s.operations.Begin(ctx)
	if err != nil {
		return DriverExceptionResult{}, err
	}
	defer tx.Rollback(ctx)
	if err := s.operations.SaveIdempotencyRecord(ctx, tx, domain.DriverOperationIdempotencyRecord{
		TenantID:           tenantID,
		DriverID:           resolved.Driver.ID,
		OperationType:      domain.DriverOperationTypeException,
		IdempotencyKey:     idempotencyKey,
		ResourceType:       "shipment",
		ResourceID:         shipmentID,
		ResponseStatusCode: 201,
		ResponseBody:       body,
	}); err != nil {
		return DriverExceptionResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return DriverExceptionResult{}, err
	}
	return result, nil
}
