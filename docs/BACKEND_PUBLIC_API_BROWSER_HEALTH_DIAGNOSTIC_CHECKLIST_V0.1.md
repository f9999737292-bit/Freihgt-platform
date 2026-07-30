# Backend Public API Browser Health Diagnostic Checklist v0.1

## Baseline Checks

| Check | Result |
|---|---|
| production / healthy | yes — 200 |
| production /login healthy | yes — 200 (punycode + unicode) |
| production /health healthy | yes — 200 |
| production /api/health status recorded | yes — 404 |
| production /api/v1 representative route recorded | yes — 400 |
| staging healthy | yes — root/login/health 200 |

## Browser Checks

| Check | Result |
|---|---|
| DevTools Network reviewed | yes — Playwright network capture |
| health request URL captured | yes — `https://xn--80abvubqje.xn--p1ai/health` |
| health request status captured | yes — 200 |
| Console errors reviewed | yes — 404 on `/api/health` probe only |
| CORS checked | yes — no CORS error |
| mixed content checked | yes — no error |
| CSP checked | yes — no error |
| safe fetch('/health') tested | yes — 200 |
| safe fetch absolute punycode health tested | yes — 200 |
| safe fetch('/api/health') tested | yes — 404 |
| no secrets captured | yes |

## Source/Runtime Checks

| Check | Result |
|---|---|
| useBackendStatus reviewed | yes |
| banner logic reviewed | yes — login panel + BackendStatusBanner |
| deployed artifact markers reviewed | yes — prior pack + browser runtime `apiBaseUrl` |
| expected response shape recorded | yes — `response.ok` only |

## Safety Checks

| Check | Result |
|---|---|
| diagnostic is read-only | yes |
| no deploy executed | yes |
| no source code change | yes |
| no server change | yes |
| no Nginx change/reload | yes |
| no DNS/Certbot | yes |
| no Docker restart | yes |
| no backend/API/migration/DB change | yes |
| no ports opened | yes |
| no secrets captured | yes |

## Decision

```text
BACKEND_PUBLIC_API_BROWSER_HEALTH_DIAGNOSTIC_CHECKLIST_CREATED
```
