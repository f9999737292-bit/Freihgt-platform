# Staging Backend DB Isolation Data Safety v0.1

## Summary

Data safety rules for creating an isolated staging backend/database.

Base commit: `5f5ad4c`.

## Decision

```text
STAGING_DATA_SAFETY_BOUNDARY_CREATED
```

## Required Safety Properties

| Property               | Requirement                          |
| ---------------------- | ------------------------------------ |
| production DB          | not modified                         |
| staging DB             | separate container/volume            |
| migrations             | staging-only during future execution |
| demo data              | staging-only DEMO records            |
| production credentials | not used                             |
| staging secrets        | server-only, not committed           |
| DB backups             | backup before execution              |
| external notifications | disabled/avoided                     |

## Production Data Rules

```text
Do not copy production customer data into staging.
Do not write demo records into production.
Do not use real customer/personal/driver/financial/legal data.
Do not expose production DB externally.
```

## Staging Data Rules

```text
All staging demo records must be marked DEMO.
Use synthetic data only.
Use staging-only credentials.
Run migrations only against staging DB in future execution.
```

## Secret Rules

```text
No DB passwords in repo/docs/chat.
No JWT/tokens/cookies in evidence.
No .env.staging committed.
No private keys copied.
```
