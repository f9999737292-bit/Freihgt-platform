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

type driverIdentityStore interface {
	GetByUserIDAndTenant(ctx context.Context, userID, tenantID uuid.UUID) (*domain.Driver, error)
}

type driverShipmentStore interface {
	GetByIDAndDriver(ctx context.Context, id, tenantID, driverID uuid.UUID) (*domain.Shipment, error)
	ListByDriverID(ctx context.Context, filter domain.ListDriverShipmentsFilter) ([]domain.Shipment, int, error)
	UpdateStatus(ctx context.Context, id, tenantID uuid.UUID, fromStatus, newStatus string, actualPickupAt, actualDeliveryAt *time.Time, expectedVersion int, transition domain.StatusTransitionContext) (*domain.Shipment, error)
	UpdateStatusWithCargoEvidence(ctx context.Context, id, tenantID uuid.UUID, fromStatus, newStatus string, actualPickupAt, actualDeliveryAt *time.Time, expectedVersion int, transition domain.StatusTransitionContext, intent domain.CargoEvidenceIntent) (*domain.Shipment, error)
}

type driverOperationStore interface {
	GetIdempotencyRecord(ctx context.Context, tenantID, driverID uuid.UUID, operationType, idempotencyKey string) (*domain.DriverOperationIdempotencyRecord, error)
	CommitIdempotency(ctx context.Context, rec domain.DriverOperationIdempotencyRecord) error
	InsertDriverEventOutbox(ctx context.Context, params domain.BuildDriverEventParams) (uuid.UUID, error)
	ReportException(ctx context.Context, params repository.ReportDriverExceptionParams) (*domain.DriverReportedException, uuid.UUID, error)
	ReportDelay(ctx context.Context, params repository.ReportDriverDelayParams) (*domain.DriverReportedDelay, uuid.UUID, error)
}

type driverExecutionContextStore interface {
	ResolveDriverExecutionContext(ctx context.Context, operatingTenantID, driverID, shipmentID uuid.UUID, executionStopID, actionID *uuid.UUID) (repository.ResolvedDriverExecutionContext, error)
}

type DriverOperationsService struct {
	drivers          driverIdentityStore
	shipments        driverShipmentStore
	operations       driverOperationStore
	departure        *repository.TransportExecutionCommandRepository
	executionContext driverExecutionContextStore
}

func NewDriverOperationsService(
	drivers driverIdentityStore,
	shipments driverShipmentStore,
	operations driverOperationStore,
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
	if strings.EqualFold(strings.TrimSpace(in.Type), "DEPARTED_PICKUP") && s.departure != nil {
		dispatched, handled, dispatchErr := s.dispatchMultistopDeparture(ctx, tenantID, resolved.Driver.ID, shipmentID, idempotencyKey, in)
		if dispatchErr != nil {
			return DriverOperationalEventResult{}, dispatchErr
		}
		if handled {
			return dispatched, nil
		}
	}
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

	var updated *domain.Shipment
	var updateErr error
	if intent := domain.CargoEvidenceIntentForDriverEvent(strings.TrimSpace(in.Type), userID, resolved.Driver.ID, occurredAt); intent != nil {
		updated, updateErr = s.shipments.UpdateStatusWithCargoEvidence(ctx, shipment.ID, tenantID, shipment.Status, targetStatus, actualPickup, actualDelivery, shipment.Version, transition, *intent)
	} else {
		updated, updateErr = s.shipments.UpdateStatus(ctx, shipment.ID, tenantID, shipment.Status, targetStatus, actualPickup, actualDelivery, shipment.Version, transition)
	}
	if updateErr != nil {
		return DriverOperationalEventResult{}, updateErr
	}
	result.ShipmentStatus = updated.Status

	if mappedType, ok := domain.MapOperationalEventType(strings.TrimSpace(in.Type)); ok {
		eventID := uuid.New()
		sourceEventID := uuid.New()
		outboxID, outboxErr := s.operations.InsertDriverEventOutbox(ctx, domain.BuildDriverEventParams{
			EventID:         eventID,
			EventType:       mappedType,
			TenantID:        tenantID,
			ShipmentID:      shipment.ID,
			ShipmentVersion: updated.Version,
			DriverID:        resolved.Driver.ID,
			VehicleID:       shipment.VehicleID,
			ActorID:         &userID,
			SourceEventID:   sourceEventID,
			OccurredAt:      occurredAt,
			ReasonCode:      strings.TrimSpace(in.Type),
		})
		if outboxErr != nil {
			return DriverOperationalEventResult{}, outboxErr
		}
		result.OutboxEventID = &outboxID
	}

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
	return s.operations.CommitIdempotency(ctx, domain.DriverOperationIdempotencyRecord{
		TenantID:           tenantID,
		DriverID:           driverID,
		OperationType:      domain.DriverOperationTypeStatusEvent,
		IdempotencyKey:     idempotencyKey,
		ResourceType:       "shipment",
		ResourceID:         shipmentID,
		ResponseStatusCode: 200,
		ResponseBody:       body,
	})
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

	stopID, actionID, err := s.verifiedProblemContext(ctx, tenantID, resolved.Driver.ID, shipmentID, in.ExecutionStopID, in.ActionID)
	if err != nil {
		return DriverExceptionResult{}, err
	}
	exc, outboxID, err := s.operations.ReportException(ctx, repository.ReportDriverExceptionParams{
		Exception:       excInput,
		ShipmentVersion: shipment.Version,
		CorrelationID:   correlationID,
		ExecutionStopID: stopID,
		ActionID:        actionID,
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
	if err := s.operations.CommitIdempotency(ctx, domain.DriverOperationIdempotencyRecord{
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
	return result, nil
}

type DriverDelayResult struct {
	Delay         domain.DriverReportedDelay
	OutboxEventID *uuid.UUID
	Replayed      bool
}

func (s *DriverOperationsService) ReportDelay(
	ctx context.Context,
	tenantID, userID, shipmentID uuid.UUID,
	in domain.DriverDelayInput,
	correlationID *string,
) (DriverDelayResult, error) {
	if err := domain.ValidateDriverDelayInput(in); err != nil {
		return DriverDelayResult{}, err
	}
	resolved, err := s.ResolveDriver(ctx, tenantID, userID)
	if err != nil {
		return DriverDelayResult{}, err
	}

	idempotencyKey := strings.TrimSpace(in.IdempotencyKey)
	if existing, err := s.operations.GetIdempotencyRecord(ctx, tenantID, resolved.Driver.ID, domain.DriverOperationTypeDelay, idempotencyKey); err != nil {
		return DriverDelayResult{}, err
	} else if existing != nil {
		var cached DriverDelayResult
		if err := json.Unmarshal(existing.ResponseBody, &cached); err == nil {
			cached.Replayed = true
			return cached, nil
		}
	}

	shipment, err := s.shipments.GetByIDAndDriver(ctx, shipmentID, tenantID, resolved.Driver.ID)
	if err != nil {
		return DriverDelayResult{}, err
	}

	receivedAt := time.Now().UTC()
	occurredAt := receivedAt
	if in.OccurredAt != nil {
		occurredAt = in.OccurredAt.UTC()
	}
	reasonCode := strings.TrimSpace(strings.ToUpper(in.ReasonCode))
	delayInput := domain.DriverReportedDelay{
		TenantID:       tenantID,
		ShipmentID:     shipmentID,
		DriverID:       resolved.Driver.ID,
		ReasonCode:     reasonCode,
		ReasonText:     domain.SanitizeDriverExceptionComment(in.ReasonText),
		NewETA:         in.NewETA,
		OccurredAt:     occurredAt,
		ReceivedAt:     receivedAt,
		IdempotencyKey: idempotencyKey,
	}

	stopID, err := s.verifiedDelayStop(ctx, tenantID, resolved.Driver.ID, shipmentID, in.ExecutionStopID)
	if err != nil {
		return DriverDelayResult{}, err
	}
	delay, outboxID, err := s.operations.ReportDelay(ctx, repository.ReportDriverDelayParams{
		Delay:           delayInput,
		ShipmentVersion: shipment.Version,
		CorrelationID:   correlationID,
		ExecutionStopID: stopID,
	})
	if err != nil {
		return DriverDelayResult{}, err
	}

	result := DriverDelayResult{Delay: *delay}
	if outboxID != uuid.Nil {
		result.OutboxEventID = &outboxID
	} else {
		result.Replayed = true
	}

	body, err := json.Marshal(result)
	if err != nil {
		return DriverDelayResult{}, err
	}
	if err := s.operations.CommitIdempotency(ctx, domain.DriverOperationIdempotencyRecord{
		TenantID:           tenantID,
		DriverID:           resolved.Driver.ID,
		OperationType:      domain.DriverOperationTypeDelay,
		IdempotencyKey:     idempotencyKey,
		ResourceType:       "shipment",
		ResourceID:         shipmentID,
		ResponseStatusCode: 201,
		ResponseBody:       body,
	}); err != nil {
		return DriverDelayResult{}, err
	}
	return result, nil
}

func (s *DriverOperationsService) BindMultistopDeparture(commands *repository.TransportExecutionCommandRepository) {
	s.departure = commands
}

func (s *DriverOperationsService) BindDriverExecutionContext(resolver driverExecutionContextStore) {
	s.executionContext = resolver
}

func (s *DriverOperationsService) verifiedDelayStop(ctx context.Context, tenantID, driverID, shipmentID uuid.UUID, executionStopID *uuid.UUID) (*uuid.UUID, error) {
	if executionStopID == nil {
		return nil, nil
	}
	resolved, err := s.lookupDriverExecutionContext(ctx, tenantID, driverID, shipmentID, executionStopID, nil)
	if err != nil {
		return nil, err
	}
	stopID := resolved.ExecutionStopID
	return &stopID, nil
}

func (s *DriverOperationsService) verifiedProblemContext(ctx context.Context, tenantID, driverID, shipmentID uuid.UUID, executionStopID, actionID *uuid.UUID) (*uuid.UUID, *uuid.UUID, error) {
	if executionStopID == nil && actionID == nil {
		return nil, nil, nil
	}
	resolved, err := s.lookupDriverExecutionContext(ctx, tenantID, driverID, shipmentID, executionStopID, actionID)
	if err != nil {
		return nil, nil, err
	}
	stopID := resolved.ExecutionStopID
	return &stopID, resolved.ActionID, nil
}

func (s *DriverOperationsService) lookupDriverExecutionContext(ctx context.Context, tenantID, driverID, shipmentID uuid.UUID, executionStopID, actionID *uuid.UUID) (repository.ResolvedDriverExecutionContext, error) {
	if s.executionContext == nil {
		return repository.ResolvedDriverExecutionContext{}, apperrors.NotFound("execution context not found")
	}
	return s.executionContext.ResolveDriverExecutionContext(ctx, tenantID, driverID, shipmentID, executionStopID, actionID)
}

func (s *DriverOperationsService) dispatchMultistopDeparture(
	ctx context.Context,
	tenantID, driverID, shipmentID uuid.UUID,
	idempotencyKey string,
	in domain.DriverOperationalEventInput,
) (DriverOperationalEventResult, bool, error) {
	executionID, revisionID, shipmentTenant, found, err := s.departure.FindAssignedParticipant(ctx, tenantID, driverID, shipmentID)
	if err != nil || !found {
		return DriverOperationalEventResult{}, false, err
	}
	receivedAt := time.Now().UTC()
	occurredAt := receivedAt
	if in.OccurredAt != nil {
		occurredAt = in.OccurredAt.UTC()
	}
	updated, err := s.departure.Execute(ctx, domain.ExecutionCommand{
		Name:              domain.CommandDepartedPickup,
		ExecutionID:       executionID,
		RevisionID:        revisionID,
		ShipmentID:        shipmentID,
		ShipmentTenantID:  shipmentTenant,
		IdempotencyKey:    idempotencyKey,
		OccurredAt:        occurredAt,
		ActorKind:         domain.ActorKindDriver,
		ActorID:           driverID,
		OperatingTenantID: tenantID,
	})
	if err != nil {
		return DriverOperationalEventResult{}, false, err
	}
	target := domain.ShipmentStatusInTransit
	return DriverOperationalEventResult{
		ShipmentID:     shipmentID,
		EventType:      strings.TrimSpace(in.Type),
		TargetStatus:   &target,
		ShipmentStatus: updated.ShipmentStatus,
		OccurredAt:     occurredAt,
		ReceivedAt:     receivedAt,
		Replayed:       updated.Replayed,
	}, true, nil
}
