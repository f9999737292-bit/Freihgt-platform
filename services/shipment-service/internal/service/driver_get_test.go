package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

type mockDriverStore struct {
	getByIDAndTenantFn func(ctx context.Context, id, tenantID uuid.UUID) (*domain.Driver, error)
}

func (m *mockDriverStore) CompanyExists(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}
func (m *mockDriverStore) Create(context.Context, domain.CreateDriverInput) (*domain.Driver, error) {
	return nil, nil
}
func (m *mockDriverStore) GetByIDAndTenant(ctx context.Context, id, tenantID uuid.UUID) (*domain.Driver, error) {
	return m.getByIDAndTenantFn(ctx, id, tenantID)
}
func (m *mockDriverStore) List(context.Context, domain.ListDriversFilter) ([]domain.Driver, int, error) {
	return nil, 0, nil
}

func TestDriverServiceGetByIDAndTenantPassesTenant(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	driverID := uuid.New()
	svc := NewDriverService(&mockDriverStore{
		getByIDAndTenantFn: func(_ context.Context, id, tenant uuid.UUID) (*domain.Driver, error) {
			if id != driverID || tenant != tenantID {
				t.Fatalf("unexpected ids tenant=%s driver=%s", tenant, id)
			}
			return &domain.Driver{ID: id, TenantID: tenant, FullName: "Driver"}, nil
		},
	})
	driver, err := svc.GetByIDAndTenant(context.Background(), tenantID, driverID)
	if err != nil || driver.FullName != "Driver" {
		t.Fatalf("unexpected result: driver=%v err=%v", driver, err)
	}
}

func TestDriverServiceGetByIDAndTenantNotFound(t *testing.T) {
	t.Parallel()
	svc := NewDriverService(&mockDriverStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Driver, error) {
			return nil, apperrors.NotFound("driver not found")
		},
	})
	_, err := svc.GetByIDAndTenant(context.Background(), uuid.New(), uuid.New())
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestDriverServiceGetByIDAndTenantMissingTenantSkipsRepository(t *testing.T) {
	t.Parallel()
	called := false
	svc := NewDriverService(&mockDriverStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Driver, error) {
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

func TestDriverServiceGetByIDAndTenantForeignSameAsNotFound(t *testing.T) {
	t.Parallel()
	svc := NewDriverService(&mockDriverStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Driver, error) {
			return nil, apperrors.NotFound("driver not found")
		},
	})
	_, err := svc.GetByIDAndTenant(context.Background(), uuid.New(), uuid.New())
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeNotFound {
		t.Fatalf("foreign tenant must surface as not found, got %v", err)
	}
}

func TestDriverServiceGetByIDAndTenantInternalErrorNotNotFound(t *testing.T) {
	t.Parallel()
	svc := NewDriverService(&mockDriverStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Driver, error) {
			return nil, apperrors.Internal("db down", errors.New("connection reset"))
		},
	})
	_, err := svc.GetByIDAndTenant(context.Background(), uuid.New(), uuid.New())
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeInternal {
		t.Fatalf("expected internal error, got %v", err)
	}
}
