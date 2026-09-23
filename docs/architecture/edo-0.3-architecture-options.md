# EDO 0.3 Architecture Options

## Status

```text
DOCUMENT_STATUS=DISCOVERY
DECISION_STATUS=VARIANT_A_ACCEPTED_AS_DISCOVERY_SCOPE
ACCEPTED_ADR_CHANGE=NO
IMPLEMENTATION_AUTHORIZED=NO
```

No new ADR is accepted by this document. ADR-EDO-001 through ADR-EDO-009 stay as frozen by EDO 0.2.

## Why a decision is required

The roadmap sentence is specific. Two nearby accepted sentences are wider:

| Source | What it assigns to EDO 0.3 |
|--------|----------------------------|
| [edo-0.2-final-report.md](edo-0.2-final-report.md) `NEXT_RECOMMENDED_PHASE` | DocumentPackage, DocumentRelationship, immutable revision rules, signing evidence extensions, `documents` schema only |
| [edo-0.2-archive-boundary.md](edo-0.2-archive-boundary.md) | Version history, signature evidence, and certificate evidence are EDO-0.3+. Legal hold is EDO-0.4+ |
| [ADR-EDO-007](../adr/ADR-EDO-007-legal-archive-boundary.md) consequence | "EDO-0.3+ adds ArchiveManifest schema" |

`0.3+` means "0.3 or a later phase". It does not by itself put `ArchiveManifest` inside the named deliverable list. Signing evidence in ADR-EDO-001 also includes MChD, which the phase sentence does not name.

## Variant A — recommended

Follow the final-report sentence literally.

In a future authorized wave, `document-service` gains:

- `DocumentPackage` with tenant, assembling company, seal state, and membership frozen at seal. Membership is the only record that a document belongs to a package. Uniqueness is one document once per package. Package and member share one tenant.
- `DocumentRelationship` as append-only semantic edges between documents (correction, replacement, related document). It does not duplicate package membership. `PACKAGE_CONTAINS_DOCUMENT` is forbidden.
- Immutability rules on the existing revision and file rows: no in-place change and no cascade delete of signed artifacts; file attach follows the same signed-state rule as version create.
- Signing evidence extension bound to the immutable revision of the signed bytes. Current code has `documents.signatures.document_id` and no revision binding; do not invent `document_version_id`. The future column or equivalent proof must name that revision and its content digest. Digest algorithm and digest value refer to the signed revision. Re-verification does not rewrite historical evidence. A new revision does not inherit the previous signature. Store no private key. Do not call an external certificate authority in this variant. Evidence is not a claim of legal validity (`LEGAL_VERIFICATION_REQUIRED`).

Leave unchanged:

- `ArchiveManifest` and WORM references (EDO-0.4 / INFRA-0.1).
- MChD registry integration.
- The single `document_status` column, while documenting that it is still the EDO 0.2 anti-pattern.
- Operator, billing, TMS, payment, and factoring schemas.

This variant can be implemented later without a legal conclusion about operator licensing or retention years. It still must not claim that the resulting signatures are legally valid. It also does not start until `DOCUMENT_READ_TENANT_ISOLATION_REMEDIATION_REQUIRED` has its own controller review. Variant A does not treat today's get-by-id path as fail-closed.

## Variant B — pull ArchiveManifest metadata forward

Same as A, plus a `documents` table for `ArchiveManifest` with hash pointers and a legal-hold flag, and no object-storage provisioning.

Reject as the default. The archive matrix already places hash manifest, retention metadata, access audit, and legal hold at EDO-0.4+. A schema without WORM would look like an archive and would not be one. Selectel object lock remains `EXTERNAL_INFRA_VERIFICATION_REQUIRED`.

## Variant C — include MChD evidence rows

Same as A, plus `PowerOfAttorneyEvidence` rows (GUID, principal company, representative company, recorded timestamp) without calling a government registry.

Defer. Storing a GUID without a verification rule creates a false authority signal. Format and duty to check an MChD are `LEGAL_VERIFICATION_REQUIRED`. See [edo-0.3-regulatory-source-register.md](edo-0.3-regulatory-source-register.md).

## Variant D — split state dimensions now

Same as A, plus new columns or tables for business, delivery, signature, and operator state, and a compatibility mapping from the current `document_status`.

Defer. The target model is already frozen in [edo-0.2-document-state-machines.md](edo-0.2-document-state-machines.md). Doing the split inside EDO 0.3 changes every current document client and is not in the named deliverable list. A later wave can map the existing enum onto the four dimensions without blocking package and relationship work.

## Recommendation

```text
RECOMMENDED_VARIANT=A
ARCHIVE_MANIFEST_IN_EDO_0_3=NO
MCHD_VERIFICATION_IN_EDO_0_3=NO
STATE_SPLIT_IN_EDO_0_3=NO
OPERATOR_OR_ETRN_IN_EDO_0_3=NO
IMPLEMENTATION_AUTHORIZED=NO
```

Controller review may select B, C, or D explicitly. Silence does not select them. Until that review, implementation stays unauthorized.

## Consequences of variant A

- EDO 0.2 event names `edo.document.revision_added` and `edo.document.signature_state_changed` stay the future bus names. EDO 0.3 does not publish them.
- Package seal and relationship append need future event names. Those names are not added to the accepted catalog in this discovery.
- Billing `ClosingDocumentPackage.package_document_ids` in ADR-EDO-002 can point at `DocumentPackage.id` only after the package exists. Building that pointer is still the billing bridge, not variant A.
- Cascade delete on signed children is a defect relative to the immutability rule and is in the variant A rule set, not a reason to redesign storage.

## References

- [edo-0.3-scope.md](edo-0.3-scope.md)
- [edo-0.3-gap-analysis.md](edo-0.3-gap-analysis.md)
- [ADR-EDO-001](../adr/ADR-EDO-001-canonical-edo-document-ownership.md)
- [ADR-EDO-002](../adr/ADR-EDO-002-billing-edo-boundary.md)
- [edo-0.2-event-contracts.md](../events/edo-0.2-event-contracts.md)
