# Shipper documents source safety 0.1A

## Status

```text
STAGE=SHIPPER-DOCUMENTS-SOURCE-SAFETY-0.1A
SHIPPER_DOCUMENT_READ_SOURCE_SAFE=YES
SHIPPER_DOCUMENTS_CUSTOMER_SAFE=NO_PENDING_GATEWAY
CUSTOMER_DOCUMENTS_SAFE=NO
SHIPPER_DOCUMENT_WRITE_SAFE=NO
SHIPPER_EDO_SIGNING_SAFE=NO
SHIPPER_QES_SAFE=NO
SHIPPER_OPERATOR_EXCHANGE_SAFE=NO
SHIPPER_EDO_LEGAL_ACTIONS_SAFE=NO
```

This stage makes document-service able to prove a shipper participant read. It does not make the public API Gateway customer-safe. Agent A still has to authorize shipper membership and close the legacy document routes for shipper users. Agent F still has to build the read-only portal UI.

Reading a document is not a legal signature, a qualified electronic signature, or an operator exchange.

## Historical gap

CP-API-005: generic document reads and writes treated caller-supplied tenant as authority, and the generic document detail exposed storage and payload internals. A shipment link was also being treated as if every document on that shipment could be shown to every commercial party.

```text
CP_API_005_TENANT_AUTHORITY=RESOLVED
CP_API_005_SHIPPER_SOURCE_READ=RESOLVED
CP_API_005_SHIPPER_PUBLIC_GATEWAY=OPEN
CP_API_005_SHIPPER_WRITES=OPEN
```

## Trusted tenant

`X-Tenant-ID` is the tenant authority for generic document list and for Create, CreateVersion, AddFile, ReadyForSigning, Cancel, Archive, CreateSession, and AddSignature.

`GET /v1/documents` no longer selects `?tenant_id=`. An empty query uses the trusted tenant. A query equal to the trusted tenant stays compatible. A conflicting query is rejected and does not list the other tenant.

On those writes, including `POST /v1/documents/{id}/signing-sessions` and `POST /v1/signing-sessions/{id}/signatures`, a body `tenant_id` may be omitted or may repeat the trusted tenant. A mismatch is rejected before a repository mutation. Attachment signature upload already takes its tenant from `X-Tenant-ID` and rejects a conflicting body tenant.

```text
DOCUMENT_LIST_TENANT_SERVER_OWNED=YES
DOCUMENT_LIST_CALLER_TENANT_AUTHORITY=NO
DOCUMENT_WRITE_TENANT_SERVER_OWNED=YES
SIGNING_WRITE_TENANT_SERVER_OWNED=YES
CREATE_SIGNING_SESSION_TENANT_SERVER_OWNED=YES
ADD_SIGNATURE_TENANT_SERVER_OWNED=YES
ATTACHMENT_SIGNATURE_TENANT_SERVER_OWNED=YES
BODY_TENANT_OVERRIDE=DENIED
CROSS_TENANT_WRITE_BY_BODY=DENIED
CROSS_TENANT_SIGNATURE_BY_BODY=DENIED
CP_API_005_TENANT_AUTHORITY=RESOLVED
```

This hardens existing operator APIs. It does not add a portal write capability.

## Shipper participant fact

A customer read is allowed only when one query proves all of the following:

- the trusted tenant matches `documents.documents.tenant_id` and `transport.shipments.tenant_id`
- `related_entity_type` is `SHIPMENT`
- `related_entity_id` is `transport.shipments.id`
- `transport.shipments.shipper_company_id` is the requested shipper company
- the shipment and the document are not deleted
- the document type and status are on the allowlists below

`owner_company_id` alone is not authority. A document owned by the requested shipper but linked to another shipper's shipment is denied. An operational document owned by the carrier and linked to the requested shipper's shipment may be read.

```text
SHIPPER_DOCUMENT_PARTICIPATION_PROOF_REQUIRED=YES
OWNER_COMPANY_ID_ALONE_IS_AUTHORITY=NO
CUSTOMER_DOCUMENT_READ_DB_SCOPE_EXACT=YES
SOURCE_SERVICE_OWNS_DOCUMENT_PARTICIPANT_FACT=YES
SOURCE_SERVICE_OWNS_USER_MEMBERSHIP_AUTH=NO
```

document-service does not decide `SHIPPER_ADMIN` or `SHIPPER_LOGIST`. That remains the API Gateway.

The list requires `shipment_id`. There is no tenant-wide customer inbox. A missing or malformed shipment id is 400. A shipment that is not this shipper's shipment in this tenant is 404, with the same not-found outcome as a missing shipment. A foreign shipper, a wrong tenant, or a company that is only carrier, forwarder, or consignee on that shipment gets 404 on detail and attachment routes. The response does not reveal that the document exists.

```text
SHIPPER_DOCUMENT_LIST_REQUIRES_SHIPMENT_ID=YES
CROSS_SHIPPER_DOCUMENT_READ=DENIED
CROSS_SHIPPER_DOCUMENT_DISCLOSURE=NO
SHIPPER_DOCUMENT_LIST_N_PLUS_ONE=NO
```

The list is one shipment visibility query plus one joined document query. It does not load the tenant document set and then look up shipments one by one. The default page size is 20 and the maximum is 50.

## Safe document class

Operational allowlist: `ETRN`, `EPD`, `WAYBILL`, `POD`, `DISCREPANCY_ACT`, `ECMR`.

Excluded even when `related_entity_type=SHIPMENT` and the shipment id matches: `INVOICE`, `VAT_INVOICE`, `ACT`, `UPD`, `CLAIM`.

One physical shipment can sit in a shipper-forwarder relationship and a forwarder-carrier relationship. The shipment id does not make those commercial documents interchangeable.

```text
FINANCIAL_DOCUMENTS_EXPOSED_TO_SHIPPER_SOURCE=NO
CLAIM_DOCUMENTS_EXPOSED_TO_SHIPPER_SOURCE=NO
CUSTOMER_SETTLEMENT_DOCUMENTS_EXPOSED=NO
SUPPLY_SIDE_DOCUMENTS_EXPOSED_TO_SHIPPER=NO
FORWARDER_DUAL_CHAIN_COLLAPSED=NO
```

Visible statuses: `READY_FOR_SIGNING`, `SIGNING_IN_PROGRESS`, `SIGNED`, `SENT_TO_OPERATOR`, `ACCEPTED`, `REJECTED`, `ARCHIVED`.

`DRAFT` is not visible. `CANCELLED` stays excluded. Repository history does not show that a cancelled document had already been customer-visible.

```text
CUSTOMER_DRAFT_DOCUMENT_VISIBLE=NO
CUSTOMER_CANCELLED_DOCUMENT_VISIBLE=NO
```

## Source routes

These routes exist only on document-service:

- `GET /v1/shipper/documents?shipper_company_id=&shipment_id=`
- `GET /v1/shipper/documents/{documentId}?shipper_company_id=`
- `GET /v1/shipper/documents/{documentId}/attachments?shipper_company_id=`
- `GET /v1/shipper/documents/{documentId}/attachments/{attachmentId}?shipper_company_id=`
- `GET /v1/shipper/documents/{documentId}/attachments/{attachmentId}/content?shipper_company_id=`

`related_entity_type` is fixed to `SHIPMENT`. A caller cannot override it. `tenant_id` on these routes is not authority.

The customer DTO is id, document number, type, status, owner company, related entity type and id, legal language, timestamps, and version. It does not include tenant id, `payload_json`, XML or PDF paths, storage provider, bucket, or object key.

```text
CUSTOMER_DOCUMENT_PAYLOAD_JSON_EXPOSED=NO
CUSTOMER_DOCUMENT_STORAGE_PROVIDER_EXPOSED=NO
CUSTOMER_DOCUMENT_BUCKET_EXPOSED=NO
CUSTOMER_DOCUMENT_OBJECT_KEY_EXPOSED=NO
CUSTOMER_DOCUMENT_INTERNAL_PATH_EXPOSED=NO
CUSTOMER_USES_LEGACY_DOCUMENT_FILES=NO
```

Legacy `documents.document_files` stay on the operator detail route. The shipper source uses `documents.document_attachments`.

Only `FINALIZED` attachments are listed or downloaded. Metadata is id, document id, optional version id, file name, media type, size, sha256, status, created time, and finalized time. Storage key, bucket, provider, and idempotency key are not returned. Content is opened only after the participant proof, the attachment belongs to that document and tenant, and the status is finalized. The existing checksum and size check still runs. The response sets `Content-Type`, `X-Content-SHA256`, and `Content-Disposition` from the stored file name. It does not return a storage URL.

```text
CUSTOMER_ATTACHMENT_DRAFT_VISIBLE=NO
CUSTOMER_ATTACHMENT_STORAGE_KEY_EXPOSED=NO
CUSTOMER_ATTACHMENT_CONTENT_FINALIZED_ONLY=YES
CUSTOMER_ATTACHMENT_CONTENT_INTEGRITY_VERIFIED=YES
SHIPPER_DOCUMENT_LIST_SOURCE_ROUTE=YES
SHIPPER_DOCUMENT_DETAIL_SOURCE_ROUTE=YES
SHIPPER_ATTACHMENT_LIST_SOURCE_ROUTE=YES
SHIPPER_ATTACHMENT_DETAIL_SOURCE_ROUTE=YES
SHIPPER_ATTACHMENT_CONTENT_SOURCE_ROUTE=YES
SHIPPER_DOCUMENT_SOURCE_CONTRACT_READY=YES
SHIPPER_ATTACHMENT_SOURCE_CONTRACT_READY=YES
```

A foreign shipper, a draft attachment, or an attachment id from another document does not open object storage.

## Signing

Customer signing is not enabled. The shipper source contract does not create a signature or a signing session. Operator signing routes remain, and their tenant now comes from `X-Tenant-ID`. `UNVERIFIED` is not presented as a valid qualified signature. Signature metadata is left off this read source.

```text
CUSTOMER_SIGNATURE_CREATE_ENABLED=NO
CUSTOMER_SIGNING_SESSION_ENABLED=NO
CUSTOMER_QES_VERIFIED_CLAIM_ENABLED=NO
```

## Legacy routes

`/v1/documents`, `/v1/documents/{id}`, `/v1/signing-sessions`, and the attachment write routes remain for operator and backward-compatible callers. They are not company-scoped customer routes.

```text
CUSTOMER_PORTAL_MAY_USE_LEGACY_DOCUMENT_ROUTE=NO
```

Agent A must not expose those legacy routes to shipper users on the public gateway. This stage does not register `/api/v1/shipper/documents` on the API Gateway and does not change Shipper Portal.

```text
GATEWAY_CODE_CHANGED=NO
FRONTEND_CODE_CHANGED=NO
MIGRATION_CREATED=NO
STAGING_CHANGED=NO
```

No migration was required. The existing shipment, document, and attachment columns are enough.

## Dependencies

- Agent A: public gateway membership authorization and closure of the legacy document bypass for shipper users. The later result is [SHIPPER_DOCUMENTS_GATEWAY_0_1.md](SHIPPER_DOCUMENTS_GATEWAY_0_1.md).
- Agent F: read-only shipment document UI against the source contract after the gateway exists.
- Later party-context work: financial and supply-side documents. This stage does not collapse the forwarder dual chain.

## Gateway result

The status block at the top of this file is the source-contract record and is unchanged. [SHIPPER_DOCUMENTS_GATEWAY_0_1.md](SHIPPER_DOCUMENTS_GATEWAY_0_1.md) records the later gateway stage. For shipper reads only, `SHIPPER_DOCUMENTS_CUSTOMER_SAFE` moves from `NO_PENDING_GATEWAY` to `YES_READ_ONLY` and `CP_API_005_SHIPPER_PUBLIC_GATEWAY=RESOLVED`. Writes stay open as a gap. Global customer documents stay `NO_GLOBAL_SHIPPER_READ_ONLY`.
