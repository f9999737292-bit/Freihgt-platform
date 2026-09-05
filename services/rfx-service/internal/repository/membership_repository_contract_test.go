package repository

import (
	"os"
	"strings"
	"testing"
)

func TestListUserBuyerCompanyIDsFiltersBuyerCompanyTypes(t *testing.T) {
	t.Parallel()

	source, err := os.ReadFile("membership_repository.go")
	if err != nil {
		t.Fatalf("read membership_repository.go: %v", err)
	}
	text := string(source)
	if !strings.Contains(text, "ListUserBuyerCompanyIDs") {
		t.Fatal("expected ListUserBuyerCompanyIDs implementation")
	}
	for _, companyType := range []string{"SHIPPER", "FORWARDER", "LSP"} {
		if !strings.Contains(text, "'"+companyType+"'") {
			t.Fatalf("expected buyer company type %q in repository filter", companyType)
		}
	}
	if strings.Contains(text, "ListUserBuyerCompanyIDs") && !strings.Contains(text, "company_type IN ('SHIPPER', 'FORWARDER', 'LSP')") {
		t.Fatal("expected SQL buyer company type filter")
	}
}
