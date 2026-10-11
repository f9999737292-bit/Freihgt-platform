# Carrier documents source safety 0.1A

## Status

```text
STAGE=CARRIER-DOCUMENTS-SOURCE-SAFETY-0.1A
CARRIER_DOCUMENT_READ_SOURCE_SAFE=YES
CARRIER_DOCUMENTS_CUSTOMER_SAFE=NO_PENDING_GATEWAY
CARRIER_DOCUMENT_SOURCE_CONTRACT_READY=YES
CARRIER_ATTACHMENT_SOURCE_CONTRACT_READY=YES
CUSTOMER_DOCUMENTS_SAFE=NO_GLOBAL_SHIPPER_READ_ONLY_CARRIER_SOURCE_PENDING_GATEWAY
CARRIER_DOCUMENT_WRITE_SAFE=NO
CARRIER_EDO_SIGNING_SAFE=NO
CARRIER_QES_SAFE=NO
CARRIER_OPERATOR_EXCHANGE_SAFE=NO
CARRIER_EDO_LEGAL_ACTIONS_SAFE=NO
```

This stage makes document-service prove that a requested carrier company is the canonical carrier on a shipment before it returns operational documents. It does not make the public API Gateway customer-safe. Agent A still has to authorize carrier membership and close legacy document routes for carrier users. Carrier Portal is unchanged.

Reading a document is not a legal signature, a qualified electronic signature, or an operator exchange.

## Participant fact

A carrier read is allowed only when one query proves all of the following:

- the trusted tenant matches `documents.documents.tenant_id` and `transport.shipments.tenant_id`
- `related_entity_type` is `SHIPMENT`
- `related_entity_id` is `transport.shipments.id`
- `transport.shipments.carrier_company_id` is the requested carrier company
- the shipment and the document are not deleted
- the document type and status are on the allowlists below

`X-Tenant-ID` is the tenant authority. A query `tenant_id` may be omitted or may repeat the trusted tenant. A conflict is rejected before a repository read.

`owner_company_id`, `shipper_company_id`, `consignee_company_id`, `forwarder_company_id`, `driver_id`, and `vehicle_id` are not carrier authority. A document owned by the requested carrier but linked to another carrier's shipment is denied. An operational document owned by the shipper and linked to the requested carrier's shipment may be read.

document-service does not decide `CARRIER_ADMIN` or `CARRIER_DISPATCHER`.

```text
CARRIER_DOCUMENT_TENANT_SERVER_OWNED=YES
CALLER_TENANT_AUTHORITY=NO
CARRIER_DOCUMENT_PARTICIPATION_PROOF_REQUIRED=YES
OWNER_COMPANY_ID_ALONE_IS_AUTHORITY=NO
CUSTOMER_DOCUMENT_READ_DB_SCOPE_EXACT=YES
SOURCE_SERVICE_OWNS_DOCUMENT_PARTICIPANT_FACT=YES
SOURCE_SERVICE_OWNS_USER_MEMBERSHIP_AUTH=NO
CARRIER_DOCUMENT_LIST_REQUIRES_SHIPMENT_ID=YES
CARRIER_DOCUMENT_LIST_N_PLUS_ONE=NO
CARRIER_RELATED_ENTITY_TYPE_SERVER_OWNED=YES
CROSS_CARRIER_DOCUMENT_READ=DENIED
CROSS_CARRIER_DOCUMENT_DISCLOSURE=NO
CROSS_TENANT_DOCUMENT_READ=DENIED
SHIPPER_AS_CARRIER_DOCUMENT_ALIAS=NO
CONSIGNEE_AS_CARRIER_DOCUMENT_ALIAS=NO
FORWARDER_AS_CARRIER_DOCUMENT_ALIAS=NO
```

## Safe class

Operational types: `ETRN`, `EPD`, `WAYBILL`, `POD`, `DISCREPANCY_ACT`, `ECMR`.

Not exposed: `INVOICE`, `VAT_INVOICE`, `ACT`, `UPD`, `CLAIM`.

Visible statuses: `READY_FOR_SIGNING`, `SIGNING_IN_PROGRESS`, `SIGNED`, `SENT_TO_OPERATOR`, `ACCEPTED`, `REJECTED`, `ARCHIVED`.

Not visible: `DRAFT`, `CANCELLED`.

```text
CARRIER_OPERATIONAL_DOCUMENT_TYPES_ONLY=YES
FINANCIAL_DOCUMENTS_EXPOSED_TO_CARRIER_SOURCE=NO
CLAIM_DOCUMENTS_EXPOSED_TO_CARRIER_SOURCE=NO
CUSTOMER_SETTLEMENT_DOCUMENTS_EXPOSED=NO
SUPPLY_SETTLEMENT_DOCUMENTS_EXPOSED=NO
FORWARDER_DUAL_CHAIN_COLLAPSED=NO
EDO_PARTY_CONTEXT_BYPASSED=NO
CUSTOMER_DRAFT_DOCUMENT_VISIBLE=NO
CUSTOMER_CANCELLED_DOCUMENT_VISIBLE=NO
```

## Source routes

```text
GET /v1/carrier/documents?carrier_company_id=&shipment_id=
GET /v1/carrier/documents/{documentId}?carrier_company_id=
GET /v1/carrier/documents/{documentId}/attachments?carrier_company_id=
GET /v1/carrier/documents/{documentId}/attachments/{attachmentId}?carrier_company_id=
GET /v1/carrier/documents/{documentId}/attachments/{attachmentId}/content?carrier_company_id=
```

There is no write method on `/v1/carrier/**`. The list requires `shipment_id`. A missing or malformed shipment id is 400. A shipment that is not this carrier's shipment in this tenant is 404. The response does not say which company owns it.

The customer document fields are `id`, `document_number`, `document_type`, `document_status`, `owner_company_id`, `related_entity_type`, `related_entity_id`, `legal_language`, `created_at`, `updated_at`, and `version`. Attachment metadata uses finalized `documents.document_attachments` rows and omits storage location. Content is opened only after participant proof, and only for a finalized attachment, with the existing size and SHA-256 check.

```text
CARRIER_DOCUMENT_LIST_SOURCE_ROUTE=YES
CARRIER_DOCUMENT_DETAIL_SOURCE_ROUTE=YES
CARRIER_ATTACHMENT_LIST_SOURCE_ROUTE=YES
CARRIER_ATTACHMENT_DETAIL_SOURCE_ROUTE=YES
CARRIER_ATTACHMENT_CONTENT_SOURCE_ROUTE=YES
CUSTOMER_DOCUMENT_PAYLOAD_JSON_EXPOSED=NO
CUSTOMER_DOCUMENT_STORAGE_PROVIDER_EXPOSED=NO
CUSTOMER_DOCUMENT_BUCKET_EXPOSED=NO
CUSTOMER_DOCUMENT_OBJECT_KEY_EXPOSED=NO
CUSTOMER_DOCUMENT_INTERNAL_PATH_EXPOSED=NO
CUSTOMER_USES_LEGACY_DOCUMENT_FILES=NO
CUSTOMER_ATTACHMENT_DRAFT_VISIBLE=NO
CUSTOMER_ATTACHMENT_STORAGE_KEY_EXPOSED=NO
OBJECT_STORAGE_OPEN_BEFORE_PARTICIPANT_PROOF=NO
CARRIER_ATTACHMENT_CONTENT_FINALIZED_ONLY=YES
CARRIER_ATTACHMENT_CONTENT_INTEGRITY_VERIFIED=YES
CUSTOMER_ATTACHMENT_DIRECT_STORAGE_URL_EXPOSED=NO
CUSTOMER_SIGNATURE_CREATE_ENABLED=NO
CUSTOMER_SIGNING_SESSION_ENABLED=NO
CUSTOMER_QES_VERIFIED_CLAIM_ENABLED=NO
CUSTOMER_PORTAL_MAY_USE_LEGACY_DOCUMENT_ROUTE=NO
```

## Gap markers

```text
CP_API_005_CARRIER_SOURCE_READ=RESOLVED
CP_API_005_CARRIER_PUBLIC_GATEWAY=OPEN
```

No migration was required. Existing shipment, document, and attachment columns are enough.

```text
MIGRATION_CREATED=NO
GATEWAY_CODE_CHANGED=NO
FRONTEND_CODE_CHANGED=NO
STAGING_CHANGED=NO
```
