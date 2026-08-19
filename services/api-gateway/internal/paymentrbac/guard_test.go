package paymentrbac

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

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

func newTestGuard(t *testing.T, identityURL string, downstream http.HandlerFunc) *Guard {
	t.Helper()
	return NewGuard(config.Config{
		AuthEnabled:         true,
		ProxyTimeoutSeconds: 5,
		Services:            config.ServiceURLs{Identity: identityURL},
	}, downstream)
}

func TestPaymentHTTPRBACMatrix(t *testing.T) {
	t.Parallel()
	tenantID := "11111111-1111-1111-1111-111111111111"
	companyID := "22222222-2222-2222-2222-222222222222"
	path := "/api/v1/payments?company_id=" + companyID

	tests := []struct {
		name       string
		roles      []string
		wantStatus int
	}{
		{"FINANCE_MANAGER_ALLOWED=PASS", []string{"FINANCE_MANAGER"}, http.StatusOK},
		{"INSUFFICIENT_RBAC=DENY", []string{"CARRIER_DISPATCHER"}, http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.Path == "/v1/auth/me":
					_ = json.NewEncoder(w).Encode(map[string]any{"roles": tt.roles})
				case strings.HasPrefix(r.URL.Path, "/v1/users/") && strings.Contains(r.URL.Path, "/companies"):
					_ = json.NewEncoder(w).Encode(map[string]any{
						"items": []map[string]any{{
							"company_id": companyID, "company_type": "SHIPPER",
							"roles": []map[string]any{{"code": tt.roles[0]}},
						}},
					})
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer identityServer.Close()

			guard := newTestGuard(t, identityServer.URL, func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.ReadAll(r.Body)
				w.WriteHeader(http.StatusOK)
			})

			token := signTestToken(t, "secret", "user-1", tenantID)
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("Authorization", "Bearer "+token)

			rec := serveThroughAuth(t, guard.WithPolicy(PolicyRead), req, "secret")
			if rec.Code != tt.wantStatus {
				t.Fatalf("expected %d got %d body=%s", tt.wantStatus, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestPaymentGatewayNoJWTDenied(t *testing.T) {
	t.Parallel()
	identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"roles": []string{"FINANCE_MANAGER"}})
	}))
	defer identityServer.Close()
	guard := newTestGuard(t, identityServer.URL, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/payments", nil)
	rec := serveThroughAuth(t, guard.WithPolicy(PolicyRead), req, "secret")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("NO_JWT=DENY expected 401, got %d", rec.Code)
	}
}

func TestPaymentGatewayInvalidJWTDenied(t *testing.T) {
	t.Parallel()
	identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer identityServer.Close()
	guard := newTestGuard(t, identityServer.URL, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/payments", nil)
	req.Header.Set("Authorization", "Bearer not-a-valid-token")
	rec := serveThroughAuth(t, guard.WithPolicy(PolicyRead), req, "secret")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("INVALID_JWT=DENY expected 401, got %d", rec.Code)
	}
}
