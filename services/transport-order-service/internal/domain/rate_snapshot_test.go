package domain

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func validPricedCreateInput(key, equipment string) CreatePricedTransportOrderInput {
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	companyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	refID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	return CreatePricedTransportOrderInput{
		CreateTransportOrderInput: CreateTransportOrderInput{
			TenantID:              tenantID,
			OrderNumber:           "TO-2026-000001",
			ShipperCompanyID:      companyID,
			ConsigneeCompanyID:    companyID,
			OriginLocationID:      refID,
			DestinationLocationID: refID,
			CargoID:               refID,
			TransportMode:         TransportModeRoad,
			EquipmentType:         &equipment,
		},
		Actor: InternalActor{
			TenantID:  tenantID,
			UserID:    uuid.MustParse("44444444-4444-4444-4444-444444444444"),
			CompanyID: companyID,
			ActorKind: "USER",
		},
		PricingContext: PricingContext{CarrierCompanyID: uuid.MustParse("55555555-5555-5555-5555-555555555555")},
		IdempotencyKey: key,
	}
}

func TestCTo013PublicCreateMissingIdempotencyKeyDeny(t *testing.T) {
	t.Parallel()

	if err := ValidateIdempotencyKey(""); err == nil {
		t.Fatal("expected empty idempotency key validation error")
	}
	if err := ValidateIdempotencyKey("   "); err == nil {
		t.Fatal("expected whitespace idempotency key validation error")
	}
	if err := ValidateIdempotencyKey(strings.Repeat("a", MaxIdempotencyKeyLength+1)); err == nil {
		t.Fatal("expected max length idempotency key validation error")
	}
	if err := ValidateIdempotencyKey("valid-key"); err != nil {
		t.Fatalf("expected valid key, got %v", err)
	}

	in := validPricedCreateInput("", "TAUTLINER")
	if err := ValidateCreatePricedTransportOrderInput(in); err == nil {
		t.Fatal("expected priced create validation error for missing idempotency key")
	}
}

func TestCTo015EquipmentBoxNotEqualBOX(t *testing.T) {
	t.Parallel()

	boxInput := validPricedCreateInput("eq-key", "Box")
	boxHash, err := ComputeCreateRequestHash(boxInput)
	if err != nil {
		t.Fatalf("box hash: %v", err)
	}
	upperInput := validPricedCreateInput("eq-key", "BOX")
	upperHash, err := ComputeCreateRequestHash(upperInput)
	if err != nil {
		t.Fatalf("BOX hash: %v", err)
	}
	if boxHash == upperHash {
		t.Fatal("request hash must differ for Box vs BOX equipment_type casing")
	}
}

func TestNormalizeEquipmentTypeTrimsOnly(t *testing.T) {
	t.Parallel()

	normalized, err := NormalizeEquipmentType("  Box  ")
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if normalized != "Box" {
		t.Fatalf("expected trim-only normalization, got %q", normalized)
	}
}
