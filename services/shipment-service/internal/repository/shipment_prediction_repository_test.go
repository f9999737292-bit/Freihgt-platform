package repository

import (
	"strings"
	"testing"
)

func TestPredictionInputQueryIsTenantScopedAndOmitsCargo(t *testing.T) {
	t.Parallel()
	query := strings.ToLower(predictionInputQuery + " " + otherActiveVehicleShipmentsQuery)
	if !strings.Contains(query, "s.tenant_id = $2") || !strings.Contains(query, "tenant_id = $1") {
		t.Fatal("prediction input must be tenant scoped")
	}
	for _, absent := range []string{"gross_weight", "driver_id", "plate_number", "phone", "email"} {
		if strings.Contains(query, absent) {
			t.Fatalf("prediction input must not select %s", absent)
		}
	}
	if !strings.Contains(query, "destination_location_id") || !strings.Contains(query, "planned_delivery_at") {
		t.Fatal("prediction input must include destination and planned delivery")
	}
}
