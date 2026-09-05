package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/api-gateway/internal/http/middleware"
)

func TestRequestIDMiddlewareUsesExistingHeader(t *testing.T) {
	const existing = "test-request-id-123"
	handler := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := middleware.RequestIDFromContext(r.Context()); got != existing {
			t.Fatalf("context request id = %q want %q", got, existing)
		}
		if got := w.Header().Get(middleware.RequestIDHeader); got != existing {
			t.Fatalf("response header = %q want %q", got, existing)
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set(middleware.RequestIDHeader, existing)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
}

func TestRequestIDMiddlewareGeneratesUUID(t *testing.T) {
	handler := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := middleware.RequestIDFromContext(r.Context())
		if _, err := uuid.Parse(got); err != nil {
			t.Fatalf("expected uuid request id, got %q", got)
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
}

func TestAuthDisabledAllowsProtectedRoute(t *testing.T) {
	called := false
	handler := middleware.Auth(false, "secret")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/companies", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("expected handler to be called when auth disabled")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", rec.Code)
	}
}

func TestAuthEnabledProtectedRouteWithoutTokenReturns401(t *testing.T) {
	// FP-AUTH-002 missing JWT
	handler := middleware.Auth(true, "secret")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/companies", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", rec.Code)
	}
}

func TestFP_AUTH_003_MalformedJWT(t *testing.T) {
	handler := middleware.Auth(true, "secret")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	for _, token := range []string{"not-a-jwt", "Bearer", "eyJhbGciOiJub25lIn0.eyJzdWIiOiJ1In0."} {
		t.Run(token, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/companies", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status=%d want 401 for malformed token %q", rec.Code, token)
			}
		})
	}
}

func TestFP_AUTH_004_ExpiredJWT(t *testing.T) {
	secret := "test-secret"
	claims := jwt.MapClaims{
		"tenant_id": "tenant-a",
		"email":     "user@example.com",
		"sub":       "user-id",
		"exp":       time.Now().Add(-time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	handler := middleware.Auth(true, secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/companies", nil)
	req.Header.Set("Authorization", "Bearer "+signed)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", rec.Code)
	}
}

func TestFP_AUTH_005_InvalidSignature(t *testing.T) {
	token := signToken(t, "correct-secret", "user-id", "tenant-id", "user@example.com")
	handler := middleware.Auth(true, "wrong-secret")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/companies", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", rec.Code)
	}
}

func TestAuthEnabledPublicLoginRouteAllowed(t *testing.T) {
	called := false
	handler := middleware.Auth(true, "secret")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("expected login route to bypass auth")
	}
}

func TestAuthEnabledOpenAPIDocsRouteAllowed(t *testing.T) {
	called := false
	handler := middleware.Auth(true, "secret")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	for _, path := range []string{"/docs", "/openapi", "/openapi.yaml", "/openapi.json", "/openapi/identity-service.yaml"} {
		t.Run(path, func(t *testing.T) {
			called = false
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if !called {
				t.Fatal("expected openapi route to bypass auth")
			}
		})
	}
}

func TestAuthEnabledValidTokenSetsHeaders(t *testing.T) {
	// FP-AUTH-001 valid JWT
	secret := "test-secret"
	token := signToken(t, secret, "user-id", "tenant-id", "user@example.com")

	var gotUserID, gotTenantID, gotEmail string
	var gotContextTenant string
	handler := middleware.Auth(true, secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID = r.Header.Get("X-User-ID")
		gotTenantID = r.Header.Get("X-Tenant-ID")
		gotEmail = r.Header.Get("X-User-Email")
		if ac, ok := middleware.AuthContextFromContext(r.Context()); ok {
			gotContextTenant = ac.TenantID
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/companies", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", rec.Code)
	}
	if gotUserID != "user-id" || gotTenantID != "tenant-id" || gotEmail != "user@example.com" {
		t.Fatalf("unexpected forwarded headers: user=%q tenant=%q email=%q", gotUserID, gotTenantID, gotEmail)
	}
	if gotContextTenant != "tenant-id" {
		t.Fatalf("context tenant=%q want tenant-id", gotContextTenant)
	}
}

func TestAuthEnabledStripsSpoofedIdentityHeaders(t *testing.T) {
	// FP-SEC-016 tenant/user header spoof — verified context from JWT only
	secret := "test-secret"
	token := signToken(t, secret, "user-id", "tenant-a", "user@example.com")

	handler := middleware.Auth(true, secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Tenant-ID"); got != "tenant-a" {
			t.Fatalf("tenant header=%q want tenant-a", got)
		}
		if got := r.Header.Get("X-User-ID"); got != "user-id" {
			t.Fatalf("user header=%q want user-id", got)
		}
		if spoofed := r.Header.Get("X-Platform-Admin"); spoofed != "" {
			t.Fatalf("X-Platform-Admin must be stripped, got %q", spoofed)
		}
		if spoofed := r.Header.Get("X-Role"); spoofed != "" {
			t.Fatalf("X-Role must be stripped, got %q", spoofed)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/companies", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Tenant-ID", "tenant-b")
	req.Header.Set("X-User-ID", "spoofed-user")
	req.Header.Set("X-User-Email", "spoofed@example.com")
	req.Header.Set("X-Company-ID", "company-b")
	req.Header.Set("X-Platform-Admin", "true")
	req.Header.Set("X-Role", "PLATFORM_ADMIN")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", rec.Code)
	}
}

func TestAuthEnabledStripsSpoofedIdentityHeadersOnPostMutation(t *testing.T) {
	secret := "test-secret"
	token := signToken(t, secret, "user-id", "tenant-a", "user@example.com")

	handler := middleware.Auth(true, secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Tenant-ID"); got != "tenant-a" {
			t.Fatalf("tenant header=%q want tenant-a", got)
		}
		if got := r.Header.Get("X-User-ID"); got != "user-id" {
			t.Fatalf("user header=%q want user-id", got)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/shipments/from-transport-order", strings.NewReader(`{"shipment_number":"SHP-1"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-b")
	req.Header.Set("X-User-ID", "spoofed-user")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", rec.Code)
	}
}

func TestAuthEnabledUserCompaniesRouteRequiresBearerToken(t *testing.T) {
	handler := middleware.Auth(true, "secret")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called without bearer token")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/33333333-3333-3333-3333-333333333333/companies", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", rec.Code)
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
		t.Fatalf("sign token: %v", err)
	}
	return signed
}
