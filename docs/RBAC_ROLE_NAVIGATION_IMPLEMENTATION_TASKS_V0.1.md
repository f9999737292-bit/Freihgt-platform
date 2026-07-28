# RBAC Role Navigation Implementation Tasks v0.1

## Summary

Task list for future RBAC and role navigation source-code implementation.

## Decision

```text
RBAC_ROLE_NAVIGATION_IMPLEMENTATION_TASKS_CREATED
```

## P0 Tasks

| ID | Task | Files |
| --- | --- | --- |
| RBAC-IMPL-P0-001 | Add canonical role constants | `composables/usePermissions.ts` or new `constants/roles.ts` |
| RBAC-IMPL-P0-002 | Add identity-role to product-role resolver | `composables/usePermissions.ts` or new helper |
| RBAC-IMPL-P0-003 | Add permission map | `composables/usePermissions.ts` or new `constants/permissions.ts` |
| RBAC-IMPL-P0-004 | Filter AppSidebar items by role/permission | `components/layout/AppSidebar.vue` |
| RBAC-IMPL-P0-005 | Define role landing route helper | `composables/usePermissions.ts` |
| RBAC-IMPL-P0-006 | Role-aware post-login redirect | `pages/login.vue`, `composables/useAuth.ts` |
| RBAC-IMPL-P0-007 | Role-aware index redirect | `pages/index.vue` |
| RBAC-IMPL-P0-008 | Remove/guard demo login defaults | `pages/login.vue` |
| RBAC-IMPL-P0-009 | Add access denied UX plan/component | new component or extend `ApiUnavailableState.vue` |
| RBAC-IMPL-P0-010 | Run frontend build/typecheck | `apps/web-admin` |

## P1 Tasks

| ID | Task | Files |
| --- | --- | --- |
| RBAC-IMPL-P1-001 | Role-aware dashboard content planning | `pages/dashboard/index.vue` |
| RBAC-IMPL-P1-002 | Role label in header/profile | `components/layout/AppHeader.vue` |
| RBAC-IMPL-P1-003 | Tests for nav visibility | test files TBD |
| RBAC-IMPL-P1-004 | Route-level guard design | `middleware/auth.ts` or new permission middleware |
| RBAC-IMPL-P1-005 | Align useLowCodePermissions with global role map | `composables/useLowCodePermissions.ts` |

## Source Change Boundary

```text
These tasks are NOT approved for source code changes yet.
Run RBAC_AND_ROLE_NAVIGATION_IMPLEMENTATION_APPROVAL_PACK before implementation.
Only web-admin frontend files listed above.
No apps/web-shipper|carrier|consignee|finance|procurement changes in first implementation.
No services/, migrations/, infrastructure/ changes.
```

## Recommended Next Pack

```text
RBAC_AND_ROLE_NAVIGATION_IMPLEMENTATION_APPROVAL_PACK v0.1
```
