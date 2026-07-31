# Live Data Demo Staging Presentation Checklist v0.1

## Summary

Checklist before running the controlled staging demo.

## 30 Minutes Before Demo

| Check | Result |
|---|---|
| staging root opens | todo |
| staging login opens | todo |
| staging health returns 200 | todo |
| production root unchanged | todo |
| demo passwords available through secure channel | todo |
| no passwords in slides/docs/chat | todo |
| browser profile/session clean | todo |
| screen sharing does not expose terminal secrets | todo |

## During Demo

| Rule | Result |
|---|---|
| verify URL is staging before login | todo |
| do not open production login | todo |
| do not paste passwords into chat | todo |
| do not open DevTools token/session storage | todo |
| do not create new data unless approved | todo |
| do not send external notifications | todo |
| avoid destructive actions | todo |

## After Demo

| Check | Result |
|---|---|
| logout/clear session | todo |
| confirm no screenshots with secrets | todo |
| record feedback separately | todo |
| keep production live-data demo not approved | todo |

## Demo Success Criteria

```text
1. Staging login works for approved users.
2. Core routes render without blank screen.
3. Demo data is visible.
4. No production login or writes.
5. No secrets captured.
6. Limitations are clearly communicated.
```

## Decision

```text
LIVE_DATA_DEMO_STAGING_PRESENTATION_CHECKLIST_CREATED
```
