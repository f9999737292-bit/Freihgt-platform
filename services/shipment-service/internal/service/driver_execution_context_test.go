package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
	"github.com/freight-platform/shipment-service/internal/repository"
)

func TestReportDelayPublishesServerStopOnly(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	driverID := uuid.New()
	shipmentID := uuid.New()
	clientStop := uuid.New()
	serverStop := uuid.New()
	ops := &recordingDriverOps{}
	svc := NewDriverOperationsService(
		&recordingDriverIdentity{driverID: driverID, userID: userID, tenantID: tenantID},
		&recordingDriverShipments{driverID: driverID, tenantID: tenantID},
		ops,
	)
	svc.BindDriverExecutionContext(&recordingExecutionContext{stopID: serverStop})

	_, err := svc.ReportDelay(context.Background(), tenantID, userID, shipmentID, domain.DriverDelayInput{
		ReasonCode:      "TRAFFIC",
		IdempotencyKey:  "delay-server-stop",
		ExecutionStopID: &clientStop,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ops.delayStop == nil || *ops.delayStop != serverStop {
		t.Fatalf("published stop = %v, want server stop %s", ops.delayStop, serverStop)
	}
	if *ops.delayStop == clientStop {
		t.Fatal("published the client stop id")
	}
}

func TestReportDelayRejectsUnverifiedStopWithoutWrite(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	driverID := uuid.New()
	shipmentID := uuid.New()
	stopID := uuid.New()
	ops := &recordingDriverOps{}
	svc := NewDriverOperationsService(
		&recordingDriverIdentity{driverID: driverID, userID: userID, tenantID: tenantID},
		&recordingDriverShipments{driverID: driverID, tenantID: tenantID},
		ops,
	)
	svc.BindDriverExecutionContext(&recordingExecutionContext{err: apperrors.NotFound("execution context not found")})

	_, err := svc.ReportDelay(context.Background(), tenantID, userID, shipmentID, domain.DriverDelayInput{
		ReasonCode:      "TRAFFIC",
		IdempotencyKey:  "delay-foreign-stop",
		ExecutionStopID: &stopID,
	}, nil)
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeNotFound {
		t.Fatalf("err = %v", err)
	}
	if ops.delayCalls != 0 || ops.exceptionCalls != 0 {
		t.Fatal("rejected context wrote a driver event")
	}
}

func TestReportExceptionRejectsWhenContextResolverMissing(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	driverID := uuid.New()
	shipmentID := uuid.New()
	actionID := uuid.New()
	ops := &recordingDriverOps{}
	svc := NewDriverOperationsService(
		&recordingDriverIdentity{driverID: driverID, userID: userID, tenantID: tenantID},
		&recordingDriverShipments{driverID: driverID, tenantID: tenantID},
		ops,
	)

	_, err := svc.ReportException(context.Background(), tenantID, userID, shipmentID, domain.DriverExceptionInput{
		Category:       "CARGO_ISSUE",
		IdempotencyKey: "problem-unwired",
		ActionID:       &actionID,
	}, nil)
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeNotFound {
		t.Fatalf("err = %v", err)
	}
	if ops.exceptionCalls != 0 {
		t.Fatal("missing resolver published a problem event")
	}
}

func TestLegacyDelaySkipsExecutionContextLookup(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	driverID := uuid.New()
	shipmentID := uuid.New()
	ops := &recordingDriverOps{}
	lookup := &recordingExecutionContext{}
	svc := NewDriverOperationsService(
		&recordingDriverIdentity{driverID: driverID, userID: userID, tenantID: tenantID},
		&recordingDriverShipments{driverID: driverID, tenantID: tenantID},
		ops,
	)
	svc.BindDriverExecutionContext(lookup)

	_, err := svc.ReportDelay(context.Background(), tenantID, userID, shipmentID, domain.DriverDelayInput{
		ReasonCode:     "TRAFFIC",
		IdempotencyKey: "legacy-delay",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if lookup.calls != 0 {
		t.Fatal("legacy delay queried execution context")
	}
	if ops.delayStop != nil {
		t.Fatal("legacy delay published a stop id")
	}
}

type recordingDriverIdentity struct {
	driverID uuid.UUID
	userID   uuid.UUID
	tenantID uuid.UUID
}

func (r *recordingDriverIdentity) GetByUserIDAndTenant(context.Context, uuid.UUID, uuid.UUID) (*domain.Driver, error) {
	return &domain.Driver{
		ID:       r.driverID,
		TenantID: r.tenantID,
		UserID:   &r.userID,
		Status:   domain.DriverStatusActive,
	}, nil
}

type recordingDriverShipments struct {
	driverID uuid.UUID
	tenantID uuid.UUID
}

func (r *recordingDriverShipments) GetByIDAndDriver(_ context.Context, id, tenantID, driverID uuid.UUID) (*domain.Shipment, error) {
	return &domain.Shipment{ID: id, TenantID: tenantID, DriverID: &r.driverID, Version: 1}, nil
}

func (r *recordingDriverShipments) ListByDriverID(context.Context, domain.ListDriverShipmentsFilter) ([]domain.Shipment, int, error) {
	return nil, 0, nil
}

func (r *recordingDriverShipments) UpdateStatus(context.Context, uuid.UUID, uuid.UUID, string, string, *time.Time, *time.Time, int, domain.StatusTransitionContext) (*domain.Shipment, error) {
	return nil, errors.New("unused")
}

func (r *recordingDriverShipments) UpdateStatusWithCargoEvidence(context.Context, uuid.UUID, uuid.UUID, string, string, *time.Time, *time.Time, int, domain.StatusTransitionContext, domain.CargoEvidenceIntent) (*domain.Shipment, error) {
	return nil, errors.New("unused")
}

type recordingDriverOps struct {
	delayCalls     int
	exceptionCalls int
	delayStop      *uuid.UUID
	problemStop    *uuid.UUID
	problemAction  *uuid.UUID
}

func (r *recordingDriverOps) GetIdempotencyRecord(context.Context, uuid.UUID, uuid.UUID, string, string) (*domain.DriverOperationIdempotencyRecord, error) {
	return nil, nil
}

func (r *recordingDriverOps) CommitIdempotency(context.Context, domain.DriverOperationIdempotencyRecord) error {
	return nil
}

func (r *recordingDriverOps) InsertDriverEventOutbox(context.Context, domain.BuildDriverEventParams) (uuid.UUID, error) {
	return uuid.Nil, errors.New("unused")
}

func (r *recordingDriverOps) ReportException(_ context.Context, params repository.ReportDriverExceptionParams) (*domain.DriverReportedException, uuid.UUID, error) {
	r.exceptionCalls++
	r.problemStop = params.ExecutionStopID
	r.problemAction = params.ActionID
	return &domain.DriverReportedException{ID: uuid.New(), TenantID: params.Exception.TenantID, ShipmentID: params.Exception.ShipmentID}, uuid.New(), nil
}

func (r *recordingDriverOps) ReportDelay(_ context.Context, params repository.ReportDriverDelayParams) (*domain.DriverReportedDelay, uuid.UUID, error) {
	r.delayCalls++
	r.delayStop = params.ExecutionStopID
	return &domain.DriverReportedDelay{ID: uuid.New(), TenantID: params.Delay.TenantID, ShipmentID: params.Delay.ShipmentID}, uuid.New(), nil
}

type recordingExecutionContext struct {
	stopID   uuid.UUID
	actionID *uuid.UUID
	err      error
	calls    int
}

func (r *recordingExecutionContext) ResolveDriverExecutionContext(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, *uuid.UUID, *uuid.UUID) (repository.ResolvedDriverExecutionContext, error) {
	r.calls++
	if r.err != nil {
		return repository.ResolvedDriverExecutionContext{}, r.err
	}
	return repository.ResolvedDriverExecutionContext{ExecutionStopID: r.stopID, ActionID: r.actionID}, nil
}
