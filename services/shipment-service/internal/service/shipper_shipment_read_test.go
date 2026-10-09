package service

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

func TestListForShipperRejectsMissingCompany(t *testing.T) {
	svc := NewShipmentService(&mockShipmentStore{}, &mockDriverLookup{}, &mockVehicleLookup{})
	_, _, err := svc.ListForShipper(context.Background(), domain.ShipperShipmentListFilter{TenantID: uuid.New(), Limit: 20})
	app, ok := err.(*apperrors.AppError)
	if !ok || app.Code != apperrors.CodeValidation {
		t.Fatalf("%v", err)
	}
}

func TestListForShipperFailsClosedOnForeignRow(t *testing.T) {
	tenant := uuid.New()
	shipper := uuid.New()
	store := &mockShipmentStore{
		listByShipperFn: func(context.Context, domain.ShipperShipmentListFilter) ([]domain.Shipment, int, error) {
			return []domain.Shipment{{ID: uuid.New(), TenantID: tenant, ShipperCompanyID: uuid.New(), ShipmentNumber: "FOREIGN"}}, 1, nil
		},
	}
	svc := NewShipmentService(store, &mockDriverLookup{}, &mockVehicleLookup{})
	items, total, err := svc.ListForShipper(context.Background(), domain.ShipperShipmentListFilter{
		TenantID: tenant, ShipperCompanyID: shipper, Limit: 20,
	})
	app, ok := err.(*apperrors.AppError)
	if !ok || app.Code != apperrors.CodeInternal || items != nil || total != 0 {
		t.Fatalf("items=%v total=%d err=%v", items, total, err)
	}
}

func TestGetForShipperHidesMismatchedParty(t *testing.T) {
	tenant := uuid.New()
	shipper := uuid.New()
	shipmentID := uuid.New()
	forwarder := shipper
	store := &mockShipmentStore{
		getByShipperFn: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			return &domain.Shipment{
				ID: shipmentID, TenantID: tenant, ShipperCompanyID: uuid.New(),
				ForwarderCompanyID: &forwarder, ShipmentNumber: "FWD-SECRET",
			}, nil
		},
	}
	svc := NewShipmentService(store, &mockDriverLookup{}, &mockVehicleLookup{})
	got, err := svc.GetForShipper(context.Background(), tenant, shipmentID, shipper)
	app, ok := err.(*apperrors.AppError)
	if !ok || app.Code != apperrors.CodeNotFound || got != nil || app.Message == "FWD-SECRET" {
		t.Fatalf("got=%v err=%v", got, err)
	}
}
