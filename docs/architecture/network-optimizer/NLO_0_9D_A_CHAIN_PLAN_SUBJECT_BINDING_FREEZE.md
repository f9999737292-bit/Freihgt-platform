# NLO-0.9D-A chain plan and execution-subject binding freeze

```text
AGENT=D
STAGE=NLO-0.9D-A-PR276-CONTROLLER-REMEDIATION-R1
BASE_SHA=028bdba38d8e925969fca96063d8a553fde8b1ea
RESYNC_MAIN=6f7ba7e0fe65beefde8989cf64105387e51730bd
IMPLEMENTATION_AUTHORIZED=NO
MIGRATION_CREATED=NO
MIGRATION_000098_MODIFIED=NO
READY_FOR_0_9D_RUNTIME=NO
MULTI_LOAD_STATUS_POLICY_DEFINED=NO
D276_001_CLOSED=YES
D276_002_CLOSED=YES
D276_003_CLOSED=YES
CHAIN_ROUTE_PLAN_DEPENDENCY_PARITY=YES
MATERIALIZATION_USES_POST_SCORING_REREAD_AS_HISTORICAL_PROVENANCE=NO
FULL_CHAIN_COMPATIBILITY_PARITY=YES
CURRENT_CODE_COMPATIBILITY_PARITY=NO
NLO_0_9D_CURRENT_TRIP_EXECUTION_POLICY=DEPOT_START_ONLY
CHAIN_CURRENT_TRIP_PRESERVES_EXISTING_ONBOARD_ROUTE=NO
CHAIN_CURRENT_TRIP_INCLUDES_ONBOARD_GROUPAGE=NO
CHAIN_CURRENT_TRIP_ROUTE_PLAN_CAN_PROJECT_SEED_SHIPMENT=NO
CURRENT_TRIP_EXECUTION_MATERIALIZATION_READY=NO
CURRENT_TRIP_EXECUTION_MATERIALIZATION_FROZEN=YES
DEPOT_START_EXECUTION_GRADE_OUTCOME_POSSIBLE=YES
BLOCKER_UNPINNED_ROUTE_PLAN=CLOSED
BLOCKER_EXECUTION_SUBJECT_MATERIALIZATION=CLOSED
BLOCKER_CARRIER_TENANT_STAMPING=CLOSED
READY_FOR_MULTI_LOAD_STATUS_POLICY_DISCOVERY=YES
```

This freeze is the contract for a later runtime. It does not implement that runtime and does not change NLO-0.9B. Current chain code still drops `routeplan.Outcome` after scoring and still plans from a reduced `VisibleLoad`. `CURRENT_CODE_COMPATIBILITY_PARITY=NO` records that gap. `FULL_CHAIN_COMPATIBILITY_PARITY=YES` is the required future input, not a claim that the code already matches. Runtime stays unauthorized. Migration `000098` is unchanged.

## Score-time materialization

```text
CHAIN_CANDIDATE_ROUTE_PLAN_MATERIALIZATION=PERSIST_SELECTED_ORDER_EXECUTION_GRADE_OUTCOME_AND_PROVENANCE
MATERIALIZATION_USES_POST_SCORING_REREAD_AS_HISTORICAL_PROVENANCE=NO
CLIENT_SUPPLIES_ROUTE_PLAN_ID=NO
CLIENT_SUPPLIES_LOAD_IDS_AGAIN=NO
CLIENT_SUPPLIES_PLAN_VERSION=NO
ONE_CANDIDATE_ONE_PLAN=YES
SECOND_PROVIDER_CALL=NO
POST_SCORE_SILENT_REPLAN=NO
```

Rejected:

- Re-running generic `EvaluateRoutePlan` with the pair. That call chooses its own order unless already forced, calls the routing provider again, and is a different provenance.
- Re-planning the selected order at accept time.
- Reading the load, catalog, rule set, service-duration policy, vehicle, or current-trip context again after `Planner.Plan` returns and recording that later read as what the score used.

Required operation, not implemented:

During the same search compute that calls `Planner.Plan` for the selected order, retain an `ExecutionGradeMaterialization` before `scoreOrder` drops the outcome. `buildRoutePlanGraph` for that candidate consumes only this bundle. The graph insert and the candidate insert commit or roll back together. `chain_candidates.route_plan_id` is set on that insert. The client does not send the search id, candidate id, load ids, versions, selected order, or seed as a substitute plan.

A feasible candidate whose insert does not include this bundle is not execution-ready. Existing rows with a null `route_plan_id` stay not execution-ready. They are not backfilled.

Activation may later re-read canonical shipment, cargo, policy, and vehicle rows and refuse a stale plan. That live check is not evidence of the earlier score.

The bundle must be sufficient to create exactly:

| Field | Score-time source |
| --- | --- |
| `routeplan.Outcome` | The `Plan` return for the selected order, including stops, actions, legs, snapshots, result status, and reason codes |
| `planning_mode` | The `routeplan.Input.Mode` passed into that `Plan` |
| Seed shipment id and version | Only when `planning_mode=CURRENT_TRIP`, copied from the seed already resolved before `Plan`. 0.9D execution does not use this mode |
| Seed capacity id and version | When `planning_mode=DEPOT_START`, copied from the capacity row already resolved before `Plan` |
| `current_trip_context_fingerprint` | Only for `CURRENT_TRIP`, the fingerprint already on that pre-plan context. 0.9D execution does not use this mode |
| Vehicle id and version | Present only when the routing profile passed into `Plan` came from a canonical vehicle. Copied from that pre-plan pin. Absent vehicle stays null |
| `LOAD_OPPORTUNITY` dependency | Each planned load id and the exact version on the `routeplan.Load` passed into `Plan` |
| Onboard `SHIPMENT_CARGO` dependency | Cargo id, cargo version, and the evidence state, evidence version, and evidence time already on the onboard action passed into `Plan`. 0.9D execution has none |
| `SERVICE_DURATION_POLICY` | Policy id, version, and the duration fingerprint of the pickup and delivery seconds on the `Input` passed into `Plan` |
| `CATALOG` fingerprint | Hash of the catalog versions returned by the groupage calls inside that `Plan`, accumulated the same way `prepareRoutePlan` accumulates `CatalogVersionsUsed` |
| `RULE_SET` fingerprint | Hash of the rule-set refs returned by those same groupage calls |
| Routing policy version | `routeplan.RoutingPolicyVersion` of that planner build |
| Algorithm policy version | `routeplan.AlgorithmPolicyVersion` of that planner build |
| Public subject snapshot | `marshalRoutePlanPublicLoadSnapshot` of the load row whose version was passed into `Plan`, taken before `Plan` returns |

`routeDependencies` for the chain graph uses only those pins. It does not call `GetLoad`, `ActiveServiceDurationPolicy`, `catalog.Evaluation`, or the current-trip builder after `Plan`.

```text
CHAIN_CANDIDATE_ROUTE_PLAN_VERSION_POLICY=ID_SUFFICIENT_VERSION_INSERT_ONLY
CANDIDATE_PLAN_BINDING_ATOMIC=YES
CANDIDATE_PLAN_MATERIALIZATION_IDEMPOTENCY=CANDIDATE_PRIMARY_KEY_AND_FINGERPRINT
BOUND_CANDIDATE_MUTABLE=NO
SELECTED_ORDER_MUTABLE_AFTER_BIND=NO
```

No `route_plan_version` column is added. Accept and activate read `route_plans.version` from the bound id. Current writers never change that integer. Status timestamps may change. The graph body may not. A successor plan is a new id.

The transaction is the existing candidate-insert transaction extended with the graph insert.

| Case | Required result |
| --- | --- |
| PLAN_BIND_01 crash before insert | No plan row and no candidate row |
| PLAN_BIND_02 graph inserted, candidate missing | Impossible inside this transaction. A commit that lacks the candidate rolls back the graph |
| PLAN_BIND_03 same candidate inserted twice | Primary key conflict. No second plan |
| PLAN_BIND_04 same candidate, different graph body | Conflict. The stored fingerprint and plan id stay |
| PLAN_BIND_05 load version differs from the candidate version at insert | Insert refused. No partial graph |
| PLAN_PROV_01 context changes after score before persistence | The stored context is the pre-plan pin, not the later context |
| PLAN_PROV_02 rule or catalog changes after score | The stored fingerprints are the ones returned inside that `Plan` |
| PLAN_PROV_03 service-duration policy changes after score | The stored policy is the one on the scored `Input`. Activation may reject it as stale |
| PLAN_PROV_04 vehicle version changes after score | The stored vehicle pin is the pre-plan pin. No later version is written as the scored version |

All `PLAN_*` rows are `FUTURE_REQUIRED`.

Future schema shape, not migrated here:

- Unique `(tenant_id, route_plan_id)` where `route_plan_id` is not null, so one plan cannot bind two candidates.
- No `UPDATE` of `first_load_id`, `second_load_id`, their versions, `selected_order`, `candidate_fingerprint`, or `route_plan_id`.

## Dependency parity

Generic path: `prepareRoutePlan` to `Planner.Plan` to `buildRoutePlanGraph` to `routeDependencies`.

Future chain path: chain compute to the selected order to `ExecutionGradeMaterialization` to `RoutePlanGraph`.

`CHAIN_ROUTE_PLAN_DEPENDENCY_PARITY=YES` means that future graph's dependency rows have the same semantics as the generic rows, pinned from the scoring invocation. It does not mean current chain code already writes those rows.

| Generic field | Chain source | Same semantics | Pinned at score time | Activation revalidated | Gap |
| --- | --- | --- | --- | --- | --- |
| `planning_mode` | `Input.Mode` of that `Plan` | YES | YES | NO | Current chain does not store it on the candidate |
| Outcome stops, actions, legs, snapshots | Selected `routeplan.Outcome` | YES | YES | NO | `scoreOrder` drops the outcome today |
| `SHIPMENT` id and version | Pre-plan current-trip seed | YES for a future current-trip plan | YES | YES | Not in the 0.9D execution policy |
| `CURRENT_TRIP_CONTEXT` fingerprint | Pre-plan `InputFingerprint` | YES for a future current-trip plan | YES | YES, stale evidence refuses activation | Not in the 0.9D execution policy |
| `CAPACITY` id, version, fingerprint | Pre-plan depot capacity | YES | YES | YES | Current chain seed does not copy the id onto the outcome |
| `VEHICLE` id and version | Pre-plan canonical vehicle beside the routing profile | YES | YES | YES | `routing.VehicleProfile` has no id or version |
| `LOAD_OPPORTUNITY` id and version | `routeplan.Load` passed into `Plan` | YES | YES | YES, version mismatch refuses | Current candidate stores the pair version, not a dependency row |
| Onboard `SHIPMENT_CARGO` id, version, evidence | Onboard actions passed into `Plan` | YES for a future current-trip plan | YES | YES | Current `planOrder` passes no onboard actions |
| `SERVICE_DURATION_POLICY` id, version, fingerprint | Seconds and policy ids on the scored `Input` | YES | YES | YES, stale policy may refuse | Chain does not retain the policy on the outcome |
| `CATALOG` | Fingerprint of refs returned inside that `Plan` | YES | YES | NO | Chain `groupage` re-reads the catalog per call and does not accumulate refs. A separate pre-search hash is not this fingerprint |
| `RULE_SET` | Fingerprint of refs returned inside that `Plan` | YES | YES | NO | Same gap as `CATALOG` |
| `ROUTING_POLICY` | Planner constant used by that build | YES | YES | NO | Not stored on the chain candidate |
| `ALGORITHM_POLICY` | Planner constant used by that build | YES | YES | NO | Not stored on the chain candidate |
| Public subject snapshot | Snapshot bytes of the pinned load version | YES | YES | NO | Chain does not build `marshalRoutePlanPublicLoadSnapshot` |

## Cargo compatibility

Choice A is frozen. A selected chain result is execution-grade only when the `routeplan.Load` values passed into that `Plan` carry the same planning facts `actionFromLoad` and `cargoFromLoad` pass into generic evaluation. Choice B is rejected: a later generic evaluation would be a second plan, and it must not be labeled as the scored candidate.

Current code is not equal. `chainapi` builds `VisibleLoad` with id, version, owner, windows, points, stackable, and weight. `toPlanLoad` copies weight and stackable into `compat.Cargo` and sets missing weight to `0`. Generic `cargoFromLoad` keeps a nil weight nil and copies the remaining cargo facts. The future input is a `routeplan.Load` built from the pinned load row with those generic facts. This stage does not change `VisibleLoad`. An outcome that was scored without them is not execution-grade.

| Fact | Generic route plan consumes | Chain consumes today | Semantics equal |
| --- | --- | --- | --- |
| weight | `LoadOpportunity.WeightKg`, nil stays nil | `VisibleLoad.WeightKg`, nil becomes `0` | NO |
| volume | `VolumeM3` on the action and cargo | absent | NO |
| pallet_count | `Cargo.PalletCount` | absent | NO |
| pallet_type | `Cargo.PalletTypeCode` | absent | NO |
| linear_meters | `Cargo.LinearMeters` | absent | NO |
| max_loaded_height | `Cargo.MaxLoadedHeightMM` | absent | NO |
| stackable | `Cargo.Stackable` | `VisibleLoad.Stackable` | YES for this one field |
| fragile | `Cargo.Fragile` | absent | NO |
| packaging | `Cargo.PackagingTypeCode` | absent | NO |
| temperature required, min, max, setpoint | cargo temperature fields | absent | NO |
| dangerous goods | `Cargo.Dangerous` | absent | NO |
| hazard classes | `Cargo.HazardClasses` | absent | NO |
| food-grade | `Cargo.FoodGradeRequired` | absent | NO |
| body type | `RequiredBodyTypes` and load `BodyType` through access and the public snapshot | absent from `toPlanLoad` | NO |
| loading access | required and allowed loading access | absent | NO |
| unloading access | required and allowed unloading access | absent | NO |
| odor | emission class and odor sensitive | absent | NO |
| contamination | `ContaminationClass` | absent | NO |

Chain `groupage` also calls `EvaluateGroupageItems` with an empty `compat.Equipment`. Generic depot evaluation uses `equipmentFromCapacity`. Generic current-trip evaluation uses `equipmentFromVehicle`. Temperature, ADR, food-grade, pallet, and access checks that read equipment therefore do not see the vehicle or capacity equipment during chain scoring. The future depot-start `Plan` must use the same equipment mapping `prepareRoutePlan` uses for `DEPOT_START`. A nil groupage callback must not become `StatusCompatible`. More than one owner, a missing catalog, or a failed catalog evaluation sets `ReferenceUnavailable`, and that outcome is `INDETERMINATE`.

`PLAN_COMPAT_01` through `PLAN_COMPAT_05` are `FUTURE_REQUIRED`. A selected execution-grade outcome must be infeasible, or explicitly indeterminate, when the generic evaluator would reject that pair for temperature, ADR, food-grade, pallet, or odor and contamination. A chain score that never received the fact must not be stored as execution-grade.

## Current trip

```text
NLO_0_9D_CURRENT_TRIP_EXECUTION_POLICY=DEPOT_START_ONLY
CHAIN_CURRENT_TRIP_PRESERVES_EXISTING_ONBOARD_ROUTE=NO
CHAIN_CURRENT_TRIP_INCLUDES_ONBOARD_GROUPAGE=NO
CHAIN_CURRENT_TRIP_ROUTE_PLAN_CAN_PROJECT_SEED_SHIPMENT=NO
CURRENT_TRIP_EXECUTION_MATERIALIZATION_READY=NO
CURRENT_TRIP_EXECUTION_MATERIALIZATION_FROZEN=YES
```

Option A, putting the full current-trip base into the scored outcome, is not the 0.9D execution contract. Option B is frozen: the first execution runtime materializes `DEPOT_START` only. Current-trip chain search may continue as discovery. It must not grow a route plan, an accept, or an activation.

Generic `prepareRoutePlan` for `CURRENT_TRIP` puts the fresh position and `ObservedAt` on the start point, the destination on an end stop, confirmed onboard units on that end stop as `SHIPMENT_CARGO` deliveries, those units into `Input.Onboard` for groupage, the shipment id and version and context fingerprint onto the plan dependencies, and the vehicle id and version when the context has them. `planOrder` copies `Seed.Start` and `OnboardSubjects` only. It does not pass an end stop, onboard actions, or `Input.Onboard`. The confirmed count does not identify a cargo. `ResolveTrip` holds the shipment id and version on `ResolvedSeed`, and the planner outcome does not receive them.

| Fact | Generic current trip | Chain `planOrder` |
| --- | --- | --- |
| Start position | Fresh latitude and longitude, tracking source | `Seed.Start` point |
| Position `observed_at` | `PositionObservedAt` or position `RecordedAt` | `ObservedAt` is on `Seed.Start` when the resolver set it. It is not a retained plan dependency |
| Seed shipment id and version | Plan dependencies | On `ResolvedSeed` only. Not on the outcome |
| Context fingerprint | `trip.InputFingerprint` | Not passed |
| Vehicle id and version | Dependency when the context has them | Routing profile only. No id or version |
| Residual capacity | Payload, volume, pallets, linear meters, height | Payload, volume, pallets, linear meters. Height is not on `chain.Capacity` |
| Confirmed onboard cargo | Delivery actions on the end stop | Count in `OnboardSubjects` only |
| Onboard cargo version and evidence | Action subject version, evidence state, version, and time | Absent |
| Onboard compatibility | Those cargos enter groupage | No onboard groupage items |
| Existing delivery and end | End stop is the trip destination | No end stop |
| `SHIPMENT_CARGO` actions | One delivery per confirmed unit | None |
| Additional load actions | `LOAD_OPPORTUNITY` pickup and delivery | `LOAD_OPPORTUNITY` pickup and delivery for the two loads only |

`CURRENT_TRIP_01` through `CURRENT_TRIP_04` are `FUTURE_REQUIRED` for a later wave that chooses Option A. They are not 0.9D execution tests. A current-trip candidate must not be given `route_plan_id`.

## Depot start

```text
DEPOT_START_EXECUTION_GRADE_OUTCOME_POSSIBLE=YES
```

Depot start has no onboard shipment and no end stop in the generic evaluator. After the score-time bundle and the cargo facts above are actually inputs to the selected `Plan`, that outcome can be the execution-grade plan. No second provider call is allowed.

Remaining implementation blockers, not new product decisions:

- Chain scoring still omits the cargo facts in the table above.
- Chain groupage still uses empty equipment and does not retain the catalog and rule refs returned inside `Plan`.
- The selected outcome is still discarded.
- Vehicle id and version are not pinned beside the routing profile.
- The public subject snapshot is not taken from the pinned load version.
- Which shipment statuses may be pinned is still undecided. That is the next discovery, not a second routing evaluation.

## Execution subjects

```text
EXECUTION_SUBJECT_PIN_OWNER=ROUTE_PLAN_ACTION
ACTIVATION_REVALIDATES_EXECUTION_SUBJECTS=YES
PUBLIC_CHAIN_RESPONSE_GAINS_INTERNAL_SUBJECT_IDS=NO
DEPOT_START_REQUIRES_ALL_LOADS_MATERIALIZED=YES
SOURCE_SHIPMENT_EXECUTION_BINDING=SOURCE_ID_IS_SHIPMENT_ID
SOURCE_TRANSPORT_ORDER_EXECUTION_BINDING=RESOLVE_EXISTING_SHIPMENT_OR_REFUSE
SHIPMENT_TENANT_SOURCE=CANONICAL_SHIPMENT_OWNER
OPERATING_TENANT_SOURCE=CALLER_CARRIER_TENANT
```

The candidate row does not gain shipment or cargo ids. The route-plan action is the pin. The cargo id and cargo version on a future action are the values resolved from the load version already inside the score-time bundle. Activate reads the canonical source again and refuses the plan when the live shipment version, cargo id, or cargo version differs from that pin. The later read does not replace the stored pin.

`ROUTE_ACTION_EXECUTION_IDENTITY_COMPLETE=NO` today. Future columns, not a migration in this stage:

- `source_shipment_tenant_id`
- `cargo_id`
- `cargo_version`

`subject_id` stays the load id for `LOAD_OPPORTUNITY` and the cargo id for `SHIPMENT_CARGO`. Those new columns are not aliases of `subject_id` or `subject_version`. `source_shipment_id` remains the shipment id. For a chain load, `source_shipment_id` and `cargo_id` are tied to the load id and version in the bundle. The shipment lookup below uses that pinned load, not a load row read after scoring.

### Shipment source

```text
CAN_RESOLVE_SHIPMENT_ID=YES
CAN_RESOLVE_SHIPMENT_OWNER_TENANT=YES
CAN_RESOLVE_SHIPMENT_VERSION=YES
CAN_RESOLVE_CARGO_ID=YES_WHEN_EXECUTION_CONTEXT_CARGO_ID_PRESENT
CAN_RESOLVE_CARGO_VERSION=YES_WHEN_CARGO_ID_PRESENT
CANONICAL_SOURCE=EXECUTION_CONTEXT_PLUS_CARGO_PLANNING_PROFILE
```

Reads use `load.owner_tenant_id`, never the carrier tenant. The load row is the one whose version was pinned before `Plan`:

1. `GET /internal/v1/shipments/{source_id}/execution-context`
2. Require non-null `cargo_id`. Otherwise the load is not an execution subject.
3. `GET /internal/v1/cargoes/{cargo_id}/planning-profile` for `version`, still under `owner_tenant_id`.

`source_id` on a `SHIPMENT` load is the shipment id.

### Transport-order source

```text
TRANSPORT_ORDER_LOAD_EXECUTION_POLICY=NO_SHIPMENT_MEANS_NOT_AN_EXECUTION_SUBJECT
CAN_RESOLVE_SHIPMENT_ID=ONLY_WHEN_ACTIVE_SHIPMENT_ROW_EXISTS
CAN_RESOLVE_SHIPMENT_OWNER_TENANT=ONLY_WHEN_THAT_ROW_EXISTS
CAN_RESOLVE_SHIPMENT_VERSION=ONLY_WHEN_THAT_ROW_EXISTS
CAN_RESOLVE_CARGO_ID=ONLY_FROM_THAT_SHIPMENT_EXECUTION_CONTEXT
CAN_RESOLVE_CARGO_VERSION=ONLY_FROM_THAT_CARGO_PLANNING_PROFILE
CANONICAL_SOURCE=uq_shipments_transport_order_active
```

`source_id` is not a shipment id. The lookup predicate is the existing unique index: one live `transport.shipments` row for `(owner_tenant_id, transport_order_id)`. No row means the load cannot be pinned as an execution subject. NLO does not call `POST /from-transport-order` and does not insert a shipment. The in-process read is `GetShipmentByTransportOrderID`. A later runtime may expose that predicate to NLO as an internal read. It may not change the predicate.

After the shipment id is found, cargo resolution is the shipment-source procedure above. Both lookups happen before `Plan` returns and are stored on the bundle. A missing shipment discovered only after scoring does not get a historical subject invented from the later read.

Depot-start has no seed shipment. Every chain load must pass this materialization or activation cannot project that plan. This freeze does not choose which shipment statuses are eligible. The shipments that a later status policy will check are the shipment ids on these pinned actions.

Current-trip execution is outside 0.9D. Discovery may still return a current-trip candidate. That candidate is not materialized.

## Tenants

```text
NLO_CARRIER_TENANT_STAMPING_VALID=NO
TMS_CROSS_SHIPPER_EXECUTION_SHAPE_PROVEN=YES
```

`actor.TenantID` is valid as the chain-search tenant, the route-plan tenant, and `operating_tenant_id`.

`actor.TenantID` is not valid as `ContextShipmentTenantID` or as a subject's `shipment_tenant_id` unless that shipment's execution-context `tenant_id` is actually the actor. The current `projectionCommand` assignments that copy `actor.TenantID` into those subject fields are an invalid substitution.

Future command:

| Field | Source |
| --- | --- |
| `operating_tenant_id` | carrier caller |
| `context_shipment_id` | null for depot-start |
| `context_shipment_tenant_id` | null for depot-start |
| participant `shipment_tenant_id` | that load's resolved shipment owner |
| `cargo_id`, `cargo_version` | the score-time pin, rechecked live on activate |

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
| SUBJECT_BIND_08 | Cargo version changed before activate | Activate refuses. The stored pin stays |
| SUBJECT_BIND_09 | Transport order with no live shipment | No execution subject, no shipment created |
| SUBJECT_BIND_10 | Two shippers | Distinct `shipment_tenant_id` values |
| SUBJECT_BIND_11 | Client sends `route_plan_id` | Rejected |
| SUBJECT_BIND_12 | Client sends shipment or cargo ids | Rejected |
| SUBJECT_BIND_13 | Crash inside the bind transaction | No plan and no candidate |
| SUBJECT_BIND_14 | Candidate already bound to another plan | Conflict |
| SUBJECT_BIND_15 | Pre-accept chain response | No new internal subject ids |
| PLAN_PROV_01 | Context changes after score, before persistence | Stored context is the score-time pin |
| PLAN_PROV_02 | Rule set or catalog changes after score | Stored fingerprints are the ones used inside `Plan` |
| PLAN_PROV_03 | Service-duration policy changes after score | Stored policy is the scored policy. Activation may reject it |
| PLAN_PROV_04 | Vehicle version changes after score | Stored vehicle pin is the pre-plan pin |
| PLAN_COMPAT_01 | Temperature incompatibility on the scored facts | Not stored as a feasible execution-grade plan |
| PLAN_COMPAT_02 | ADR incompatibility on the scored facts | Not stored as a feasible execution-grade plan |
| PLAN_COMPAT_03 | Food-grade incompatibility on the scored facts | Not stored as a feasible execution-grade plan |
| PLAN_COMPAT_04 | Pallet incompatibility on the scored facts | Not stored as a feasible execution-grade plan |
| PLAN_COMPAT_05 | Odor or contamination incompatibility on the scored facts | Not stored as a feasible execution-grade plan |
| CURRENT_TRIP_01 | Onboard cargo must be preserved | Not a 0.9D execution case. Current-trip materialization stays blocked |
| CURRENT_TRIP_02 | Onboard compatibility must participate | Not a 0.9D execution case |
| CURRENT_TRIP_03 | Seed shipment delivery must remain | Not a 0.9D execution case |
| CURRENT_TRIP_04 | Changed onboard evidence makes activation stale | Not a 0.9D execution case |
