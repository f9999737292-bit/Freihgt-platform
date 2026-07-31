# Live Data Demo Workflow Staging Smoke Runbook v0.1

## Summary

Runbook for staging live-data demo workflow smoke after staging backend/DB isolation and demo seed data creation.

## Scope

```text
Target environment: staging only
Production login: forbidden
Production writes: forbidden
Production live-data demo: not approved
Credentials/tokens in docs/repo/chat: forbidden
```

## Approved Demo Users

| Alias                | Email                                                                             | Role            |
| -------------------- | --------------------------------------------------------------------------------- | --------------- |
| DEMO_PLATFORM_ADMIN  | [demo-platform-admin@bintrans.local](mailto:demo-platform-admin@bintrans.local)   | PLATFORM_ADMIN  |
| DEMO_SHIPPER_ADMIN   | [demo-shipper-admin@bintrans.local](mailto:demo-shipper-admin@bintrans.local)     | SHIPPER_ADMIN   |
| DEMO_CARRIER_ADMIN   | [demo-carrier-admin@bintrans.local](mailto:demo-carrier-admin@bintrans.local)     | CARRIER_ADMIN   |
| DEMO_FINANCE_MANAGER | [demo-finance-manager@bintrans.local](mailto:demo-finance-manager@bintrans.local) | FINANCE_MANAGER |

## Required Gates

```text
1. Git/source safety gate.
2. Endpoint baseline.
3. Staging isolation gate.
4. Secure credential handling.
5. Browser login smoke.
6. Role navigation smoke.
7. Staging API/list smoke where safe.
8. Post-smoke endpoint verification.
```

## Demo Route Scope

```text
dashboard
companies
freight/RFx
transport orders
shipments
documents
billing registers
low-code admin if PLATFORM_ADMIN only and safe
```

## Decision

```text
LIVE_DATA_DEMO_WORKFLOW_STAGING_SMOKE_RUNBOOK_CREATED
```
