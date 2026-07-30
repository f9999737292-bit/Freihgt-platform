# Demo Scenario Smoke Evidence v0.1

## Summary

Controlled production demo scenario smoke completed for the signed-off production static UI.

This smoke is read-only. No deployment, source code change, server change, Nginx change, DNS change, Certbot action, Docker restart, backend change, API change, migration, database write, or port exposure was executed.

Base commit: `60fa973` (`docs: sign off backend public api canonical paths`).

Smoke date: 2026-07-30.

## Decision

```text
DEMO_SCENARIO_SMOKE_COMPLETE
```

## Classification

```text
DEMO_SCENARIO_STATIC_WALKTHROUGH_READY
DEMO_SCENARIO_LIVE_DATA_PARTIAL
DEMO_SCENARIO_AUTHENTICATED_WORKFLOW_NOT_SIGNED_OFF
```

## Baseline

| Check                          | Result              |
| ------------------------------ | ------------------- |
| production /                   | 200 text/html       |
| production /login                | 200 text/html       |
| production /health               | 200 application/json |
| production /api/v1/companies     | 400 application/json |
| production /api/health           | 404 expected        |
| staging /                      | 200 text/html       |
| staging /login                 | 200 text/html       |
| staging /health              | 200 application/json |

## Production SPA Route Curl Smoke

| Route | Result |
|---|---|
| `/` | 200 text/html |
| `/login` | 200 text/html |
| `/dashboard` | 200 text/html |
| `/shipments` | 200 text/html |
| `/freight-requests` | 200 text/html |
| `/billing-registers` | 200 text/html |
| `/transport-orders` | 200 text/html |
| `/documents` | 200 text/html |
| `/companies` | 200 text/html |
| `/low-code` | 200 text/html |
| `/health` | 200 application/json |

No Nginx 404 observed on listed routes.

## Browser Smoke

Browser entry host: `https://бинтранс.рф/` (redirects to punycode login).

| Scenario                | Result | Demo-safe |
| ----------------------- | ------ | --------- |
| root entry              | pass — redirects to login, UI loaded | partial |
| login screen            | pass — clean login, backend online | yes |
| dashboard route         | redirect-login | partial |
| shipments route         | redirect-login | partial |
| freight requests route  | redirect-login | partial |
| billing registers route | redirect-login | partial |
| transport orders route  | redirect-login | partial |
| documents route         | redirect-login | partial |
| companies route         | redirect-login | partial |
| low-code route          | redirect-login | partial |
| health endpoint         | pass — gateway JSON | technical only |

Unauthenticated product routes redirect to `/login/` via SPA guest middleware. This is expected and safe for static walkthrough framing, but authenticated page content is not visible without credentials.

## Login Cleanliness

| Check                 | Result |
| --------------------- | ------ |
| login page opens      | pass |
| email field empty     | pass |
| password field empty  | pass |
| demo email absent     | pass |
| demo password absent  | pass |
| backend status online | pass — «Backend доступен» |
| offline banner absent | pass |
| blank screen absent   | pass |

## Browser Console Summary

| Check                   | Result |
| ----------------------- | ------ |
| critical console errors | no |
| static asset 404        | no |
| auth/API errors visible | no |
| blank screen            | no |

## Demo Interpretation

```text
Production static UI is ready for a controlled walkthrough centered on entry/login and product-area concept.
Live-data demo remains partial.
Authenticated workflow is not signed off.
No full production readiness is claimed.
```

## What Can Be Shown

```text
1. Production entry/login screen.
2. Clean login without demo prefill.
3. Product route concept and SPA availability (routes return UI shell; unauthenticated routes redirect to login).
4. RBAC/product concept as UI capability, with caveat that authenticated role views are not signed off.
5. Health/backend status as technical proof only (/health JSON).
```

## What Should Not Be Promised

```text
1. Full production readiness.
2. Full backend/API readiness.
3. Full live-data demo readiness.
4. Authenticated role workflow readiness.
5. SLA/security/legal/document/billing/E2E readiness.
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
Fake session created: no
Smoke scope: controlled production static UI demo walkthrough
```

## Next Recommended Pack

```text
DEMO_SCENARIO_FINAL_SIGNOFF_PACK v0.1
```

Alternative:

```text
LIVE_DATA_DEMO_WORKFLOW_PLAN_PACK v0.1
```
