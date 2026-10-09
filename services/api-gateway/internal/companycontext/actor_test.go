package companycontext

import "testing"

func TestMembershipAllowsCarrierRead(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		companyType string
		roles       []string
		want        bool
	}{
		{name: "carrier dispatcher", companyType: "CARRIER", roles: []string{"CARRIER_DISPATCHER"}, want: true},
		{name: "carrier admin", companyType: "carrier", roles: []string{"CARRIER_ADMIN"}, want: true},
		{name: "driver on carrier company", companyType: "CARRIER", roles: []string{"DRIVER"}, want: false},
		{name: "shipper admin", companyType: "SHIPPER", roles: []string{"SHIPPER_ADMIN"}, want: false},
		{name: "shipper role label", companyType: "SHIPPER", roles: []string{"SHIPPER"}, want: false},
		{name: "carrier role on shipper company", companyType: "SHIPPER", roles: []string{"CARRIER_DISPATCHER"}, want: false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := MembershipAllowsCarrierRead(tt.companyType, tt.roles); got != tt.want {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}
