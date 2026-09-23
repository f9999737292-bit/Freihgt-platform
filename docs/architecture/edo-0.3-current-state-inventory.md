# EDO 0.3 Current-State Inventory

## Status

```text
DOCUMENT_STATUS=DISCOVERY
EDO_0_2_STATUS=IMPLEMENTED_ACCEPTED
EDO_0_3_STATUS=NOT_STARTED
IMPLEMENTATION_AUTHORIZED=NO
BASE_SHA=c38f2d13f08fffe2018d631881ff93b3ad2479f2
CHECKED_ON=2026-09-23
```

Evidence is the tree at `origin/main` `c38f2d13f08fffe2018d631881ff93b3ad2479f2`. This inventory does not change product code.

Accepted EDO 0.2 baseline remains [edo-0.2-final-report.md](edo-0.2-final-report.md), [ADR-EDO-001](../adr/ADR-EDO-001-canonical-edo-document-ownership.md) through [ADR-EDO-009](../adr/ADR-EDO-009-cross-workstream-mutation-policy.md), and [ADR-PLAT-001](../adr/ADR-PLAT-001-membership-user-roles-canonical-writer.md).

## 1. Capabilities present in code

`document-service` is a pre-EDO registry, not an EDO 0.3 implementation.

| Capability | Evidence | Classification |
|------------|----------|----------------|
| Document aggregate with `tenant_id`, `owner_company_id`, type, single `document_status` | `infrastructure/migrations/000005_create_documents_tables.up.sql`; `services/document-service/internal/domain/document.go` | Implemented |
| Types `ETRN`, `EPD`, `WAYBILL`, `POD`, `DISCREPANCY_ACT`, `CLAIM`, `INVOICE`, `VAT_INVOICE`, `ACT`, `UPD`, `ECMR` | same migration check constraint | Implemented as labels only |
| Versions (`documents.document_versions`) and file metadata (`documents.document_files`, optional SHA-256) | migration `000005` | Partial revision model |
| Signing sessions and signature rows, including `certificate_fingerprint` and verification status | migration `000005`; `internal/domain/signing_session.go` | Partial signing evidence |
| Status guards: new version only from `DRAFT` or `REJECTED`; cancel blocked for `SIGNED` and `ARCHIVED`; archive only from `SIGNED` or `ACCEPTED` | `internal/domain/document.go` | Implemented |
| File metadata may be added without a status guard | `DocumentService.AddFile` in `internal/service/document_service.go` | Gap |
| POD upload intent with idempotency key | migration `000033_add_document_upload_intent.up.sql`; `internal/service/pod_upload_service.go` | Implemented for POD only |
| Local filesystem object store | `internal/platform/storage/local.go` | Implemented; not WORM |
| HTTP routes for create, list, get, version, file, ready-for-signing, signing session, cancel, archive | `internal/http/router.go` | Implemented |
| Gateway proxy of `/api/v1/documents` and `/api/v1/signing-sessions` | `services/api-gateway/internal/http/proxy.go` | Implemented |
| OpenAPI document paths | `packages/openapi/document-service.yaml` | Implemented for the routes above |
| Unit tests for status guards and signature completion | `internal/service/document_service_test.go`; `internal/domain/document_test.go` | Implemented |
| POD integration tests, including cross-tenant shipment rejection and idempotency | `internal/integration/podupload/pod_upload_integration_test.go` | Implemented for POD |

`storage_provider` defaults to `S3` in SQL. The running store implementation in this tree is `LocalObjectStore`.

## 2. Capabilities that exist only in ADR or roadmap

These names are frozen in EDO 0.2 and are absent as tables or services in this tree:

| Concept | Source | Code |
|---------|--------|------|
| `DocumentPackage` | [edo-0.2-domain-model-freeze.md](edo-0.2-domain-model-freeze.md); EDO-0.3 deliverable in the final report | No table, no type |
| `DocumentRelationship` | same | No table, no type |
| `CertificateEvidence` as its own aggregate | ADR-EDO-001; archive matrix marks it EDO-0.3+ | Only `signatures.certificate_fingerprint` |
| `PowerOfAttorneyEvidence` / MChD | ADR-EDO-001 | No matches in `document-service` |
| `DeliveryEvidence`, `OperatorReceipt`, `FormatDefinition`, `FormatVersion`, `ValidationResult`, `ArchiveManifest` | domain model freeze | No tables |
| `transport-edo-service` and EPD operator port | [ADR-EDO-004](../adr/ADR-EDO-004-epd-operator-port-ownership.md) | No service directory |
| `edo.document.*` Kafka outbox | [edo-0.2-event-contracts.md](../events/edo-0.2-event-contracts.md) | No outbox package in `document-service` |
| Orthogonal business, delivery, signature, and operator states | [edo-0.2-document-state-machines.md](edo-0.2-document-state-machines.md) | One `document_status` column |
| Mandatory `document_id` before operator-facing UPD states | [ADR-EDO-002](../adr/ADR-EDO-002-billing-edo-boundary.md) | `billing.upd_documents.document_id` is nullable |
| Receivable and factoring chain | [ADR-EDO-005](../adr/ADR-EDO-005-receivable-vs-payment-obligation.md) | Design only |
| S3/WORM archive | [ADR-EDO-007](../adr/ADR-EDO-007-legal-archive-boundary.md) | Not provisioned; Selectel object lock not verified in-repo |

Program next phase for EDO is **EDO-0.3 document extensions**. TEDO next phase is **TEDO-0.3 ETRN lifecycle design**, which is a different workstream. See [workstream-status-v0.1.md](../program/workstream-status-v0.1.md).

## 3. Services and data owners touched by a future EDO 0.3

Recommended scope stays inside `document-service` and schema `documents`.

| Service | Role for EDO 0.3 | Change in recommended scope |
|---------|------------------|-----------------------------|
| `document-service` | Canonical document owner (ADR-EDO-001) | Future additive schema and rules only, after a separate authorization |
| `api-gateway` | Human and integration auth, reverse proxy | No change in this discovery. Later exposure of new routes needs its own review |
| `identity-service` / `company-service` | User and company existence checks already used by signing | No ownership change. PLAT-0.1 dual-write remains outside EDO 0.3 |
| `billing-register-service` | Commercial UPD projection | Out of recommended EDO 0.3. Bridge is EDO-0.5 in the program row for FC |
| `payment-service` | `PaymentObligation` and its own outbox | Out of scope |
| `shipment-service` / `transport-order-service` | Referenced by `related_entity_type` | Read-only references. No LOG or TMS schema edits (ADR-EDO-009) |
| `transport-edo-service` | Future TEDO owner | Does not exist. Out of EDO 0.3 |

## 4. Existing contracts

| Contract | Location | Relevant fact |
|----------|----------|----------------|
| Document OpenAPI | `packages/openapi/document-service.yaml` | CRUD-style document, version, file, signing, cancel, archive |
| Document SQL | `infrastructure/migrations/000005_create_documents_tables.up.sql`, `000033_add_document_upload_intent.up.sql` | Five document tables plus upload intent |
| Billing UPD | `infrastructure/migrations/000006_create_billing_tables.up.sql` | `billing.upd_documents.document_id UUID` nullable; function codes `СЧФ`, `СЧФДОП`, `ДОП` |
| Billing idempotency | `000044_billing_closing_idempotency_v1.8.1.up.sql` | Unique `(tenant_id, register_id)` on UPD rows |
| Settlement evidence pointer | `000042_freight_settlement_v1.7.up.sql` | `evidence_document_id` |
| Payment | `services/payment-service` | Separate aggregate and transactional outbox. Not an EDO document store |
| Events | [edo-0.2-event-contracts.md](../events/edo-0.2-event-contracts.md), [event-catalog-v0.1.md](../events/event-catalog-v0.1.md) | `edo.document.*` proposed, not produced |

## 5. Tenant and company isolation

Current document read isolation is not complete and is not fail-closed.

```text
List and mutations:
GetByIDAndTenant / tenant predicate = IMPLEMENTED

GET /v1/documents/{id}:
DocumentHandler.GetByID → service.GetDetail → repository.GetByID
repository predicate = id + deleted_at only
tenant predicate = ABSENT

GetSession:
tenant predicate = ABSENT
```

The service method on that path is `DocumentService.GetByID`. It calls repository `GetDetail`, which calls repository `GetByID`.

```text
DOCUMENT_READ_TENANT_ISOLATION_REMEDIATION_REQUIRED
```

Closing that gap is a separate product-remediation wave with its own controller review. It is a prerequisite of any EDO 0.3 implementation wave. This discovery does not change the handler or the repository.

```text
CURRENT_PRODUCT_SECURITY_GAP:
GET /v1/documents/{id} does not enforce tenant predicate at repository read path.

ACTION:
Separate security remediation required.

NOT_IN_THIS_PR:
No product fix, migration, API change, or test implementation.
```

- List passes a caller-supplied `tenant_id` into `List`, which filters `tenant_id` and `deleted_at`. It does not filter `owner_company_id`.
- Mutations that call `GetByIDAndTenant` also take `tenant_id` from the JSON body. That body value is not the gateway trust boundary.
- POD internal handlers take `X-Tenant-ID` (`internal/http/handlers/pod_upload_handler.go`). The header is trusted only when the gateway set it.
- Signing checks that signer user and signer company exist in the same tenant. That is existence, not a membership or power-of-attorney check.

## 6. Authentication

Gateway router applies `AuthWithIntegrationSupport` and `IntegrationAuth` before proxies (`services/api-gateway/internal/http/router.go`). Document routes are not a separate public bypass in `proxy.go`.

Three identities already exist in the platform and must stay distinct for any later EDO 0.3 API:

| Actor | Where it is enforced today | EDO 0.3 note |
|-------|----------------------------|--------------|
| Human JWT | Gateway auth middleware | Browser and operator callers |
| Integration JWT | Gateway integration auth | Machine clients |
| Service-to-service | Internal POD route and gateway document client using trusted context | Not a third document identity model |

`document-service` itself does not verify JWT. It trusts the tenant value it is given. That is a boundary gap for later implementation, not a license to accept client-supplied tenant as the authorization source.

## 7. Signature, power of attorney, and authority

Implemented signature types: `SIMPLE_ELECTRONIC`, `ENHANCED_UNQUALIFIED`, `ENHANCED_QUALIFIED`. Verification status values include `PENDING`, `VALID`, `INVALID`, `EXPIRED`, `REVOKED`, `FAILED`.

```text
documents.signatures.document_id=IMPLEMENTED
revision binding=ABSENT
```

`documents.signatures` references `document_id` and `signing_session_id`. It has `certificate_fingerprint`. It does not reference `document_version_id`. This inventory does not invent that column. A fingerprint is not a binding to the signed revision.

No code path validates a qualified certificate chain, a timestamp authority, or a machine-readable power of attorney. No MChD GUID column exists. Storing a signature row does not mean the signature is legally valid. That question stays `LEGAL_VERIFICATION_REQUIRED`.

Whether a qualified signature or an MChD is legally required for a document type is `LEGAL_VERIFICATION_REQUIRED`. See [edo-0.3-regulatory-source-register.md](edo-0.3-regulatory-source-register.md).

## 8. Document lifecycle

The code lifecycle is one column:

`DRAFT` → `READY_FOR_SIGNING` → `SIGNING_IN_PROGRESS` → `SIGNED` → (`SENT_TO_OPERATOR` | `ACCEPTED` | `REJECTED` | `ARCHIVED` | `CANCELLED`).

EDO 0.2 forbids collapsing business, delivery, signature, and operator state into one column. The code still does. `SENT_TO_OPERATOR` can be stored even though `transport-edo-service` does not exist.

## 9. Immutability, audit, archive, retention

| Control | Current state |
|---------|----------------|
| New version after `SIGNED` or `ARCHIVED` | Rejected in domain validation |
| Update of an existing version payload | No update API found. No database trigger forbids `UPDATE` |
| Add file after signature | Allowed |
| Delete | `documents.documents.deleted_at` exists. Child rows use `ON DELETE CASCADE` |
| Archive | Status flip to `ARCHIVED`. No `ArchiveManifest`, no hash manifest, no legal hold |
| Retention period | Not stored. Registry entry `LR-RU-ARCHIVE-RET-001` is unverified |
| Audit trail | No `documents` audit/outbox table. Platform `core.audit_logs` correlation is an EDO-0.4+ item in the archive matrix |

## 10. Redelivery, idempotency, deduplication

POD upload intents deduplicate on `(tenant_id, driver_id, idempotency_key)`.

Document create, version, file, and signature routes have no idempotency key. Unique `(tenant_id, document_number)` prevents duplicate numbers. It does not make a retried POST safe.

Event idempotency keys in [edo-0.2-event-versioning-policy.md](../events/edo-0.2-event-versioning-policy.md) apply when an EDO producer exists. `document-service` has no outbox.

## 11. Events, outbox, versioning

Catalog SSOT is markdown. JSON Schema files for `edo.document.*` are still a finding (`GATE_EVENT_CONTRACTS=PASS_WITH_FINDING`).

Payment-service has a transactional outbox. Document-service does not. EDO 0.2 policy: the first EDO bus producer must use an outbox and must not publish directly to Kafka. That policy is not an authorization to build the bus in EDO 0.3.

Proposed names already reserved include `edo.document.revision_added` and `edo.document.signature_state_changed`. Package and relationship names are not assigned in the accepted catalog.

## 12. External operators and adapters

ADR-EDO-008 sets `EXTERNAL_OPERATOR_MODE=YES`, `FUTURE_OWN_OPERATOR_READY=YES`, `OWN_IS_EPD_OPERATOR_MODE=NO`, `GIS_EPD_CONNECTED=NO`.

No adapter, credential vault, or operator HTTP client exists in this tree. This discovery does not claim that BINTRANS is an accredited EDI operator or an operator of an electronic transport-document information system.

## 13. Electronic consignment note and transport electronic documents

`ETRN` and `EPD` are allowed `document_type` values. There is no title model (Т1–Тn), no GIS EPD exchange, and no operator transaction state table.

TEDO-0.3 is the program phase for ETRN lifecycle design. It is not EDO 0.3.

## 14. Multimodal boundary

ADR-EDO-003 freezes `Shipment → TransportJourney → TransportLeg → CargoHandover` and `ONE_SHIPMENT_ID`. Code remains road-oriented. EDO 0.3 does not add leg tables. A document may already point at `SHIPMENT` or `TRANSPORT_ORDER` through `related_entity_*`.

## 15. Factoring and finance boundary

ADR-EDO-005 keeps Receivable distinct from PaymentObligation. Factoring aggregates are future FF work. `payment-service` is the payment rail. EDO 0.3 does not create either aggregate and does not store financing state on `Document`.

## 16. Personal data, secrets, logging

Document payloads are `jsonb` and file bytes. They can contain personal data when a business document includes a person. No dedicated redaction policy was found in `document-service`.

This discovery records no secret values. Signing stores a fingerprint and a payload path, not a private key. ADR-EDO-007 places private keys outside `document-service`.

## 17. Selectel and archive storage

Selectel is the staging host family described in staging and pilot docs. EDO 0.2 records Selectel WORM / object lock as `EXTERNAL_INFRA_VERIFICATION_REQUIRED`. This discovery did not open a Selectel account, bucket, or object-lock configuration. INFRA-0.1 remains the storage phase. EDO 0.3 does not provision buckets.

## 18. OpenAPI, migrations, tests, acceptance

| Layer | Present for current document API | Present for EDO 0.3 entities |
|-------|----------------------------------|------------------------------|
| OpenAPI | Yes | No |
| Migrations | `000005`, `000033` | No |
| Unit tests | Status and signing guards | No |
| Integration tests | POD upload | No |
| Browser or operator acceptance | Not found for EDO package, relationship, or signature-evidence flows | No |

## 19. TMS and other workstreams

| Dependency | Nature |
|------------|--------|
| TMS / LOG shipment and transport order | Optional `related_entity_id`. Cross-workstream request `CWS-EDO-2026-002` is inventoried and not submitted |
| TEDO | ETRN lifecycle and operator port. Separate next phase |
| FC / billing | `CWS-EDO-2026-001`, phase label EDO-0.5 |
| PLAT | `CWS-EDO-2026-003`, phase label PLAT-0.1 |
| INFRA | `CWS-TEDO-2026-001` for vault and object storage. Not an EDO 0.3 code change |
| Control Tower | Must not assume primary mode (finding F-012) |

Template: [cross-workstream-request-template.md](../program/cross-workstream-request-template.md).

## 20. Explicitly outside EDO 0.3

See [edo-0.3-scope.md](edo-0.3-scope.md). Short form: operator exchange, GIS EPD, ETRN titles, billing bridge, receivable/factoring, multimodal leg schema, WORM provisioning, PLAT dual-write remediation, and any product code in this discovery.

## References

- [edo-0.3-discovery.md](edo-0.3-discovery.md)
- [edo-0.3-gap-analysis.md](edo-0.3-gap-analysis.md)
- [edo-0.2-domain-model-freeze.md](edo-0.2-domain-model-freeze.md)
- [edo-0.2-archive-boundary.md](edo-0.2-archive-boundary.md)
