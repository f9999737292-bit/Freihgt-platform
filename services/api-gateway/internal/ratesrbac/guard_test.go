package ratesrbac

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/api-gateway/internal/companycontext"
	"github.com/freight-platform/api-gateway/internal/config"
	gwmiddleware "github.com/freight-platform/api-gateway/internal/http/middleware"
)

func serveThroughAuth(t *testing.T, handler http.Handler, req *http.Request, secret string) *httptest.ResponseRecorder {
	t.Helper()
	chain := gwmiddleware.Auth(true, secret)(handler)
	rec := httptest.NewRecorder()
	chain.ServeHTTP(rec, req)
	return rec
}

func signTestToken(t *testing.T, secret, userID, tenantID string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"tenant_id": tenantID,
		"email":     "user@example.com",
		"sub":       userID,
		"exp":       time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

func newIdentityServer(t *testing.T, memberships []map[string]any, tenantRoles []string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/companies"):
			_ = json.NewEncoder(w).Encode(map[string]any{"items": memberships})
		case strings.HasSuffix(r.URL.Path, "/roles"):
			items := make([]map[string]any, 0, len(tenantRoles))
			for _, role := range tenantRoles {
				items = append(items, map[string]any{"code": role})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestRatesRBACCrossCompanyRoleBleedDenied(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New().String()
	userID := uuid.New().String()
	companyA := uuid.New().String()
	companyB := uuid.New().String()

	identityServer := newIdentityServer(t, []map[string]any{
		{"company_id": companyA, "company_type": "SHIPPER", "roles": []map[string]any{{"code": "SHIPPER_ADMIN"}}},
		{"company_id": companyB, "company_type": "SHIPPER", "roles": []map[string]any{{"code": "SHIPPER_LOGIST"}}},
	}, nil)
	defer identityServer.Close()

	downstreamCalled := false
	handler := NewGuard(config.Config{
		AuthEnabled:         true,
		ProxyTimeoutSeconds: 5,
		Services:            config.ServiceURLs{Identity: identityServer.URL},
	}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downstreamCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	token := signTestToken(t, "secret", userID, tenantID)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/transport-contracts/"+uuid.New().String(), strings.NewReader(`{"description":"x"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Company-ID", companyB)
	req.Header.Set("Content-Type", "application/json")
	rec := serveThroughAuth(t, handler.WithPolicy(PolicyEditContract), req, "secret")

	if rec.Code != http.StatusForbidden {
		t.Fatalf("CROSS_COMPANY_ROLE_BLEED expected 403, got %d body=%s", rec.Code, rec.Body.String())
	}
	if downstreamCalled {
		t.Fatal("downstream must not be called on RBAC deny")
	}
}

func TestRatesRBACShipperLogistMutateDenied(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New().String()
	userID := uuid.New().String()
	companyID := uuid.New().String()

	identityServer := newIdentityServer(t, []map[string]any{
		{"company_id": companyID, "company_type": "SHIPPER", "roles": []map[string]any{{"code": "SHIPPER_LOGIST"}}},
	}, nil)
	defer identityServer.Close()

	handler := NewGuard(config.Config{
		AuthEnabled:         true,
		ProxyTimeoutSeconds: 5,
		Services:            config.ServiceURLs{Identity: identityServer.URL},
	}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("downstream must not be called")
	}))

	token := signTestToken(t, "secret", userID, tenantID)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/transport-contracts", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Company-ID", companyID)
	rec := serveThroughAuth(t, handler.WithPolicy(PolicyCreateContract), req, "secret")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("SHIPPER_LOGIST mutate expected 403, got %d", rec.Code)
	}
}

func TestRatesRBACHeaderSpoofStripped(t *testing.T) {
	t.Parallel()
	tenantA := uuid.New().String()
	userA := uuid.New().String()
	companyA := uuid.New().String()
	tenantB := uuid.New().String()

	identityServer := newIdentityServer(t, []map[string]any{
		{"company_id": companyA, "company_type": "SHIPPER", "roles": []map[string]any{{"code": "PROCUREMENT_MANAGER"}}},
	}, nil)
	defer identityServer.Close()

	var gotTenant, gotUser, gotCompany, gotActor string
	handler := NewGuard(config.Config{
		AuthEnabled:         true,
		ProxyTimeoutSeconds: 5,
		Services:            config.ServiceURLs{Identity: identityServer.URL},
	}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTenant = r.Header.Get("X-Tenant-ID")
		gotUser = r.Header.Get("X-User-ID")
		gotCompany = r.Header.Get(companycontext.HeaderCompanyID)
		gotActor = r.Header.Get(companycontext.HeaderActorKind)
		if r.Header.Get("X-Internal-Service-Token") != "" {
			t.Fatal("internal token must not be forwarded from client")
		}
		w.WriteHeader(http.StatusOK)
	}))

	token := signTestToken(t, "secret", userA, tenantA)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/transport-contracts", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Tenant-ID", tenantB)
	req.Header.Set("X-User-ID", uuid.New().String())
	req.Header.Set("X-Company-ID", companyA)
	req.Header.Set("X-Actor-Kind", companycontext.ActorCarrier)
	req.Header.Set("X-Internal-Service-Token", "spoof")
	rec := serveThroughAuth(t, handler.WithPolicy(PolicyRead), req, "secret")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if gotTenant != tenantA || gotUser != userA {
		t.Fatalf("identity spoof not replaced: tenant=%q user=%q", gotTenant, gotUser)
	}
	if gotCompany != companyA || gotActor != companycontext.ActorBuyer {
		t.Fatalf("company/actor=%q/%q want verified %q/%q", gotCompany, gotActor, companyA, companycontext.ActorBuyer)
	}
}

func TestRatesRBACMissingCompanyID(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New().String()
	userID := uuid.New().String()
	identityServer := newIdentityServer(t, nil, nil)
	defer identityServer.Close()

	handler := NewGuard(config.Config{
		AuthEnabled:         true,
		ProxyTimeoutSeconds: 5,
		Services:            config.ServiceURLs{Identity: identityServer.URL},
	}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("downstream must not be called")
	}))

	token := signTestToken(t, "secret", userID, tenantID)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/transport-contracts", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := serveThroughAuth(t, handler.WithPolicy(PolicyRead), req, "secret")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing X-Company-ID expected 400, got %d", rec.Code)
	}
}
