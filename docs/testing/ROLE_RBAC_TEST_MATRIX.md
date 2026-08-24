# Role & RBAC Test Matrix — Freight Platform v1

Roles from migrations `000009`, `000013` and gateway RBAC modules.

## System Roles

| Role code | Scope | Persona |
|-----------|-------|---------|
| PLATFORM_ADMIN | GLOBAL | Platform admin |
| SHIPPER_ADMIN | TENANT | Shipper admin |
| SHIPPER_LOGIST | TENANT | Shipper logistician |
| CARRIER_ADMIN | TENANT | Carrier admin |
| CARRIER_DISPATCHER | TENANT | Carrier dispatcher |
| DRIVER | TENANT | Driver (driver-mobile) |
| CONSIGNEE_OPERATOR | TENANT | Consignee |
| PROCUREMENT_MANAGER | TENANT | Buyer/procurement |
| FINANCE_MANAGER | TENANT | Finance/accounting |
| FORWARDER_MANAGER | TENANT | Forwarder |
| GOV_INSPECTOR | GLOBAL | Inspector |

## Gateway RBAC Modules

| Module | Path | Roles |
|--------|------|-------|
| shipmentrbac | shipment mutations | create: SHIPPER_*, FORWARDER; accept/status: CARRIER_* |
| fleetrbac | drivers/vehicles | CARRIER_*, PLATFORM_ADMIN |
| rfxrbac | tender/bid | buyer vs carrier split |
| settlementrbac | `/freight-settlements` | buyer/carrier/finance |
| billingrbac | billing registers | FINANCE_*, buyer |
| freightcostrbac | `/freight-costs/*` | buyer analytics vs carrier read |
| paymentrbac | payments | FINANCE_* |
| controltower | CT routes | PLATFORM_ADMIN, CARRIER_DISPATCHER, SHIPPER_*, FORWARDER |

## Role × Domain × Action (summary)

| Action | SHIPPER_LOGIST | PROCUREMENT | CARRIER_DISP | DRIVER | FINANCE | FORWARDER |
|--------|----------------|-------------|--------------|--------|---------|-----------|
| rfx.publish | N | Y | N | N | N | N |
| rfx.bid | N | N | Y | N | N | N |
| rfx.award | N | Y | N | N | N | N |
| shipment.create | Y | N | N | N | N | Y |
| shipment.status | N | N | Y | Y* | N | N |
| settlement.approve | N | N | N | N | Y | N |
| billing.approve | N | N | N | N | Y | N |
| freight_cost.analytics | B | B | N | N | B | B |
| control_tower | N | N | Y | N | N | Y |

\* Driver: milestones only.

Each **N** cell → FP-SEC negative test. See [BUSINESS_E2E_CATALOG.md](./BUSINESS_E2E_CATALOG.md).
