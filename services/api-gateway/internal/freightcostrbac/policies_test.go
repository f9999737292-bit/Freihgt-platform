package freightcostrbac

import "testing"

func TestPolicyAllowsReadRoles(t *testing.T) {
	if !policyAllows(PolicyRead, []string{"CARRIER_ADMIN"}, "CARRIER", false) {
		t.Fatal("carrier admin must read freight costs")
	}
	if policyAllows(PolicyRead, []string{"UNKNOWN_ROLE"}, "BUYER", false) {
		t.Fatal("unknown role must be denied")
	}
	if !policyAllows(PolicyRead, nil, "BUYER", true) {
		t.Fatal("platform admin must read")
	}
}
