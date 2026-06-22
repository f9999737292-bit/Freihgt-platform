package domain

import "testing"

func TestValidateEntityTypeValid(t *testing.T) {
	if err := ValidateEntityType("TRANSPORT_ORDER"); err != nil {
		t.Fatalf("expected valid entity type, got %v", err)
	}
}

func TestValidateEntityTypeEmpty(t *testing.T) {
	if err := ValidateEntityType(""); err != nil {
		t.Fatalf("expected empty entity type to pass, got %v", err)
	}
}

func TestValidateEntityTypeInvalid(t *testing.T) {
	if err := ValidateEntityType("INVALID"); err == nil {
		t.Fatal("expected validation error for invalid entity_type")
	}
}
