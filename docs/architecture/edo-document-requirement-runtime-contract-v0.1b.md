# EDO document requirement runtime contract 0.1B

## Status

```text
STAGE=EDO-DOCUMENT-REQUIREMENT-POLICY-0.1B-RUNTIME-TASK-CONTRACT
BASE_SHA=109ec8117b75cb8205af7667eef6d52b30a11bc6
KIND=DOCS_ONLY_ARCHITECTURE
PRODUCT_CODE_AUTHORIZED=NO
MIGRATION_AUTHORIZED=NO
OPENAPI_AUTHORIZED=NO
GATEWAY_AUTHORIZED=NO
STAGING_AUTHORIZED=NO
GAP_B_001_RUNTIME_IMPLEMENTATION_AUTHORIZED=NO
OWNER_CONFLICT=NO
DOCUMENT_COMPLETENESS_RUNTIME_OWNER=document-service
REQUIREMENT_COMPOSITION_OWNER=document-service
TMS_DECISION_AUTHORITY=shipment-service/TMS
FINANCE_DECISION_AUTHORITY=Finance domain
FINANCE_POLICY_DUPLICATED_IN_EDO=NO
POD_POLICY_DUPLICATED_IN_EDO=NO
CROSS_DOMAIN_WRITE_REQUIRED=NO
SELECTED_RUNTIME_ARCHITECTURE=C_HYBRID_IMMUTABLE_SNAPSHOT
PERSISTENCE_REQUIRED=YES
MIGRATION_REQUIRED_FUTURE=YES
MIGRATION_NUMBER=UNASSIGNED
SUCCESSOR_CURRENT_DOCUMENT_POLICY_FROZEN=NO
POLICY_VERSION_REQUIRED=YES
EFFECTIVE_DATE_REQUIRED=YES
HISTORICAL_POLICY_REPLAY=VERSION_PINNED
TENANT_SERVER_DERIVED=YES
CLIENT_TENANT_AUTHORITY=NO
CROSS_TENANT_POLICY_READ=DENY
CROSS_TENANT_DOCUMENT_READ=DENY
FOREIGN_ID_DISCLOSURE=NO
UNKNOWN_FAIL_CLOSED=YES
SILENT_NEWEST_WINS=NO
```

This note turns the frozen requirement policy 0.1A into an implementation-ready Task Contract. It does not implement the runtime. It does not add Go, a migration, OpenAPI, a gateway route, or a staging change.

Authoritative inputs:

- [edo-document-completeness-source-v0.4b.md](edo-document-completeness-source-v0.4b.md)
- [edo-document-requirement-policy-v0.1a.md](edo-document-requirement-policy-v0.1a.md)
- [tms/TMS_ANALYTICS_SOURCE_CONTRACTS_0_4B.md](tms/TMS_ANALYTICS_SOURCE_CONTRACTS_0_4B.md)
- [finance-accounting/FINANCE_0_1D_ACCOUNTING_DOCUMENT_RULES.md](finance-accounting/FINANCE_0_1D_ACCOUNTING_DOCUMENT_RULES.md)

`DOCUMENTED_FUTURE_CONTRACT` below is a freeze that has no runtime writer. `ACTUAL_RUNTIME_SOURCE` is a table, type, or query that exists in this tree at `109ec811`.

## 1. Ownership

No repository source contradicts document-service as the owner of completeness composition.

| Responsibility | Owner | Evidence |
| --- | --- | --- |
| Canonical document type vocabulary | Agent B / document-service | `allowedDocumentTypes` and `documents.documents` check constraint |
| Document evidence, attachments, signature blobs | Agent B / document-service | `documents` schema, migrations `000005`, `000087`, `000095`, `000096` |
| Current and successor document semantics | Agent B | Edges exist. The selection policy does not |
| Completeness evaluation | Agent B / document-service | Completeness 0.4B. No evaluator package exists yet |
| Requirement-policy composition envelope | Agent B / document-service | Policy 0.1A. No registry table exists yet |
| POD and other execution requirement facts | Agent C / shipment-service / TMS | TMS Analytics source contracts 0.4B |
| Accounting document requirement decisions | Agent G / Finance | Finance 0.1D |

```text
DOCUMENT_COMPLETENESS_RUNTIME_OWNER=document-service
REQUIREMENT_COMPOSITION_OWNER=document-service
TMS_DECISION_AUTHORITY=shipment-service/TMS
FINANCE_DECISION_AUTHORITY=Finance domain
OWNER_CONFLICT=NO
```

document-service consumes those decisions. It does not become the authority that decides `POD_REQUIRED`, VAT class, issuer, recipient, or legal applicability.

```text
FINANCE_POLICY_DUPLICATED_IN_EDO=NO
POD_POLICY_DUPLICATED_IN_EDO=NO
CROSS_DOMAIN_WRITE_REQUIRED=NO
```

## 2. Runtime source inventory

Inspected at base `109ec811`. A documented future contract is not marked present.

| Fact | Classification | Where |
| --- | --- | --- |
| Canonical document row | `ACTUAL_RUNTIME_SOURCE` | `documents.documents` (`000005`). Repository reads filter `tenant_id` and `deleted_at IS NULL` |
| Document type vocabulary | `ACTUAL_RUNTIME_SOURCE` | `ETRN`, `EPD`, `WAYBILL`, `POD`, `DISCREPANCY_ACT`, `CLAIM`, `INVOICE`, `VAT_INVOICE`, `ACT`, `UPD`, `ECMR` |
| Lifecycle status | `ACTUAL_RUNTIME_SOURCE` | `DRAFT` through `CANCELLED` as frozen in 0.4B |
| Related entity | `ACTUAL_RUNTIME_SOURCE` | Nullable. Application values `SHIPMENT`, `TRANSPORT_ORDER`, `BILLING_REGISTER`, `INVOICE`, `COMPANY` |
| Attachment bytes metadata | `ACTUAL_RUNTIME_SOURCE` | `documents.document_attachments`. Status `DRAFT` or `FINALIZED` |
| Relationship edge | `ACTUAL_RUNTIME_SOURCE` | `documents.document_relationships`. Types `CORRECTS`, `REPLACES`, `RELATED_TO`. Append-only. Schema tests only. No completeness writer |
| Signature blob and I4B evidence | `ACTUAL_RUNTIME_SOURCE` | `documents.attachment_signature_blobs`, `documents.signature_verification_evidence`. I4B check allows only `PENDING` / `VERIFIER_UNAVAILABLE`. Older `documents.signatures` and `documents.signature_verification_records` also exist. Neither path is a completeness evaluator |
| TMS POD requirement record | `DOCUMENTED_FUTURE_CONTRACT` | TMS 0.4B: `POD_REQUIREMENT_SOURCE=NONE` |
| TMS internal requirement read | `ABSENT` | No `GetTMSDocumentRequirementDecision`. `ListShipmentPODDocuments` reads `documents.documents` rows of type `POD`. That is a document list, not a requirement decision |
| `RequiresPOD: false` | `ACTUAL_RUNTIME_SOURCE` and not authority | `AllowedDriverMilestoneActions` in `execution_tracking.go`. `ShipmentExecutionView.PODRequired` is a Go field with no assignment on the execution read path |
| Finance policy decision | `DOCUMENTED_FUTURE_CONTRACT` | Finance 0.1D defines `ACCOUNTING_DOCUMENT_POLICY_DECISION` and sets `ACCOUNTING_DOCUMENT_POLICY_DECISION_IMPLEMENTED=NO` |
| Finance internal decision read | `ABSENT` | No Go type, table, or HTTP handler for that decision |
| Successor current-document policy | `DOCUMENTED_FUTURE_CONTRACT` | Named fields in 0.4B and 0.1A. No runtime values |
| Completeness evaluator | `ABSENT` | No `EvaluateDocumentCompleteness` in document-service |

```text
DOCUMENT_EVIDENCE_RUNTIME_PRESENT=YES
DOCUMENT_RELATIONSHIP_RUNTIME_PRESENT=YES
SIGNATURE_EVIDENCE_RUNTIME_PRESENT=YES
TMS_POD_REQUIREMENT_RUNTIME_PRESENT=NO
TMS_POD_REQUIREMENT_INTERNAL_READ_PRESENT=NO
FINANCE_POLICY_DECISION_RUNTIME_PRESENT=NO
FINANCE_POLICY_DECISION_INTERNAL_READ_PRESENT=NO
SUCCESSOR_CURRENT_DOCUMENT_POLICY_RUNTIME_PRESENT=NO
SUCCESSOR_CURRENT_DOCUMENT_POLICY_FROZEN=NO
```

Migration head on this tree is `000097_nlo_chain_search_persistence_v0_9c_i2a`. No later document migration adds a requirement or completeness table. `000041_execution_tracking_pod_v1.6` adds Control Tower projection timestamps. It is not a POD requirement source.

Preserved blockers, still true on this base:

```text
POD_REQUIREMENT_SOURCE=NONE
POD_REQUIRED=UNKNOWN
ACCOUNTING_DOCUMENT_POLICY_DECISION_IMPLEMENTED=NO
HISTORICAL_REPLAY_FOR_TMS=BLOCKED
HISTORICAL_REPLAY_FOR_FINANCE=BLOCKED
```

`UNKNOWN` is not `OPTIONAL`, not `NOT_APPLICABLE`, not false, and not an empty required set.

## 3. Normalized requirement-decision envelope

document-service consumes one normalized decision per owner-domain row. This is a conceptual contract. It is not a Go type in this stage.

| Field | Rule |
| --- | --- |
| `tenant_id` | Server-derived operating tenant. Client tenant is ignored |
| `subject_type` | Subject class the owner decision names |
| `subject_id` | Server id of that subject. Opaque to document-service |
| `business_context` | Owner event or context. Same role as 0.1A `EVENT_TYPE` / `BUSINESS_CONTEXT` |
| `business_event_time` | Time of the business fact the decision applies to. Not processing time |
| `policy_owner_domain` | `TMS_EXECUTION` or `FINANCE_ACCOUNTING`. Another domain only when a later contract names it |
| `source_decision_id` | Owner id when the owner supplies one. Absent id blocks historical replay |
| `source_decision_version` | Immutable owner version. Absent version blocks historical replay |
| `requirement_decided_at` | When the owner recorded the decision |
| `effective_from` | Inclusive applicability start |
| `effective_to` | Nullable exclusive end |
| `document_type` | Canonical document-service type, or an owner class that must be mapped |
| `requirement_state` | `REQUIRED`, `OPTIONAL`, `NOT_APPLICABLE`, or `UNKNOWN` |
| `acceptance_statuses` | Statuses that may satisfy this type. Omitted set with `REQUIRED` yields completeness `UNKNOWN` |
| `source_rule_id` | Owner rule id. document-service stores it. It does not execute it |
| `decision_state` | Owner provenance: `KNOWN`, `UNKNOWN`, or `NEEDS_REVIEW` |
| `captured_at` | When document-service stored this immutable snapshot |

```text
REQUIREMENT_STATE=REQUIRED|OPTIONAL|NOT_APPLICABLE|UNKNOWN
UNKNOWN_TO_OPTIONAL=NO
UNKNOWN_TO_NOT_APPLICABLE=NO
UNKNOWN_TO_FALSE=NO
UNKNOWN_TO_EMPTY_REQUIRED_SET=NO
```

An empty `REQUIRED` list is valid only when an authoritative `KNOWN` decision explicitly proves that nothing is required. Finance 0.1D uses an empty `required_document_types` when the decision is not known. That empty list is `UNKNOWN`, not a proven empty set. A `KNOWN` decision whose required list is empty and that does not carry an explicit nothing-required proof is `CONFLICTING_POLICY`.

Three clocks stay distinct:

| Clock | Field | Must not be replaced by |
| --- | --- | --- |
| Business event time | `business_event_time` | Processing time, `evaluated_at`, or `documents.updated_at` |
| Requirement decision time | `requirement_decided_at` | Business event time or evaluation time |
| Evaluation time | `evaluated_at` | Either source clock |

`effective_from` / `effective_to` select the version for `business_event_time`. A later version does not rewrite a pinned evaluation.

If the owner cannot supply a stable `source_decision_id` and `source_decision_version`:

```text
HISTORICAL_REPLAY_FOR_SOURCE=BLOCKED
COMPLETENESS_STATUS=UNKNOWN
REASON=HISTORICAL_REPLAY_BLOCKED
```

That result fails closed. It is not `INCOMPLETE`.

## 4. TMS input

Future POD input, owned by Agent C, one decision per delivery identity:

| Field | Rule |
| --- | --- |
| `operating_tenant_id` | Server tenant. Maps to envelope `tenant_id` |
| Delivery identity | `subject_type=DELIVERY`, `subject_id` from TMS |
| `shipment_id` | TMS subject link. A document satisfies the delivery only when its related entity is the subject the decision names |
| `pod_required` | `true`, `false`, or `UNKNOWN` |
| `required_document_type` | Required when `pod_required=true`. Must be a canonical document type. Absent or non-canonical type yields `UNKNOWN` |
| `requirement_decided_at` | TMS decision time |
| `source_decision_id` | Required for replay |
| `source_decision_version` | Required for replay |
| `business_event_time` | Delivery fact time the decision applies to |

Mapping into the envelope:

| TMS fact | Envelope |
| --- | --- |
| `pod_required=true` and a canonical type | `requirement_state=REQUIRED` for that type |
| `pod_required=false` from a server-owned decision | `requirement_state=NOT_APPLICABLE` for the POD slot. This false does not exist today |
| `pod_required=UNKNOWN` | `requirement_state=UNKNOWN`. Satisfaction stays `UNKNOWN` |

```text
POD_REQUIRED=true + satisfying canonical evidence => satisfied
POD_REQUIRED=true + known absent satisfying evidence => unsatisfied
POD_REQUIRED=UNKNOWN => satisfaction UNKNOWN
```

A POD document row does not prove that POD was required. A missing POD document does not prove that POD was optional. `RequiresPOD: false` on the driver milestone is not authority. `ListShipmentPODDocuments` is not authority.

Until Agent C publishes the source:

```text
POD_REQUIRED=UNKNOWN
POD_RUNTIME_IMPLEMENTABLE_NOW=NO
HISTORICAL_REPLAY_FOR_TMS=BLOCKED
```

TMS 0.4B already records `HISTORICAL_REPLAY_AVAILABLE=NO` for this gap. document-service must not reconstruct a historical POD requirement from current shipment status or from current document rows.

## 5. Finance input

document-service consumes only a server-owned `ACCOUNTING_DOCUMENT_POLICY_DECISION`. It does not recompute VAT class, VAT rate, VAT exemption, issuer, recipient, or legal-rule applicability.

| Finance `decision_state` | EDO requirement |
| --- | --- |
| `KNOWN` | Map the authoritative required and optional document types into envelope rows |
| `UNKNOWN` | `requirement_state=UNKNOWN` for that accounting subject |
| `NEEDS_REVIEW` | `requirement_state=UNKNOWN` for that accounting subject |

Issuer, recipient, `vat_treatment`, and `vat_rate_if_known` stay on the Finance decision. They are not requirement states and they are not copied into a second tax policy.

Mapping is allowed only onto a type that already exists in `allowedDocumentTypes`. An unmapped class is a mapping gap. It is `UNKNOWN` with `UNSUPPORTED_DOCUMENT_TYPE_MAPPING`. It is not dropped and it is not given a new type.

| Finance class in 0.1D | Canonical type | Mapping |
| --- | --- | --- |
| VAT invoice, when the decision names that class and not a UPD substitute | `VAT_INVOICE` | Aligned by name with the existing type. document-service still does not decide when a VAT invoice is required |
| Act, when the decision explicitly names an act | `ACT` | Aligned by name only for that explicit token |
| UPD, when the decision explicitly names UPD | `UPD` | The type only. Status 1 and status 2 are not separate canonical types |
| Primary document of unspecified form | none | Mapping gap. Do not use `ACT` |
| Payment invoice | none | Mapping gap. Do not use `INVOICE` |
| UKD | none | Mapping gap |
| Correction VAT invoice | none | Mapping gap. Do not reuse `VAT_INVOICE` |
| Consent or notification document | none | Mapping gap |

UPD status 1 may, under Finance `R-UPD1`, carry both a VAT invoice and a primary document. That substitution is a Finance decision. document-service applies it only when the `KNOWN` decision lists the canonical types that one `UPD` row is allowed to satisfy. document-service does not infer the substitution from the letter.

```text
FINANCE_POLICY_DECISION_RUNTIME_PRESENT=NO
FINANCE_RUNTIME_INTEGRATION_IMPLEMENTABLE_NOW=NO
HISTORICAL_REPLAY_FOR_FINANCE=BLOCKED
FINANCE_POLICY_DUPLICATED_IN_EDO=NO
```

An accounting required set and an operational required set stay separate. The completeness result names both policy inputs. It does not merge them silently.

## 6. Canonical evidence

Agent B may use these runtime fields as evidence inputs. Presence is not satisfaction.

| Field | Use |
| --- | --- |
| `documents.documents.id` | Canonical id |
| `tenant_id` | Must equal the server tenant of the evaluation |
| `document_type` | Must equal the required canonical type |
| `document_status` | Satisfies only when listed in that decision's `acceptance_statuses` |
| `related_entity_type`, `related_entity_id` | Must be the evaluated subject |
| `deleted_at` | Non-null rows do not satisfy |
| `documents.document_attachments.status` | Extra gate only when the decision requires `FINALIZED` |
| Signature blob and `signature_verification_evidence` | Extra gate only when the decision requires a signature result |
| `documents.document_relationships` | Successor inventory. Not a satisfaction rule until the successor policy is frozen |
| `documents.documents.version` | Optimistic concurrency. Not a successor document |
| `documents.document_versions` | Payload generation of the same document. Not a successor document |

Always excluded in this contract, with no policy override:

- `CANCELLED` does not satisfy;
- a soft-deleted row does not satisfy;
- a document from another tenant does not satisfy;
- `DRAFT` and `REJECTED` do not satisfy unless a later named policy explicitly includes that status for that type. This contract does not include them.

`READY_FOR_SIGNING`, `SIGNING_IN_PROGRESS`, `SIGNED`, `SENT_TO_OPERATOR`, `ACCEPTED`, and `ARCHIVED` are eligible only when the decision lists that status. A named type with no acceptance set evaluates to `UNKNOWN`.

```text
SIGNED_STATUS_IS_COMPLETENESS=NO
DOCUMENT_COUNT_IS_COMPLETENESS=NO
ATTACHMENT_EXISTS_IS_COMPLETENESS=NO
SIGNATURE_REQUIRED_FOR_EVERY_TYPE=NO
ATTACHMENT_FINALIZED_REQUIRED_FOR_EVERY_TYPE=NO
```

Qualified `VALID` is not an I4B runtime result. A decision that requires qualified `VALID` stays `UNKNOWN` with `DEPENDENCY_UNAVAILABLE` while the verifier cannot produce that result. That is not `INCOMPLETE` and not `COMPLETE`.

Completeness is:

```text
KNOWN REQUIRED SET
+ KNOWN CURRENT CANONICAL DOCUMENT EVIDENCE
= COMPLETE | INCOMPLETE | UNKNOWN
```

`OPTIONAL` does not block. `NOT_APPLICABLE` is not required. `UNKNOWN` is not optional.

Several live documents of the same required type, with no `CORRECTS` or `REPLACES` ambiguity, satisfy that type when at least one meets the acceptance set. They do not satisfy a different type.

## 7. Successor semantics

These fields remain unresolved. This stage does not assign them:

```text
SUCCESSOR_EDGE_DIRECTION
SUCCESSOR_CARDINALITY
CURRENT_DOCUMENT_SELECTION
PREDECESSOR_VALIDITY_AFTER_CORRECTS
PREDECESSOR_VALIDITY_AFTER_REPLACES
SUCCESSOR_ACCEPTANCE_STATE_RULE
SUCCESSOR_CURRENT_DOCUMENT_POLICY_FROZEN=NO
SUCCESSOR_SATISFACTION=NOT_EVALUABLE
```

If a required type is touched by `CORRECTS` or `REPLACES` and those fields are unproven:

```text
COMPLETENESS_STATUS=UNKNOWN
REASON=UNRESOLVED_SUCCESSOR
SATISFIED_DOCUMENT_ID=NOT_SELECTED
```

Do not choose the latest document. Do not choose the highest `version`. Do not treat `RELATED_TO` as replacement or substitution. `RELATED_TO` alone does not make the evaluation `UNKNOWN`.

## 8. Conflict and precedence

Same subject, same `business_context`, same canonical `document_type`, two applicable decisions, and no proven precedence:

```text
REQUIREMENT_STATE=UNKNOWN
COMPLETENESS_STATUS=UNKNOWN
REASON=CONFLICTING_POLICY
SILENT_NEWEST_WINS=NO
```

Precedence, when proven by a server-owned grant:

1. Domain-specific authoritative decision for that subject class. `TMS_EXECUTION` for the POD slot. `FINANCE_ACCOUNTING` for the accounting subject.
2. Tenant override, only when that same server-owned policy source grants the override.
3. Platform default, only when an explicit server-owned default exists.

A client-supplied tenant, policy id, policy version, or source decision id is not authority. A tenant override that the server-owned source did not grant is ignored.

TMS and Finance decisions about different subjects are not a conflict. They are separate inputs.

## 9. Tenancy

```text
TENANT_SERVER_DERIVED=YES
CLIENT_TENANT_AUTHORITY=NO
CROSS_TENANT_POLICY_READ=DENY
CROSS_TENANT_DOCUMENT_READ=DENY
FOREIGN_ID_DISCLOSURE=NO
```

The tenant comes from trusted gateway or internal service identity. Missing and foreign source or document identities fail the same way. The response does not say which of the two occurred. A cross-tenant relationship edge does not change completeness and does not reveal the foreign id.

## 10. Architecture choice

| Option | What it does | Why it is not selected, or why it is |
| --- | --- | --- |
| A. Live calls | document-service calls TMS and Finance on every evaluation and does not store the decision | Rejected. TMS historical replay is already `NO`. Finance has no versioned runtime. A later policy change would be what the next call returns. Dependency outage would be indistinguishable from a missing document unless a separate cache existed. Live calls also couple completeness availability to both owners |
| B. Local copy of the business decision | document-service persists the decision and evaluates only locally, including owner rule text | Rejected as stated. Copying VAT, issuer, and POD rules would duplicate authority. A mutable local copy can drift from the owner |
| C. Hybrid | Owners remain the only writers of their decisions. document-service stores an immutable, version-pinned, normalized snapshot and provenance, then evaluates evidence locally against that snapshot | Selected |

```text
SELECTED_RUNTIME_ARCHITECTURE=C_HYBRID_IMMUTABLE_SNAPSHOT
```

Comparison for C:

| Concern | Result |
| --- | --- |
| Authority duplication | Low, if the snapshot stores the owner result and not the owner rule. `FINANCE_POLICY_DUPLICATED_IN_EDO=NO`. `POD_POLICY_DUPLICATED_IN_EDO=NO` |
| Cross-domain coupling | Read-only, at capture time. Evaluation does not call the owner again |
| Dependency availability | A missing snapshot plus an unreachable owner is `DEPENDENCY_UNAVAILABLE`, not `INCOMPLETE`. A stored snapshot can still be evaluated |
| Historical replay | The pin is `policy_owner_domain + source_decision_id + source_decision_version + business_event_time` |
| Auditability | Snapshot row plus append-only evaluation row |
| Tenant isolation | `tenant_id` on every snapshot and evidence read. No cross-service foreign key |
| Idempotency | Re-capture of the same owner version is the same row |
| Schema cost | One snapshot table and one append-only evaluation table in `documents`. No owner-domain table |
| Failure mode | Owner outage blocks new pins. It does not rewrite old pins and it does not mark the subject incomplete |

Repository support for C:

- Completeness 0.4B and policy 0.1A already require version-pinned replay and forbid silent rewrite.
- Production data ownership is per service. document-service must not read TMS or Finance tables. Shipment-service already reads `documents.documents` for POD file lists. That cross-schema read is not the pattern this contract extends.
- Internal service HTTP already uses `X-Internal-Service-Token` and `X-Internal-Service-Name` (`analytics-service`, `contract-rate-service`). Capture uses that style of internal read when the owner endpoint exists.
- This contract does not add Kafka or another broker.

Capture is not a browser API. There is no public policy mutation API in 0.1B.

## 11. Persistence design

Required by architecture C. Not created in this stage.

```text
PERSISTENCE_REQUIRED=YES
MIGRATION_REQUIRED_FUTURE=YES
MIGRATION_NUMBER=UNASSIGNED
SCHEMA_OWNER=documents
TABLE_OWNER=document-service
```

`documents.document_requirement_snapshots` is the conceptual snapshot. One row is one document type inside one owner decision version.

| Column | Rule |
| --- | --- |
| `id` | Stable UUID primary key |
| `tenant_id` | Server tenant. Required |
| `policy_owner_domain` | `TMS_EXECUTION` or `FINANCE_ACCOUNTING` |
| `source_decision_id` | Owner id. Required for a replayable row |
| `source_decision_version` | Owner version. Required for a replayable row |
| `subject_type`, `subject_id` | Owner subject |
| `business_context` | Owner event |
| `business_event_time` | Applicability clock |
| `requirement_decided_at` | Owner decision clock |
| `effective_from`, `effective_to` | Owner applicability window |
| `document_type` | Canonical type after mapping. Unmapped classes are not inserted as invented types. They are recorded as a gap row with `requirement_state=UNKNOWN` and `reason_code=UNSUPPORTED_DOCUMENT_TYPE_MAPPING` |
| `requirement_state` | The four-state set |
| `acceptance_statuses` | Nullable. Null plus `REQUIRED` cannot become `COMPLETE` or `INCOMPLETE` |
| `source_rule_id` | Provenance only |
| `decision_state` | `KNOWN`, `UNKNOWN`, or `NEEDS_REVIEW` |
| `provenance` | Owner payload hash and capture metadata. Not a second rule engine |
| `captured_at` | Insert time |

Immutability: no update of decision columns. A new owner version is a new row. `captured_at` is not an applicability clock.

Uniqueness: (`tenant_id`, `policy_owner_domain`, `source_decision_id`, `source_decision_version`, `subject_type`, `subject_id`, `business_context`, `document_type`).

Foreign keys: to `documents.documents` only if a later evaluation table cites a satisfying document id in the same tenant. No foreign key to shipment, billing, or finance tables. `subject_id` stays an opaque UUID.

Indexes: tenant plus subject plus business context plus document type; tenant plus source decision id plus version.

`documents.document_completeness_evaluations` is append-only. It stores `tenant_id`, subject, business context, `completeness_status`, reason codes, the snapshot ids and versions used, satisfying document ids that are safe to emit, `business_event_time`, and `evaluated_at`. A later evaluation inserts a new row. It does not update the earlier row.

Retention: snapshots and evaluation rows stay for replay. Deleting them because a current policy changed is a rewrite and is forbidden. A later retention contract may archive bytes. It may not drop the pin.

`000098` is not reserved.

## 12. Internal interfaces

Conceptual only. No OpenAPI. No browser route.

| Interface | Kind | Owner |
| --- | --- | --- |
| `GetApplicableDocumentRequirements(...)` | Internal Go port in document-service. Reads immutable snapshots for the server tenant, subject, and business event time | Agent B |
| `EvaluateDocumentCompleteness(...)` | Internal Go port in document-service. Composes those requirements with local evidence | Agent B |
| `GetTMSDocumentRequirementDecision(...)` | Internal HTTP read against shipment-service, used only at capture. Not present today | Agent C publishes. Agent B calls |
| `GetAccountingDocumentPolicyDecision(...)` | Internal HTTP read against the Finance runtime, used only at capture. Not present today | Agent G publishes. Agent B calls |

The composition of record is the Go port over stored snapshots. The owner HTTP read is the capture adapter. Events may later deliver the same immutable decision into the same snapshot table. They are not required, and this contract does not add a broker.

Evaluation response for an authorized subject is an envelope, including when the status is `UNKNOWN`. Transport failure is reserved for authentication failure and for the indistinguishable missing-or-foreign identity deny. Callers must not treat a transport deny as `INCOMPLETE`.

No browser-facing policy mutation API belongs to 0.1B or to the foundation slice.

## 13. Error and UNKNOWN semantics

| Code | Meaning | Caller-visible result |
| --- | --- | --- |
| `DOMAIN_UNKNOWN` | Owner decision is absent, `UNKNOWN`, or `NEEDS_REVIEW` | Envelope `completeness_status=UNKNOWN` |
| `DEPENDENCY_UNAVAILABLE` | Owner or verifier cannot be read and no pinned snapshot exists, or a required `VALID` verifier result cannot be produced | Envelope `UNKNOWN`. Not `INCOMPLETE` |
| `NOT_FOUND` | Collapsed with forbid for foreign or missing ids | Transport deny. No existence bit. Not a completeness status |
| `FORBIDDEN` | Same external result as `NOT_FOUND` for foreign identities. Authenticated caller outside the server tenant | Transport deny. No foreign id |
| `CONFLICTING_POLICY` | Applicable decisions disagree without proven precedence | Envelope `UNKNOWN` |
| `UNRESOLVED_SUCCESSOR` | `CORRECTS` or `REPLACES` without a frozen current-document policy | Envelope `UNKNOWN` |
| `UNSUPPORTED_DOCUMENT_TYPE_MAPPING` | Owner class has no canonical type | Envelope `UNKNOWN` |
| `HISTORICAL_REPLAY_BLOCKED` | Owner cannot supply a stable decision id and version | Envelope `UNKNOWN` |

```text
DEPENDENCY_OUTAGE_IS_INCOMPLETE=NO
MISSING_POLICY_IS_COMPLETE=NO
UNKNOWN_FAIL_CLOSED=YES
```

`COMPLETE` and `INCOMPLETE` are returned only from a known required set, a known acceptance set, a pinned source version, and known current evidence, with no unresolved successor on a required type.

## 14. Analytics boundary

GAP-B-001 still requires Analytics to consume completeness from EDO. Analytics does not derive it. This stage does not change Analytics.

Future output, one evaluation:

| Field | Rule |
| --- | --- |
| `tenant_id` | Server tenant |
| `subject_type`, `subject_id` | Evaluated subject |
| `business_context` | Event the evaluation used |
| `completeness_status` | `COMPLETE`, `INCOMPLETE`, or `UNKNOWN` |
| `required_document_types` | Known required types. Omitted when the set is unknown. Not an empty list standing in for unknown |
| `missing_required_document_types` | Known gaps. Omitted when status is `UNKNOWN` |
| `satisfied_document_ids` | Canonical ids in the same tenant that are safe satisfying evidence. Ambiguous successor ids are omitted |
| `policy_inputs` | Owner domain, source decision id, source decision version |
| `evaluated_at` | Evaluation clock |
| `unknown_reason_codes` | The codes in section 13 when status is `UNKNOWN` |

The output does not include payload JSON, attachment bytes, certificate material, issuer tax identifiers, or a foreign document id.

## 15. Implementation slices

| Slice | Owner | Prerequisite | Authorized now |
| --- | --- | --- | --- |
| B-next. Fail-closed foundation in document-service: snapshot and evaluation tables, Go ports, evaluator that returns `UNKNOWN` without an owner decision, and `COMPLETE` / `INCOMPLETE` only for a fixture or captured decision that already carries canonical type, acceptance statuses, and a pinned version | Agent B | Controller authorization of that limited scope. Successor policy may stay unfrozen: any `CORRECTS` or `REPLACES` on a required type returns `UNKNOWN` | The scope is limited enough to implement after authorization. This 0.1B document does not authorize it |
| C-next. Server-owned POD requirement source and internal read, including delivery identity, `pod_required`, canonical type, `requirement_decided_at`, decision id, and decision version | Agent C | TMS source design. Not an Agent B business rule | No |
| G-next. Server-owned `ACCOUNTING_DOCUMENT_POLICY_DECISION` runtime and internal read | Agent G | Finance implementation authorization. Not an Agent B tax rule | No |
| B-integration. Capture adapters and composition of C and G snapshots into completeness | Agent B | C-next and G-next endpoints, plus the B-next foundation | No |
| E-next. Analytics consumption of the section 14 output for the GAP-B-001 KPI | Agent E | A finalized EDO completeness result from B-integration. Analytics must not compute the status | No |

Agent B does not implement C-next or G-next business rules.

```text
B_FOUNDATION_IMPLEMENTABLE_NOW=YES
POD_RUNTIME_IMPLEMENTABLE_NOW=NO
FINANCE_RUNTIME_INTEGRATION_IMPLEMENTABLE_NOW=NO
FULL_GAP_B_001_RUNTIME_IMPLEMENTABLE_NOW=NO
```

`B_FOUNDATION_IMPLEMENTABLE_NOW=YES` means only the fail-closed foundation. It does not mean GAP-B-001 can be published. The next stage a controller can authorize is that foundation. POD runtime waits on Agent C. Finance integration waits on Agent G. Full GAP-B-001 waits on B-integration and on the successor freeze for any evaluation that must choose a current document.

## 16. Future acceptance tests

All tests are `FUTURE_REQUIRED`. This stage does not add them.

| Id | Expected result |
| --- | --- |
| `REQ-RUN-01` | Known required type plus a satisfying current document, no successor ambiguity → `COMPLETE` |
| `REQ-RUN-02` | Known required type plus known absent satisfying evidence → `INCOMPLETE` |
| `REQ-RUN-03` | Missing policy → `UNKNOWN` |
| `REQ-RUN-04` | Finance `UNKNOWN` → `UNKNOWN` |
| `REQ-RUN-05` | Finance `NEEDS_REVIEW` → `UNKNOWN` |
| `REQ-RUN-06` | POD requirement `UNKNOWN` → satisfaction `UNKNOWN` |
| `REQ-RUN-07` | `SIGNED` alone does not imply `COMPLETE` |
| `REQ-RUN-08` | Document count does not imply `COMPLETE` |
| `REQ-RUN-09` | Cross-tenant requirement read is denied and does not leak |
| `REQ-RUN-10` | A cross-tenant document cannot satisfy |
| `REQ-RUN-11` | Policy conflict without proven precedence → `UNKNOWN` |
| `REQ-RUN-12` | Historical evaluation pins `source_decision_id` and `source_decision_version` for the business event time |
| `REQ-RUN-13` | A later policy version does not update the prior evaluation row |
| `REQ-RUN-14` | Multiple required types are evaluated independently |
| `REQ-RUN-15` | `NOT_APPLICABLE` is not `REQUIRED` |
| `REQ-RUN-16` | `UNKNOWN` is not `OPTIONAL`, not false, and not an empty required set |
| `REQ-RUN-17` | Ambiguous `CORRECTS` or `REPLACES` → `UNKNOWN`, and neither id is satisfying evidence |
| `REQ-RUN-18` | Owner or verifier unavailable, with no pinned snapshot, is `UNKNOWN` / `DEPENDENCY_UNAVAILABLE`, not `INCOMPLETE` |
| `REQ-RUN-19` | Finance VAT, issuer, recipient, and legal applicability are not recomputed in document-service |
| `REQ-RUN-20` | TMS POD requirement rules are not recomputed in document-service. `RequiresPOD: false` is not the decision |

```text
ACCEPTANCE_MATRIX_FROZEN=YES
```

## 17. Non-goals

No product code, migration, OpenAPI, gateway, staging, KPI publication, or ADR belongs to this stage. Runtime implementation of GAP-B-001 stays unauthorized. Agent B does not self-approve a later merge.

```text
GAP_B_001_RUNTIME_IMPLEMENTATION_AUTHORIZED=NO
PRODUCT_CODE_CHANGED=NO
MIGRATION_CREATED=NO
OPENAPI_CHANGED=NO
GATEWAY_CHANGED=NO
STAGING_CHANGED=NO
```
