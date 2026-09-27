# NLO-0.4A multi-stop algorithm

Status: proposed. Not a production planner. The counts below are asserted by `services/network-optimizer-service/internal/nlo04adiscovery/cost_model_test.go`.

```text
SEQUENCING_ALGORITHM=INCREMENTAL_COST_MIN_INSERTION
UNRESTRICTED_PERMUTATION_SEARCH=NO
SOLVER=NONE
FIRST_RELEASE_MAX_ADDITIONAL_LOADS=2
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

Loads are considered in load-id order. After a load is placed, only the winning sequence is kept. The next load is inserted into that one sequence. This is incremental insertion, not the Cartesian product of all loads.

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

Uncached routing calls count every leg of every evaluated sequence. Unique legs use `from_location + to_location` inside that one search. The test keeps the first sequence as the parent, which does not change the candidate or leg counts.

| Scenario | Candidates | Legs without cache | Unique legs | Stops after |
| --- | --- | --- | --- | --- |
| 2 future stops + 1 load | 6 | 18 | 10 | 4 |
| 2 future stops + 2 loads | 21 | 93 | 27 | 6 |
| 2 future stops + 3 loads | 49 | 289 | 52 | 8 |
| 4 future stops + 2 loads | 43 | 271 | 45 | 8 |
| 4 future stops + 3 loads | 88 | 676 | 78 | 10 |

NLO-0.3D today spends 4 route calls on one fixed sequence for one load. The table is the search amplification once insertion positions are enumerated.

Compatibility upper bound for the accepted case (4 future stops, 2 loads): 43 sequences, and the longer sequences have 7 legs. Two catalog contexts, the same ceiling NLO-0.3 uses when owners differ, gives at most `43 * 7 * 2 = 602` groupage evaluations if every leg has a distinct onboard set. Real sets repeat. The groupage engine remains `EvaluateGroupageItems`. There is no second rules engine.

## Proposed server limits

| Limit | Value | Why | Fail |
| --- | --- | --- | --- |
| `MAX_ADDITIONAL_LOADS` | 2 | On a 4-stop future route, 3 loads cost 676 uncached legs. 2 loads cost 271 | `SEARCH_BUDGET_EXCEEDED` `budget=loads` before the third load is opened |
| `MAX_STOPS` | 8 | 4 stops plus 2 loads end at 8 stops. 3 loads end at 10 | `budget=stops` |
| `MAX_ACTIVE_LOADS_PER_PLAN` | 4 | Two confirmed onboard loads plus two additional loads | `budget=loads` |
| `MAX_SEQUENCE_CANDIDATES` | 64 | The 2-load rich search evaluates 43 sequences. The 3-load rich search evaluates 88 | `budget=sequences` |
| `MAX_ROUTING_CALLS` | 300 | 271 fits. 676 does not. The cap is calls, not a latency promise | `budget=routing` |
| `TIME_BUDGET` | 5s | Watchdog, same role as NLO-0.3E. `TIME_BUDGET_IS_SLO=NO` | `budget=time` |
| `MAX_PICKUPS` | 2 | New pickup actions created by this search | `budget=pickups` |
| `MAX_DELIVERIES` | 2 | New delivery actions created by this search | `budget=deliveries` |
| `MAX_ACTIONS_PER_STOP` | 4 | Shared warehouse cap | `budget=actions` |

`MAX_ROUTING_CALLS=300` alone would still allow the empty-route 3-load case (289 legs). The load cap of 2 is therefore structural, not only a call cap. No limit is `UNLIMITED`.

`ROUTING_CALLS_PER_SEQUENCE` equals the number of adjacent pairs in that sequence. `MAX_ROUTING_CALLS_PER_SEARCH=300`. `CACHEABLE_ROUTE_LEGS=YES` inside one search when `RouteLegKey` matches. Cache does not cross requests and does not survive `ExpiresAt`.

## Routing semantics

Road distance and road duration only. Haversine is not a road metric (`HaversineCanonicalRoad=false`). Traffic mode for planning is `STATISTICAL` when a departure time is known, otherwise `CURRENT`, using the existing port modes. Vehicle profile is the trusted vehicle or capacity profile. Provider timeout, unavailable, not found, and invalid response all fail that sequence as `ROUTING_UNAVAILABLE`. They do not become distance 0.

```text
ROAD_ROUTE_UNKNOWN != ZERO_DISTANCE
HAVERSINE_FOR_ROAD_METRICS=NO
ROAD_ROUTING_REQUIRED=YES
```

## Onboard intervals

Load A overlaps load B on a leg when A's pickup is at or before the start of the leg and A's delivery is after the start of the leg, and the same is true for B. Compatibility, temperature, ADR, food-grade, and odor use that set and the existing groupage engine. Unknown required data is `INDETERMINATE`, not feasible. A hard groupage reject on any overlapping interval rejects the sequence.

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

The window of a stop with several actions is the intersection of those action windows. An empty intersection is `STOP_WINDOW_VIOLATION` before routing.

## Search completeness

```text
SEARCH_COMPLETE
SEARCH_BUDGET_EXHAUSTED
```

`NO_FEASIBLE_SEQUENCE` is allowed only when every pair inside the caps was evaluated and none was feasible. If any cap stops the search, the result is `SEARCH_BUDGET_EXHAUSTED` and `NO_FEASIBLE_SEQUENCE` is forbidden.

Public reason codes, with no private rule text:

```text
STOP_WINDOW_VIOLATION
CAPACITY_EXCEEDED_AT_STOP
CARGO_COMPATIBILITY_FAILED_ON_LEG
ROUTING_UNAVAILABLE
PICKUP_AFTER_DELIVERY
MAX_STOPS_EXCEEDED
SEARCH_BUDGET_EXCEEDED
PLAN_STALE
```

`PICKUP_AFTER_DELIVERY` is a generator invariant failure, not a normal candidate. The generator must not emit it.

## First release answer

```text
NLO_0_4_FIRST_RELEASE_MAX_ADDITIONAL_LOADS=2
NLO_0_4_FIRST_RELEASE_MAX_STOPS=8
```

Two additional loads are the first release. Three are not. The 4-stop route makes three loads cost 676 uncached route legs and 88 sequences, past both the routing cap and the sequence cap. Two loads stay inside both.
