# Demo Credentials and Seed Data Staging Execution Runbook v0.2

## Summary

Runbook for retrying demo credentials and seed data creation after staging backend/DB isolation passed.

## Scope

```text
Target environment: isolated staging only
Staging API: 127.0.0.1:18080
Production API: 127.0.0.1:8080
Production writes: forbidden
Production live-data demo: not approved
```

## Required Gates

```text
1. Git/source safety gate.
2. Endpoint baseline.
3. Staging isolation gate.
4. Supported staging seed method gate.
5. Secret handling gate.
6. Post-execution endpoint verification.
```

## Approved Staging Aliases

| Alias                | Role            |
| -------------------- | --------------- |
| DEMO_PLATFORM_ADMIN  | PLATFORM_ADMIN  |
| DEMO_SHIPPER_ADMIN   | SHIPPER_ADMIN   |
| DEMO_CARRIER_ADMIN   | CARRIER_ADMIN   |
| DEMO_FINANCE_MANAGER | FINANCE_MANAGER |

## Approved Dataset

| Entity            | Alias/Name                       |
| ----------------- | -------------------------------- |
| tenant            | DEMO_BINTRANS_TENANT             |
| company           | DEMO Shipper Company             |
| company           | DEMO Carrier Company             |
| RFx               | DEMO RFx 001                     |
| transport order   | DEMO Transport Order Draft       |
| transport order   | DEMO Transport Order In Progress |
| transport order   | DEMO Transport Order Completed   |
| shipment          | DEMO Shipment In Transit         |
| shipment          | DEMO Shipment Delivered          |
| billing register  | DEMO Billing Register 001        |
| document metadata | DEMO Document Metadata 001       |

## Supported Method

```text
scripts/dev/seed_dev_admin.sh
scripts/dev/seed_demo_data.sh
Post-seed staging API calls via http://127.0.0.1:18080/api/v1/*
Server-only secret directory for generated passwords
```

## Decision

```text
DEMO_CREDENTIALS_AND_SEED_DATA_STAGING_EXECUTION_RUNBOOK_V0_2_CREATED
```
