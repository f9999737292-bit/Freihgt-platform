package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestValidateCreateTransportOrderInput(t *testing.T) {
	t.Parallel()

	validTenant := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	validID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	pickup := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	delivery := time.Date(2026, 7, 3, 0, 0, 0, 0, time.UTC)

	if err := ValidateCreateTransportOrderInput(CreateTransportOrderInput{
		TenantID:              validTenant,
		OrderNumber:           "TO-2026-000001",
		ShipperCompanyID:      validID,
		ConsigneeCompanyID:    validID,
		OriginLocationID:      validID,
		DestinationLocationID: validID,
		CargoID:               validID,
		RequestedPickupDate:   &pickup,
		RequestedDeliveryDate: &delivery,
		TransportMode:         TransportModeRoad,
	}); err != nil {
		t.Fatalf("expected valid input, got %v", err)
	}

	if err := ValidateCreateTransportOrderInput(CreateTransportOrderInput{
		OrderNumber:        "TO-2026-000001",
		ShipperCompanyID:   validID,
		ConsigneeCompanyID: validID,
		OriginLocationID:   validID,
		DestinationLocationID: validID,
		CargoID:            validID,
		TransportMode:      TransportModeRoad,
	}); err == nil {
		t.Fatalf("expected tenant_id validation error")
	}
}

func TestValidateCreateTransportOrderDeliveryBeforePickup(t *testing.T) {
	t.Parallel()

	validTenant := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	validID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	pickup := time.Date(2026, 7, 5, 0, 0, 0, 0, time.UTC)
	delivery := time.Date(2026, 7, 3, 0, 0, 0, 0, time.UTC)

	if err := ValidateCreateTransportOrderInput(CreateTransportOrderInput{
		TenantID:              validTenant,
		OrderNumber:           "TO-2026-000001",
		ShipperCompanyID:      validID,
		ConsigneeCompanyID:    validID,
		OriginLocationID:      validID,
		DestinationLocationID: validID,
		CargoID:               validID,
		RequestedPickupDate:   &pickup,
		RequestedDeliveryDate: &delivery,
		TransportMode:         TransportModeRoad,
	}); err == nil {
		t.Fatalf("expected delivery date validation error")
	}
}

func TestValidateSubmitTransportOrder(t *testing.T) {
	t.Parallel()

	if err := ValidateSubmitTransportOrder(TransportOrderStatusDraft); err != nil {
		t.Fatalf("expected valid submit from DRAFT, got %v", err)
	}

	if err := ValidateSubmitTransportOrder(TransportOrderStatusReadyForSourcing); err == nil {
		t.Fatalf("expected validation error for non-DRAFT submit")
	}
}

func TestValidateCancelTransportOrder(t *testing.T) {
	t.Parallel()

	if err := ValidateCancelTransportOrder(TransportOrderStatusDraft); err != nil {
		t.Fatalf("expected cancel from DRAFT, got %v", err)
	}
	if err := ValidateCancelTransportOrder(TransportOrderStatusReadyForSourcing); err != nil {
		t.Fatalf("expected cancel from READY_FOR_SOURCING, got %v", err)
	}
	if err := ValidateCancelTransportOrder(TransportOrderStatusAssigned); err == nil {
		t.Fatalf("expected validation error for ASSIGNED cancel")
	}
}

func TestValidateUpdateTransportOrderStatus(t *testing.T) {
	t.Parallel()

	if err := ValidateUpdateTransportOrderStatus(TransportOrderStatusDraft); err != nil {
		t.Fatalf("expected update in DRAFT, got %v", err)
	}
	if err := ValidateUpdateTransportOrderStatus(TransportOrderStatusReadyForSourcing); err == nil {
		t.Fatalf("expected validation error for non-DRAFT update")
	}
}
