//go:build integration

package erp

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	"github.com/freight-platform/rfx-service/internal/erpjson"
)

func TestE7P2INT134CapabilitiesWithoutAuth401(t *testing.T) {
	env := setupTestEnv(t)
	_, _, _, router := seedCreateCommitReady(t, env, "INT134")
	rec := getERP(t, router, "/v1/integrations/erp/capabilities", uuid.Nil, uuid.Nil, uuid.Nil, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestE7P2INT189CapabilitiesSchemaAndDeferred(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, principal, router := seedCreateCommitReady(t, env, "INT189")
	rec := getERP(t, router, "/v1/integrations/erp/capabilities", tenantID, companyID, principal.ID, statusReadScopes())
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	out := decodeERPJSON(t, rec.Result(), rec.Body.Bytes())
	if out["schema_version"] != domain.SchemaVersionERPJSONV1 {
		t.Fatalf("schema_version=%v", out["schema_version"])
	}
	limits, _ := out["limits"].(map[string]any)
	if limits["max_body_bytes"] != float64(erpjson.MaxBodyBytes) || limits["max_json_depth"] != float64(erpjson.MaxJSONDepth) {
		t.Fatalf("limits=%v", limits)
	}
	deferred, _ := out["deferred_fields"].([]any)
	found := map[string]bool{}
	for _, item := range deferred {
		found[item.(string)] = true
	}
	for _, want := range []string{"lanes", "cargo", "invited_carriers"} {
		if !found[want] {
			t.Fatalf("deferred_fields missing %s: %v", want, deferred)
		}
	}
	types, _ := out["supported_mapping_types"].([]any)
	if len(types) == 0 {
		t.Fatal("supported_mapping_types required")
	}
}

func TestE6CapabilitiesMissingScope403(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, principal, router := seedCreateCommitReady(t, env, "E6CAP")
	rec := getERP(t, router, "/v1/integrations/erp/capabilities", tenantID, companyID, principal.ID, readScopes())
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
