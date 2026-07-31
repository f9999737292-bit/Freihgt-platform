# Live Data Demo Staging Presentation Safety Boundary v0.1

## Summary

Safety boundary for controlled staging presentation.

## Hard Rules

| Rule | Status |
|---|---|
| use staging URL only | required |
| production login | forbidden |
| production writes | forbidden |
| staging writes during presentation | avoid unless separately approved |
| passwords in slides/docs/chat | forbidden |
| tokens/JWT/cookies/localStorage capture | forbidden |
| real customer data | forbidden |
| external notifications | forbidden |
| destructive actions | forbidden |

## Approved URL

```text
https://staging.xn--80abvubqje.xn--p1ai/
```

## Forbidden URLs For Demo Login

```text
https://xn--80abvubqje.xn--p1ai/
https://бинтранс.рф/
```

## Staging Limitation

```text
staging AUTH_ENABLED=false
role-based API denial was not verified
this presentation is not a full RBAC/security enforcement demo
```

## Safe Demo Wording

```text
This is a controlled staging demo with synthetic DEMO data.
It demonstrates product workflow readiness, not production operational readiness.
```

## Unsafe Demo Wording

```text
Do not say:
- production is ready for live operations;
- RBAC enforcement is fully verified;
- real billing/legal document workflow is production-approved;
- production live-data demo is approved.
```

## Decision

```text
LIVE_DATA_DEMO_STAGING_PRESENTATION_SAFETY_BOUNDARY_RECORDED
```
