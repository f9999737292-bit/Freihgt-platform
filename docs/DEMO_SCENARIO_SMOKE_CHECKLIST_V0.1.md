# Demo Scenario Smoke Checklist v0.1

## Endpoint Checks

| Check | Result |
|---|---|
| production / healthy | yes — 200 |
| production /login healthy | yes — 200 |
| production /health healthy | yes — 200 |
| production SPA routes not Nginx 404 | yes — all listed routes 200 |
| production /api/v1 representative route exists | yes — 400 |
| staging healthy | yes — root/login/health 200 |

## Login Checks

| Check | Result |
|---|---|
| login opens | yes |
| no blank screen | yes |
| email field empty | yes |
| password field empty | yes |
| demo prefill absent | yes |
| backend status online | yes |
| offline banner absent | yes |

## Demo Route Checks

| Route | Status | Demo-safe |
|---|---|---|
| / | pass — redirect login | partial |
| /login | pass | yes |
| /dashboard | redirect-login | partial |
| /shipments | redirect-login | partial |
| /freight-requests | redirect-login | partial |
| /billing-registers | redirect-login | partial |
| /transport-orders | redirect-login | partial |
| /documents | redirect-login | partial |
| /companies | redirect-login | partial |
| /low-code | redirect-login | partial |

## Safety Checks

| Check | Result |
|---|---|
| smoke is read-only | yes |
| no deploy executed | yes |
| no source code change | yes |
| no server change | yes |
| no Nginx change/reload | yes |
| no DNS/Certbot | yes |
| no Docker restart | yes |
| no backend/API/migration/DB change | yes |
| no ports opened | yes |
| no secrets captured | yes |
| no credentials entered | yes |
| no fake session created | yes |
| full production readiness not claimed | yes |

## Decision

```text
DEMO_SCENARIO_SMOKE_CHECKLIST_CREATED
```
