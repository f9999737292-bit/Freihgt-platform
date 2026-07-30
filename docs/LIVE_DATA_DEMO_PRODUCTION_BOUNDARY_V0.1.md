# Live Data Demo Production Boundary v0.1

## Summary

Production boundary for live-data demo work.

Base commit: `43ab15f`.

## Decision

```text
PRODUCTION_LIVE_DATA_DEMO_NOT_APPROVED
PRODUCTION_STATIC_WALKTHROUGH_ONLY
```

## Allowed Production Use

```text
Controlled static UI walkthrough only.
```

## Not Approved In Production

```text
Production demo credentials.
Production seed data.
Production authenticated workflow.
Production writes.
Production live-data external demo.
Fake production sessions.
Real credential entry.
```

## Production Claims Boundary

```text
Production static UI demo readiness is signed off.
Production live-data demo readiness is not signed off.
Full production readiness is not claimed.
```

## Required For Any Future Production Live-data Demo

```text
1. Staging-first workflow completed.
2. Dedicated production demo tenant/users approved.
3. Production seed data approved.
4. Production write approval.
5. Security/credentials handling approved.
6. Owner execution approval.
7. Rollback/cleanup plan.
```
