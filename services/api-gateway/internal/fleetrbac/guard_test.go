package fleetrbac

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

func TestViewRBACTable(t *testing.T) {
	tenantID := "11111111-1111-1111-1111-111111111111"
	driverID := "dddddddd-dddd-dddd-dddd-dddddddddddd"
	paths := []string{
		"/api/v1/drivers",
		"/api/v1/drivers/" + driverID,
		"/api/v1/vehicles",
		"/api/v1/vehicles/" + driverID,
	}

	tests := []struct {
		name       string
		roles      []string
		wantStatus int
	}{
		{"PLATFORM_ADMIN", []string{"PLATFORM_ADMIN"}, http.StatusOK},
		{"CARRIER_ADMIN", []string{"CARRIER_ADMIN"}, http.StatusOK},
		{"CARRIER_DISPATCHER", []string{"CARRIER_DISPATCHER"}, http.StatusOK},
		{"FINANCE_MANAGER", []string{"FINANCE_MANAGER"}, http.StatusForbidden},
		{"PROCUREMENT_MANAGER", []string{"PROCUREMENT_MANAGER"}, http.StatusForbidden},
		{"SHIPPER_LOGIST", []string{"SHIPPER_LOGIST"}, http.StatusForbidden},
		{"unknown role", []string{"UNKNOWN_ROLE"}, http.StatusForbidden},
		{"no role", nil, http.StatusForbidden},
	}

	for _, path := range paths {
		for _, tt := range tests {
			t.Run(path+"_"+tt.name, func(t *testing.T) {
				downstreamCalled := false
				identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_ = json.NewEncoder(w).Encode(map[string]any{"roles": tt.roles})
				}))
				defer identityServer.Close()

				guard := NewGuard(config.Config{
					AuthEnabled:         true,
					ProxyTimeoutSeconds: 5,
					Services:            config.ServiceURLs{Identity: identityServer.URL},
				}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					downstreamCalled = true
					w.WriteHeader(http.StatusOK)
				}))

				token := signTestToken(t, "secret", "user-1", tenantID, "user@example.com")
				req := httptest.NewRequest(http.MethodGet, path, nil)
				req.Header.Set("Authorization", "Bearer "+token)

				rec := serveThroughAuth(t, guard.WithPolicy(PolicyView), req, "secret", true)
				if rec.Code != tt.wantStatus {
					t.Fatalf("status=%d want=%d body=%s", rec.Code, tt.wantStatus, rec.Body.String())
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
}

func TestViewUnauthenticatedReturns401(t *testing.T) {
	guard := NewGuard(config.Config{AuthEnabled: true, ProxyTimeoutSeconds: 5}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("downstream should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/drivers", nil)
	rec := serveThroughAuth(t, guard.WithPolicy(PolicyView), req, "secret", true)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", rec.Code)
	}
}

func TestCreateRBACTable(t *testing.T) {
	tenantID := "11111111-1111-1111-1111-111111111111"
	paths := []string{"/api/v1/drivers", "/api/v1/vehicles"}

	tests := []struct {
		name       string
		roles      []string
		wantStatus int
	}{
		{"PLATFORM_ADMIN", []string{"PLATFORM_ADMIN"}, http.StatusCreated},
		{"CARRIER_ADMIN", []string{"CARRIER_ADMIN"}, http.StatusCreated},
		{"CARRIER_DISPATCHER", []string{"CARRIER_DISPATCHER"}, http.StatusForbidden},
		{"FINANCE_MANAGER", []string{"FINANCE_MANAGER"}, http.StatusForbidden},
		{"PROCUREMENT_MANAGER", []string{"PROCUREMENT_MANAGER"}, http.StatusForbidden},
		{"unknown", []string{"UNKNOWN_ROLE"}, http.StatusForbidden},
	}

	for _, path := range paths {
		for _, tt := range tests {
			t.Run(path+"_"+tt.name, func(t *testing.T) {
				downstreamCalled := false
				identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_ = json.NewEncoder(w).Encode(map[string]any{"roles": tt.roles})
				}))
				defer identityServer.Close()

				guard := NewGuard(config.Config{
					AuthEnabled:         true,
					ProxyTimeoutSeconds: 5,
					Services:            config.ServiceURLs{Identity: identityServer.URL},
				}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					downstreamCalled = true
					w.WriteHeader(http.StatusCreated)
				}))

				token := signTestToken(t, "secret", "user-1", tenantID, "user@example.com")
				req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"carrier_company_id":"cccccccc-cccc-cccc-cccc-cccccccccccc","full_name":"Test"}`))
				req.Header.Set("Authorization", "Bearer "+token)
				req.Header.Set("Content-Type", "application/json")

				rec := serveThroughAuth(t, guard.WithPolicy(PolicyCreate), req, "secret", true)
				if rec.Code != tt.wantStatus {
					t.Fatalf("status=%d want=%d body=%s", rec.Code, tt.wantStatus, rec.Body.String())
				}
				if tt.wantStatus == http.StatusForbidden && downstreamCalled {
					t.Fatal("downstream must not be called on 403")
				}
			})
		}
	}
}

func TestCreateUnauthenticatedReturns401(t *testing.T) {
	guard := NewGuard(config.Config{AuthEnabled: true, ProxyTimeoutSeconds: 5}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("downstream should not be called")
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/drivers", strings.NewReader(`{}`))
	rec := serveThroughAuth(t, guard.WithPolicy(PolicyCreate), req, "secret", true)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", rec.Code)
	}
}

func TestAssignRBACTable(t *testing.T) {
	tenantID := "11111111-1111-1111-1111-111111111111"
	shipmentID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	paths := []string{
		"/api/v1/shipments/" + shipmentID + "/assign-driver",
		"/api/v1/shipments/" + shipmentID + "/assign-vehicle",
	}

	tests := []struct {
		name       string
		roles      []string
		wantStatus int
	}{
		{"PLATFORM_ADMIN", []string{"PLATFORM_ADMIN"}, http.StatusOK},
		{"CARRIER_ADMIN", []string{"CARRIER_ADMIN"}, http.StatusOK},
		{"CARRIER_DISPATCHER", []string{"CARRIER_DISPATCHER"}, http.StatusOK},
		{"FINANCE_MANAGER", []string{"FINANCE_MANAGER"}, http.StatusForbidden},
		{"PROCUREMENT_MANAGER", []string{"PROCUREMENT_MANAGER"}, http.StatusForbidden},
		{"SHIPPER_LOGIST", []string{"SHIPPER_LOGIST"}, http.StatusForbidden},
		{"unknown", []string{"UNKNOWN_ROLE"}, http.StatusForbidden},
	}

	for _, path := range paths {
		for _, tt := range tests {
			t.Run(path+"_"+tt.name, func(t *testing.T) {
				downstreamCalled := false
				identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_ = json.NewEncoder(w).Encode(map[string]any{"roles": tt.roles})
				}))
				defer identityServer.Close()

				guard := NewGuard(config.Config{
					AuthEnabled:         true,
					ProxyTimeoutSeconds: 5,
					Services:            config.ServiceURLs{Identity: identityServer.URL},
				}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					downstreamCalled = true
					w.WriteHeader(http.StatusOK)
				}))

				token := signTestToken(t, "secret", "user-1", tenantID, "user@example.com")
				req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"driver_id":"dddddddd-dddd-dddd-dddd-dddddddddddd"}`))
				req.Header.Set("Authorization", "Bearer "+token)
				req.Header.Set("Content-Type", "application/json")

				rec := serveThroughAuth(t, guard.WithPolicy(PolicyAssign), req, "secret", true)
				if rec.Code != tt.wantStatus {
					t.Fatalf("status=%d want=%d body=%s", rec.Code, tt.wantStatus, rec.Body.String())
				}
				if tt.wantStatus == http.StatusForbidden && downstreamCalled {
					t.Fatal("downstream must not be called on 403")
				}
			})
		}
	}
}

func TestAssignAllowedForwardsTrustedHeaders(t *testing.T) {
	tenantID := "11111111-1111-1111-1111-111111111111"
	userID := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	requestID := "req-fleet-123"
	shipmentID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"

	var gotTenant, gotUser, gotAuth, gotRequestID string
	identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"roles": []string{"CARRIER_DISPATCHER"}})
	}))
	defer identityServer.Close()

	guard := NewGuard(config.Config{
		AuthEnabled:         true,
		ProxyTimeoutSeconds: 5,
		Services:            config.ServiceURLs{Identity: identityServer.URL},
	}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTenant = r.Header.Get("X-Tenant-ID")
		gotUser = r.Header.Get("X-User-ID")
		gotAuth = r.Header.Get("Authorization")
		gotRequestID = r.Header.Get(sharedmiddleware.RequestIDHeader)
		w.WriteHeader(http.StatusOK)
	}))

	token := signTestToken(t, "secret", userID, tenantID, "dispatcher@example.com")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/shipments/"+shipmentID+"/assign-driver", strings.NewReader(`{"driver_id":"dddddddd-dddd-dddd-dddd-dddddddddddd"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(sharedmiddleware.RequestIDHeader, requestID)

	rec := serveThroughAuth(t, guard.WithPolicy(PolicyAssign), req, "secret", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if gotTenant != tenantID {
		t.Fatalf("tenant=%q want %q", gotTenant, tenantID)
	}
	if gotUser != userID {
		t.Fatalf("user=%q want %q", gotUser, userID)
	}
	if !strings.HasPrefix(gotAuth, "Bearer ") {
		t.Fatalf("authorization not forwarded: %q", gotAuth)
	}
	if gotRequestID != requestID {
		t.Fatalf("request id=%q want %q", gotRequestID, requestID)
	}
}

func TestSpoofedUserRolesHeaderDenied(t *testing.T) {
	tenantID := "11111111-1111-1111-1111-111111111111"
	downstreamCalled := false

	identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"roles": []string{"FINANCE_MANAGER"}})
	}))
	defer identityServer.Close()

	guard := NewGuard(config.Config{
		AuthEnabled:         true,
		ProxyTimeoutSeconds: 5,
		Services:            config.ServiceURLs{Identity: identityServer.URL},
	}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downstreamCalled = true
		if roles := r.Header.Get("X-User-Roles"); roles != "" {
			t.Fatalf("X-User-Roles must be stripped, got %q", roles)
		}
		w.WriteHeader(http.StatusOK)
	}))

	token := signTestToken(t, "secret", "finance-user", tenantID, "finance@example.com")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/drivers", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-User-Roles", "PLATFORM_ADMIN")
	req.Header.Set("X-Tenant-ID", "22222222-2222-2222-2222-222222222222")
	req.Header.Set("X-User-ID", "spoofed-user")
	req.Header.Set("X-User-Email", "spoof@example.com")
	req.Header.Set("X-Company-ID", "cccccccc-cccc-cccc-cccc-cccccccccccc")

	rec := serveThroughAuth(t, guard.WithPolicy(PolicyView), req, "secret", true)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d want 403 body=%s", rec.Code, rec.Body.String())
	}
	if downstreamCalled {
		t.Fatal("downstream must not be called on 403")
	}
}

func TestIdentityUnavailableReturns503(t *testing.T) {
	tenantID := "11111111-1111-1111-1111-111111111111"
	downstreamCalled := false

	identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer identityServer.Close()

	guard := NewGuard(config.Config{
		AuthEnabled:         true,
		ProxyTimeoutSeconds: 5,
		Services:            config.ServiceURLs{Identity: identityServer.URL},
	}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downstreamCalled = true
	}))

	token := signTestToken(t, "secret", "user-1", tenantID, "user@example.com")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/drivers", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rec := serveThroughAuth(t, guard.WithPolicy(PolicyView), req, "secret", true)
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

func TestAssignUnauthenticatedReturns401(t *testing.T) {
	guard := NewGuard(config.Config{AuthEnabled: true, ProxyTimeoutSeconds: 5}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("downstream should not be called")
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/shipments/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/assign-driver", strings.NewReader(`{}`))
	rec := serveThroughAuth(t, guard.WithPolicy(PolicyAssign), req, "secret", true)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", rec.Code)
	}
}
