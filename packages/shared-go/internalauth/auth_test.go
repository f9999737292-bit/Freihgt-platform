package internalauth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthorizeValidToken(t *testing.T) {
	cfg := Config{Token: "secret-token", Environment: "development"}
	req := httptest.NewRequest(http.MethodPost, "/internal/v1/test", nil)
	req.Header.Set(HeaderName, "secret-token")
	if !cfg.Authorize(req) {
		t.Fatal("expected valid token to authorize")
	}
}

func TestAuthorizeMissingToken(t *testing.T) {
	cfg := Config{Token: "secret-token", Environment: "development"}
	req := httptest.NewRequest(http.MethodPost, "/internal/v1/test", nil)
	if cfg.Authorize(req) {
		t.Fatal("expected missing token to deny")
	}
}

func TestAuthorizeBadToken(t *testing.T) {
	cfg := Config{Token: "secret-token", Environment: "development"}
	req := httptest.NewRequest(http.MethodPost, "/internal/v1/test", nil)
	req.Header.Set(HeaderName, "wrong-token")
	if cfg.Authorize(req) {
		t.Fatal("expected bad token to deny")
	}
}

func TestAuthorizeEmptyConfiguredTokenFailsClosed(t *testing.T) {
	cfg := Config{Token: "", Environment: "production"}
	req := httptest.NewRequest(http.MethodPost, "/internal/v1/test", nil)
	req.Header.Set(HeaderName, "anything")
	if cfg.Authorize(req) {
		t.Fatal("expected empty configured token to fail closed")
	}
}

func TestMiddlewareDeniesMissingToken(t *testing.T) {
	cfg := Config{Token: "secret-token"}
	called := false
	handler := cfg.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	req := httptest.NewRequest(http.MethodPost, "/internal/v1/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if called {
		t.Fatal("handler should not run without token")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}
