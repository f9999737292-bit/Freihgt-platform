# Backend Public API Readiness Plan v0.1

## Summary

Planning completed for removing the production live-data demo limitation caused by public `/api/*` returning 404 and backend-offline banner being visible in the production UI.

This pack is read-only and docs-only. No deployment, source code change, server change, Nginx change, DNS change, Certbot action, backend change, API change, migration, database write, Docker restart, or port exposure was executed.

## Decision

```text
BACKEND_PUBLIC_API_READINESS_PLAN_COMPLETE
```

## Current Status

| Area                                | Result      |
| ----------------------------------- | ----------- |
| production static UI demo readiness | signed off  |
| production live-data demo readiness | partial     |
| public /api/health                  | 404         |
| public /api/                        | 404         |
| backend-offline banner              | visible     |
| full production readiness           | not claimed |
| backend/API readiness               | not claimed |

## Root Cause Classification

```text
API_READINESS_ROOT_CAUSE_GATEWAY_ROUTE_MISSING
API_READINESS_ROOT_CAUSE_FRONTEND_BASE_URL_MISMATCH
```

Primary: the api-gateway has no routes for `/api/` or `/api/health`. Health is exposed at `/health`; business API routes are under `/api/v1/*` only. Nginx forwards `/api/` to the gateway correctly; the 404 body is gateway JSON `ROUTE_NOT_FOUND`, not an Nginx static 404.

Secondary (banner): the deployed frontend health check targets `{apiBaseUrl}/health` (not `/api/health`). Public `/health` returns HTTP 200 from curl, but the signoff still records a visible backend-offline banner on login. Browser-runtime verification (origin alignment, CORS, punycode vs Unicode host) is deferred to the approval pack.

## Findings

| Check                          | Result                  |
| ------------------------------ | ----------------------- |
| production /                   | pass — 200 text/html    |
| production /login              | pass — 200 text/html    |
| production /health             | pass — 200 application/json (api-gateway) |
| public /api/                   | 404 application/json (gateway ROUTE_NOT_FOUND) |
| public /api/health             | 404 application/json (gateway ROUTE_NOT_FOUND) |
| public /api/v1/companies       | 400 application/json (route reachable; tenant/auth required) |
| local gateway /health          | pass — 200              |
| local gateway /api/health      | fail — 404              |
| services health                | pass — 8081–8088 all 200 |
| Nginx /api proxy config        | present                 |
| Nginx /health proxy config     | present                 |
| frontend API base URL          | `https://xn--80abvubqje.xn--p1ai` (source + deployed artifact) |
| deployed artifact API base URL | `https://xn--80abvubqje.xn--p1ai`, `mockAuth:false` |
| frontend backend health path   | `{apiBaseUrl}/health` via `useBackendStatus.ts` |
| frontend data API paths        | `{apiBaseUrl}/api/v1/*` via `useApi.ts` |
| backend containers             | 10 healthy (gateway + 8 services + postgres) |

## Target State

```text
Production frontend can reach backend health/data through an approved public API route.
Backend-offline banner is not shown during normal production demo.
Public API exposure is deliberate, minimal, secured, and documented.
```

## Possible Fix Options

### Option A — Nginx `/api/` prefix routes to api-gateway

```text
Expose a controlled /api/ reverse proxy to api-gateway, preserving frontend /api base URL.
```

Use only if:

* gateway supports routes under `/api/*`, or Nginx rewrite is planned correctly;
* public exposure is acceptable;
* rate limiting/security headers/auth boundaries are documented.

**Current state:** production and staging Nginx already proxy `location /api/` to `http://127.0.0.1:8080`. This option alone does not fix `/api/health` 404 because the gateway lacks that route.

### Option B — Frontend uses public gateway path without `/api`

```text
Adjust frontend runtime API base URL to match already-working public gateway route.
```

Use only if:

* public `/health` and relevant gateway routes are already intentionally exposed;
* frontend build/runtime config can be updated safely;
* static artifact refresh is approved separately.

**Current state:** frontend already uses origin root as `apiBaseUrl` and `/health` for health checks. Business calls already use `/api/v1/*`. No base-URL change is required for v1 routing; banner investigation should focus on browser runtime health fetch behavior.

### Option C — Gateway route normalization

```text
Add/enable gateway routes for expected public health/API paths.
```

Use only if:

* current gateway lacks `/api/health` route;
* backend services are healthy internally;
* API contract change is approved separately.

**Recommended candidate:** add `/api/health` alias (or informational `/api/` root) in api-gateway, or document that public health is `/health` only and align monitoring/docs accordingly.

### Option D — Keep public API disabled and prepare static-only demo

```text
Do not expose backend publicly yet; prepare controlled static demo and record live-data limitation.
```

Use only if:

* customer demo does not require live data;
* security/public API readiness is deferred.

## Recommended Path

```text
Recommended: BACKEND_PUBLIC_API_READINESS_APPROVAL_PACK v0.1 before any execution.
```

The approval pack must choose exactly one execution strategy and define:

* public API path (`/health` vs `/api/health` vs both);
* gateway target and any route aliases;
* Nginx/no-Nginx scope (likely no Nginx change if gateway alias suffices);
* frontend artifact scope (likely none unless browser health fetch fix requires rebuild);
* security guardrails (CORS allowed origins, auth boundaries for `/api/v1/*`);
* rollback path;
* post-deploy browser verification on `/login` backend status panel.

## Not Approved

```text
No backend/API/Nginx/source/server change is approved by this plan.
No public API exposure change is approved by this plan.
No production deploy is approved by this plan.
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
Planning scope: backend public API readiness only
```

## Next Recommended Pack

```text
BACKEND_PUBLIC_API_READINESS_APPROVAL_PACK v0.1
```
