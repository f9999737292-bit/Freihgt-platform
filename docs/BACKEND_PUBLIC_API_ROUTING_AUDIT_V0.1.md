# Backend Public API Routing Audit v0.1

## Summary

Read-only routing audit for production public API readiness.

Base commit: `8558bfa` (`docs: sign off production demo readiness`).

## Public Endpoint Matrix

| Endpoint | Result | Notes |
|---|---|---|
| https://бинтранс.рф/ | 200 | UI — static SPA |
| https://бинтранс.рф/login | 200 | UI — static SPA |
| https://бинтранс.рф/health | 200 | api-gateway JSON `{"status":"ok","service":"api-gateway",...}` |
| https://бинтранс.рф/api/ | 404 | gateway `ROUTE_NOT_FOUND` JSON |
| https://бинтранс.рф/api/health | 404 | gateway `ROUTE_NOT_FOUND` JSON |
| https://бинтранс.рф/api/v1/companies | 400 | route reachable; missing tenant/query validation |

Punycode equivalents (`xn--80abvubqje.xn--p1ai`) return the same status codes.

## Staging Public Endpoint Matrix

| Endpoint | Result | Notes |
|---|---|---|
| https://staging.бинтранс.рф/ | 200 | UI |
| https://staging.бинтранс.рф/login | 200 | UI |
| https://staging.бинтранс.рф/health | 200 | api-gateway health |
| https://staging.бинтранс.рф/api/ | 404 | same gateway behavior as production |
| https://staging.бинтранс.рф/api/health | 404 | same gateway behavior as production |

## Internal Gateway Matrix

| Endpoint | Result | Notes |
|---|---|---|
| http://127.0.0.1:8080/health | 200 | gateway native health |
| http://127.0.0.1:8080/api/health | 404 | no gateway route |
| http://127.0.0.1:8080/ | 404 | no root route |
| http://127.0.0.1:8080/api/ | 404 | proxy handler; no prefix match |
| http://127.0.0.1:8080/routes | 200 | lists `/api/v1/*` prefixes only |
| http://127.0.0.1:8080/api/v1/auth/login | OPTIONS 204 | v1 auth route registered |

## Internal Services Health Matrix

| Service/Port | Result |
|---|---|
| 8081 /health | 200 |
| 8082 /health | 200 |
| 8083 /health | 200 |
| 8084 /health | 200 |
| 8085 /health | 200 |
| 8086 /health | 200 |
| 8087 /health | 200 |
| 8088 /health | 200 |

All backend containers reported healthy (`Up 2 weeks (healthy)`).

## Nginx Routing Findings

| Item | Result |
|---|---|
| production vhost found | yes — `00-bintrans-production.conf` |
| production root | `/var/www/bintrans-web-admin` → `bintrans-web-admin-release-20260717_193920` |
| staging root | `/var/www/staging-bintrans-web-admin` |
| roots distinct | yes |
| production /api location | present — `proxy_pass http://127.0.0.1:8080` |
| production /health location | present — `proxy_pass http://127.0.0.1:8080/health` |
| staging /api location | present — same pattern |
| staging /health location | present — same pattern |
| Nginx syntax | pass |
| Nginx reload executed | no |

## Frontend Runtime/API Findings

| Item | Result |
|---|---|
| source API base URL | `NUXT_PUBLIC_API_BASE_URL` → `https://xn--80abvubqje.xn--p1ai` in production build |
| deployed artifact API base URL marker | `apiBaseUrl:"https://xn--80abvubqje.xn--p1ai"`, `mockAuth:false` |
| backend health check path | `{apiBaseUrl}/health` — `apps/web-admin/composables/useBackendStatus.ts` |
| business API path pattern | `{apiBaseUrl}/api/v1/*` — `apps/web-admin/composables/useApi.ts` |
| mockAuth | false |
| backend-offline banner source | `login.vue` backend status panel; `BackendStatusBanner.vue` in authenticated shell |

## API 404 Source

```text
Application/gateway 404 — not Nginx static 404.
Body: {"error":{"code":"ROUTE_NOT_FOUND","message":"no route found for path","details":{}}}
```

Requests reach api-gateway via Nginx `/api/` proxy. The gateway `ProxyHandler` only matches `/api/v1/*` prefixes defined in `services/api-gateway/internal/http/proxy.go`. Paths `/api/` and `/api/health` have no matching prefix.

Public `/health` bypasses `/api/` and is proxied by dedicated Nginx `location = /health` to gateway `/health`.

## Routing Hypothesis

```text
Nginx public routing is correctly configured. The gateway exposes health at /health and business API at /api/v1/* only.
Exploratory paths /api/ and /api/health return gateway ROUTE_NOT_FOUND even though /health and /api/v1/* are reachable.
The frontend health probe uses /health (works via curl), but signoff records a visible backend-offline banner on login — likely browser-runtime fetch behavior or live-data conflation; approval pack must verify in-browser.
Live-data demo limitation is real for unauthenticated /api/health probes and authenticated flows requiring tenant/auth headers even when v1 routes respond.
```

## Decision

```text
BACKEND_PUBLIC_API_ROUTING_AUDIT_COMPLETE
```
