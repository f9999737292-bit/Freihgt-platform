# Shipper shipment safe reads 0.1A

Shipment-service source contract only. This does not make the Shipper Portal customer-safe.

```text
SHIPPER_SHIPMENT_SOURCE_CONTRACT_READY=YES
SHIPPER_SHIPMENT_INBOX_SAFE=NO
SHIPPER_SHIPMENT_CUSTOMER_SAFE=NO_PENDING_GATEWAY
SHIPPER_COMPANY_SCOPE_REQUIRED=YES
CUSTOMER_PORTAL_MAY_USE_LEGACY_UNSCOPED_SHIPMENT_READ=NO
FORWARDER_AS_SHIPPER_ALIAS=NO
CONSIGNEE_PORTAL_IMPLEMENTATION_BLOCKED=YES
CUSTOMER_TRACKING_SAFE=NO
SHIPMENT_SERVICE_OWNS_PARTICIPANT_FACT=YES
SHIPMENT_SERVICE_OWNS_USER_MEMBERSHIP_AUTH=NO
INDEX_CHANGE_REQUIRED=NO
MIGRATION_CREATED=NO
```

## Unsafe baseline

`GET /v1/shipments` lists the verified tenant. `shipper_company_id` is an optional filter, so a missing value is tenant-wide. `GET /v1/shipments/{id}` checks tenant only. Those routes remain operator compatibility paths. The Shipper Portal must not use them.

## Safe source contract

```text
GET /v1/shipper/shipments?shipper_company_id=<uuid>
GET /v1/shipper/shipments/{id}?shipper_company_id=<uuid>
```

Tenant comes from trusted `X-Tenant-ID`. `shipper_company_id` is required. A missing or malformed value is HTTP 400. There is no tenant-wide default.

List and detail both require `tenant_id` and `shipper_company_id`. Detail also requires the shipment id. A shipment is returned only when both the tenant and the shipper company match. A same-tenant shipment owned by another shipper, a forwarder match, or a consignee match is HTTP 404 and the body does not include that shipment. Another tenant is the same 404.

The response is the existing shipment record: identity, parties, locations, cargo, mode, status, and planned or actual pickup and delivery. This stage does not add tracking, ETA, slots, finance, EDO, analytics, or Control Tower fields. It does not add shipper mutations.

`forwarder_company_id` is not a shipper alias. `consignee_company_id` does not open the Consignee Portal.

## Indexes

`transport.shipments` already has `idx_shipments_tenant_id` and `idx_shipments_shipper`. The shipper read is an equality on both columns plus `deleted_at IS NULL`. Those indexes can serve the predicate. No new index and no migration in this stage.

## Later owners

Agent A must bind the authenticated user to a selected company membership and to `SHIPPER_ADMIN` or `SHIPPER_LOGIST`, then pass that canonical company context. The gateway prefix table does not currently match `/api/v1/shipper`, so this source route is not a public portal API.

Agent F must call only this contract from the Shipper Portal. Tracking, documents, finance, and EDO stay blocked.

## Gateway result

The status block at the top of this file is the source-contract record and is unchanged. [SHIPPER_COMPANY_CONTEXT_GATEWAY_0_1.md](SHIPPER_COMPANY_CONTEXT_GATEWAY_0_1.md) records the later gateway stage: `SHIPPER_SHIPMENT_INBOX_SAFE` moves from `NO` to `YES`, and `SHIPPER_SHIPMENT_CUSTOMER_SAFE` moves from `NO_PENDING_GATEWAY` to `YES`, under `CP-SHIPPER-API-001`. Tracking and documents stay unsafe.
