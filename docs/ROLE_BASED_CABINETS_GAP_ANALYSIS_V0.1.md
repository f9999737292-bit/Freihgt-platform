# Role-based Cabinets Gap Analysis v0.1

## Summary

Role-based cabinets gap analysis completed after UI/navigation audit.

This analysis is read-only. No source code, production, staging, server, API contracts, migrations, or database data were changed.

## Decision

```text
ROLE_BASED_CABINETS_GAP_ANALYSIS_COMPLETE
```

## Current Context

```text
Production deployment: CLOSED
Monitoring cycle v0.2: PASS
Demo readiness: PREPARED
Product next iteration planning: COMPLETE
UI/navigation audit: COMPLETE
Pilot launch: paused
Operating mode: event-based monitoring
```

## Endpoint Check (2026-07-28)

| Check | Result |
| ----- | ------ |
| Production `/` | PASS — 200 text/html |
| Production `/login` | PASS — 200 text/html |
| Production `/health` | PASS — 200 |
| Staging `/` | PASS — 200 text/html |
| Staging `/login` | PASS — 200 text/html |
| Staging `/health` | PASS — 200 |

## Apps Reviewed

| App             | Exists | Intended Role             | Current Readiness | Notes |
| --------------- | ------ | ------------------------- | ----------------- | ----- |
| web-admin       | yes    | admin / universal control | partial           | Deployed to production; 30 pages, 95 components, 21 composables; flat sidebar for all users; no role-based nav |
| web-shipper     | yes    | грузовладелец             | missing           | Skeleton only: 1 welcome page, 0 components/composables; dev port 3001 |
| web-carrier     | yes    | перевозчик                | missing           | Skeleton only: 1 welcome page; dev port 3002 |
| web-consignee   | yes    | грузополучатель           | missing           | Skeleton only: 1 welcome page; dev port 3003 |
| web-finance     | yes    | финансы                   | missing           | Skeleton only: 1 welcome page; dev port 3004 |
| web-procurement | yes    | закупки / тендеры         | missing           | Skeleton only: 1 welcome page; dev port 3005; closest app to forwarder/procurement persona |

**Forwarder (экспедитор):** no dedicated frontend app. Forwarder flows currently live in web-admin (`/freight-requests`, `/rfx`, `/transport-orders`) and demo seed maps `PROCUREMENT_MANAGER` / `forwarder@bintrans.local`.

## web-admin vs Role Apps Comparison

| Capability | web-admin | Role apps (5) |
| ---------- | --------- | ------------- |
| Auth/login | yes | no |
| Dashboard | yes | no |
| TMS / transport orders | yes | no |
| RFx / freight requests | yes | no |
| Shipments / documents / billing | yes | no |
| Companies / users / low-code | yes | no |
| Role-specific UX | no | not implemented |
| Production deploy | yes | no |
| Shared workspace packages | partial | uses `@freight-platform/i18n`, `ui`, `shared-ts` |

## Role Strategy Options

| Option                                   | Description                                                 | Pros                    | Risks                        |
| ---------------------------------------- | ----------------------------------------------------------- | ----------------------- | ---------------------------- |
| Single web-admin + role-based navigation | One production app with role-filtered menus and permissions | fastest, simpler deploy | can become overloaded        |
| Separate role apps                       | Each role has its own app                                   | cleaner UX by role      | more deploy/build complexity |
| Hybrid                                   | web-admin as control center + role apps later               | balanced                | needs clear routing strategy |

## Recommended Strategy

```text
Hybrid strategy:
1. Keep web-admin as current production control center.
2. Implement role-based navigation and RBAC in web-admin first.
3. Keep separate role apps as future cabinet specialization track.
4. Do not expose separate role apps to production until reviewed and hardened.
5. Treat forwarder as a web-admin role persona first; no dedicated app yet.
```

## RBAC Reference Status

| Item | Status |
| ---- | ------ |
| `docs/AUTH_RBAC.md` | present — documents login, usePermissions, dev fallback |
| `usePermissions.ts` | partial — roles/permissions from authStore; dev mock fallback |
| Sidebar role filtering | missing |
| Role apps auth | missing |

## Role Gaps

| Role        | Current Gap | Severity | Recommendation |
| ----------- | ----------- | -------- | -------------- |
| Admin       | Full module access shown to all users; RBAC nav not enforced | P0 | Role-filtered sidebar + PLATFORM_ADMIN full access in RBAC design pack |
| Shipper     | No shipper cabinet in production; web-shipper is skeleton | P0 | Define shipper module subset in web-admin first (orders, shipments, documents) |
| Carrier     | No carrier cabinet; web-carrier is skeleton | P0 | Define carrier module subset (assigned shipments, bids, documents) |
| Forwarder   | No dedicated app; mixed into admin TMS/RFx surfaces | P0 | Define forwarder persona nav (freight-requests, rfx, delegated orders) |
| Consignee   | No consignee cabinet; web-consignee is skeleton | P1 | Limited nav: receive shipments, documents, status |
| Finance     | No finance cabinet; web-finance is skeleton | P1 | Limited nav: billing registers, financial views, approvals |
| Procurement | No procurement cabinet; web-procurement is skeleton | P1 | Limited nav: RFx, freight requests, tender participation |

## P0 Recommendations

```text
- Adopt hybrid strategy: web-admin first, role apps later
- Define canonical role list aligned with identity-service roles (PLATFORM_ADMIN, SHIPPER_LOGIST, CARRIER_*, FORWARDER_*, CONSIGNEE_*, finance/procurement roles)
- Design role-based sidebar filtering in web-admin before any role app deploy
- Wire RBAC payload plan: roles[] and permissions[] from /auth/me (see AUTH_RBAC.md TODO)
- Define first-screen per role: admin→dashboard/control-tower, shipper→transport-orders, carrier→shipments, forwarder→freight-requests/rfx
- Do not deploy skeleton role apps to production
```

## P1 Recommendations

```text
- Map each role to module access matrix (see ROLE_TO_MODULE_ACCESS_MATRIX_V0.1.md)
- Define consignee/finance/procurement limited nav sets
- Plan role-specific dashboard cards instead of full admin dashboard for non-admin roles
- Review demo seed role users (shipper/carrier/forwarder/consignee) for future walkthrough — paused until pilot approved
- Hide admin-only modules (low-code admin, users, health) from non-admin roles
```

## P2 Recommendations

```text
- Evaluate web-shipper/carrier/consignee/finance/procurement skeleton apps for future extraction
- Assign dev ports and build pipeline strategy for multi-app deploy (3001–3005)
- Consider dedicated forwarder app only after web-admin role nav proves stable
- Role app shared component library reuse from web-admin
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
RBAC_AND_ROLE_NAVIGATION_DESIGN_PACK v0.1
```
