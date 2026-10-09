# Shipper company-context gateway 0.1

Public shipper shipment reads. This stage does not add Shipper Portal screens.

The source contract recorded in [SHIPPER_SHIPMENT_SAFE_READS_0_1A.md](SHIPPER_SHIPMENT_SAFE_READS_0_1A.md) stays as written, including `SHIPPER_SHIPMENT_CUSTOMER_SAFE=NO_PENDING_GATEWAY`. This document is the gateway result after that source contract.

```text
CP-SHIPPER-API-001=CLOSED
SHIPPER_SHIPMENT_SOURCE_CONTRACT_READY=YES
SHIPPER_SHIPMENT_CUSTOMER_SAFE=YES
SHIPPER_SHIPMENT_INBOX_SAFE=YES
SHIPPER_PROXY_ROUTE=shipment-service
SELECTED_COMPANY_MEMBERSHIP_IS_AUTHORIZATION_SCOPE=YES
SHIPPER_CUSTOMER_ROUTE_REQUIRES_COMPANY_TYPE_SHIPPER=YES
FORWARDER_AS_SHIPPER_ALIAS=NO
LSP_AS_SHIPPER_ALIAS=NO
TENANT_GLOBAL_PLATFORM_ADMIN_BYPASS=NO
LEGACY_SHIPMENT_READ_BYPASS=CLOSED
CUSTOMER_TRACKING_SAFE=NO
CUSTOMER_DOCUMENTS_SAFE=NO
CONSIGNEE_PORTAL_IMPLEMENTATION_BLOCKED=YES
FORWARDER_DUAL_SIDE_BACKEND_READY=NO
```

## CP-SHIPPER-API-001

The public customer routes are:

```text
GET /api/v1/shipper/shipments?shipper_company_id=<uuid>
GET /api/v1/shipper/shipments/{id}?shipper_company_id=<uuid>
```

Both proxy to shipment-service `/v1/shipper/shipments` and `/v1/shipper/shipments/{id}`. `/api/v1/shipper` is not an RFX prefix. `/api/v1/carrier` remains RFX.

`shipper_company_id` selects the company. It is not authority. The gateway uses the verified JWT tenant and user, loads ACTIVE memberships from identity-service, and allows the call only when that same membership has `company_type=SHIPPER` and `SHIPPER_ADMIN` or `SHIPPER_LOGIST`. The gateway then forwards the canonical company id as `shipper_company_id` and `X-Company-ID`, and sets `X-Actor-Kind=BUYER`. Client `X-Company-ID`, `X-Actor-Kind`, `X-Tenant-ID`, and `X-User-ID` are not authority.

A role on another company does not authorize the selected company. `FORWARDER` and `LSP` are not shipper aliases. Tenant-global `PLATFORM_ADMIN` does not authorize this customer route.

Missing or malformed `shipper_company_id` is HTTP 400. No matching membership, the wrong company type, the wrong role, or conflicting repeated company ids fail closed and do not reach shipment-service.

## Legacy tenant-wide reads

`GET /api/v1/shipments` and `GET /api/v1/shipments/{id}` remain the operator compatibility paths. They are no longer an open proxy for any authenticated caller. The gateway allows them only for a tenant-global `PLATFORM_ADMIN` role, the existing operator role. `SHIPPER_ADMIN` and `SHIPPER_LOGIST` cannot use them. Shipment mutations, driver APIs, tracking, ETA, and slots are unchanged.

`CP-API-001` remains the historical record of the unscoped list. This id is the shipper customer closure. Consignee inbound reads are still not a customer contract.
