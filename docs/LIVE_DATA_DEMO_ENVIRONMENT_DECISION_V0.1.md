# Live Data Demo Environment Decision v0.1

## Summary

Environment decision for future authenticated live-data demo workflow.

Base commit: `43ab15f`.

## Decision

```text
LIVE_DATA_DEMO_ENVIRONMENT_STAGING_FIRST_APPROVED
```

## Selected Environment

```text
Staging-first.
```

## Rationale

```text
Staging-first reduces risk before any authenticated production demo.
Production is already signed off for controlled static UI walkthrough, but production live-data/authenticated workflow is not signed off.
```

## Production Boundary

```text
Production may be used for static UI walkthrough only.
Production live-data demo, production seed data, production demo credentials, and production writes are not approved.
```

## Staging Boundary

```text
Staging is selected for future authenticated live-data workflow execution, but staging writes are not approved in this pack.
A separate credentials/seed data approval and execution pack is required.
```

## Required Before Staging Execution

```text
1. Approved demo tenant.
2. Approved demo users.
3. Approved seed dataset.
4. Approved credentials handling.
5. Explicit staging execution approval.
6. Cleanup/rollback plan.
```

## Not Approved

```text
No staging writes are approved here.
No production writes are approved here.
No login is approved here.
No credentials creation is approved here.
```
