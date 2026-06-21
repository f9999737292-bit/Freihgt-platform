package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestValidateCreateCargoInput(t *testing.T) {
	t.Parallel()

	validTenant := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	if err := ValidateCreateCargoInput(CreateCargoInput{
		TenantID:  validTenant,
		CargoType: "FMCG",
		Items: []CreateCargoItemInput{{
			Name:     "Товар 1",
			Quantity: 100,
			Unit:     "PALLET",
		}},
	}); err != nil {
		t.Fatalf("expected valid input, got %v", err)
	}

	if err := ValidateCreateCargoInput(CreateCargoInput{
		TenantID:  validTenant,
		CargoType: "FMCG",
	}); err == nil {
		t.Fatalf("expected items validation error")
	}
}
