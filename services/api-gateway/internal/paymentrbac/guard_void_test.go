package paymentrbac

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPaymentVoidRBACMatrix(t *testing.T) {
	t.Parallel()
	tenantID := "11111111-1111-1111-1111-111111111111"
	companyID := "22222222-2222-2222-2222-222222222222"
	path := "/api/v1/payments/00000000-0000-0000-0000-000000000001/void?company_id=" + companyID

	tests := []struct {
		name       string
		roles      []string
		wantStatus int
	}{
		{"FINANCE_MANAGER_VOID=ALLOW", []string{"FINANCE_MANAGER"}, http.StatusOK},
		{"SHIPPER_ADMIN_VOID=ALLOW", []string{"SHIPPER_ADMIN"}, http.StatusOK},
		{"CARRIER_ACCOUNTANT_VOID=ALLOW", []string{"CARRIER_ACCOUNTANT"}, http.StatusOK},
		{"SHIPPER_LOGIST_VOID=DENY", []string{"SHIPPER_LOGIST"}, http.StatusForbidden},
		{"FORWARDER_MANAGER_VOID=DENY", []string{"FORWARDER_MANAGER"}, http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.Path == "/v1/auth/me":
					_ = json.NewEncoder(w).Encode(map[string]any{"roles": tt.roles})
				case strings.HasPrefix(r.URL.Path, "/v1/users/") && strings.Contains(r.URL.Path, "/companies"):
					_ = json.NewEncoder(w).Encode(map[string]any{
						"items": []map[string]any{{
							"company_id": companyID, "company_type": "SHIPPER",
							"roles": []map[string]any{{"code": tt.roles[0]}},
						}},
					})
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer identityServer.Close()

			guard := newTestGuard(t, identityServer.URL, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
			token := signTestToken(t, "secret", "user-1", tenantID)
			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"reason":"test"}`))
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Content-Type", "application/json")
			rec := serveThroughAuth(t, guard.WithPolicy(PolicyVoid), req, "secret")
			if rec.Code != tt.wantStatus {
				t.Fatalf("expected %d got %d body=%s", tt.wantStatus, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestPaymentVoidGatewayHeaderSpoofStripped(t *testing.T) {
	t.Parallel()
	tenantID := "11111111-1111-1111-1111-111111111111"
	companyID := "22222222-2222-2222-2222-222222222222"
	spoofTenant := "99999999-9999-9999-9999-999999999999"
	identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1/auth/me":
			_ = json.NewEncoder(w).Encode(map[string]any{"roles": []string{"FINANCE_MANAGER"}})
		case strings.HasPrefix(r.URL.Path, "/v1/users/") && strings.Contains(r.URL.Path, "/companies"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items": []map[string]any{{
					"company_id": companyID, "company_type": "SHIPPER",
					"roles": []map[string]any{{"code": "FINANCE_MANAGER"}},
				}},
			})
		}
	}))
	defer identityServer.Close()

	var seenTenant string
	guard := newTestGuard(t, identityServer.URL, func(w http.ResponseWriter, r *http.Request) {
		seenTenant = r.Header.Get("X-Tenant-ID")
		w.WriteHeader(http.StatusOK)
	})
	token := signTestToken(t, "secret", "user-1", tenantID)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/00000000-0000-0000-0000-000000000001/void?company_id="+companyID, strings.NewReader(`{"reason":"test"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", spoofTenant)
	rec := serveThroughAuth(t, guard.WithPolicy(PolicyVoid), req, "secret")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", rec.Code)
	}
	if seenTenant != tenantID {
		t.Fatalf("HEADER_SPOOF_VOID=DENY_OR_STRIPPED expected trusted tenant, got %s", seenTenant)
	}
}
