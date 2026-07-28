# RBAC and Role Navigation Design v0.1

## Summary

RBAC and role-based navigation design completed after role-based cabinets gap analysis.

This is a design-only document. No frontend code, backend code, production, staging, server, API contracts, migrations, or database data were changed.

## Decision

```text
RBAC_AND_ROLE_NAVIGATION_DESIGN_COMPLETE
```

## Current Context

```text
Production deployment: CLOSED
Monitoring cycle v0.2: PASS
UI/navigation audit: COMPLETE
Role-based cabinets gap analysis: COMPLETE
Pilot launch: paused
Operating mode: event-based monitoring
```

## Current RBAC State (read-only audit, 2026-07-28)

| Component | Current Behavior | Gap |
| --------- | ---------------- | --- |
| `AppSidebar.vue` | Static `navItems` — 13 routes shown to all authenticated users | No role filtering |
| `middleware/auth.ts` | Redirects unauthenticated users to `/login` | No role/permission check on routes |
| `usePermissions.ts` | Reads `roles[]` / `permissions[]` from authStore; dev mock fallback for `admin@7rights.local` | TODO to wire full RBAC payload from `/auth/me` |
| `useLowCodePermissions.ts` | Role constants exist (PLATFORM_ADMIN, SHIPPER_*, CARRIER_*, etc.) | Used only in low-code UI, not sidebar |
| `login.vue` | Pre-fills demo credentials; backend status panel | Production-facing demo defaults |
| `docs/AUTH_RBAC.md` | Documents login, usePermissions, dev fallback | Reference for implementation pack |

## Endpoint Check (2026-07-28)

| Check | Result |
| ----- | ------ |
| Production `/` | PASS — 200 text/html |
| Production `/login` | PASS — 200 text/html |
| Production `/health` | PASS — 200 |
| Staging `/` | PASS — 200 text/html |
| Staging `/login` | PASS — 200 text/html |
| Staging `/health` | PASS — 200 |

## Strategy

```text
Use hybrid strategy:
1. Keep web-admin as current production control center.
2. Implement role-based navigation in web-admin first.
3. Keep separate role apps as future specialization track.
4. Do not expose skeleton role apps to production yet.
```

## Canonical Roles

| Role Code   | Russian Name    | Identity Role Examples (pilot)              | Purpose                                                  | First Screen                    |
| ----------- | --------------- | ----------------------------------------- | -------------------------------------------------------- | ------------------------------- |
| admin       | Администратор   | PLATFORM_ADMIN                            | platform/company administration                          | /dashboard or /control-tower    |
| shipper     | Грузовладелец   | SHIPPER_ADMIN, SHIPPER_LOGIST             | create/manage freight needs and transport orders         | /dashboard or /transport-orders |
| carrier     | Перевозчик      | CARRIER_ADMIN, CARRIER_DISPATCHER         | view/respond/execute assigned transport work             | /shipments                      |
| forwarder   | Экспедитор      | FORWARDER_MANAGER                         | manage delegated freight/RFx/shipment flows              | /freight-requests or /rfx       |
| consignee   | Грузополучатель | CONSIGNEE_OPERATOR, CONSIGNEE_VIEWER      | receive/confirm deliveries and related docs              | /shipments                      |
| finance     | Финансы         | FINANCE_MANAGER                           | view/approve billing, documents, financial status        | /billing-registers              |
| procurement | Закупки         | PROCUREMENT_MANAGER                       | manage freight requests, RFx/tenders, supplier selection | /freight-requests or /rfx       |

## Navigation Design Principle

```text
Do not show all modules to all roles.
Each role should see only modules that are relevant and allowed.
Admin keeps full navigation.
Non-admin roles get scoped navigation.
```

## Role-Based Navigation Draft

| Route              | Admin | Shipper       | Carrier          | Forwarder          | Consignee       | Finance        | Procurement       |
| ------------------ | ----- | ------------- | ---------------- | ------------------ | --------------- | -------------- | ----------------- |
| /dashboard         | show  | show          | show             | show               | show            | show           | show              |
| /control-tower     | show  | limited       | limited          | show               | limited         | limited        | show              |
| /transport-orders  | show  | show          | assigned/view    | show               | related/view    | financial view | view              |
| /freight-requests  | show  | show          | respond/view     | show               | hide            | hide/limited   | show              |
| /rfx               | show  | show          | participate/view | show               | hide            | cost view      | show              |
| /shipments         | show  | own/manage    | assigned/update  | assigned/manage    | receive/confirm | view           | view              |
| /documents         | show  | own/sign      | own/upload/sign  | manage             | receive/view    | approve/view   | view              |
| /billing-registers | show  | approve/view  | settlements/view | manage settlements | hide            | show           | view              |
| /companies         | show  | own company   | own company      | own company        | own company     | view           | view              |
| /users             | show  | company users | company users    | company users      | limited         | finance users  | procurement users |
| /low-code          | show  | hide          | hide             | hide               | hide            | hide/limited   | limited           |
| /settings          | show  | show          | show             | show               | show            | show           | show              |
| /health            | show  | hide          | hide             | hide               | hide            | limited        | limited           |

## RBAC Design Principle

```text
Frontend role-based navigation is not security by itself.
Backend/API permissions must remain authoritative.
Frontend should hide irrelevant modules and show clear access-denied states.
```

## Permission Groups

| Permission Group | Example Permissions                                                                               |
| ---------------- | ------------------------------------------------------------------------------------------------- |
| dashboard        | dashboard.view                                                                                    |
| control_tower    | control_tower.view                                                                                |
| transport_orders | transport_orders.view, transport_orders.create, transport_orders.manage                           |
| freight_requests | freight_requests.view, freight_requests.create, freight_requests.respond, freight_requests.manage |
| rfx              | rfx.view, rfx.create, rfx.respond, rfx.manage                                                     |
| shipments        | shipments.view, shipments.update, shipments.confirm, shipments.manage                             |
| documents        | documents.view, documents.upload, documents.sign, documents.approve                               |
| billing          | billing.view, billing.approve, billing.manage                                                     |
| companies        | companies.view, companies.manage                                                                  |
| users            | users.view, users.manage                                                                          |
| low_code         | low_code.view, low_code.manage                                                                    |
| settings         | settings.view                                                                                     |
| health           | health.view                                                                                       |

## P0 Design Decisions

```text
1. Canonical roles must be fixed before implementation.
2. web-admin sidebar must become role-aware.
3. usePermissions TODO must be replaced with explicit permission map.
4. First screen per role must be defined.
5. Skeleton role apps must stay out of production.
6. Map product roles to existing identity role codes (PLATFORM_ADMIN, SHIPPER_LOGIST, etc.).
7. Reuse useLowCodePermissions role constants as reference for broader nav design.
```

## Implementation Boundary

```text
This pack does not implement RBAC.
Implementation requires a separate approved pack.
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
RBAC_AND_ROLE_NAVIGATION_IMPLEMENTATION_PLAN_PACK v0.1
```
