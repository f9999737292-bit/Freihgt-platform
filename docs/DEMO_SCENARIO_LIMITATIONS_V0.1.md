# Demo Scenario Limitations v0.1

## Summary

Known limitations for the current production demo scenario.

Base commit: `60fa973`.

## Signed Off

```text
Controlled static UI walkthrough.
Login screen cleanliness.
Backend status online via /health.
SPA route availability without Nginx 404.
```

## Not Signed Off

```text
Authenticated role workflow.
Live operational data demo.
Full backend/API readiness.
Full E2E business process readiness.
SLA readiness.
Security readiness.
Legal/document/billing readiness.
```

## Current Technical Boundaries

| Area                        | Status                    |
| --------------------------- | ------------------------- |
| production static UI        | signed off                |
| login cleanliness           | fixed                     |
| backend status path         | /health                   |
| business API route family   | /api/v1/*                 |
| /api/health                 | expected 404, not blocker |
| live-data demo              | partial                   |
| authenticated demo workflow | not signed off            |

## Route Behavior Without Authentication

| Behavior | Impact on demo |
|---|---|
| unauthenticated routes redirect to login | safe for login-centric walkthrough |
| authenticated page content not visible | cannot demo live lists/forms without credentials |
| product routes return SPA shell at HTTP level | can explain product map, not live data |

## Risk For External Demo

| Risk                                | Mitigation                                             |
| ----------------------------------- | ------------------------------------------------------ |
| audience expects live data          | state upfront this is static UI walkthrough            |
| auth workflow not ready             | do not enter credentials or promise authenticated demo |
| API errors appear during navigation | frame as live-data limitation                          |
| fallback pages appear               | use prepared route list and avoid unverified flows     |
| overclaiming readiness              | use required disclaimer                                |

## Decision

```text
DEMO_SCENARIO_LIMITATIONS_RECORDED
```
