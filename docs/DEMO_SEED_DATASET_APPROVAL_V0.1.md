# Demo Seed Dataset Approval v0.1

## Summary

Approved future staging demo seed dataset boundary.

This document does not create data.

Base commit: `47144b1`.

## Decision

```text
DEMO_SEED_DATASET_APPROVED_FOR_FUTURE_STAGING_EXECUTION
DEMO_SEED_DATA_NOT_CREATED_IN_THIS_PACK
```

## Approved Dataset

| Entity                 | Required | Count |
| ---------------------- | -------- | ----: |
| DEMO tenant            | yes      |     1 |
| DEMO shipper company   | yes      |     1 |
| DEMO carrier company   | yes      |     1 |
| DEMO forwarder company | optional |   0–1 |
| DEMO consignee company | optional |   0–1 |
| DEMO users             | yes      |     4 |
| RFx/freight request    | yes      |     1 |
| transport orders       | yes      |     3 |
| shipments              | yes      |     2 |
| billing register       | yes      |     1 |
| document metadata      | yes      |   1–2 |

## Required Naming Convention

```text
Every seeded record must include DEMO in name, title, code, or description.
```

## Example Safe Names

```text
DEMO Bintrans Tenant
DEMO Shipper Company
DEMO Carrier Company
DEMO RFx 001
DEMO Transport Order Draft
DEMO Transport Order In Progress
DEMO Transport Order Completed
DEMO Shipment In Transit
DEMO Shipment Delivered
DEMO Billing Register 001
DEMO Document Metadata 001
```

## Transport Order Status Aliases

```text
DEMO_TO_DRAFT
DEMO_TO_IN_PROGRESS
DEMO_TO_COMPLETED
```

## Shipment Status Aliases

```text
DEMO_SHIPMENT_IN_TRANSIT
DEMO_SHIPMENT_DELIVERED
```

## Forbidden Data

```text
Real customer data.
Real personal data.
Real driver data.
Real vehicle owner data.
Real bank/account data.
Real legally binding documents.
Real invoices/UPD/acts.
External notifications.
Production data writes.
```

## Future Execution Requirement

```text
Create seed data only in staging after explicit execution approval.
Production seed data requires separate explicit approval.
```
