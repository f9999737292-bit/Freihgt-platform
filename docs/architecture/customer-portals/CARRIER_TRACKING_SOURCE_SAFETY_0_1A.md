# Carrier tracking source safety 0.1A

Stage: `CARRIER-TRACKING-SOURCE-SAFETY-0.1A`

This stage adds a participant-safe source contract for carrier tracking, ETA, and slots. It does not make the public API Gateway customer-safe and it does not change the Carrier Portal.

```text
CARRIER_PARTICIPATION_SOURCE=transport.shipments.carrier_company_id
CARRIER_TRACKING_SOURCE_CONTRACT_READY=YES
CARRIER_ETA_SOURCE_CONTRACT_READY=YES
CARRIER_SLOTS_SOURCE_CONTRACT_READY=YES
CARRIER_PARTICIPATION_PROOF_REQUIRED=YES
TRACKING_SERVICE_VERIFIES_CARRIER_PARTICIPATION=YES
TRACKING_DATA_READ_BEFORE_PARTICIPANT_PROOF=NO
PARTICIPANT_CHECK_FAILURE_FALLBACK_TO_TENANT_READ=NO
PARTICIPANT_LOOKUPS_PER_REQUEST_MAX=1
CROSS_CARRIER_TRACKING_READ=DENIED
CROSS_CARRIER_TRACKING_DISCLOSURE=NO
CROSS_TENANT_TRACKING_READ=DENIED
FORWARDER_AS_CARRIER_TRACKING_ALIAS=NO
SHIPPER_AS_CARRIER_TRACKING_ALIAS=NO
CONSIGNEE_AS_CARRIER_TRACKING_ALIAS=NO
ETA_CONTEXT_SERVER_OWNED=YES
CUSTOMER_AUTHORED_ETA_CONTEXT_ALLOWED=NO
SLOT_CONTEXT_SERVER_OWNED=YES
CUSTOMER_AUTHORED_SLOT_CONTEXT_ALLOWED=NO
CUSTOMER_LOCATION_PROVIDER_DEVICE_ID_EXPOSED=NO
CUSTOMER_ETA_PROVIDER_EVENT_ID_EXPOSED=NO
CUSTOMER_SLOT_PROVIDER_SLOT_ID_EXPOSED=NO
CUSTOMER_PORTAL_MAY_USE_LEGACY_TRACKING_ROUTE=NO
CP_API_006_CARRIER_SOURCE=RESOLVED
CP_API_006_CARRIER_PUBLIC_GATEWAY=OPEN
CARRIER_TRACKING_CUSTOMER_SAFE=NO_PENDING_GATEWAY
CUSTOMER_TRACKING_SAFE=NO_GLOBAL_SHIPPER_ONLY_CARRIER_SOURCE_PENDING_GATEWAY
SOURCE_SERVICE_OWNS_PARTICIPANT_FACT=YES
SOURCE_SERVICE_OWNS_USER_MEMBERSHIP_AUTH=NO
MIGRATION_CREATED=NO
```

`driver_id`, `vehicle_id`, `forwarder_company_id`, and `shipper_company_id` are not carrier participation.

## Participant context

```text
GET /internal/v1/shipments/{shipmentId}/carrier-tracking-context?carrier_company_id=<uuid>
```

Auth is the existing `X-Internal-Service-Token` plus trusted `X-Tenant-ID`. The lookup is `GetByIDAndCarrier`:

```text
WHERE id = $1 AND tenant_id = $2 AND carrier_company_id = $3 AND deleted_at IS NULL
```

The body returns only `shipmentId`, `tenantId`, `carrierCompanyId`, `status`, `plannedPickupAt`, `plannedDeliveryAt`, `actualPickupAt`, and `actualDeliveryAt`. A matching shipper, forwarder, or consignee is not a carrier. Wrong carrier, wrong tenant, and a deleted shipment return 404 without saying the shipment belongs to another carrier. A missing or malformed company id returns 400. A missing or wrong internal token returns 401.

Shipment-service does not decide `CARRIER_ADMIN` or `CARRIER_DISPATCHER`.

## Source routes

These tracking-service routes are not public portal routes. This stage does not add `/api/v1/carrier/shipments/...` to the API Gateway.

```text
GET /v1/carrier/shipments/{shipmentId}/tracking?carrier_company_id=<uuid>
GET /v1/carrier/shipments/{shipmentId}/tracking/locations?carrier_company_id=<uuid>
GET /v1/carrier/shipments/{shipmentId}/eta?carrier_company_id=<uuid>
GET /v1/carrier/shipments/{shipmentId}/eta/history?carrier_company_id=<uuid>
GET /v1/carrier/shipments/{shipmentId}/slots?carrier_company_id=<uuid>
GET /v1/carrier/shipments/{shipmentId}/slots/history?carrier_company_id=<uuid>
```

Each request proves participation once, then reads tracking, location, ETA, or slot data. Slot current reuses that same shipment context and tracking-service's own ETA query. A failed participant check does not fall back to a tenant-only read. A dependency failure is unavailable and does not return tracking data.

ETA and slot timing facts come from the shipment context. Caller query keys for planned or actual times, shipment status, and ETA milestone fields are rejected with 400. History filters stay `targetType` or `slotType` of `pickup` or `delivery`, plus `from`, `to`, `limit`, and `offset`. `execution_stop` is not a customer ETA target.

Customer responses omit `providerDeviceId`, `providerEventId`, `providerCode`, `providerConfidence`, and `providerSlotId`. Operator DTOs and the generic `/v1/shipments/{shipmentId}/...` routes are unchanged. The shipper source routes stay in place.

`CUSTOMER_PORTAL_MAY_USE_LEGACY_TRACKING_ROUTE=NO`

## Still open

Agent A must gate a public carrier tracking route after this source contract. Until that lands, carrier tracking is not customer-safe. The Carrier Portal is unchanged in this stage.
