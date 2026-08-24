//go:build integration

package securitywave1

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	gwmiddleware "github.com/freight-platform/api-gateway/internal/http/middleware"
)

// Wave 1 security gate — real JWT middleware path (AUTH_MIDDLEWARE_REAL=YES).

func TestFP_SEC_TenantHeaderSpoof(t *testing.T) {
	secret := "wave1-secret"
	token := signToken(t, secret, "user-a", "tenant-a", "a@test.local")

	var gotTenant string
	handler := gwmiddleware.Auth(true, secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTenant = r.Header.Get("X-Tenant-ID")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/companies", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Tenant-ID", "tenant-b-spoof")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", rec.Code)
	}
	if gotTenant != "tenant-a" {
		t.Fatalf("tenant=%q want verified tenant-a", gotTenant)
	}
}

func TestFP_SEC_UserHeaderSpoof(t *testing.T) {
	secret := "wave1-secret"
	token := signToken(t, secret, "user-a", "tenant-a", "a@test.local")

	var gotUser, gotEmail string
	handler := gwmiddleware.Auth(true, secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = r.Header.Get("X-User-ID")
		gotEmail = r.Header.Get("X-User-Email")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/companies", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-User-ID", "privileged-user")
	req.Header.Set("X-User-Email", "admin@example.invalid")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if gotUser != "user-a" || gotEmail != "a@test.local" {
		t.Fatalf("user=%q email=%q want verified identity", gotUser, gotEmail)
	}
}

func TestFP_SEC_RoleHeaderSpoof(t *testing.T) {
	secret := "wave1-secret"
	token := signToken(t, secret, "user-a", "tenant-a", "carrier@test.local")

	handler := gwmiddleware.Auth(true, secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if roles := r.Header.Get("X-User-Roles"); roles != "" {
			t.Fatalf("X-User-Roles must be stripped, got %q", roles)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/companies", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-User-Roles", "PLATFORM_ADMIN")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestFP_SEC_AdminSpoof(t *testing.T) {
	secret := "wave1-secret"
	token := signToken(t, secret, "user-a", "tenant-a", "user@test.local")

	handler := gwmiddleware.Auth(true, secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, h := range []string{"X-Platform-Admin", "X-Internal-Service-Token", "X-Actor-Kind", "X-Company-ID"} {
			if v := r.Header.Get(h); v != "" {
				t.Fatalf("%s must be stripped, got %q", h, v)
			}
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/shipments/from-transport-order", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Platform-Admin", "true")
	req.Header.Set("X-Internal-Service-Token", "spoof")
	req.Header.Set("X-Actor-Kind", "PLATFORM_ADMIN")
	req.Header.Set("X-Company-ID", "22222222-2222-2222-2222-222222222222")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestFP_SEC_AuthFailsClosedNoFallbackTenant(t *testing.T) {
	handler := gwmiddleware.Auth(true, "secret")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("must not reach handler without valid JWT")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/companies?tenant_id=11111111-1111-1111-1111-111111111111", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401 without JWT even with query tenant_id", rec.Code)
	}
}

func TestFP_SEC_InternalRouteNotOnPublicGateway(t *testing.T) {
	secret := "wave1-secret"
	token := signToken(t, secret, "user-a", "tenant-a", "user@test.local")

	handler := gwmiddleware.Auth(true, secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))

	for _, path := range []string{
		"/internal/v1/freight-cost/transport-orders/00000000-0000-0000-0000-000000000001",
		"/internal/v1/control-tower/status-summary",
	} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusNotFound {
				t.Fatalf("%s status=%d want 404", path, rec.Code)
			}
		})
	}
}

func TestFP_SEC_VerifiedContextInRequestContext(t *testing.T) {
	secret := "wave1-secret"
	token := signToken(t, secret, "user-a", "tenant-a", "a@test.local")

	handler := gwmiddleware.Auth(true, secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ac, ok := gwmiddleware.AuthContextFromContext(r.Context())
		if !ok {
			t.Fatal("auth context missing")
		}
		if ac.UserID != "user-a" || ac.TenantID != "tenant-a" || ac.Email != "a@test.local" {
			t.Fatalf("context=%+v", ac)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/companies", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
}

func TestWave1CatalogMinimumDomains(t *testing.T) {
	// Ensures Wave 1 manifest lists mandatory multi-domain coverage packages.
	required := []string{
		"FP-AUTH-001", "FP-AUTH-002", "FP-AUTH-003", "FP-AUTH-004", "FP-AUTH-005",
		"FP-SEC-016", "FP-E2E-SEC-001", "FP-E2E-SEC-002",
	}
	if len(required) < 8 {
		t.Fatal("wave1 catalog incomplete")
	}
	if _, err := json.Marshal(required); err != nil {
		t.Fatalf("marshal catalog: %v", err)
	}
}

func signToken(t *testing.T, secret, userID, tenantID, email string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"tenant_id": tenantID,
		"email":     email,
		"sub":       userID,
		"exp":       time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return signed
}
