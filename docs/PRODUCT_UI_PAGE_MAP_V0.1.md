# Product UI Page Map v0.1

## Summary

Current web-admin page map for product planning.

## Page Map

| Route | Product Area | User Purpose | Priority | Detail Route |
| --- | --- | --- | --- | --- |
| /login | Access | User login | P0 | — |
| / | Access | Auth redirect hub | P0 | — |
| /dashboard | Overview | Module counts and quick links | P0 | — |
| /control-tower | Operations | Operational KPIs, funnels, alerts | P0/P1 | — |
| /transport-orders | TMS | Manage transport orders | P1 | /transport-orders/[id] |
| /freight-requests | Freight procurement | Manage freight requests and bids | P1 | /freight-requests/[id] |
| /rfx | RFx / tenders | Manage tender process | P1 | /rfx/[id] |
| /shipments | Shipments | Track shipment lifecycle | P1 | /shipments/[id] |
| /documents | Documents | Manage operational documents | P1 | /documents/[id] |
| /billing-registers | Billing | Manage billing registers | P1 | /billing-registers/[id] |
| /companies | Admin | Manage companies and members | P1 | /companies/[id] |
| /users | Admin | Manage users | P1 | /users/[id] |
| /low-code | Admin/low-code | Configure templates and fields | P1 | sub-routes below |
| /low-code/form-templates | Low-code | View form templates | P1 | /low-code/form-templates/[id] |
| /low-code/custom-field-values | Low-code | Manage custom field values | P1 | — |
| /low-code/audit | Low-code | Configuration audit log | P1 | — |
| /low-code/admin/form-templates | Low-code admin | Admin template editor | P1 | /low-code/admin/form-templates/[id], /new |
| /settings | Settings | Session, tenant, environment info | P2 | — |
| /health | Reliability | Gateway and service health | P0/P1 | — |

## Navigation Source

```text
Primary nav: AppSidebar.vue — flat 13-item sidebar
Auth: middleware/auth.ts — redirect to /login if unauthenticated
Guest: middleware/guest.ts — login page
Low-code admin: middleware/low-code-admin.ts
```

## Composables Supporting Navigation/API

```text
useAuth, usePermissions, useBackendStatus, useTenant
useControlTower, useLowCodeApi, useLowCodePermissions
Module APIs: useFreightRequestsApi, useRfxApi, useShipmentsApi,
  useDocumentsApi, useCompanies, useUsersApi, ...
```

## Product Interpretation

```text
web-admin currently acts as the main production UI.
It contains a broad admin+operations surface in one app.
Separate role apps exist in the repository (web-shipper, web-carrier,
web-consignee, web-finance, web-procurement) and require a separate
role-based cabinets gap analysis.
Production deploy currently serves web-admin only.
```

## Next

```text
ROLE_BASED_CABINETS_GAP_ANALYSIS_PACK v0.1
```
