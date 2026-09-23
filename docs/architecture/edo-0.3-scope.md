# EDO 0.3 Scope

## Status

```text
DOCUMENT_STATUS=DISCOVERY
SCOPE_STATUS=ACCEPTED_VARIANT_A_DISCOVERY_ONLY
IMPLEMENTATION_AUTHORIZED=NO
PRODUCT_CODE_IN_THIS_CHANGE=NO
```

## Name and source

The accepted program name is:

```text
EDO-0.3 — Document domain extensions (document-service only)
```

Source: [edo-0.2-final-report.md](edo-0.2-final-report.md) section `NEXT_RECOMMENDED_PHASE`, and the EDO row of [workstream-status-v0.1.md](../program/workstream-status-v0.1.md).

Named deliverables in that section:

- `DocumentPackage`
- `DocumentRelationship`
- immutable revision rules
- signing evidence extensions
- additive migrations in schema `documents` only
- a separate EDO Task Contract before implementation

This discovery does not rename EDO 0.3 and does not start that Task Contract.

## Proposed in-scope outcome

After a future authorization, EDO 0.3 would let `document-service` represent:

1. A sealed package of documents owned by the assembling company, tenant-scoped. Membership of a document in a package is stored only on `DocumentPackage`. The same document appears at most once in a package. The package and the member document share one tenant. Cross-tenant membership is forbidden. That membership row is the only source of truth for composition.
2. Append-only `DocumentRelationship` rows for semantic links between documents, such as correction, replacement, or related document. A relationship does not say that a package contains a document. A relationship type `PACKAGE_CONTAINS_DOCUMENT` is forbidden because it would duplicate membership. UKD and cancellation remain new documents plus semantic relationships, as ADR-EDO-002 already requires. This phase does not implement UKD XML.
3. Revision immutability: a signed revision's payload and file bytes are not updated or replaced in place. A legally significant change creates a new revision. `AddFile` on a signed document is in the gap this phase is meant to close.
4. Signing evidence extensions in the `documents` schema. Current state: `documents.signatures.document_id` is implemented and revision binding is absent. This discovery does not invent `document_version_id`. The future contract binds the signature and certificate evidence to the immutable revision of the signed bytes, by `document_version_id` or an equivalent proof that names that revision and its content digest. Digest algorithm and digest value refer to that revision. Re-verification does not change historical evidence. A new revision does not inherit the previous signature. Certificate evidence has no private key. Evidence does not mean the signature is legally valid. Legal effect stays `LEGAL_VERIFICATION_REQUIRED`.

Ownership stays with `document-service` ([ADR-EDO-001](../adr/ADR-EDO-001-canonical-edo-document-ownership.md)). Other services keep referencing `document_id`.

## Proposed out of scope

| Item | Owner phase | Reason |
|------|-------------|--------|
| TEDO-0.3 ETRN lifecycle, titles, GIS EPD | TEDO | Different roadmap row |
| `transport-edo-service`, operator adapters, operator credentials | TEDO / INFRA | ADR-EDO-004, ADR-EDO-008 |
| Claiming EDI-operator or IS-EPD-operator accreditation | Legal | No official source in this repo names BINTRANS as an operator |
| Billing UPD bridge, mandatory `document_id`, mock `mark-sent-to-edo` removal | EDO-0.5 / FC | ADR-EDO-002; `CWS-EDO-2026-001` |
| Receivable, factoring, assignment, financing | FF | ADR-EDO-005 |
| `PaymentObligation` behavior | FF / payment-service | Separate rail |
| TransportJourney, TransportLeg, CargoHandover tables | LOG / MM | ADR-EDO-003, ADR-EDO-009 |
| `user_roles` dual-write removal | PLAT-0.1 | ADR-PLAT-001 |
| S3, WORM, Selectel object lock, bucket provisioning | INFRA-0.1 | ADR-EDO-007; not verified |
| `ArchiveManifest`, legal hold, retention enforcement | EDO-0.4+ unless the controller expands scope | See options |
| MChD registry calls and qualified-signature cryptography | Later, after legal verification | Not in the named sentence |
| Splitting `document_status` into four columns | Controller option, not the named sentence | State-machine freeze still stands as the target model |
| Kafka topics, JSON Schema files, Control Tower primary mode | Later EDO bus work | ADR-EDO-006; F-012 |
| Frontend screens and browser acceptance of a new UI | Not required to freeze the domain extension | No EDO 0.3 UI is specified |
| Product code, migrations, OpenAPI edits, CI workflow edits | This discovery task | Forbidden here |

## Boundary rules that stay in force

- One canonical shipment id. No `MultimodalShipment`. No `edo_shipments`, `edo_companies`, or `edo_users`.
- Billing does not gain a copy of XML or signature bytes.
- EDO agents do not edit LOG-owned or TMS-owned code under this scope.
- Current BINTRANS shipment flow stays unchanged until a reviewed implementation wave says otherwise.
- `EXTERNAL_LEGAL_VERIFICATION_REQUIRED` remains the default for statutory format, signature, MChD, and retention claims.

## Controller decisions required before any implementation wave

1. Confirm variant A in [edo-0.3-architecture-options.md](edo-0.3-architecture-options.md), or explicitly select B, C, or D.
2. Confirm that `ArchiveManifest` stays out of the first implementation wave.
3. Confirm that MChD verification stays out until a legal review.
4. Confirm tenant is taken from the gateway trust boundary on any new route.
5. Accept a separate product remediation of `DOCUMENT_READ_TENANT_ISOLATION_REMEDIATION_REQUIRED` before any EDO 0.3 implementation Task Contract. This discovery is not that authorization.
6. Authorize a Task Contract. This discovery is not that authorization.

## References

- [edo-0.3-discovery.md](edo-0.3-discovery.md)
- [edo-0.3-gap-analysis.md](edo-0.3-gap-analysis.md)
- [edo-0.3-implementation-waves.md](edo-0.3-implementation-waves.md)
- [ADR-EDO-009](../adr/ADR-EDO-009-cross-workstream-mutation-policy.md)
