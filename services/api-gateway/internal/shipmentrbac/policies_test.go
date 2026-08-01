package shipmentrbac

import "testing"

func TestCanCreateShipment(t *testing.T) {
	t.Parallel()
	tests := []struct {
		roles []string
		want  bool
	}{
		{[]string{"PLATFORM_ADMIN"}, true},
		{[]string{"SHIPPER_ADMIN"}, true},
		{[]string{"SHIPPER_LOGIST"}, true},
		{[]string{"FORWARDER_MANAGER"}, true},
		{[]string{"CARRIER_ADMIN"}, false},
		{[]string{"CARRIER_DISPATCHER"}, false},
		{[]string{"FINANCE_MANAGER"}, false},
		{[]string{"PROCUREMENT_MANAGER"}, false},
		{[]string{"UNKNOWN_ROLE"}, false},
		{nil, false},
	}
	for _, tt := range tests {
		if got := CanCreateShipment(tt.roles); got != tt.want {
			t.Fatalf("roles=%v got=%v want=%v", tt.roles, got, tt.want)
		}
	}
}

func TestCanAcceptShipment(t *testing.T) {
	t.Parallel()
	tests := []struct {
		roles []string
		want  bool
	}{
		{[]string{"PLATFORM_ADMIN"}, true},
		{[]string{"CARRIER_ADMIN"}, true},
		{[]string{"CARRIER_DISPATCHER"}, true},
		{[]string{"FORWARDER_MANAGER"}, false},
		{[]string{"SHIPPER_ADMIN"}, false},
		{[]string{"SHIPPER_LOGIST"}, false},
		{[]string{"FINANCE_MANAGER"}, false},
		{[]string{"PROCUREMENT_MANAGER"}, false},
		{[]string{"UNKNOWN_ROLE"}, false},
		{nil, false},
	}
	for _, tt := range tests {
		if got := CanAcceptShipment(tt.roles); got != tt.want {
			t.Fatalf("roles=%v got=%v want=%v", tt.roles, got, tt.want)
		}
	}
}

func TestCanUpdateShipmentStatus(t *testing.T) {
	t.Parallel()
	tests := []struct {
		roles []string
		want  bool
	}{
		{[]string{"PLATFORM_ADMIN"}, true},
		{[]string{"CARRIER_ADMIN"}, true},
		{[]string{"CARRIER_DISPATCHER"}, true},
		{[]string{"SHIPPER_ADMIN"}, false},
		{[]string{"SHIPPER_LOGIST"}, false},
		{[]string{"FINANCE_MANAGER"}, false},
		{[]string{"PROCUREMENT_MANAGER"}, false},
		{[]string{"UNKNOWN_ROLE"}, false},
		{nil, false},
	}
	for _, tt := range tests {
		if got := CanUpdateShipmentStatus(tt.roles); got != tt.want {
			t.Fatalf("roles=%v got=%v want=%v", tt.roles, got, tt.want)
		}
	}
}

func TestCanCancelShipment(t *testing.T) {
	t.Parallel()
	tests := []struct {
		roles []string
		want  bool
	}{
		{[]string{"PLATFORM_ADMIN"}, true},
		{[]string{"SHIPPER_ADMIN"}, true},
		{[]string{"SHIPPER_LOGIST"}, true},
		{[]string{"FORWARDER_MANAGER"}, true},
		{[]string{"CARRIER_ADMIN"}, false},
		{[]string{"CARRIER_DISPATCHER"}, false},
		{[]string{"FINANCE_MANAGER"}, false},
		{[]string{"PROCUREMENT_MANAGER"}, false},
		{[]string{"UNKNOWN_ROLE"}, false},
		{nil, false},
	}
	for _, tt := range tests {
		if got := CanCancelShipment(tt.roles); got != tt.want {
			t.Fatalf("roles=%v got=%v want=%v", tt.roles, got, tt.want)
		}
	}
}

func TestMultiRoleSemantics(t *testing.T) {
	t.Parallel()
	financeCarrier := []string{"FINANCE_MANAGER", "CARRIER_DISPATCHER"}
	if !CanUpdateShipmentStatus(financeCarrier) {
		t.Fatal("UpdateStatus should allow when any role is permitted")
	}
	if !CanAcceptShipment(financeCarrier) {
		t.Fatal("Accept should allow when carrier role is present")
	}
	if CanCreateShipment(financeCarrier) {
		t.Fatal("Create should deny finance+carrier without create role")
	}

	financeProcurement := []string{"FINANCE_MANAGER", "PROCUREMENT_MANAGER"}
	if CanCreateShipment(financeProcurement) || CanAcceptShipment(financeProcurement) ||
		CanUpdateShipmentStatus(financeProcurement) || CanCancelShipment(financeProcurement) {
		t.Fatal("finance+procurement must be denied for all shipment mutations")
	}
}
