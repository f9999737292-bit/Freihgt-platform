# RBAC Role Navigation Implementation Backlog v0.1

## P0

| ID | Task | Output |
| --- | --- | --- |
| RBAC-P0-001 | Define canonical role constants | role constants/spec mapped to identity codes |
| RBAC-P0-002 | Define permission constants | permission constants/spec per module |
| RBAC-P0-003 | Replace usePermissions TODO with explicit map | permission helper design |
| RBAC-P0-004 | Create role-based sidebar item filtering in AppSidebar | role-aware nav design |
| RBAC-P0-005 | Define first-screen redirect per role after login | login redirect strategy |
| RBAC-P0-006 | Define access-denied UI state component/page | UX spec |
| RBAC-P0-007 | Remove demo credential defaults from production login | production login safety task |
| RBAC-P0-008 | Map identity roles[] to product role for nav resolution | role resolver spec |

## P1

| ID | Task | Output |
| --- | --- | --- |
| RBAC-P1-001 | Add role-specific dashboard cards | dashboard role scoping |
| RBAC-P1-002 | Hide admin modules for non-admin users | UI scoping |
| RBAC-P1-003 | Add role labels/profile context in AppHeader | role-aware header |
| RBAC-P1-004 | Add backend status visibility by role | health visibility policy |
| RBAC-P1-005 | Add route middleware for permission check (UI layer) | nav guard design |
| RBAC-P1-006 | Add tests for nav visibility | test plan |
| RBAC-P1-007 | Align useLowCodePermissions with global role map | low-code RBAC consistency |

## P2

| ID | Task | Output |
| --- | --- | --- |
| RBAC-P2-001 | Prepare role apps readiness review | role app future track |
| RBAC-P2-002 | Shared nav component extraction | component reuse plan |
| RBAC-P2-003 | Multi-app RBAC alignment | future architecture |
| RBAC-P2-004 | Visual nav grouping by Navigation Groups | sidebar UX polish |

## Current Code Touchpoints (for implementation pack)

| File | Change Scope |
| ---- | ------------ |
| `apps/web-admin/components/layout/AppSidebar.vue` | filter navItems by role |
| `apps/web-admin/composables/usePermissions.ts` | explicit permission map, role resolver |
| `apps/web-admin/middleware/auth.ts` | optional permission middleware |
| `apps/web-admin/pages/login.vue` | post-login redirect, remove demo defaults |
| `apps/web-admin/pages/index.vue` | role-aware redirect |
| `apps/web-admin/stores/auth.ts` | ensure roles[] populated from /auth/me |

## Implementation Rule

```text
Do not implement directly from this design pack.
Run a separate approved implementation pack before changing source code.
```

## Recommended Next Pack

```text
RBAC_AND_ROLE_NAVIGATION_IMPLEMENTATION_PLAN_PACK v0.1
```
