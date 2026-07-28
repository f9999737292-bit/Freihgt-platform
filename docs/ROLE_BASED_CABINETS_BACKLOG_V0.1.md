# Role-based Cabinets Backlog v0.1

## P0 — Role Navigation and RBAC Foundation

| ID | Task | Output |
| --- | --- | --- |
| ROLE-P0-001 | Define role list and role hierarchy | role model aligned with identity-service |
| ROLE-P0-002 | Define role-to-module access | access matrix (see ROLE_TO_MODULE_ACCESS_MATRIX_V0.1.md) |
| ROLE-P0-003 | Design web-admin role-based sidebar | navigation spec per role |
| ROLE-P0-004 | Replace RBAC TODO with explicit mapping plan | RBAC implementation plan from AUTH_RBAC.md |
| ROLE-P0-005 | Define first screen per role | role landing strategy (admin→dashboard, shipper→orders, etc.) |
| ROLE-P0-006 | Confirm hybrid strategy: web-admin first, role apps later | architecture decision record |
| ROLE-P0-007 | Block skeleton role apps from production deploy | deploy policy note |

## P1 — Cabinet UX

| ID | Task | Output |
| --- | --- | --- |
| ROLE-P1-001 | Shipper cabinet flow | shipper journey (orders, shipments, documents) |
| ROLE-P1-002 | Carrier cabinet flow | carrier journey (assigned shipments, bids, status updates) |
| ROLE-P1-003 | Forwarder cabinet flow | forwarder journey (freight-requests, rfx, delegated TMS) |
| ROLE-P1-004 | Consignee cabinet flow | consignee journey (receive, confirm, documents) |
| ROLE-P1-005 | Finance cabinet flow | finance journey (billing registers, approvals) |
| ROLE-P1-006 | Procurement cabinet flow | procurement journey (RFx, tenders, freight requests) |
| ROLE-P1-007 | Role-scoped dashboard cards | dashboard spec per role |
| ROLE-P1-008 | Hide admin-only modules from non-admin roles | nav filter rules |

## P2 — Separate Role Apps Strategy

| ID | Task | Output |
| --- | --- | --- |
| ROLE-P2-001 | Review web-shipper readiness | skeleton — 1 page, port 3001; not production-ready |
| ROLE-P2-002 | Review web-carrier readiness | skeleton — 1 page, port 3002; not production-ready |
| ROLE-P2-003 | Review web-consignee readiness | skeleton — 1 page, port 3003; not production-ready |
| ROLE-P2-004 | Review web-finance readiness | skeleton — 1 page, port 3004; not production-ready |
| ROLE-P2-005 | Review web-procurement readiness | skeleton — 1 page, port 3005; not production-ready |
| ROLE-P2-006 | Multi-app build/deploy pipeline plan | CI/CD strategy for role apps |
| ROLE-P2-007 | Evaluate dedicated forwarder app need | forwarder app decision note |

## Role App Readiness Snapshot (2026-07-28)

| App | Pages | Components | Composables | Auth | Production |
| --- | ---: | ---: | ---: | --- | --- |
| web-admin | 30 | 95 | 21 | yes | deployed |
| web-shipper | 1 | 0 | 0 | no | no |
| web-carrier | 1 | 0 | 0 | no | no |
| web-consignee | 1 | 0 | 0 | no | no |
| web-finance | 1 | 0 | 0 | no | no |
| web-procurement | 1 | 0 | 0 | no | no |

## Recommended Next Pack

```text
RBAC_AND_ROLE_NAVIGATION_DESIGN_PACK v0.1
```
