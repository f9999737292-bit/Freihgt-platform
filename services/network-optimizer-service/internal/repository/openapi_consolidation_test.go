package repository

import (
	"os"
	"strings"
	"testing"
)

func TestNLO03B_082_083_088_OpenAPIConsolidation(t *testing.T) {
	raw, err := os.ReadFile("../../../../packages/openapi/network-optimizer-service.yaml")
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if strings.Count(text, "  /api/v1/network/consolidation/search:") != 1 {
		t.Fatal("NLO03B_082 consolidation path missing or duplicated")
	}
	if !strings.Contains(text, "ConsolidationSearchRequest") || !strings.Contains(text, "ConsolidationSearchResponse") {
		t.Fatal("NLO03B_082 schemas missing")
	}
	owner := strings.Split(text, "NetworkLoadOpportunity:")
	if len(owner) < 2 || !strings.Contains(strings.Split(owner[1], "NetworkMarketplaceLoad:")[0], "consolidation_allowed:") {
		t.Fatal("NLO03B_083 owner opt-in missing")
	}
	market := strings.Split(text, "NetworkMarketplaceLoad:")
	if len(market) < 2 || strings.Contains(strings.Split(market[1], "NetworkCapacity:")[0], "consolidation_allowed:") {
		t.Fatal("NLO03B_083 marketplace exposes opt-in")
	}
	if strings.Count(text, "ConsolidationSearchRequest:") != 1 || strings.Count(text, "      requestBody:") == 0 {
		t.Fatal("NLO03B_088 generation shape")
	}
}
