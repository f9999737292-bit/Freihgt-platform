package service

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
	"github.com/freight-platform/shipment-service/internal/repository"
)

type deliveryDispositionRecorder interface {
	RecordDeliveryDisposition(ctx context.Context, cmd domain.RecordDeliveryDispositionCommand) (domain.DispositionResult, error)
}

type DriverStopService struct {
	drivers      driverIdentityStore
	commands     *repository.TransportExecutionCommandRepository
	dispositions deliveryDispositionRecorder
}

func NewDriverStopService(drivers driverIdentityStore, commands *repository.TransportExecutionCommandRepository) *DriverStopService {
	return &DriverStopService{drivers: drivers, commands: commands}
}

// BindDeliveryDisposition attaches the canonical disposition writer. Driver reports do not authorize return or redirect.
func (s *DriverStopService) BindDeliveryDisposition(recorder deliveryDispositionRecorder) {
	s.dispositions = recorder
}

type DriverStopCommandInput struct {
	StopID          uuid.UUID
	ActionID        uuid.UUID
	IdempotencyKey  string
	OccurredAt      time.Time
	ExpectedVersion int
	ReasonCode      string
}

type DriverStopCommandResult struct {
	TaskID          uuid.UUID
	ExecutionStopID uuid.UUID
	Status          string
	Version         int
	ActionStatus    string
	EvidenceID      *uuid.UUID
	ShipmentStatus  string
	Replayed        bool
}

func (s *DriverStopService) List(ctx context.Context, tenantID, userID uuid.UUID) (domain.DriverCurrentNextStops, error) {
	resolved, err := s.resolve(ctx, tenantID, userID)
	if err != nil {
		return domain.DriverCurrentNextStops{}, err
	}
	return s.commands.ListCurrentNext(ctx, tenantID, resolved.Driver.ID)
}

func (s *DriverStopService) Arrive(ctx context.Context, tenantID, userID uuid.UUID, in DriverStopCommandInput) (DriverStopCommandResult, error) {
	return s.dispatch(ctx, tenantID, userID, domain.CommandArriveStop, in)
}

func (s *DriverStopService) StartService(ctx context.Context, tenantID, userID uuid.UUID, in DriverStopCommandInput) (DriverStopCommandResult, error) {
	return s.dispatch(ctx, tenantID, userID, domain.CommandStartStopService, in)
}

func (s *DriverStopService) Complete(ctx context.Context, tenantID, userID uuid.UUID, in DriverStopCommandInput) (DriverStopCommandResult, error) {
	return s.dispatch(ctx, tenantID, userID, domain.CommandCompleteStop, in)
}

func (s *DriverStopService) Confirm(ctx context.Context, tenantID, userID uuid.UUID, in DriverStopCommandInput) (DriverStopCommandResult, error) {
	actionType, err := s.commands.LoadStopAction(ctx, in.StopID, in.ActionID)
	if err != nil {
		return DriverStopCommandResult{}, err
	}
	switch actionType {
	case domain.ActionTypePickup:
		return s.dispatch(ctx, tenantID, userID, domain.CommandConfirmPickup, in)
	case domain.ActionTypeDelivery:
		return s.dispatch(ctx, tenantID, userID, domain.CommandConfirmDelivery, in)
	default:
		return DriverStopCommandResult{}, domain.ExecutionCommandError(domain.ReasonActionTransitionDenied, false)
	}
}

// DriverDispositionInput carries only the facts a driver may report. Identity and revision stay server-owned.
type DriverDispositionInput struct {
	StopID         uuid.UUID
	ActionID       uuid.UUID
	ShipmentID     uuid.UUID
	CargoID        uuid.UUID
	Accepted       int
	Rejected       int
	UOM            string
	ReasonCode     string
	ReasonComment  string
	IdempotencyKey string
	OccurredAt     time.Time
	Evidence       []domain.DeliveryEvidenceRef
}

// ReportDeliveryDisposition records a full or partial rejection through the canonical disposition engine.
// The authenticated driver is the actor. Return and redirect authorization are not available here.
func (s *DriverStopService) ReportDeliveryDisposition(ctx context.Context, tenantID, userID uuid.UUID, in DriverDispositionInput) (domain.DispositionResult, error) {
	if s.dispositions == nil {
		return domain.DispositionResult{}, apperrors.Internal("delivery disposition is not configured", nil)
	}
	resolved, err := s.resolve(ctx, tenantID, userID)
	if err != nil {
		return domain.DispositionResult{}, err
	}
	task, operating, assigned, revisionID, err := s.commands.LoadDriverStop(ctx, in.StopID)
	if err != nil {
		return domain.DispositionResult{}, err
	}
	if operating != tenantID {
		return domain.DispositionResult{}, apperrors.NotFound("driver stop task not found")
	}
	if assigned == nil {
		return domain.DispositionResult{}, domain.ExecutionCommandError(domain.ReasonUnassignedDriver, false)
	}
	if *assigned != resolved.Driver.ID {
		return domain.DispositionResult{}, domain.ExecutionCommandError(domain.ReasonWrongDriver, true)
	}
	actionShipment, actionCargo, err := s.commands.LoadDeliveryAction(ctx, in.StopID, in.ActionID)
	if err != nil {
		return domain.DispositionResult{}, err
	}
	if actionShipment != in.ShipmentID || actionCargo != in.CargoID {
		return domain.DispositionResult{}, domain.ExecutionCommandError(domain.ReasonNotParticipant, false)
	}
	if in.OccurredAt.IsZero() {
		in.OccurredAt = time.Now().UTC()
	}
	return s.dispositions.RecordDeliveryDisposition(ctx, domain.RecordDeliveryDispositionCommand{
		ExecutionID:        task.ExecutionID,
		ExpectedRevisionID: revisionID,
		OperatingTenantID:  tenantID,
		SourceStopID:       in.StopID,
		ShipmentID:         in.ShipmentID,
		CargoID:            in.CargoID,
		AcceptedQuantity:   in.Accepted,
		RejectedQuantity:   in.Rejected,
		UOM:                in.UOM,
		ReasonCode:         in.ReasonCode,
		ReasonComment:      in.ReasonComment,
		IdempotencyKey:     strings.TrimSpace(in.IdempotencyKey),
		OccurredAt:         in.OccurredAt,
		ActorKind:          domain.ActorKindDriver,
		ActorID:            resolved.Driver.ID,
		Evidence:           in.Evidence,
	})
}

func (s *DriverStopService) Fail(ctx context.Context, tenantID, userID uuid.UUID, in DriverStopCommandInput) (DriverStopCommandResult, error) {
	if _, err := s.commands.LoadStopAction(ctx, in.StopID, in.ActionID); err != nil {
		return DriverStopCommandResult{}, err
	}
	return s.dispatch(ctx, tenantID, userID, domain.CommandFailAction, in)
}

func (s *DriverStopService) dispatch(ctx context.Context, tenantID, userID uuid.UUID, name string, in DriverStopCommandInput) (DriverStopCommandResult, error) {
	resolved, err := s.resolve(ctx, tenantID, userID)
	if err != nil {
		return DriverStopCommandResult{}, err
	}
	task, operating, assigned, revisionID, err := s.commands.LoadDriverStop(ctx, in.StopID)
	if err != nil {
		return DriverStopCommandResult{}, err
	}
	if operating != tenantID {
		return DriverStopCommandResult{}, apperrors.NotFound("driver stop task not found")
	}
	if assigned == nil {
		return DriverStopCommandResult{}, domain.ExecutionCommandError(domain.ReasonUnassignedDriver, false)
	}
	if *assigned != resolved.Driver.ID {
		return DriverStopCommandResult{}, domain.ExecutionCommandError(domain.ReasonWrongDriver, true)
	}
	if in.OccurredAt.IsZero() {
		in.OccurredAt = time.Now().UTC()
	}
	result, err := s.commands.Execute(ctx, domain.ExecutionCommand{
		Name:                name,
		ExecutionID:         task.ExecutionID,
		RevisionID:          revisionID,
		StopID:              in.StopID,
		ActionID:            in.ActionID,
		IdempotencyKey:      strings.TrimSpace(in.IdempotencyKey),
		OccurredAt:          in.OccurredAt,
		ExpectedStopVersion: in.ExpectedVersion,
		ReasonCode:          in.ReasonCode,
		ActorKind:           domain.ActorKindDriver,
		ActorID:             resolved.Driver.ID,
		OperatingTenantID:   tenantID,
	})
	if err != nil {
		return DriverStopCommandResult{}, err
	}
	updated, _, _, _, err := s.commands.LoadDriverStop(ctx, in.StopID)
	if err != nil {
		return DriverStopCommandResult{}, err
	}
	return DriverStopCommandResult{
		TaskID:          updated.TaskID,
		ExecutionStopID: updated.ExecutionStopID,
		Status:          updated.Status,
		Version:         updated.Version,
		ActionStatus:    result.ActionStatus,
		EvidenceID:      result.EvidenceID,
		ShipmentStatus:  result.ShipmentStatus,
		Replayed:        result.Replayed,
	}, nil
}

func (s *DriverStopService) resolve(ctx context.Context, tenantID, userID uuid.UUID) (*ResolvedDriver, error) {
	if tenantID == uuid.Nil || userID == uuid.Nil {
		return nil, apperrors.Unauthorized("authentication required")
	}
	driver, err := s.drivers.GetByUserIDAndTenant(ctx, userID, tenantID)
	if err != nil {
		return nil, err
	}
	if driver.Status != domain.DriverStatusActive || driver.UserID == nil || *driver.UserID != userID {
		return nil, apperrors.Unauthorized("driver is not active")
	}
	return &ResolvedDriver{Driver: *driver, UserID: userID}, nil
}
