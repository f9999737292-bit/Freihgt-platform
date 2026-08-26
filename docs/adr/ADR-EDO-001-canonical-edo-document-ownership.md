# ADR-EDO-001: Canonical EDO Document Ownership

## Status

Accepted — architecture freeze (EDO-0.2)

## Context

Discovery v0.1 identified `document-service` as the pre-EDO document registry with signing sessions, file storage, and document types including `UPD`, `ETRN`, `EPD`. Billing holds fiscal SSOT in `billing.upd_documents` with optional `document_id` link. EDO must unify legally significant document lifecycle without duplicating platform identity or logistics aggregates.

## Decision

**`document-service` (EDO workstream) is the canonical owner of the EDO `Document` aggregate and all legally significant electronic document artifacts.**

Ownership includes:

- XML / structured payload representation and format version binding
- Document revisions and immutability rules post-signature
- Signatures, certificate evidence, power-of-attorney (MChD) evidence
- Validation evidence and delivery evidence
- Operator receipts and cancellation/correction document lifecycle
- Immutable archive manifest and retention metadata

**Non-owners must reference documents by `document_id` only.** They must not store duplicate XML payloads, signature bytes, or operator transaction state.

### Aggregate map (canonical)

| Entity | Owner | Aggregate root |
|--------|-------|----------------|
| Document | document-service | Document |
| DocumentPackage | document-service | DocumentPackage |
| DocumentRevision | document-service | Document (via DocumentRevision) |
| DocumentRelationship | document-service | DocumentPackage or Document |
| DocumentSignature | document-service | Document |
| CertificateEvidence | document-service | Document |
| PowerOfAttorneyEvidence | document-service | Document |
| DocumentEvent (audit/bus) | document-service | Document |
| DeliveryEvidence | document-service | Document |
| OperatorReceipt | document-service | Document |
| FormatDefinition / FormatVersion | document-service | FormatDefinition |
| ValidationResult | document-service | Document |
| ArchiveManifest | document-service | Document or DocumentPackage |

### Explicit non-ownership

| Concern | Owner |
|---------|-------|
| Commercial/accounting intent, monetary amounts | billing-register-service |
| Settlement basis, tax source rows | billing-register-service |
| Shipment execution FSM | shipment-service |
| EPD operator HTTP/API transaction correlation | transport-edo-service (see ADR-EDO-004) |
| Platform identity (User, Company, Tenant) | identity-service / company-service |

## Consequences

### Positive

- Single legal document lifecycle and archive chain
- Billing and transport contexts stay thin — reference IDs only
- Existing `documents.*` schema is the evolution anchor

### Negative / migration

- Billing closing flows must treat `billing.upd_documents` as commercial projection; mandatory `document_id` bridge required before production EDO (see ADR-EDO-002)
- Gateway timeline document events are derived reads, not SSOT — must converge on `edo.document.*` bus contracts (ADR-EDO-006)

## Compliance note

Statutory format, signature, and MChD rules are **EXTERNAL_LEGAL_VERIFICATION_REQUIRED**. This ADR does not assert regulatory compliance.

## References

- Discovery finding F-009 (UPD SSOT split) — resolved via ADR-EDO-002
- `services/document-service/internal/domain/document.go`
- `infrastructure/migrations/000006_create_billing_tables.up.sql` (`billing.upd_documents.document_id`)
