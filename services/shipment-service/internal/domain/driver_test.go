package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestValidateCreateDriverInput(t *testing.T) {
	t.Parallel()
	if err := ValidateCreateDriverInput(CreateDriverInput{
		TenantID: uuid.New(), CarrierCompanyID: uuid.New(), FullName: "Иван Водитель",
	}); err != nil {
		t.Fatalf("expected valid input, got %v", err)
	}
	if err := ValidateCreateDriverInput(CreateDriverInput{
		TenantID: uuid.New(), CarrierCompanyID: uuid.New(),
	}); err == nil {
		t.Fatalf("expected validation error for missing full_name")
	}
}
