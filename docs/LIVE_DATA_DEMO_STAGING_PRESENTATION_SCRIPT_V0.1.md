# Live Data Demo Staging Presentation Script v0.1

## Decision

```text
LIVE_DATA_DEMO_STAGING_PRESENTATION_SCRIPT_CREATED
CONTROLLED_STAGING_DEMO_SCRIPT_READY
PRODUCTION_LIVE_DATA_DEMO_NOT_APPROVED
```

## Purpose

This document defines a controlled staging demo script for the Bintrans live-data workflow.

The demo uses only synthetic DEMO data in the isolated staging environment.

## Demo Boundary

```text
Environment: staging only
URL: https://staging.xn--80abvubqje.xn--p1ai/
Data: synthetic DEMO data only
Production login: forbidden
Production writes: forbidden
Production live-data demo: not approved
Secrets in docs/repo/chat: forbidden
```

## Recommended Demo Duration

| Section                 |      Time |
| ----------------------- | --------: |
| Opening and boundary    |     2 min |
| Platform admin overview |     4 min |
| Shipper workflow        |     6 min |
| Carrier workflow        |     4 min |
| Finance workflow        |     4 min |
| Summary and limitations |     3 min |
| Total                   | 20–25 min |

## Approved Demo Users

| Order | Alias                | Email                                                                             | Role            |
| ----: | -------------------- | --------------------------------------------------------------------------------- | --------------- |
|     1 | DEMO_PLATFORM_ADMIN  | [demo-platform-admin@bintrans.local](mailto:demo-platform-admin@bintrans.local)   | PLATFORM_ADMIN  |
|     2 | DEMO_SHIPPER_ADMIN   | [demo-shipper-admin@bintrans.local](mailto:demo-shipper-admin@bintrans.local)     | SHIPPER_ADMIN   |
|     3 | DEMO_CARRIER_ADMIN   | [demo-carrier-admin@bintrans.local](mailto:demo-carrier-admin@bintrans.local)     | CARRIER_ADMIN   |
|     4 | DEMO_FINANCE_MANAGER | [demo-finance-manager@bintrans.local](mailto:demo-finance-manager@bintrans.local) | FINANCE_MANAGER |

## Opening Statement

```text
Today we are showing a controlled staging demo of the Bintrans logistics platform.
This is not production. The demo uses synthetic DEMO data only.
The goal is to show the end-to-end business workflow: companies, RFx, transport orders, shipments, documents, and billing registers.
```

## Key Demo Message

```text
Bintrans is moving from a static product demonstration to an authenticated staging workflow with real backend/API data, isolated staging database, and synthetic demo records.
```

## Closing Statement

```text
The staging workflow is signed off for controlled demo/read-list presentation.
Production live-data demo is not approved yet.
Full RBAC/API denial enforcement is a separate next step because staging AUTH_ENABLED=false.
```
