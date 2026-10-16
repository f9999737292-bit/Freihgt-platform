package chainapi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestI2C_GW_contract_files(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..")
	spec, err := os.ReadFile(filepath.Join(root, "packages", "openapi", "network-optimizer-service.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(spec)
	for _, path := range []string{
		"/api/v1/network/chain-searches:",
		"/api/v1/network/chain-searches/{searchId}:",
		"/api/v1/network/chain-searches/{searchId}/candidates:",
		"Idempotency-Key",
		"Retry-After",
		"CHAIN_SEARCH_IN_PROGRESS",
		"PREDICTED_CAPACITY",
		"FEASIBLE",
		"UNFINISHED",
		"execution_supported",
	} {
		if !strings.Contains(text, path) {
			t.Fatalf("openapi missing %s", path)
		}
	}
	router, err := os.ReadFile(filepath.Join(root, "services", "api-gateway", "internal", "http", "router.go"))
	if err != nil {
		t.Fatal(err)
	}
	routes := string(router)
	for _, path := range []string{
		`r.Post("/api/v1/network/chain-searches", networkGuard.WithPolicy(bnorbac.PolicySearchChain))`,
		`r.Get("/api/v1/network/chain-searches/{searchId}", networkGuard.WithPolicy(bnorbac.PolicySearchChain))`,
		`r.Get("/api/v1/network/chain-searches/{searchId}/candidates", networkGuard.WithPolicy(bnorbac.PolicySearchChain))`,
	} {
		if !strings.Contains(routes, path) {
			t.Fatalf("gateway route missing %s", path)
		}
	}
	internal, err := os.ReadFile(filepath.Join(root, "services", "network-optimizer-service", "internal", "http", "router.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		`r.Post("/v1/network/chain-searches", h.PostChainSearch)`,
		`r.Get("/v1/network/chain-searches/{searchId}", h.GetChainSearch)`,
		`r.Get("/v1/network/chain-searches/{searchId}/candidates", h.GetChainCandidates)`,
	} {
		if !strings.Contains(string(internal), path) {
			t.Fatalf("internal route missing %s", path)
		}
	}
	ci, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ci), "TestI2A_|TestI2B_|TestI270_|TestI2C_") {
		t.Fatal("dedicated CI selector does not include TestI2C_")
	}
}
