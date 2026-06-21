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
	getByIDFn func(ctx context.Context, id uuid.UUID) (*domain.TransportOrder, error)
	updateStatusFn func(ctx context.Context, id uuid.UUID, expectedStatus, newStatus string) (*domain.TransportOrder, error)
}

func (m *mockOrderStore) CompanyExists(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}

func (m *mockOrderStore) Create(context.Context, domain.CreateTransportOrderInput) (*domain.TransportOrder, error) {
	return nil, nil
}

func (m *mockOrderStore) GetByID(ctx context.Context, id uuid.UUID) (*domain.TransportOrder, error) {
	return m.getByIDFn(ctx, id)
}

func (m *mockOrderStore) List(context.Context, domain.ListTransportOrdersFilter) ([]domain.TransportOrder, int, error) {
	return nil, 0, nil
}

func (m *mockOrderStore) Update(context.Context, uuid.UUID, domain.UpdateTransportOrderInput) (*domain.TransportOrder, error) {
	return nil, nil
}

func (m *mockOrderStore) UpdateStatus(ctx context.Context, id uuid.UUID, expectedStatus, newStatus string) (*domain.TransportOrder, error) {
	return m.updateStatusFn(ctx, id, expectedStatus, newStatus)
}

type mockLocationStore struct{}

func (m *mockLocationStore) CompanyExists(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}

func (m *mockLocationStore) Create(context.Context, domain.CreateLocationInput) (*domain.Location, error) {
	return nil, nil
}

func (m *mockLocationStore) GetByID(context.Context, uuid.UUID) (*domain.Location, error) {
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

func (m *mockCargoStore) GetByID(context.Context, uuid.UUID) (*domain.Cargo, error) {
	return nil, nil
}

func (m *mockCargoStore) ExistsInTenant(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}

func TestTransportOrderServiceSubmitOnlyFromDraft(t *testing.T) {
	t.Parallel()

	orderID := uuid.New()
	svc := NewTransportOrderService(&mockLocationStore{}, &mockCargoStore{}, &mockOrderStore{
		getByIDFn: func(context.Context, uuid.UUID) (*domain.TransportOrder, error) {
			return &domain.TransportOrder{ID: orderID, Status: domain.TransportOrderStatusReadyForSourcing}, nil
		},
		updateStatusFn: func(context.Context, uuid.UUID, string, string) (*domain.TransportOrder, error) {
			return nil, errors.New("should not update")
		},
	}, &mockLocationStore{})

	_, err := svc.SubmitTransportOrder(context.Background(), orderID)
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

	orderID := uuid.New()
	svc := NewTransportOrderService(&mockLocationStore{}, &mockCargoStore{}, &mockOrderStore{
		getByIDFn: func(context.Context, uuid.UUID) (*domain.TransportOrder, error) {
			return &domain.TransportOrder{ID: orderID, Status: domain.TransportOrderStatusDraft}, nil
		},
		updateStatusFn: func(_ context.Context, id uuid.UUID, expectedStatus, newStatus string) (*domain.TransportOrder, error) {
			if expectedStatus != domain.TransportOrderStatusDraft || newStatus != domain.TransportOrderStatusReadyForSourcing {
				t.Fatalf("unexpected status transition")
			}
			return &domain.TransportOrder{ID: id, Status: newStatus, UpdatedAt: time.Now().UTC()}, nil
		},
	}, &mockLocationStore{})

	order, err := svc.SubmitTransportOrder(context.Background(), orderID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if order.Status != domain.TransportOrderStatusReadyForSourcing {
		t.Fatalf("unexpected status: %s", order.Status)
	}
}

func TestTransportOrderServiceCancelAllowedStatuses(t *testing.T) {
	t.Parallel()

	orderID := uuid.New()
	svc := NewTransportOrderService(&mockLocationStore{}, &mockCargoStore{}, &mockOrderStore{
		getByIDFn: func(context.Context, uuid.UUID) (*domain.TransportOrder, error) {
			return &domain.TransportOrder{ID: orderID, Status: domain.TransportOrderStatusAssigned}, nil
		},
		updateStatusFn: func(context.Context, uuid.UUID, string, string) (*domain.TransportOrder, error) {
			return nil, nil
		},
	}, &mockLocationStore{})

	_, err := svc.CancelTransportOrder(context.Background(), orderID)
	if err == nil {
		t.Fatalf("expected validation error")
	}
}
