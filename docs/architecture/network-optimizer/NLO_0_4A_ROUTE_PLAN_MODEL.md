# NLO-0.4A route plan model

Status: accepted. Architecture frozen. No migration. No generated OpenAPI. No runtime implementation.

```text
ADR_STATUS=ACCEPTED
ARCHITECTURE_FROZEN=YES
OWNERSHIP_DECISION_FROZEN=YES
BLOCKING_FINDINGS=0
NLO04A_F001=CLOSED
NLO04A_F002=CLOSED
NLO04A_F003=CLOSED
NLO04A_F004=CLOSED
NLO04A_F005=CLOSED
NLO04A_R001=CLOSED
NLO04A_ERRATUM_E1=PROPOSED
NLO04A_POST_ACCEPT_F001=RESOLVED_AT_DESIGN_LEVEL
ROUTE_PLAN_OWNER=network-optimizer-service
EXECUTION_OWNER=shipment-service
ACCEPTED_PLAN_STRUCTURE_IMMUTABLE=YES
PLAN_STRUCTURE_IMMUTABLE_AFTER_EVALUATION=YES
LIFECYCLE_STATUS_MUTABLE=YES
MULTIPLE_ACTIONS_PER_STOP=YES
MIGRATION_CREATED=NO
```

NLO-0.3 contracts stay as they are. `CURRENT_TRIP_FILL` remains one additional load until a later authorized implementation. This model is the planning artifact those later waves would persist.

## Ownership

`network-optimizer-service` owns the planning artifact: the evaluated sequence, the accepted snapshot, and the successor link. `shipment-service` owns shipment status, driver tasks, and actual arrival or departure. `tracking-service` owns position and ETA freshness. `transport-order-service` owns the commercial order. ADR-NET-005 already says a plan is not a shipment. This proposal keeps that split. The optimizer does not become the source of truth for execution.

## RoutePlan

One row is one plan version. After `EVALUATED` is persisted, the structure does not change: stops, actions, legs, capacity snapshots, dependencies, and fingerprints. Lifecycle status and its clocks may move under compare-and-swap and idempotent commands: `EVALUATED` to `ACCEPTED`, then to `SUPERSEDED` or `CANCELLED`. That is not an edit of the route.

| Field | Role |
| --- | --- |
| `id`, `tenant_id`, `version` | Identity and optimistic concurrency |
| `planning_mode` | `CURRENT_TRIP` or `DEPOT_START` |
| `capacity_id`, `capacity_version` | Null on a current-trip plan, same rule as NLO-0.3D |
| `shipment_id`, `shipment_version` | Required for `CURRENT_TRIP`. Null for a depot-start plan that has no executing shipment |
| `vehicle_id` | Copied from trusted shipment or capacity context. Not caller-supplied |
| `context_fingerprint` | Server-built current-trip or depot context version |
| `algorithm_policy_version` | Proposed `nlo-0.4a-insert-v1` |
| `routing_policy_version` | Provider mode and profile hash policy |
| `status` | See lifecycle |
| `supersedes_plan_id` | Set only on a successor |
| `created_at`, `accepted_at`, `cancelled_at`, `superseded_at` | Audit clocks |

`activated_at` is not a column on the plan. Activation is a separate record so accepting a plan does not edit it into an execution object.

### Lifecycle

Only these planning states:

```text
EVALUATED
ACCEPTED
SUPERSEDED
CANCELLED
```

`DRAFT` is not used. Evaluation persists the plan. `ACTIVE` and `COMPLETED` are not plan states. Execution progress lives on the shipment. A cancelled plan was accepted or evaluated and then withdrawn before execution linked it. A superseded plan has a successor. Structure of an evaluated plan is not edited. Status transitions are the lifecycle metadata described above.

```text
PLAN_STRUCTURE_IMMUTABLE_AFTER_EVALUATION=YES
LIFECYCLE_STATUS_MUTABLE=YES
```

## RouteStop

A stop is an ordinal, a role, and a `RoutePoint`. It is not a load. Post-freeze erratum E1 is proposed in ADR-NET-019. It separates business location identity from the point the router uses. `MAX_STOPS` stays 8. Roles stay `START`, `CARGO`, and `END`.

```text
RoutePoint
  point_kind = CANONICAL_LOCATION or POSITION_ANCHOR
  location_id?
  latitude
  longitude
  point_source
  observed_at?

ONE_STOP_MANY_LOADS=YES
PICKUP_AND_DELIVERY_AT_SAME_STOP=YES when canonical location_id matches
SHARED_STOP_EQUALITY=same canonical location_id
CARGO_STOP_CANONICAL_LOCATION_REQUIRED=YES
END_STOP_CANONICAL_LOCATION_REQUIRED=YES
EXECUTION_ONLY_WAYPOINT_IN_V0_4_PLANNER=NO
BREAK_IN_V0_4_PLANNER=NO
COMPLETED_STOP_REORDER=NO
```

| Field | Role |
| --- | --- |
| `route_plan_id`, `stop_id`, `ordinal` | Ordinal is `1..MAX_STOPS`, unique per plan |
| `stop_role` | `START`, `CARGO`, or `END` |
| `point_kind` | `CANONICAL_LOCATION` or `POSITION_ANCHOR` |
| `location_id` | Required for `CANONICAL_LOCATION`. Null for `POSITION_ANCHOR`. Never a fabricated UUID |
| `latitude`, `longitude` | Exact coordinates used for routing. Canonical coordinates are resolved by a trusted server source |
| `point_source` | `CANONICAL_LOCATION`, `TRACKING_POSITION`, or `CAPACITY_POSITION` |
| `point_observed_at` | Set for a tracking anchor. Null for a canonical location |
| `planned_arrival`, `planned_departure` | Forward propagation result when service duration is known. Otherwise the time fields stay unset and time feasibility is `INDETERMINATE` |
| `service_duration` | Known only from an authoritative source. Cargo stops do not default to 0. See the algorithm document |

Constraints:

```text
stop_role=CARGO -> point_kind=CANONICAL_LOCATION and location_id IS NOT NULL
stop_role=END   -> point_kind=CANONICAL_LOCATION and location_id IS NOT NULL
stop_role=START -> CANONICAL_LOCATION or POSITION_ANCHOR
```

Cargo equality uses canonical `location_id` only. Label, city, Haversine, and coordinate similarity do not merge pickup, delivery, or destination stops. A `POSITION_ANCHOR` does not participate in that equality and does not absorb pickup or delivery actions.

`CURRENT_TRIP` `START` is the trusted `CurrentTripContext.CurrentPosition` when freshness is `FRESH` and both coordinates are present. It is persisted as one planning anchor: `POSITION_ANCHOR`, `source=TRACKING_POSITION`, latitude, longitude, and `observed_at`, plus the context fingerprint. The request body cannot supply those coordinates. `CALLER_SUPPLIED_START_POSITION_ALLOWED=NO`. A missing or not-fresh position is `ROUTE_START_POSITION_UNKNOWN`.

`DEPOT_START` `START` is `CANONICAL_LOCATION` when the trusted capacity `LocationID` is set. If that id is null and the trusted capacity has latitude and longitude, the start is `POSITION_ANCHOR` with `source=CAPACITY_POSITION`. If neither is present, the result is `ROUTE_START_POSITION_UNKNOWN`. No city centroid is invented.

`CURRENT_TRIP` `END` is required. Its `location_id` is the trusted `ShipmentExecution.DestinationLocationID`, and its coordinates are the trusted resolution of that location. A tracking-coordinate-only end is not allowed. `DEPOT_START` does not add an `END` stop. The route ends at the last cargo delivery.

`START` is not reordered. Actual arrival and departure are not stored on this row. They belong to shipment execution. A stop may exist without a cargo action only for `START` and, on a current trip, `END`. The v0.4 planner does not insert a break or a pure waypoint.

## RouteStopAction

An action names a `RouteLoadSubject`. It does not require `load_opportunity_id`.

```text
RouteLoadSubject
  subject_type = LOAD_OPPORTUNITY or SHIPMENT_CARGO
  subject_id
  subject_version

RouteStopAction
  subject_type
  subject_id
  subject_version
  action_type = PICKUP or DELIVERY
  quantity_delta for weight, volume, pallets, linear metres
  window_start, window_end
```

A subject is one logical cargo, not one shipment and not one action. Counting rules:

- Confirmed onboard cargo is `SHIPMENT_CARGO`. Its id is `OnboardCargoUnit.CargoID`. Its version is the cargo profile version.
- An additional marketplace load is `LOAD_OPPORTUNITY`.
- Cargo already on the executing shipment but not yet picked up is `SHIPMENT_CARGO` and is an existing future route load.
- If a load opportunity has been materialized as that shipment cargo id, it is the same subject. It is not counted again and it does not receive a second pickup. The evaluate request is rejected as `DUPLICATE_ROUTE_LOAD_SUBJECT` before search.

```text
ADDITIONAL LOAD:
  subject_type=LOAD_OPPORTUNITY
  actions=pickup and delivery

ALREADY ONBOARD:
  subject_type=SHIPMENT_CARGO
  actions=delivery only
```

Onboard delivery provenance, copied from the trusted context and not from the caller:

```text
shipment_id
shipment_version
cargo_id
cargo_version
evidence_state = CONFIRMED_ONBOARD
evidence_state_version
evidence_occurred_at
```

`START` and `END` are stop roles, not cargo actions. `BREAK` is not a v0.4 action. Several actions may share one stop when the canonical `location_id` is equal. A position anchor is never that shared stop. Pickup of subject L and delivery of subject L are never the same action. For a subject that has both, pickup ordinal is less than or equal to delivery ordinal, and when they share a stop the pickup action is ordered before the delivery action. Already-onboard cargo has no pickup action in the plan.

## RouteLeg

`Stop N` to `Stop N+1`.

| Field | Role |
| --- | --- |
| `from_stop_id`, `to_stop_id` | Adjacent ordinals only |
| `distance_m`, `duration_s` | Road metrics from `routing.Provider` |
| `provider`, `request_fingerprint`, `response_fingerprint` | Existing fingerprint functions |
| `traffic_mode`, `calculated_at`, `expires_at` | From `RouteResult` |

Geometry and the raw provider payload are not persisted in v0.4. The port may return geometry. The plan stores the metrics and the fingerprints. A missing road result is not zero distance.

`routing.RouteRequest` routes by origin and destination coordinates. Canonical location id is not the routing identity. `RoutePointFingerprint` hashes `point_kind`, the canonical `location_id` when present, the exact coordinates sent to the provider, `point_source`, and `observed_at` for a position anchor. `RouteLegKey` is the two point fingerprints plus `vehicle_profile_hash`, `route_mode`, `traffic_mode`, and `departure_bucket`. That matches `routing.Fingerprint`. Reuse is allowed only inside one search, only before `ExpiresAt`, and not across requests. A cached leg is not a reason to skip a capacity or compatibility check. `CACHE_KEY_SUPPORTS_POSITION_ANCHOR=YES`.

## Capacity ledger

`RouteCapacitySnapshot` is calculated after every future cargo action and persisted with the plan as an audit row. It is not a live balance that later code mutates.

For `CURRENT_TRIP`, the first snapshot is the trusted NLO-0.3C residual at `START`. `INITIAL_CAPACITY_SOURCE=CURRENT_TRIP_CONTEXT`. The caller does not recompute occupancy. Each later snapshot follows one action. If a required residual dimension is `UNKNOWN`, the plan result is `INDETERMINATE`.

Each snapshot records weight, volume, pallet count, linear metres, and whether temperature, ADR, and food-grade constraints were known. The onboard set starts as confirmed onboard cargo and then follows pickups and deliveries. It is not inferred by inventing a pickup for cargo that is already loaded. `EvaluateGroupageItems` runs on the set for each following leg. Unknown required facts stay `INDETERMINATE`.

## Dependencies

`route_plan_dependencies` stores the versions the plan was computed from: shipment version, capacity version when present, each load id and version, context fingerprint, and routing policy version. Accept and activate compare these to current trusted reads. A mismatch is `409` with reason `PLAN_STALE`.

## ERD (accepted, not migrated)

```text
route_plans
  id, tenant_id, version, status, planning_mode
  capacity_id?, shipment_id?
  supersedes_plan_id?

route_plan_stops
  route_plan_id, stop_id, ordinal, stop_role
  point_kind, location_id?, latitude, longitude, point_source, point_observed_at?

route_stop_actions
  stop_id, action_ordinal, action_type
  subject_type, subject_id, subject_version

route_plan_legs
  route_plan_id, from_stop_id, to_stop_id
  distance_m, duration_s, fingerprints

route_capacity_snapshots
  route_plan_id, after_stop_id, after_action_ordinal, onboard metrics

route_plan_dependencies
  route_plan_id, subject_type, subject_id, subject_version

route_plan_activations
  route_plan_id, plan_version, idempotency_key
  execution_shipment_id?, status
```

Links: capacity and shipment are references, not copies of their rows. Load opportunities are references plus version. Driver tasks are not children of the plan. Shipment-service creates them in a later wave from an activation record.

## API shape (contract only)

Existing prefix is `/v1/network/...`.

```text
POST /v1/network/route-plans/evaluate
GET  /v1/network/route-plans/{id}
POST /v1/network/route-plans/{id}/accept
POST /v1/network/route-plans/{id}/activate
```

Evaluate body: `planning_mode`, optional `shipment_id` for current trip, optional `capacity_id` for depot start, and the candidate load ids. It does not accept position, onboard cargo, residual numbers, or a stop list. Tenant comes from the gateway. `Idempotency-Key` is required on evaluate, accept, and activate, using the existing conflict rule when the body changes.

Errors:

| Condition | Result |
| --- | --- |
| Foreign shipment or capacity | `404` |
| Start has neither a canonical location nor trusted coordinates, or a current-trip position is not `FRESH` | `ROUTE_START_POSITION_UNKNOWN` |
| Base subjects plus requested additional loads above 4, or more than 4 subjects already | `422` `PLAN_LOAD_LIMIT_EXCEEDED` |
| Same cargo named twice | `422` `DUPLICATE_ROUTE_LOAD_SUBJECT` |
| Search cap | `422` `SEARCH_BUDGET_EXHAUSTED` with `SEARCH_BUDGET_EXCEEDED` |
| Heuristic finished with no plan | `NO_PLAN_FOUND_WITHIN_POLICY` |
| Version mismatch | `409` `PLAN_STALE` |
| Second activate with the same key and body | Replay of the first activation |
| Routing provider down | `ROUTING_UNAVAILABLE`, not a zero-length leg |
