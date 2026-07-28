# Product Next Iteration Roadmap v0.1

## Summary

Pilot launch is paused. The next workstream is product development planning after production deployment closure and demo readiness preparation.

This roadmap defines the next internal product iteration without changing production, staging, server, source code, API contracts, or migrations.

## Decision

```text
PRODUCT_NEXT_ITERATION_PLANNING_COMPLETE
```

## Current State

```text
Production deployment: CLOSED
Monitoring cycle v0.2: PASS
Demo readiness: PREPARED
Pilot launch: paused
Operating mode: event-based monitoring
```

## Product Goal

```text
Move from technical production baseline to stronger internal product baseline before pilot.
```

## Repo Inventory Evidence (read-only, 2026-07-28)

```text
Frontend apps:
- web-admin (deployed to production)
- web-carrier
- web-consignee
- web-finance
- web-procurement
- web-shipper

web-admin pages present:
- login, dashboard, control-tower
- transport-orders, freight-requests, rfx
- shipments, documents, billing-registers
- companies, users, settings
- low-code (admin, form-templates, audit, custom-field-values)
- health

Backend services present:
- transport-order-service
- rfx-service
- shipment-service
- low-code-service
(+ additional services in services/ tree)

Key implication:
Production currently exposes web-admin only; role-based cabinets exist as separate apps but need gap analysis before pilot.
```

## Recommended Priority Order

| Priority | Area                             | Why                                                                |
| -------- | -------------------------------- | ------------------------------------------------------------------ |
| P0       | Owner/product walkthrough gaps   | Need to know what is visually and functionally missing             |
| P0       | Role-based navigation            | Platform must be understandable by shipper/carrier/forwarder/admin |
| P0       | Login/dashboard first impression | First screen defines confidence                                    |
| P1       | TMS order flow                   | Core product value                                                 |
| P1       | RFx/tender flow                  | Key enterprise logistics feature                                   |
| P1       | Billing/documents flow           | Important for operational closure                                  |
| P1       | Admin/low-code configuration     | Differentiator and internal control                                |
| P2       | Analytics/dashboard polish       | Useful after core flows are clearer                                |
| P2       | Integrations planning            | Important but should follow stable core flows                      |

## Recommended Next Development Packs

```text
1. PRODUCT_UI_AND_NAVIGATION_AUDIT_PACK
2. ROLE_BASED_CABINETS_GAP_ANALYSIS_PACK
3. TMS_ORDER_FLOW_GAP_ANALYSIS_PACK
4. RFX_TENDER_FLOW_GAP_ANALYSIS_PACK
5. BILLING_DOCUMENTS_FLOW_GAP_ANALYSIS_PACK
6. ADMIN_LOW_CODE_GAP_ANALYSIS_PACK
7. DASHBOARD_ANALYTICS_GAP_ANALYSIS_PACK
```

## Not In Scope Now

```text
Pilot users
External customer onboarding
Production deploy
Database migrations
API contract changes
New infrastructure
Server changes
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
