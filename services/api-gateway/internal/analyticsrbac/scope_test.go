package analyticsrbac

import (
	"errors"
	"strings"
	"testing"
)

func TestResolvePolicyA(t *testing.T) {
	shipper := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	carrier := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	other := "cccccccc-cccc-cccc-cccc-cccccccccccc"

	admin, err := Resolve([]string{"platform_admin"}, []Membership{{
		CompanyID:   carrier,
		CompanyType: "CARRIER",
		RoleCodes:   []string{"CARRIER_DISPATCHER"},
	}})
	if err != nil || admin.Kind != ScopeTenant || admin.CompanyID != "" {
		t.Fatalf("platform admin scope=%+v err=%v", admin, err)
	}

	own, err := Resolve(nil, []Membership{{
		CompanyID:   strings.ToUpper(carrier),
		CompanyType: "carrier",
		RoleCodes:   []string{"carrier_dispatcher"},
	}, {
		CompanyID:   carrier,
		CompanyType: "CARRIER",
		RoleCodes:   []string{"CARRIER_ACCOUNTANT"},
	}})
	if err != nil || own.Kind != ScopeCarrier || own.CompanyID != carrier {
		t.Fatalf("carrier scope=%+v err=%v", own, err)
	}

	shipperScope, err := Resolve([]string{"SHIPPER_ADMIN"}, []Membership{{
		CompanyID:   shipper,
		CompanyType: "SHIPPER",
		RoleCodes:   []string{"SHIPPER_LOGIST"},
	}})
	if err != nil || shipperScope.Kind != ScopeShipper || shipperScope.CompanyID != shipper {
		t.Fatalf("shipper scope=%+v err=%v", shipperScope, err)
	}

	denied := []struct {
		name        string
		roles       []string
		memberships []Membership
	}{
		{name: "no membership", roles: []string{"PROCUREMENT_MANAGER", "FORWARDER_MANAGER", "FINANCE_MANAGER", "DRIVER"}},
		{name: "driver on carrier", memberships: []Membership{{CompanyID: carrier, CompanyType: "CARRIER", RoleCodes: []string{"DRIVER"}}}},
		{name: "forwarder company", memberships: []Membership{{CompanyID: shipper, CompanyType: "FORWARDER", RoleCodes: []string{"FORWARDER_MANAGER"}}}},
		{name: "two carriers", memberships: []Membership{
			{CompanyID: carrier, CompanyType: "CARRIER", RoleCodes: []string{"CARRIER_ADMIN"}},
			{CompanyID: other, CompanyType: "CARRIER", RoleCodes: []string{"CARRIER_ADMIN"}},
		}},
		{name: "shipper and carrier", memberships: []Membership{
			{CompanyID: shipper, CompanyType: "SHIPPER", RoleCodes: []string{"SHIPPER_ADMIN"}},
			{CompanyID: carrier, CompanyType: "CARRIER", RoleCodes: []string{"CARRIER_ADMIN"}},
		}},
		{name: "conflicting type", memberships: []Membership{
			{CompanyID: shipper, CompanyType: "SHIPPER", RoleCodes: []string{"SHIPPER_ADMIN"}},
			{CompanyID: shipper, CompanyType: "CARRIER", RoleCodes: []string{"CARRIER_ADMIN"}},
		}},
		{name: "malformed only", memberships: []Membership{{CompanyID: "not-a-company", CompanyType: "CARRIER", RoleCodes: []string{"CARRIER_ADMIN"}}}},
		{name: "company platform admin", memberships: []Membership{{CompanyID: carrier, CompanyType: "CARRIER", RoleCodes: []string{"PLATFORM_ADMIN"}}}},
	}
	for _, tt := range denied {
		t.Run(tt.name, func(t *testing.T) {
			scope, err := Resolve(tt.roles, tt.memberships)
			if !errors.Is(err, ErrScopeUnavailable) || scope != (Scope{}) {
				t.Fatalf("scope=%+v err=%v", scope, err)
			}
			if strings.Contains(err.Error(), shipper) || strings.Contains(err.Error(), carrier) || strings.Contains(err.Error(), other) {
				t.Fatalf("scope error disclosed a company id: %v", err)
			}
		})
	}
}
