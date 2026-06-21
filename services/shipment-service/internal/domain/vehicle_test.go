package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestValidateCreateVehicleInput(t *testing.T) {
	t.Parallel()
	if err := ValidateCreateVehicleInput(CreateVehicleInput{
		TenantID: uuid.New(), CarrierCompanyID: uuid.New(), PlateNumber: "А123ВС777",
		VehicleType: VehicleTypeTruck,
	}); err != nil {
		t.Fatalf("expected valid input, got %v", err)
	}
	if err := ValidateCreateVehicleInput(CreateVehicleInput{
		TenantID: uuid.New(), CarrierCompanyID: uuid.New(),
	}); err == nil {
		t.Fatalf("expected validation error for missing plate_number")
	}
}
