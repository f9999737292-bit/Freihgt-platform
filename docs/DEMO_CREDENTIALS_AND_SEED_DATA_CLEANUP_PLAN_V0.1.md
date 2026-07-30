# Demo Credentials and Seed Data Cleanup Plan v0.1

## Summary

Cleanup and rollback plan for future staging demo credentials and seed data.

This document does not delete or create anything.

Base commit: `47144b1`.

## Decision

```text
DEMO_CREDENTIALS_AND_SEED_DATA_CLEANUP_PLAN_CREATED
```

## Cleanup Options

| Option                     | Meaning                                      |
| -------------------------- | -------------------------------------------- |
| disable demo users         | safest after demo if records should remain   |
| rotate demo passwords      | keep users but invalidate old access         |
| archive demo tenant/data   | keep audit trail but remove from active demo |
| delete demo records        | only if safe and approved                    |
| leave staging demo dataset | allowed if marked DEMO and owner approves    |

## Required Tracking

```text
Every future created demo user/tenant/record must be listed in execution evidence by alias/id where safe.
Do not record passwords/tokens.
```

## Rollback Rules

```text
Rollback/cleanup must be staging-only unless separate production approval exists.
Do not run destructive deletes without explicit owner approval.
Prefer disable/archive over delete for auditability.
```

## Not Approved

```text
No cleanup is executed here.
No delete is executed here.
No production rollback is approved here.
```
