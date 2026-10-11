# Shipper documents gateway 0.1

Public read-only shipper document and attachment routes. This stage does not add Shipper Portal screens, customer writes, or signing. It does not change document-service.

The source contract in [SHIPPER_DOCUMENTS_SOURCE_SAFETY_0_1A.md](SHIPPER_DOCUMENTS_SOURCE_SAFETY_0_1A.md) stays as written, including `SHIPPER_DOCUMENTS_CUSTOMER_SAFE=NO_PENDING_GATEWAY` and `CP_API_005_SHIPPER_PUBLIC_GATEWAY=OPEN`. This document is the gateway result.

```text
CP_API_005_TENANT_AUTHORITY=RESOLVED
CP_API_005_SHIPPER_SOURCE_READ=RESOLVED
CP_API_005_SHIPPER_PUBLIC_GATEWAY=RESOLVED
CP_API_005_SHIPPER_WRITES=OPEN
SHIPPER_DOCUMENTS_CUSTOMER_SAFE=YES_READ_ONLY
SHIPPER_ATTACHMENT_DOWNLOAD_CUSTOMER_SAFE=YES
CUSTOMER_DOCUMENTS_SAFE=NO_GLOBAL_SHIPPER_READ_ONLY
SHIPPER_DOCUMENT_WRITE_SAFE=NO
SHIPPER_EDO_SIGNING_SAFE=NO
SHIPPER_QES_SAFE=NO
SHIPPER_OPERATOR_EXCHANGE_SAFE=NO
LEGACY_DOCUMENT_BYPASS=CLOSED
LEGACY_DOCUMENT_OPERATOR_POLICY=TENANT_GLOBAL_PLATFORM_ADMIN
SHIPPER_DOCUMENT_PUBLIC_ROUTE_TARGET=document-service
SHIPPER_SHIPMENT_PUBLIC_ROUTE_TARGET=shipment-service
```

## Membership gate

The five public reads use the same selected-company rule as shipper shipment reads:

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

Explicit gateway routes call document-service. They are not served by the `/api/v1/shipper` shipment-service proxy. Shipment inbox and detail stay on shipment-service.

```text
GET /api/v1/shipper/documents?shipper_company_id=&shipment_id=
GET /api/v1/shipper/documents/{documentId}?shipper_company_id=
GET /api/v1/shipper/documents/{documentId}/attachments?shipper_company_id=
GET /api/v1/shipper/documents/{documentId}/attachments/{attachmentId}?shipper_company_id=
GET /api/v1/shipper/documents/{documentId}/attachments/{attachmentId}/content?shipper_company_id=
```

Downstream paths are `/v1/shipper/documents/...`. The list requires a UUID `shipment_id`. A browser `tenant_id` is rejected. `related_entity_type` stays server-owned `SHIPMENT` and is not forwarded as a customer override. Safe list filters that pass through are `document_type`, `document_status`, `limit`, and `offset`.

The gateway does not reimplement document type, status, shipment participation, finalized-attachment, or checksum rules. Attachment content is streamed with `Content-Type`, `Content-Disposition`, and `X-Content-SHA256`. Storage bucket, object key, and provider headers are not copied.

There is no public customer route for create, version, file, upload, finalize, signature, signing session, cancel, or archive.

## Legacy operator routes

`/api/v1/documents/**` and `/api/v1/signing-sessions/**` remain the operator compatibility paths. web-admin is that consumer, and no document role matrix exists beyond the operator surface already used for shipment and tracking reads. Those prefixes now allow only a tenant-global `PLATFORM_ADMIN`. `SHIPPER_ADMIN` and `SHIPPER_LOGIST` cannot use them.

Driver POD stays on `/api/v1/driver/me/.../pod/uploads` and document-service `/internal/v1/pod-uploads`. It is not a shipper membership route.

Carrier and consignee still have no customer-safe document contract. Customer writes and signing stay closed, so `CUSTOMER_DOCUMENTS_SAFE` is shipper read-only, not global.
