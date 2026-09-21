//go:build integration

package studio

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestIdentityStubServesUserRoles(t *testing.T) {
	stub := startBrowserIdentityStub(t, map[string][]string{"user-1": {"PROCUREMENT_MANAGER"}})
	resp, err := http.Get(stub.URL() + "/v1/users/user-1/roles?tenant_id=tenant-1")
	if err != nil {
		t.Fatalf("GET /roles: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /roles -> %d", resp.StatusCode)
	}
	var payload struct {
		Items []struct {
			Code string `json:"code"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode roles: %v", err)
	}
	if len(payload.Items) != 1 || payload.Items[0].Code != "PROCUREMENT_MANAGER" {
		t.Fatalf("roles payload %+v", payload)
	}
}
