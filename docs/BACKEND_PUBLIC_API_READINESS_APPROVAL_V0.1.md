# Backend Public API Readiness Approval v0.1

## Summary

Approval boundary prepared for future backend public API readiness work.

This approval pack is read-only and docs-only. It does not approve or execute production changes, Nginx changes, backend changes, API contract changes, Docker restarts, database writes, source changes, or port exposure.

Base commit: `83a730d` (`docs: plan backend public api readiness`).

## Decision

```text
BACKEND_PUBLIC_API_READINESS_APPROVAL_BOUNDARY_COMPLETE
```

## Current State

| Area                                    | Result                                             |
| --------------------------------------- | -------------------------------------------------- |
| production static UI demo readiness     | signed off                                         |
| live-data demo readiness                | partial                                            |
| public /health                          | 200 application/json (api-gateway)                 |
| public /api/                            | 404 application/json (gateway ROUTE_NOT_FOUND)     |
| public /api/health                      | 404 application/json (gateway ROUTE_NOT_FOUND)     |
| public /api/v1/companies                | 400 application/json (route exists; validation)    |
| backend-offline banner                  | visible per signoff; browser DevTools not completed in this pack |
| browser health proxy finding            | deployed `apiBaseUrl` is punycode; `/health` returns 200; no `Access-Control-Allow-Origin` for production Origin in curl probe |
| Nginx /api proxy                        | present                                            |
| Nginx /health proxy                     | present                                            |
| Nginx change selected                   | no                                                 |
| production change approved by this pack | no                                                 |

## Selected Approval Boundary

```text
No production execution is approved in this pack.
No Nginx change is selected as primary path.
Future work must first verify browser/runtime health behavior, then choose minimal gateway/frontend remediation.
```

## Selected Future Strategy

```text
Strategy: API_READINESS_BROWSER_FIRST_GATEWAY_NORMALIZATION
```

### Stage A — Browser/runtime health verification

Confirm in browser why the backend-offline banner is visible even though public `/health` returns 200.

Required checks:

1. Open `https://бинтранс.рф/login` (Unicode host) and `https://xn--80abvubqje.xn--p1ai/login` (punycode host).
2. DevTools → Network: identify health request URL, status, CORS/mixed-content/CSP errors.
3. Compare with deployed runtime config: `apiBaseUrl:"https://xn--80abvubqje.xn--p1ai"`, health path `{apiBaseUrl}/health`.

Proxy finding from this pack (not a substitute for browser DevTools):

- Health endpoint responds 200 to curl.
- Gateway default CORS allows localhost origins only (`CORS_ALLOWED_ORIGINS` default in source).
- Cross-origin fetch from Unicode host to punycode `apiBaseUrl` may fail without CORS headers — candidate root cause for offline banner.

### Stage B — Gateway route normalization (only if required)

If public consistency requires `/api/health`, add a gateway alias in a separate approved execution pack.

If browser diagnostic confirms `/health` is sufficient and same-origin works, prefer documentation-only canonical path (Candidate C).

### Stage C — Post-fix static/live-data demo review

After any approved execution:

1. Re-check production `/`, `/login`, `/health`, `/api/health` (if added), representative `/api/v1/*`.
2. Verify login backend status panel and authenticated live-data demo scope.
3. Do not claim full production readiness from API path readiness alone.

## Approved Future Execution Candidates

Only after separate explicit execution approval:

### Candidate A — Browser/runtime health fix

```text
If backend-offline banner is caused by frontend runtime logic, CORS, origin mismatch, or health parsing, prepare a source/frontend/backend targeted fix.
```

Likely scope if CORS confirmed: add production/staging origins to gateway `CORS_ALLOWED_ORIGINS`, or align `apiBaseUrl` with page origin (relative URL / same host form).

### Candidate B — Gateway `/api/health` alias

```text
If public API readiness requires /api/health, add a gateway route alias from /api/health to gateway health response.
```

### Candidate C — Document canonical paths only

```text
If frontend and demo can rely on /health plus /api/v1/*, document canonical paths and do not add /api/health.
```

## Not Selected Now

```text
Nginx /api proxy change is not selected now because /api requests already reach the gateway and return gateway/application ROUTE_NOT_FOUND.
Opening internal ports publicly is not selected.
Database or migration work is not selected.
```

## Explicitly Not Approved

```text
Production execution.
Production deploy.
Staging deploy.
Nginx edits.
Nginx reload.
DNS changes.
Certbot actions.
Docker restart.
Backend deploy.
API contract changes.
Source code changes.
Database migrations/writes.
Opening ports.
Secrets/private key handling.
```

## Required Before Execution

```text
1. Browser/runtime health verification result (BACKEND_PUBLIC_API_BROWSER_HEALTH_DIAGNOSTIC_PACK v0.1).
2. Selected exact fix path (Candidate A, B, or C).
3. Security guardrails.
4. Rollback plan.
5. Pre/post endpoint checks.
6. Explicit execution approval from owner.
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
```

## Next Recommended Pack

```text
BACKEND_PUBLIC_API_BROWSER_HEALTH_DIAGNOSTIC_PACK v0.1
```

Alternative:

```text
BACKEND_PUBLIC_API_GATEWAY_ROUTE_NORMALIZATION_APPROVAL_PACK v0.1
```
