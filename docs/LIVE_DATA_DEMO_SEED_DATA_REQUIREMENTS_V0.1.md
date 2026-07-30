# Live Data Demo Seed Data Requirements v0.1

## Summary

Seed data requirements for controlled live-data demo.

This document does not create seed data.

Base commit: `86368ca`.

## Minimum Dataset

| Entity | Count | Notes |
|---|---:|---|
| demo tenant | 1 | clearly marked DEMO |
| shipper company | 1 | demo only |
| carrier company | 1 | demo only |
| forwarder company | 0–1 | optional |
| consignee company | 0–1 | optional |
| demo users | 2–4 | no real credentials in docs |
| freight request / RFx | 1 | simple scenario |
| transport orders | 3 | draft/new, in progress, completed |
| shipments | 2 | in transit, delivered/documents completed |
| billing register | 1 | demo-only |
| document metadata | 1–2 | no real legal docs |

## Data Naming Rules

```text
Use DEMO prefix in names.
Example only:
DEMO Shipper LLC
DEMO Carrier LLC
DEMO Transport Order 001
DEMO Billing Register 001
```

## Existing Seed Reference (staging, not executed in this pack)

Prior demo seed plan (`docs/LOW_CODE_PILOT_WEEK3_DEMO_SEED_PLAN_V0.1.md`) documents staging seed targets including:

- tenant `74519f22-ff9b-4a8b-8fff-a958c689682f` (dev-bintrans)
- users: admin@bintrans.local, shipper@bintrans.local (exist); carrier/forwarder/consignee users needed
- demo entities: DEMO-TO-*, DEMO-SH-*, etc. via `make seed-demo-data`

Passwords must be provided separately — **never recorded in docs**.

## Data Safety Rules

```text
No real customer data.
No real personal data.
No real driver data.
No real bank/account data.
No legally binding documents.
No external notifications.
No production writes without explicit approval.
```

## Environment Recommendation

```text
Create/verify seed data in staging first.
Production seed data requires separate explicit approval.
```

## Decision

```text
LIVE_DATA_DEMO_SEED_DATA_REQUIREMENTS_CREATED
```
