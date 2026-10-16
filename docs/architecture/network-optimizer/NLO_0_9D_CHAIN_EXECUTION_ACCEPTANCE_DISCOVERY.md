# NLO-0.9D chain execution acceptance discovery

```text
AGENT=D
STAGE=NLO-0.9D-CHAIN-EXECUTION-ACCEPTANCE-DISCOVERY
BASE_SHA=4976e5d111ed2f284910c87baa7f83f3b88cc7d1
PRODUCT_CODE_CHANGED=NO
MIGRATION_CREATED=NO
OPENAPI_CHANGED=NO
GATEWAY_CHANGED=NO
STAGING_CHANGED=NO
IMPLEMENTATION_AUTHORIZED=NO
ARCHITECTURE_FULLY_FROZEN=NO
READY_FOR_0_9D_RUNTIME=NO
FREEZE_DOCUMENT=NOT_WRITTEN
```

Evidence is the code and migrations on `4976e5d111ed2f284910c87baa7f83f3b88cc7d1`. NLO-0.9B remains the chain policy. This note does not change that policy, does not enable `execution_supported`, and does not authorize a runtime.

## 1. Baseline

```text
CHAIN_SEARCH_RUNTIME_PRESENT=YES
CHAIN_CANDIDATE_PERSISTENCE_PRESENT=YES
CHAIN_RESULT_EXECUTION_SUPPORTED=false
ROUTE_PLAN_ACCEPT_PRESENT=YES
ROUTE_PLAN_ACTIVATE_PRESENT=YES
TMS_PROJECTION_ENDPOINT_PRESENT=YES
TMS_PROJECTION_PATH=/internal/v1/transport-executions/from-route-plan-activation
TMS_EXECUTION_OWNER=shipment-service
NLO_EXECUTION_OWNER=NO
PROJECTION_ACK_AFTER_COMMIT=YES
```

Public chain search is `POST/GET /v1/network/chain-searches` and `GET .../candidates` in `services/network-optimizer-service/internal/http/router.go`. The gateway exposes the same three routes under `/api/v1/network`. `chainapi.projectSearch` and `projectCandidates` force `ExecutionSupported: false`. Migration `000097_nlo_chain_search_persistence_v0_9c_i2a.up.sql` checks `chain_search_runs.execution_supported = false`. Route plans keep `execution_supported = false` from migration `000085`.

Route-plan accept and activate are `POST /v1/network/route-plans/{id}/accept` and `.../activate`, implemented by `Service.AcceptRoutePlan` and `Service.ActivateRoutePlan`. The gateway maps both under `/api/v1/network` with `PolicyEvaluateRoutePlan`.

Shipment-service owns execution. The internal route is registered in `services/shipment-service/internal/http/router.go`. `TransportExecutionRepository.AcceptRoutePlanActivation` commits the TMS transaction and then reads the stored revision. The acknowledgement is that read, not `shipment.execution_plan.created`.

## 2. Candidate and route-plan identity

```text
CHAIN_CANDIDATE_HAS_SINGLE_SELECTED_ROUTE_PLAN=NO
ROUTE_PLAN_ID_PINNED_TO_CANDIDATE=NO
ROUTE_PLAN_VERSION_PINNED=NO
SELECTED_ORDER_IMMUTABLE_AFTER_ACCEPT=NOT_APPLICABLE_UNTIL_PLAN_IS_PINNED
ACCEPT_FORBIDDEN_WHILE_ROUTE_PLAN_ID_IS_NULL=YES
CLIENT_CAN_SUBSTITUTE_ROUTE_PLAN_ID=NO
```

`chain_candidates` stores `search_id`, `tenant_id`, the canonical load pair and versions, `feasibility_status`, nullable `selected_order` (`L1_BEFORE_L2` or `L2_BEFORE_L1`), and nullable `route_plan_id`. The foreign key is `(tenant_id, route_plan_id)` to `route_plans`. There is no `route_plan_version` column. A non-null `route_plan_id` is allowed only when `feasibility_status = 'FEASIBLE'`. `selected_order` is likewise only allowed for `FEASIBLE`.

`chainorch.project` writes `RoutePlanID: nil` and does not insert a route plan. A feasible candidate can therefore exist with an order code and no plan. NLO-0.9B already said `route_plan_id` stays null until an evaluation is stored, and that the evaluation identity is that plan after storage. Storage does not happen in NLO-0.9C.

Accept of a candidate with `route_plan_id` null is forbidden. The client must not send `route_plan_id`, load ids, or a plan version. The server resolves the stored plan. Until a later authorized runtime stores that plan and its version, there is no candidate that can pass this gate.

`selected_order` is immutable in the candidate row today because candidates are insert-only. That does not pin a plan version. `MarkRoutePlanAccepted` sets `status='ACCEPTED'` without incrementing `route_plans.version`.

## 3. Public accept shape

```text
CHAIN_ACCEPT_API_SHAPE=C_THIN_CANDIDATE_ACCEPT
CLIENT_SELECTS_CHAIN_CANDIDATE=YES
CLIENT_SELECTS_LOAD_IDS_AGAIN=NO
CLIENT_SUPPLIES_ROUTE_PLAN_ID=NO
ROUTE_PLAN_ACCEPT_REUSED=YES
```

Rejected shapes:

- A. Calling `POST /api/v1/network/route-plans/{id}/accept` directly lets the caller choose the plan id. The chain candidate is not the selection.
- B. A second accept aggregate that stores its own plan id from the body duplicates the route-plan lifecycle.

Frozen shape, not implemented:

`POST /api/v1/network/chain-searches/{chainSearchId}/candidates/{candidateId}/accept`

Header `Idempotency-Key` is required. The body is an empty JSON object. Any other field is rejected, including `route_plan_id`, load ids, shipment ids, and `version`. The server loads the candidate in the caller tenant, checks that it belongs to that search, and accepts only the pinned plan through the existing `AcceptRoutePlan` rules. This route is not in the gateway or the service router.

Accept is not activation and is not execution.

## 4. Accept preconditions

```text
CHAIN_ACCEPT_STATUS=ROUTE_PLAN_ACCEPTED
CHAIN_ACCEPT_IDEMPOTENCY_SCOPE=TENANT_PLUS_ACCEPT_CHAIN_CANDIDATE_KEY
CHAIN_ACCEPT_REPLAY_SEMANTICS=SAME_KEY_SAME_EMPTY_BODY_REPLAYS_CHANGED_BODY_CONFLICTS
```

Existing route-plan accept, which the thin endpoint must call unchanged:

- The plan is loaded with `GetRoutePlan(tenant, id)`. A missing row is not found. A foreign tenant is the same not-found result.
- The candidate must belong to the search in that tenant. The candidate foreign key already requires `(tenant_id, search_id)`.
- Accept requires stored status `EVALUATED`, or an already `ACCEPTED` plan at the same version, which replays.
- `ensureRoutePlanFresh` rechecks service-duration policy id, version, and fingerprint; each load id and version and visibility; catalog and rule fingerprints; routing and algorithm policy fingerprints; and, for `CURRENT_TRIP`, shipment version, current-trip input fingerprint, vehicle version, and confirmed cargo versions. A mismatch is `409` with `PLAN_STALE`.
- `DEPOT_START` rechecks the owned capacity id and version. It does not read a shipment status.
- Idempotency key `accept_route_plan:` plus the caller key is tenant-scoped. `peekIdempotency` replays an equal body hash and conflicts on a different hash.

The thin chain accept adds, before that call:

- Search and candidate are in the caller tenant.
- Candidate `search_id` equals the path search id.
- Search status is `COMPLETED`. Pending, running, failed, cancelled, and expired searches are not acceptable.
- Candidate `feasibility_status` is `FEASIBLE`.
- `route_plan_id` is non-null. Otherwise accept is refused and no plan status changes.
- The plan tenant matches the candidate tenant. The existing foreign key is the storage rule. The accept path still loads the plan only in the caller tenant.

Chain accept must not create an execution, assign a driver, change a shipment status, book a slot, or write a TMS row. `AcceptRoutePlan` today only marks the plan accepted, writes an audit row, and emits `network.route_plan.accepted`.

There is no chain-accept idempotency key yet. The frozen scope is a distinct prefix, `accept_chain_candidate:`, so a route-plan accept key cannot replay a chain accept. Same key and the same empty body replay the stored response. Same key and any non-empty or different body conflict. A replay must not insert a second acceptance.

## 5. Activate preconditions

```text
ROUTE_PLAN_ACTIVATE_REUSED=YES
DEPOT_START_MULTI_LOAD_STATUS_POLICY=UNPROVEN
CURRENT_TRIP_MULTI_LOAD_STATUS_POLICY=SEED_ONLY_PROVEN_ADDITIONAL_LOADS_UNPROVEN
```

`ActivateRoutePlan` requires an `ACCEPTED` plan, a matching version, the same freshness checks as accept, `activationBlockReason` clear, every leg `ExpiresAt` still in the future, and `ActivationStatusAllowed(planningMode, shipmentStatus)`. It inserts one `PENDING_EXECUTION` row. Unique `(tenant_id, route_plan_id)` in migration `000086` prevents a second activation of the same plan.

`activationBlockReason` refuses unknown service duration, a result other than `FEASIBLE`, and any capacity snapshot whose `compatibility_status` is `INDETERMINATE`.

Proven single-status allow-lists in `routeplan/policy.go`:

| Planning mode | Status value passed today | Allowed |
| --- | --- | --- |
| `DEPOT_START` | Empty string. `ensureRoutePlanFresh` returns `""` because the plan has no shipment. | `CARRIER_ASSIGNED`, `ACCEPTED_BY_CARRIER`, `VEHICLE_ASSIGNED`, `DRIVER_ASSIGNED`, `PICKUP_SLOT_BOOKED` |
| `CURRENT_TRIP` | The seed shipment status from `CurrentTripContext`. | `IN_PICKUP`, `LOADED`, `IN_TRANSIT` |

Empty status matches neither depot-start value, so every current depot-start activation is `SHIPMENT_STATUS_NOT_ELIGIBLE`. That matches `NLO_0_4D_ARCHITECTURE_FREEZE.md`. This discovery does not weaken that allow-list.

`LoadOpportunity.Status` is the publication status. It is not a shipment FSM status. Chain candidates do not store a shipment id, shipment tenant, cargo id, or shipment status for L1 or L2. The current activation function does not iterate the two loads.

Because those facts are absent, this discovery does not define a combined L1/L2 eligibility matrix. Applying the depot-start allow-list to both loads, or treating an additional load as pre-execution eligible, would be a guess. Both remain open:

- Depot-start chain: which shipment status applies to each load, and whether both must already be materialized shipments, is unproven.
- Current-trip chain: the seed shipment gate above is proven. The status rule for the additional load is unproven.

`projectionCommand` also requires `Plan.ShipmentID` and builds a current-trip command only. A depot-start plan never reaches TMS through the current builder. TMS `validateContext` rejects a depot-start command that includes a context shipment, and rejects a current-trip command that omits it.

## 6. Service duration

```text
SERVICE_DURATION_SOURCE_PRESENT=YES
SERVICE_DURATION_VERSION_PINNED=YES
UNKNOWN_SERVICE_DURATION_ACTIVATION_ALLOWED=NO
ALL_CHAIN_CARGO_ACTIONS_CARRY_DURATION_BEFORE_ACTIVATION=REQUIRED_NOT_TRUE_OF_UNPINNED_CANDIDATES
```

`network_optimizer.service_duration_policies` is migration `000094`. `ActiveServiceDurationPolicy` returns the operating tenant's `ACTIVE` row. Chain claim stores `service_duration_policy_id`, `service_duration_policy_version`, and `service_duration_snapshot_json` on `chain_search_runs`. `SnapshotFromPolicy` pins pickup and delivery seconds. `UnknownDurationSnapshot` is `NO_ACTIVE_POLICY` when no active row exists.

Accept and activate call `serviceDurationFresh`. Zero dependency rows are ignored. One row must match the active policy id, version, and pickup/delivery fingerprint. Anything else is stale. `ProductionActivationRequiresServiceDurationSource` is true. A plan reason `SERVICE_DURATION_UNKNOWN`, or a cargo stop that has actions and a null `service_duration_seconds`, refuses activation.

A chain candidate without a stored plan has no stop rows, so this gate cannot succeed for current candidates. Known duration on the search snapshot is not itself an activation proof.

## 7. Stacking and compatibility

```text
STACKING_EXECUTION_GATE=CHAIN_SEARCH_REJECTS_UNKNOWN_OR_FALSE_OVERLAP_ACTIVATION_RECHECKS_FEASIBLE_AND_INDETERMINATE
UNKNOWN_STACKABILITY_ACTIVATION_ALLOWED=NO
```

`chain.stackingDecision` rejects overlap when either `stackable` is nil (`STACKING_UNKNOWN`) or either flag is false (`STACKING_REJECTED`). Overlap with both flags true does not reject for stacking and does not add pallet space. No overlap skips the check. That is the NLO-0.9B policy, now present in the chain engine. NLO-0.9B's older line `STACKING_CHECK_PRESENT_IN_CURRENT_SOURCE=NO` is historical for the pre-0.9C tree.

Activation does not read `stackable` again. It refuses a non-feasible plan and any `INDETERMINATE` compatibility snapshot. Other capacity dimensions remain the planner's own known-capacity checks. A chain accept must not treat a stacking pass as proof of weight, volume, pallets, linear metres, or height.

Unknown stackability does not produce a feasible chain candidate. Activation of an indeterminate compatibility snapshot is refused. A generic route plan that bypassed the chain engine is outside this gate.

## 8. TMS projection capability

```text
TMS_MULTI_SHIPMENT_PROJECTION_SUPPORTED=YES
TMS_MULTIPLE_DELIVERY_ACTIONS_SUPPORTED=YES
TMS_CROSS_SHIPPER_PARTICIPANTS_SUPPORTED=YES_WHEN_COMMAND_CARRIES_OWNER_TENANT
ONE_EXECUTION_ROOT_FOR_CHAIN=NOT_ENFORCED_ACROSS_CANDIDATES
NLO_BUILDER_PRESERVES_FOREIGN_SHIPMENT_TENANT=NO
```

`materializedSubjects` accepts many subjects. The participant insert writes one row per subject and one active-shipment row per `(shipment_tenant_id, shipment_id)`. `projection_integration_test.go` projects two shipper shipments into one execution, keeps both owner tenants, does not create a shipment in the operating tenant, and does not change shipment status. Replay of the same activation returns the same execution and revision.

Each subject and each cargo action requires `execution_shipment_id`, `shipment_tenant_id`, `execution_shipment_version`, `cargo_id`, and `cargo_version`, plus route subject type, id, and version. `verifyMaterializedRows` reads `transport.shipments` and `transport.cargoes` with `id` and `tenant_id = shipment_tenant_id`. A tenant that does not own the row is `EXECUTION_SUBJECT_UNMATERIALIZED`. No shipment row is the execution root. The execution root is `transport.transport_executions` under `operating_tenant_id`.

The current NLO builder does not use that multi-shipper proof. `projectionCommand` sets `ContextShipmentTenantID` and every `ShipmentTenantID` to `actor.TenantID`. It materializes an action only when `SubjectType` is `SHIPMENT_CARGO` and `SourceShipmentID` is set. Chain load actions are `LOAD_OPPORTUNITY`. Those actions are sent without shipment and cargo identity, which TMS rejects as unmaterialized. A foreign shipper load is not projected under its `owner_tenant_id`.

## 9. Activation handshake

Proven route-plan sequence in `ActivateRoutePlan` and `finishActivationHandoff`:

1. Plan is already `ACCEPTED`.
2. Activation row is inserted as `PENDING_EXECUTION` and committed with the activation-requested audit and outbox event.
3. If a projection client is configured, NLO sends the command to shipment-service.
4. TMS validates, commits, and returns `operating_tenant_id`, `activation_id`, `route_plan_id`, `execution_id`, `execution_revision_id`.
5. NLO links only when those fields match the pending row and the plan.
6. The same NLO transaction stores the ids, sets `EXECUTION_LINKED`, audits, and emits `network.route_plan.execution_linked`.

```text
ACK_AFTER_COMMIT_REQUIRED=YES
NLO_LINKS_ONLY_AFTER_ACK=YES
MAX_EXECUTION_LINKED_PLANS_PER_CHAIN=UNPROVEN
```

`EXECUTION_LINKED` is not written before the acknowledgement is stored. A temporary projection error returns the pending body and leaves the row pending. A permanent projection error marks `REJECTED` and does not link. A correlation mismatch does not link.

Chain candidate acceptance is step 0 and is not implemented. It must finish before step 1. Nothing in `chain_search_runs` or `chain_candidates` limits a search to one linked plan. `MAX_EXECUTION_LINKED_PLANS_PER_CHAIN` is therefore unproven. The frozen target is one linked plan per search, but no table enforces it.

## 10. Idempotency and crash recovery

Existing route-plan recovery is sufficient for one activation id:

| Window | Proven behavior |
| --- | --- |
| Accept stored, activate not called | Plan stays `ACCEPTED`. No TMS write. |
| Activation stored, projection times out | Row stays `PENDING_EXECUTION`. Retry calls the same activation id. |
| TMS committed, ack lost | TMS `findActivation` returns the stored revision. NLO then links. |
| Ack received, linkage transaction fails | NLO transaction rolls back. Row stays pending. Retry reads the same TMS revision. |
| Process crash after link | Status is `EXECUTION_LINKED`. A later activate returns the stored row. |

TMS same activation id and same body do not create a second execution root or a second revision. Same id and a different body return `ACTIVATION_BODY_CONFLICT`.

```text
RETRY_CREATES_SECOND_EXECUTION_ROOT=NO_FOR_SAME_ACTIVATION_ID
CHAIN_ACCEPT_IDEMPOTENCY=NOT_IMPLEMENTED
CHAIN_ACTIVATION_IDEMPOTENCY=ROUTE_PLAN_KEY_AND_UNIQUE_PLAN_ACTIVATION
CRASH_RECOVERY_SUFFICIENT=YES_FOR_ONE_ROUTE_PLAN_NO_FOR_CHAIN_SELECTION
```

Chain-specific gaps the current recovery does not close:

- Two feasible candidates in one search have no mutual exclusion.
- There is no chain-accept idempotency record.
- A candidate has no acceptance status, so a crash between "candidate selected" and "plan accepted" is not a defined row.

A later runtime must keep the 0.4D activation recovery and add candidate-scoped accept idempotency. It must not add a second TMS root for one activation.

## 11. Cross-shipper tenant model

```text
OPERATING_TENANT=CALLER_CARRIER_TENANT
SHIPMENT_TENANT_IDS_PRESERVED=YES_ON_TMS_NO_ON_CURRENT_NLO_BUILDER
TMS_EXECUTION_OWNER=shipment-service
NLO_EXECUTION_OWNER=NO
```

TMS stores `operating_tenant_id` on the execution and `shipment_tenant_id` on each participant. Verification uses the shipment tenant, not the operating tenant. The integration test proves the owner tenants survive projection.

The NLO command builder overwrites subject tenant with the carrier tenant. That fails closed at TMS when the shipment is not in the carrier tenant. It is not a proof that foreign ownership is preserved on the chain path. A chain runtime must copy each shipment's owner tenant into `shipment_tenant_id` and must not update `transport.shipments.tenant_id`.

No chain candidate stores those owner shipment identities today.

## 12. Privacy after accept

```text
POST_ACCEPT_PRIVACY_CONTRACT=EXISTING_ACCEPTED_ROUTE_PLAN_AND_TMS_NO_NEW_COMMERCIAL_SURFACE
```

NLO-0.9B section F stays in force. Chain search responses do not gain the other shipper's rate, customer, contract, or internal shipment id because a candidate was accepted. `projectCandidates` does not copy `revenue_amount` or `revenue_currency`. Accept and activate responses are the existing route-plan document plus the activation row. They are not a new chain privacy surface.

TMS driver and execution reads keep their existing field filters. This discovery does not authorize a chain response to expose commercial terms that those contracts omit.

## 13. Successor and replan

```text
AT_RISK_REOPTIMIZATION_INCLUDED=NO
NLO_ACTIVATION_SETS_SUPERSEDES=NO
AUTOMATIC_REOPTIMIZATION=NO
```

`projectionCommand` does not set a superseding plan or activation. Successor revisions stay on shipment-service. NLO-0.9D does not mutate completed stops, re-parent an execution, or start at-risk reoptimization. A later plan change remains a new plan and a new activation. Supersede of the previous revision happens only inside the existing TMS successor command after that command commits. NLO does not mark a predecessor superseded before that commit.

## 14. Failure matrix

Every row is `FUTURE_REQUIRED`. None of these tests exist as chain-accept tests.

| Id | Condition | Required result |
| --- | --- | --- |
| CHAIN_EXEC_01 | Candidate does not belong to the search | No accept, no TMS write |
| CHAIN_EXEC_02 | Search or candidate is outside the caller tenant | Not found, no existence leak |
| CHAIN_EXEC_03 | Candidate has no evaluated route plan, or plan status is not `EVALUATED` | Accept refused |
| CHAIN_EXEC_04 | Stored dependency version or fingerprint differs | `PLAN_STALE`, no accept |
| CHAIN_EXEC_05 | A plan leg is expired | Activation refused, no link |
| CHAIN_EXEC_06 | Service duration unknown or cargo stop duration null | Activation refused |
| CHAIN_EXEC_07 | Overlap with unknown stackability | Not a feasible candidate; activation not allowed |
| CHAIN_EXEC_08 | Compatibility snapshot `INDETERMINATE` | Activation refused |
| CHAIN_EXEC_09 | Shipment status outside the proven allow-list for that mode | Activation refused. Multi-load rule remains open |
| CHAIN_EXEC_10 | Same chain-accept idempotency key and same empty body | Replay, one acceptance |
| CHAIN_EXEC_11 | Same key and a different body | Conflict, stored acceptance unchanged |
| CHAIN_EXEC_12 | TMS permanent rejection | Activation `REJECTED`, no `EXECUTION_LINKED` |
| CHAIN_EXEC_13 | TMS timeout before commit | Stays `PENDING_EXECUTION`, no link |
| CHAIN_EXEC_14 | TMS committed and ack lost | Retry links the same ids |
| CHAIN_EXEC_15 | Retry of the same activation | Same execution and revision |
| CHAIN_EXEC_16 | Two owner tenants on one execution | Both `shipment_tenant_id` values unchanged |
| CHAIN_EXEC_17 | Retry or replay | No second execution root for that activation |
| CHAIN_EXEC_18 | Accepted chain response | No foreign commercial rate, customer, or contract |

## 15. Implementability

```text
READY_FOR_0_9D_RUNTIME=NO
```

Blockers:

1. No chain candidate stores a route plan. `chainorch.project` forces `route_plan_id` null. Accept cannot resolve a plan, and no column pins `route_plan_version`.
2. Chain loads are `LOAD_OPPORTUNITY` publication rows. They do not carry shipment id, shipment tenant, cargo id, or shipment FSM status. TMS materialization and the per-load status matrix cannot be closed from the current candidate.
3. The NLO projection builder stamps the carrier tenant onto every subject and only materializes `SHIPMENT_CARGO`. It does not preserve a foreign `shipment_tenant_id`. TMS itself can, when the command already carries the owner tenant.
4. Depot-start activation is refused because the status input is empty. Current-trip activation checks only the seed shipment. The additional-load status rule is unproven, so a multi-load matrix is not frozen.
5. Nothing limits one search to one `EXECUTION_LINKED` plan. Chain-accept idempotency does not exist.

Closed facts that a later runtime must keep:

- `execution_supported` stays false until a separate authorized stage changes the check. This discovery does not change it.
- Accept is not execution.
- The caller selects `chain_candidate_id` and does not supply `route_plan_id` or load ids.
- Unknown service duration cannot activate.
- Unknown or false stackability on overlap cannot activate.
- `INDETERMINATE` compatibility cannot activate.
- NLO links only after the TMS ack.
- The same activation id does not create a second TMS execution root.
- Shipment-service remains the execution owner.
- At-risk reoptimization is out of scope.

`NLO_0_9D_CHAIN_EXECUTION_ACCEPTANCE_FREEZE.md` is not written. The open status matrix and missing plan binding are architecture gaps, not documentation omissions.
