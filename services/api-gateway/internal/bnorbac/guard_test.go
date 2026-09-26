package bnorbac

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/api-gateway/internal/config"
	gwmiddleware "github.com/freight-platform/api-gateway/internal/http/middleware"
)

func TestBNO14UnauthenticatedRejected(t *testing.T) {
	guard := NewGuard(config.Config{AuthEnabled: true, ProxyTimeoutSeconds: 5}, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("downstream must not be called")
	}))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/network/load-opportunities", strings.NewReader(`{}`))
	req.Header.Set("X-Tenant-ID", uuid.NewString())
	req.Header.Set("X-User-ID", uuid.NewString())
	rec := httptest.NewRecorder()
	guard.WithPolicy(PolicyPublishLoad).ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("BNO14 status=%d", rec.Code)
	}
}

func TestBNO15CarrierCannotPublishLoad(t *testing.T) {
	tenantID := uuid.NewString()
	userID := uuid.NewString()
	companyID := uuid.NewString()
	identity := identityServer(t, companyID, "CARRIER", []string{"CARRIER_ADMIN"}, nil)
	defer identity.Close()
	called := false
	guard := NewGuard(config.Config{AuthEnabled: true, ProxyTimeoutSeconds: 5, Services: config.ServiceURLs{Identity: identity.URL}}, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/network/load-opportunities", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+signToken(t, "secret", userID, tenantID))
	req.Header.Set("X-Company-ID", companyID)
	req.Header.Set("X-Tenant-ID", uuid.NewString())
	rec := serve(t, guard.WithPolicy(PolicyPublishLoad), req)
	if rec.Code != http.StatusForbidden || called {
		t.Fatalf("BNO15 status=%d called=%v", rec.Code, called)
	}
}

func TestBNO13SpoofedIdentityIgnored(t *testing.T) {
	tenantID := uuid.NewString()
	userID := uuid.NewString()
	companyID := uuid.NewString()
	identity := identityServer(t, companyID, "SHIPPER", []string{"SHIPPER_ADMIN"}, nil)
	defer identity.Close()
	var gotTenant, gotUser, gotEmail, gotAdmin, gotToken, gotPrincipal, gotActor string
	guard := NewGuard(config.Config{AuthEnabled: true, ProxyTimeoutSeconds: 5, Services: config.ServiceURLs{Identity: identity.URL}}, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotTenant = r.Header.Get("X-Tenant-ID")
		gotUser = r.Header.Get("X-User-ID")
		gotEmail = r.Header.Get("X-User-Email")
		gotAdmin = r.Header.Get("X-Platform-Admin")
		gotToken = r.Header.Get("X-Internal-Service-Token")
		gotPrincipal = r.Header.Get("X-Integration-Principal-ID")
		gotActor = r.Header.Get("X-Actor-Kind")
	}))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/network/load-opportunities", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+signToken(t, "secret", userID, tenantID))
	req.Header.Set("X-Company-ID", companyID)
	req.Header.Set("X-Tenant-ID", "spoofed-tenant")
	req.Header.Set("X-User-ID", "spoofed-user")
	req.Header.Set("X-User-Email", "spoofed@example.com")
	req.Header.Set("X-Platform-Admin", "true")
	req.Header.Set("X-Internal-Service-Token", "client-forged-token")
	req.Header.Set("X-Integration-Principal-ID", uuid.NewString())
	req.Header.Set("X-Integration-Scopes", "rfx:draft:read")
	req.Header.Set("X-Integration-Auth-Scheme", "OAUTH")
	req.Header.Set("X-Actor-Kind", "INTEGRATION")
	rec := serve(t, guard.WithPolicy(PolicyPublishLoad), req)
	if rec.Code != http.StatusOK {
		t.Fatalf("BNO13 status=%d body=%s", rec.Code, rec.Body.String())
	}
	if gotTenant != tenantID || gotUser != userID || gotEmail != "" || gotAdmin != "" || gotToken != "" || gotPrincipal != "" || gotActor != "BUYER" {
		t.Fatalf("spoof reached downstream tenant=%s user=%s email=%s admin=%s token=%s principal=%s actor=%s", gotTenant, gotUser, gotEmail, gotAdmin, gotToken, gotPrincipal, gotActor)
	}
}

func TestShipperCanPublishLoad(t *testing.T) {
	tenantID := uuid.NewString()
	userID := uuid.NewString()
	companyID := uuid.NewString()
	identity := identityServer(t, companyID, "SHIPPER", []string{"SHIPPER_LOGIST"}, nil)
	defer identity.Close()
	called := false
	guard := NewGuard(config.Config{AuthEnabled: true, ProxyTimeoutSeconds: 5, Services: config.ServiceURLs{Identity: identity.URL}}, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/network/marketplace/capacities", nil)
	req.Header.Set("Authorization", "Bearer "+signToken(t, "secret", userID, tenantID))
	req.Header.Set("X-Company-ID", companyID)
	rec := serve(t, guard.WithPolicy(PolicyViewMarketplaceCapacities), req)
	if rec.Code != http.StatusOK || !called {
		t.Fatalf("shipper marketplace capacity status=%d called=%v body=%s", rec.Code, called, rec.Body.String())
	}
}

func TestCarrierCanSearchNextLoad(t *testing.T) {
	tenantID := uuid.NewString()
	userID := uuid.NewString()
	companyID := uuid.NewString()
	identity := identityServer(t, companyID, "CARRIER", []string{"CARRIER_DISPATCHER"}, nil)
	defer identity.Close()
	called := false
	guard := NewGuard(config.Config{AuthEnabled: true, ProxyTimeoutSeconds: 5, Services: config.ServiceURLs{Identity: identity.URL}}, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/network/next-load/search", strings.NewReader(`{"capacity_id":"`+uuid.NewString()+`"}`))
	req.Header.Set("Authorization", "Bearer "+signToken(t, "secret", userID, tenantID))
	req.Header.Set("X-Company-ID", companyID)
	req.Header.Set("X-Tenant-ID", "spoofed-tenant")
	rec := serve(t, guard.WithPolicy(PolicySearchNextLoad), req)
	if rec.Code != http.StatusOK || !called {
		t.Fatalf("carrier search status=%d called=%v body=%s", rec.Code, called, rec.Body.String())
	}
}

func TestNLO03BCarrierConsolidationSearch(t *testing.T) {
	for _, tc := range []struct{ name, role string }{
		{"NLO03B_084_GATEWAY_CARRIER_ADMIN_ALLOWED", "CARRIER_ADMIN"},
		{"NLO03B_085_GATEWAY_CARRIER_DISPATCHER_ALLOWED", "CARRIER_DISPATCHER"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			tenantID := uuid.NewString()
			userID := uuid.NewString()
			companyID := uuid.NewString()
			identity := identityServer(t, companyID, "CARRIER", []string{tc.role}, nil)
			defer identity.Close()
			var gotToken, gotTenant, gotUser string
			guard := NewGuard(config.Config{AuthEnabled: true, ProxyTimeoutSeconds: 5, Services: config.ServiceURLs{Identity: identity.URL}}, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				gotToken = r.Header.Get("X-Internal-Service-Token")
				gotTenant = r.Header.Get("X-Tenant-ID")
				gotUser = r.Header.Get("X-User-ID")
			}))
			req := httptest.NewRequest(http.MethodPost, "/api/v1/network/consolidation/search", strings.NewReader(`{"capacity_id":"`+uuid.NewString()+`","pattern":"SAME_ORIGIN_SAME_DESTINATION"}`))
			req.Header.Set("Authorization", "Bearer "+signToken(t, "secret", userID, tenantID))
			req.Header.Set("X-Company-ID", companyID)
			req.Header.Set("X-Tenant-ID", "spoofed-tenant")
			req.Header.Set("X-User-ID", "spoofed-user")
			req.Header.Set("X-Internal-Service-Token", "client-forged-token")
			rec := serve(t, guard.WithPolicy(PolicySearchConsolidation), req)
			if rec.Code != http.StatusOK || gotToken != "" || gotTenant != tenantID || gotUser != userID {
				t.Fatalf("NLO03B_087 %s status=%d token=%q tenant=%s user=%s body=%s", tc.role, rec.Code, gotToken, gotTenant, gotUser, rec.Body.String())
			}
		})
	}
}

func TestNLO03B_086_GATEWAY_SHIPPER_DENIED(t *testing.T) {
	tenantID := uuid.NewString()
	userID := uuid.NewString()
	companyID := uuid.NewString()
	identity := identityServer(t, companyID, "SHIPPER", []string{"SHIPPER_ADMIN"}, nil)
	defer identity.Close()
	called := false
	guard := NewGuard(config.Config{AuthEnabled: true, ProxyTimeoutSeconds: 5, Services: config.ServiceURLs{Identity: identity.URL}}, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/network/consolidation/search", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+signToken(t, "secret", userID, tenantID))
	req.Header.Set("X-Company-ID", companyID)
	rec := serve(t, guard.WithPolicy(PolicySearchConsolidation), req)
	if rec.Code != http.StatusForbidden || called {
		t.Fatalf("shipper status=%d called=%v body=%s", rec.Code, called, rec.Body.String())
	}
}

func TestShipperCannotSearchNextLoad(t *testing.T) {
	tenantID := uuid.NewString()
	userID := uuid.NewString()
	companyID := uuid.NewString()
	identity := identityServer(t, companyID, "SHIPPER", []string{"SHIPPER_ADMIN"}, nil)
	defer identity.Close()
	called := false
	guard := NewGuard(config.Config{AuthEnabled: true, ProxyTimeoutSeconds: 5, Services: config.ServiceURLs{Identity: identity.URL}}, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/network/next-load/search", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+signToken(t, "secret", userID, tenantID))
	req.Header.Set("X-Company-ID", companyID)
	rec := serve(t, guard.WithPolicy(PolicySearchNextLoad), req)
	if rec.Code != http.StatusForbidden || called {
		t.Fatalf("shipper search status=%d called=%v body=%s", rec.Code, called, rec.Body.String())
	}
}

func TestBNO43ShipperCannotActivatePrediction(t *testing.T) {
	tenantID := uuid.NewString()
	userID := uuid.NewString()
	companyID := uuid.NewString()
	identity := identityServer(t, companyID, "SHIPPER", []string{"SHIPPER_ADMIN"}, nil)
	defer identity.Close()
	called := false
	guard := NewGuard(config.Config{AuthEnabled: true, ProxyTimeoutSeconds: 5, Services: config.ServiceURLs{Identity: identity.URL}}, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/network/predicted-capacities/"+uuid.NewString()+"/activate", strings.NewReader(`{"version":1}`))
	req.Header.Set("Authorization", "Bearer "+signToken(t, "secret", userID, tenantID))
	req.Header.Set("X-Company-ID", companyID)
	req.Header.Set("X-Internal-Service-Token", "client-forged-token")
	rec := serve(t, guard.WithPolicy(PolicyPublishCapacity), req)
	if rec.Code != http.StatusForbidden || called {
		t.Fatalf("BNO43 status=%d called=%v body=%s", rec.Code, called, rec.Body.String())
	}
}

func TestBNO52PredictionRouteStripsForgedInternalToken(t *testing.T) {
	tenantID := uuid.NewString()
	userID := uuid.NewString()
	companyID := uuid.NewString()
	identity := identityServer(t, companyID, "CARRIER", []string{"CARRIER_DISPATCHER"}, nil)
	defer identity.Close()
	var gotToken, gotTenant string
	guard := NewGuard(config.Config{AuthEnabled: true, ProxyTimeoutSeconds: 5, Services: config.ServiceURLs{Identity: identity.URL}}, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotToken = r.Header.Get("X-Internal-Service-Token")
		gotTenant = r.Header.Get("X-Tenant-ID")
	}))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/network/shipments/"+uuid.NewString()+"/predicted-capacity", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+signToken(t, "secret", userID, tenantID))
	req.Header.Set("X-Company-ID", companyID)
	req.Header.Set("X-Tenant-ID", "spoofed-tenant")
	req.Header.Set("X-Internal-Service-Token", "client-forged-token")
	rec := serve(t, guard.WithPolicy(PolicyPublishCapacity), req)
	if rec.Code != http.StatusOK || gotToken != "" || gotTenant != tenantID {
		t.Fatalf("BNO52 status=%d token=%q tenant=%s body=%s", rec.Code, gotToken, gotTenant, rec.Body.String())
	}
}

func serve(t *testing.T, handler http.Handler, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	gwmiddleware.Auth(true, "secret")(handler).ServeHTTP(rec, req)
	return rec
}

func identityServer(t *testing.T, companyID, companyType string, roles, tenantRoles []string) *httptest.Server {
	t.Helper()
	roleItems := make([]map[string]string, 0, len(roles))
	for _, role := range roles {
		roleItems = append(roleItems, map[string]string{"code": role})
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/companies"):
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []map[string]any{{
				"company_id": companyID, "company_type": companyType, "roles": roleItems,
			}}})
		case strings.HasSuffix(r.URL.Path, "/roles"):
			items := make([]map[string]any, 0, len(tenantRoles))
			for _, role := range tenantRoles {
				items = append(items, map[string]any{"code": role})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func signToken(t *testing.T, secret, userID, tenantID string) string {
	t.Helper()
	claims := jwt.MapClaims{"tenant_id": tenantID, "email": "user@example.com", "sub": userID, "exp": time.Now().Add(time.Hour).Unix()}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	return signed
}
