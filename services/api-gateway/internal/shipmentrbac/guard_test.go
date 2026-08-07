package shipmentrbac

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
	sharedmiddleware "github.com/freight-platform/shared-go/middleware"
)

func serveThroughAuth(t *testing.T, handler http.Handler, req *http.Request, secret string, enabled bool) *httptest.ResponseRecorder {
	t.Helper()
	chain := gwmiddleware.Auth(enabled, secret)(handler)
	rec := httptest.NewRecorder()
	chain.ServeHTTP(rec, req)
	return rec
}

func signTestToken(t *testing.T, secret, userID, tenantID, email string) string {
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

func newTestGuard(t *testing.T, identityURL string, downstream http.HandlerFunc) *Guard {
	t.Helper()
	return NewGuard(config.Config{
		AuthEnabled:         true,
		ProxyTimeoutSeconds: 5,
		Services:            config.ServiceURLs{Identity: identityURL},
	}, downstream)
}

func TestCreateRBACTable(t *testing.T) {
	tenantID := "11111111-1111-1111-1111-111111111111"
	paths := []string{
		"/api/v1/shipments/from-transport-order",
		"/api/v1/shipments/from-bid",
	}
	body := `{"shipment_number":"SHP-1","transport_order_id":"33333333-3333-3333-3333-333333333333","carrier_company_id":"22222222-2222-2222-2222-222222222222","bid_id":"44444444-4444-4444-4444-444444444444"}`

	tests := []struct {
		name       string
		roles      []string
		wantStatus int
	}{
		{"PLATFORM_ADMIN", []string{"PLATFORM_ADMIN"}, http.StatusCreated},
		{"SHIPPER_ADMIN", []string{"SHIPPER_ADMIN"}, http.StatusCreated},
		{"SHIPPER_LOGIST", []string{"SHIPPER_LOGIST"}, http.StatusCreated},
		{"FORWARDER_MANAGER", []string{"FORWARDER_MANAGER"}, http.StatusCreated},
		{"CARRIER_ADMIN", []string{"CARRIER_ADMIN"}, http.StatusForbidden},
		{"CARRIER_DISPATCHER", []string{"CARRIER_DISPATCHER"}, http.StatusForbidden},
		{"FINANCE_MANAGER", []string{"FINANCE_MANAGER"}, http.StatusForbidden},
		{"PROCUREMENT_MANAGER", []string{"PROCUREMENT_MANAGER"}, http.StatusForbidden},
		{"unknown", []string{"UNKNOWN_ROLE"}, http.StatusForbidden},
		{"no role", nil, http.StatusForbidden},
	}

	for _, path := range paths {
		for _, tt := range tests {
			t.Run(path+"_"+tt.name, func(t *testing.T) {
				downstreamCalled := false
				var gotBody string
				identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_ = json.NewEncoder(w).Encode(map[string]any{"roles": tt.roles})
				}))
				defer identityServer.Close()

				guard := newTestGuard(t, identityServer.URL, func(w http.ResponseWriter, r *http.Request) {
					downstreamCalled = true
					raw, _ := io.ReadAll(r.Body)
					gotBody = string(raw)
					w.WriteHeader(http.StatusCreated)
				})

				token := signTestToken(t, "secret", "user-1", tenantID, "user@example.com")
				req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
				req.Header.Set("Authorization", "Bearer "+token)
				req.Header.Set("Content-Type", "application/json")

				rec := serveThroughAuth(t, guard.WithPolicy(PolicyCreate), req, "secret", true)
				if rec.Code != tt.wantStatus {
					t.Fatalf("status=%d want=%d body=%s", rec.Code, tt.wantStatus, rec.Body.String())
				}
				if tt.wantStatus == http.StatusForbidden && downstreamCalled {
					t.Fatal("downstream must not be called on 403")
				}
				if tt.wantStatus == http.StatusCreated {
					if !downstreamCalled {
						t.Fatal("downstream should be called on allow")
					}
					if gotBody != body {
						t.Fatalf("body changed: %q", gotBody)
					}
				}
			})
		}
	}
}

func TestAcceptRBACTable(t *testing.T) {
	tenantID := "11111111-1111-1111-1111-111111111111"
	shipmentID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	path := "/api/v1/shipments/" + shipmentID + "/accept"

	tests := []struct {
		name       string
		roles      []string
		wantStatus int
	}{
		{"PLATFORM_ADMIN", []string{"PLATFORM_ADMIN"}, http.StatusOK},
		{"CARRIER_ADMIN", []string{"CARRIER_ADMIN"}, http.StatusOK},
		{"CARRIER_DISPATCHER", []string{"CARRIER_DISPATCHER"}, http.StatusOK},
		{"FORWARDER_MANAGER", []string{"FORWARDER_MANAGER"}, http.StatusForbidden},
		{"SHIPPER_ADMIN", []string{"SHIPPER_ADMIN"}, http.StatusForbidden},
		{"SHIPPER_LOGIST", []string{"SHIPPER_LOGIST"}, http.StatusForbidden},
		{"FINANCE_MANAGER", []string{"FINANCE_MANAGER"}, http.StatusForbidden},
		{"PROCUREMENT_MANAGER", []string{"PROCUREMENT_MANAGER"}, http.StatusForbidden},
		{"unknown", []string{"UNKNOWN_ROLE"}, http.StatusForbidden},
		{"no role", nil, http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			downstreamCalled := false
			identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewEncoder(w).Encode(map[string]any{"roles": tt.roles})
			}))
			defer identityServer.Close()

			guard := newTestGuard(t, identityServer.URL, func(w http.ResponseWriter, r *http.Request) {
				downstreamCalled = true
				w.WriteHeader(http.StatusOK)
			})

			token := signTestToken(t, "secret", "user-1", tenantID, "user@example.com")
			req := httptest.NewRequest(http.MethodPost, path, nil)
			req.Header.Set("Authorization", "Bearer "+token)

			rec := serveThroughAuth(t, guard.WithPolicy(PolicyAccept), req, "secret", true)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status=%d want=%d body=%s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantStatus == http.StatusForbidden && downstreamCalled {
				t.Fatal("downstream must not be called on 403")
			}
		})
	}
}

func TestUpdateStatusRBACTable(t *testing.T) {
	tenantID := "11111111-1111-1111-1111-111111111111"
	shipmentID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	path := "/api/v1/shipments/" + shipmentID + "/status"

	tests := []struct {
		name       string
		roles      []string
		wantStatus int
	}{
		{"PLATFORM_ADMIN", []string{"PLATFORM_ADMIN"}, http.StatusOK},
		{"CARRIER_ADMIN", []string{"CARRIER_ADMIN"}, http.StatusOK},
		{"CARRIER_DISPATCHER", []string{"CARRIER_DISPATCHER"}, http.StatusOK},
		{"SHIPPER_ADMIN", []string{"SHIPPER_ADMIN"}, http.StatusForbidden},
		{"SHIPPER_LOGIST", []string{"SHIPPER_LOGIST"}, http.StatusForbidden},
		{"FINANCE_MANAGER", []string{"FINANCE_MANAGER"}, http.StatusForbidden},
		{"PROCUREMENT_MANAGER", []string{"PROCUREMENT_MANAGER"}, http.StatusForbidden},
		{"unknown", []string{"UNKNOWN_ROLE"}, http.StatusForbidden},
		{"no role", nil, http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := 0
			identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewEncoder(w).Encode(map[string]any{"roles": tt.roles})
			}))
			defer identityServer.Close()

			guard := newTestGuard(t, identityServer.URL, func(w http.ResponseWriter, r *http.Request) {
				callCount++
				w.WriteHeader(http.StatusOK)
			})

			token := signTestToken(t, "secret", "user-1", tenantID, "user@example.com")
			req := httptest.NewRequest(http.MethodPatch, path, strings.NewReader(`{"status":"IN_TRANSIT"}`))
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Content-Type", "application/json")

			rec := serveThroughAuth(t, guard.WithPolicy(PolicyUpdateStatus), req, "secret", true)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status=%d want=%d body=%s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantStatus == http.StatusForbidden && callCount > 0 {
				t.Fatal("downstream must not be called on 403")
			}
			if tt.wantStatus == http.StatusOK && callCount != 1 {
				t.Fatalf("downstream call count=%d want 1", callCount)
			}
		})
	}
}

func TestCancelRBACTable(t *testing.T) {
	tenantID := "11111111-1111-1111-1111-111111111111"
	shipmentID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	path := "/api/v1/shipments/" + shipmentID + "/cancel"
	body := `{"reason":"CUSTOMER_REQUEST"}`

	tests := []struct {
		name       string
		roles      []string
		wantStatus int
	}{
		{"PLATFORM_ADMIN", []string{"PLATFORM_ADMIN"}, http.StatusOK},
		{"SHIPPER_ADMIN", []string{"SHIPPER_ADMIN"}, http.StatusOK},
		{"SHIPPER_LOGIST", []string{"SHIPPER_LOGIST"}, http.StatusOK},
		{"FORWARDER_MANAGER", []string{"FORWARDER_MANAGER"}, http.StatusOK},
		{"CARRIER_ADMIN", []string{"CARRIER_ADMIN"}, http.StatusForbidden},
		{"CARRIER_DISPATCHER", []string{"CARRIER_DISPATCHER"}, http.StatusForbidden},
		{"FINANCE_MANAGER", []string{"FINANCE_MANAGER"}, http.StatusForbidden},
		{"PROCUREMENT_MANAGER", []string{"PROCUREMENT_MANAGER"}, http.StatusForbidden},
		{"unknown", []string{"UNKNOWN_ROLE"}, http.StatusForbidden},
		{"no role", nil, http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			downstreamCalled := false
			var gotBody string
			identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewEncoder(w).Encode(map[string]any{"roles": tt.roles})
			}))
			defer identityServer.Close()

			guard := newTestGuard(t, identityServer.URL, func(w http.ResponseWriter, r *http.Request) {
				downstreamCalled = true
				raw, _ := io.ReadAll(r.Body)
				gotBody = string(raw)
				w.WriteHeader(http.StatusOK)
			})

			token := signTestToken(t, "secret", "user-1", tenantID, "user@example.com")
			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Content-Type", "application/json")

			rec := serveThroughAuth(t, guard.WithPolicy(PolicyCancel), req, "secret", true)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status=%d want=%d body=%s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantStatus == http.StatusForbidden && downstreamCalled {
				t.Fatal("downstream must not be called on 403")
			}
			if tt.wantStatus == http.StatusOK && gotBody != body {
				t.Fatalf("cancel body=%q want %q", gotBody, body)
			}
		})
	}
}

func TestMutationUnauthenticatedReturns401(t *testing.T) {
	guard := NewGuard(config.Config{AuthEnabled: true, ProxyTimeoutSeconds: 5}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("downstream should not be called")
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/shipments/from-transport-order", strings.NewReader(`{}`))
	rec := serveThroughAuth(t, guard.WithPolicy(PolicyCreate), req, "secret", true)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", rec.Code)
	}
}

func TestUpdateStatusMultiRoleFinanceAndCarrierAllows(t *testing.T) {
	tenantID := "11111111-1111-1111-1111-111111111111"
	identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"roles": []string{"FINANCE_MANAGER", "CARRIER_DISPATCHER"}})
	}))
	defer identityServer.Close()

	downstreamCalled := false
	guard := newTestGuard(t, identityServer.URL, func(w http.ResponseWriter, r *http.Request) {
		downstreamCalled = true
		w.WriteHeader(http.StatusOK)
	})

	token := signTestToken(t, "secret", "user-1", tenantID, "user@example.com")
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/shipments/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/status", strings.NewReader(`{"status":"IN_TRANSIT"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	rec := serveThroughAuth(t, guard.WithPolicy(PolicyUpdateStatus), req, "secret", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !downstreamCalled {
		t.Fatal("downstream should be called when carrier role is present")
	}
}

func TestAcceptMultiRoleFinanceAndCarrierAllows(t *testing.T) {
	tenantID := "11111111-1111-1111-1111-111111111111"
	identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"roles": []string{"FINANCE_MANAGER", "CARRIER_DISPATCHER"}})
	}))
	defer identityServer.Close()

	downstreamCalled := false
	guard := newTestGuard(t, identityServer.URL, func(w http.ResponseWriter, r *http.Request) {
		downstreamCalled = true
		w.WriteHeader(http.StatusOK)
	})

	token := signTestToken(t, "secret", "user-1", tenantID, "user@example.com")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/shipments/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/accept", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rec := serveThroughAuth(t, guard.WithPolicy(PolicyAccept), req, "secret", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !downstreamCalled {
		t.Fatal("downstream should be called when carrier role is present")
	}
}

func TestCreateMultiRoleFinanceAndCarrierDenied(t *testing.T) {
	tenantID := "11111111-1111-1111-1111-111111111111"
	identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"roles": []string{"FINANCE_MANAGER", "CARRIER_DISPATCHER"}})
	}))
	defer identityServer.Close()

	downstreamCalled := false
	guard := newTestGuard(t, identityServer.URL, func(w http.ResponseWriter, r *http.Request) {
		downstreamCalled = true
	})

	token := signTestToken(t, "secret", "user-1", tenantID, "user@example.com")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/shipments/from-transport-order", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+token)

	rec := serveThroughAuth(t, guard.WithPolicy(PolicyCreate), req, "secret", true)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d want 403", rec.Code)
	}
	if downstreamCalled {
		t.Fatal("downstream must not be called on 403")
	}
}

func TestSpoofedRoleHeaderDenied(t *testing.T) {
	tenantID := "11111111-1111-1111-1111-111111111111"
	userID := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	downstreamCalled := false

	identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"roles": []string{"FINANCE_MANAGER"}})
	}))
	defer identityServer.Close()

	var gotTenant, gotUser string
	guard := newTestGuard(t, identityServer.URL, func(w http.ResponseWriter, r *http.Request) {
		downstreamCalled = true
		gotTenant = r.Header.Get("X-Tenant-ID")
		gotUser = r.Header.Get("X-User-ID")
		if roles := r.Header.Get("X-User-Roles"); roles != "" {
			t.Fatalf("X-User-Roles must be stripped, got %q", roles)
		}
	})

	token := signTestToken(t, "secret", userID, tenantID, "finance@example.com")
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/shipments/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/status", strings.NewReader(`{"status":"IN_TRANSIT"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-Roles", "PLATFORM_ADMIN")
	req.Header.Set("X-Tenant-ID", "22222222-2222-2222-2222-222222222222")
	req.Header.Set("X-User-ID", "spoofed-user")

	rec := serveThroughAuth(t, guard.WithPolicy(PolicyUpdateStatus), req, "secret", true)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d want 403 body=%s", rec.Code, rec.Body.String())
	}
	if downstreamCalled {
		t.Fatal("downstream must not be called on 403")
	}
	if gotTenant != "" || gotUser != "" {
		t.Fatalf("downstream must not receive headers when denied tenant=%q user=%q", gotTenant, gotUser)
	}
}

func TestAllowedRoleForwardsTrustedHeaders(t *testing.T) {
	tenantID := "11111111-1111-1111-1111-111111111111"
	userID := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	requestID := "req-shipment-123"

	identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"roles": []string{"CARRIER_ADMIN"}})
	}))
	defer identityServer.Close()

	var gotTenant, gotUser, gotRequestID string
	guard := newTestGuard(t, identityServer.URL, func(w http.ResponseWriter, r *http.Request) {
		gotTenant = r.Header.Get("X-Tenant-ID")
		gotUser = r.Header.Get("X-User-ID")
		gotRequestID = r.Header.Get(sharedmiddleware.RequestIDHeader)
		w.WriteHeader(http.StatusOK)
	})

	token := signTestToken(t, "secret", userID, tenantID, "carrier@example.com")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/shipments/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/accept", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set(sharedmiddleware.RequestIDHeader, requestID)
	req.Header.Set("X-Tenant-ID", "spoofed-tenant")
	req.Header.Set("X-User-ID", "spoofed-user")

	rec := serveThroughAuth(t, guard.WithPolicy(PolicyAccept), req, "secret", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if gotTenant != tenantID || gotUser != userID {
		t.Fatalf("trusted headers tenant=%q user=%q want jwt values", gotTenant, gotUser)
	}
	if gotRequestID != requestID {
		t.Fatalf("request id=%q want %q", gotRequestID, requestID)
	}
}

func TestIdentityUnavailableReturns503(t *testing.T) {
	tenantID := "11111111-1111-1111-1111-111111111111"
	downstreamCalled := false

	identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer identityServer.Close()

	guard := newTestGuard(t, identityServer.URL, func(w http.ResponseWriter, r *http.Request) {
		downstreamCalled = true
	})

	token := signTestToken(t, "secret", "user-1", tenantID, "user@example.com")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/shipments/from-transport-order", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+token)

	rec := serveThroughAuth(t, guard.WithPolicy(PolicyCreate), req, "secret", true)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d want 503 body=%s", rec.Code, rec.Body.String())
	}
	body, _ := io.ReadAll(rec.Body)
	if !strings.Contains(string(body), "AUTH_DEPENDENCY_UNAVAILABLE") {
		t.Fatalf("expected AUTH_DEPENDENCY_UNAVAILABLE, body=%s", string(body))
	}
	if downstreamCalled {
		t.Fatal("downstream must not be called when identity is unavailable")
	}
}

func TestIdentityForbiddenReturns403(t *testing.T) {
	tenantID := "11111111-1111-1111-1111-111111111111"
	downstreamCalled := false

	identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer identityServer.Close()

	guard := newTestGuard(t, identityServer.URL, func(w http.ResponseWriter, r *http.Request) {
		downstreamCalled = true
	})

	token := signTestToken(t, "secret", "user-1", tenantID, "user@example.com")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/shipments/from-transport-order", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+token)

	rec := serveThroughAuth(t, guard.WithPolicy(PolicyCreate), req, "secret", true)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d want 403", rec.Code)
	}
	if downstreamCalled {
		t.Fatal("downstream must not be called when identity returns 403")
	}
}
