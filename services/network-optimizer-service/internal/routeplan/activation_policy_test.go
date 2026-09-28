package routeplan

import "testing"

func TestNLO04CActivationStatusSets(t *testing.T) {
	if !ProductionActivationRequiresServiceDurationSource {
		t.Fatal("production activation requires a service-duration source")
	}
	for _, status := range []string{"CARRIER_ASSIGNED", "ACCEPTED_BY_CARRIER", "VEHICLE_ASSIGNED", "DRIVER_ASSIGNED", "PICKUP_SLOT_BOOKED"} {
		if !ActivationStatusAllowed(ModeDepotStart, status) || ActivationStatusAllowed(ModeCurrentTrip, status) {
			t.Fatalf("depot status %s", status)
		}
	}
	for _, status := range []string{"IN_PICKUP", "LOADED", "IN_TRANSIT"} {
		if !ActivationStatusAllowed(ModeCurrentTrip, status) || ActivationStatusAllowed(ModeDepotStart, status) {
			t.Fatalf("current trip status %s", status)
		}
	}
	for _, status := range []string{"", "ARRIVED_AT_CONSIGNEE", "UNLOADING", "DELIVERY_SLOT_BOOKED", "DRIVER_ASSIGNED"} {
		if status == "DRIVER_ASSIGNED" {
			continue
		}
		if ActivationStatusAllowed(ModeCurrentTrip, status) {
			t.Fatalf("current trip allowed %s", status)
		}
	}
	if ActivationStatusAllowed(ModeCurrentTrip, "DRIVER_ASSIGNED") || ActivationStatusAllowed(ModeDepotStart, "IN_TRANSIT") {
		t.Fatal("status sets overlap")
	}
}
