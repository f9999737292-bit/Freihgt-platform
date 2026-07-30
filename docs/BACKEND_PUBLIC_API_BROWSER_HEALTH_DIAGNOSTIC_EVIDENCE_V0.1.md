# Backend Public API Browser Health Diagnostic Evidence v0.1

## Summary

Browser/runtime health diagnostic completed for production backend-offline banner.

This pack is read-only. No deployment, source code change, server change, Nginx change, DNS change, Certbot action, backend change, API change, Docker restart, migration, database write, or port exposure was executed.

Base commit: `48b11c3` (`docs: approve backend public api readiness boundary`).

Diagnostic method: Playwright headless (Microsoft Edge channel) against production login URLs, with safe in-page `fetch()` probes. No credentials, cookies, tokens, or storage contents recorded.

## Decision

```text
BACKEND_PUBLIC_API_BROWSER_HEALTH_DIAGNOSTIC_COMPLETE
```

## Baseline

| Check                        | Result              |
| ---------------------------- | ------------------- |
| production /                 | 200 text/html       |
| production /login (punycode) | 200 text/html       |
| production /login (unicode)  | 200 text/html       |
| production /health           | 200 application/json |
| production /api/health       | 404 application/json |
| production /api/v1/companies | 400 application/json |
| staging /                    | 200 text/html       |
| staging /login               | 200 text/html       |
| staging /health              | 200 application/json |

## Health Response Shape

| Item                              | Result |
| --------------------------------- | ------ |
| /health status                    | 200 |
| /health content-type              | application/json |
| /health body summary              | `{"status":"ok","service":"api-gateway","version":"0.1.0",...}` |
| CORS headers with punycode Origin | missing (not required for same-origin after redirect) |
| CORS headers with unicode Origin  | missing in curl probe; browser cross-origin fetch succeeded after redirect to punycode host |
| matches frontend expectation      | yes — `response.ok` gate only; JSON shape not parsed |

## Frontend Health Code Findings

| Item                       | Result |
| -------------------------- | ------ |
| configured apiBaseUrl      | `https://xn--80abvubqje.xn--p1ai` (deployed + source via `NUXT_PUBLIC_API_BASE_URL`) |
| health path used by source | `{apiBaseUrl}/health` — `apps/web-admin/composables/useBackendStatus.ts` |
| expected success condition | `response.ok` (HTTP 2xx) |
| expected response shape    | any JSON body accepted; no field validation |
| banner visibility logic    | login: panel always shown; offline styling when `!backendOnline`; `BackendStatusBanner.vue` shows when offline/checking/dev/mockAuth |

## Browser DevTools Findings

### Punycode entry: `https://xn--80abvubqje.xn--p1ai/login`

| Item                                         | Result |
| -------------------------------------------- | ------ |
| page URL after load                          | `https://xn--80abvubqje.xn--p1ai/login/` |
| health request found                         | yes |
| health request URL                           | `https://xn--80abvubqje.xn--p1ai/health` |
| health request status                        | 200 |
| response content-type                        | application/json |
| response body summary                        | gateway ok JSON |
| CORS error                                   | no |
| mixed content error                          | no |
| CSP error                                    | no |
| backend-offline banner visible after request | no — online panel, text `Backend доступен` |

### Unicode entry: `https://бинтранс.рф/login`

| Item                                         | Result |
| -------------------------------------------- | ------ |
| page URL after load                          | redirects to `https://xn--80abvubqje.xn--p1ai/login/` |
| health request found                         | yes |
| health request URL                           | `https://xn--80abvubqje.xn--p1ai/health` |
| health request status                        | 200 |
| response content-type                        | application/json |
| response body summary                        | gateway ok JSON |
| CORS error                                   | no |
| mixed content error                          | no |
| CSP error                                    | no |
| backend-offline banner visible after request | no — online panel, text `Backend доступен` |

Console note (both hosts): one 404 resource error for `/api/health` during manual safe fetch probe; not used by `useBackendStatus`.

## Browser Console Fetch Tests

| Fetch | Punycode page | Unicode page (redirected) |
| ----- | ------------- | ------------------------ |
| `fetch('/health')` | ok=true, status=200, type=basic | ok=true, status=200, type=basic |
| `fetch('https://xn--80abvubqje.xn--p1ai/health')` | ok=true, status=200, type=basic | ok=true, status=200, type=basic |
| `fetch('/api/health')` | ok=false, status=404 | ok=false, status=404 |

## Diagnostic Classification

```text
BROWSER_HEALTH_OK_BANNER_NOT_REPRODUCED
BROWSER_HEALTH_API_HEALTH_404_EXPECTED
```

Interpretation:

- Public `/health` works in browser; frontend health check succeeds; login backend status shows **online**.
- Prior signoff "backend-offline banner visible" is **not reproduced** in this diagnostic run.
- `/api/health` returns 404 as expected (gateway route missing); frontend does **not** use `/api/health` for banner logic.
- Live-data demo remains **partial** due to authenticated API/demo workflow scope, not due to gateway health reachability at `/health`.

## Recommended Next Path

```text
BACKEND_PUBLIC_API_CANONICAL_PATH_SIGNOFF_PACK v0.1
```

Alternative if live-data demo is priority without `/api/health` alias:

```text
DEMO_SCENARIO_SMOKE_PACK v0.1
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
Diagnostic scope: browser/runtime health only
```
