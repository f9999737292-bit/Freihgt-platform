# EDO document completeness source contract 0.4B

## Status

```text
STAGE=SOURCE-CONTRACT-B-DOCUMENT-COMPLETENESS-0.4B
BASE_SHA=d7f865b3e64faf08cdd4fc824851b0b6d0d677b4
SOURCE_GAP=GAP-B-001
REQUEST_ID=AN-SRC-B-001
DOCUMENT_COMPLETENESS_SOURCE_CONTRACT_VERSION=v0.4b
IMPLEMENTATION_AUTHORIZED=NO
REQUIRED_DOCUMENT_POLICY_PRESENT=NO
ANALYTICS_DIRECT_DB_ACCESS=NO
FUTURE_ACCEPTANCE_TESTS=FUTURE_REQUIRED
```

This note freezes the source contract Analytics asked for in GAP-B-001. It does not change document-service, OpenAPI, migrations, gateway, S3, or staging. It does not change Analytics KPI formulas.

Analytics must receive completeness from document-service / EDO. Analytics must not derive it from a document count, from `SIGNED`, from Control Tower `awaitingDocuments`, or from a client rule.

## Inventory at this baseline

Migration head on this tree is `000096_edo_0_3_i4b_signature_verification_foundation`. No later document migration exists here.

The canonical document row is `documents.documents` (`000005`).

| Fact | Where it lives | What it proves |
| --- | --- | --- |
| Document id | `documents.documents.id` | A canonical id inside one tenant |
| Tenant | `documents.documents.tenant_id` | The row's tenant. Not a completeness policy |
| Type | `document_type` | One allowed class. Not that the class is required |
| Lifecycle | `document_status` | The row's state. Not completeness |
| Related entity | `related_entity_type`, `related_entity_id` | An optional link. Nullable |
| Owner company | `owner_company_id` | Ownership metadata. Not authority for a customer read, and not a required-set |
| Soft delete | `deleted_at` | The row is withdrawn from normal presence |
| Version counter | `documents.documents.version` | Optimistic concurrency. Not a successor document |
| Payload version | `documents.document_versions` | A payload generation of the same document |
| Legacy file row | `documents.document_files` | Older file metadata. Not the customer attachment path |
| Attachment | `documents.document_attachments` (`000095`) | `DRAFT` or `FINALIZED` bytes for one document |
| Detached signature blob | `documents.attachment_signature_blobs` (`000096`) | Stored signature bytes. Not a qualified result |
| Evidence | `documents.signature_verification_evidence` | Current qualified path allows `PENDING` / `VERIFIER_UNAVAILABLE` |
| Relationship edge | `documents.document_relationships` (`000087`) | `CORRECTS`, `REPLACES`, or `RELATED_TO`. Append-only and tenant-scoped. No completeness writer |

Allowed document types in the check constraint and in `allowedDocumentTypes`:

```text
ETRN
EPD
WAYBILL
POD
DISCREPANCY_ACT
CLAIM
INVOICE
VAT_INVOICE
ACT
UPD
ECMR
```

Allowed lifecycle statuses:

```text
DRAFT
READY_FOR_SIGNING
SIGNING_IN_PROGRESS
SIGNED
SENT_TO_OPERATOR
ACCEPTED
REJECTED
ARCHIVED
CANCELLED
```

Application related-entity values are `SHIPMENT`, `TRANSPORT_ORDER`, `BILLING_REGISTER`, `INVOICE`, and `COMPANY`. The column is nullable. A missing link does not mean the document is complete or incomplete.

Shipper and carrier read contracts use the same operational allowlist: `ETRN`, `EPD`, `WAYBILL`, `POD`, `DISCREPANCY_ACT`, `ECMR`. They hide `INVOICE`, `VAT_INVOICE`, `ACT`, `UPD`, and `CLAIM`. They also hide `DRAFT` and `CANCELLED`. Those lists are participant visibility. They are not a required set for a shipment.

`billing.upd_documents` is a billing table. It is not the EDO completeness source.

### What presence can prove today

Document-service can prove that a non-deleted row exists for a tenant, with a type, a status, and an optional related entity. A finalized attachment can prove bytes for that document. A signature row can prove that a blob was stored. None of these facts say the document was required.

### What is not a requirement policy

No server-owned required-document policy was found in:

- transport or shipment execution;
- customer or shipper configuration;
- a commercial contract;
- tenant policy;
- document-domain tables;
- operation type;
- Finance or accounting policy.

`execution_tracking.go` sets `RequiresPOD: false` on the delivery-completed milestone. That hardcoded false is not a POD requirement. Control Tower `CalculateKPI` increments `awaitingDocuments` when a shipment status is `DELIVERED` or `DELIVERY_CONFIRMED`. That counter does not read documents.

```text
REQUIRED_DOCUMENT_POLICY_PRESENT=NO
CONTROL_TOWER_AWAITING_DOCUMENTS_IS_CANONICAL_COMPLETENESS=NO
SIGNED_STATUS_IS_COMPLETENESS=NO
DOCUMENT_COUNT_IS_COMPLETENESS=NO
```

Because the policy is absent, a completeness evaluation on this baseline is `UNKNOWN`. It is not `COMPLETE`.

## Requirement ownership

Agent B owns the completeness evaluation and the canonical document ids that satisfy a requirement. Agent B does not invent the required set.

Future inputs stay with their domains:

| Input | Owner | This baseline |
| --- | --- | --- |
| Shipment execution requirement, including whether POD is required for a delivery | Agent C | Not a completeness policy. `RequiresPOD` false is not proof |
| Accounting or legal document requirement | Agent G / Finance | Finance 0.1C is research. It does not set an operational required set |
| Legal XML, signature container, EDO artifact, and canonical document id | Agent B | Presence can be read. Requirement cannot |
| Customer operational required set | Not present | Do not copy the shipper visibility allowlist into a requirement |

Finance 0.1C says the 2013 UPD letter recommends UPD and does not make it a mandatory blank. That research must not become automatic operational completeness. An accounting required document and a customer operational required document may differ. The completeness source composes those policies. It does not collapse them.

```text
DOCUMENT_REQUIREMENT_POLICY_OWNER=NONE
COMPLETENESS_EVALUATION_OWNER=AGENT_B
```

`NONE` means no authoritative policy exists. It does not assign the missing policy to Analytics.

## Completeness contract

```text
DOCUMENT_COMPLETENESS_SOURCE_CONTRACT_VERSION=v0.4b
```

| Field | Rule |
| --- | --- |
| `RELATED_ENTITY_TYPE` | `SHIPMENT` or `TRANSPORT_ORDER` for v0.4b. Other related-entity values are out of this subject |
| `RELATED_ENTITY_ID` | Server-known id of that entity |
| `REQUIRED_DOCUMENT_TYPES` | Server-owned evaluated set. Empty only when a real policy says nothing is required. Unknown is not empty |
| `PRESENT_DOCUMENT_IDS` | Canonical ids that are safe satisfying evidence. Inventory rows that are not safe evidence stay out of this list |
| `MISSING_DOCUMENT_TYPES` | Required types with no satisfying document |
| `COMPLETENESS_STATUS` | `COMPLETE`, `INCOMPLETE`, or `UNKNOWN` |
| `EVALUATED_AT` | Time of this evaluation. Not `documents.updated_at` and not an Analytics `generatedAt` |
| `TENANT_SCOPE` | Server-derived tenant |
| `POLICY_VERSION` | Required when the source policy is versioned. A claimed known policy with no version is `UNKNOWN` |
| `POLICY_INPUTS` | Identifiers of the domain policies that were composed, including an Agent C POD decision when POD is in the set |

```text
COMPLETENESS_STATUS=COMPLETE|INCOMPLETE|UNKNOWN
```

Rules:

- Unknown requirement policy, or a known type set without an acceptance state for each type: `UNKNOWN`.
- No `CORRECTS` or `REPLACES` ambiguity, known requirement policy, known acceptance state, and every required type has a satisfying canonical document: `COMPLETE`.
- No `CORRECTS` or `REPLACES` ambiguity, known policy, and at least one required type has no satisfying document: `INCOMPLETE`.
- A required type touched by unresolved `CORRECTS` or `REPLACES` ambiguity: `UNKNOWN` for that evaluation. That is not `INCOMPLETE`.
- `SIGNED` alone is not completeness.
- A count of rows is not completeness.
- An extra document does not fill a different missing type.
- A document linked to another entity does not satisfy this entity.
- Analytics does not read these tables directly.

`EVALUATED_AT` is the clock of the evaluation. Repeating the evaluation may move it. It does not rewrite document history.

## Satisfaction

Presence of a row is not satisfaction. Signature is not a universal gate. The acceptance state is per document type and comes from the policy that named the type.

A document can satisfy a required type only when all of these hold:

- `tenant_id` is the server tenant of the evaluation;
- `id` is that tenant's canonical document id;
- `document_type` equals the required type;
- `related_entity_type` and `related_entity_id` are the evaluated subject;
- `deleted_at` is null;
- `document_status` is in the acceptance set of that type's policy.

Always excluded, with no policy override in v0.4b:

- `CANCELLED` does not satisfy;
- a soft-deleted row does not satisfy;
- a document from another tenant does not satisfy.

`DRAFT` and `REJECTED` do not satisfy unless a future named policy explicitly includes that status for that type. This contract does not include them.

`READY_FOR_SIGNING`, `SIGNING_IN_PROGRESS`, `SIGNED`, `SENT_TO_OPERATOR`, `ACCEPTED`, and `ARCHIVED` are eligible only when the type policy lists that status. If the policy names the type and omits the acceptance set, the evaluation is `UNKNOWN`.

A finalized attachment, a signature blob, or `SIGNED` is an extra gate only when that type policy says so. Qualified verification remains independently blocked. A policy that demands a qualified `VALID` cannot be evaluated as `COMPLETE` while that verifier is unavailable. That evaluation stays `UNKNOWN`. It does not become `INCOMPLETE` merely because the verifier is absent, and it does not become `COMPLETE` because a legacy signing session stored `SIGNED`.

```text
SIGNATURE_REQUIRED_FOR_EVERY_TYPE=NO
ATTACHMENT_FINALIZED_REQUIRED_FOR_EVERY_TYPE=NO
SIGNED_STATUS_IS_COMPLETENESS=NO
```

### Normal evaluation

When the required type has no `CORRECTS` or `REPLACES` ambiguity, evaluation proceeds. It still needs a known requirement policy, a known acceptance state for that type, the server tenant, the evaluated related entity, a matching type, a row that is not soft-deleted, and a status in that type's acceptance set. Then the result may be `COMPLETE` or `INCOMPLETE`.

Several live documents of the same required type, with no `CORRECTS` or `REPLACES` edge and no other ambiguity, satisfy that type when at least one of them meets the acceptance state. They do not satisfy a different missing type. This rule stops where successor ambiguity starts.

```text
MULTIPLE_SAME_TYPE_NO_SUCCESSOR_NORMAL_EVALUATION=YES
```

### Successor

`documents.document_relationships` can store `CORRECTS`, `REPLACES`, and `RELATED_TO`. The table is append-only and tenant-scoped. Document-service does not currently use those edges to decide completeness. The schema does not state which endpoint is the current document.

```text
SUCCESSOR_SATISFACTION=NOT_EVALUABLE
SUCCESSOR_CURRENT_DOCUMENT_POLICY_PRESENT=NO
AMBIGUOUS_SUCCESSOR_COMPLETENESS=UNKNOWN
SATISFIED_DOCUMENT_ID_WHILE_SUCCESSOR_AMBIGUOUS=NOT_SELECTED
AMBIGUOUS_SUCCESSOR_PRESENT_AS_SATISFYING=NO
RELATED_TO_CHANGES_SATISFACTION=NO
```

If documents of a required type are joined by `CORRECTS` or `REPLACES`, and the evaluation cannot prove edge direction, the authoritative successor, the current canonical document, predecessor validity, and cardinality, the evaluation fails closed:

- do not select the predecessor as satisfying;
- do not select the other endpoint as satisfying;
- do not return `COMPLETE`;
- do not return `INCOMPLETE` only because one side cannot be used;
- return `UNKNOWN` for the affected required-document evaluation.

`PRESENT_DOCUMENT_IDS` lists only ids that are safe satisfying evidence. An ambiguous predecessor or successor may remain in internal inventory. It is not emitted as satisfying evidence.

`RELATED_TO` is not substitution. That edge alone does not make completeness `UNKNOWN`, does not satisfy another type, does not replace predecessor or successor semantics, and does not change the required type.

A later contract has to freeze these fields before runtime can choose a current document. This stage does not assign their values:

```text
SUCCESSOR_EDGE_DIRECTION
SUCCESSOR_CARDINALITY
CURRENT_DOCUMENT_SELECTION
PREDECESSOR_VALIDITY_AFTER_CORRECTS
PREDECESSOR_VALIDITY_AFTER_REPLACES
SUCCESSOR_ACCEPTANCE_STATE_RULE
```

## POD boundary

GAP-C-002 stays with Agent C for the requirement. Agent B answers only whether a canonical document satisfies a type Agent C named.

```text
POD_REQUIREMENT_OWNER=AGENT_C
POD_DOCUMENT_EVIDENCE_OWNER=AGENT_B
```

Document-service does not copy POD requirement rules. A future lookup, not built in this stage, is:

| Field | Owner |
| --- | --- |
| `DELIVERY_IDENTITY` | Agent C |
| `SHIPMENT_ID` | Agent C subject; document link is Agent B |
| `REQUIRED_DOCUMENT_TYPE` | Agent C when POD is required. The type value must be a canonical document type |
| `SATISFIED_DOCUMENT_ID` | Agent B, or null |
| `SATISFIED_AT` | Agent B evaluation time when satisfied, otherwise null |

If Agent C has not said whether POD is required, document-service does not treat a POD row as proof that POD was required, and it does not treat a missing POD row as `INCOMPLETE`.

If the POD document itself has unresolved `CORRECTS` or `REPLACES` ambiguity:

```text
POD_SATISFACTION_STATUS=UNKNOWN
SATISFIED_DOCUMENT_ID=null
```

Neither ambiguous POD id is satisfying evidence.

## Security

```text
TENANT_SERVER_DERIVED=YES
CLIENT_SUPPLIED_TENANT_AUTHORITY=NO
CROSS_TENANT_DOCUMENT_READ=DENY
```

The tenant comes from the trusted gateway or internal identity. A caller tenant, body tenant, or query tenant is not authority. Present ids are canonical ids in that tenant. A foreign document id is not returned and cannot satisfy the requirement. Missing and foreign both fail closed without disclosing the other tenant's row. A relationship edge that crosses tenants does not change completeness and does not reveal the foreign id.

## Future acceptance tests

These tests are `FUTURE_REQUIRED`. This stage does not add them.

1. A known required set with a satisfying document for every type evaluates to `COMPLETE`.
2. A known required set with one type missing evaluates to `INCOMPLETE`.
3. An unknown requirement policy evaluates to `UNKNOWN`.
4. `SIGNED` alone, with no known required set, does not evaluate to `COMPLETE`.
5. An extra document of another type does not fill a missing required type.
6. A `CANCELLED` document does not satisfy its type.
7. `SUCCESSOR-01`. Known policy, a satisfying document, and no successor edge evaluate to `COMPLETE`.
8. `SUCCESSOR-02`. A `CORRECTS` edge whose current document cannot be determined evaluates to `UNKNOWN`.
9. `SUCCESSOR-03`. A `REPLACES` edge whose current document cannot be determined evaluates to `UNKNOWN`.
10. `SUCCESSOR-04`. Ambiguous predecessor and successor ids are not returned as satisfying evidence.
11. `SUCCESSOR-05`. `RELATED_TO` does not substitute a missing required type.
12. `SUCCESSOR-06`. Two documents of the same required type, with no successor ambiguity, may satisfy that type when one meets the policy.
13. `SUCCESSOR-07`. A cross-tenant relationship edge is denied, does not change completeness, and does not leak ids.
14. `SUCCESSOR-08`. POD successor ambiguity returns POD satisfaction `UNKNOWN` and no satisfying document id.
15. A document from another tenant does not satisfy the requirement, and the read does not leak it.
16. When Agent C requires POD and there is no successor ambiguity, satisfaction returns the canonical document id of the satisfying POD, or null when none satisfies.
17. A client-supplied tenant does not select the document population.

## Non-goals

No code, migration, OpenAPI change, gateway change, S3 change, credential, or staging change belongs to this stage. Durable object storage stays on its own contract. Write gates stay blocked. KPI publication of `OPS_DOCUMENT_COMPLETENESS`, `CAR_DOCUMENT_COMPLETENESS`, and `DOC_DOCUMENT_COMPLETENESS` stays blocked until a later authorized implementation can evaluate this contract.
