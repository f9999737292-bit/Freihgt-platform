package authority

import (
	"context"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/identity-service/internal/platform/errors"
)

const PlatformAdminRole = "PLATFORM_ADMIN"

// Action is a user or role management operation.
// Delegated tenant user-management permissions are not proven in the current
// runtime, so every mutation and tenant-wide read requires a server-verified
// PLATFORM_ADMIN. Self read does not grant administrative mutation.
type Action string

const (
	ActionUserRead              Action = "user_read"
	ActionUserRoleList          Action = "user_role_list"
	ActionCompanyMembershipRead Action = "company_membership_read"
	ActionTenantUserList        Action = "tenant_user_list"
	ActionUserCreate            Action = "user_create"
	ActionUserUpdate            Action = "user_update"
	ActionUserDelete            Action = "user_delete"
	ActionUserRoleAssign        Action = "user_role_assign"
	ActionCompanyRoleAssign     Action = "company_role_assign"
	ActionCompanyRoleRemove     Action = "company_role_remove"
	ActionTenantRoleList        Action = "tenant_role_list"
	ActionRolePermissionRead    Action = "role_permission_read"
)

// Principal is identity derived from a verified access token and a server-side
// role lookup. Client headers and caller tenant fields are not inputs.
type Principal struct {
	UserID        uuid.UUID
	TenantID      uuid.UUID
	PlatformAdmin bool
	RoleCodes     []string
}

// Lookup loads server-side authority. Platform-admin state comes from a
// tenant-global PLATFORM_ADMIN assignment, not from a caller header.
type Lookup interface {
	LookupPrincipal(ctx context.Context, userID, tenantID uuid.UUID) (platformAdmin bool, roleCodes []string, err error)
	TenantExists(ctx context.Context, tenantID uuid.UUID) (bool, error)
}

// Override is the explicit platform-admin target-tenant record.
// The target tenant is a selector. Authority remains the verified role.
type Override struct {
	ActorUserID    uuid.UUID
	ActorTenantID  uuid.UUID
	TargetTenantID uuid.UUID
	Action         string
}

type Auditor interface {
	RecordPlatformAdminOverride(ctx context.Context, event Override)
}

type LogAuditor struct {
	Log *slog.Logger
}

func (a LogAuditor) RecordPlatformAdminOverride(ctx context.Context, event Override) {
	log := a.Log
	if log == nil {
		log = slog.Default()
	}
	log.InfoContext(ctx, "platform_admin_tenant_override",
		slog.String("actor_user_id", event.ActorUserID.String()),
		slog.String("actor_tenant_id", event.ActorTenantID.String()),
		slog.String("target_tenant_id", event.TargetTenantID.String()),
		slog.String("action", event.Action),
	)
}

// ResolveTenant returns the effective tenant.
// An omitted or equal supplied tenant_id uses the verified JWT tenant and is
// not authority. A different value is TENANT_CONTEXT_CONFLICT unless the
// caller is a server-verified platform admin, the target tenant exists, and
// the override is audited.
func ResolveTenant(ctx context.Context, principal Principal, supplied string, action Action, lookup Lookup, auditor Auditor) (uuid.UUID, error) {
	if principal.TenantID == uuid.Nil || principal.UserID == uuid.Nil {
		return uuid.Nil, apperrors.TenantContextRequired()
	}
	supplied = strings.TrimSpace(supplied)
	if supplied == "" {
		return principal.TenantID, nil
	}
	parsed, err := uuid.Parse(supplied)
	if err != nil || parsed == uuid.Nil {
		if principal.PlatformAdmin {
			return uuid.Nil, apperrors.Validation("target tenant is invalid", map[string]any{"field": "tenant_id"})
		}
		return uuid.Nil, apperrors.TenantContextConflict()
	}
	if parsed == principal.TenantID {
		return principal.TenantID, nil
	}
	if !principal.PlatformAdmin {
		return uuid.Nil, apperrors.TenantContextConflict()
	}
	if lookup == nil {
		return uuid.Nil, apperrors.Internal("authority lookup is not configured", nil)
	}
	exists, err := lookup.TenantExists(ctx, parsed)
	if err != nil {
		return uuid.Nil, err
	}
	if !exists {
		return uuid.Nil, apperrors.ResourceNotFound()
	}
	if auditor == nil {
		auditor = LogAuditor{}
	}
	auditor.RecordPlatformAdminOverride(ctx, Override{
		ActorUserID:    principal.UserID,
		ActorTenantID:  principal.TenantID,
		TargetTenantID: parsed,
		Action:         string(action),
	})
	return parsed, nil
}

// Allow enforces the least-privilege matrix. Self read does not authorize
// create, update, delete, or role assignment.
func Allow(principal Principal, action Action, targetUserID uuid.UUID) error {
	switch action {
	case ActionUserRead, ActionUserRoleList, ActionCompanyMembershipRead:
		if principal.PlatformAdmin || (targetUserID != uuid.Nil && principal.UserID == targetUserID) {
			return nil
		}
		return apperrors.AuthorizationDenied()
	case ActionTenantUserList, ActionUserCreate, ActionUserUpdate, ActionUserDelete,
		ActionUserRoleAssign, ActionCompanyRoleAssign, ActionCompanyRoleRemove,
		ActionTenantRoleList, ActionRolePermissionRead:
		if principal.PlatformAdmin {
			return nil
		}
		return apperrors.AuthorizationDenied()
	default:
		return apperrors.AuthorizationDenied()
	}
}
