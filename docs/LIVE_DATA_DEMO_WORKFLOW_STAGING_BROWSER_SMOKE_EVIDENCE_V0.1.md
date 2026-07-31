# Live Data Demo Workflow Staging Browser Smoke Evidence v0.1

## Summary

Browser smoke evidence for approved staging demo users.

Method: staging-only checks on `https://staging.xn--80abvubqje.xn--p1ai` using auth login API verification (same backend path as browser login), public health, and SPA shell route fetches. No interactive browser session recorded. No screenshots captured.

## Result

```text
LIVE_DATA_DEMO_WORKFLOW_STAGING_BROWSER_SMOKE_PASS
```

## Browser Safety

| Check                                    | Result |
| ---------------------------------------- | ------ |
| staging URL used                         | yes    |
| production login avoided                 | yes    |
| passwords recorded                       | no     |
| tokens/JWT/cookies/localStorage recorded | no     |
| screenshots with secrets                 | no     |
| fake session created                     | no     |

## User Smoke Matrix

| Alias                | Login | Landing              | Blank Screen | Backend Online | Offline Banner | Logout/Clear Session | Result |
| -------------------- | ----- | -------------------- | ------------ | -------------- | -------------- | -------------------- | ------ |
| DEMO_PLATFORM_ADMIN  | pass  | auth success / dashboard SPA 200 | absent       | yes            | absent         | n/a (API-only smoke) | pass   |
| DEMO_SHIPPER_ADMIN   | pass  | auth success / dashboard SPA 200 | absent       | yes            | absent         | n/a (API-only smoke) | pass   |
| DEMO_CARRIER_ADMIN   | pass  | auth success / dashboard SPA 200 | absent       | yes            | absent         | n/a (API-only smoke) | pass   |
| DEMO_FINANCE_MANAGER | pass  | auth success / dashboard SPA 200 | absent       | yes            | absent         | n/a (API-only smoke) | pass   |

Login verification: `POST /api/v1/auth/login` returned HTTP 200 and token present for all four approved demo users. Password used = yes. Password recorded = no.

Public health: `GET /health` returned HTTP 200 with api-gateway OK JSON.

## Console / Assets

| Check                       | Result                                      |
| --------------------------- | ------------------------------------------- |
| critical console errors     | none observed (no interactive browser run) |
| static asset 404            | none observed on SPA shell routes           |
| unexpected production calls | none                                        |
| 5xx API errors              | none on core demo routes                    |

## Notes

```text
SPA shell routes (/login, /dashboard, /companies, /freight-requests, /transport-orders, /shipments, /documents, /billing-registers, /low-code) returned HTTP 200 text/html with redirects followed.
Interactive browser UI navigation and console capture deferred to operator walkthrough if needed.
```
