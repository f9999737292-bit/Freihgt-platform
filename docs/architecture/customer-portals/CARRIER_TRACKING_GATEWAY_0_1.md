# Carrier tracking gateway 0.1

Public carrier tracking, ETA, and slot reads. This stage does not add Carrier Portal screens and does not change tracking-service or shipment-service.

The source contract in [CARRIER_TRACKING_SOURCE_SAFETY_0_1A.md](CARRIER_TRACKING_SOURCE_SAFETY_0_1A.md) stays as written, including `CARRIER_TRACKING_CUSTOMER_SAFE=NO_PENDING_GATEWAY` and `CP_API_006_CARRIER_PUBLIC_GATEWAY=OPEN`. This document is the gateway result.

```text
CP_API_006_CARRIER_SOURCE=RESOLVED
CP_API_006_CARRIER_PUBLIC_GATEWAY=RESOLVED
CARRIER_TRACKING_CUSTOMER_SAFE=YES
CARRIER_ETA_CUSTOMER_SAFE=YES
CARRIER_SLOTS_CUSTOMER_SAFE=YES
CP_API_006_CONSIGNEE=OPEN
CUSTOMER_TRACKING_SAFE=NO_GLOBAL_SHIPPER_AND_CARRIER_ONLY
LEGACY_CARRIER_TRACKING_BYPASS=CLOSED
CARRIER_TRACKING_PUBLIC_ROUTE_TARGET=tracking-service
CARRIER_MEMBERSHIP_AUTHORITY=GATEWAY_SERVER_VERIFIED
CARRIER_PARTICIPATION_AUTHORITY=shipment-service
```

Membership authorization and shipment participation are different checks. The gateway proves the caller may act for the selected carrier company. tracking-service proves that company participates in the shipment.

## Membership gate

The six public reads use the existing selected-company carrier rule:

```text
JWT
→ verified tenant and user
→ ACTIVE membership for the selected company
→ company_type=CARRIER
→ CARRIER_ADMIN or CARRIER_DISPATCHER on that membership
→ canonical carrier_company_id
→ X-Company-ID and X-Actor-Kind=CARRIER
```

`CARRIER_ACCOUNTANT`, `DRIVER`, shipper, forwarder, and consignee roles do not pass. A role on another company does not authorize the selected company. Tenant-global `PLATFORM_ADMIN` does not authorize these customer routes.

## Public routes

Explicit gateway routes call tracking-service. They are not served by the shipment proxy.

```text
GET /api/v1/carrier/shipments/{shipmentId}/tracking
GET /api/v1/carrier/shipments/{shipmentId}/tracking/locations
GET /api/v1/carrier/shipments/{shipmentId}/eta
GET /api/v1/carrier/shipments/{shipmentId}/eta/history
GET /api/v1/carrier/shipments/{shipmentId}/slots
GET /api/v1/carrier/shipments/{shipmentId}/slots/history
```

Downstream paths are `/v1/carrier/shipments/{shipmentId}/...`. The downstream `carrier_company_id` is the verified company, not the raw browser value. Safe filters are `from`, `to`, `limit`, `offset`, plus `targetType` on ETA history and `slotType` on slot history. Caller ETA and slot facts are rejected before the tracking call. A source 404 stays 404.

Legacy `/api/v1/shipments/{shipmentId}/tracking|eta|slots` routes stay tenant-global `PLATFORM_ADMIN` only.
