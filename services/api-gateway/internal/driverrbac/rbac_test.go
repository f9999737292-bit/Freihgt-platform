package driverrbac

import "testing"

func TestCanAccessDriverRoutes(t *testing.T) {
	if !CanAccessDriverRoutes([]string{"DRIVER"}) {
		t.Fatal("expected DRIVER allowed")
	}
	if CanAccessDriverRoutes([]string{"CARRIER_DISPATCHER"}) {
		t.Fatal("expected dispatcher denied")
	}
	if CanAccessDriverRoutes([]string{"PLATFORM_ADMIN"}) {
		t.Fatal("expected admin denied on driver routes")
	}
}
