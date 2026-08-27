package companyrbac

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

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
		"email":     "driver@test.local",
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

func newIdentityStub(t *testing.T, tenantID, userID, ownCompany, foreignCompany string, roles []string, global []string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/companies"):
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []map[string]any{{
				"company_id":   ownCompany,
				"company_type": "CARRIER",
				"roles":        rolesPayload(roles),
			}}})
		case strings.Contains(r.URL.Path, "/roles"):
			items := make([]map[string]any, 0, len(global))
			for _, code := range global {
				items = append(items, map[string]any{"code": code})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
		_, _, _, _ = tenantID, userID, ownCompany, foreignCompany
	}))
}

func rolesPayload(codes []string) []map[string]any {
	out := make([]map[string]any, 0, len(codes))
	for _, code := range codes {
		out = append(out, map[string]any{"code": code})
	}
	return out
}

func TestW1_SEC_002_DriverGetForeignCompanyDenied(t *testing.T) {
	tenantID := uuid.New().String()
	userID := uuid.New().String()
	ownCompany := uuid.New().String()
	foreignCompany := uuid.New().String()

	identity := newIdentityStub(t, tenantID, userID, ownCompany, foreignCompany, []string{"CARRIER_DISPATCHER"}, nil)
	defer identity.Close()

	downstreamCalled := false
	guard := NewGuard(config.Config{
		AuthEnabled:         true,
		ProxyTimeoutSeconds: 5,
		Services:            config.ServiceURLs{Identity: identity.URL},
	}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downstreamCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	token := signTestToken(t, "secret", userID, tenantID)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/companies/"+foreignCompany, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := serveThroughAuth(t, guard.WithPolicy(PolicyRead), req, "secret")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d want 403", rec.Code)
	}
	if downstreamCalled {
		t.Fatal("downstream must not be called")
	}
}

func TestW1_SEC_001_DriverPatchForeignCompanyDenied(t *testing.T) {
	tenantID := uuid.New().String()
	userID := uuid.New().String()
	ownCompany := uuid.New().String()
	foreignCompany := uuid.New().String()

	identity := newIdentityStub(t, tenantID, userID, ownCompany, foreignCompany, []string{"CARRIER_ADMIN"}, nil)
	defer identity.Close()

	downstreamCalled := false
	guard := NewGuard(config.Config{
		AuthEnabled:         true,
		ProxyTimeoutSeconds: 5,
		Services:            config.ServiceURLs{Identity: identity.URL},
	}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		downstreamCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	token := signTestToken(t, "secret", userID, tenantID)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/companies/"+foreignCompany, strings.NewReader(`{"short_name":"evil"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := serveThroughAuth(t, guard.WithPolicy(PolicyUpdate), req, "secret")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d want 403", rec.Code)
	}
	if downstreamCalled {
		t.Fatal("downstream must not be called")
	}
}

func TestW1_SEC_003_TenantQuerySpoofDenied(t *testing.T) {
	tenantID := uuid.New().String()
	userID := uuid.New().String()
	foreignTenant := uuid.New().String()

	identity := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}})
	}))
	defer identity.Close()

	downstreamCalled := false
	guard := NewGuard(config.Config{
		AuthEnabled:         true,
		ProxyTimeoutSeconds: 5,
		Services:            config.ServiceURLs{Identity: identity.URL},
	}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		downstreamCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	token := signTestToken(t, "secret", userID, tenantID)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/companies?tenant_id="+foreignTenant, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := serveThroughAuth(t, guard.WithPolicy(PolicyList), req, "secret")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d want 403", rec.Code)
	}
	if downstreamCalled {
		t.Fatal("downstream must not be called")
	}
}

func TestAuthorizedCompanyAdminReadPass(t *testing.T) {
	tenantID := uuid.New().String()
	userID := uuid.New().String()
	companyID := uuid.New().String()

	identity := newIdentityStub(t, tenantID, userID, companyID, uuid.New().String(), []string{"CARRIER_DISPATCHER"}, nil)
	defer identity.Close()

	var gotTenant, gotUser string
	guard := NewGuard(config.Config{
		AuthEnabled:         true,
		ProxyTimeoutSeconds: 5,
		Services:            config.ServiceURLs{Identity: identity.URL},
	}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTenant = r.Header.Get("X-Tenant-ID")
		gotUser = r.Header.Get("X-User-ID")
		w.WriteHeader(http.StatusOK)
	}))

	token := signTestToken(t, "secret", userID, tenantID)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/companies/"+companyID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := serveThroughAuth(t, guard.WithPolicy(PolicyRead), req, "secret")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", rec.Code)
	}
	if gotTenant != tenantID || gotUser != userID {
		t.Fatalf("trusted headers not forwarded tenant=%q user=%q", gotTenant, gotUser)
	}
}

func TestDriverDeleteForeignCompanyDenied(t *testing.T) {
	tenantID := uuid.New().String()
	userID := uuid.New().String()
	foreignCompany := uuid.New().String()

	identity := newIdentityStub(t, tenantID, userID, uuid.New().String(), foreignCompany, []string{"CARRIER_ADMIN"}, nil)
	defer identity.Close()

	guard := NewGuard(config.Config{
		AuthEnabled:         true,
		ProxyTimeoutSeconds: 5,
		Services:            config.ServiceURLs{Identity: identity.URL},
	}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("downstream must not be called")
	}))

	token := signTestToken(t, "secret", userID, tenantID)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/companies/"+foreignCompany, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := serveThroughAuth(t, guard.WithPolicy(PolicyDelete), req, "secret")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d want 403", rec.Code)
	}
}

func TestPlatformAdminDeleteAllowed(t *testing.T) {
	tenantID := uuid.New().String()
	userID := uuid.New().String()
	companyID := uuid.New().String()

	identity := newIdentityStub(t, tenantID, userID, uuid.New().String(), companyID, nil, []string{"PLATFORM_ADMIN"})
	defer identity.Close()

	guard := NewGuard(config.Config{
		AuthEnabled:         true,
		ProxyTimeoutSeconds: 5,
		Services:            config.ServiceURLs{Identity: identity.URL},
	}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	token := signTestToken(t, "secret", userID, tenantID)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/companies/"+companyID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := serveThroughAuth(t, guard.WithPolicy(PolicyDelete), req, "secret")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d want 204", rec.Code)
	}
}
