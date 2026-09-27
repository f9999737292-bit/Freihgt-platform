# NLO-0.4A multi-stop algorithm

Status: accepted. Not a production planner. The counts below are asserted by `services/network-optimizer-service/internal/nlo04adiscovery/cost_model_test.go`.

```text
ARCHITECTURE_FROZEN=YES
BOUNDED_SEARCH_MODEL=ACCEPTED
SEARCH_ALGORITHM=BOUNDED_INCREMENTAL_HEURISTIC
SEQUENCING_ALGORITHM=INCREMENTAL_COST_MIN_INSERTION
UNRESTRICTED_PERMUTATION_SEARCH=NO
GLOBAL_SEQUENCE_SPACE_EXHAUSTIVE=NO
GLOBAL_INFEASIBILITY_PROVEN=NO
LOAD_ORDER=LEXICOGRAPHIC_LOAD_ID
LOAD_ORDER_PERMUTATIONS_EXPLORED=NO
SOLVER=NONE
FIRST_RELEASE_MAX_ADDITIONAL_LOADS=2
MAX_ROUTE_LOAD_SUBJECTS=4
TIME_BUDGET_IS_SLO=NO
```

## What is searched

Completed execution history is a frozen prefix. The searchable sequence is the ordered future stops only. For a current trip the first future stop is the trusted current position (`START`) and the last is the trusted destination (`END`) when that destination still remains. Those two anchors are the NLO-0.3D shape. A richer trip may already have more future stops.

One additional load is placed by choosing a pickup gap and a delivery gap at the same gap or a later gap. Pickup is written before delivery when they share a gap. The number of pairs for `N` future stops is `(N+1)*(N+2)/2`.

| Future stops | Insertion pairs for one load |
| --- | --- |
| 0 | 1 |
| 2 | 6 |
| 4 | 15 |
| 8 | 45 |

Loads are considered in lexicographic load-id order. That order is deterministic. It is not a proof that another order would fail. Load-order permutations are not explored. A different order may admit a plan this search does not find.

After a load is placed, only the winning sequence is kept. The next load is inserted into that one sequence. Other parent sequences are discarded. This is a bounded incremental heuristic, not the Cartesian product of all loads, and not an optimal route search.

Winner rule, in order:

1. Feasible beats indeterminate. Hard reject is discarded.
2. Lower total road duration.
3. Lower pickup ordinal, then lower delivery ordinal.
4. Load id is already fixed by the outer order, so it is not a second permutation.

Equal cost does not depend on map iteration. Gaps are scanned in increasing pickup gap, then increasing delivery gap.

Shared stops: if a candidate gap lands on an existing future stop with the same `location_id`, the action attaches to that stop instead of creating a new stop. The cost model below counts the larger case, two new stops, which is the routing upper bound.

## Approaches that were measured

| Approach | Decision |
| --- | --- |
| A. Insert one load at a time | Used. Keeps one parent sequence |
| B. Cap additional loads at 2 | Proposed first release |
| C. Enumerate every interleaving of all loads | Rejected. Unrestricted growth |
| D. Greedy lexicographic only, ignoring road duration | Rejected as the primary key. Used only as the tie-break |
| E. Cost-min insertion with a deterministic tie-break | Used together with A and B |

## Cost table

`legs without cache` counts every adjacent pair of every evaluated sequence. That number is the safe provider-call upper bound for the heuristic. `UNIQUE_LOCATION_PAIRS` counts `from_location + to_location` only. It is not a provider-call count. The real cache key also includes vehicle profile hash, traffic mode, and departure bucket, so one location pair can still require more than one provider call. The test keeps the first sequence as the parent, which does not change the candidate or leg counts.

| Scenario | Greedy candidates | Leg evaluations | Unique location pairs | Stops after |
| --- | --- | --- | --- | --- |
| 2 future stops + 1 load | 6 | 18 | 10 | 4 |
| 2 future stops + 2 loads | 21 | 93 | 27 | 6 |
| 2 future stops + 3 loads | 49 | 289 | 52 | 8 |
| 4 future stops + 2 loads | 43 | 271 | 45 | 8 |
| 4 future stops + 3 loads | 88 | 676 | 78 | 10 |

NLO-0.3D today spends 4 route calls on one fixed sequence for one load. The table is the search amplification once insertion positions are enumerated.

## Why 43 is not exhaustive

For 4 future stops the first load has 15 insertion candidates. If every first-load parent were retained, each parent would have 6 stops and the second load would have 28 insertions on each parent: `15 * 28 = 420` second-level candidates. Adding the first level gives `435`. The heuristic evaluates `15 + 28 = 43` because only one first-stage winner survives.

```text
GREEDY=43
FULL_PARENT_RETENTION=435
43 != exhaustive 435
GLOBAL_SEQUENCE_SPACE_EXHAUSTIVE=NO
```

The opposite load order is not included in 435. Full parent retention is intentionally out of the first release. This comparison is why a finished heuristic cannot prove that no feasible sequence exists.

The groupage engine remains `EvaluateGroupageItems`. There is no second rules engine. Work is capped by the structural ceilings below, not by the 5 second watchdog alone.

## Proposed server limits

| Limit | Value | Why | Fail |
| --- | --- | --- | --- |
| `MAX_ADDITIONAL_LOADS` | 2 | On a 4-stop future route, 3 loads cost 676 uncached legs. 2 loads cost 271 | `SEARCH_BUDGET_EXCEEDED` `budget=loads` before the third additional load is opened |
| `MAX_ROUTE_LOAD_SUBJECTS` | 4 | Total distinct planning subjects, not "2 onboard plus 2 additional". NLO-0.3C `OnboardCargoUnits` has no hard maximum of 2 | `PLAN_LOAD_LIMIT_EXCEEDED`. Onboard cargo is not truncated |
| `MAX_STOPS` | 8 | 4 stops plus 2 loads end at 8 stops. 3 loads end at 10 | `budget=stops` |
| `MAX_SEQUENCE_CANDIDATES` | 64 | The 2-load greedy search evaluates 43 sequences. The 3-load greedy search evaluates 88 | `budget=sequences` |
| `MAX_ROUTE_LEG_EVALUATIONS` | 300 | Structural work. Every candidate adjacent leg consumes one evaluation. 271 fits. 676 does not | `budget=routing` |
| `MAX_ROUTING_PROVIDER_CALLS` | 300 | Actual provider calls. Cache may only reduce this number | `budget=routing` |
| `MAX_GROUPAGE_EVALUATIONS` | 600 | `300` leg evaluations times `MAX_COMPATIBILITY_CONTEXTS_PER_LEG=2` | `budget=groupage` |
| `TIME_BUDGET` | 5s | Watchdog, same role as NLO-0.3E. `TIME_BUDGET_IS_SLO=NO`. Not the only compatibility guard | `budget=time` |
| `MAX_PICKUPS` | 2 | New pickup actions for additional load opportunities. Already-onboard cargo gets none | `budget=pickups` |
| `MAX_DELIVERIES` | 2 | New delivery actions for additional load opportunities. Deliveries of subjects already on the trip are not counted here | `budget=deliveries` |
| `MAX_ACTIONS_PER_STOP` | 4 | Shared warehouse cap | `budget=actions` |

`MAX_ROUTE_LEG_EVALUATIONS=300` alone would still allow the empty-route 3-load case (289 legs). The additional-load cap of 2 is therefore structural, not only a call cap. No limit is `UNLIMITED`. `MAX_ACTIVE_LOADS_PER_PLAN` is not used.

A route load subject is one of `ONBOARD_CARGO`, `ADDITIONAL_LOAD_OPPORTUNITY`, or `EXISTING_FUTURE_ROUTE_LOAD`. Current-trip planning requires `BASE_ROUTE_LOAD_SUBJECTS + REQUESTED_ADDITIONAL_LOADS <= 4` and, separately, `REQUESTED_ADDITIONAL_LOADS <= 2`.

| Base subjects | Additional | Result |
| --- | --- | --- |
| 0 | 2 | allowed |
| 1 | 2 | allowed |
| 2 | 2 | allowed |
| 3 | 1 | allowed |
| 3 | 2 | `PLAN_LOAD_LIMIT_EXCEEDED` |
| 4 | 0 | representable |
| 4 | 1 | `PLAN_LOAD_LIMIT_EXCEEDED` |
| more than 4 already | 0 | no v0.4 first-release plan |

```text
ROUTING_CALLS_PER_SEQUENCE = adjacent pairs in that sequence
ROUTING_PROVIDER_CALLS <= ROUTE_LEG_EVALUATIONS
ROUTING_PROVIDER_CALLS <= MAX_ROUTING_PROVIDER_CALLS
CACHEABLE_ROUTE_LEGS=YES
GROUPAGE_WORK_STRUCTURALLY_BOUNDED=YES
```

Cache reuse is inside one search, only when `RouteLegKey` matches, and only before `ExpiresAt`. Cache does not cross requests and does not let the search evaluate more sequences than the structural caps allow. `RouteLegKey` is `from_location_id + to_location_id + vehicle_profile_hash + traffic_mode + departure_bucket`, plus route mode when the existing fingerprint includes it.

## Routing semantics

Road distance and road duration only. Haversine is not a road metric (`HaversineCanonicalRoad=false`). Traffic mode for planning is `STATISTICAL` when a departure time is known, otherwise `CURRENT`, using the existing port modes. Vehicle profile is the trusted vehicle or capacity profile. Provider timeout, unavailable, not found, and invalid response all fail that sequence as `ROUTING_UNAVAILABLE`. They do not become distance 0.

```text
ROAD_ROUTE_UNKNOWN != ZERO_DISTANCE
HAVERSINE_FOR_ROAD_METRICS=NO
ROAD_ROUTING_REQUIRED=YES
```

## Onboard intervals

```text
CURRENT_TRIP_INITIAL_ONBOARD_SET = trusted CurrentTripContext confirmed onboard cargo
COMPLETED_PICKUP_HISTORY_IMMUTABLE=YES
SYNTHETIC_PICKUP_FOR_ALREADY_ONBOARD_CARGO=NO
INITIAL_CAPACITY_SOURCE=CURRENT_TRIP_CONTEXT
```

At `START`, the onboard set is the confirmed onboard cargo. Historical pickups are not rewritten as future pickup actions. Then each future action is applied in order. `PICKUP` adds that subject. `DELIVERY` removes that subject. Compatibility, temperature, ADR, food-grade, and odor use the set that is onboard for the following leg, through `EvaluateGroupageItems`. Unknown required data is `INDETERMINATE`, not feasible. A hard groupage reject on any leg rejects that sequence.

The formula "pickup ordinal at or before the leg and delivery after the leg" does not seed cargo that was already loaded before this plan starts. It is not the current-trip rule.

The `START` capacity snapshot is the trusted NLO-0.3C residual. The caller does not supply it. The sequence is `START_RESIDUAL`, then action, then `RouteCapacitySnapshot`, repeated. If a required residual dimension is `UNKNOWN`, `PLAN_RESULT=INDETERMINATE`. A depot-start plan starts from trusted vehicle capacity and an empty onboard set, not from caller occupancy.

## Time

Forward propagation only.

```text
arrival(stop 1) = planning start
arrival(stop n+1) = departure(stop n) + leg duration
if arrival < window_start: wait until window_start
if arrival > window_end: HARD_REJECT STOP_WINDOW_VIOLATION
departure = max(arrival, window_start) + service_duration
```

Waiting is supported. Late arrival is `HARD_REJECT`. There is no backward scheduling and no search over wait choices.

```text
SERVICE_DURATION_SOURCE=AUTHORITATIVE_OR_UNKNOWN
DEFAULT_ZERO=NO
```

No shipment field, dwell table, or versioned duration policy exists. `OPEN_QUESTIONS.md` Q2 records that absence. Cargo-action service duration is `UNKNOWN`. Zero is not assumed. `TIME_FEASIBILITY=INDETERMINATE` and `PLAN_RESULT=INDETERMINATE`. An indeterminate plan may be returned as advice. `ACTIVATION_ALLOWED=NO` until a later accepted policy names an owner, a version, a pickup duration, a delivery duration, and a rationale. This acceptance does not invent those numbers. `START` and `END` have no cargo service duration.

```text
NLO04B_ADVISORY_PLANNING_WITH_UNKNOWN_DURATION_ALLOWED=YES
NLO04C_PRODUCTION_ACTIVATION_REQUIRES_SERVICE_DURATION_SOURCE=YES
NLO_0_4C_ACTIVATION_RELEASE_BLOCKED_UNTIL_SERVICE_DURATION_SOURCE=YES
```

The window of a stop with several actions is the intersection of those action windows. An empty intersection is `STOP_WINDOW_VIOLATION` before routing.

## Search completeness

The configured heuristic can finish, or a structural or time budget can stop it. Those are different results. Finishing the heuristic does not exhaust the sequence space, so it does not prove global infeasibility.

```text
SEARCH_ALGORITHM=BOUNDED_INCREMENTAL_HEURISTIC
GLOBAL_INFEASIBILITY_PROVEN=NO
NO_FEASIBLE_SEQUENCE_PUBLIC_USED=NO
```

Public results:

```text
FEASIBLE_PLAN_FOUND
INDETERMINATE_PLAN_FOUND
NO_PLAN_FOUND_WITHIN_POLICY
SEARCH_BUDGET_EXHAUSTED
ROUTING_UNAVAILABLE
PLAN_STALE
```

`NO_PLAN_FOUND_WITHIN_POLICY` means the configured heuristic finished and kept no feasible plan and no indeterminate plan. `SEARCH_BUDGET_EXHAUSTED` means a cap stopped the algorithm before that finish. They are not interchangeable. `NO_FEASIBLE_SEQUENCE` is not a public result for this heuristic. A later algorithm may use it only if that algorithm exhausts the complete authorized combinatorial space.

`ROUTING_UNAVAILABLE` is the result when the provider cannot route and the heuristic therefore has no plan. A single sequence that fails routing is discarded. It does not by itself end the search.

Stable detail reasons, with no private rule text:

```text
STOP_WINDOW_VIOLATION
CAPACITY_EXCEEDED_AT_STOP
CARGO_COMPATIBILITY_FAILED_ON_LEG
SERVICE_DURATION_UNKNOWN
MAX_STOPS_EXCEEDED
PLAN_LOAD_LIMIT_EXCEEDED
SEARCH_BUDGET_EXCEEDED
```

`PICKUP_AFTER_DELIVERY` remains a generator invariant. The generator must not emit it, and it is not a normal public reason. `PLAN_LOAD_LIMIT_EXCEEDED` is decided before insertion starts.

## First release answer

```text
NLO_0_4_FIRST_RELEASE_MAX_ADDITIONAL_LOADS=2
NLO_0_4_FIRST_RELEASE_MAX_STOPS=8
```

Two additional loads are the first release. Three are not. The 4-stop route makes three loads cost 676 uncached route legs and 88 sequences, past both the routing cap and the sequence cap. Two loads stay inside both.
