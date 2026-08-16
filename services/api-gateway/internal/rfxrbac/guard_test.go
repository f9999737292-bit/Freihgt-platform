package rfxrbac

import (
	"encoding/json"
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
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"tenant_id": tenantID,
		"sub":       userID,
		"exp":       time.Now().Add(time.Hour).Unix(),
	})
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

func TestCreateRfxEventBuyerAllowed(t *testing.T) {
	tenantID := "11111111-1111-1111-1111-111111111111"
	identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"roles": []string{"PROCUREMENT_MANAGER"}})
	}))
	defer identityServer.Close()

	downstreamCalled := false
	guard := NewGuard(config.Config{
		AuthEnabled:         true,
		ProxyTimeoutSeconds: 5,
		Services:            config.ServiceURLs{Identity: identityServer.URL},
	}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downstreamCalled = true
		w.WriteHeader(http.StatusCreated)
	}))

	token := signTestToken(t, "secret", "user-1", tenantID)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/rfx-events", strings.NewReader(`{"title":"RFQ"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	rec := serveThroughAuth(t, guard.WithPolicy(PolicyBuyerManage), req, "secret")
	if rec.Code != http.StatusCreated || !downstreamCalled {
		t.Fatalf("status=%d downstream=%v body=%s", rec.Code, downstreamCalled, rec.Body.String())
	}
}

func TestCreateRfxEventCarrierDenied(t *testing.T) {
	tenantID := "11111111-1111-1111-1111-111111111111"
	identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"roles": []string{"CARRIER_ADMIN"}})
	}))
	defer identityServer.Close()

	downstreamCalled := false
	guard := NewGuard(config.Config{
		AuthEnabled:         true,
		ProxyTimeoutSeconds: 5,
		Services:            config.ServiceURLs{Identity: identityServer.URL},
	}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downstreamCalled = true
	}))

	token := signTestToken(t, "secret", "user-1", tenantID)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/rfx-events", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+token)

	rec := serveThroughAuth(t, guard.WithPolicy(PolicyBuyerManage), req, "secret")
	if rec.Code != http.StatusForbidden || downstreamCalled {
		t.Fatalf("status=%d downstream=%v", rec.Code, downstreamCalled)
	}
}

func TestSubmitBidCarrierAllowed(t *testing.T) {
	tenantID := "11111111-1111-1111-1111-111111111111"
	identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"roles": []string{"CARRIER_DISPATCHER"}})
	}))
	defer identityServer.Close()

	downstreamCalled := false
	guard := NewGuard(config.Config{
		AuthEnabled:         true,
		ProxyTimeoutSeconds: 5,
		Services:            config.ServiceURLs{Identity: identityServer.URL},
	}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downstreamCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	token := signTestToken(t, "secret", "user-1", tenantID)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/bids/bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb/submit", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rec := serveThroughAuth(t, guard.WithPolicy(PolicyCarrierRespond), req, "secret")
	if rec.Code != http.StatusOK || !downstreamCalled {
		t.Fatalf("status=%d downstream=%v", rec.Code, downstreamCalled)
	}
}

func TestAcceptBidBuyerAllowedCarrierDenied(t *testing.T) {
	tenantID := "11111111-1111-1111-1111-111111111111"
	path := "/api/v1/bids/bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb/accept"

	tests := []struct {
		name       string
		roles      []string
		wantStatus int
	}{
		{"buyer", []string{"SHIPPER_ADMIN"}, http.StatusOK},
		{"carrier", []string{"CARRIER_ADMIN"}, http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewEncoder(w).Encode(map[string]any{"roles": tt.roles})
			}))
			defer identityServer.Close()

			downstreamCalled := false
			guard := NewGuard(config.Config{
				AuthEnabled:         true,
				ProxyTimeoutSeconds: 5,
				Services:            config.ServiceURLs{Identity: identityServer.URL},
			}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				downstreamCalled = true
				w.WriteHeader(http.StatusOK)
			}))

			token := signTestToken(t, "secret", "user-1", tenantID)
			req := httptest.NewRequest(http.MethodPost, path, nil)
			req.Header.Set("Authorization", "Bearer "+token)

			rec := serveThroughAuth(t, guard.WithPolicy(PolicyAcceptBid), req, "secret")
			if rec.Code != tt.wantStatus {
				t.Fatalf("status=%d want=%d", rec.Code, tt.wantStatus)
			}
			if tt.wantStatus == http.StatusForbidden && downstreamCalled {
				t.Fatal("downstream must not be called on 403")
			}
			if tt.wantStatus == http.StatusOK && !downstreamCalled {
				t.Fatal("downstream should be called on allow")
			}
		})
	}
}
