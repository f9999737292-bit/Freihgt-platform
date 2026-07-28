# RBAC Role Navigation Source Boundary v0.1

## Summary

Source change boundary for future RBAC role navigation frontend implementation.

## Decision

```text
RBAC_ROLE_NAVIGATION_SOURCE_BOUNDARY_CREATED
```

## Approved Files for Future Implementation

| File | Approved Change |
| --- | --- |
| `apps/web-admin/composables/usePermissions.ts` | role constants, resolver, permission map, landing route helper |
| `apps/web-admin/components/layout/AppSidebar.vue` | role-based sidebar filtering |
| `apps/web-admin/pages/login.vue` | remove/dev-guard demo credential defaults; post-login redirect |
| `apps/web-admin/pages/index.vue` | role landing redirect if needed |

## Optional Files

| File | Condition |
| --- | --- |
| `apps/web-admin/components/common/AccessDeniedState.vue` | only if access denied UI is implemented |
| `apps/web-admin/pages/access-denied.vue` | only if route-level denied page is implemented |

## Read-only Reference Files (do not change unless scope expanded)

| File | Purpose |
| --- | --- |
| `apps/web-admin/stores/auth.ts` | roles[] source — read for resolver design |
| `apps/web-admin/middleware/auth.ts` | auth guard — no change in first implementation |
| `apps/web-admin/pages/dashboard/index.vue` | first-screen target — no change unless role-aware dashboard approved |
| `apps/web-admin/components/common/ApiUnavailableState.vue` | pattern reference for access denied |

## Forbidden Files

```text
apps/web-shipper/
apps/web-carrier/
apps/web-consignee/
apps/web-finance/
apps/web-procurement/
services/
infrastructure/
migrations/
docker-compose*
.env*
server configs
cert files
private keys
rollback docs (local modified)
selectel/staging modified docs
staging regression pair
scripts/
web-admin-dist-staging.tar.gz
```

## Next

```text
RBAC_ROLE_NAVIGATION_FRONTEND_IMPLEMENTATION_PACK v0.1
```
