package identityguard_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/freight-platform/api-gateway/internal/http/middleware"
	"github.com/freight-platform/api-gateway/internal/identityguard"
)

const guardSecret = "gateway-identity-guard-secret"

const (
	guardUserA   = "33333333-3333-3333-3333-333333333333"
	guardAdminA  = "55555555-5555-5555-5555-555555555555"
	guardTenantA = "11111111-1111-1111-1111-111111111111"
	guardTenantB = "22222222-2222-2222-2222-222222222222"
	guardRole    = "66666666-6666-6666-6666-666666666666"
	guardCompany = "77777777-7777-7777-7777-777777777777"
)

func TestGatewayIdentityManagementRoutesFailClosed(t *testing.T) {
	verifier := &scriptedVerifier{}
	nextCalled := false
	var downstreamTenant string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		downstreamTenant = r.Header.Get("X-Tenant-ID")
		w.WriteHeader(http.StatusNoContent)
	})
	guard := identityguard.New(guardSecret, verifier).Middleware(next)
	handler := middleware.Auth(false, guardSecret)(guard)

	t.Run("anonymous POST users denied", func(t *testing.T) {
		nextCalled = false
		rec := serve(handler, http.MethodPost, "/api/v1/users", "", `{"email":"a@b.c"}`)
		assertGuardCode(t, rec, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED")
		if nextCalled {
			t.Fatal("anonymous create was proxied")
		}
	})
	t.Run("anonymous GET users denied", func(t *testing.T) {
		rec := serve(handler, http.MethodGet, "/api/v1/users", "", "")
		assertGuardCode(t, rec, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED")
	})
	t.Run("anonymous GET roles denied", func(t *testing.T) {
		rec := serve(handler, http.MethodGet, "/api/v1/roles", "", "")
		assertGuardCode(t, rec, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED")
	})
	t.Run("tenant A JWT cannot use query tenant B", func(t *testing.T) {
		nextCalled = false
		verifier.admin = false
		rec := serve(handler, http.MethodGet, "/api/v1/users?tenant_id="+guardTenantB, signGuardToken(t, guardUserA, guardTenantA), "")
		assertGuardCode(t, rec, http.StatusConflict, "TENANT_CONTEXT_CONFLICT")
		if nextCalled {
			t.Fatal("query tenant override was proxied")
		}
	})
	t.Run("tenant A JWT cannot use body tenant B", func(t *testing.T) {
		nextCalled = false
		body := `{"tenant_id":"` + guardTenantB + `","email":"a@b.c","password":"secret","full_name":"A","preferred_locale":"en-US"}`
		rec := serve(handler, http.MethodPost, "/api/v1/users", signGuardToken(t, guardUserA, guardTenantA), body)
		assertGuardCode(t, rec, http.StatusConflict, "TENANT_CONTEXT_CONFLICT")
		if nextCalled {
			t.Fatal("body tenant override was proxied")
		}
	})
	t.Run("spoofed identity headers do not select tenant", func(t *testing.T) {
		nextCalled = false
		req := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+guardUserA+"/companies?tenant_id="+guardTenantA, nil)
		req.Header.Set("Authorization", "Bearer "+signGuardToken(t, guardUserA, guardTenantA))
		req.Header.Set("X-Tenant-ID", guardTenantB)
		req.Header.Set("X-User-ID", guardAdminA)
		req.Header.Set("X-Platform-Admin", "true")
		req.Header.Set("X-Internal-Service-Token", "shared")
		req.Header.Set("X-Internal-Service-Name", "api-gateway")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		if !nextCalled || downstreamTenant != guardTenantA {
			t.Fatalf("downstream tenant=%q called=%v", downstreamTenant, nextCalled)
		}
	})
	t.Run("ordinary JWT cannot perform platform-admin action", func(t *testing.T) {
		nextCalled = false
		verifier.admin = false
		rec := serve(handler, http.MethodPost, "/api/v1/users", signGuardToken(t, guardUserA, guardTenantA), `{"email":"a@b.c"}`)
		assertGuardCode(t, rec, http.StatusForbidden, "AUTHORIZATION_DENIED")
		if nextCalled {
			t.Fatal("ordinary JWT performed an admin action")
		}
	})
	t.Run("verified platform admin follows the documented override", func(t *testing.T) {
		nextCalled = false
		verifier.admin = true
		rec := serve(handler, http.MethodGet, "/api/v1/users?tenant_id="+guardTenantB, signGuardToken(t, guardAdminA, guardTenantA), "")
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		if !nextCalled || downstreamTenant != guardTenantA {
			t.Fatalf("admin override downstream tenant=%q", downstreamTenant)
		}
		if !strings.HasPrefix(verifier.sawAuthorization, "Bearer ") {
			t.Fatal("platform admin check did not use the caller bearer token")
		}
		if verifier.sawInternalToken || verifier.sawInternalName {
			t.Fatal("generic internal credential was presented to the verifier")
		}
	})
	t.Run("gateway membership lookup still allows the authenticated subject", func(t *testing.T) {
		nextCalled = false
		verifier.admin = false
		verifier.calls = 0
		rec := serve(handler, http.MethodGet, "/api/v1/users/"+guardUserA+"/companies?tenant_id="+guardTenantA, signGuardToken(t, guardUserA, guardTenantA), "")
		if rec.Code != http.StatusNoContent || !nextCalled {
			t.Fatalf("self membership lookup status=%d called=%v", rec.Code, nextCalled)
		}
		other := serve(handler, http.MethodGet, "/api/v1/users/"+guardAdminA+"/companies?tenant_id="+guardTenantA, signGuardToken(t, guardUserA, guardTenantA), "")
		assertGuardCode(t, other, http.StatusForbidden, "AUTHORIZATION_DENIED")
	})
	t.Run("AUTH_ENABLED false does not make identity management anonymous", func(t *testing.T) {
		rec := serve(handler, http.MethodDelete, "/api/v1/users/"+guardUserA+"/companies/"+guardCompany+"/roles/"+guardRole, "", "")
		assertGuardCode(t, rec, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED")
	})
}

func TestHTTPVerifierUsesCallerBearerOnly(t *testing.T) {
	var sawAuth, sawInternal, sawName string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawAuth = r.Header.Get("Authorization")
		sawInternal = r.Header.Get("X-Internal-Service-Token")
		sawName = r.Header.Get("X-Internal-Service-Name")
		if !strings.Contains(r.URL.Path, guardAdminA) || r.URL.Query().Get("tenant_id") != guardTenantA {
			t.Fatalf("lookup path=%s query=%s", r.URL.Path, r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{{"code": "PLATFORM_ADMIN"}},
		})
	}))
	defer server.Close()

	admin, err := identityguard.NewHTTPVerifier(server.URL).IsPlatformAdmin(context.Background(), "Bearer caller-token", guardAdminA, guardTenantA)
	if err != nil {
		t.Fatal(err)
	}
	if !admin {
		t.Fatal("expected platform admin from server-verified roles")
	}
	if sawAuth != "Bearer caller-token" || sawInternal != "" || sawName != "" {
		t.Fatalf("auth=%q internal=%q name=%q", sawAuth, sawInternal, sawName)
	}
}

func TestPostUsersIsNotAPublicGatewayRoute(t *testing.T) {
	handler := middleware.Auth(true, guardSecret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("public user create reached the handler")
	}))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rec.Code)
	}
}

type scriptedVerifier struct {
	admin            bool
	calls            int
	sawAuthorization string
	sawInternalToken bool
	sawInternalName  bool
}

func (s *scriptedVerifier) IsPlatformAdmin(_ context.Context, authorization, _, _ string) (bool, error) {
	s.calls++
	s.sawAuthorization = authorization
	s.sawInternalToken = strings.Contains(authorization, "shared")
	s.sawInternalName = strings.Contains(authorization, "api-gateway")
	return s.admin, nil
}

func serve(handler http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func assertGuardCode(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status=%d want %d body=%s", rec.Code, status, rec.Body.String())
	}
	var payload struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v body=%s", err, rec.Body.String())
	}
	if payload.Error.Code != code {
		t.Fatalf("code=%s want %s", payload.Error.Code, code)
	}
}

func signGuardToken(t *testing.T, userID, tenantID string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"sub":       userID,
		"tenant_id": tenantID,
		"email":     "user@example.com",
		"exp":       time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(guardSecret))
	if err != nil {
		t.Fatal(err)
	}
	return signed
}
