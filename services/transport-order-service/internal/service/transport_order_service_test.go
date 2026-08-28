package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/transport-order-service/internal/domain"
	apperrors "github.com/freight-platform/transport-order-service/internal/platform/errors"
)

type mockOrderStore struct {
	getByIDAndTenantFn     func(ctx context.Context, id, tenantID uuid.UUID) (*domain.TransportOrder, error)
	getCarrierCompanyIDFn  func(ctx context.Context, tenantID, orderID uuid.UUID) (*uuid.UUID, error)
	updateStatusByTenantFn func(ctx context.Context, tenantID, id uuid.UUID, expectedStatus, newStatus string) (*domain.TransportOrder, error)
}

func (m *mockOrderStore) CompanyExists(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}

func (m *mockOrderStore) Create(context.Context, domain.CreateTransportOrderInput) (*domain.TransportOrder, error) {
	return nil, nil
}

func (m *mockOrderStore) GetByIDAndTenant(ctx context.Context, id, tenantID uuid.UUID) (*domain.TransportOrder, error) {
	return m.getByIDAndTenantFn(ctx, id, tenantID)
}

func (m *mockOrderStore) GetCarrierCompanyID(ctx context.Context, tenantID, orderID uuid.UUID) (*uuid.UUID, error) {
	if m.getCarrierCompanyIDFn != nil {
		return m.getCarrierCompanyIDFn(ctx, tenantID, orderID)
	}
	return nil, nil
}

func (m *mockOrderStore) List(context.Context, domain.ListTransportOrdersFilter) ([]domain.TransportOrder, int, error) {
	return nil, 0, nil
}

func (m *mockOrderStore) UpdateByIDAndTenant(context.Context, uuid.UUID, uuid.UUID, domain.UpdateTransportOrderInput) (*domain.TransportOrder, error) {
	return nil, nil
}

func (m *mockOrderStore) UpdateStatusByIDAndTenant(ctx context.Context, tenantID, id uuid.UUID, expectedStatus, newStatus string) (*domain.TransportOrder, error) {
	return m.updateStatusByTenantFn(ctx, tenantID, id, expectedStatus, newStatus)
}

type mockLocationStore struct{}

func (m *mockLocationStore) CompanyExists(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}

func (m *mockLocationStore) Create(context.Context, domain.CreateLocationInput) (*domain.Location, error) {
	return nil, nil
}

func (m *mockLocationStore) GetByIDAndTenant(context.Context, uuid.UUID, uuid.UUID) (*domain.Location, error) {
	return nil, nil
}

func (m *mockLocationStore) List(context.Context, domain.ListLocationsFilter) ([]domain.Location, int, error) {
	return nil, 0, nil
}

func (m *mockLocationStore) ExistsInTenant(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}

type mockCargoStore struct{}

func (m *mockCargoStore) Create(context.Context, domain.CreateCargoInput) (*domain.Cargo, error) {
	return nil, nil
}

func (m *mockCargoStore) GetByIDAndTenant(context.Context, uuid.UUID, uuid.UUID) (*domain.Cargo, error) {
	return nil, nil
}

func (m *mockCargoStore) ExistsInTenant(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}

func buyerActor(companyID uuid.UUID) domain.OrderAccessActor {
	return domain.OrderAccessActor{CompanyID: companyID, ActorKind: domain.ActorKindBuyer}
}

func TestTransportOrderServiceSubmitOnlyFromDraft(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	orderID := uuid.New()
	shipperID := uuid.New()
	svc := NewTransportOrderService(&mockLocationStore{}, &mockCargoStore{}, &mockOrderStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.TransportOrder, error) {
			return &domain.TransportOrder{ID: orderID, TenantID: tenantID, ShipperCompanyID: shipperID, Status: domain.TransportOrderStatusReadyForSourcing}, nil
		},
		updateStatusByTenantFn: func(context.Context, uuid.UUID, uuid.UUID, string, string) (*domain.TransportOrder, error) {
			return nil, errors.New("should not update")
		},
	}, &mockLocationStore{})

	_, err := svc.SubmitTransportOrder(context.Background(), tenantID, orderID, buyerActor(shipperID))
	if err == nil {
		t.Fatalf("expected validation error")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeValidation {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestTransportOrderServiceSubmitFromDraft(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	orderID := uuid.New()
	shipperID := uuid.New()
	svc := NewTransportOrderService(&mockLocationStore{}, &mockCargoStore{}, &mockOrderStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.TransportOrder, error) {
			return &domain.TransportOrder{ID: orderID, TenantID: tenantID, ShipperCompanyID: shipperID, Status: domain.TransportOrderStatusDraft}, nil
		},
		updateStatusByTenantFn: func(_ context.Context, gotTenant, id uuid.UUID, expectedStatus, newStatus string) (*domain.TransportOrder, error) {
			if gotTenant != tenantID {
				t.Fatalf("unexpected tenant")
			}
			if expectedStatus != domain.TransportOrderStatusDraft || newStatus != domain.TransportOrderStatusReadyForSourcing {
				t.Fatalf("unexpected status transition")
			}
			return &domain.TransportOrder{ID: id, Status: newStatus, UpdatedAt: time.Now().UTC()}, nil
		},
	}, &mockLocationStore{})

	order, err := svc.SubmitTransportOrder(context.Background(), tenantID, orderID, buyerActor(shipperID))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if order.Status != domain.TransportOrderStatusReadyForSourcing {
		t.Fatalf("unexpected status: %s", order.Status)
	}
}

func TestTransportOrderServiceCancelAllowedStatuses(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	orderID := uuid.New()
	shipperID := uuid.New()
	svc := NewTransportOrderService(&mockLocationStore{}, &mockCargoStore{}, &mockOrderStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.TransportOrder, error) {
			return &domain.TransportOrder{ID: orderID, TenantID: tenantID, ShipperCompanyID: shipperID, Status: domain.TransportOrderStatusAssigned}, nil
		},
		updateStatusByTenantFn: func(context.Context, uuid.UUID, uuid.UUID, string, string) (*domain.TransportOrder, error) {
			return nil, nil
		},
	}, &mockLocationStore{})

	_, err := svc.CancelTransportOrder(context.Background(), tenantID, orderID, buyerActor(shipperID))
	if err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestTransportOrderServiceCrossTenantGetDenied(t *testing.T) {
	t.Parallel()

	tenantA := uuid.New()
	tenantB := uuid.New()
	orderID := uuid.New()
	shipperID := uuid.New()
	svc := NewTransportOrderService(&mockLocationStore{}, &mockCargoStore{}, &mockOrderStore{
		getByIDAndTenantFn: func(_ context.Context, id, tenantID uuid.UUID) (*domain.TransportOrder, error) {
			if tenantID == tenantB {
				return nil, apperrors.NotFound("transport order not found")
			}
			return &domain.TransportOrder{ID: id, TenantID: tenantA, ShipperCompanyID: shipperID}, nil
		},
	}, &mockLocationStore{})

	_, err := svc.GetTransportOrder(context.Background(), tenantB, orderID, buyerActor(uuid.New()))
	if err == nil {
		t.Fatal("expected not found for cross-tenant get")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestTransportOrderServiceCompanyIsolationDenied(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	orderID := uuid.New()
	shipperID := uuid.New()
	otherCompany := uuid.New()
	svc := NewTransportOrderService(&mockLocationStore{}, &mockCargoStore{}, &mockOrderStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.TransportOrder, error) {
			return &domain.TransportOrder{ID: orderID, TenantID: tenantID, ShipperCompanyID: shipperID, ConsigneeCompanyID: uuid.New()}, nil
		},
	}, &mockLocationStore{})

	_, err := svc.GetTransportOrder(context.Background(), tenantID, orderID, buyerActor(otherCompany))
	if err == nil {
		t.Fatal("expected not found for foreign company")
	}
}
