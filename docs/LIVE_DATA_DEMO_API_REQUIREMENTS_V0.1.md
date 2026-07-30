# Live Data Demo API Requirements v0.1

## Summary

API requirements for controlled live-data demo workflow.

Base commit: `86368ca`.

## Canonical Paths

| Purpose | Path |
|---|---|
| health | /health |
| business API | /api/v1/* |
| /api/health | not canonical / expected 404 |

## Required API Families For Demo

| Demo Area | Required API Family | Gateway Service | Required For v0.1 |
|---|---|---|---|
| login/auth | `/api/v1/auth/login` | identity-service | yes |
| dashboard | list APIs below or graceful fallback | multiple | yes |
| companies | `/api/v1/companies` | company-service | yes |
| freight requests / RFx | `/api/v1/freight-requests`, `/api/v1/rfx-events` | rfx-service | yes |
| transport orders | `/api/v1/transport-orders` | transport-order-service | yes |
| shipments | `/api/v1/shipments` | shipment-service | yes |
| documents | `/api/v1/documents` | document-service | yes |
| billing registers | `/api/v1/billing-registers` | billing-register-service | yes |
| users (admin) | `/api/v1/users` | identity-service | optional |
| low-code | `/api/v1/low-code/*` | low-code-service | optional / admin only |
| shipment detail extras | `/api/v1/locations`, `/api/v1/cargoes` | transport-order-service | optional |

## Endpoint Baseline (read-only, no auth)

| Endpoint | Production | Staging |
|---|---|---|
| `/` | 200 text/html | 200 text/html |
| `/login` | 200 text/html | 200 text/html |
| `/health` | 200 application/json | 200 application/json |
| `/api/v1/companies` | 400 (route exists, auth/tenant required) | not re-checked |
| `/api/health` | 404 expected | not re-checked |

## Readiness Questions

```text
1. Does login return usable token/session for demo user?
2. Do APIs return demo-scoped records?
3. Do list pages render without critical errors?
4. Do empty states look acceptable if data is missing?
5. Are write buttons hidden/disabled or safe for demo?
6. Are errors/fallbacks acceptable for external demo?
```

## Required Future Verification

```text
1. Login smoke with approved demo credentials.
2. API list smoke per demo module.
3. Browser route smoke after authentication.
4. Role navigation smoke for selected roles.
5. No real data exposure verification.
6. Logout/session cleanup.
```

## Not Approved

```text
No API changes are approved by this plan.
No source changes are approved by this plan.
No production writes are approved by this plan.
```

## Decision

```text
LIVE_DATA_DEMO_API_REQUIREMENTS_CREATED
```
