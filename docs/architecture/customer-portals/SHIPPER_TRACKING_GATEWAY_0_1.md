# Shipper tracking gateway 0.1

Public shipper tracking, ETA, and slot reads. This stage does not add Shipper Portal screens and does not change tracking-service or shipment-service.

The source contract in [SHIPPER_TRACKING_SOURCE_SAFETY_0_1A.md](SHIPPER_TRACKING_SOURCE_SAFETY_0_1A.md) stays as written, including `SHIPPER_TRACKING_CUSTOMER_SAFE=NO_PENDING_GATEWAY` and `CP_API_006_PUBLIC_GATEWAY=OPEN`. This document is the gateway result.

```text
CP_API_006_SHIPPER_SOURCE=RESOLVED
CP_API_006_SHIPPER_PUBLIC_GATEWAY=RESOLVED
CP_API_006_CARRIER=OPEN
CP_API_006_CONSIGNEE=OPEN
SHIPPER_TRACKING_CUSTOMER_SAFE=YES
SHIPPER_ETA_CUSTOMER_SAFE=YES
SHIPPER_SLOTS_CUSTOMER_SAFE=YES
CUSTOMER_TRACKING_SAFE=NO_GLOBAL_SHIPPER_ONLY
CUSTOMER_DOCUMENTS_SAFE=NO
CONSIGNEE_PORTAL_IMPLEMENTATION_BLOCKED=YES
LEGACY_TRACKING_BYPASS=CLOSED
LEGACY_TRACKING_OPERATOR_POLICY=TENANT_GLOBAL_PLATFORM_ADMIN
SHIPPER_TRACKING_PUBLIC_ROUTE_TARGET=tracking-service
SHIPPER_SHIPMENT_PUBLIC_ROUTE_TARGET=shipment-service
```

## Membership gate

The six public routes use the same selected-company rule as shipper shipment reads:

```text
JWT
→ verified tenant and user
→ ACTIVE membership for the selected company
→ company_type=SHIPPER
→ SHIPPER_ADMIN or SHIPPER_LOGIST on that membership
→ canonical shipper_company_id
```

`FORWARDER`, `LSP`, and `CARRIER` are not shipper aliases. A role on another company does not authorize the selected company. Tenant-global `PLATFORM_ADMIN` does not authorize these customer routes.

## Public routes

Explicit gateway routes call tracking-service. They are not served by the `/api/v1/shipper` shipment-service proxy. Shipment inbox and detail stay on shipment-service.

```text
GET /api/v1/shipper/shipments/{shipmentId}/tracking
GET /api/v1/shipper/shipments/{shipmentId}/tracking/locations
GET /api/v1/shipper/shipments/{shipmentId}/eta
GET /api/v1/shipper/shipments/{shipmentId}/eta/history
GET /api/v1/shipper/shipments/{shipmentId}/slots
GET /api/v1/shipper/shipments/{shipmentId}/slots/history
```

Downstream paths are `/v1/shipper/shipments/{shipmentId}/...` with the canonical `shipper_company_id` and the JWT tenant. The gateway passes the source response body through. It does not add tracking, ETA, or slot facts.

Caller ETA keys `plannedPickupAt`, `plannedDeliveryAt`, `actualPickupAt`, `actualDeliveryAt`, and `shipmentStatus` fail closed with 400. Caller slot milestone and ETA keys fail closed with 400. History filters that remain are `targetType`, `slotType`, `from`, `to`, `limit`, and `offset`.

## Legacy operator routes

`GET /api/v1/shipments/{shipmentId}/tracking`, locations, ETA, ETA history, slots, and slot history stay the operator compatibility paths. They now allow only a tenant-global `PLATFORM_ADMIN`, the same operator rule as generic shipment list and detail. `SHIPPER_ADMIN` and `SHIPPER_LOGIST` cannot use them. web-admin is that operator surface. Driver ingestion, provider ingestion, and tracking internal lookups are not these public GET routes and are unchanged.

Carrier and consignee still have no customer-safe tracking contract. `CUSTOMER_TRACKING_SAFE` stays shipper-only, not global.
