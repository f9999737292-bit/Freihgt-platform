package bnorbac

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/api-gateway/internal/config"
)

func TestI2C_SEC_01_unauthenticated_post_401(t *testing.T) {
	guard := NewGuard(config.Config{AuthEnabled: true, ProxyTimeoutSeconds: 5}, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("downstream must not be called")
	}))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/network/chain-searches", strings.NewReader(`{"seed_type":"MANUAL_CAPACITY"}`))
	rec := serve(t, guard.WithPolicy(PolicySearchChain), req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestI2C_SEC_02_shipper_role_403(t *testing.T) {
	assertChainRole(t, "SHIPPER", "SHIPPER_ADMIN", nil, http.StatusForbidden, false)
}

func TestI2C_SEC_03_forwarder_role_403(t *testing.T) {
	assertChainRole(t, "FORWARDER", "FORWARDER_MANAGER", nil, http.StatusForbidden, false)
}

func TestI2C_SEC_04_carrier_admin_allowed(t *testing.T) {
	assertChainRole(t, "CARRIER", "CARRIER_ADMIN", nil, http.StatusOK, true)
}

func TestI2C_SEC_05_carrier_dispatcher_allowed(t *testing.T) {
	assertChainRole(t, "CARRIER", "CARRIER_DISPATCHER", nil, http.StatusOK, true)
}

func TestI2C_SEC_06_platform_admin_allowed(t *testing.T) {
	assertChainRole(t, "SHIPPER", "SHIPPER_ADMIN", []string{"PLATFORM_ADMIN"}, http.StatusOK, true)
}

func TestI2C_SEC_07_spoof_tenant_cannot_change_tenant(t *testing.T) {
	tenantID := uuid.NewString()
	userID := uuid.NewString()
	companyID := uuid.NewString()
	identity := identityServer(t, companyID, "CARRIER", []string{"CARRIER_ADMIN"}, nil)
	defer identity.Close()
	var gotTenant, gotUser string
	guard := NewGuard(config.Config{AuthEnabled: true, ProxyTimeoutSeconds: 5, Services: config.ServiceURLs{Identity: identity.URL}}, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotTenant = r.Header.Get("X-Tenant-ID")
		gotUser = r.Header.Get("X-User-ID")
	}))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/network/chain-searches", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+signToken(t, "secret", userID, tenantID))
	req.Header.Set("X-Company-ID", companyID)
	req.Header.Set("X-Tenant-ID", uuid.NewString())
	req.Header.Set("X-User-ID", uuid.NewString())
	req.Header.Set("X-Role", "PLATFORM_ADMIN")
	rec := serve(t, guard.WithPolicy(PolicySearchChain), req)
	if rec.Code != http.StatusOK || gotTenant != tenantID || gotUser != userID {
		t.Fatalf("status=%d tenant=%s user=%s body=%s", rec.Code, gotTenant, gotUser, rec.Body.String())
	}
}

func TestI2C_SEC_08_spoof_user_cannot_become_authority(t *testing.T) {
	TestI2C_SEC_07_spoof_tenant_cannot_change_tenant(t)
}

func TestI2C_SEC_09_foreign_company_membership_403(t *testing.T) {
	tenantID := uuid.NewString()
	userID := uuid.NewString()
	companyID := uuid.NewString()
	identity := identityServer(t, companyID, "CARRIER", []string{"CARRIER_ADMIN"}, nil)
	defer identity.Close()
	called := false
	guard := NewGuard(config.Config{AuthEnabled: true, ProxyTimeoutSeconds: 5, Services: config.ServiceURLs{Identity: identity.URL}}, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/network/chain-searches/"+uuid.NewString(), nil)
	req.Header.Set("Authorization", "Bearer "+signToken(t, "secret", userID, tenantID))
	req.Header.Set("X-Company-ID", uuid.NewString())
	rec := serve(t, guard.WithPolicy(PolicySearchChain), req)
	if rec.Code != http.StatusForbidden || called {
		t.Fatalf("status=%d called=%v body=%s", rec.Code, called, rec.Body.String())
	}
}

func TestI2C_GW_04_rbac_matches_carrier_search_roles(t *testing.T) {
	for _, role := range []string{"CARRIER_ADMIN", "CARRIER_DISPATCHER"} {
		if !policyAllows(PolicySearchChain, []string{role}, "CARRIER", false) {
			t.Fatalf("%s denied", role)
		}
		if !policyAllows(PolicySearchNextLoad, []string{role}, "CARRIER", false) {
			t.Fatalf("next-load %s denied", role)
		}
	}
	for _, role := range []string{"SHIPPER_ADMIN", "SHIPPER_LOGIST", "FORWARDER_MANAGER"} {
		if policyAllows(PolicySearchChain, []string{role}, "BUYER", false) || policyAllows(PolicySearchChain, []string{role}, "CARRIER", false) {
			t.Fatalf("%s allowed", role)
		}
	}
	if !policyAllows(PolicySearchChain, nil, "BUYER", true) {
		t.Fatal("platform admin denied")
	}
}

func assertChainRole(t *testing.T, companyType, role string, tenantRoles []string, want int, wantCalled bool) {
	t.Helper()
	tenantID := uuid.NewString()
	userID := uuid.NewString()
	companyID := uuid.NewString()
	identity := identityServer(t, companyID, companyType, []string{role}, tenantRoles)
	defer identity.Close()
	called := false
	guard := NewGuard(config.Config{AuthEnabled: true, ProxyTimeoutSeconds: 5, Services: config.ServiceURLs{Identity: identity.URL}}, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/network/chain-searches", strings.NewReader(`{"seed_type":"MANUAL_CAPACITY"}`))
	req.Header.Set("Authorization", "Bearer "+signToken(t, "secret", userID, tenantID))
	req.Header.Set("X-Company-ID", companyID)
	rec := serve(t, guard.WithPolicy(PolicySearchChain), req)
	if rec.Code != want || called != wantCalled {
		t.Fatalf("role=%s status=%d called=%v body=%s", role, rec.Code, called, rec.Body.String())
	}
}
