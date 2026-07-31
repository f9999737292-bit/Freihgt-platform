package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

func TestAssignDriverForeignDriverReturns404(t *testing.T) {
	t.Parallel()
	carrierID := uuid.New()
	tenantID := uuid.New()
	shipmentID := uuid.New()
	driverID := uuid.New()

	driverLookupCalled := false
	svc := NewShipmentService(&mockShipmentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			return &domain.Shipment{
				ID: shipmentID, TenantID: tenantID, Status: domain.ShipmentStatusCarrierAssigned,
				CarrierCompanyID: &carrierID, Version: 1,
			}, nil
		},
	}, &mockDriverLookup{
		getByIDAndTenantFn: func(_ context.Context, id, tenant uuid.UUID) (*domain.Driver, error) {
			driverLookupCalled = true
			if id != driverID || tenant != tenantID {
				t.Fatalf("unexpected driver lookup id=%s tenant=%s", id, tenant)
			}
			return nil, apperrors.NotFound("driver not found")
		},
	}, &mockVehicleLookup{})

	_, err := svc.AssignDriver(context.Background(), tenantID, shipmentID, driverID)
	if !driverLookupCalled {
		t.Fatal("driver lookup must be tenant-scoped")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestAssignDriverForeignShipmentReturns404(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	svc := NewShipmentService(&mockShipmentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			return nil, apperrors.NotFound("shipment not found")
		},
	}, &mockDriverLookup{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Driver, error) {
			t.Fatal("driver lookup must not run when shipment is missing")
			return nil, nil
		},
	}, &mockVehicleLookup{})

	_, err := svc.AssignDriver(context.Background(), tenantID, uuid.New(), uuid.New())
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestAssignVehicleForeignVehicleReturns404(t *testing.T) {
	t.Parallel()
	carrierID := uuid.New()
	tenantID := uuid.New()
	shipmentID := uuid.New()
	vehicleID := uuid.New()

	vehicleLookupCalled := false
	svc := NewShipmentService(&mockShipmentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			return &domain.Shipment{
				ID: shipmentID, TenantID: tenantID, Status: domain.ShipmentStatusAcceptedByCarrier,
				CarrierCompanyID: &carrierID, Version: 1,
			}, nil
		},
	}, &mockDriverLookup{}, &mockVehicleLookup{
		getByIDAndTenantFn: func(_ context.Context, id, tenant uuid.UUID) (*domain.Vehicle, error) {
			vehicleLookupCalled = true
			if id != vehicleID || tenant != tenantID {
				t.Fatalf("unexpected vehicle lookup id=%s tenant=%s", id, tenant)
			}
			return nil, apperrors.NotFound("vehicle not found")
		},
	})

	_, err := svc.AssignVehicle(context.Background(), tenantID, shipmentID, vehicleID)
	if !vehicleLookupCalled {
		t.Fatal("vehicle lookup must be tenant-scoped")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestAssignVehicleForeignShipmentReturns404(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	svc := NewShipmentService(&mockShipmentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			return nil, apperrors.NotFound("shipment not found")
		},
	}, &mockDriverLookup{}, &mockVehicleLookup{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Vehicle, error) {
			t.Fatal("vehicle lookup must not run when shipment is missing")
			return nil, nil
		},
	})

	_, err := svc.AssignVehicle(context.Background(), tenantID, uuid.New(), uuid.New())
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestAssignDriverMissingTenantSkipsRepository(t *testing.T) {
	t.Parallel()
	shipmentCalled := false
	driverCalled := false
	svc := NewShipmentService(&mockShipmentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			shipmentCalled = true
			return nil, nil
		},
	}, &mockDriverLookup{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Driver, error) {
			driverCalled = true
			return nil, nil
		},
	}, &mockVehicleLookup{})

	_, err := svc.AssignDriver(context.Background(), uuid.Nil, uuid.New(), uuid.New())
	if shipmentCalled || driverCalled {
		t.Fatal("repository must not be called without tenant")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeUnauthorized {
		t.Fatalf("expected unauthorized, got %v", err)
	}
}

func TestAssignDriverRepositoryFailureReturns500(t *testing.T) {
	t.Parallel()
	carrierID := uuid.New()
	tenantID := uuid.New()
	shipmentID := uuid.New()
	driverID := uuid.New()
	svc := NewShipmentService(&mockShipmentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			return &domain.Shipment{
				TenantID: tenantID, Status: domain.ShipmentStatusCarrierAssigned,
				CarrierCompanyID: &carrierID, Version: 1,
			}, nil
		},
		assignDriverFn: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string, int) (*domain.Shipment, error) {
			return nil, apperrors.Internal("db failure", errors.New("boom"))
		},
	}, &mockDriverLookup{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Driver, error) {
			return &domain.Driver{TenantID: tenantID, CarrierCompanyID: carrierID}, nil
		},
	}, &mockVehicleLookup{})

	_, err := svc.AssignDriver(context.Background(), tenantID, shipmentID, driverID)
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeInternal {
		t.Fatalf("expected internal error, got %v", err)
	}
}

func TestAssignVehicleMissingTenantSkipsRepository(t *testing.T) {
	t.Parallel()
	shipmentCalled := false
	vehicleCalled := false
	svc := NewShipmentService(&mockShipmentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			shipmentCalled = true
			return nil, nil
		},
	}, &mockDriverLookup{}, &mockVehicleLookup{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Vehicle, error) {
			vehicleCalled = true
			return nil, nil
		},
	})

	_, err := svc.AssignVehicle(context.Background(), uuid.Nil, uuid.New(), uuid.New())
	if shipmentCalled || vehicleCalled {
		t.Fatal("repository must not be called without tenant")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeUnauthorized {
		t.Fatalf("expected unauthorized, got %v", err)
	}
}

func TestAssignVehicleRepositoryFailureReturns500(t *testing.T) {
	t.Parallel()
	carrierID := uuid.New()
	tenantID := uuid.New()
	shipmentID := uuid.New()
	vehicleID := uuid.New()
	svc := NewShipmentService(&mockShipmentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			return &domain.Shipment{
				TenantID: tenantID, Status: domain.ShipmentStatusAcceptedByCarrier,
				CarrierCompanyID: &carrierID, Version: 1,
			}, nil
		},
		assignVehicleFn: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string, int) (*domain.Shipment, error) {
			return nil, apperrors.Internal("db failure", errors.New("boom"))
		},
	}, &mockDriverLookup{}, &mockVehicleLookup{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Vehicle, error) {
			return &domain.Vehicle{TenantID: tenantID, CarrierCompanyID: carrierID}, nil
		},
	})

	_, err := svc.AssignVehicle(context.Background(), tenantID, shipmentID, vehicleID)
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeInternal {
		t.Fatalf("expected internal error, got %v", err)
	}
}

func TestAssignDriverVerifiedTenantPassedToRepositories(t *testing.T) {
	t.Parallel()
	carrierID := uuid.New()
	tenantID := uuid.New()
	shipmentID := uuid.New()
	driverID := uuid.New()

	var shipmentLookupTenant uuid.UUID
	var driverLookupTenant uuid.UUID
	var assignTenant uuid.UUID

	svc := NewShipmentService(&mockShipmentStore{
		getByIDAndTenantFn: func(_ context.Context, id, tenant uuid.UUID) (*domain.Shipment, error) {
			shipmentLookupTenant = tenant
			return &domain.Shipment{
				ID: shipmentID, TenantID: tenantID, Status: domain.ShipmentStatusCarrierAssigned,
				CarrierCompanyID: &carrierID, Version: 1,
			}, nil
		},
		assignDriverFn: func(_ context.Context, id, tenant, gotDriverID uuid.UUID, _ string, _ int) (*domain.Shipment, error) {
			assignTenant = tenant
			return &domain.Shipment{Status: domain.ShipmentStatusAcceptedByCarrier, DriverID: &gotDriverID}, nil
		},
	}, &mockDriverLookup{
		getByIDAndTenantFn: func(_ context.Context, id, tenant uuid.UUID) (*domain.Driver, error) {
			driverLookupTenant = tenant
			return &domain.Driver{ID: driverID, TenantID: tenantID, CarrierCompanyID: carrierID}, nil
		},
	}, &mockVehicleLookup{})

	if _, err := svc.AssignDriver(context.Background(), tenantID, shipmentID, driverID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if shipmentLookupTenant != tenantID || driverLookupTenant != tenantID || assignTenant != tenantID {
		t.Fatalf("tenant mismatch shipment=%s driver=%s assign=%s", shipmentLookupTenant, driverLookupTenant, assignTenant)
	}
}

func TestAssignVehicleVerifiedTenantPassedToRepositories(t *testing.T) {
	t.Parallel()
	carrierID := uuid.New()
	tenantID := uuid.New()
	shipmentID := uuid.New()
	vehicleID := uuid.New()

	var shipmentLookupTenant uuid.UUID
	var vehicleLookupTenant uuid.UUID
	var assignTenant uuid.UUID

	svc := NewShipmentService(&mockShipmentStore{
		getByIDAndTenantFn: func(_ context.Context, id, tenant uuid.UUID) (*domain.Shipment, error) {
			shipmentLookupTenant = tenant
			return &domain.Shipment{
				ID: shipmentID, TenantID: tenantID, Status: domain.ShipmentStatusAcceptedByCarrier,
				CarrierCompanyID: &carrierID, Version: 1,
			}, nil
		},
		assignVehicleFn: func(_ context.Context, id, tenant, gotVehicleID uuid.UUID, _ string, _ int) (*domain.Shipment, error) {
			assignTenant = tenant
			return &domain.Shipment{Status: domain.ShipmentStatusVehicleAssigned, VehicleID: &gotVehicleID}, nil
		},
	}, &mockDriverLookup{}, &mockVehicleLookup{
		getByIDAndTenantFn: func(_ context.Context, id, tenant uuid.UUID) (*domain.Vehicle, error) {
			vehicleLookupTenant = tenant
			return &domain.Vehicle{ID: vehicleID, TenantID: tenantID, CarrierCompanyID: carrierID}, nil
		},
	})

	if _, err := svc.AssignVehicle(context.Background(), tenantID, shipmentID, vehicleID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if shipmentLookupTenant != tenantID || vehicleLookupTenant != tenantID || assignTenant != tenantID {
		t.Fatalf("tenant mismatch shipment=%s vehicle=%s assign=%s", shipmentLookupTenant, vehicleLookupTenant, assignTenant)
	}
}
