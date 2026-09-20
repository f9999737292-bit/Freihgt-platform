//go:build integration

package erp

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	"github.com/freight-platform/rfx-service/internal/repository"
)

func TestE7P2INT182NoSecretsInAuditAndGetIsReadonly(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, principal, router := seedCreateCommitReady(t, env, "INT182")
	analysisID := previewCreateAnalysis(t, router, tenantID, companyID, principal.ID, "INT182")
	rec := postERPCreateCommit(t, router, tenantID, companyID, principal.ID, commitScopes(), analysisID, "key-182")
	if rec.Code != http.StatusCreated {
		t.Fatalf("commit status=%d body=%s", rec.Code, rec.Body.String())
	}
	out := decodeCreateCommit(t, rec.Result(), rec.Body.Bytes())
	eventID := uuid.MustParse(out["rfx_event_id"].(string))

	auditRepo := repository.NewAuditRepository(env.pool)
	events, err := auditRepo.ListByEntity(context.Background(), tenantID, "rfx_event", eventID, 20)
	if err != nil {
		t.Fatalf("list audit: %v", err)
	}
	found := false
	for _, ev := range events {
		if ev.Action != domain.ERPCreateCommitAuditAction && ev.Action != domain.ERPUpdateCommitAuditAction && ev.Action != domain.ERPCreateCommitReplayAudit {
			continue
		}
		found = true
		raw, _ := json.Marshal(ev.Metadata)
		blob := strings.ToLower(string(raw))
		for _, secret := range []string{"password", "secret", "token", "api_key", "authorization", "client_secret"} {
			if strings.Contains(blob, secret) {
				t.Fatalf("audit metadata must not contain %s", secret)
			}
		}
	}
	if !found {
		t.Fatal("expected existing ERP commit audit to inspect")
	}

	before := countTenantAuditEvents(t, env, tenantID)
	reads := []string{
		"/v1/integrations/erp/rfx-events/" + eventID.String(),
		externalLookupPath("SAP", "INT182", ""),
		"/v1/integrations/erp/analyses/" + analysisID.String(),
		"/v1/integrations/erp/capabilities",
	}
	for _, path := range reads {
		scopes := readScopes()
		if strings.Contains(path, "/analyses/") || strings.HasSuffix(path, "/capabilities") {
			scopes = statusReadScopes()
		}
		got := getERP(t, router, path, tenantID, companyID, principal.ID, scopes)
		if got.Code != http.StatusOK {
			t.Fatalf("%s status=%d body=%s", path, got.Code, got.Body.String())
		}
	}
	after := countTenantAuditEvents(t, env, tenantID)
	if after != before {
		t.Fatalf("GET must not write audit rows, before=%d after=%d", before, after)
	}
}
