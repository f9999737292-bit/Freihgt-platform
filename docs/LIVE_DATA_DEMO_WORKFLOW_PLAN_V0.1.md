# Live Data Demo Workflow Plan v0.1

## Summary

Plan for moving from signed-off static production UI walkthrough to controlled authenticated live-data demo workflow.

This pack is plan-only and docs-only. It does not create credentials, seed data, production records, source changes, backend changes, API changes, database writes, migrations, Nginx changes, deploys, or server changes.

Base commit: `86368ca` (`docs: sign off production demo scenario`).

Plan date: 2026-07-30.

## Decision

```text
LIVE_DATA_DEMO_WORKFLOW_PLAN_COMPLETE
```

## Current State

| Area                                | Status         |
| ----------------------------------- | -------------- |
| production static UI demo readiness | signed off     |
| backend public API canonical paths  | signed off     |
| demo scenario chain                 | closed         |
| controlled static walkthrough       | ready          |
| live-data demo readiness            | partial        |
| authenticated workflow readiness    | not signed off |
| full production readiness           | not claimed    |

## Recommended Demo Strategy

```text
Plan first, approve credentials/seed data second, execute only after explicit owner approval.
Prefer staging for first authenticated live-data workflow.
Use production only for controlled read-only/static walkthrough until demo credentials and seed data are explicitly approved.
```

## Recommended v0.1 Roles

| Role                | Demo Purpose                                           | Include in v0.1 |
| ------------------- | ------------------------------------------------------ | --------------- |
| PLATFORM_ADMIN      | platform overview / companies / health / admin concept | yes             |
| SHIPPER_ADMIN       | shipper dashboard, freight requests, transport orders  | yes             |
| CARRIER_ADMIN       | carrier shipment execution concept                     | yes             |
| FINANCE_MANAGER     | billing registers concept                              | yes             |
| FORWARDER_ADMIN     | forwarder scenario                                     | later           |
| CONSIGNEE_ADMIN     | consignee scenario                                     | later           |
| PROCUREMENT_MANAGER | procurement scenario                                   | later           |

## Recommended v0.1 Workflow

```text
1. Login as dedicated approved demo user.
2. Open dashboard.
3. Open companies.
4. Open freight requests / RFx.
5. Open transport orders.
6. Open shipments.
7. Open documents.
8. Open billing registers.
9. Optionally repeat with another approved demo role to show navigation difference.
10. Logout / close session.
```

## Minimum Demo Data Requirements

| Data                  | Minimum    |
| --------------------- | ---------- |
| demo tenant           | 1          |
| shipper company       | 1          |
| carrier company       | 1          |
| users                 | 2 minimum  |
| transport orders      | 3 statuses |
| shipments             | 2 statuses |
| freight request / RFx | 1          |
| billing register      | 1          |
| documents metadata    | 1–2        |

## Data Guardrails

```text
All demo data must be clearly marked DEMO.
No real customer data.
No real driver personal data.
No real legal/financial commitments.
No external notifications.
No production writes without explicit approval.
No real credentials in docs.
```

## Environment Recommendation

```text
Preferred first authenticated workflow: staging.
Production authenticated live-data demo requires separate approval and should use dedicated demo tenant/users only.
```

## Source Inspection Findings (read-only)

### Frontend pages requiring auth

All product pages under `apps/web-admin/pages/` except `/login` and guest entry redirect to login when unauthenticated (confirmed in demo smoke). Key list routes:

| Route | API families used |
|---|---|
| `/dashboard` | companies, users, transport-orders, rfx-events, shipments, documents, billing-registers |
| `/companies` | `/api/v1/companies` |
| `/users` | `/api/v1/users` |
| `/transport-orders` | `/api/v1/transport-orders` |
| `/freight-requests`, `/rfx` | `/api/v1/freight-requests`, `/api/v1/rfx-events` |
| `/shipments` | `/api/v1/shipments`, locations, cargoes |
| `/documents` | `/api/v1/documents` |
| `/billing-registers` | `/api/v1/billing-registers` |
| `/low-code` | `/api/v1/low-code/*` |
| `/control-tower` | aggregated list APIs |
| `/health` | gateway `/health` (technical) |

Login: `POST /api/v1/auth/login` via `apps/web-admin/stores/auth.ts`.

### Gateway route families (source)

From `services/api-gateway/internal/http/proxy.go`:

| Prefix | Service |
|---|---|
| `/api/v1/auth`, `/api/v1/users`, `/api/v1/roles` | identity-service |
| `/api/v1/companies` | company-service |
| `/api/v1/locations`, `/api/v1/cargoes`, `/api/v1/transport-orders` | transport-order-service |
| `/api/v1/rfx-events`, `/api/v1/freight-requests`, `/api/v1/bids` | rfx-service |
| `/api/v1/shipments`, `/api/v1/drivers`, `/api/v1/vehicles` | shipment-service |
| `/api/v1/documents`, `/api/v1/signing-sessions` | document-service |
| `/api/v1/billing-registers` | billing-register-service |
| `/api/v1/low-code` | low-code-service |

`/api/health` is **not** proxied — expected 404 at gateway.

### RBAC UI roles (source)

From `apps/web-admin/composables/usePermissions.ts`:

| Identity role | Product role | Landing route |
|---|---|---|
| PLATFORM_ADMIN | admin | `/dashboard` |
| SHIPPER_ADMIN, SHIPPER_LOGIST | shipper | `/dashboard` |
| CARRIER_ADMIN, CARRIER_DISPATCHER | carrier | `/shipments` |
| FORWARDER_MANAGER | forwarder | `/freight-requests` |
| CONSIGNEE_OPERATOR, CONSIGNEE_VIEWER | consignee | `/shipments` |
| FINANCE_MANAGER | finance | `/billing-registers` |
| PROCUREMENT_MANAGER | procurement | `/freight-requests` |

## Required Future Packs

```text
1. LIVE_DATA_DEMO_WORKFLOW_APPROVAL_PACK v0.1
2. DEMO_CREDENTIALS_AND_SEED_DATA_APPROVAL_PACK v0.1
3. LIVE_DATA_DEMO_WORKFLOW_EXECUTION_PACK v0.1
4. LIVE_DATA_DEMO_WORKFLOW_REVIEW_PACK v0.1
```

## Not Approved

```text
Demo credentials are not approved.
Seed data writes are not approved.
Production writes are not approved.
Authenticated production demo is not approved.
Backend/API/source changes are not approved.
```

## Safety Result

```text
Production changed in this pack: no
Production deploy executed in this pack: no
Staging deploy executed in this pack: no
Server changed in this pack: no
Nginx changed: no
Nginx reload executed: no
DNS changed: no
Certbot changed: no
Docker restarted: no
Backend changed: no
API contracts changed: no
Migrations changed: no
Database writes executed: no
Source code changed: no
Ports opened: no
Secrets captured: no
Credentials entered: no
Credentials created: no
Seed data created: no
Fake session created: no
Planning scope: live-data demo workflow only
```

## Next Recommended Pack

```text
LIVE_DATA_DEMO_WORKFLOW_APPROVAL_PACK v0.1
```

See also:

- `docs/LIVE_DATA_DEMO_ROLE_MATRIX_V0.1.md`
- `docs/LIVE_DATA_DEMO_API_REQUIREMENTS_V0.1.md`
- `docs/LIVE_DATA_DEMO_SEED_DATA_REQUIREMENTS_V0.1.md`
- `docs/LIVE_DATA_DEMO_WORKFLOW_RISK_MATRIX_V0.1.md`
- `docs/LIVE_DATA_DEMO_WORKFLOW_APPROVAL_CHECKLIST_V0.1.md`
