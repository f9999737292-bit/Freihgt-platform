package middleware_test

import (
	"net/http"
	"net/http/httptest"
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
	secret := "test-secret"
	token := signToken(t, secret, "user-id", "tenant-id", "user@example.com")

	var gotUserID, gotTenantID, gotEmail string
	handler := middleware.Auth(true, secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID = r.Header.Get("X-User-ID")
		gotTenantID = r.Header.Get("X-Tenant-ID")
		gotEmail = r.Header.Get("X-User-Email")
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
