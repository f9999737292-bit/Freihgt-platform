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
DEMO_CREDENTIALS_AND_SEED_DATA_STAGING_EXECUTION_PACK v0.1
```

## Approval Boundary Update v0.1

```text
DEMO_CREDENTIALS_AND_SEED_DATA_APPROVAL_COMPLETE
DEMO_CREDENTIALS_STAGING_FIRST_APPROVED_FOR_FUTURE_EXECUTION
DEMO_SEED_DATA_STAGING_FIRST_APPROVED_FOR_FUTURE_EXECUTION
DEMO_CREDENTIALS_NOT_CREATED_IN_THIS_PACK
DEMO_SEED_DATA_NOT_CREATED_IN_THIS_PACK
PRODUCTION_DEMO_CREDENTIALS_NOT_APPROVED
PRODUCTION_SEED_DATA_NOT_APPROVED
```

## Approved Future Staging Aliases

| Alias                | Role            |
| -------------------- | --------------- |
| DEMO_PLATFORM_ADMIN  | PLATFORM_ADMIN  |
| DEMO_SHIPPER_ADMIN   | SHIPPER_ADMIN   |
| DEMO_CARRIER_ADMIN   | CARRIER_ADMIN   |
| DEMO_FINANCE_MANAGER | FINANCE_MANAGER |

## Approved Future Tenant Alias

```text
DEMO_BINTRANS_TENANT
```

## Production Boundary

```text
Production credentials and production seed data remain not approved.
```
