# EDO 0.3 Gap Analysis

## Status

```text
DOCUMENT_STATUS=DISCOVERY
IMPLEMENTATION_AUTHORIZED=NO
EDO_0_3_IMPLEMENTATION=NOT_STARTED
```

Gaps below are the distance between accepted EDO 0.2 contracts, the code at `c38f2d13f08fffe2018d631881ff93b3ad2479f2`, and the recommended EDO 0.3 scope. Closing a gap requires a later Task Contract. This document does not close any gap in code.

## Gap table

| ID | Topic | Accepted contract | Code at baseline | EDO 0.3 recommended disposition |
|----|-------|-------------------|------------------|---------------------------------|
| G-01 | DocumentPackage | document-service aggregate | Absent | In recommended scope, schema and rules only after authorization |
| G-02 | DocumentRelationship | Append-only edges | Absent | In recommended scope |
| G-03 | Immutable revisions | Signed bytes never mutate; new revision for a legally significant change | Version create is blocked after `SIGNED`/`ARCHIVED`. File add is not. Rows are `ON DELETE CASCADE`. No anti-update constraint | In recommended scope as rules on existing `document_versions` |
| G-04 | Signing evidence bound to a revision | Signature and certificate evidence refer to the immutable revision of the signed bytes | `documents.signatures.document_id` is implemented. Revision binding is absent. There is no `document_version_id` on `documents.signatures`. Fingerprint is not that binding | Future I1 contract: bind evidence to `document_version_id` or an equivalent revision plus content digest. Re-verification does not rewrite history. A new revision does not inherit the previous signature. Evidence is not a legal-validity claim |
| G-05 | MChD / authority | PowerOfAttorneyEvidence in ADR-EDO-001 | Absent | Controller option. Not in the named EDO 0.3 sentence. `LEGAL_VERIFICATION_REQUIRED` |
| G-06 | ArchiveManifest | ADR-EDO-007 says EDO-0.3+ adds the schema. Archive matrix puts legal hold, hash manifest, and WORM at EDO-0.4+ / INFRA-0.1 | Status `ARCHIVED` only | Deferred by the recommended variant. Controller may pull a metadata-only schema forward |
| G-07 | Orthogonal states | Four dimensions, never one column | One `document_status` | Not in the named EDO 0.3 sentence. Recorded as an option. Do not pretend the current column is compliant with the freeze |
| G-08 | Outbox and `edo.document.*` | First producer uses transactional outbox | No document outbox. Catalog names are proposed | Design revisions so a later outbox can emit `edo.document.revision_added` and `edo.document.signature_state_changed`. Do not create topics in EDO 0.3 |
| G-09 | Idempotency of document writes | Versioning policy for bus producers | Only POD upload intents | Package and relationship commands need an idempotency decision before implementation. Not built here |
| G-10 | Tenant trust on list and mutations | Gateway-set tenant | List and `GetByIDAndTenant` mutations filter `tenant_id`, but the value comes from the body or query | New routes must use the gateway tenant. Do not describe this path as fail-closed |
| G-10b | Document get-by-id tenant predicate | Every document read is tenant-scoped | Absent. `GET /v1/documents/{id}` is `DocumentHandler.GetByID` → `GetDetail` → `GetByID` with `id` and `deleted_at` only. `GetSession` also has no tenant predicate | `DOCUMENT_READ_TENANT_ISOLATION_REMEDIATION_REQUIRED`. Separate product wave and controller review before EDO 0.3 implementation. Not fixed in this discovery |
| G-10c | Package membership source of truth | Membership lives only on `DocumentPackage` | Neither package nor relationship tables exist | Do not add a `PACKAGE_CONTAINS_DOCUMENT` relationship. One fact, one store. Uniqueness is one document once per package, same tenant |
| G-11 | Company isolation | Company boundary on aggregates | Tenant filter only | Later waves must define company authorization. Discovery does not invent a new RBAC model |
| G-12 | Billing `document_id` | Mandatory before operator-facing UPD states (ADR-EDO-002) | Nullable | Out of scope. Program label EDO-0.5 / `CWS-EDO-2026-001` |
| G-13 | Operator port | transport-edo-service | Service absent. `GIS_EPD_CONNECTED=NO` | Out of scope. TEDO |
| G-14 | ETRN lifecycle | TEDO-0.3 in the roadmap | Type label only | Out of scope |
| G-15 | Multimodal legs | ADR-EDO-003 | Not in shipment schema as the frozen extension | Out of scope. LOG/MM |
| G-16 | Receivable / factoring | ADR-EDO-005 | Not implemented as EDO tables | Out of scope. FF |
| G-17 | WORM / Selectel object lock | External infra verification | Local disk store | Out of scope. INFRA-0.1 |
| G-18 | Legal rule registry | Structure only, all entries unverified | Unchanged | This discovery adds a source register. It does not mark any entry `VERIFIED` |
| G-19 | Format catalog | FormatDefinition / FormatVersion | Absent | Out of recommended EDO 0.3. Formats stay `LEGAL_VERIFICATION_REQUIRED` |
| G-20 | Tests for package, relationship, immutability | Required before any implementation is called accepted | Current tests cover the pre-EDO registry and POD | Strategy only. No new tests in this task |

## What is not a gap in EDO 0.3

- EDO 0.2 ADR pack is merged and present on `origin/main` (pull request #73, merge `18a85074`).
- Document tenant column and owner company column already exist. List and tenant-scoped mutations use them. Get-by-id does not. Read isolation is not complete.
- Signature type enum already distinguishes simple, enhanced unqualified, and enhanced qualified signatures. That enum is not a compliance claim.
- Payment outbox exists for payments. Copying it into `document-service` is not part of the recommended scope.

## Ambiguity that this discovery does not silently resolve

The final report names EDO 0.3 deliverables as DocumentPackage, DocumentRelationship, immutable revision rules, and signing evidence extensions, limited to additive `documents` migrations.

ADR-EDO-007 says EDO-0.3+ adds `ArchiveManifest`. The archive matrix schedules hash manifest, retention metadata, access audit, and legal hold at EDO-0.4+ / INFRA-0.1.

Those sentences are both accepted and they do not pick the same schema list. The resolution is a controller decision in [edo-0.3-architecture-options.md](edo-0.3-architecture-options.md). Recommended variant follows the final-report sentence and leaves `ArchiveManifest` to EDO-0.4.

## References

- [edo-0.3-current-state-inventory.md](edo-0.3-current-state-inventory.md)
- [edo-0.2-final-report.md](edo-0.2-final-report.md)
- [ADR-EDO-007](../adr/ADR-EDO-007-legal-archive-boundary.md)
- [edo-0.2-event-versioning-policy.md](../events/edo-0.2-event-versioning-policy.md)
