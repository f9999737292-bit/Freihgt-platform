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

func TestPolicyBuyerAnalyticsDeniesCarrier(t *testing.T) {
	if policyAllows(PolicyBuyerAnalytics, []string{"CARRIER_ADMIN"}, "CARRIER", false) {
		t.Fatal("carrier must not access buyer analytics routes")
	}
	if !policyAllows(PolicyBuyerAnalytics, []string{"PROCUREMENT_MANAGER"}, "BUYER", false) {
		t.Fatal("buyer manager must access buyer analytics routes")
	}
}
