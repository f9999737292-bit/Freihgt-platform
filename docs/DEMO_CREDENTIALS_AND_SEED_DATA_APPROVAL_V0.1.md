# Demo Credentials and Seed Data Approval v0.1

## Summary

Approval boundary completed for future staging-first demo credentials and seed data.

This pack is approval-only and docs-only. It does not create credentials, passwords, users, tenants, companies, transport orders, shipments, RFx records, documents, billing registers, source changes, backend changes, API changes, database writes, migrations, Nginx changes, deploys, Docker restarts, or server changes.

Base commit: `47144b1` (`docs: approve live data demo workflow boundary`).

Approval date: 2026-07-30.

## Decision

```text
DEMO_CREDENTIALS_AND_SEED_DATA_APPROVAL_COMPLETE
DEMO_CREDENTIALS_STAGING_FIRST_APPROVED_FOR_FUTURE_EXECUTION
DEMO_SEED_DATA_STAGING_FIRST_APPROVED_FOR_FUTURE_EXECUTION
DEMO_CREDENTIALS_NOT_CREATED_IN_THIS_PACK
DEMO_SEED_DATA_NOT_CREATED_IN_THIS_PACK
PRODUCTION_DEMO_CREDENTIALS_NOT_APPROVED
PRODUCTION_SEED_DATA_NOT_APPROVED
STAGING_WRITES_NOT_EXECUTED
PRODUCTION_WRITES_NOT_EXECUTED
```

## Environment

```text
Staging-first.
```

Production remains approved only for controlled static walkthrough. Production live-data demo is not approved.

## Approved Future Staging Demo Tenant Alias

```text
DEMO_BINTRANS_TENANT
```

## Approved Future Staging Demo User Aliases

| Alias                | Role            | Purpose                                       |
| -------------------- | --------------- | --------------------------------------------- |
| DEMO_PLATFORM_ADMIN  | PLATFORM_ADMIN  | platform overview / companies / admin concept |
| DEMO_SHIPPER_ADMIN   | SHIPPER_ADMIN   | shipper workflow                              |
| DEMO_CARRIER_ADMIN   | CARRIER_ADMIN   | carrier shipment workflow                     |
| DEMO_FINANCE_MANAGER | FINANCE_MANAGER | billing register concept                      |

## Password / Secret Policy

```text
Passwords must not be stored in repo, docs, chat, screenshots, terminal logs, or tickets.
Passwords may be generated only during a separately approved execution pack.
Credentials must be communicated only through an approved secure owner channel.
Tokens/JWT/cookies/localStorage must never be recorded.
```

## Approved Future Staging Seed Dataset

| Entity                 | Count | Notes                             |
| ---------------------- | ----: | --------------------------------- |
| DEMO tenant            |     1 | DEMO_BINTRANS_TENANT              |
| DEMO shipper company   |     1 | clearly marked DEMO               |
| DEMO carrier company   |     1 | clearly marked DEMO               |
| DEMO forwarder company |   0–1 | optional                          |
| DEMO consignee company |   0–1 | optional                          |
| DEMO users             |     4 | approved role aliases only        |
| RFx/freight request    |     1 | demo-only                         |
| transport orders       |     3 | draft/new, in progress, completed |
| shipments              |     2 | in transit, delivered             |
| billing register       |     1 | demo-only                         |
| document metadata      |   1–2 | metadata only, no real legal docs |

## Data Guardrails

```text
All demo records must contain DEMO in name/description.
No real customer data.
No real personal data.
No real driver data.
No real financial/bank data.
No legally binding documents.
No external notifications.
No production writes.
```

## Not Approved In This Pack

```text
Credential creation.
Password generation.
User creation.
Tenant creation.
Seed data creation.
Staging writes.
Production writes.
Login execution.
Fake sessions.
Source/backend/API changes.
Database migrations/writes.
Nginx changes.
Deploys.
```

## Required Before Execution

```text
1. Explicit owner approval for DEMO_CREDENTIALS_AND_SEED_DATA_STAGING_EXECUTION_PACK v0.1.
2. Confirm staging target only.
3. Confirm exact tenant/user/data creation method.
4. Confirm credentials delivery channel.
5. Confirm cleanup/rollback rules.
6. Confirm no production writes.
```

## Safety Result

```text
Production changed in this pack: no
Production deploy executed in this pack: no
Staging deploy executed in this pack: no
Staging writes executed in this pack: no
Server changed in this pack: no
Nginx changed: no
Nginx reload executed: no
DNS changed: no
Certbot changed: no
Docker restarted: no
Backend changed: no
API contracts changed: no
Migrations changed: no
Database writes executed: no
Source code changed: no
Ports opened: no
Secrets captured: no
Credentials entered: no
Credentials created: no
Passwords generated: no
Seed data created: no
Fake session created: no
Approval scope: future staging demo credentials and seed data only
```

## Next Recommended Pack

```text
DEMO_CREDENTIALS_AND_SEED_DATA_STAGING_EXECUTION_PACK v0.1
```

See also:

- `docs/DEMO_CREDENTIAL_HANDLING_POLICY_V0.1.md`
- `docs/DEMO_SEED_DATASET_APPROVAL_V0.1.md`
- `docs/DEMO_CREDENTIALS_AND_SEED_DATA_STAGING_EXECUTION_BOUNDARY_V0.1.md`
- `docs/DEMO_CREDENTIALS_AND_SEED_DATA_CLEANUP_PLAN_V0.1.md`
- `docs/DEMO_CREDENTIALS_AND_SEED_DATA_APPROVAL_BOUNDARY_V0.1.md`
