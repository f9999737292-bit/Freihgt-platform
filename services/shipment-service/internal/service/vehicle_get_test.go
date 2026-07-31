package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

type mockVehicleStore struct {
	getByIDAndTenantFn func(ctx context.Context, id, tenantID uuid.UUID) (*domain.Vehicle, error)
	createFn           func(ctx context.Context, tenantID uuid.UUID, in domain.CreateVehicleInput) (*domain.Vehicle, error)
	listFn             func(ctx context.Context, tenantID uuid.UUID, filter domain.ListVehiclesFilter) ([]domain.Vehicle, int, error)
	companyExistsFn    func(ctx context.Context, companyID, tenantID uuid.UUID) (bool, error)
}

func (m *mockVehicleStore) CompanyExists(ctx context.Context, companyID, tenantID uuid.UUID) (bool, error) {
	if m.companyExistsFn != nil {
		return m.companyExistsFn(ctx, companyID, tenantID)
	}
	return true, nil
}
func (m *mockVehicleStore) Create(ctx context.Context, tenantID uuid.UUID, in domain.CreateVehicleInput) (*domain.Vehicle, error) {
	if m.createFn != nil {
		return m.createFn(ctx, tenantID, in)
	}
	return nil, nil
}
func (m *mockVehicleStore) GetByIDAndTenant(ctx context.Context, id, tenantID uuid.UUID) (*domain.Vehicle, error) {
	return m.getByIDAndTenantFn(ctx, id, tenantID)
}
func (m *mockVehicleStore) List(ctx context.Context, tenantID uuid.UUID, filter domain.ListVehiclesFilter) ([]domain.Vehicle, int, error) {
	if m.listFn != nil {
		return m.listFn(ctx, tenantID, filter)
	}
	return nil, 0, nil
}

func TestVehicleServiceGetByIDAndTenantPassesTenant(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	vehicleID := uuid.New()
	svc := NewVehicleService(&mockVehicleStore{
		getByIDAndTenantFn: func(_ context.Context, id, tenant uuid.UUID) (*domain.Vehicle, error) {
			if id != vehicleID || tenant != tenantID {
				t.Fatalf("unexpected ids tenant=%s vehicle=%s", tenant, id)
			}
			return &domain.Vehicle{ID: id, TenantID: tenant, PlateNumber: "A123"}, nil
		},
	})
	vehicle, err := svc.GetByIDAndTenant(context.Background(), tenantID, vehicleID)
	if err != nil || vehicle.PlateNumber != "A123" {
		t.Fatalf("unexpected result: vehicle=%v err=%v", vehicle, err)
	}
}

func TestVehicleServiceGetByIDAndTenantNotFound(t *testing.T) {
	t.Parallel()
	svc := NewVehicleService(&mockVehicleStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Vehicle, error) {
			return nil, apperrors.NotFound("vehicle not found")
		},
	})
	_, err := svc.GetByIDAndTenant(context.Background(), uuid.New(), uuid.New())
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestVehicleServiceGetByIDAndTenantMissingTenantSkipsRepository(t *testing.T) {
	t.Parallel()
	called := false
	svc := NewVehicleService(&mockVehicleStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Vehicle, error) {
			called = true
			return nil, nil
		},
	})
	_, err := svc.GetByIDAndTenant(context.Background(), uuid.Nil, uuid.New())
	if called {
		t.Fatal("repository must not be called")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeUnauthorized {
		t.Fatalf("expected unauthorized, got %v", err)
	}
}

func TestVehicleServiceGetByIDAndTenantForeignSameAsNotFound(t *testing.T) {
	t.Parallel()
	svc := NewVehicleService(&mockVehicleStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Vehicle, error) {
			return nil, apperrors.NotFound("vehicle not found")
		},
	})
	_, err := svc.GetByIDAndTenant(context.Background(), uuid.New(), uuid.New())
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeNotFound {
		t.Fatalf("foreign tenant must surface as not found, got %v", err)
	}
}

func TestVehicleServiceGetByIDAndTenantInternalErrorNotNotFound(t *testing.T) {
	t.Parallel()
	svc := NewVehicleService(&mockVehicleStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Vehicle, error) {
			return nil, apperrors.Internal("db down", errors.New("connection reset"))
		},
	})
	_, err := svc.GetByIDAndTenant(context.Background(), uuid.New(), uuid.New())
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeInternal {
		t.Fatalf("expected internal error, got %v", err)
	}
}
