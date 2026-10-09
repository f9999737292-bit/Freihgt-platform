package companycontext

import "testing"

func TestExecutionActorFromMembership(t *testing.T) {
	t.Parallel()
	carrier, err := ExecutionActorFromMembership("CARRIER", []string{"CARRIER_DISPATCHER"})
	if err != nil || carrier != ActorCarrier {
		t.Fatalf("dispatcher actor=%s err=%v", carrier, err)
	}
	buyer, err := ExecutionActorFromMembership("SHIPPER", []string{"SHIPPER_ADMIN"})
	if err != nil || buyer != ActorBuyer {
		t.Fatalf("shipper actor=%s err=%v", buyer, err)
	}
	if _, err := ExecutionActorFromMembership("CARRIER", []string{"CARRIER_ACCOUNTANT"}); err == nil {
		t.Fatal("accountant must not read execution as carrier")
	}
	if _, err := ExecutionActorFromMembership("SHIPPER", []string{"PROCUREMENT_MANAGER"}); err == nil {
		t.Fatal("procurement manager is not an execution reader")
	}
	if _, err := ExecutionActorFromMembership("SHIPPER", []string{"SHIPPER"}); err == nil {
		t.Fatal("unlisted shipper role must not read execution")
	}
	platform, err := ExecutionActorFromMembership("SHIPPER", []string{"PLATFORM_ADMIN"})
	if err != nil || platform != ActorBuyer {
		t.Fatalf("platform admin on membership actor=%s err=%v", platform, err)
	}
}

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
		{name: "carrier accountant", companyType: "CARRIER", roles: []string{"CARRIER_ACCOUNTANT"}, want: false},
		{name: "platform admin on carrier company", companyType: "CARRIER", roles: []string{"PLATFORM_ADMIN"}, want: false},
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
