package routeauth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/freight-platform/api-gateway/internal/routeauth"
)

func TestFetchUserRolesReturnsRoles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/auth/me" {
			t.Fatalf("path=%s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer token" {
			t.Fatalf("authorization=%q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"roles": []string{"CARRIER_ADMIN"}})
	}))
	defer server.Close()

	client := routeauth.NewIdentityClient(server.Client(), server.URL)
	roles, err := client.FetchUserRoles(context.Background(), routeauth.RequestContext{
		AuthToken: "Bearer token",
		TenantID:  "tenant-a",
		UserID:    "user-a",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(roles) != 1 || roles[0] != "CARRIER_ADMIN" {
		t.Fatalf("roles=%v", roles)
	}
}

func TestFetchUserRolesUnauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := routeauth.NewIdentityClient(server.Client(), server.URL)
	_, err := client.FetchUserRoles(context.Background(), routeauth.RequestContext{AuthToken: "Bearer token"})
	if err == nil || err != routeauth.ErrIdentityUnauthorized {
		t.Fatalf("expected ErrIdentityUnauthorized, got %v", err)
	}
}

func TestFetchUserRolesForbidden(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	client := routeauth.NewIdentityClient(server.Client(), server.URL)
	_, err := client.FetchUserRoles(context.Background(), routeauth.RequestContext{AuthToken: "Bearer token"})
	if err == nil || err != routeauth.ErrIdentityForbidden {
		t.Fatalf("expected ErrIdentityForbidden, got %v", err)
	}
}

func TestFetchUserRolesMalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{invalid"))
	}))
	defer server.Close()

	client := routeauth.NewIdentityClient(server.Client(), server.URL)
	_, err := client.FetchUserRoles(context.Background(), routeauth.RequestContext{AuthToken: "Bearer token"})
	if err == nil {
		t.Fatal("expected decode error")
	}
}

func TestHasAnyRoleExactMatch(t *testing.T) {
	allowed := map[string]struct{}{"CARRIER_ADMIN": {}}
	if !routeauth.HasAnyRole([]string{"FINANCE_MANAGER", "CARRIER_ADMIN"}, allowed) {
		t.Fatal("expected allow on exact role match")
	}
	if routeauth.HasAnyRole([]string{"carrier_admin"}, allowed) {
		t.Fatal("role matching must be exact, not case-insensitive")
	}
}
