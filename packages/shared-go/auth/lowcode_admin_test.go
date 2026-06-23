package auth

import "testing"

func TestHasLowCodeAdminRole(t *testing.T) {
	if !HasLowCodeAdminRole([]string{"SHIPPER_ADMIN", "PLATFORM_ADMIN"}) {
		t.Fatal("expected PLATFORM_ADMIN to grant access")
	}
	if HasLowCodeAdminRole([]string{"SHIPPER_ADMIN"}) {
		t.Fatal("expected non-admin role to be denied")
	}
	if HasLowCodeAdminRole(nil) {
		t.Fatal("expected empty roles to be denied")
	}
}
