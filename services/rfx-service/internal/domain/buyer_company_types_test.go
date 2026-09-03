package domain

import "testing"

func TestBuyerCompanyTypesMatchRFxOwnerContract(t *testing.T) {
	t.Parallel()

	expected := []string{"SHIPPER", "FORWARDER", "LSP"}
	if len(buyerCompanyTypes) != len(expected) {
		t.Fatalf("buyerCompanyTypes count=%d want %d", len(buyerCompanyTypes), len(expected))
	}
	for _, companyType := range expected {
		if _, ok := buyerCompanyTypes[companyType]; !ok {
			t.Fatalf("missing buyer company type %q", companyType)
		}
	}
}
