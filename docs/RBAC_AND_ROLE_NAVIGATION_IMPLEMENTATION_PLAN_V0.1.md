# RBAC and Role Navigation Implementation Plan v0.1

## Summary

Implementation plan prepared for RBAC and role-based navigation in web-admin.

This is a planning-only document. No source code, production, staging, server, API contracts, migrations, or database data were changed.

## Decision

```text
RBAC_AND_ROLE_NAVIGATION_IMPLEMENTATION_PLAN_COMPLETE
```

## Current Context

```text
Production deployment: CLOSED
Monitoring cycle v0.2: PASS
UI/navigation audit: COMPLETE
Role-based cabinets gap analysis: COMPLETE
RBAC and role navigation design: COMPLETE
Design commit: 33695b7 docs: design RBAC and role navigation
Pilot launch: paused
Operating mode: event-based monitoring
```

## Implementation Strategy

```text
- implement role-based navigation inside web-admin first
- do not expose separate role apps to production
- backend/API permissions remain authoritative
- frontend navigation visibility is UX, not security
```

## Current Source State (read-only audit)

| File | Current State |
| ---- | ------------- |
| `components/layout/AppSidebar.vue` | Static `navItems` — 13 routes, no filtering |
| `composables/usePermissions.ts` | TODO comment; roles from authStore; dev mock fallback |
| `stores/auth.ts` | mockAuth assigns PLATFORM_ADMIN; real login via `/api/v1/auth/login` |
| `middleware/auth.ts` | Authentication only — no route permission guard |
| `pages/login.vue` | Pre-filled demo credentials |
| `pages/index.vue` | Redirect to /dashboard or /login |
| `components/common/ApiUnavailableState.vue` | Reusable fallback pattern for error states |

## Target Files for Future Implementation

| File | Planned Change |
| ---- | -------------- |
| `apps/web-admin/composables/usePermissions.ts` | replace TODO/fallback with explicit role/permission helpers, product role resolver |
| `apps/web-admin/components/layout/AppSidebar.vue` | filter nav items by permission/role |
| `apps/web-admin/stores/auth.ts` | confirm roles[] source and role normalization from login/me |
| `apps/web-admin/middleware/auth.ts` | keep auth-only or add route guard plan |
| `apps/web-admin/pages/login.vue` | remove production demo credential defaults; role-aware post-login redirect |
| `apps/web-admin/pages/index.vue` | role-aware landing redirect |
| `apps/web-admin/pages/dashboard/index.vue` | prepare role-aware first screen |
| `apps/web-admin/components/common/ApiUnavailableState.vue` | reuse pattern for access-denied component |

## Phases

### Phase 1 — Role Constants and Identity Role Resolver

```text
Define canonical product roles:
admin, shipper, carrier, forwarder, consignee, finance, procurement.

Map identity roles to product roles:
PLATFORM_ADMIN -> admin
SHIPPER_ADMIN / SHIPPER_LOGIST -> shipper
CARRIER_ADMIN / CARRIER_DISPATCHER -> carrier
FORWARDER_MANAGER -> forwarder
CONSIGNEE_OPERATOR / CONSIGNEE_VIEWER -> consignee
FINANCE_MANAGER -> finance
PROCUREMENT_MANAGER -> procurement
```

### Phase 2 — Permission Map

```text
Define explicit permission groups:
dashboard, control_tower, transport_orders, freight_requests, rfx, shipments,
documents, billing, companies, users, low_code, settings, health.

Source: docs/RBAC_ROLE_PERMISSION_MATRIX_V0.1.md
```

### Phase 3 — Sidebar Filtering

```text
Convert static 13-item sidebar into role-aware navigation.
Admin sees full menu.
Non-admin users see role-scoped menu only.
Source: docs/ROLE_BASED_SIDEBAR_NAVIGATION_SPEC_V0.1.md
```

### Phase 4 — First-screen Redirect

```text
After login or app load:
admin -> /dashboard
shipper -> /dashboard
carrier -> /shipments
forwarder -> /freight-requests
consignee -> /shipments
finance -> /billing-registers
procurement -> /freight-requests
```

### Phase 5 — Access Denied UX

```text
If route is opened directly without permission:
show access denied state,
do not expose data,
provide link to role landing page.
Reuse ApiUnavailableState pattern or add dedicated AccessDeniedState component.
```

### Phase 6 — Production Login Safety

```text
Remove demo credential defaults from production login UI.
Keep local/dev helper only if explicitly guarded by development mode (mockAuth).
```

### Phase 7 — Validation

```text
Run frontend typecheck/build in apps/web-admin.
Verify production/staging not changed until explicit deploy approval.
No API contract changes.
No migrations.
Manual review per acceptance checklist.
```

## Out of Scope

```text
Backend authorization implementation
Database migrations
Production deploy
Separate role apps deployment
Pilot demo data
External user onboarding
```

## Safety Result

```text
Production changed: no
Staging changed: no
Server changed: no
Source code changed: no
Database writes executed: no
Secrets captured: no
```

## Next Recommended Pack

```text
RBAC_AND_ROLE_NAVIGATION_IMPLEMENTATION_APPROVAL_PACK v0.1
```
