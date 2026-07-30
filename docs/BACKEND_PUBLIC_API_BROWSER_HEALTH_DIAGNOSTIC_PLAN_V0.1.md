# Backend Public API Browser Health Diagnostic Plan v0.1

## Summary

Plan for diagnosing why the production UI shows a backend-offline banner while public `/health` returns 200.

This is the required next diagnostic before backend/API execution.

## Decision

```text
BACKEND_PUBLIC_API_BROWSER_HEALTH_DIAGNOSTIC_PLAN_CREATED
```

## Known Facts

| Item                          | Result                          |
| ----------------------------- | ------------------------------- |
| public /health                | 200 application/json            |
| public /api/health            | 404 gateway ROUTE_NOT_FOUND     |
| public /api/                  | 404 gateway ROUTE_NOT_FOUND     |
| public /api/v1/companies      | 400 (route family exists)       |
| frontend expected health path | `{apiBaseUrl}/health`           |
| frontend apiBaseUrl (deployed)| `https://xn--80abvubqje.xn--p1ai` |
| mockAuth (deployed)           | false                           |
| backend-offline banner        | visible (signoff + login UI panel) |
| gateway CORS default (source)| localhost:3000,3001,5173 only   |
| browser DevTools in approval pack | not completed (automation environment) |

## Diagnostic Questions

| Question                                       | Why it matters                            |
| ---------------------------------------------- | ----------------------------------------- |
| Does browser call /health or /api/health?      | determines frontend/runtime path          |
| Does browser receive 200 from /health?         | confirms network availability             |
| Is there CORS error?                           | may explain curl/browser mismatch         |
| Is response JSON shape accepted by frontend?   | banner may be parsing/status logic        |
| Is Unicode domain vs punycode involved?        | may explain origin mismatch               |
| Is banner using another endpoint than /health? | may reveal different backend status check |
| Is request blocked by CSP/mixed content?       | browser-only failure                      |

## Proxy Evidence From Approval Pack (Read-Only)

| Check | Result |
|---|---|
| curl GET `https://xn--80abvubqje.xn--p1ai/health` | 200, gateway JSON `{"status":"ok","service":"api-gateway",...}` |
| curl GET with `Origin: https://xn--80abvubqje.xn--p1ai` | 200; no `Access-Control-Allow-Origin` observed in response headers |
| deployed login runtime config | `apiBaseUrl:"https://xn--80abvubqje.xn--p1ai"` |
| source health check | `useBackendStatus.ts` → `${apiBaseUrl}/health`, `response.ok` gate |

**Working hypothesis:** user opens `https://бинтранс.рф/login` (Unicode), frontend fetches `https://xn--80abvubqje.xn--p1ai/health` (punycode) → cross-origin request without CORS allowance → fetch fails → offline banner. Same-origin punycode visit may show online — must be confirmed in browser.

## Required Evidence (Browser Diagnostic Pack)

```text
1. Network request URL.
2. Status code.
3. Response type/body summary.
4. Console error summary if any.
5. Whether CORS/mixed-content/CSP error appears.
6. Whether banner disappears after successful health request.
7. Test both Unicode and punycode page URLs.
```

## Forbidden Evidence

```text
Do not record cookies.
Do not record JWT.
Do not record Authorization headers.
Do not record passwords.
Do not record private tokens.
```

## Possible Outcomes

```text
BROWSER_HEALTH_OK_BANNER_LOGIC_ISSUE
BROWSER_HEALTH_CORS_ORIGIN_ISSUE
BROWSER_HEALTH_PATH_MISMATCH
BROWSER_HEALTH_RESPONSE_SHAPE_MISMATCH
BROWSER_HEALTH_NETWORK_BLOCKED
BROWSER_HEALTH_UNKNOWN
```

## Current Pack Outcome

```text
BROWSER_RUNTIME_HEALTH_VERIFICATION_NOT_COMPLETED
```

Full interactive DevTools verification deferred to `BACKEND_PUBLIC_API_BROWSER_HEALTH_DIAGNOSTIC_PACK v0.1`.

## Next After Diagnostic

```text
If browser/runtime root cause is confirmed, prepare the minimal approval/execution pack for that exact fix.
```

Candidate paths after diagnostic:

- **CORS/origin fix** → Candidate A (gateway CORS origins and/or frontend apiBaseUrl alignment)
- **`/api/health` required** → Candidate B (gateway alias)
- **`/health` sufficient, same-origin OK** → Candidate C (documentation + demo script)
