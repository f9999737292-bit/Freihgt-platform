package transportorderrbac

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
		"email":     "buyer@test.local",
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

func TestTransportOrderCreateMissingJWTDenied(t *testing.T) {
	t.Parallel()
	handler := NewGuard(config.Config{AuthEnabled: true}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("downstream must not be called")
	}))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/transport-orders", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.WithPolicy(PolicyCreate).ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing JWT expected 401, got %d", rec.Code)
	}
}

func TestTransportOrderCreateMissingCompanyDenied(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New().String()
	userID := uuid.New().String()
	identityServer := newIdentityServer(t, nil, nil)
	defer identityServer.Close()

	handler := NewGuard(config.Config{
		AuthEnabled: true, ProxyTimeoutSeconds: 5,
		Services: config.ServiceURLs{Identity: identityServer.URL},
	}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("downstream must not be called")
	}))

	token := signTestToken(t, "secret", userID, tenantID)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/transport-orders", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := serveThroughAuth(t, handler.WithPolicy(PolicyCreate), req, "secret")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing X-Company-ID expected 400, got %d", rec.Code)
	}
}

func TestTransportOrderCompanyIsolationDenied(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New().String()
	userID := uuid.New().String()
	companyA := uuid.New().String()
	companyB := uuid.New().String()

	identityServer := newIdentityServer(t, []map[string]any{
		{"company_id": companyA, "company_type": "SHIPPER", "roles": []map[string]any{{"code": "SHIPPER_ADMIN"}}},
	}, nil)
	defer identityServer.Close()

	handler := NewGuard(config.Config{
		AuthEnabled: true, ProxyTimeoutSeconds: 5,
		Services: config.ServiceURLs{Identity: identityServer.URL},
	}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("downstream must not be called")
	}))

	token := signTestToken(t, "secret", userID, tenantID)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/transport-orders", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Company-ID", companyB)
	req.Header.Set("Content-Type", "application/json")
	rec := serveThroughAuth(t, handler.WithPolicy(PolicyCreate), req, "secret")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("foreign company expected 403, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestTransportOrderHeaderSpoofStrippedAndVerified(t *testing.T) {
	t.Parallel()
	tenantA := uuid.New().String()
	userA := uuid.New().String()
	companyA := uuid.New().String()
	tenantB := uuid.New().String()

	identityServer := newIdentityServer(t, []map[string]any{
		{"company_id": companyA, "company_type": "SHIPPER", "roles": []map[string]any{{"code": "SHIPPER_ADMIN"}}},
	}, nil)
	defer identityServer.Close()

	var gotTenant, gotUser, gotCompany, gotActor string
	handler := NewGuard(config.Config{
		AuthEnabled: true, ProxyTimeoutSeconds: 5,
		Services: config.ServiceURLs{Identity: identityServer.URL},
	}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTenant = r.Header.Get("X-Tenant-ID")
		gotUser = r.Header.Get("X-User-ID")
		gotCompany = r.Header.Get(companycontext.HeaderCompanyID)
		gotActor = r.Header.Get(companycontext.HeaderActorKind)
		w.WriteHeader(http.StatusCreated)
	}))

	token := signTestToken(t, "secret", userA, tenantA)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/transport-orders", strings.NewReader(`{"order_number":"TO-1"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "idem-1")
	req.Header.Set("X-Tenant-ID", tenantB)
	req.Header.Set("X-User-ID", uuid.New().String())
	req.Header.Set("X-Company-ID", companyA)
	req.Header.Set("X-Actor-Kind", companycontext.ActorCarrier)
	rec := serveThroughAuth(t, handler.WithPolicy(PolicyCreate), req, "secret")
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	if gotTenant != tenantA || gotUser != userA {
		t.Fatalf("identity spoof not replaced: tenant=%q user=%q", gotTenant, gotUser)
	}
	if gotCompany != companyA || gotActor != companycontext.ActorBuyer {
		t.Fatalf("company/actor=%q/%q want verified %q/%q", gotCompany, gotActor, companyA, companycontext.ActorBuyer)
	}
}

func TestTransportOrderActorSpoofCannotElevate(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New().String()
	userID := uuid.New().String()
	companyID := uuid.New().String()

	identityServer := newIdentityServer(t, []map[string]any{
		{"company_id": companyID, "company_type": "CARRIER", "roles": []map[string]any{{"code": "CARRIER_ADMIN"}}},
	}, nil)
	defer identityServer.Close()

	handler := NewGuard(config.Config{
		AuthEnabled: true, ProxyTimeoutSeconds: 5,
		Services: config.ServiceURLs{Identity: identityServer.URL},
	}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("carrier actor must not create transport order")
	}))

	token := signTestToken(t, "secret", userID, tenantID)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/transport-orders", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Company-ID", companyID)
	req.Header.Set("X-Actor-Kind", companycontext.ActorBuyer)
	req.Header.Set("Content-Type", "application/json")
	rec := serveThroughAuth(t, handler.WithPolicy(PolicyCreate), req, "secret")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("carrier create expected 403, got %d", rec.Code)
	}
}

func TestTransportOrderTenantIsolationDenied(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New().String()
	userID := uuid.New().String()
	foreignCompany := uuid.New().String()

	identityServer := newIdentityServer(t, nil, nil)
	defer identityServer.Close()

	handler := NewGuard(config.Config{
		AuthEnabled: true, ProxyTimeoutSeconds: 5,
		Services: config.ServiceURLs{Identity: identityServer.URL},
	}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("downstream must not be called without membership")
	}))

	token := signTestToken(t, "secret", userID, tenantID)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/transport-orders", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Company-ID", foreignCompany)
	req.Header.Set("Content-Type", "application/json")
	rec := serveThroughAuth(t, handler.WithPolicy(PolicyCreate), req, "secret")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("missing membership expected 403, got %d", rec.Code)
	}
}

func TestTransportOrderCreateShipperAdminPass(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New().String()
	userID := uuid.New().String()
	companyID := uuid.New().String()

	identityServer := newIdentityServer(t, []map[string]any{
		{"company_id": companyID, "company_type": "SHIPPER", "roles": []map[string]any{{"code": "SHIPPER_ADMIN"}}},
	}, nil)
	defer identityServer.Close()

	handler := NewGuard(config.Config{
		AuthEnabled: true, ProxyTimeoutSeconds: 5,
		Services: config.ServiceURLs{Identity: identityServer.URL},
	}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(companycontext.HeaderCompanyID) != companyID {
			t.Fatalf("missing verified company header")
		}
		if r.Header.Get(companycontext.HeaderActorKind) != companycontext.ActorBuyer {
			t.Fatalf("missing verified actor kind")
		}
		w.WriteHeader(http.StatusCreated)
	}))

	token := signTestToken(t, "secret", userID, tenantID)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/transport-orders", strings.NewReader(`{"order_number":"TO-W2"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Company-ID", companyID)
	req.Header.Set("Idempotency-Key", "wave2-idem")
	req.Header.Set("Content-Type", "application/json")
	rec := serveThroughAuth(t, handler.WithPolicy(PolicyCreate), req, "secret")
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}
}
