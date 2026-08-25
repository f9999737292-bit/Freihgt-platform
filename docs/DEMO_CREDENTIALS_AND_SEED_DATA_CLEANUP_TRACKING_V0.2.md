# Demo Credentials and Seed Data Cleanup Tracking v0.2

## Summary

Cleanup tracking for staging demo credentials and seed data.

## Created Objects

Do not include passwords/tokens.

| Object | Alias/Name | Safe ID | Cleanup Action |
|---|---|---|
| tenant | DEMO_BINTRANS_TENANT | da13ede3-e957-4618-965f-926807fc643e | keep/disable/archive/delete later |
| user | DEMO_PLATFORM_ADMIN | 180b792b-4ff9-4dfb-95a6-cefce0952c0a | disable/rotate later |
| user | DEMO_SHIPPER_ADMIN | f48cf684-1b87-4931-b107-12dd76b110c2 | disable/rotate later |
| user | DEMO_CARRIER_ADMIN | 4e16d805-7fc6-4a18-924c-e3b33776db1b | disable/rotate later |
| user | DEMO_FINANCE_MANAGER | 9af37ff6-866d-46b4-9d89-1a50c3247eb3 | disable/rotate later |
| company | DEMO Shipper Company | 7803f029-47e1-4321-a41a-d89d0e86b413 | archive/delete later |
| company | DEMO Carrier Company | 1addc7ec-2345-4012-8cf0-1342ec6de856 | archive/delete later |
| RFx | DEMO-RFX-001 | 68999503-0af6-4da8-83c3-f7b710faf064 | archive/delete later |
| transport orders | DEMO-TO-001..005 | multiple | archive/delete later |
| shipments | DEMO-SH-PLANNED / IN-PROGRESS / BILLING | multiple | archive/delete later |
| billing register | DEMO-BR-001 | 7bcbf89f-de18-42f4-a550-f99f5cda9717 | archive/delete later |
| document metadata | DEMO-DOC-001 | a8d08846-abcc-487b-bf7c-90b362362378 | archive/delete later |

## Canonical Seed Users (dev tenant)

| Email | Note |
|---|---|
| admin@bintrans.local | PLATFORM_ADMIN via `seed-dev-admin` |
| shipper@bintrans.local | SHIPPER_LOGIST demo user |
| carrier@bintrans.local | CARRIER_DISPATCHER demo user |
| forwarder@bintrans.local | PROCUREMENT_MANAGER demo user |
| consignee@bintrans.local | CONSIGNEE_OPERATOR demo user |

Staging may still contain pre-A2.5 legacy identity rows until controlled migration (Wave A2.5). Use canonical `@bintrans.local` credentials only.

## Legacy staging rows (pre-A2.5)

| Status | Note |
|---|---|
| Pending A2.5 migration | Legacy `@7rights.local` rows may still exist on staging alongside approved alias users — see staging execution evidence |

## Rules

```text
No cleanup executed in this pack unless explicitly required and approved.
Prefer disable/archive over destructive delete.
No production cleanup is approved.
Server secret directory: /root/bintrans-staging-demo-secrets-20260731_131617 (server-only, not in repo).
```

## Decision

```text
DEMO_CREDENTIALS_AND_SEED_DATA_CLEANUP_TRACKING_V0_2_CREATED
```
