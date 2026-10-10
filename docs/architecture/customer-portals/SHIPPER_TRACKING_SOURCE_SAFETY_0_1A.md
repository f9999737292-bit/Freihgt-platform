# Shipper tracking source safety 0.1A

Stage: `SHIPPER-TRACKING-SOURCE-SAFETY-0.1A`

This stage adds a participant-safe source contract for shipper tracking, ETA, and slots. It does not make the public API Gateway customer-safe.

```text
SHIPPER_TRACKING_SOURCE_CONTRACT_READY=YES
SHIPPER_ETA_SOURCE_CONTRACT_READY=YES
SHIPPER_SLOTS_SOURCE_CONTRACT_READY=YES
SHIPPER_TRACKING_CONTEXT_INTERNAL_AUTH=YES
SHIPPER_PARTICIPATION_PROOF_REQUIRED=YES
TRACKING_SERVICE_VERIFIES_SHIPPER_PARTICIPATION=YES
TRACKING_DATA_READ_BEFORE_PARTICIPANT_PROOF=NO
PARTICIPANT_CHECK_FAILURE_FALLBACK_TO_TENANT_READ=NO
PARTICIPANT_LOOKUPS_PER_REQUEST_MAX=1
CROSS_SHIPPER_TRACKING_READ=DENIED
CROSS_SHIPPER_TRACKING_DISCLOSURE=NO
CROSS_TENANT_TRACKING_READ=DENIED
FORWARDER_AS_SHIPPER_TRACKING_ALIAS=NO
CONSIGNEE_AS_SHIPPER_TRACKING_ALIAS=NO
CARRIER_AS_SHIPPER_TRACKING_ALIAS=NO
ETA_CONTEXT_SERVER_OWNED=YES
CUSTOMER_AUTHORED_PLANNED_TIMES_ALLOWED=NO
CUSTOMER_AUTHORED_ACTUAL_TIMES_ALLOWED=NO
CUSTOMER_AUTHORED_SHIPMENT_STATUS_ALLOWED=NO
ETA_CONTEXT_TAMPER=DENIED
SLOT_CONTEXT_SERVER_OWNED=YES
CUSTOMER_AUTHORED_SLOT_MILESTONE_CONTEXT_ALLOWED=NO
CUSTOMER_AUTHORED_ETA_FOR_SLOT_ALLOWED=NO
SLOT_CONTEXT_TAMPER=DENIED
CUSTOMER_LOCATION_PROVIDER_DEVICE_ID_EXPOSED=NO
CUSTOMER_ETA_PROVIDER_EVENT_ID_EXPOSED=NO
CUSTOMER_SLOT_PROVIDER_SLOT_ID_EXPOSED=NO
CUSTOMER_PORTAL_MAY_USE_LEGACY_TRACKING_ROUTE=NO
CP_API_006_SOURCE_LAYER=RESOLVED
CP_API_006_PUBLIC_GATEWAY=OPEN
CUSTOMER_TRACKING_SAFE=NO_PENDING_GATEWAY
SHIPPER_TRACKING_CUSTOMER_SAFE=NO_PENDING_GATEWAY
SOURCE_SERVICE_OWNS_PARTICIPANT_FACT=YES
SOURCE_SERVICE_OWNS_USER_MEMBERSHIP_AUTH=NO
MIGRATION_CREATED=NO
```

## Unsafe generic routes

These tracking-service routes remain for operator and backward-compatible callers. They scope by tenant and shipment id. They do not prove that a company is the shipment shipper.

```text
GET /v1/shipments/{shipmentId}/tracking
GET /v1/shipments/{shipmentId}/tracking/locations
GET /v1/shipments/{shipmentId}/eta
GET /v1/shipments/{shipmentId}/eta/history
GET /v1/shipments/{shipmentId}/slots
GET /v1/shipments/{shipmentId}/slots/history
```

The public gateway equivalents under `/api/v1/shipments/{shipmentId}/...` still prove tenant only. A Shipper Portal user must not use them.

`CUSTOMER_PORTAL_MAY_USE_LEGACY_TRACKING_ROUTE=NO`

## Participant-safe source routes

tracking-service now serves source routes. They are not public portal routes. The gateway does not match `/api/v1/shipper` with the existing `/api/v1/shipments` prefix.

```text
GET /v1/shipper/shipments/{shipmentId}/tracking?shipper_company_id=<uuid>
GET /v1/shipper/shipments/{shipmentId}/tracking/locations?shipper_company_id=<uuid>
GET /v1/shipper/shipments/{shipmentId}/eta?shipper_company_id=<uuid>
GET /v1/shipper/shipments/{shipmentId}/eta/history?shipper_company_id=<uuid>
GET /v1/shipper/shipments/{shipmentId}/slots?shipper_company_id=<uuid>
GET /v1/shipper/shipments/{shipmentId}/slots/history?shipper_company_id=<uuid>
```

Each request parses the trusted `X-Tenant-ID`, the shipment id, and the required `shipper_company_id`. It then asks shipment-service to prove participation. Tracking, location, ETA, and slot reads run only after that proof. A 404 from shipment-service stays 404. A dependency or authorization failure is unavailable (`503`). There is no tenant-only fallback.

One shipment-service lookup is the maximum per customer request. Slot current uses that same context plus tracking-service's own ETA query. It does not call shipment-service again per row. There is no cross-company cache.

## Shipment participant context

```text
GET /internal/v1/shipments/{shipmentId}/shipper-tracking-context?shipper_company_id=<uuid>
```

Auth is `X-Internal-Service-Token` plus trusted `X-Tenant-ID`. The lookup is `GetForShipper`, which uses the existing exact query on shipment id, tenant id, shipper company id, and `deleted_at IS NULL`. A matching forwarder, consignee, or carrier company is not a shipper.

The body returns only `shipmentId`, `tenantId`, `shipperCompanyId`, `status`, `plannedPickupAt`, `plannedDeliveryAt`, `actualPickupAt`, and `actualDeliveryAt`.

Correct tenant, shipper, and shipment return 200. Wrong shipper, wrong tenant, forwarder-only, consignee-only, and carrier-only return 404 without saying that the shipment belongs to another company. A missing or malformed company id returns 400. A missing or wrong internal token returns 401.

Shipment-service and tracking-service do not decide `SHIPPER_ADMIN` or `SHIPPER_LOGIST`.

## ETA and slot facts

The safe ETA route derives status and planned/actual pickup and delivery from the shipment context. Caller query keys `plannedPickupAt`, `plannedDeliveryAt`, `actualPickupAt`, `actualDeliveryAt`, and `shipmentStatus` are rejected with 400.

The safe slot route builds milestone context on the server from that shipment context and from tracking-service ETA. Caller milestone and ETA query keys are rejected with 400. Those keys include `shipmentStatus`, `actualPickupAt`, `actualDeliveryAt`, and pickup/delivery ETA status, freshness, quality, and estimated arrival.

History filters that do not create business truth remain: ETA `targetType` (`pickup` or `delivery`), slot `slotType` (`pickup` or `delivery`), plus `from`, `to`, `limit`, and `offset`. `execution_stop` is not a customer ETA target.

## Data minimization

Operator DTOs are unchanged and may still include provider correlation ids.

Customer location history omits `providerDeviceId`, provider event ids, and provider code. Provider code is a technical correlation value, not a customer display fact. Safe fields are coordinates, recorded and received times, source type, speed, heading, accuracy, and quality.

Customer ETA current and history omit provider event ids, provider code, and provider confidence. Customer slot current and history omit `providerSlotId` and provider code. Responses do not include the internal service token.

## Configuration

tracking-service uses the existing `TRACKING_SHIPMENT_INTERNAL_URL` (`ShipmentInternalURL`) and `TRACKING_INTERNAL_SERVICE_TOKEN` or `INTERNAL_SERVICE_TOKEN` (`InternalServiceToken`). No new secret was added. An empty URL or token fails closed. Staging configuration was not changed.

The existing execution-stop `ShipmentContextClient` is unchanged.

## Dependencies still open

Agent A must gate the public gateway so Shipper Portal users cannot call the legacy tenant-only tracking, ETA, and slot routes, and must authorize the new source routes with shipper membership. Until that lands:

```text
CP_API_006_PUBLIC_GATEWAY=OPEN
CUSTOMER_TRACKING_SAFE=NO_PENDING_GATEWAY
SHIPPER_TRACKING_CUSTOMER_SAFE=NO_PENDING_GATEWAY
```

Agent F must not render tracking from the legacy public routes.

## Gateway result

The status block at the top of this file is the source-contract record and is unchanged. [SHIPPER_TRACKING_GATEWAY_0_1.md](SHIPPER_TRACKING_GATEWAY_0_1.md) records the later gateway stage. For shipper only, `SHIPPER_TRACKING_CUSTOMER_SAFE`, `SHIPPER_ETA_CUSTOMER_SAFE`, and `SHIPPER_SLOTS_CUSTOMER_SAFE` move from `NO_PENDING_GATEWAY` to `YES`. `CP_API_006_SHIPPER_PUBLIC_GATEWAY=RESOLVED`. Carrier and consignee stay open. Global customer tracking is `NO_GLOBAL_SHIPPER_ONLY`.
