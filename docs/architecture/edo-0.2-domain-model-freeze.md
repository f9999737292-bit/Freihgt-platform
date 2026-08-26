# EDO-0.2 Domain Model Freeze

## Status

Architecture freeze — no SQL, no product code (EDO-0.2)

## Purpose

Proposed aggregate map for EDO core entities. Every entity defines OWNER, AGGREGATE_ROOT, IDENTIFIER, TENANT_BOUNDARY, COMPANY_BOUNDARY, IMMUTABILITY_RULE, RETENTION_CONSIDERATION.

## Entity catalog

| Entity | OWNER | AGGREGATE_ROOT | IDENTIFIER | TENANT_BOUNDARY | COMPANY_BOUNDARY | IMMUTABILITY_RULE | RETENTION_CONSIDERATION |
|--------|-------|----------------|------------|-----------------|------------------|-------------------|-------------------------|
| **Document** | document-service | Document | `document.id` | `tenant_id` required on all rows | `owner_company_id`, party company refs | Post-SIGNATURE: metadata mutable only via new revision; signed bytes immutable | Legal retention per document type registry; **EXTERNAL_LEGAL_VERIFICATION_REQUIRED** |
| **DocumentPackage** | document-service | DocumentPackage | `document_package.id` | `tenant_id` | Assembling company | Package membership frozen when package sealed | Same as contained documents |
| **DocumentRevision** | document-service | Document | `document_revision.id`, FK `document_id` | via Document | via Document | New revision for each legally significant change; signed revisions immutable | Full revision chain retained |
| **DocumentRelationship** | document-service | Document or DocumentPackage | `relationship.id` | via parent | via parent | Append-only graph edges | Retained with parent document |
| **DocumentSignature** | document-service | Document | `signature.id` | via Document | `signer_company_id` | Immutable after completion | Retained for legal hold |
| **CertificateEvidence** | document-service | Document | `certificate_evidence.id` | via Document | Signer company | Immutable once recorded | Retained with signature |
| **PowerOfAttorneyEvidence** | document-service | Document | `mchd_evidence.id` | via Document | Principal/representative companies | Immutable; revocation adds new evidence row | **EXTERNAL_LEGAL_VERIFICATION_REQUIRED** |
| **DocumentEvent** | document-service | Document | `event.id` (outbox/bus) | `tenant_id` in envelope | `company_id` when applicable | Append-only audit/bus | Bus retention per INFRA policy; audit permanent |
| **DeliveryEvidence** | document-service | Document | `delivery_evidence.id` | via Document | Recipient/sender companies | Immutable | Operator retention rules TBD |
| **OperatorReceipt** | document-service | Document | `operator_receipt.id` | via Document | Operator company ref | Immutable | Long-term archive |
| **FormatDefinition** | document-service | FormatDefinition | `format_definition.id` | Global or tenant-scoped catalog | N/A | Versioned catalog entries | Current + superseded versions kept |
| **FormatVersion** | document-service | FormatDefinition | `format_version.id` | via FormatDefinition | N/A | Immutable once published | XSD/schema artifacts retained |
| **ValidationResult** | document-service | Document | `validation_result.id` | via Document | Validator context | Immutable result snapshot | Retained with revision |
| **ArchiveManifest** | document-service | Document or DocumentPackage | `archive_manifest.id` | via parent | Archiving company | Immutable after seal | WORM tier; legal hold capable |

## Related aggregates (other workstreams)

| Entity | OWNER | Relationship to EDO |
|--------|-------|---------------------|
| Shipment | shipment-service | Referenced by `shipment_id` on documents; ONE_SHIPMENT_ID invariant |
| TransportJourney / Leg / Handover | shipment-service | Optional `transport_leg_id` on transport documents |
| BillingRegister / UPDDocument | billing-register-service | Commercial projection; `document_id` FK to Document |
| EPDTransaction | transport-edo-service | Operator correlation; references `document_id` |
| Receivable | FF (future) | May reference `document_id` for factoring evidence |
| PaymentObligation | payment-service | Separate from Receivable; may share billing register source |

## Anti-patterns (blocked)

- `edo_shipments`, `edo_companies`, `edo_users` parallel tables
- Duplicate XML in billing schema
- Collapsed `status=SIGNED_AND_SENT_AND_ACCEPTED` single column (see state machines doc)
- Second canonical shipment entity (`MultimodalShipment`)

## References

- ADR-EDO-001, ADR-EDO-002, ADR-EDO-003
- Discovery EDO_TARGET section
