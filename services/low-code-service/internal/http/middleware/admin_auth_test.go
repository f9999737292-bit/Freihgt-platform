package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	sharedlowcode "github.com/freight-platform/shared-go/lowcode"
)

type stubRoleChecker struct {
	codes []string
	err   error
}

func (s stubRoleChecker) ListUserRoleCodes(context.Context, uuid.UUID, uuid.UUID) ([]string, error) {
	return s.codes, s.err
}

func TestRequireLowCodeAdminDisabledPassesThrough(t *testing.T) {
	called := false
	handler := RequireLowCodeAdmin(AdminAuthConfig{Enabled: false})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusNoContent || !called {
		t.Fatalf("expected pass-through, got %d called=%v", rec.Code, called)
	}
}

func TestRequireLowCodeAdminMissingUserUnauthorized(t *testing.T) {
	handler := RequireLowCodeAdmin(AdminAuthConfig{
		Enabled:     true,
		RoleChecker: stubRoleChecker{codes: []string{"PLATFORM_ADMIN"}},
	})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("handler should not be called")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set(sharedlowcode.HeaderTenantID, uuid.New().String())
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestRequireLowCodeAdminForbiddenForDriver(t *testing.T) {
	handler := RequireLowCodeAdmin(AdminAuthConfig{
		Enabled:     true,
		RoleChecker: stubRoleChecker{codes: []string{"DRIVER"}},
	})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("handler should not be called")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set(sharedlowcode.HeaderTenantID, uuid.New().String())
	req.Header.Set(sharedlowcode.HeaderUserID, uuid.New().String())
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for DRIVER, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestRequireLowCodeAdminForbiddenWithoutRole(t *testing.T) {
	handler := RequireLowCodeAdmin(AdminAuthConfig{
		Enabled:     true,
		RoleChecker: stubRoleChecker{codes: []string{"SHIPPER_ADMIN"}},
	})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("handler should not be called")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set(sharedlowcode.HeaderTenantID, uuid.New().String())
	req.Header.Set(sharedlowcode.HeaderUserID, uuid.New().String())
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestRequireLowCodeAdminAllowsPlatformAdmin(t *testing.T) {
	called := false
	handler := RequireLowCodeAdmin(AdminAuthConfig{
		Enabled:     true,
		RoleChecker: stubRoleChecker{codes: []string{"PLATFORM_ADMIN"}},
	})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set(sharedlowcode.HeaderTenantID, uuid.New().String())
	req.Header.Set(sharedlowcode.HeaderUserID, uuid.New().String())
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || !called {
		t.Fatalf("expected 200, got %d called=%v body=%s", rec.Code, called, rec.Body.String())
	}
}
