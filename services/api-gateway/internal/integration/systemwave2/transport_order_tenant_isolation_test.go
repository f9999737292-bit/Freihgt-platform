//go:build integration

package systemwave2

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/api-gateway/internal/companycontext"
	"github.com/freight-platform/api-gateway/internal/config"
	gwmiddleware "github.com/freight-platform/api-gateway/internal/http/middleware"
	"github.com/freight-platform/api-gateway/internal/transportorderrbac"
)

func TestW2_TransportOrderCrossTenantGetUsesVerifiedTenant(t *testing.T) {
	tenantA := uuid.New().String()
	tenantB := uuid.New().String()
	userB := uuid.New().String()
	companyB := uuid.New().String()
	orderA := uuid.New().String()
	secret := "w2r5-to-isolation"

	identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/companies"):
			_, _ = w.Write([]byte(`{"items":[{"company_id":"` + companyB + `","company_type":"SHIPPER","roles":[{"code":"SHIPPER_ADMIN"}]}]}`))
		case strings.HasSuffix(r.URL.Path, "/roles"):
			_, _ = w.Write([]byte(`{"items":[]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer identityServer.Close()

	var gotTenant string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTenant = r.Header.Get("X-Tenant-ID")
		// Service-level tenant scope: order belongs to tenant A, caller is tenant B.
		w.WriteHeader(http.StatusNotFound)
	}))
	defer backend.Close()

	cfg := config.Config{
		AuthEnabled:         true,
		JWTSecret:           secret,
		ProxyTimeoutSeconds: 5,
		Services:            config.ServiceURLs{Identity: identityServer.URL},
	}
	guard := transportorderrbac.NewGuard(cfg, backend.Config.Handler)

	token := signToken(t, secret, userB, tenantB, "b@tenant-b.test")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/transport-orders/"+orderA, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Company-ID", companyB)
	req.Header.Set("X-Tenant-ID", tenantA)
	req.Header.Set("X-User-ID", uuid.NewString())
	req.Header.Set("X-Actor-Kind", companycontext.ActorBuyer)

	rec := httptest.NewRecorder()
	gwmiddleware.Auth(true, secret)(guard.WithPolicy(transportorderrbac.PolicyRead)).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant get status=%d body=%s", rec.Code, rec.Body.String())
	}
	if gotTenant != tenantB {
		t.Fatalf("downstream tenant=%q want verified tenantB %q", gotTenant, tenantB)
	}
}
