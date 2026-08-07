package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

func TestDriverServiceCreateMissingTenantSkipsRepository(t *testing.T) {
	t.Parallel()
	called := false
	svc := NewDriverService(&mockDriverStore{
		createFn: func(context.Context, uuid.UUID, domain.CreateDriverInput) (*domain.Driver, error) {
			called = true
			return nil, nil
		},
	})
	_, err := svc.Create(context.Background(), uuid.Nil, domain.CreateDriverInput{
		CarrierCompanyID: uuid.New(), FullName: "Driver",
	})
	if called {
		t.Fatal("repository must not be called without tenant")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeUnauthorized {
		t.Fatalf("expected unauthorized, got %v", err)
	}
}

func TestDriverServiceCreatePassesVerifiedTenant(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	companyID := uuid.New()
	var createTenant uuid.UUID
	svc := NewDriverService(&mockDriverStore{
		companyExistsFn: func(_ context.Context, company, tenant uuid.UUID) (bool, error) {
			if company != companyID || tenant != tenantID {
				t.Fatalf("unexpected company validation company=%s tenant=%s", company, tenant)
			}
			return true, nil
		},
		createFn: func(_ context.Context, tenant uuid.UUID, in domain.CreateDriverInput) (*domain.Driver, error) {
			createTenant = tenant
			return &domain.Driver{TenantID: tenant, CarrierCompanyID: in.CarrierCompanyID, FullName: in.FullName}, nil
		},
	})
	driver, err := svc.Create(context.Background(), tenantID, domain.CreateDriverInput{
		CarrierCompanyID: companyID, FullName: "Driver A",
	})
	if err != nil || createTenant != tenantID || driver.FullName != "Driver A" {
		t.Fatalf("unexpected result driver=%v err=%v tenant=%s", driver, err, createTenant)
	}
}

func TestDriverServiceCreateForeignCompanyReturns404(t *testing.T) {
	t.Parallel()
	svc := NewDriverService(&mockDriverStore{
		companyExistsFn: func(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
			return false, nil
		},
		createFn: func(context.Context, uuid.UUID, domain.CreateDriverInput) (*domain.Driver, error) {
			t.Fatal("create must not run when company missing")
			return nil, nil
		},
	})
	_, err := svc.Create(context.Background(), uuid.New(), domain.CreateDriverInput{
		CarrierCompanyID: uuid.New(), FullName: "Driver",
	})
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestDriverServiceListMissingTenantSkipsRepository(t *testing.T) {
	t.Parallel()
	called := false
	svc := NewDriverService(&mockDriverStore{
		listFn: func(context.Context, uuid.UUID, domain.ListDriversFilter) ([]domain.Driver, int, error) {
			called = true
			return nil, 0, nil
		},
	})
	_, _, err := svc.List(context.Background(), uuid.Nil, domain.ListDriversFilter{Limit: 20})
	if called {
		t.Fatal("repository must not be called without tenant")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeUnauthorized {
		t.Fatalf("expected unauthorized, got %v", err)
	}
}

func TestDriverServiceListPassesVerifiedTenant(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	var listTenant uuid.UUID
	svc := NewDriverService(&mockDriverStore{
		listFn: func(_ context.Context, tenant uuid.UUID, filter domain.ListDriversFilter) ([]domain.Driver, int, error) {
			listTenant = tenant
			return []domain.Driver{{FullName: "A"}}, 1, nil
		},
	})
	drivers, total, err := svc.List(context.Background(), tenantID, domain.ListDriversFilter{Limit: 20})
	if err != nil || listTenant != tenantID || total != 1 || len(drivers) != 1 {
		t.Fatalf("unexpected list result=%v total=%d err=%v tenant=%s", drivers, total, err, listTenant)
	}
}

func TestVehicleServiceCreateMissingTenantSkipsRepository(t *testing.T) {
	t.Parallel()
	called := false
	svc := NewVehicleService(&mockVehicleStore{
		createFn: func(context.Context, uuid.UUID, domain.CreateVehicleInput) (*domain.Vehicle, error) {
			called = true
			return nil, nil
		},
	})
	_, err := svc.Create(context.Background(), uuid.Nil, domain.CreateVehicleInput{
		CarrierCompanyID: uuid.New(), PlateNumber: "A123",
	})
	if called {
		t.Fatal("repository must not be called without tenant")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeUnauthorized {
		t.Fatalf("expected unauthorized, got %v", err)
	}
}

func TestVehicleServiceCreatePassesVerifiedTenant(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	companyID := uuid.New()
	var createTenant uuid.UUID
	svc := NewVehicleService(&mockVehicleStore{
		companyExistsFn: func(_ context.Context, company, tenant uuid.UUID) (bool, error) {
			return company == companyID && tenant == tenantID, nil
		},
		createFn: func(_ context.Context, tenant uuid.UUID, in domain.CreateVehicleInput) (*domain.Vehicle, error) {
			createTenant = tenant
			return &domain.Vehicle{TenantID: tenant, PlateNumber: in.PlateNumber}, nil
		},
	})
	vehicle, err := svc.Create(context.Background(), tenantID, domain.CreateVehicleInput{
		CarrierCompanyID: companyID, PlateNumber: "A123", VehicleType: domain.VehicleTypeTruck,
	})
	if err != nil || createTenant != tenantID || vehicle.PlateNumber != "A123" {
		t.Fatalf("unexpected result vehicle=%v err=%v tenant=%s", vehicle, err, createTenant)
	}
}

func TestVehicleServiceListPassesVerifiedTenant(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	var listTenant uuid.UUID
	svc := NewVehicleService(&mockVehicleStore{
		listFn: func(_ context.Context, tenant uuid.UUID, _ domain.ListVehiclesFilter) ([]domain.Vehicle, int, error) {
			listTenant = tenant
			return []domain.Vehicle{{PlateNumber: "A123"}}, 1, nil
		},
	})
	vehicles, total, err := svc.List(context.Background(), tenantID, domain.ListVehiclesFilter{Limit: 20})
	if err != nil || listTenant != tenantID || total != 1 || len(vehicles) != 1 {
		t.Fatalf("unexpected list result=%v total=%d err=%v tenant=%s", vehicles, total, err, listTenant)
	}
}
