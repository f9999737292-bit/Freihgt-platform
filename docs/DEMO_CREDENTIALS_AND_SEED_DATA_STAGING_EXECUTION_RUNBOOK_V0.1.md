# Demo Credentials and Seed Data Staging Execution Runbook v0.1

## Summary

Runbook for staging-only demo credentials and seed data execution.

User provided explicit staging write approval. Production writes remain forbidden.

Base commit: `8f7eaeb`.

## Scope

```text
Execution target: staging only
Production writes: forbidden
Production live-data demo: not approved
Credentials/secrets in docs/repo/chat: forbidden
```

## Required Gates

```text
1. Git/source safety gate.
2. Endpoint baseline.
3. Staging isolation gate.
4. Supported staging seed method gate.
5. Secret handling gate.
```

## STOP Conditions

```text
Stop if staging is not isolated from production.
Stop if staging and production share API/backend/DB data scope.
Stop if the supported seed method is not found.
Stop if any production URL/DB/tenant would be used.
Stop if passwords/tokens would be recorded.
Stop if source/backend/API/migration/Nginx changes are required.
```

## Approved Staging Aliases

| Alias                | Role            |
| -------------------- | --------------- |
| DEMO_PLATFORM_ADMIN  | PLATFORM_ADMIN  |
| DEMO_SHIPPER_ADMIN   | SHIPPER_ADMIN   |
| DEMO_CARRIER_ADMIN   | CARRIER_ADMIN   |
| DEMO_FINANCE_MANAGER | FINANCE_MANAGER |

## Approved Demo Dataset

| Entity            | Name/Alias                       |
| ----------------- | -------------------------------- |
| tenant            | DEMO_BINTRANS_TENANT             |
| shipper company   | DEMO Shipper Company             |
| carrier company   | DEMO Carrier Company             |
| RFx               | DEMO RFx 001                     |
| transport order   | DEMO Transport Order Draft       |
| transport order   | DEMO Transport Order In Progress |
| transport order   | DEMO Transport Order Completed   |
| shipment          | DEMO Shipment In Transit         |
| shipment          | DEMO Shipment Delivered          |
| billing register  | DEMO Billing Register 001        |
| document metadata | DEMO Document Metadata 001       |

## Decision

```text
DEMO_CREDENTIALS_AND_SEED_DATA_STAGING_EXECUTION_RUNBOOK_CREATED
```
