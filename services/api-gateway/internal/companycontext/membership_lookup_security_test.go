package companycontext

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/freight-platform/api-gateway/internal/routeauth"
)

func TestListUserCompaniesRequiresCallerBearer(t *testing.T) {
	const (
		userID   = "33333333-3333-3333-3333-333333333333"
		tenantID = "11111111-1111-1111-1111-111111111111"
		bearer   = "Bearer caller-access-token"
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != bearer {
			t.Fatalf("authorization=%q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("X-Internal-Service-Token") != "" || r.Header.Get("X-Internal-Service-Name") != "" {
			t.Fatal("membership lookup used a generic internal credential")
		}
		if !strings.Contains(r.URL.Path, userID) {
			t.Fatalf("path=%s", r.URL.Path)
		}
		if r.URL.Query().Get("tenant_id") != tenantID {
			t.Fatalf("query tenant=%s", r.URL.Query().Get("tenant_id"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{{
				"company_id":   "77777777-7777-7777-7777-777777777777",
				"company_type": "SHIPPER",
				"roles":        []map[string]string{{"code": "SHIPPER_ADMIN"}},
			}},
		})
	}))
	defer server.Close()

	client := NewIdentityClient(server.Client(), server.URL)
	companies, err := client.ListUserCompanies(context.Background(), routeauth.RequestContext{
		TenantID:  tenantID,
		UserID:    userID,
		AuthToken: bearer,
	}, userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(companies) != 1 || companies[0].CompanyType != "SHIPPER" {
		t.Fatalf("companies=%+v", companies)
	}
}
