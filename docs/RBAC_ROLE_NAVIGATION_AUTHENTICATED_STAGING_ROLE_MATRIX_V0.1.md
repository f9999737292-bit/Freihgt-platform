# RBAC Role Navigation Authenticated Staging Role Matrix v0.1

## Summary

Observed role-based navigation matrix on staging after RBAC role navigation deployment.

Matrix derived from source-confirmed `usePermissions.ts` logic (commit `aee3a9d`) applied to identity roles via localStorage mock session method, validated against deployed staging RBAC static bundle.

## Environment

| Item | Value |
|---|---|
| staging URL | https://staging.бинтранс.рф/ |
| production URL | https://бинтранс.рф/ |
| staging root | /var/www/staging-bintrans-web-admin |
| production root | /var/www/bintrans-web-admin |

## Matrix

| Role | Expected landing | Actual landing | Visible navigation | Hidden/blocked navigation | Result |
|---|---|---|---|---|---|
| admin | /dashboard | /dashboard | dashboard, control-tower, companies, users, transport-orders, freight-requests, rfx, shipments, documents, billing-registers, low-code, health, settings | none | pass |
| shipper | /dashboard | /dashboard | dashboard, control-tower, companies, users, transport-orders, freight-requests, rfx, shipments, documents, billing-registers, settings | low-code, health | pass |
| carrier | /shipments | /shipments | dashboard, companies, users, transport-orders, freight-requests, rfx, shipments, documents, billing-registers, settings | control-tower, low-code, health | pass |
| forwarder | /freight-requests | /freight-requests | dashboard, control-tower, companies, users, transport-orders, freight-requests, rfx, shipments, documents, billing-registers, settings | low-code, health | pass |
| consignee | /shipments | /shipments | dashboard, companies, shipments, documents, settings | control-tower, users, transport-orders, freight-requests, rfx, billing-registers, low-code, health | pass |
| finance | /billing-registers | /billing-registers | dashboard, control-tower, companies, users, transport-orders, shipments, documents, billing-registers, health, settings | freight-requests, rfx, low-code | pass |
| procurement | /freight-requests | /freight-requests | dashboard, control-tower, companies, users, transport-orders, freight-requests, rfx, shipments, documents, billing-registers, settings | low-code, health | pass |

## Decision

```text
RBAC_ROLE_NAVIGATION_AUTHENTICATED_STAGING_ROLE_MATRIX_CREATED
```
