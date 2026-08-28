package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestResolveVerifiedTenantFromHeader(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/v1/transport-orders", nil)
	req.Header.Set("X-Tenant-ID", tenantID.String())
	got, err := resolveVerifiedTenant(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != tenantID {
		t.Fatalf("tenant=%s want %s", got, tenantID)
	}
}

func TestResolveVerifiedTenantMissingReturns401(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/v1/transport-orders", nil)
	_, err := resolveVerifiedTenant(req)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseOrderAccessActorRequiresVerifiedCompany(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/v1/transport-orders", nil)
	req.Header.Set("X-Company-ID", uuid.NewString())
	req.Header.Set("X-Actor-Kind", "BUYER")
	actor, err := parseOrderAccessActor(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actor.ActorKind != "BUYER" {
		t.Fatalf("actor kind=%q", actor.ActorKind)
	}
}

func TestParseOrderAccessActorPlatformAdminFromGatewayHeader(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/v1/transport-orders", nil)
	req.Header.Set("X-Company-ID", uuid.NewString())
	req.Header.Set("X-Actor-Kind", "BUYER")
	req.Header.Set("X-Platform-Admin", "true")
	actor, err := parseOrderAccessActor(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !actor.IsPlatformAdmin {
		t.Fatal("expected platform admin flag")
	}
}
