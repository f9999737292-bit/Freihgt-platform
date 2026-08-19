package domain

import (
	"testing"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/billing-register-service/internal/platform/errors"
)

func TestEnforceOptionalBodyTenantAllowsEmpty(t *testing.T) {
	t.Parallel()
	trusted := uuid.New()
	if err := EnforceOptionalBodyTenant(trusted, ""); err != nil {
		t.Fatalf("empty body tenant should be allowed: %v", err)
	}
}

func TestEnforceOptionalBodyTenantAllowsMatch(t *testing.T) {
	t.Parallel()
	trusted := uuid.New()
	if err := EnforceOptionalBodyTenant(trusted, trusted.String()); err != nil {
		t.Fatalf("matching body tenant should be allowed: %v", err)
	}
}

func TestEnforceOptionalBodyTenantRejectsMismatch(t *testing.T) {
	t.Parallel()
	trusted := uuid.New()
	err := EnforceOptionalBodyTenant(trusted, uuid.New().String())
	if err == nil {
		t.Fatal("expected forbidden for tenant mismatch")
	}
	appErr, ok := err.(*apperrors.AppError)
	if !ok || appErr.Code != apperrors.CodeForbidden {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestResolveApprovedByUsesTrustedUser(t *testing.T) {
	t.Parallel()
	trusted := uuid.New()
	got, err := ResolveApprovedBy(trusted, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != trusted {
		t.Fatalf("approved_by=%v want trusted %v", got, trusted)
	}
}

func TestResolveApprovedByRejectsSpoof(t *testing.T) {
	t.Parallel()
	trusted := uuid.New()
	_, err := ResolveApprovedBy(trusted, uuid.New().String())
	if err == nil {
		t.Fatal("expected forbidden for approved_by spoof")
	}
	appErr, ok := err.(*apperrors.AppError)
	if !ok || appErr.Code != apperrors.CodeForbidden {
		t.Fatalf("expected forbidden, got %v", err)
	}
}
