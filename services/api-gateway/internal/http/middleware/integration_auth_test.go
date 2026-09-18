package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/api-gateway/internal/http/middleware"
	"github.com/freight-platform/shared-go/clientip"
	"github.com/freight-platform/shared-go/integrationauth"
)

const (
	testHumanJWTSecret       = "human-jwt-secret"
	testIntegrationJWTSecret = "integration-jwt-secret"
	integrationFixturePath   = "/api/v1/integrations/erp/_fixture/protected"
	oauthTokenPath           = "/api/v1/integrations/oauth/token"
	humanProtectedPath       = "/api/v1/rfx-events"
)

func newGatewayAuthChain(t *testing.T, final http.HandlerFunc, limiter *integrationauth.PrincipalRateLimiter) http.Handler {
	t.Helper()
	integrationJWT := integrationauth.NewJWTService(testIntegrationJWTSecret, integrationauth.TokenTTL)
	integrationLayer := middleware.IntegrationAuth(middleware.IntegrationAuthConfig{
		Enabled:              true,
		IntegrationJWTSecret: testIntegrationJWTSecret,
		RateLimiter:          limiter,
		ClientIPResolver:     clientip.NewResolver(nil),
	})(final)
	return middleware.AuthWithIntegrationSupport(true, testHumanJWTSecret, integrationJWT)(integrationLayer)
}

func signIntegrationToken(t *testing.T, principalID, tenantID, companyID uuid.UUID) string {
	t.Helper()
	svc := integrationauth.NewJWTService(testIntegrationJWTSecret, integrationauth.TokenTTL)
	token, _, err := svc.CreateIntegrationToken(integrationauth.AuthenticatedContext{
		PrincipalID: principalID,
		TenantID:    tenantID,
		CompanyID:   companyID,
		AuthScheme:  integrationauth.AuthSchemeOAuth,
		Scopes:      []string{"rfx:draft:read"},
	})
	if err != nil {
		t.Fatalf("integration token: %v", err)
	}
	return token
}

func TestE2GW001OAuthTokenPublicWithoutBearer(t *testing.T) {
	called := false
	handler := newGatewayAuthChain(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}), nil)

	req := httptest.NewRequest(http.MethodPost, oauthTokenPath, strings.NewReader("grant_type=client_credentials&client_id=a&client_secret=b"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("oauth token request must reach upstream without bearer")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", rec.Code)
	}
}

func TestE2GW001OAuthTokenNeighborAndMethodProtected(t *testing.T) {
	cases := []struct {
		method string
		path   string
	}{
		{http.MethodGet, oauthTokenPath},
		{http.MethodPut, oauthTokenPath},
		{http.MethodPost, oauthTokenPath + "/extra"},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			handler := newGatewayAuthChain(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Fatal("protected route must not reach upstream without auth")
			}), nil)
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status=%d want 401", rec.Code)
			}
		})
	}
}

func TestE2GW001OAuthTokenStripsSpoofedIntegrationHeaders(t *testing.T) {
	var gotPrincipal string
	handler := newGatewayAuthChain(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPrincipal = r.Header.Get(integrationauth.HeaderIntegrationPrincipalID)
		w.WriteHeader(http.StatusOK)
	}), nil)

	req := httptest.NewRequest(http.MethodPost, oauthTokenPath, nil)
	req.Header.Set(integrationauth.HeaderIntegrationPrincipalID, uuid.NewString())
	req.Header.Set("X-Integration-Tenant-ID", uuid.NewString())
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if gotPrincipal != "" {
		t.Fatalf("expected spoofed integration principal header to be stripped, got %q", gotPrincipal)
	}
}

func TestE2GW002IntegrationJWTRejectedOnHumanRoute(t *testing.T) {
	handler := newGatewayAuthChain(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("human route must not accept integration jwt")
	}), nil)
	token := signIntegrationToken(t, uuid.New(), uuid.New(), uuid.New())
	req := httptest.NewRequest(http.MethodGet, humanProtectedPath, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", rec.Code)
	}
}

func TestE2GW002APIKeyRejectedOnHumanRoute(t *testing.T) {
	handler := newGatewayAuthChain(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("human route must not accept api key bearer")
	}), nil)
	req := httptest.NewRequest(http.MethodGet, humanProtectedPath, nil)
	req.Header.Set("Authorization", "Bearer bt_live_test_key_value")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", rec.Code)
	}
}

func TestE2GW002HumanJWTAcceptedOnHumanRoute(t *testing.T) {
	called := false
	handler := newGatewayAuthChain(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Header.Get("X-User-ID") == "" {
			t.Fatal("expected human user header")
		}
		w.WriteHeader(http.StatusOK)
	}), nil)
	token := signToken(t, testHumanJWTSecret, "user-id", "tenant-id", "user@example.com")
	req := httptest.NewRequest(http.MethodGet, humanProtectedPath, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if !called || rec.Code != http.StatusOK {
		t.Fatalf("called=%v status=%d want human jwt pass", called, rec.Code)
	}
}

func TestE2GW002IntegrationJWTAcceptedOnFixtureRoute(t *testing.T) {
	called := false
	principalID := uuid.New()
	tenantID := uuid.New()
	companyID := uuid.New()
	handler := newGatewayAuthChain(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if got := r.Header.Get(integrationauth.HeaderIntegrationPrincipalID); got != principalID.String() {
			t.Fatalf("principal header=%q", got)
		}
		if got := r.Header.Get("X-Tenant-ID"); got != tenantID.String() {
			t.Fatalf("tenant header=%q", got)
		}
		if r.Header.Get("X-User-ID") != "" {
			t.Fatal("integration route must not inject human user header")
		}
		w.WriteHeader(http.StatusOK)
	}), nil)
	token := signIntegrationToken(t, principalID, tenantID, companyID)
	req := httptest.NewRequest(http.MethodGet, integrationFixturePath, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if !called || rec.Code != http.StatusOK {
		t.Fatalf("called=%v status=%d want integration jwt pass", called, rec.Code)
	}
}

func TestE7P2INT125MissingAuthorizationOnIntegrationFixture(t *testing.T) {
	handler := newGatewayAuthChain(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("integration fixture must require authorization")
	}), nil)
	req := httptest.NewRequest(http.MethodGet, integrationFixturePath, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", rec.Code)
	}
}

func TestE7P2INT125MalformedBearerOnIntegrationFixture(t *testing.T) {
	handler := newGatewayAuthChain(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("malformed bearer must be rejected")
	}), nil)
	req := httptest.NewRequest(http.MethodGet, integrationFixturePath, nil)
	req.Header.Set("Authorization", "Bearer")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", rec.Code)
	}
}

func TestE7P2INT125HumanJWTRejectedOnIntegrationFixture(t *testing.T) {
	handler := newGatewayAuthChain(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("human jwt must not satisfy integration fixture auth")
	}), nil)
	token := signToken(t, testHumanJWTSecret, "user-id", "tenant-id", "user@example.com")
	req := httptest.NewRequest(http.MethodGet, integrationFixturePath, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", rec.Code)
	}
}

func TestE7P2INT186IntegrationRouteHTTP429(t *testing.T) {
	limiter := integrationauth.NewPrincipalRateLimiter(1, time.Minute)
	principalID := uuid.New()
	tenantID := uuid.New()
	companyID := uuid.New()
	token := signIntegrationToken(t, principalID, tenantID, companyID)
	handler := newGatewayAuthChain(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), limiter)

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, integrationFixturePath, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if i == 0 && rec.Code != http.StatusOK {
			t.Fatalf("first request status=%d want 200", rec.Code)
		}
		if i == 1 {
			if rec.Code != http.StatusTooManyRequests {
				t.Fatalf("second request status=%d want 429", rec.Code)
			}
			if rec.Header().Get("Retry-After") == "" {
				t.Fatal("expected Retry-After header")
			}
		}
	}
}

func TestE7P2INT186PrincipalRateLimitIsolation(t *testing.T) {
	limiter := integrationauth.NewPrincipalRateLimiter(1, time.Minute)
	keyA := limiter.Key(uuid.NewString(), uuid.NewString(), "integration_request")
	keyB := limiter.Key(uuid.NewString(), uuid.NewString(), "integration_request")
	if allowed, _ := limiter.Allow(keyA); !allowed {
		t.Fatal("first principal A request should pass")
	}
	if allowed, _ := limiter.Allow(keyA); allowed {
		t.Fatal("second principal A request should be limited")
	}
	if allowed, _ := limiter.Allow(keyB); !allowed {
		t.Fatal("principal B should have independent quota")
	}
}

func TestJWTClassSeparationHumanCannotParseIntegrationToken(t *testing.T) {
	integrationToken := signIntegrationToken(t, uuid.New(), uuid.New(), uuid.New())
	handler := middleware.Auth(true, testHumanJWTSecret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("integration token must not pass human auth middleware")
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/companies", nil)
	req.Header.Set("Authorization", "Bearer "+integrationToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", rec.Code)
	}
}
