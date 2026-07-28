# Role-based Sidebar Navigation Spec v0.1

## Summary

Sidebar navigation specification for web-admin role-based UI.

## Decision

```text
ROLE_BASED_SIDEBAR_NAVIGATION_SPEC_CREATED
```

## Current State

```text
Source: apps/web-admin/components/layout/AppSidebar.vue
Current: static navItems array — all 13 routes visible to every authenticated user
Target: filter navItems by resolved product role + permission map
```

## Navigation Groups

| Group               | Routes                         |
| ------------------- | ------------------------------ |
| Overview            | /dashboard, /control-tower     |
| Transport           | /transport-orders, /shipments  |
| Procurement         | /freight-requests, /rfx        |
| Documents & Finance | /documents, /billing-registers |
| Administration      | /companies, /users, /low-code  |
| System              | /settings, /health             |

## Admin Sidebar

```text
/dashboard
/control-tower
/transport-orders
/freight-requests
/rfx
/shipments
/documents
/billing-registers
/companies
/users
/low-code
/settings
/health
```

## Shipper Sidebar

```text
/dashboard
/control-tower
/transport-orders
/freight-requests
/rfx
/shipments
/documents
/billing-registers
/companies
/users
/settings
```

## Carrier Sidebar

```text
/dashboard
/shipments
/transport-orders
/freight-requests
/rfx
/documents
/billing-registers
/companies
/users
/settings
```

## Forwarder Sidebar

```text
/dashboard
/control-tower
/transport-orders
/freight-requests
/rfx
/shipments
/documents
/billing-registers
/companies
/users
/settings
```

## Consignee Sidebar

```text
/dashboard
/shipments
/documents
/companies
/settings
```

## Finance Sidebar

```text
/dashboard
/control-tower
/documents
/billing-registers
/transport-orders
/shipments
/companies
/users
/settings
/health
```

## Procurement Sidebar

```text
/dashboard
/control-tower
/freight-requests
/rfx
/transport-orders
/shipments
/documents
/billing-registers
/companies
/users
/settings
```

## Implementation Notes

```text
1. Resolve product role from identity roles[] (map table in RBAC_ROLE_PERMISSION_MATRIX_V0.1.md).
2. Filter AppSidebar navItems by role sidebar spec above.
3. Optional: group nav items visually by Navigation Groups.
4. Post-login redirect: use First Screen by Role table below.
5. Direct URL access: enforce access-denied UX even if sidebar hides route.
```

## Access-denied UX

```text
If a user opens a route directly but lacks permission:
1. show access denied state
2. provide link back to role dashboard
3. do not expose data
4. log frontend event if telemetry exists later
```

## First Screen by Role

| Role        | First Screen                 | Post-login redirect target |
| ----------- | ---------------------------- | -------------------------- |
| admin       | /dashboard or /control-tower | /dashboard                 |
| shipper     | /dashboard                   | /dashboard                 |
| carrier     | /shipments                   | /shipments                 |
| forwarder   | /freight-requests            | /freight-requests          |
| consignee   | /shipments                   | /shipments                 |
| finance     | /billing-registers           | /billing-registers         |
| procurement | /freight-requests            | /freight-requests          |

## Next

```text
RBAC_AND_ROLE_NAVIGATION_IMPLEMENTATION_PLAN_PACK v0.1
```
