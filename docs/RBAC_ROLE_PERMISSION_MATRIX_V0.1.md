# RBAC Role Permission Matrix v0.1

## Summary

Initial RBAC role-permission matrix for web-admin role-based navigation planning.

This matrix is planning-only and does not change runtime permissions.

## Roles

```text
admin
shipper
carrier
forwarder
consignee
finance
procurement
```

## Matrix

| Permission               | Admin | Shipper           | Carrier            | Forwarder         | Consignee  | Finance   | Procurement |
| ------------------------ | ----- | ----------------- | ------------------ | ----------------- | ---------- | --------- | ----------- |
| dashboard.view           | yes   | yes               | yes                | yes               | yes        | yes       | yes         |
| control_tower.view       | yes   | limited           | limited            | yes               | limited    | limited   | yes         |
| transport_orders.view    | yes   | own               | assigned           | delegated         | related    | financial | view        |
| transport_orders.create  | yes   | yes               | no                 | delegated         | no         | no        | no          |
| transport_orders.manage  | yes   | own               | no                 | delegated         | no         | no        | no          |
| freight_requests.view    | yes   | own               | available/assigned | delegated         | no         | no        | yes         |
| freight_requests.create  | yes   | yes               | no                 | delegated         | no         | no        | yes         |
| freight_requests.respond | yes   | no                | yes                | yes               | no         | no        | no          |
| rfx.view                 | yes   | own               | invited            | delegated         | no         | cost view | yes         |
| rfx.create               | yes   | yes               | no                 | delegated         | no         | no        | yes         |
| rfx.respond              | yes   | no                | yes                | yes               | no         | no        | no          |
| shipments.view           | yes   | own               | assigned           | delegated         | related    | view      | view        |
| shipments.update         | yes   | no/limited        | assigned           | delegated         | no         | no        | no          |
| shipments.confirm        | yes   | no                | no                 | delegated         | yes        | no        | no          |
| documents.view           | yes   | own               | own                | delegated         | related    | yes       | view        |
| documents.upload         | yes   | own               | own                | delegated         | no         | no        | no          |
| documents.sign           | yes   | own               | own                | delegated         | no/limited | no        | no          |
| documents.approve        | yes   | no/limited        | no                 | delegated         | no         | yes       | no          |
| billing.view             | yes   | own               | own                | delegated         | no         | yes       | view        |
| billing.approve          | yes   | own/limited       | no                 | delegated         | no         | yes       | no          |
| billing.manage           | yes   | no                | no                 | delegated         | no         | yes       | no          |
| companies.view           | yes   | own               | own                | own               | own        | view      | view        |
| companies.manage         | yes   | own/limited       | own/limited        | own/limited       | no         | no        | no          |
| users.view               | yes   | own company       | own company        | own company       | limited    | own group | own group   |
| users.manage             | yes   | own company admin | own company admin  | own company admin | no         | limited   | limited     |
| low_code.view            | yes   | no                | no                 | no                | no         | limited   | limited     |
| low_code.manage          | yes   | no                | no                 | no                | no         | no        | no          |
| settings.view            | yes   | yes               | yes                | yes               | yes        | yes       | yes         |
| health.view              | yes   | no                | no                 | no                | no         | limited   | limited     |

## Identity Role Mapping Reference

| Product Role | Identity Codes (from pilot/low-code) |
| --- | --- |
| admin | PLATFORM_ADMIN |
| shipper | SHIPPER_ADMIN, SHIPPER_LOGIST |
| carrier | CARRIER_ADMIN, CARRIER_DISPATCHER |
| forwarder | FORWARDER_MANAGER |
| consignee | CONSIGNEE_OPERATOR, CONSIGNEE_VIEWER |
| finance | FINANCE_MANAGER |
| procurement | PROCUREMENT_MANAGER |

## Notes

```text
Values like own, assigned, delegated, related, limited require backend scoping rules.
Frontend must not treat navigation visibility as authorization.
Forwarder and procurement overlap in seed docs — split explicitly during implementation.
Multi-role users: resolve by union of permissions with admin override rules.
```

## Next

```text
RBAC_AND_ROLE_NAVIGATION_IMPLEMENTATION_PLAN_PACK v0.1
```
