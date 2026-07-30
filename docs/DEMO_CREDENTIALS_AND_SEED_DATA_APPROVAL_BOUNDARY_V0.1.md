# Demo Credentials and Seed Data Approval Boundary v0.1

## Summary

Boundary for future demo credentials and seed data approval.

This document does not create credentials or seed data.

Base commit: `43ab15f`.

## Decision

```text
DEMO_CREDENTIALS_AND_SEED_DATA_APPROVAL_BOUNDARY_CREATED
```

## Future Credentials Requirements

| Item                    | Requirement                                                   |
| ----------------------- | ------------------------------------------------------------- |
| account type            | dedicated demo users only                                     |
| environment             | staging first                                                 |
| roles                   | PLATFORM_ADMIN, SHIPPER_ADMIN, CARRIER_ADMIN, FINANCE_MANAGER |
| password storage        | never in repo/docs/chat                                       |
| token/session handling  | never recorded                                                |
| real user accounts      | forbidden                                                     |
| shared real credentials | forbidden                                                     |

## Future Seed Data Requirements

| Item                   | Requirement                       |
| ---------------------- | --------------------------------- |
| tenant                 | clearly marked DEMO               |
| companies              | demo-only shipper/carrier minimum |
| transport orders       | demo-only, 3 statuses             |
| shipments              | demo-only, 2 statuses             |
| RFx/freight request    | demo-only, 1 record               |
| billing register       | demo-only, 1 record               |
| documents              | metadata only, no real legal docs |
| external notifications | disabled/avoided                  |

## Explicitly Not Approved

```text
Credential creation.
Password generation.
Seed data creation.
Database writes.
Production writes.
Staging writes.
Login execution.
Fake sessions.
```

## Required Next Pack

```text
DEMO_CREDENTIALS_AND_SEED_DATA_APPROVAL_PACK v0.1
```
