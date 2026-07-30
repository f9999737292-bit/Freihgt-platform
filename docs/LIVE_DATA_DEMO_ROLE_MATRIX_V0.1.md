# Live Data Demo Role Matrix v0.1

## Summary

Role matrix for controlled authenticated live-data demo planning.

Base commit: `86368ca`.

## v0.1 Recommended Roles

| Role | Landing Area | Demo Objective | Risk |
|---|---|---|---|
| PLATFORM_ADMIN | dashboard/admin areas | show platform overview and tenant/admin concept | high if admin actions are writable |
| SHIPPER_ADMIN | dashboard / freight requests / transport orders | show shipper-side logistics workflow | medium |
| CARRIER_ADMIN | shipments | show carrier-side execution workflow | medium |
| FINANCE_MANAGER | billing registers | show financial workflow concept | medium/high if real finance data exists |

## v0.2 Deferred Roles

| Role | Reason Deferred |
|---|---|
| FORWARDER_ADMIN | separate scenario complexity |
| CONSIGNEE_ADMIN | requires consignee-specific data |
| PROCUREMENT_MANAGER | overlaps with shipper/RFx for first demo |

## Role-to-Navigation Mapping (source)

From `apps/web-admin/composables/usePermissions.ts`:

| Product role | Primary nav areas |
|---|---|
| admin | all routes including `/low-code`, `/health`, `/users` |
| shipper | dashboard, control-tower, transport-orders, freight-requests, rfx, shipments, documents, billing-registers, companies, users |
| carrier | dashboard, shipments, transport-orders, freight-requests, rfx, documents, billing-registers, companies, users |
| finance | dashboard, control-tower, documents, billing-registers, transport-orders, shipments, companies, users, health |

## Role Guardrails

```text
Use dedicated demo users only.
Do not use real employee/customer accounts.
Do not record passwords in docs.
Do not grant production write/admin permissions unless explicitly approved.
Prefer read-only or demo-scoped permissions for external walkthrough.
```

## Required Before Use

```text
1. Approved demo tenant.
2. Approved demo users.
3. Approved role permissions.
4. Approved seed data.
5. Explicit environment decision: staging first or production demo.
```

## Decision

```text
LIVE_DATA_DEMO_ROLE_MATRIX_CREATED
```
