package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestValidateCreateFreightRequestInput(t *testing.T) {
	t.Parallel()
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	orderID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	shipperID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	if err := ValidateCreateFreightRequestInput(CreateFreightRequestFromOrderInput{
		TenantID: tenantID, TransportOrderID: orderID, FreightRequestNumber: "FR-1",
		RequestType: "MINI_TENDER", ShipperCompanyID: shipperID,
	}); err != nil {
		t.Fatalf("expected valid input, got %v", err)
	}
}

func TestValidateTransportOrderForFreightRequest(t *testing.T) {
	t.Parallel()
	if err := ValidateTransportOrderForFreightRequest(TransportOrderStatusReadyForSourcing); err != nil {
		t.Fatalf("expected valid status")
	}
	if err := ValidateTransportOrderForFreightRequest("DRAFT"); err == nil {
		t.Fatalf("expected validation error")
	}
}
