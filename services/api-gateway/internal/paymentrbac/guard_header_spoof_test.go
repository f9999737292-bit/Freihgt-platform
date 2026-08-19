package paymentrbac

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/api-gateway/internal/companycontext"
	"github.com/freight-platform/api-gateway/internal/config"
)

func TestPaymentRouteHeaderSpoofStripped(t *testing.T) {
	t.Parallel()
	tenantA := uuid.New().String()
	userA := uuid.New().String()
	companyA := uuid.New().String()
	tenantB := uuid.New().String()
	userB := uuid.New().String()
	companyB := uuid.New().String()

	identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/auth/me"):
			_ = json.NewEncoder(w).Encode(map[string]any{"roles": []string{"FINANCE_MANAGER"}})
		case strings.Contains(r.URL.Path, "/companies"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items": []map[string]any{{
					"company_id": companyA, "company_type": "SHIPPER",
					"roles": []map[string]any{{"code": "FINANCE_MANAGER"}},
				}},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer identityServer.Close()

	var gotTenant, gotUser, gotCompany, gotActor string
	guard := NewGuard(config.Config{
		AuthEnabled:         true,
		ProxyTimeoutSeconds: 5,
		Services:            config.ServiceURLs{Identity: identityServer.URL},
	}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTenant = r.Header.Get("X-Tenant-ID")
		gotUser = r.Header.Get("X-User-ID")
		gotCompany = r.Header.Get(companycontext.HeaderCompanyID)
		gotActor = r.Header.Get(companycontext.HeaderActorKind)
		w.WriteHeader(http.StatusOK)
	}))

	token := signTestToken(t, "secret", userA, tenantA)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/payments?company_id="+companyA, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Tenant-ID", tenantB)
	req.Header.Set("X-User-ID", userB)
	req.Header.Set("X-Company-ID", companyB)
	req.Header.Set("X-Actor-Kind", companycontext.ActorCarrier)
	rec := serveThroughAuth(t, guard.WithPolicy(PolicyRead), req, "secret")
	if rec.Code != http.StatusOK {
		t.Fatalf("PAYMENT_ROUTE_HEADER_SPOOF_TEST expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if gotTenant != tenantA {
		t.Fatalf("tenant header=%q want authenticated tenant %q", gotTenant, tenantA)
	}
	if gotUser != userA {
		t.Fatalf("user header=%q want authenticated user %q", gotUser, userA)
	}
	if gotCompany != companyA {
		t.Fatalf("company header=%q want verified company %q", gotCompany, companyA)
	}
	if gotActor != companycontext.ActorBuyer {
		t.Fatalf("actor header=%q want verified BUYER", gotActor)
	}
}
