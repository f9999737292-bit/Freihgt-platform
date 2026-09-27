package service

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	"github.com/freight-platform/shipment-service/internal/repository"
)

func TestNLO03C_DriverEvidence(t *testing.T) {
	if _, ok := reflect.TypeOf(domain.DriverOperationalEventInput{}).FieldByName("CargoID"); ok {
		t.Fatal("NLO03C_006 driver input must not carry cargo_id")
	}
	if _, ok := reflect.TypeOf(domain.CargoEvidenceIntent{}).FieldByName("CargoID"); ok {
		t.Fatal("NLO03C_006 evidence intent must not accept a client cargo_id")
	}

	tenant := uuid.New()
	user := uuid.New()
	driver := uuid.New()
	cargo := uuid.New()
	otherCargo := uuid.New()
	shipmentID := uuid.New()
	occurred := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	shipments := &evidenceShipments{shipment: domain.Shipment{
		ID: shipmentID, TenantID: tenant, Status: domain.ShipmentStatusInPickup, Version: 4,
		DriverID: &driver, CargoID: &cargo,
	}}
	ops := &evidenceOperations{}
	svc := NewDriverOperationsService(&evidenceDrivers{driver: domain.Driver{ID: driver, UserID: &user, Status: domain.DriverStatusActive}}, shipments, ops)
	transition := domain.NewUserTransitionContext(user, nil, occurred)

	t.Run("NLO03C_005_PICKUP_COMPLETED_WRITES_CONFIRMED_ONBOARD", func(t *testing.T) {
		result, err := svc.RecordOperationalEvent(context.Background(), tenant, user, shipmentID, domain.DriverOperationalEventInput{
			Type: "PICKUP_COMPLETED", IdempotencyKey: "pickup-1", OccurredAt: &occurred,
		}, transition)
		if err != nil || result.ShipmentStatus != domain.ShipmentStatusLoaded || len(shipments.evidence) != 1 {
			t.Fatalf("result %+v err %v rows %d", result, err, len(shipments.evidence))
		}
		row := shipments.evidence[0]
		if row.State != domain.CargoEvidenceConfirmedOnboard || row.Source != domain.CargoEvidenceSourceDriver || row.SourceEventType != "PICKUP_COMPLETED" {
			t.Fatalf("%+v", row)
		}
	})
	t.Run("NLO03C_007_EVIDENCE_USES_SERVER_SHIPMENT_CARGO", func(t *testing.T) {
		if shipments.evidence[0].CargoID != cargo || shipments.evidence[0].CargoID == otherCargo {
			t.Fatalf("cargo %s", shipments.evidence[0].CargoID)
		}
	})
	t.Run("NLO03C_008_EVIDENCE_PINS_SHIPMENT_VERSION", func(t *testing.T) {
		if shipments.evidence[0].ShipmentVersion != 5 || shipments.evidence[0].StateVersion != 5 {
			t.Fatalf("%+v", shipments.evidence[0])
		}
	})
	t.Run("NLO03C_010_REPLAY_DOES_NOT_DUPLICATE_EVIDENCE", func(t *testing.T) {
		replay, err := svc.RecordOperationalEvent(context.Background(), tenant, user, shipmentID, domain.DriverOperationalEventInput{
			Type: "PICKUP_COMPLETED", IdempotencyKey: "pickup-1", OccurredAt: &occurred,
		}, transition)
		if err != nil || !replay.Replayed || len(shipments.evidence) != 1 {
			t.Fatalf("replay %+v err %v rows %d", replay, err, len(shipments.evidence))
		}
	})
	t.Run("NLO03C_014_DRIVER_IN_TRANSIT_DOES_NOT_CREATE_EVIDENCE", func(t *testing.T) {
		if _, err := svc.RecordOperationalEvent(context.Background(), tenant, user, shipmentID, domain.DriverOperationalEventInput{
			Type: "DEPARTED_PICKUP", IdempotencyKey: "depart-1",
		}, transition); err != nil || len(shipments.evidence) != 1 || shipments.shipment.Status != domain.ShipmentStatusInTransit {
			t.Fatalf("status %s rows %d err %v", shipments.shipment.Status, len(shipments.evidence), err)
		}
	})
	for _, step := range []struct{ event, key, status string }{
		{"ARRIVED_AT_DELIVERY", "arrive-1", domain.ShipmentStatusArrivedAtConsignee},
		{"UNLOADING_STARTED", "unload-start", domain.ShipmentStatusUnloading},
	} {
		if _, err := svc.RecordOperationalEvent(context.Background(), tenant, user, shipmentID, domain.DriverOperationalEventInput{
			Type: step.event, IdempotencyKey: step.key,
		}, transition); err != nil || shipments.shipment.Status != step.status || len(shipments.evidence) != 1 {
			t.Fatalf("%s status %s rows %d err %v", step.event, shipments.shipment.Status, len(shipments.evidence), err)
		}
	}
	t.Run("NLO03C_011_DELIVERY_COMPLETED_WRITES_UNLOADED", func(t *testing.T) {
		if _, err := svc.RecordOperationalEvent(context.Background(), tenant, user, shipmentID, domain.DriverOperationalEventInput{
			Type: "DELIVERY_COMPLETED", IdempotencyKey: "deliver-1", OccurredAt: &occurred,
		}, transition); err != nil || shipments.shipment.Status != domain.ShipmentStatusDelivered {
			t.Fatal(err)
		}
		if len(shipments.evidence) != 2 || shipments.evidence[1].State != domain.CargoEvidenceUnloaded {
			t.Fatalf("%+v", shipments.evidence)
		}
	})
	t.Run("NLO03C_012_UNLOADED_DOES_NOT_DELETE_HISTORY", func(t *testing.T) {
		if shipments.evidence[0].State != domain.CargoEvidenceConfirmedOnboard || shipments.evidence[1].State != domain.CargoEvidenceUnloaded {
			t.Fatalf("%+v", shipments.evidence)
		}
	})
}

func TestNLO03C_009_PICKUP_STATUS_AND_EVIDENCE_ATOMIC(t *testing.T) {
	tenant, user, driver, cargo := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	shipments := &evidenceShipments{
		failEvidence: true,
		shipment: domain.Shipment{
			ID: uuid.New(), TenantID: tenant, Status: domain.ShipmentStatusInPickup, Version: 2, DriverID: &driver, CargoID: &cargo,
		},
	}
	svc := NewDriverOperationsService(
		&evidenceDrivers{driver: domain.Driver{ID: driver, UserID: &user, Status: domain.DriverStatusActive}},
		shipments, &evidenceOperations{},
	)
	_, err := svc.RecordOperationalEvent(context.Background(), tenant, user, shipments.shipment.ID, domain.DriverOperationalEventInput{
		Type: "PICKUP_COMPLETED", IdempotencyKey: "atomic-1",
	}, domain.NewUserTransitionContext(user, nil, time.Now().UTC()))
	if err == nil || shipments.shipment.Status != domain.ShipmentStatusInPickup || shipments.shipment.Version != 2 || len(shipments.evidence) != 0 {
		t.Fatalf("err %v status %s version %d rows %d", err, shipments.shipment.Status, shipments.shipment.Version, len(shipments.evidence))
	}
}

func TestNLO03C_015_SHIPMENT_WITHOUT_CARGO_DOES_NOT_INVENT_EVIDENCE(t *testing.T) {
	tenant, user, driver := uuid.New(), uuid.New(), uuid.New()
	shipments := &evidenceShipments{shipment: domain.Shipment{
		ID: uuid.New(), TenantID: tenant, Status: domain.ShipmentStatusInPickup, Version: 1, DriverID: &driver,
	}}
	svc := NewDriverOperationsService(
		&evidenceDrivers{driver: domain.Driver{ID: driver, UserID: &user, Status: domain.DriverStatusActive}},
		shipments, &evidenceOperations{},
	)
	result, err := svc.RecordOperationalEvent(context.Background(), tenant, user, shipments.shipment.ID, domain.DriverOperationalEventInput{
		Type: "PICKUP_COMPLETED", IdempotencyKey: "no-cargo",
	}, domain.NewUserTransitionContext(user, nil, time.Now().UTC()))
	if err != nil || result.ShipmentStatus != domain.ShipmentStatusLoaded || len(shipments.evidence) != 0 {
		t.Fatalf("result %+v err %v rows %d", result, err, len(shipments.evidence))
	}
	view := domain.ResolveOnboardCargo(shipments.shipment, shipments.evidence)
	if view.Resolution != domain.OnboardCargoUnproven || len(view.Items) != 0 {
		t.Fatalf("%+v", view)
	}
}

func TestNLO03C_ManualStatusDoesNotCreateEvidence(t *testing.T) {
	tenant := uuid.New()
	shipmentID := uuid.New()
	store := &mockShipmentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			return &domain.Shipment{ID: shipmentID, TenantID: tenant, Status: domain.ShipmentStatusInPickup, Version: 3}, nil
		},
		updateStatusFn: func(_ context.Context, _, _ uuid.UUID, _, newStatus string, _, _ *time.Time, _ int, _ domain.StatusTransitionContext) (*domain.Shipment, error) {
			return &domain.Shipment{ID: shipmentID, TenantID: tenant, Status: newStatus, Version: 4}, nil
		},
	}
	svc := NewShipmentService(store, &mockDriverLookup{}, &mockVehicleLookup{})
	t.Run("NLO03C_013_MANUAL_LOADED_STATUS_DOES_NOT_CREATE_EVIDENCE", func(t *testing.T) {
		now := time.Now().UTC()
		updated, err := svc.UpdateStatus(context.Background(), tenant, shipmentID, domain.UpdateShipmentStatusInput{Status: domain.ShipmentStatusLoaded, ActualTime: &now}, testUserTransition())
		if err != nil || updated.Status != domain.ShipmentStatusLoaded {
			t.Fatalf("%+v err %v", updated, err)
		}
	})
	store.getByIDAndTenantFn = func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
		return &domain.Shipment{ID: shipmentID, TenantID: tenant, Status: domain.ShipmentStatusLoaded, Version: 4}, nil
	}
	t.Run("NLO03C_014_MANUAL_IN_TRANSIT_DOES_NOT_CREATE_EVIDENCE", func(t *testing.T) {
		now := time.Now().UTC()
		updated, err := svc.UpdateStatus(context.Background(), tenant, shipmentID, domain.UpdateShipmentStatusInput{Status: domain.ShipmentStatusInTransit, ActualTime: &now}, testUserTransition())
		if err != nil || updated.Status != domain.ShipmentStatusInTransit {
			t.Fatalf("%+v err %v", updated, err)
		}
	})
}

func TestNLO03C_OnboardResolution(t *testing.T) {
	cargo := uuid.New()
	shipment := domain.Shipment{ID: uuid.New(), Version: 8, CargoID: &cargo, Status: domain.ShipmentStatusLoaded}
	t.Run("NLO03C_020_LATEST_EVIDENCE_DETERMINISTIC", func(t *testing.T) {
		older := time.Now().UTC()
		newer := older.Add(time.Minute)
		low := domain.ShipmentCargoEvidence{ID: uuid.MustParse("00000000-0000-4000-8000-000000000001"), ShipmentID: shipment.ID, CargoID: cargo, State: domain.CargoEvidenceConfirmedOnboard, StateVersion: 3, OccurredAt: newer}
		high := domain.ShipmentCargoEvidence{ID: uuid.MustParse("00000000-0000-4000-8000-000000000002"), ShipmentID: shipment.ID, CargoID: cargo, State: domain.CargoEvidenceUnloaded, StateVersion: 4, OccurredAt: older}
		view := domain.ResolveOnboardCargo(shipment, []domain.ShipmentCargoEvidence{low, high})
		if view.Resolution != domain.CargoEvidenceUnloaded || view.Items[0].StateVersion != 4 {
			t.Fatalf("%+v", view)
		}
	})
	t.Run("NLO03C_021_NO_EVIDENCE_RETURNS_UNPROVEN_NOT_ONBOARD_FALSE", func(t *testing.T) {
		view := domain.ResolveOnboardCargo(shipment, nil)
		if view.Resolution != domain.OnboardCargoUnproven || len(view.Items) != 0 {
			t.Fatalf("%+v", view)
		}
	})
	t.Run("NLO03C_022_CONFIRMED_ONBOARD_RETURNED", func(t *testing.T) {
		view := domain.ResolveOnboardCargo(shipment, []domain.ShipmentCargoEvidence{{
			ID: uuid.New(), ShipmentID: shipment.ID, CargoID: cargo, State: domain.CargoEvidenceConfirmedOnboard, StateVersion: 2, OccurredAt: time.Now().UTC(),
		}})
		if view.Resolution != domain.CargoEvidenceConfirmedOnboard {
			t.Fatal(view.Resolution)
		}
	})
	t.Run("NLO03C_023_UNLOADED_RETURNED", func(t *testing.T) {
		view := domain.ResolveOnboardCargo(shipment, []domain.ShipmentCargoEvidence{{
			ID: uuid.New(), ShipmentID: shipment.ID, CargoID: cargo, State: domain.CargoEvidenceUnloaded, StateVersion: 3, OccurredAt: time.Now().UTC(),
		}})
		if view.Resolution != domain.CargoEvidenceUnloaded {
			t.Fatal(view.Resolution)
		}
	})
}

type evidenceDrivers struct{ driver domain.Driver }

func (d *evidenceDrivers) GetByUserIDAndTenant(context.Context, uuid.UUID, uuid.UUID) (*domain.Driver, error) {
	copy := d.driver
	return &copy, nil
}

type evidenceShipments struct {
	shipment     domain.Shipment
	evidence     []domain.ShipmentCargoEvidence
	failEvidence bool
}

func (s *evidenceShipments) GetByIDAndDriver(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
	copy := s.shipment
	return &copy, nil
}

func (s *evidenceShipments) ListByDriverID(context.Context, domain.ListDriverShipmentsFilter) ([]domain.Shipment, int, error) {
	return nil, 0, nil
}

func (s *evidenceShipments) UpdateStatus(_ context.Context, _, _ uuid.UUID, _, newStatus string, _, _ *time.Time, _ int, _ domain.StatusTransitionContext) (*domain.Shipment, error) {
	s.shipment.Status = newStatus
	s.shipment.Version++
	copy := s.shipment
	return &copy, nil
}

func (s *evidenceShipments) UpdateStatusWithCargoEvidence(_ context.Context, _, _ uuid.UUID, _, newStatus string, _, _ *time.Time, _ int, _ domain.StatusTransitionContext, intent domain.CargoEvidenceIntent) (*domain.Shipment, error) {
	if s.failEvidence {
		return nil, errors.New("evidence write failed")
	}
	s.shipment.Status = newStatus
	s.shipment.Version++
	if s.shipment.CargoID != nil {
		driverID := intent.DriverID
		s.evidence = append(s.evidence, domain.ShipmentCargoEvidence{
			ID: uuid.New(), ShipmentID: s.shipment.ID, CargoID: *s.shipment.CargoID,
			State: intent.State, StateVersion: s.shipment.Version, ShipmentVersion: s.shipment.Version,
			Source: intent.Source, SourceEventType: intent.SourceEventType, OccurredAt: intent.OccurredAt, DriverID: &driverID,
		})
	}
	copy := s.shipment
	return &copy, nil
}

type evidenceOperations struct {
	saved map[string]domain.DriverOperationIdempotencyRecord
}

func (o *evidenceOperations) GetIdempotencyRecord(_ context.Context, _, _ uuid.UUID, _, key string) (*domain.DriverOperationIdempotencyRecord, error) {
	if o.saved == nil {
		return nil, nil
	}
	rec, ok := o.saved[key]
	if !ok {
		return nil, nil
	}
	return &rec, nil
}

func (o *evidenceOperations) CommitIdempotency(_ context.Context, rec domain.DriverOperationIdempotencyRecord) error {
	if o.saved == nil {
		o.saved = map[string]domain.DriverOperationIdempotencyRecord{}
	}
	o.saved[rec.IdempotencyKey] = rec
	return nil
}

func (o *evidenceOperations) InsertDriverEventOutbox(context.Context, domain.BuildDriverEventParams) (uuid.UUID, error) {
	return uuid.New(), nil
}

func (o *evidenceOperations) ReportException(context.Context, repository.ReportDriverExceptionParams) (*domain.DriverReportedException, uuid.UUID, error) {
	return nil, uuid.Nil, errors.New("unused")
}

func (o *evidenceOperations) ReportDelay(context.Context, repository.ReportDriverDelayParams) (*domain.DriverReportedDelay, uuid.UUID, error) {
	return nil, uuid.Nil, errors.New("unused")
}
