# RBAC Role Navigation Implementation Approval v0.1

## Summary

Approval prepared for future frontend implementation of RBAC and role-based navigation in web-admin.

This approval pack does not change source code, production, staging, server, API contracts, migrations, or database data.

## Decision

```text
RBAC_ROLE_NAVIGATION_IMPLEMENTATION_APPROVED_FOR_FRONTEND_PACK
```

## Current Context

```text
Production deployment: CLOSED
Monitoring cycle v0.2: PASS
RBAC design: COMPLETE (33695b7)
RBAC implementation plan: COMPLETE (da08c06)
Pilot launch: paused
Operating mode: event-based monitoring
Strategy: Hybrid — web-admin first, separate role apps later
```

## Target File Verification (2026-07-28)

All approved-scope files verified present:

```text
FOUND apps/web-admin/components/layout/AppSidebar.vue
FOUND apps/web-admin/composables/usePermissions.ts
FOUND apps/web-admin/stores/auth.ts
FOUND apps/web-admin/middleware/auth.ts
FOUND apps/web-admin/pages/login.vue
FOUND apps/web-admin/pages/index.vue
FOUND apps/web-admin/pages/dashboard/index.vue
FOUND apps/web-admin/components/common/ApiUnavailableState.vue
```

## Approved Future Source Scope

The next implementation pack may change only these frontend files:

```text
apps/web-admin/components/layout/AppSidebar.vue
apps/web-admin/composables/usePermissions.ts
apps/web-admin/pages/login.vue
apps/web-admin/pages/index.vue
```

Optional only if required and explicitly reported:

```text
apps/web-admin/components/common/AccessDeniedState.vue
apps/web-admin/pages/access-denied.vue
```

## Not Approved in Next Implementation Pack

```text
services/
infrastructure/
migrations/
database schema
API contracts
Docker compose
server config
Nginx
DNS
Certbot
production deploy
staging deploy
role apps deployment
apps/web-shipper/
apps/web-carrier/
apps/web-consignee/
apps/web-finance/
apps/web-procurement/
stores/auth.ts (read-only unless explicitly added to scope later)
middleware/auth.ts (read-only unless explicitly added to scope later)
```

## Implementation Goals for Next Pack

```text
1. Add canonical product roles.
2. Add identity-role to product-role resolver.
3. Add permission map.
4. Filter AppSidebar navigation by role/permission.
5. Add role landing route helper.
6. Remove or dev-guard demo login defaults.
7. Keep backend/API permissions authoritative.
8. Run frontend build/typecheck.
```

## Required Acceptance Gate

```text
Frontend build/typecheck must pass.
No backend code changed.
No API contracts changed.
No migrations changed.
No deploy executed.
No secrets captured.
Only approved frontend files changed.
```

## Production Deployment Boundary

```text
This approval does not approve production deployment.
After source implementation, a separate build/review/deploy approval is required.
```

## Next Recommended Pack

```text
RBAC_ROLE_NAVIGATION_FRONTEND_IMPLEMENTATION_PACK v0.1
```
