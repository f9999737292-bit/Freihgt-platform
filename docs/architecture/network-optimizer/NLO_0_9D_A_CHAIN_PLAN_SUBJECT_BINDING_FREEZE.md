# NLO-0.9D-A chain plan and execution-subject binding freeze

```text
AGENT=D
STAGE=NLO-0.9D-A-CHAIN-PLAN-SUBJECT-BINDING-FREEZE
BASE_SHA=028bdba38d8e925969fca96063d8a553fde8b1ea
IMPLEMENTATION_AUTHORIZED=NO
MIGRATION_CREATED=NO
READY_FOR_0_9D_RUNTIME=NO
MULTI_LOAD_STATUS_POLICY_DEFINED=NO
BLOCKER_UNPINNED_ROUTE_PLAN=CLOSED
BLOCKER_EXECUTION_SUBJECT_MATERIALIZATION=CLOSED
BLOCKER_CARRIER_TENANT_STAMPING=CLOSED
READY_FOR_MULTI_LOAD_STATUS_POLICY_DISCOVERY=YES
```

This freeze is the contract for a later runtime. It does not implement that runtime and does not change NLO-0.9B.

## Plan materialization

```text
CHAIN_CANDIDATE_ROUTE_PLAN_MATERIALIZATION=PERSIST_SELECTED_ORDER_OUTCOME
CLIENT_SUPPLIES_ROUTE_PLAN_ID=NO
CLIENT_SUPPLIES_LOAD_IDS_AGAIN=NO
CLIENT_SUPPLIES_PLAN_VERSION=NO
ONE_CANDIDATE_ONE_PLAN=YES
```

Rejected:

- Re-running generic `EvaluateRoutePlan` with the pair. The generic planner does not keep the chain's selected order, and a new provider call is a different provenance.
- Re-planning only the selected order at accept time. The stored candidate does not contain the scored legs.

Required operation, not implemented:

During the search compute that still holds the selected order's `routeplan.Outcome`, build one `RoutePlanGraph` with the existing `buildRoutePlanGraph` rules (`version=1`, `execution_supported=false`) and insert it in the same transaction as the candidate. Set `chain_candidates.route_plan_id` on that insert. The server resolves search id, candidate id, both load ids and versions, `selected_order`, and the seed from the search row. The client sends none of those as a substitute plan.

A feasible candidate whose insert does not include a plan id is not execution-ready. Existing rows with a null `route_plan_id` stay not execution-ready. They are not backfilled by a later re-plan.

```text
CHAIN_CANDIDATE_ROUTE_PLAN_VERSION_POLICY=ID_SUFFICIENT_VERSION_INSERT_ONLY
```

No `route_plan_version` column is added. Accept and activate read `route_plans.version` from the bound id. Current writers never change that integer. Status timestamps may change. The graph body may not. A successor plan is a new id.

```text
CANDIDATE_PLAN_BINDING_ATOMIC=YES
CANDIDATE_PLAN_MATERIALIZATION_IDEMPOTENCY=CANDIDATE_PRIMARY_KEY_AND_FINGERPRINT
BOUND_CANDIDATE_MUTABLE=NO
SELECTED_ORDER_MUTABLE_AFTER_BIND=NO
```

The transaction is the existing candidate-insert transaction extended with the graph insert. Commit both or neither.

| Case | Required result |
| --- | --- |
| PLAN_BIND_01 crash before insert | No plan row and no candidate row |
| PLAN_BIND_02 graph inserted, candidate missing | Impossible inside this transaction. A commit that lacks the candidate rolls back the graph |
| PLAN_BIND_03 same candidate inserted twice | Primary key conflict. No second plan |
| PLAN_BIND_04 same candidate, different graph body | Conflict. The stored fingerprint and plan id stay |
| PLAN_BIND_05 load version differs from the candidate version at insert | Insert refused. No partial graph |

Future schema shape, not migrated here:

- Unique `(tenant_id, route_plan_id)` where `route_plan_id` is not null, so one plan cannot bind two candidates.
- No `UPDATE` of `first_load_id`, `second_load_id`, their versions, `selected_order`, `candidate_fingerprint`, or `route_plan_id`.

## Execution subjects

```text
EXECUTION_SUBJECT_PIN_OWNER=ROUTE_PLAN_ACTION
ACTIVATION_REVALIDATES_EXECUTION_SUBJECTS=YES
PUBLIC_CHAIN_RESPONSE_GAINS_INTERNAL_SUBJECT_IDS=NO
DEPOT_START_REQUIRES_ALL_LOADS_MATERIALIZED=YES
```

The candidate row does not gain shipment or cargo ids. The route-plan action is the pin. Activate reads the canonical source again and refuses the plan when the live shipment version, cargo id, or cargo version differs.

`ROUTE_ACTION_EXECUTION_IDENTITY_COMPLETE=NO` today. Future columns, not a migration in this stage:

- `source_shipment_tenant_id`
- `cargo_id`
- `cargo_version`

`subject_id` stays the load id for `LOAD_OPPORTUNITY` and the cargo id for `SHIPMENT_CARGO`. Those new columns are not aliases of `subject_id` or `subject_version`. `source_shipment_id` remains the shipment id.

### Shipment source

```text
SOURCE_SHIPMENT_EXECUTION_BINDING=SOURCE_ID_IS_SHIPMENT_ID
CAN_RESOLVE_SHIPMENT_ID=YES
CAN_RESOLVE_SHIPMENT_OWNER_TENANT=YES
CAN_RESOLVE_SHIPMENT_VERSION=YES
CAN_RESOLVE_CARGO_ID=YES_WHEN_EXECUTION_CONTEXT_CARGO_ID_PRESENT
CAN_RESOLVE_CARGO_VERSION=YES_WHEN_CARGO_ID_PRESENT
CANONICAL_SOURCE=EXECUTION_CONTEXT_PLUS_CARGO_PLANNING_PROFILE
```

Reads use `load.owner_tenant_id`, never the carrier tenant:

1. `GET /internal/v1/shipments/{source_id}/execution-context`
2. Require non-null `cargo_id`. Otherwise the load is not an execution subject.
3. `GET /internal/v1/cargoes/{cargo_id}/planning-profile` for `version`, still under `owner_tenant_id`.

### Transport-order source

```text
SOURCE_TRANSPORT_ORDER_EXECUTION_BINDING=RESOLVE_EXISTING_SHIPMENT_OR_REFUSE
TRANSPORT_ORDER_LOAD_EXECUTION_POLICY=NO_SHIPMENT_MEANS_NOT_AN_EXECUTION_SUBJECT
CAN_RESOLVE_SHIPMENT_ID=ONLY_WHEN_ACTIVE_SHIPMENT_ROW_EXISTS
CAN_RESOLVE_SHIPMENT_OWNER_TENANT=ONLY_WHEN_THAT_ROW_EXISTS
CAN_RESOLVE_SHIPMENT_VERSION=ONLY_WHEN_THAT_ROW_EXISTS
CAN_RESOLVE_CARGO_ID=ONLY_FROM_THAT_SHIPMENT_EXECUTION_CONTEXT
CAN_RESOLVE_CARGO_VERSION=ONLY_FROM_THAT_CARGO_PLANNING_PROFILE
CANONICAL_SOURCE=uq_shipments_transport_order_active
```

`source_id` is not a shipment id. The lookup predicate is the existing unique index: one live `transport.shipments` row for `(owner_tenant_id, transport_order_id)`. No row means the load cannot be pinned as an execution subject. NLO does not call `POST /from-transport-order` and does not insert a shipment. The in-process read is `GetShipmentByTransportOrderID`. A later runtime may expose that predicate to NLO as an internal read. It may not change the predicate.

After the shipment id is found, cargo resolution is the shipment-source procedure above.

Depot-start has no seed shipment. Every chain load must pass this materialization or activation cannot project that plan. This freeze does not choose which shipment statuses are eligible.

Current-trip keeps the seed shipment identity from its own execution context. Each additional load is resolved independently. The seed owner and a foreign load owner are not collapsed.

## Tenants

```text
OPERATING_TENANT_SOURCE=CALLER_CARRIER_TENANT
SHIPMENT_TENANT_SOURCE=CANONICAL_SHIPMENT_OWNER
NLO_CARRIER_TENANT_STAMPING_VALID=NO
TMS_CROSS_SHIPPER_EXECUTION_SHAPE_PROVEN=YES
```

`actor.TenantID` is valid as the chain-search tenant, the route-plan tenant, and `operating_tenant_id`.

`actor.TenantID` is not valid as `ContextShipmentTenantID` or as a subject's `shipment_tenant_id` unless that shipment's execution-context `tenant_id` is actually the actor. The current `projectionCommand` assignments that copy `actor.TenantID` into those subject fields are an invalid substitution.

Future command:

| Field | Source |
| --- | --- |
| `operating_tenant_id` | carrier caller |
| `context_shipment_id` | current-trip seed shipment, or null for depot-start |
| `context_shipment_tenant_id` | that seed's execution-context tenant |
| participant `shipment_tenant_id` | that load's resolved shipment owner |
| `cargo_id`, `cargo_version` | the pinned action, rechecked live |

Example: operating tenant `C`, load owners `S1` and `S2`. The execution row uses `C`. Participant 1 uses `S1`. Participant 2 uses `S2`. TMS verifies each shipment and cargo with its own tenant. No shipment tenant is rewritten to `C`.

Pre-accept chain responses do not gain `shipment_id`, `shipment_tenant_id`, `cargo_id`, customer identity, or contract or rate fields.

## Failure matrix

All rows are `FUTURE_REQUIRED`.

| Id | Condition | Required result |
| --- | --- | --- |
| SUBJECT_BIND_01 | Feasible candidate with null `route_plan_id` | Not execution-ready |
| SUBJECT_BIND_02 | Same candidate materialized twice | One plan, conflict on the second insert |
| SUBJECT_BIND_03 | Load version changed before insert | No plan |
| SUBJECT_BIND_04 | Shipment source id missing | No execution subject |
| SUBJECT_BIND_05 | Foreign shipment, load published to this carrier | Owner tenant preserved |
| SUBJECT_BIND_06 | Two owners | Projection uses each owner, not the carrier |
| SUBJECT_BIND_07 | Execution-context `cargo_id` null | No execution subject |
| SUBJECT_BIND_08 | Cargo version changed before activate | Activate refuses |
| SUBJECT_BIND_09 | Transport order with no live shipment | No execution subject, no shipment created |
| SUBJECT_BIND_10 | Two shippers | Distinct `shipment_tenant_id` values |
| SUBJECT_BIND_11 | Client sends `route_plan_id` | Rejected |
| SUBJECT_BIND_12 | Client sends shipment or cargo ids | Rejected |
| SUBJECT_BIND_13 | Crash inside the bind transaction | No plan and no candidate |
| SUBJECT_BIND_14 | Candidate already bound to another plan | Conflict |
| SUBJECT_BIND_15 | Pre-accept chain response | No new internal subject ids |
