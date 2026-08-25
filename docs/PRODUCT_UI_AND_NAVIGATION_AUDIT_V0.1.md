# Product UI and Navigation Audit v0.1

## Summary

UI and navigation audit completed for web-admin after product next iteration planning.

This audit is read-only. No source code, production, staging, server, API contracts, migrations, or database data were changed.

## Decision

```text
PRODUCT_UI_AND_NAVIGATION_AUDIT_COMPLETE
```

## Current Context

```text
Production deployment: CLOSED
Monitoring cycle v0.2: PASS
Demo readiness: PREPARED
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

## Web-admin Page Inventory

| Area              | Route / Page       | Status  | Notes |
| ----------------- | ------------------ | ------- | ----- |
| Login             | /login             | present | auth layout, guest middleware, backend status panel, tenant/email/password form |
| Root redirect     | /                  | present | redirects to /dashboard if authenticated else /login |
| Dashboard         | /dashboard         | present | stat cards for 7 modules, backend/API unavailable states |
| Control tower     | /control-tower     | present | KPI, funnels, operations board, risk alerts, recent activity |
| Transport orders  | /transport-orders  | present | list + /transport-orders/[id] detail |
| Freight requests  | /freight-requests  | present | list + detail, bid integration |
| RFx               | /rfx               | present | list + detail, participants |
| Shipments         | /shipments         | present | list + detail, status timeline |
| Documents         | /documents         | present | list + detail, versions/files |
| Billing registers | /billing-registers | present | list + detail |
| Companies         | /companies         | present | list + detail, members |
| Users             | /users             | present | list + detail |
| Low-code          | /low-code          | present | hub + form-templates, custom-field-values, audit, admin/form-templates |
| Settings          | /settings          | present | session/tenant/API config display |
| Health            | /health            | present | gateway health/ready, per-service status |

**Total page files:** 30 (including index, detail, and low-code sub-routes)

## Navigation Structure

Sidebar (`AppSidebar.vue`) exposes a flat list of 13 routes to all authenticated users:

```text
/dashboard, /control-tower, /companies, /users, /transport-orders,
/freight-requests, /rfx, /shipments, /documents, /billing-registers,
/low-code, /health, /settings
```

Auth middleware protects authenticated routes. Low-code admin routes use `low-code-admin` middleware separately.

## Navigation Findings

| Finding | Severity | Recommendation |
| ------- | -------- | -------------- |
| Sidebar brand aligned to Bintrans Freight Platform (wave A1) | P0 | Resolved in wave A1 branding alignment |
| Sidebar shows all modules to all users; no role-based nav filtering | P0 | Define role-to-nav matrix; hide irrelevant modules per role |
| `usePermissions` has TODO — roles/permissions not loaded from `/auth/me` | P0 | Wire RBAC payload before role-based cabinets work |
| Login form no longer pre-fills demo credentials (empty form) | P0 | **Resolved in Wave A2.1** |
| No public landing page; `/` only redirects | P1 | Decide first-screen strategy: landing vs login vs dashboard |
| Control tower links to localhost Swagger/Prometheus/Grafana | P1 | Replace with production-safe links or hide in prod |
| Health page lists localhost service URLs and dev tooling links | P1 | Environment-aware health/dev links |
| Dashboard meta card labels environment as "Local" | P1 | Show production/staging environment label |
| Create buttons on list pages (e.g. transport orders) may lack wired actions | P1 | Verify create flows during module gap analysis packs |
| Low-code hub is rich but complex for first owner walkthrough | P2 | Add guided entry path or simplified admin overview |
| Nav uses Unicode icon placeholders, not consistent design system icons | P2 | UI polish in later iteration |

## Owner Review Readiness

| Area              | Ready for Owner Review? | Notes |
| ----------------- | ----------------------- | ----- |
| Login             | partial                 | Route works; branding and demo defaults need cleanup |
| Dashboard         | partial                 | Structure ready; meaningful counts need authenticated session + backend data |
| Control tower     | partial                 | Rich operational view; localhost dev links confusing in production |
| TMS pages         | partial                 | List/detail exist; create flow and demo data needed for full walkthrough |
| RFx pages         | partial                 | List/detail exist; tender scenario needs demo data |
| Shipment pages    | partial                 | List/detail/timeline exist; tracking scenario needs demo data |
| Documents/Billing | partial                 | CRUD UI present; operational closure scenario needs demo data |
| Admin/Low-code    | partial                 | Hub and admin sub-routes present; best for technical/admin review first |

## P0 Recommendations

```text
- Align web-admin brand with Bintrans for owner/product review
- Define and implement role-based navigation filtering strategy
- Wire RBAC roles/permissions from auth payload (usePermissions TODO)
- Remove or environment-gate demo login defaults before external review
- Confirm first-screen strategy: login → dashboard vs dedicated landing
```

## P1 Recommendations

```text
- Hide or environment-configure localhost dev tool links (control tower, health)
- Fix environment labeling on dashboard (Local → production/staging)
- Verify create/action buttons on list pages across TMS/RFx/shipment modules
- Prepare internal demo data paths before owner walkthrough (separate pack, paused for now)
- Map sidebar items to shipper/carrier/forwarder/admin personas in next pack
```

## P2 Recommendations

```text
- UI icon/design system polish for sidebar and page headers
- Simplify low-code entry for non-technical owner review
- Add breadcrumbs consistency audit across detail pages
- Plan analytics/dashboard enhancements after core flow gaps close
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
ROLE_BASED_CABINETS_GAP_ANALYSIS_PACK v0.1
```
