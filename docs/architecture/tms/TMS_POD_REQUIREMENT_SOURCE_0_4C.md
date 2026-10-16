# TMS POD requirement source 0.4C

STAGE=TMS-POD-REQUIREMENT-SOURCE-0.4C-TASK-CONTRACT
BASE_SHA=109ec8117b75cb8205af7667eef6d52b30a11bc6
KIND=DOCS_ONLY_ARCHITECTURE
PRODUCT_CODE_AUTHORIZED=NO
MIGRATION_AUTHORIZED=NO
OPENAPI_AUTHORIZED=NO
GATEWAY_AUTHORIZED=NO
STAGING_AUTHORIZED=NO
GAP_C_002_RUNTIME_IMPLEMENTATION_AUTHORIZED=NO

This document freezes the server-owned TMS contract for whether proof of delivery is required for one delivery action. It does not add a table, command, route, OpenAPI operation, or migration.

Inventory is against `origin/main` `109ec8117b75cb8205af7667eef6d52b30a11bc6`. Runtime claims below were checked in shipment-service code and migrations, not taken from earlier architecture notes alone.

Prior ownership remains in [TMS analytics source contracts 0.4B](TMS_ANALYTICS_SOURCE_CONTRACTS_0_4B.md). Document evidence and completeness stay in [EDO document requirement policy 0.1A](../edo-document-requirement-policy-v0.1a.md) and [EDO document completeness source 0.4B](../edo-document-completeness-source-v0.4b.md). Successor and completed-history rules remain in [REPLAN_SUCCESSOR_MODEL.md](../tms-multistop/REPLAN_SUCCESSOR_MODEL.md) and [ADR-TMS-004](../tms-multistop/adr/ADR-TMS-004-completed-stop-immutability.md).

```text
POD_REQUIREMENT_OWNER=shipment-service/TMS/Agent C
POD_DOCUMENT_EVIDENCE_OWNER=document-service/Agent B
TMS_DECIDES_REQUIREMENT=YES
EDO_DECIDES_REQUIREMENT=NO
EDO_EVALUATES_DOCUMENT_EVIDENCE=YES
CROSS_DOMAIN_WRITE_REQUIRED=NO
POD_REQUIREMENT_SOURCE=NONE
UNKNOWN_TO_FALSE=NO
UNKNOWN_TO_ZERO=NO
UNKNOWN_TO_NOT_REQUIRED=NO
```

Agent C does not read document rows to decide whether POD is satisfied. Agent B does not infer `POD_REQUIRED` from a document row. Analytics does not read TMS tables and does not count POD documents.

## Current runtime

```text
POD_REQUIREMENT_SOURCE_PRESENT=NO
POD_REQUIREMENT_TABLE_PRESENT=NO
POD_REQUIREMENT_INTERNAL_READ_PRESENT=NO
POD_REQUIREMENT_DECISION_COMMAND_PRESENT=NO
DRIVER_REQUIRES_POD_FLAG_PRESENT=YES
DRIVER_REQUIRES_POD_FLAG_VALUE=false
DRIVER_REQUIRES_POD_FLAG_AUTHORITATIVE=NO
SHIPMENT_EXECUTION_VIEW_POD_FIELD_PRESENT=YES
SHIPMENT_EXECUTION_VIEW_POD_FIELD_AUTHORITATIVE=NO
CUSTOMER_POD_POLICY_PRESENT=NO
CONTRACT_POD_POLICY_PRESENT=NO
TENANT_POD_POLICY_PRESENT=NO
TRANSPORT_ORDER_POD_POLICY_PRESENT=NO
EXECUTION_ACTION_POD_POLICY_PRESENT=NO
```

`DriverMilestoneAction.RequiresPOD` is a bool on the driver milestone map in `services/shipment-service/internal/domain/execution_tracking.go`. `AllowedDriverMilestoneActions` sets `RequiresPOD: false` on both `DELIVERY_COMPLETED` entries (`ARRIVED_AT_CONSIGNEE` and `UNLOADING`). Other milestone entries leave the Go zero value, which is also false. The order-execution response copies `allowed_actions` through for a carrier actor. Nothing in shipment-service reads this flag when recording a stop, an action, or a disposition.

```text
REQUIRES_POD_DRIVER_ACTION_FLAG_IS_POLICY=NO
REQUIRES_POD_FALSE_IS_KNOWN_NOT_REQUIRED=NO
REQUIRES_POD_FALSE_IS_COMPLETENESS_PROOF=NO
```

The flag is UI/action metadata for the driver milestone list. It tells a client which milestone button was labeled as needing a POD upload. It is not a stored decision, not a version, and not proof that POD is not required.

`ShipmentExecutionView.PODRequired` is a bool field on a type that no handler constructs and that has no JSON tag. It is not an API field and it is not assigned anywhere. Its zero value is false. That zero value is not a business decision.

`OrderExecutionRepository.ListShipmentPODDocuments` selects `documents.documents` rows with `document_type = 'POD'` for one shipment. The order-execution view returns those rows as `pod_documents` when any exist. That query is a shipment-scoped evidence list. It is not a requirement, and it is not satisfaction. A future POD decision writer must not use this query.

No migration creates a POD requirement table. No internal route reads or writes a POD requirement. `transport.transport_execution_actions` stores `action_type` `PICKUP` or `DELIVERY` and a status. It does not store a POD policy. `CUSTOMER_REFUSAL` is a disposition reason code in `transport_execution_disposition.go`. It is not a requirement rule.

Searches of Go and SQL for `pod_required`, `requires_pod`, `PODRequired`, and `proof_of_delivery` find only the two fields in `execution_tracking.go`. No customer, contract, rate, tenant, transport-order, or execution-action policy in this tree states that POD is required or not required.

Until a later authorized writer records a known decision, every delivery action has:

```text
POD_REQUIRED=UNKNOWN
```

A missing row is the same answer. It is not false.

## Canonical delivery identity

One shipment may have many delivery actions. The requirement subject is one delivery action, not the shipment.

```text
ONE_SHIPMENT_MAY_HAVE_MULTIPLE_DELIVERY_REQUIREMENTS=YES
POD_DECISION_SCOPE=DELIVERY_ACTION
CANONICAL_DELIVERY_IDENTITY=operating_tenant_id + execution_id + execution_action_id
```

`execution_action_id` is `transport.transport_execution_actions.id`. The action row is parented by `execution_stop_id`, and the stop is parented by `execution_id`. The action already carries `shipment_id`, `shipment_tenant_id`, and `cargo_id`. Only `action_type = DELIVERY` is a POD subject. A `PICKUP` action is not a POD subject.

| Field | Role |
| --- | --- |
| `operating_tenant_id` | Tenant scope. Server-derived. Part of the address so a decision cannot be read across tenants. Not a client choice |
| `execution_id` | `transport.transport_executions.id`. The route aggregate that owns the stop |
| `execution_action_id` | Stable action row. This is the delivery subject. It does not change when a successor revision links the same action |
| `execution_stop_id` | Provenance. The parent stop. Copied onto the decision for readers. Not a second subject |
| `execution_revision_id` | Provenance. The revision current when a known decision is recorded. Membership links do not move the action |
| `shipment_id` | Provenance. Must match the action row. A shipment boolean is not the decision |
| `cargo_id` | Provenance. Must match the action row. Two deliveries of one cargo are two action ids |
| `shipment_tenant_id` | Participant attribute on the action. It does not replace `operating_tenant_id` |
| `source_route_plan_action_id` | Plan correlation on `transport_execution_revision_actions`. A successor link may record a different plan action id. It is not the decision subject |

`source_action_id` on `transport.delivery_disposition_cases` and `transport.delivery_attempt_facts` is the same execution action id. Disposition identity and POD identity meet on that action. They stay different facts. Quantity rejection does not set the POD decision.

## Decision model

The canonical value is not a bool.

```text
DECISION_MODEL=decision_state KNOWN|UNKNOWN plus nullable pod_required
KNOWN_REQUIRED=decision_state KNOWN and pod_required true
KNOWN_NOT_REQUIRED=decision_state KNOWN and pod_required false
UNKNOWN=decision_state UNKNOWN and pod_required null
```

Illegal combinations fail closed and are not stored: `KNOWN` with null `pod_required`, and `UNKNOWN` with a non-null `pod_required`.

`required_document_type` uses the existing document-service type token `POD` only on `KNOWN_REQUIRED`. It is null on `KNOWN_NOT_REQUIRED`. It is null on `UNKNOWN`. Null plus `UNKNOWN` means unresolved. It does not mean the type is optional, and it does not mean an empty required set.

```text
UNKNOWN_TO_FALSE=NO
UNKNOWN_TO_ZERO=NO
UNKNOWN_TO_NOT_REQUIRED=NO
MISSING_ROW_IS_FALSE=NO
MISSING_ROW_IS_UNKNOWN=YES
```

Agent B maps a TMS decision onto its requirement vocabulary as follows. This mapping is the TMS side of the composition. It does not authorize document-service to write the TMS row.

| TMS decision | Agent B requirement state for type `POD` |
| --- | --- |
| `KNOWN_REQUIRED` | `REQUIRED` |
| `KNOWN_NOT_REQUIRED` | `NOT_APPLICABLE` |
| `UNKNOWN` or missing row | `UNKNOWN` |

`KNOWN_NOT_REQUIRED` is not `OPTIONAL`. A missing TMS row is not an empty required set.

## Conceptual decision record

No table is created in this stage.

```text
MIGRATION_NUMBER=UNASSIGNED
```

A future server-owned record, one current version per delivery action, contains:

| Field | Rule |
| --- | --- |
| `pod_requirement_decision_id` | Server id of this version row |
| `decision_version` | Integer, starts at 1. Immutable once written |
| `operating_tenant_id` | Server-derived execution tenant |
| `execution_id` | Canonical identity |
| `execution_action_id` | Canonical identity. Must be a `DELIVERY` action on that execution |
| `execution_revision_id` | Revision observed when this version was decided. Null while `UNKNOWN` has no stored row |
| `execution_stop_id` | Stop that parents the action |
| `shipment_id` | Copied from the action |
| `cargo_id` | Copied from the action |
| `decision_state` | `KNOWN` or `UNKNOWN` |
| `pod_required` | `true`, `false`, or null, constrained as above |
| `required_document_type` | `POD` only when `KNOWN_REQUIRED`. Otherwise null |
| `requirement_decided_at` | Server time a `KNOWN` version was recorded. Null for `UNKNOWN` |
| `business_effective_at` | Effective instant of the source policy, when that policy has one. Null when unknown. Not `requirement_decided_at` |
| `source_policy_id` | Null until an authoritative policy source exists |
| `source_policy_version` | Null until that source versions a rule |
| `source_rule_id` | Null until that source names a rule |
| `decision_actor_class` | `SYSTEM` for a future policy writer. Not a browser user |
| `created_at` | Insert time of this version row |
| `supersedes_decision_id` | Prior version id when this row is an explicit successor version. Null on the first version |

A stored `UNKNOWN` version is optional. The internal read treats a missing row as `UNKNOWN` with null decision id, null version, null `pod_required`, null document type, and null `requirement_decided_at`. Writers must not insert a `KNOWN` row to fill those nulls.

## What decides POD_REQUIRED

No source in this repository can produce a known answer.

| Candidate | Classification | Evidence |
| --- | --- | --- |
| Shipment policy | `ABSENT` | No shipment column or command states a POD requirement |
| Transport order policy | `ABSENT` | Order execution view has readiness and provenance. No POD rule |
| Customer or shipper requirement | `ABSENT` | No customer configuration states POD required |
| Commercial contract or rate | `ABSENT` | No contract or rate field carries this decision. A contract object is not authority by itself |
| Tenant configuration | `ABSENT` | No tenant policy states POD required |
| Delivery execution action | `NOT_AUTHORITY` | The action row identifies the subject and stores status. It does not store a requirement |
| `RequiresPOD` milestone flag | `NOT_AUTHORITY` | Hardcoded false. UI metadata |
| `ShipmentExecutionView.PODRequired` | `NOT_AUTHORITY` | Unused zero-value bool |
| POD document row | `NOT_AUTHORITY` | Evidence owned by document-service. Presence is not requirement |
| `CUSTOMER_REFUSAL` and other disposition reasons | `NOT_AUTHORITY` | Cargo disposition. Separate from requirement |
| Shipment status `DELIVERED` or `DELIVERY_CONFIRMED` | `NOT_AUTHORITY` | Status is not a requirement |
| Explicit operator or admin decision | `ABSENT` | No privileged command exists. This stage does not authorize one |
| Future server-owned policy that names this delivery action | `CANDIDATE_FUTURE_SOURCE` | Not present. A later stage must show the rule before a known writer is authorized |

```text
AUTHORITATIVE_KNOWN_POLICY_SOURCE=NONE
SELECTED_POLICY_SOURCE_ARCHITECTURE=B_PERSIST_IMMUTABLE_TMS_DECISION
```

Architecture choices:

| Option | Result |
| --- | --- |
| A. Derive the requirement from current shipment or order fields | Rejected. Those fields do not contain the rule. Deriving it would invent one, and a later field change would rewrite history |
| B. Persist an immutable TMS decision produced by an authoritative policy source | Selected shape. The record above is the future fact. Today the only honest value is `UNKNOWN`, because the policy source is absent |
| C. Hardcode the requirement by milestone or action type | Rejected. `RequiresPOD: false` is already this failure mode |
| D. Client or browser submits `POD_REQUIRED` | Rejected. Client truth is not authority |

Option B does not mean a known writer can run now. The persistence shape is frozen. The rule that would set `KNOWN_REQUIRED` or `KNOWN_NOT_REQUIRED` is not.

## Decision timing

These clocks stay distinct:

| Clock | Meaning |
| --- | --- |
| `business_effective_at` | When the source policy is effective, if a versioned policy exists |
| `requirement_decided_at` | When TMS records a `KNOWN` decision version |
| `transport_execution_actions.completed_at` | When that delivery action completed |
| Agent B `evaluated_at` | When document-service composes requirement and evidence |

`requirement_decided_at` is not completion time and not processing time.

A decision becomes authoritative only when a future server-owned policy source emits a known result for that action. Materializing an execution does not create a known decision. Driver arrival does not create one. Action completion does not create one. If the source cannot prove a known result at the time it is asked, the result stays `UNKNOWN` and `requirement_decided_at` stays null. No timestamp is fabricated for an unknown decision.

Existing actions are not backfilled. The following are not evidence of a known decision:

- `RequiresPOD: false`
- a POD document row
- a missing POD document
- status `DELIVERED` or `DELIVERY_CONFIRMED`
- `CUSTOMER_REFUSAL` or any other disposition reason

## Historical replay

```text
POD_REQUIREMENT_HISTORICAL_REPLAY=VERSION_PINNED
HISTORICAL_REPLAY_AVAILABLE=NO
```

`HISTORICAL_REPLAY_AVAILABLE=NO` describes current runtime. The future contract is version-pinned. A later customer, tenant, or contract change must not update an existing decision version.

Replay of a past delivery uses the stored decision id and version, the stored `source_policy_id` and `source_policy_version`, `business_effective_at`, and `requirement_decided_at`. If those cannot be proven, the replay result is `UNKNOWN`. It is not today's policy.

Agent B evaluates satisfaction against that pinned TMS version. Agent B does not re-select a policy by its own `EFFECTIVE_FROM` window for POD. The EDO 0.1A effective-window rule still applies to policies EDO owns. It does not replace this pin. That difference is recorded under cross-domain gaps.

## Multi-stop

Delivery action A and delivery action B on the same execution are independent. A known result on one does not write the other. A shipment-level field must not overwrite them.

A completed action keeps its decision version. Completion makes that version historical evidence. It does not change `pod_required`.

## Replan and successor

Accepted successor semantics stay in force. Completed and in-service stop and action rows keep their ids and their parent execution. A successor revision adds membership links. It does not re-parent the row and does not rewrite a decision stored for that action.

```text
SUCCESSOR_REWRITES_COMPLETED_REQUIREMENT=NO
NEW_SUCCESSOR_POD_REQUIREMENT=UNKNOWN
```

| Situation | POD requirement |
| --- | --- |
| Completed delivery action | Same `execution_action_id`. Decision version stays. Successor `INHERITED_COMPLETED` link does not copy or change it |
| `ARRIVED` or `SERVICE_STARTED` stop, action id preserved | Same action id. Decision stays on that id. Replan does not open a second subject for the same action |
| Planned stop superseded, action `CANCELLED` | The cancelled action keeps its own decision, usually `UNKNOWN`. The new introduced delivery action is a new `execution_action_id` |
| Skipped stop | Skip does not set `pod_required`. Any action that remains addressable keeps its own decision |
| Return destination | New successor delivery action. Own decision. Starts `UNKNOWN` |
| Redirect destination | New successor delivery action. Own decision. Starts `UNKNOWN` |
| Hold | No new delivery action. Source decision unchanged. Hold quantity is disposition |
| Customer refusal | Reason code only. Source decision unchanged |

A new successor delivery action does not inherit the source action's requirement. Inheritance is not proven by any current policy. The new action stays `UNKNOWN` until a future policy source evaluates that new action.

```text
DECISION_IMMUTABILITY_CUTOFF=ACTION_TERMINAL
```

Action statuses `COMPLETED`, `FAILED`, and `CANCELLED` are terminal. The current decision version for that action, including a missing row read as `UNKNOWN`, cannot be replaced after the action is terminal. Before the action is terminal, a future authoritative policy event may append a new version that sets `supersedes_decision_id`. That event updates nothing on the previous version: not `pod_required`, not source policy version, not `requirement_decided_at`.

## Return and redirection

TMS return already records rejection on the source action and can create a successor revision for return or redirect. Those flows stay separate from this requirement.

```text
RETURN_REDIRECT_AUTO_INHERITANCE=NO
CUSTOMER_REFUSAL_DETERMINES_POD_REQUIRED=NO
```

The source delivery requirement stays on the source delivery action. The return destination is not the same decision. The redirect destination is not the same decision. `CUSTOMER_REFUSAL` does not mean required and does not mean not required.

## Internal read

This route is not implemented and is not added to OpenAPI.

```text
INTERNAL_READ_CONTRACT_FROZEN=YES
METHOD=GET
PATH=/internal/v1/transport-executions/{executionId}/actions/{actionId}/pod-requirement
```

The path follows existing shipment-service internal execution routes under `/internal/v1/transport-executions/{executionId}/...`.

Authentication is the existing `internalauth` middleware: header `X-Internal-Service-Token`. A missing or wrong token is HTTP 401 `UNAUTHORIZED`. A browser session is not a caller.

Tenant is the verified `X-Tenant-ID` already required by `resolveVerifiedTenant` on internal execution handlers. The server accepts that header only after internal authentication, and it must equal `transport.transport_executions.operating_tenant_id` for the execution. Query or body tenant is ignored. The caller cannot set `pod_required`, `decision_version`, source policy identity, or `requirement_decided_at` on this read.

Same-tenant response shape:

| Field | Missing decision row | Stored version |
| --- | --- | --- |
| `operating_tenant_id` | Server tenant | Stored |
| `execution_id` | Path id | Stored |
| `execution_action_id` | Path id | Stored |
| `execution_stop_id` | From the action row | Stored |
| `execution_revision_id` | Null | Stored pin, nullable |
| `shipment_id` | From the action row | Stored |
| `cargo_id` | From the action row | Stored |
| `pod_requirement_decision_id` | Null | Stored |
| `decision_version` | Null | Stored |
| `decision_state` | `UNKNOWN` | Stored |
| `pod_required` | Null | Stored |
| `required_document_type` | Null | Stored |
| `requirement_decided_at` | Null | Stored |
| `business_effective_at` | Null | Stored |
| `source_policy_id` | Null | Stored |
| `source_policy_version` | Null | Stored |
| `source_rule_id` | Null | Stored |
| `decision_actor_class` | Null | Stored |

HTTP semantics:

| Status | When |
| --- | --- |
| 200 | The action is a `DELIVERY` action on that execution in the caller tenant. Body is the table above. `UNKNOWN` is 200, not an error |
| 401 | Internal token missing or wrong |
| 404 | Unknown execution, unknown action, or the execution belongs to another operating tenant. One body. No field that distinguishes those cases |
| 409 | Same tenant, and the action exists but is not `action_type = DELIVERY`. Reason `ACTION_NOT_DELIVERY`. No foreign ids |
| 503 | Execution store unavailable. Body must not contain `pod_required=false` or `decision_state=KNOWN` |

The read does not use 403 for a cross-tenant id. Existing execution commands can return `TENANT_DENIED` as 403 when a caller names a mismatched tenant on a command. This read must not copy that distinction. A 403 would tell the caller that the delivery exists.

A 503 leaves the caller's composition at `UNKNOWN`. It is not a stored false decision.

## Composition, analytics, security

```text
TMS_POD_REQUIREMENT_DECISION + EDO_CANONICAL_CURRENT_DOCUMENT_EVIDENCE = POD_SATISFACTION
CROSS_DOMAIN_WRITE_REQUIRED=NO
```

TMS returns only the requirement decision. Document-service returns evidence and evaluates satisfaction. TMS does not ask document-service and then store `POD_SATISFIED`. Document-service does not update TMS decision rows.

Analytics, when a later stage authorizes it, consumes the finalized EDO satisfaction output. Analytics does not read the future TMS decision table, does not count `document_type = POD`, and does not infer the requirement from shipment status. This stage does not change Analytics formulas.

```text
TENANT_SERVER_DERIVED=YES
CLIENT_SUPPLIED_TENANT_AUTHORITY=NO
CROSS_TENANT_READ=DENY
CROSS_TENANT_READ_FAIL_CLOSED=YES
FOREIGN_DELIVERY_DISCLOSURE=NO
```

No client, including a future admin UI, may set tenant, `pod_required`, `decision_version`, source policy identity, or `requirement_decided_at` under this contract. A privileged admin writer would need a separate authorization. It is not included here.

## Future writes

Writes are not implemented. When a later stage authorizes them, they follow the execution command pattern: an idempotency key scoped by `operating_tenant_id`, plus a canonical request digest.

| Case | Result |
| --- | --- |
| Same source event and same digest | Return the original decision version. No second row |
| Same source event and a different digest | 409 conflict. Fail closed. Do not update the stored version |
| New authoritative policy event before the action is terminal | Insert a new version. Set `supersedes_decision_id`. Leave the previous version unchanged |
| Any write after the action is `COMPLETED`, `FAILED`, or `CANCELLED` | Reject. The pinned version stays |

There is no historical backfill job.

## Implementation slices

| Slice | Owner | Depends on | What it may do |
| --- | --- | --- | --- |
| `C-FOUNDATION` | Agent C / shipment-service | This contract and a later implementation authorization | Persist the decision model and serve the internal read. May record or return `UNKNOWN` before a policy source exists. Must not write `KNOWN` from `RequiresPOD`, documents, or status |
| `C-POLICY-SOURCE` | Agent C / shipment-service | A real server-owned rule that can prove `KNOWN_REQUIRED` or `KNOWN_NOT_REQUIRED` | The known-decision writer. Not implementable on current evidence |
| `B-INTEGRATION` | Agent B / document-service | Live TMS internal read | Compose the TMS decision with canonical document evidence. Must not invent the requirement |
| `E-INTEGRATION` | Agent E / Analytics | Finalized EDO satisfaction or completeness output | Consume that output. Must not read TMS tables or count POD rows |

Order is `C-FOUNDATION`, then `B-INTEGRATION` for `UNKNOWN` composition once the read exists, then `C-POLICY-SOURCE` when a rule exists, then known satisfaction in `B-INTEGRATION`, then `E-INTEGRATION`. Agent C does not implement the EDO or Analytics slices.

```text
TMS_POD_DECISION_FOUNDATION_IMPLEMENTABLE_NOW=YES
TMS_POD_KNOWN_DECISION_WRITER_IMPLEMENTABLE_NOW=NO
TMS_POD_INTERNAL_READ_IMPLEMENTABLE_NOW=YES
EDO_POD_COMPOSITION_IMPLEMENTABLE_NOW=NO
GAP_C_002_FULL_RUNTIME_IMPLEMENTABLE_NOW=NO
```

Foundation and the internal read are implementable because both can return `UNKNOWN` without a policy source. The known writer is not implementable. EDO composition is not implementable until the read exists at runtime. Full GAP-C-002 runtime needs the known writer and the EDO composition, so it is not implementable now. This document does not authorize any of those slices to start.

## Cross-domain gaps

Agent B's 0.1A contract is not edited here. These items need controller reconciliation:

```text
CROSS_DOMAIN_CONTRACT_GAP=EDO_0_1A_DELIVERY_SUBJECT_UNBOUND;EDO_0_1A_POD_REPLAY_MUST_USE_TMS_PIN;KNOWN_NOT_REQUIRED_MAPS_TO_NOT_APPLICABLE
```

- 0.1A names the POD subject `DELIVERY` and leaves stop and action as future-supported. This contract binds that subject to `transport.transport_execution_actions.id` with `action_type = DELIVERY`.
- 0.1A replays EDO policy versions by `EFFECTIVE_FROM` / `EFFECTIVE_TO`. For POD, Agent B must consume the TMS decision version pin and must not recompute the requirement from a later policy window.
- 0.1A does not yet map `KNOWN_NOT_REQUIRED` to `NOT_APPLICABLE`. This contract does. It must not be mapped to `OPTIONAL` or to an empty required set chosen by document-service.

Ownership is unchanged: TMS decides the requirement, EDO evaluates evidence.

## Future acceptance tests

These tests are `FUTURE_REQUIRED`. This stage does not add them.

| Id | Assertion |
| --- | --- |
| POD-REQ-01 | One delivery action holds one versioned decision |
| POD-REQ-02 | Two delivery actions on one shipment keep independent requirements |
| POD-REQ-03 | `KNOWN_REQUIRED` stays required |
| POD-REQ-04 | `KNOWN_NOT_REQUIRED` stays not required |
| POD-REQ-05 | `UNKNOWN` is not false |
| POD-REQ-06 | A missing row is not false |
| POD-REQ-07 | `RequiresPOD: false` does not become policy |
| POD-REQ-08 | A POD document does not make the requirement true |
| POD-REQ-09 | A missing POD document does not make the requirement false |
| POD-REQ-10 | A client-supplied tenant is not authority |
| POD-REQ-11 | A cross-tenant lookup returns the same 404 as a missing delivery and leaks no foreign id |
| POD-REQ-12 | The source policy id and version are pinned on a known decision |
| POD-REQ-13 | A later policy does not rewrite a historical decision version |
| POD-REQ-14 | Replaying the same source event returns the same decision and does not insert a duplicate |
| POD-REQ-15 | The same source identity with a different body fails closed |
| POD-REQ-16 | A successor revision does not rewrite the completed source delivery's decision |
| POD-REQ-17 | A successor delivery action receives its own decision |
| POD-REQ-18 | Return and redirect do not inherit the source requirement |
| POD-REQ-19 | `CUSTOMER_REFUSAL` does not set `POD_REQUIRED` |
| POD-REQ-20 | A dependency failure is 503 and does not become known false |

```text
ACCEPTANCE_MATRIX_FROZEN=YES
```

## Non-authorization

```text
IMPLEMENTATION_AUTHORIZED=NO
PRODUCT_CODE_CHANGED=NO
MIGRATION_CREATED=NO
OPENAPI_CHANGED=NO
GATEWAY_CHANGED=NO
STAGING_CHANGED=NO
GAP_C_002_RUNTIME_IMPLEMENTATION_AUTHORIZED=NO
```

`OPS_POD_COMPLETENESS` and `CAR_POD_COMPLETENESS` stay unpublished. GAP-B-001 runtime stays with Agent B and is not started here.
