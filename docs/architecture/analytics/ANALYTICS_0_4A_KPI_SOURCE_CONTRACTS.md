# Analytics-0.4A KPI source contracts

Consumer contract only. Analytics states the facts it needs. Source owners keep the facts, the storage, and the write path. This stage does not implement a source fact, add a KPI, or close a gap.

```
STAGE=ANALYTICS-KPI-SOURCE-CONTRACTS-0.4A
BASE_SHA=cbbba8e413c8ef1c716b242533d7f79aaaafe47a
DOCS_ONLY=YES
SOURCE_IMPLEMENTATION_AUTHORIZED=NO
ANALYTICS_IMPLEMENTATION_AUTHORIZED=NO
MIGRATION_AUTHORIZED=NO
STAGING_DEPLOY_AUTHORIZED=NO
CURRENT_KPI_COUNT=11
NEW_KPI_COUNT=0
KPI_FORMULA_CHANGED=NO
SOURCE_GAPS_CLOSED_BY_THIS_STAGE=0
NEW_KPI_IMPLEMENTED=NO
BLOCKED_KPI_PROMOTED=NO
```

The live source read stays `GET /internal/v1/analytics/operations-foundation-v2`. Analytics-service stays stateless and has no database client.

## Ownership

```
ANALYTICS_OWNS_KPI_SEMANTICS=YES
ANALYTICS_OWNS_SOURCE_FACTS=NO
ANALYTICS_DIRECT_SOURCE_DB_READ=NO
SOURCE_OWNED_INTERNAL_READ_CONTRACTS=YES
```

Analytics may name the fact, the consumer fields, completeness, the required dimension, and failure behavior. Analytics does not choose a table, a migration number, a storage design, or a source write workflow. Those belong to Agent C for shipment and execution facts and to Agent B for document facts.

## Gap status

`CONTRACT_REQUESTED` means the consumer request is written. It does not mean the fact exists.

| Gap | Owner | Before 0.4A | After 0.4A | Implementation |
| --- | --- | --- | --- | --- |
| GAP-C-001 | C | OPEN | CONTRACT_REQUESTED | NO |
| GAP-C-002 | C | OPEN | CONTRACT_REQUESTED | NO |
| GAP-C-005 | C | OPEN | CONTRACT_REQUESTED | NO |
| GAP-B-001 | B | OPEN | CONTRACT_REQUESTED | NO |
| GAP-E-001 | Business/E | OPEN | OPEN | NO |

```
GAP_E_001=OPEN
CARRIER_SCORE_IMPLEMENTED=NO
CARRIER_SCORE_READY=NO
```

No component set, weight, missing-component rule, minimum sample, or score scale is chosen.

## Shared rules

Every future source contract exposes an explicit `SOURCE_CONTRACT_VERSION`. The marker may be an endpoint version or a response contract version. Analytics fails closed on an unsupported version.

```
SILENT_SOURCE_SEMANTIC_CHANGE_ALLOWED=NO
```

Event time and evaluated time stay separate. Business event time is not replaced by request time or processing time. Wire timestamps are UTC RFC3339.

```
PROCESSING_TIME_SUBSTITUTES_EVENT_TIME=NO
```

Tenant scope is server-owned. A caller-supplied tenant is not authority. Analytics continues to use the tenant derived by the gateway. Future internal reads use the trusted service-to-service boundary. No public source-fact endpoint is required.

```
CLIENT_SUPPLIED_TENANT_AUTHORITY=NO
ANALYTICS_TENANT_SERVER_DERIVED=YES
```

A missing fact is not zero, not false, and not complete. Each contract reports `KNOWN` or `UNKNOWN` / `NOT_EVALUABLE`, or an equivalent explicit completeness state. Analytics does not coerce an unknown fact into a zero denominator or into false.

```
UNKNOWN_FACT_COERCED_TO_FALSE=NO
UNKNOWN_FACT_COERCED_TO_ZERO=NO
UNKNOWN_FACT_COERCED_TO_COMPLETE=NO
```

The transport shape is the source owner's decision. The population semantics are not.

```
ROW_LEVEL_OR_SOURCE_AGGREGATE=SOURCE_OWNER_DECISION
SEMANTIC_CORRELATION_PRESERVED=YES
```

A source aggregate is acceptable only when it preserves the exact population the frozen KPI semantics need.

```
DWH_IMPLEMENTED=NO
HISTORICAL_ANALYTICS_READY=NO
ANALYTICS_UI_IMPLEMENTED=NO
ANALYTICS_UI_OWNER=AGENT_F
ANALYTICS_STAGING_DEPLOYED=NO
ANALYTICS_STAGING_DEPLOY_OWNER=AGENT_A
```

These contracts are current authoritative facts. A period or as-of requirement stays a later history stage.

## GAP-C-001 in-full

```
REQUEST_ID=AN-SRC-C-001
SOURCE_GAP=GAP-C-001
SOURCE_OWNER=AGENT_C
DOMAIN_OWNER=shipment-service / TMS execution
```

A completed delivery needs an authoritative, server-owned in-full fact. Absence of a delivery-disposition case is not in-full. Status `DELIVERED` is not in-full.

Minimum semantics for each relevant completed delivery:

| Field | Requirement |
| --- | --- |
| DELIVERY_IDENTITY | Server-owned stable delivery, stop, or action identity |
| SHIPMENT_ID | Required |
| CARRIER_COMPANY_ID | Required when a carrier KPI dimension is needed |
| COMPLETED_AT | Authoritative event time |
| SHIPPED_QUANTITY | Authoritative quantity |
| ACCEPTED_QUANTITY | Authoritative quantity |
| REJECTED_QUANTITY | Authoritative quantity |
| QUANTITY_UOM | Explicit |
| IN_FULL | Server-derived boolean or an equivalent closed enum |

```
IN_FULL_DERIVED_FROM_QUANTITY_FACTS=YES
IN_FULL_INFERRED_FROM_MISSING_CASE=NO
IN_FULL_INFERRED_FROM_DELIVERED_STATUS=NO
```

Unknown or incomplete quantity evidence stays `UNKNOWN` / `NOT_EVALUABLE`. It is neither true nor false.

A future OTIF measure is on-time and in-full for the same delivery. Separate tenant-wide counts that drop that correlation are not enough, unless the source contract itself emits the authoritative joint population.

```
OTIF_SEMANTIC_CORRELATION_PRESERVED=YES
```

This stage does not publish `OPS_OTIF`, `CAR_OTIF`, or `EXEC_OTIF`.

## GAP-C-002 POD requirement

```
REQUEST_ID=AN-SRC-C-002
SOURCE_GAP=GAP-C-002
SOURCE_OWNER=AGENT_C
DOCUMENT_EVIDENCE_OWNER=AGENT_B
```

Document-service remains the owner of document evidence. Analytics needs to know whether a delivery requires POD, whether a required POD is satisfied, and which canonical document satisfied it.

| Field | Requirement |
| --- | --- |
| DELIVERY_IDENTITY | Required |
| SHIPMENT_ID | Required |
| POD_REQUIRED | Server-owned |
| REQUIRED_DOCUMENT_TYPE | Canonical type when `POD_REQUIRED` is true |
| POD_SATISFIED | Server-owned result, or a source-owned composed result |
| SATISFIED_DOCUMENT_ID | Nullable. Canonical document reference when satisfied |
| SATISFIED_AT | Nullable |
| EVALUATED_AT | Required |

```
POD_REQUIRED_DEFAULT_FALSE_IS_NOT_COMPLETENESS_PROOF=YES
DOCUMENT_ROW_COUNT_IS_NOT_POD_REQUIREMENT=YES
POD_REQUIREMENT_SERVER_OWNED=YES
```

Analytics does not compute POD completeness as a count of POD documents divided by delivered shipments unless the source contract proves that denominator. This stage does not design the cross-domain write path between Agent C and Agent B, and it does not publish `OPS_POD_COMPLETENESS` or `CAR_POD_COMPLETENESS`.

## GAP-C-005 carrier assignment rejection

```
REQUEST_ID=AN-SRC-C-005
SOURCE_GAP=GAP-C-005
SOURCE_OWNER=AGENT_C
DOMAIN_OWNER=shipment-service / TMS
```

The needed fact is a canonical assignment decision. It is not a delivery or customer refusal, not cargo rejection, not an unanswered RFx invite, and not a shipment cancellation whose actor is unknown.

| Field | Requirement |
| --- | --- |
| ASSIGNMENT_OFFER_ID | Stable server-owned identity |
| SHIPMENT_ID | Required |
| CARRIER_COMPANY_ID | Required |
| DECISION | `ACCEPTED` or `REJECTED` |
| DECIDED_AT | Authoritative event time |
| DECISION_ACTOR | Server-derived actor identity or class |
| REASON_CODE | Nullable, versioned domain reason |
| TENANT_SCOPE | Server-owned |

```
DELIVERY_REJECTION_IS_CARRIER_ASSIGNMENT_REJECTION=NO
RFX_NON_RESPONSE_IS_CARRIER_ASSIGNMENT_REJECTION=NO
GENERIC_CANCELLED_STATUS_IS_REJECTION=NO
CARRIER_ASSIGNMENT_REJECTION_DISTINCT=YES
```

A future rejection-rate denominator is the canonical offer and decision population, not all shipments. This stage does not publish `CAR_REJECTION_RATE`.

## GAP-B-001 document completeness

```
REQUEST_ID=AN-SRC-B-001
SOURCE_GAP=GAP-B-001
SOURCE_OWNER=AGENT_B
DOMAIN_OWNER=document-service / EDO
```

Analytics does not invent which documents are required.

| Field | Requirement |
| --- | --- |
| RELATED_ENTITY_TYPE | `SHIPMENT` or `TRANSPORT_ORDER`, as the source policy supports |
| RELATED_ENTITY_ID | Required |
| REQUIRED_DOCUMENT_TYPES | Server-owned evaluated set |
| PRESENT_DOCUMENT_IDS | Canonical document ids that satisfy the requirement |
| MISSING_DOCUMENT_TYPES | Server-derived |
| COMPLETENESS_STATUS | `COMPLETE`, `INCOMPLETE`, or `UNKNOWN` |
| EVALUATED_AT | Required |
| TENANT_SCOPE | Server-owned |

```
DOCUMENT_STATUS_SIGNED_IS_COMPLETENESS=NO
DOCUMENT_COUNT_IS_COMPLETENESS=NO
CONTROL_TOWER_AWAITING_DOCUMENTS_IS_CANONICAL_COMPLETENESS=NO
DOCUMENT_REQUIRED_SET_SERVER_OWNED=YES
```

Required-document policy belongs to Agent B and the document domain. This stage does not publish `OPS_DOCUMENT_COMPLETENESS` or `CAR_DOCUMENT_COMPLETENESS`.

## Handoffs

`AN-SRC-*` identifiers are Analytics-local tracking ids. They are not platform cross-workstream ids.

### AN-SRC-C-001

```
REQUEST_ID=AN-SRC-C-001
SOURCE_GAP=GAP-C-001
FROM=AGENT_E
TO=AGENT_C
BUSINESS_REASON=Future on-time-and-in-full needs an authoritative in-full result for the same delivery. A missing disposition case and status DELIVERED are not that result.
REQUIRED_SEMANTICS=Server-owned delivery identity, shipment, quantities with an explicit unit, and an in-full result derived from those quantities. Unknown quantity stays not evaluable. Carrier id is required when a carrier dimension is needed.
SECURITY_REQUIREMENT=Server-owned tenant. Trusted internal read. No caller-supplied tenant authority.
BREAKING_CHANGE_EXPECTED=NO
ANALYTICS_DIRECT_DB_ACCESS=NO
IMPLEMENTATION_AUTHORIZED=NO
NEXT_CONTROLLER_TASK_REQUIRED=YES
```

### AN-SRC-C-002

```
REQUEST_ID=AN-SRC-C-002
SOURCE_GAP=GAP-C-002
FROM=AGENT_E
TO=AGENT_C
BUSINESS_REASON=POD completeness needs a server-owned requirement and a satisfaction result. A hardcoded false requirement and a count of document rows are not that fact.
REQUIRED_SEMANTICS=Delivery identity, shipment, whether POD is required, the canonical required type when it is, whether it is satisfied, the satisfying document id when it is, and the evaluation time.
SECURITY_REQUIREMENT=Server-owned tenant. Trusted internal read. No caller-supplied tenant authority.
BREAKING_CHANGE_EXPECTED=NO
ANALYTICS_DIRECT_DB_ACCESS=NO
IMPLEMENTATION_AUTHORIZED=NO
NEXT_CONTROLLER_TASK_REQUIRED=YES
```

### AN-SRC-C-005

```
REQUEST_ID=AN-SRC-C-005
SOURCE_GAP=GAP-C-005
FROM=AGENT_E
TO=AGENT_C
BUSINESS_REASON=A carrier assignment rejection rate needs an offer decision. Customer refusal, cargo rejection, an unanswered RFx invite, and a generic cancellation are different facts.
REQUIRED_SEMANTICS=Stable offer identity, shipment, carrier, decision ACCEPTED or REJECTED, decision time, server-derived actor, optional versioned reason, and server-owned tenant. The denominator is the offer population.
SECURITY_REQUIREMENT=Server-owned tenant. Trusted internal read. No caller-supplied tenant authority.
BREAKING_CHANGE_EXPECTED=NO
ANALYTICS_DIRECT_DB_ACCESS=NO
IMPLEMENTATION_AUTHORIZED=NO
NEXT_CONTROLLER_TASK_REQUIRED=YES
```

### AN-SRC-B-001

```
REQUEST_ID=AN-SRC-B-001
SOURCE_GAP=GAP-B-001
FROM=AGENT_E
TO=AGENT_B
BUSINESS_REASON=Document completeness needs the required set and the canonical documents that satisfy it. Signed status, document count, and a Control Tower awaiting-documents signal are not that set.
REQUIRED_SEMANTICS=Related entity, server-owned required types, present canonical document ids, missing types, and completeness COMPLETE, INCOMPLETE, or UNKNOWN. Unknown policy stays UNKNOWN.
SECURITY_REQUIREMENT=Server-owned tenant. Present documents are canonical and tenant-scoped. No caller-supplied tenant authority.
BREAKING_CHANGE_EXPECTED=NO
ANALYTICS_DIRECT_DB_ACCESS=NO
IMPLEMENTATION_AUTHORIZED=NO
NEXT_CONTROLLER_TASK_REQUIRED=YES
```

## Future source tasks

These names are recommendations. They are not authorized.

```
AGENT_C_NEXT_STAGE=SOURCE-CONTRACT-C-ANALYTICS-FACTS-0.4B
AGENT_C_NEXT_STAGE_SCOPE=GAP-C-001, GAP-C-002, GAP-C-005
AGENT_C_NEXT_STAGE_AUTHORIZED=NO
AGENT_B_NEXT_STAGE=SOURCE-CONTRACT-B-DOCUMENT-COMPLETENESS-0.4B
AGENT_B_NEXT_STAGE_SCOPE=GAP-B-001
AGENT_B_NEXT_STAGE_AUTHORIZED=NO
```

Each one needs its own controller task. Agent E does not implement either.

A future Agent C implementation has to prove that a zero-rejection delivery can still produce authoritative in-full, a partial delivery cannot be in-full, missing quantity stays unknown, the POD-required denominator is server-owned, POD satisfaction links a canonical document, assignment rejection stays distinct from customer refusal, and a cross-tenant read fails closed.

A future Agent B implementation has to prove that the required set is server-owned, present documents are canonical and tenant-scoped, a missing required type is `INCOMPLETE`, an unknown policy is `UNKNOWN`, `SIGNED` is not completeness, and a cross-tenant read fails closed.

Those proofs are later acceptance requirements. This stage does not add the tests.

## Current KPI protection

```
CURRENT_KPI_COUNT_BEFORE=11
CURRENT_KPI_COUNT_AFTER=11
CURRENT_11_KPI_DEFINITION_CHANGE=NO
CURRENT_SOURCE_V2_CHANGE=NO
```

`operations-foundation-v2`, the analytics KPI registry, the gateway route, and OpenAPI stay as they are.
