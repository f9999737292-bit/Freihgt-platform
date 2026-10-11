# Carrier documents gateway 0.1

Public read-only carrier document and attachment routes. This stage does not add Carrier Portal screens, customer writes, or signing. It does not change document-service.

The source contract in [CARRIER_DOCUMENTS_SOURCE_SAFETY_0_1A.md](CARRIER_DOCUMENTS_SOURCE_SAFETY_0_1A.md) stays as written, including `CARRIER_DOCUMENTS_CUSTOMER_SAFE=NO_PENDING_GATEWAY` and `CP_API_005_CARRIER_PUBLIC_GATEWAY=OPEN`. This document is the gateway result.

```text
CP_API_005_CARRIER_SOURCE_READ=RESOLVED
CP_API_005_CARRIER_PUBLIC_GATEWAY=RESOLVED
CARRIER_DOCUMENTS_CUSTOMER_SAFE=YES_READ_ONLY
CARRIER_ATTACHMENT_DOWNLOAD_CUSTOMER_SAFE=YES
CUSTOMER_DOCUMENTS_SAFE=NO_GLOBAL_SHIPPER_AND_CARRIER_READ_ONLY
CARRIER_DOCUMENT_WRITE_SAFE=NO
CARRIER_EDO_SIGNING_SAFE=NO
CARRIER_QES_SAFE=NO
CARRIER_OPERATOR_EXCHANGE_SAFE=NO
LEGACY_CARRIER_DOCUMENT_BYPASS=CLOSED
LEGACY_DOCUMENT_OPERATOR_POLICY=TENANT_GLOBAL_PLATFORM_ADMIN
CARRIER_DOCUMENT_PUBLIC_ROUTE_TARGET=document-service
```

Membership authorization and shipment participation are different checks. The gateway proves the caller may act for the selected carrier company. document-service proves that company participates in the shipment and document.

## Membership gate

The five public reads use the existing selected-company carrier rule:

```text
JWT
→ verified tenant and user
→ ACTIVE membership for the selected company
→ company_type=CARRIER
→ CARRIER_ADMIN or CARRIER_DISPATCHER on that membership
→ canonical carrier_company_id
→ X-Company-ID and X-Actor-Kind=CARRIER
```

A role on another company does not authorize the selected company. Tenant-global `PLATFORM_ADMIN` does not authorize these customer routes.

## Public routes

Explicit gateway routes call document-service. They are not served by the `/api/v1/carrier` RFX proxy.

```text
GET /api/v1/carrier/documents?carrier_company_id=&shipment_id=
GET /api/v1/carrier/documents/{documentId}?carrier_company_id=
GET /api/v1/carrier/documents/{documentId}/attachments?carrier_company_id=
GET /api/v1/carrier/documents/{documentId}/attachments/{attachmentId}?carrier_company_id=
GET /api/v1/carrier/documents/{documentId}/attachments/{attachmentId}/content?carrier_company_id=
```

Downstream paths are `/v1/carrier/documents/...`. The list requires a UUID `shipment_id`. A browser `tenant_id` is rejected. `related_entity_type` stays server-owned `SHIPMENT` and is not forwarded. Safe list filters are `document_type`, `document_status`, `limit`, and `offset`. Detail and attachment routes forward only the canonical `carrier_company_id`.

The gateway does not reimplement document type, status, shipment participation, finalized-attachment, or checksum rules. Attachment content is streamed with `Content-Type`, `Content-Disposition`, `X-Content-SHA256`, and `Content-Length`. Storage headers are not copied. A source 404 stays 404.

There is no public customer route for create, version, file, upload, finalize, signature, signing session, cancel, or archive. Legacy `/api/v1/documents/**` and `/api/v1/signing-sessions/**` stay tenant-global `PLATFORM_ADMIN` only.
