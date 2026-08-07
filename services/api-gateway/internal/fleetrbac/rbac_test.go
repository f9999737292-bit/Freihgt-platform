package fleetrbac

import (
	"testing"
)

func TestRBACViewTable(t *testing.T) {
	tests := []struct {
		roles []string
		allow bool
	}{
		{[]string{"PLATFORM_ADMIN"}, true},
		{[]string{"CARRIER_ADMIN"}, true},
		{[]string{"CARRIER_DISPATCHER"}, true},
		{[]string{"FINANCE_MANAGER"}, false},
		{[]string{"PROCUREMENT_MANAGER"}, false},
		{[]string{"SHIPPER_LOGIST"}, false},
		{[]string{"UNKNOWN_ROLE"}, false},
		{nil, false},
	}
	for _, tt := range tests {
		if got := CanViewFleet(tt.roles); got != tt.allow {
			t.Fatalf("CanViewFleet(%v)=%v want %v", tt.roles, got, tt.allow)
		}
	}
}

func TestRBACCreateTable(t *testing.T) {
	tests := []struct {
		roles []string
		allow bool
	}{
		{[]string{"PLATFORM_ADMIN"}, true},
		{[]string{"CARRIER_ADMIN"}, true},
		{[]string{"CARRIER_DISPATCHER"}, false},
		{[]string{"FINANCE_MANAGER"}, false},
		{[]string{"PROCUREMENT_MANAGER"}, false},
		{[]string{"UNKNOWN_ROLE"}, false},
	}
	for _, tt := range tests {
		if got := CanCreateFleet(tt.roles); got != tt.allow {
			t.Fatalf("CanCreateFleet(%v)=%v want %v", tt.roles, got, tt.allow)
		}
	}
}

func TestRBACAssignTable(t *testing.T) {
	tests := []struct {
		roles []string
		allow bool
	}{
		{[]string{"PLATFORM_ADMIN"}, true},
		{[]string{"CARRIER_ADMIN"}, true},
		{[]string{"CARRIER_DISPATCHER"}, true},
		{[]string{"FINANCE_MANAGER"}, false},
		{[]string{"PROCUREMENT_MANAGER"}, false},
		{[]string{"SHIPPER_LOGIST"}, false},
		{[]string{"UNKNOWN_ROLE"}, false},
	}
	for _, tt := range tests {
		if got := CanAssignFleet(tt.roles); got != tt.allow {
			t.Fatalf("CanAssignFleet(%v)=%v want %v", tt.roles, got, tt.allow)
		}
	}
}
