# EDO document requirement policy 0.1A

## Status

```text
STAGE=DOCUMENT-REQUIREMENT-POLICY-0.1A
BASE_SHA=755f2a946f5fa746363935de25d0925e32fd9e90
KIND=DOCS_ONLY_DISCOVERY
PRODUCT_IMPLEMENTATION_AUTHORIZED=NO
MIGRATION_AUTHORIZED=NO
OPENAPI_AUTHORIZED=NO
STAGING_AUTHORIZED=NO
REQUIRED_DOCUMENT_POLICY_RUNTIME_PRESENT=NO
DOCUMENT_REQUIREMENT_POLICY_MODEL_FROZEN=YES
DOCUMENT_COMPLETENESS_SOURCE_CONTRACT=edo-document-completeness-source-v0.4b
GAP_B_001_RUNTIME_IMPLEMENTATION_AUTHORIZED=NO
```

This note freezes the server-owned architecture for `REQUIRED_DOCUMENT_POLICY`. It does not implement a table, API, Go interface, migration, OpenAPI change, or staging change. It does not invent POD, tax, transport-execution, or Finance business rules.

It answers how a future evaluation names, for a business subject and event, which canonical document types are `REQUIRED`, `OPTIONAL`, `NOT_APPLICABLE`, or `UNKNOWN`, under which policy version and effective period. That framework is the missing input for GAP-B-001 runtime design. Completeness evaluation stays on [edo-document-completeness-source-v0.4b.md](edo-document-completeness-source-v0.4b.md).

## Ownership

| Responsibility | Owner |
| --- | --- |
| POD and other shipment-execution requirement facts | Agent C / TMS |
| Accounting document requirement decisions | Agent G / Finance |
| Canonical document type vocabulary | Agent B / document-service |
| Document evidence and current-document semantics | Agent B |
| Completeness evaluation that composes requirements and evidence | Agent B |
| Requirement-policy framework and composition envelope | Agent B |

```text
POD_REQUIREMENT_OWNER=AGENT_C
ACCOUNTING_DOCUMENT_POLICY_OWNER=AGENT_G
DOCUMENT_TYPE_CANONICALIZATION_OWNER=AGENT_B
DOCUMENT_EVIDENCE_OWNER=AGENT_B
DOCUMENT_COMPLETENESS_EVALUATION_OWNER=AGENT_B
FINANCE_POLICY_DUPLICATED_IN_EDO=NO
```

Agent B must not invent:

```text
POD_REQUIRED=true|false
EVERY_SHIPMENT_REQUIRES_POD=NO
EVERY_SHIPMENT_REQUIRES_UPD=NO
SIGNED_DOCUMENT_IS_REQUIRED_DOCUMENT=NO
DOCUMENT_EXISTS_IMPLIES_REQUIREMENT=NO
```

Finance 0.1D remains the accounting decision authority. TMS Analytics source contracts 0.4B remain the POD requirement ownership record. This framework consumes those decisions. It does not rewrite them.

## Policy envelope

A conceptual requirement-policy identity contains at least:

| Field | Meaning |
| --- | --- |
| `POLICY_ID` | Stable server id of the policy definition |
| `POLICY_VERSION` | Immutable version of that definition |
| `POLICY_OWNER_DOMAIN` | `TMS_EXECUTION`, `FINANCE_ACCOUNTING`, `TENANT_OPERATIONS`, or another named owner domain |
| `POLICY_SCOPE` | What subject population the version covers |
| `EFFECTIVE_FROM` | Inclusive start of applicability |
| `EFFECTIVE_TO` | Nullable exclusive end |
| `TENANT_SCOPE` | Server-derived tenant or platform applicability class |
| `SUBJECT_TYPE` | Subject class the row applies to |
| `EVENT_TYPE` / `BUSINESS_CONTEXT` | Business event or context that activates the row |
| `DOCUMENT_TYPE` | Canonical document-service type |
| `REQUIREMENT_STATE` | One of the states below |
| `SOURCE_RULE_ID` | Owning-domain rule id that produced the row |
| `CREATED_AT` | When this version was recorded |
| `SUPERSEDES_POLICY_VERSION` | Nullable prior version this one replaces |

```text
REQUIREMENT_STATE=REQUIRED|OPTIONAL|NOT_APPLICABLE|UNKNOWN
POLICY_VERSION_REQUIRED=YES
EFFECTIVE_DATE_REQUIRED=YES
```

`UNKNOWN` is not `NOT_APPLICABLE`. `NOT_APPLICABLE` means the owner domain decided the type is out of scope. `UNKNOWN` means the owner domain, or the composition, cannot prove a state.

A policy result for one evaluation returns:

| Result field | Meaning |
| --- | --- |
| `REQUIRED_DOCUMENT_TYPES` | Types with `REQUIREMENT_STATE=REQUIRED` |
| `OPTIONAL_DOCUMENT_TYPES` | Types with `REQUIREMENT_STATE=OPTIONAL` |
| `UNKNOWN_DOCUMENT_REQUIREMENTS` | Types or slots whose state is `UNKNOWN` |
| `NOT_APPLICABLE_DOCUMENT_TYPES` | Types the owner marked out of scope |
| `POLICY_ID` / `POLICY_VERSION` | Version used for the evaluation |
| `EVALUATED_AT` | Time of this evaluation |

There is no hardcoded universal required set. An empty `REQUIRED_DOCUMENT_TYPES` is valid only when an authoritative policy proves that nothing is required. Missing policy is not an empty set.

## Subject and context inventory

Document-service already links documents with nullable `related_entity_type` values `SHIPMENT`, `TRANSPORT_ORDER`, `BILLING_REGISTER`, `INVOICE`, and `COMPANY`. Completeness 0.4B evaluates `SHIPMENT` and `TRANSPORT_ORDER` as subjects. TMS owns delivery and execution identities. Finance owns accounting events and commercial chains.

| Subject | Classification | Notes |
| --- | --- | --- |
| `SHIPMENT` | `SUPPORTED_BY_CURRENT_FACTS` | Document related-entity and customer read contracts already use it |
| `TRANSPORT_ORDER` | `SUPPORTED_BY_CURRENT_FACTS` | Document related-entity already allows it |
| `DELIVERY` | `OWNER_DOMAIN_REQUIRED` | Agent C owns delivery identity for POD. Not a document related-entity today |
| `EXECUTION` | `FUTURE_SUPPORTED` | TMS execution revisions exist. No requirement policy binds them yet |
| `STOP` / `ACTION` | `FUTURE_SUPPORTED` | Multi-stop actions exist. Requirement ownership stays with Agent C if used |
| `COMMERCIAL_RELATIONSHIP` | `OWNER_DOMAIN_REQUIRED` | Finance commercial chain. Agent G |
| `SETTLEMENT` | `OWNER_DOMAIN_REQUIRED` | Finance / settlement. Agent G |
| `INVOICE` / accounting event | `OWNER_DOMAIN_REQUIRED` | Finance 0.1D decision subject. Agent G |
| `BILLING_REGISTER` | `FUTURE_SUPPORTED` | Present as a document related-entity. Not an operational completeness subject in 0.4B |
| `COMPANY` | `NOT_APPLICABLE` | Ownership metadata target, not a completeness subject for GAP-B-001 |

```text
ACTIVE_OPERATIONAL_SUBJECTS_FOR_COMPLETENESS=SHIPMENT,TRANSPORT_ORDER
ACTIVE_POD_SUBJECT=DELIVERY
ACTIVE_ACCOUNTING_SUBJECT=ACCOUNTING_EVENT
```

A subject that is `OWNER_DOMAIN_REQUIRED` cannot be completed by document-service alone. The owning domain must emit a requirement decision first.

## Composition with Agent C

Operational requirements such as POD stay with Agent C.

Agent C provides, when its source exists:

- delivery identity;
- `POD_REQUIRED`;
- `requirement_decided_at`;
- server tenant;
- canonical required document type when `POD_REQUIRED` is true.

Agent B provides:

- the document type vocabulary;
- tenant-scoped document evidence;
- successor and current-document semantics, once frozen;
- completeness or POD-satisfaction evaluation.

```text
C_REQUIREMENT + B_EVIDENCE → COMPLETENESS_OR_SATISFACTION
CROSS_DOMAIN_WRITE_REQUIRED=NO
```

Until Agent C publishes a server-owned requirement, `POD_REQUIRED` stays `UNKNOWN`. Document-service must not treat a hardcoded `RequiresPOD: false` milestone flag, a POD row count, or a Control Tower awaiting-documents counter as that requirement.

## Composition with Agent G

Finance 0.1D defines `ACCOUNTING_DOCUMENT_POLICY_DECISION` with `required_document_types`, issuer, recipient, VAT treatment, legal rule and version, and `decision_state` `KNOWN`, `UNKNOWN`, or `NEEDS_REVIEW`.

Agent B consumes a future known Finance decision as requirement input:

- map Finance document classes onto canonical document-service types;
- treat a Finance `UNKNOWN` or `NEEDS_REVIEW` decision as `REQUIREMENT_STATE=UNKNOWN` for that accounting subject;
- never recompute VAT class, rate, exemption, or issuer inside document-service.

```text
FINANCE_POLICY_DUPLICATED_IN_EDO=NO
ACCOUNTING_REQUIREMENT_INPUT=ACCOUNTING_DOCUMENT_POLICY_DECISION
```

An accounting required set and an operational customer required set may differ. Completeness evaluation names which policy inputs it composed. It does not collapse them into one silent list.

## Versioning and historical replay

```text
POLICY_VERSION_REQUIRED=YES
EFFECTIVE_DATE_REQUIRED=YES
HISTORICAL_POLICY_REPLAY=VERSION_PINNED
```

A historical completeness result is evaluated against the policy version applicable to the business event time, not against today's current policy by default. `EFFECTIVE_FROM` / `EFFECTIVE_TO` select the applicable version. `SUPERSEDES_POLICY_VERSION` records lineage. It does not authorize silent rewrite of past evaluations.

If the applicable historical version cannot be proven:

```text
REQUIREMENT_STATE=UNKNOWN
COMPLETENESS_STATUS=UNKNOWN
```

A later policy change creates a new version. It does not mutate the pinned version of an earlier evaluation.

## Conflict and precedence

When two applicable policies disagree on the same `DOCUMENT_TYPE` for the same subject and event, the system must not pick the newest row silently.

Conceptual evaluation order:

1. Domain-specific authoritative decision for that subject class (`TMS_EXECUTION` for POD, `FINANCE_ACCOUNTING` for accounting events).
2. Tenant override, only when an explicit server-owned override grant exists.
3. Platform default, only when an explicit server-owned default exists.

```text
SILENT_NEWEST_WINS=NO
POLICY_CONFLICT_WITHOUT_PROVEN_PRECEDENCE=UNKNOWN
```

If precedence cannot be proven, or two same-rank decisions disagree:

```text
REQUIREMENT_STATE=UNKNOWN
COMPLETENESS_STATUS=UNKNOWN
```

A tenant override that is not server-granted is ignored. A client-supplied policy id or version is not authority.

## Successor document policy

Completeness 0.4B already freezes:

```text
SUCCESSOR_SATISFACTION=NOT_EVALUABLE
SUCCESSOR_CURRENT_DOCUMENT_POLICY_PRESENT=NO
AMBIGUOUS_SUCCESSOR_COMPLETENESS=UNKNOWN
SATISFIED_DOCUMENT_ID_WHILE_SUCCESSOR_AMBIGUOUS=NOT_SELECTED
AMBIGUOUS_SUCCESSOR_PRESENT_AS_SATISFYING=NO
RELATED_TO_CHANGES_SATISFACTION=NO
```

This stage does not invent runtime values. It names the future fields a later authorized contract must freeze before successor satisfaction can leave `NOT_EVALUABLE`:

| Future field | Question it must answer |
| --- | --- |
| `EDGE_DIRECTION` | Which endpoint of `CORRECTS` / `REPLACES` is the current document |
| `CARDINALITY` | Whether multiple live edges are allowed for one requirement |
| `CURRENT_DOCUMENT_SELECTION` | How the authoritative current id is chosen |
| `PREDECESSOR_VALIDITY` | Whether the predecessor remains satisfying after `CORRECTS` or `REPLACES` |
| `SUCCESSOR_ACCEPTANCE_RULE` | Which acceptance states the successor must meet |

```text
RELATED_TO_CHANGES_SATISFACTION=NO
SUCCESSOR_CURRENT_DOCUMENT_POLICY_FROZEN=NO
```

Until that later freeze exists, ambiguous `CORRECTS` or `REPLACES` stays fail-closed `UNKNOWN`. `RELATED_TO` alone still does not substitute a type and does not create successor ambiguity by itself.

## Completeness semantics

Preserved from 0.4B:

```text
SIGNED_STATUS_IS_COMPLETENESS=NO
DOCUMENT_COUNT_IS_COMPLETENESS=NO
COMPLETENESS_STATUS=COMPLETE|INCOMPLETE|UNKNOWN
```

Conceptual evaluation:

```text
applicable required set
+ canonical current document evidence
→ COMPLETE | INCOMPLETE | UNKNOWN
```

| Condition | Result |
| --- | --- |
| Missing policy | `UNKNOWN` |
| Ambiguous or conflicting policy | `UNKNOWN` |
| Applicable historical version unproven | `UNKNOWN` |
| Ambiguous successor | `UNKNOWN` |
| Evidence unavailable | `UNKNOWN` |
| Known required set and every required type satisfied | `COMPLETE` |
| Known required set and at least one required type missing | `INCOMPLETE` |

Only a known required set plus known evidence may produce `COMPLETE` or `INCOMPLETE`. `OPTIONAL` types never block completeness. `NOT_APPLICABLE` types are not required. `UNKNOWN` types are not treated as optional.

## Tenancy

```text
TENANT_SERVER_DERIVED=YES
CLIENT_TENANT_AUTHORITY=NO
CROSS_TENANT_POLICY_READ=DENY
```

Policy applicability is selected from the trusted server tenant and the subject identity. A client tenant, body tenant, query tenant, or client-supplied policy id is not authority. A cross-tenant policy or relationship edge does not change the result and does not leak foreign ids.

## Future runtime boundary

Conceptual interfaces only. Not implemented here.

| Interface | Purpose | Future owner class |
| --- | --- | --- |
| `GetApplicableDocumentRequirements(...)` | Return the version-pinned requirement set for a subject and event | Policy decision composition. Inputs from TMS and Finance. Envelope owned by Agent B |
| `EvaluateDocumentCompleteness(...)` | Compose requirements with evidence into `COMPLETE` / `INCOMPLETE` / `UNKNOWN` | Completeness evaluator owned by Agent B |

| Component | Role | Ownership evidence |
| --- | --- | --- |
| Policy registry | Store versioned policy envelopes if a registry is later authorized | Not present. Owner not invented beyond the framework |
| Policy decision source | Emit domain decisions (`POD_REQUIRED`, Finance decision) | Agent C and Agent G for their domains |
| Evidence source | Canonical documents, attachments, successor edges | document-service / Agent B |
| Completeness evaluator | Apply 0.4B satisfaction rules to the required set | Agent B |

```text
RUNTIME_API_IMPLEMENTED=NO
GO_INTERFACE_IMPLEMENTED=NO
```

Service ownership of a shared registry is not assigned until a later Task Contract has evidence for where it must live.

## Future acceptance tests

All tests are `FUTURE_REQUIRED`. This stage does not add them.

1. `REQ-01`. Known required type and a satisfying current document → `COMPLETE`.
2. `REQ-02`. Known required type and no satisfying document → `INCOMPLETE`.
3. `REQ-03`. No policy → `UNKNOWN`.
4. `REQ-04`. Policy conflict without proven precedence → `UNKNOWN`.
5. `REQ-05`. Historical evaluation uses the version pinned to the business event.
6. `REQ-06`. A later policy change does not rewrite a historical pinned result.
7. `REQ-07`. POD requirement comes from Agent C and is composed with Agent B evidence.
8. `REQ-08`. Accounting requirement comes from Agent G and is not recomputed in EDO.
9. `REQ-09`. Cross-tenant policy or document read is denied without leakage.
10. `REQ-10`. Ambiguous successor → `UNKNOWN`.
11. `REQ-11`. `NOT_APPLICABLE` is not treated as `REQUIRED`.
12. `REQ-12`. `UNKNOWN` is not treated as `OPTIONAL`.
13. `REQ-13`. Multiple required types are evaluated independently.
14. `REQ-14`. `EFFECTIVE_FROM` / `EFFECTIVE_TO` boundaries select the applicable version.
15. `REQ-15`. A superseded version remains the pin for events that fell under it.

## Non-goals

No product code, migration, OpenAPI, gateway, S3, credential, staging, or KPI publication belongs to this stage. No ADR is created. Runtime implementation of GAP-B-001 stays unauthorized until a server-owned requirement policy exists and a controller authorizes that work.

```text
GAP_B_001_ARCHITECTURE_READY=YES
GAP_B_001_RUNTIME_IMPLEMENTATION_AUTHORIZED=NO
DOCUMENT_COMPLETENESS_RUNTIME_READY=NO
```
