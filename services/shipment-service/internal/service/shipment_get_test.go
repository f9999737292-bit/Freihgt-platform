package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

func TestShipmentServiceGetByIDAndTenantPassesIDsToRepository(t *testing.T) {
	t.Parallel()
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	shipmentID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")

	var gotTenant, gotShipment uuid.UUID
	svc := NewShipmentService(&mockShipmentStore{
		getByIDAndTenantFn: func(_ context.Context, id, tenant uuid.UUID) (*domain.Shipment, error) {
			gotTenant = tenant
			gotShipment = id
			return &domain.Shipment{ID: id, TenantID: tenant, Status: domain.ShipmentStatusInTransit}, nil
		},
	}, &mockDriverLookup{}, &mockVehicleLookup{})

	shipment, err := svc.GetByIDAndTenant(context.Background(), tenantID, shipmentID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotTenant != tenantID || gotShipment != shipmentID {
		t.Fatalf("repository called with tenant=%s shipment=%s", gotTenant, gotShipment)
	}
	if shipment.TenantID != tenantID {
		t.Fatalf("unexpected tenant in result")
	}
}

func TestShipmentServiceGetByIDAndTenantNotFound(t *testing.T) {
	t.Parallel()
	svc := NewShipmentService(&mockShipmentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			return nil, apperrors.NotFound("shipment not found")
		},
	}, &mockDriverLookup{}, &mockVehicleLookup{})

	_, err := svc.GetByIDAndTenant(context.Background(), uuid.New(), uuid.New())
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestShipmentServiceGetByIDAndTenantForeignTenantSameAsNotFound(t *testing.T) {
	t.Parallel()
	svc := NewShipmentService(&mockShipmentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			return nil, apperrors.NotFound("shipment not found")
		},
	}, &mockDriverLookup{}, &mockVehicleLookup{})

	_, err := svc.GetByIDAndTenant(context.Background(), uuid.New(), uuid.New())
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeNotFound {
		t.Fatalf("foreign tenant must surface as not found, got %v", err)
	}
}

func TestShipmentServiceGetByIDAndTenantMissingTenantSkipsRepository(t *testing.T) {
	t.Parallel()
	called := false
	svc := NewShipmentService(&mockShipmentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			called = true
			return nil, nil
		},
	}, &mockDriverLookup{}, &mockVehicleLookup{})

	_, err := svc.GetByIDAndTenant(context.Background(), uuid.Nil, uuid.New())
	if called {
		t.Fatal("repository must not be called without tenant")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeUnauthorized {
		t.Fatalf("expected unauthorized, got %v", err)
	}
}

func TestShipmentServiceGetByIDAndTenantUnexpectedErrorNotNotFound(t *testing.T) {
	t.Parallel()
	svc := NewShipmentService(&mockShipmentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			return nil, apperrors.Internal("db down", errors.New("connection reset"))
		},
	}, &mockDriverLookup{}, &mockVehicleLookup{})

	_, err := svc.GetByIDAndTenant(context.Background(), uuid.New(), uuid.New())
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeInternal {
		t.Fatalf("expected internal error, got %v", err)
	}
}
