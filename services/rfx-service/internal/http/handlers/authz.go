package handlers

import (
	"context"
	"net/http"
	"os"
	"strings"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

type PermissionChecker interface {
	UserHasPermission(r *http.Request, permissionCode string) error
}

type permissionChecker struct {
	repo PermissionStore
}

type PermissionStore interface {
	UserHasPermission(ctx context.Context, userID, tenantID uuid.UUID, permissionCode string) (bool, error)
}

// PermissionCheckerFrom returns a fail-closed checker when RBAC is enabled.
func PermissionCheckerFrom(store PermissionStore) PermissionChecker {
	if store == nil {
		return noopPermissionChecker{}
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("RFX_RBAC_ENABLED")), "false") {
		return noopPermissionChecker{}
	}
	return permissionChecker{repo: store}
}

type noopPermissionChecker struct{}

func (noopPermissionChecker) UserHasPermission(*http.Request, string) error { return nil }

func (c permissionChecker) UserHasPermission(r *http.Request, permissionCode string) error {
	userID, err := resolveVerifiedUser(r)
	if err != nil {
		return err
	}
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		return err
	}
	allowed, err := c.repo.UserHasPermission(r.Context(), userID, tenantID, permissionCode)
	if err != nil {
		return err
	}
	if !allowed {
		return apperrors.Forbidden("missing required permission: " + permissionCode)
	}
	return nil
}
