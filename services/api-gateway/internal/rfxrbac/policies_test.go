package rfxrbac

import "testing"

func TestBuyerManageRoles(t *testing.T) {
	t.Parallel()
	for _, role := range []string{"PLATFORM_ADMIN", "PROCUREMENT_MANAGER", "SHIPPER_ADMIN", "SHIPPER_LOGIST", "FORWARDER_MANAGER"} {
		if !CanBuyerManage([]string{role}) {
			t.Fatalf("expected buyer manage for %s", role)
		}
	}
	if CanBuyerManage([]string{"CARRIER_ADMIN"}) {
		t.Fatal("carrier must not manage rfx")
	}
}

func TestCarrierRespondRoles(t *testing.T) {
	t.Parallel()
	for _, role := range []string{"CARRIER_ADMIN", "CARRIER_DISPATCHER"} {
		if !CanCarrierRespond([]string{role}) {
			t.Fatalf("expected carrier respond for %s", role)
		}
	}
	if CanCarrierRespond([]string{"SHIPPER_ADMIN"}) {
		t.Fatal("buyer must not respond as carrier")
	}
}

func TestCombinedReadAllowsBuyerAndCarrier(t *testing.T) {
	t.Parallel()
	if !CanReadFreightRequests([]string{"SHIPPER_ADMIN"}) {
		t.Fatal("buyer must read freight requests")
	}
	if !CanReadFreightRequests([]string{"CARRIER_DISPATCHER"}) {
		t.Fatal("carrier must read freight requests")
	}
	if CanReadFreightRequests([]string{"FINANCE_MANAGER"}) {
		t.Fatal("finance must not read freight requests")
	}
}
