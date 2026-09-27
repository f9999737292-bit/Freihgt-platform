# NLO-0.3E bounded expansion model

This is an architecture freeze. It does not implement a search.

```text
UNRESTRICTED_SUBSET_ENUMERATION=FORBIDDEN
UNRESTRICTED_ROUTE_PERMUTATION=FORBIDDEN
```

NLO-0.3E must not loop over every subset of N loads and must not loop over N! stop orders.

## Chosen algorithm

Option A: fixed maximum set size with incremental lexicographic extension.

Loads in one canonical origin-destination group are sorted by load id. Sets are generated in lexicographic order of those ids, from size 2 up through `MAX_SET_SIZE`. Generation stops when `MAX_SETS_EVALUATED` is reached, when the group is exhausted under the size cap, or when the routing budget is reached. Each set is assessed once. Order of assessment is the generation order, then the existing status rank for the returned page. Status rank is not a commercial score.

Work is bounded by `MAX_SETS_EVALUATED`, not by `2^N`. Generating the next combination is linear in set size.

Option B, bounded beam or frontier expansion, is rejected. A beam needs a score to decide which partial sets survive. No consolidation score is authorized.

Option C, deterministic top-K seed expansion, is rejected for the same reason. Top-K without a score is an arbitrary cut. Lexicographic order already defines a deterministic cut without weights.

| Option | Complexity | Determinism | Audit | Routing cost | Compatibility cost | Privacy | Implementation |
| --- | --- | --- | --- | --- | --- | --- | --- |
| A lex extension | O(sets evaluated), sets capped | load-id order | each assessed set can be stored | one canonical sequence per set | one groupage pass per set, two contexts when two tenants | same fail-closed cross-shipper rule | extends the current pair loop |
| B beam | O(beam * set size) | only if the score is total | needs the score in the fingerprint | depends on the beam | depends on the beam | score must not leak private facts | needs an unapproved score |
| C top-K seeds | O(K * extensions) | only if the rank is total | same | depends on K | depends on K | same | needs an unapproved score |

No VRP, MILP, CP-SAT, LNS, genetic algorithm, or ML.

## Contract fields

`candidate_limit` stays the returned-result cap. It is applied after evaluation today. NLO-0.3E must not reuse it as a compute budget.

These server-owned policy fields are separate. The caller does not supply them.

| Field | Meaning | Proposed value | Evidence | Safety margin | Failure | Owner | Default until accepted |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `MAX_CANDIDATE_POOL` | loads read from the public pool before search stops | UNSET | In-memory 500-load group produced 124750 pairs, ~277 MB JSON, ~1.7 GB heap, ~11 s. Postgres time unknown | not chosen | `POOL_LIMIT_EXCEEDED`, no partial feasible claim | server policy, versioned | implementation blocked |
| `MAX_SET_SIZE` | largest set the lex generator emits | UNSET | Schema and OpenAPI allow ordinals 1 and 2 only. No N-member timing exists | not chosen | generator refuses a larger set | server policy | implementation blocked |
| `MAX_SETS_EVALUATED` | assessments per search | UNSET | Pairwise evaluates every pair. 500 loads evaluated 124750 sets | not chosen | `SEARCH_BUDGET_EXCEEDED` | server policy | implementation blocked |
| `MAX_ROUTING_CALLS` | `roadLeg` calls per search | UNSET | Fill uses 4 calls per fresh candidate, including hard rejects. Provider time was not 2GIS | not chosen | `ROUTING_BUDGET_EXCEEDED` | server policy | implementation blocked |
| `MAX_GROUPAGE_CALLS` | `EvaluateGroupageItems` calls per search | UNSET | Pairwise used 2 calls per pair. 500 loads used 249500 calls | not chosen | `SEARCH_BUDGET_EXCEEDED` | server policy | implementation blocked |
| `TIME_BUDGET` | wall clock cap | UNSET | Local p95 is not an SLO | not chosen | `SEARCH_BUDGET_EXCEEDED` | server policy | implementation blocked |
| `DETERMINISTIC_ORDER` | generation and tie break | load id lexicographic, then status rank for the page | Current pairwise sort is status rank then load ids. Fingerprints were stable across repeats | structural | a changed order changes the fingerprint | frozen in ADR-NET-017 | required |
| `ROUTE_SEQUENCE_BOUND` | sequences considered per set | 1 | See sequence model. This is a structural cap, not a timing SLO | the cap is the algorithm | a second sequence is out of policy | frozen in ADR-NET-017 | 1 |

```text
IMPLEMENTATION_BLOCKED=YES
```

for every numeric production threshold above. A later controller decision may set them from a Postgres measurement and a real routing budget. This discovery does not.

## Route sequence

For a set of K additional loads on a current trip, NLO-0.3E considers one sequence:

1. Current fresh position.
2. Additional pickups in load-id order.
3. Additional deliveries in the same load-id order.
4. The trip destination last.

Precedence is pickup before delivery for each load. Loads are not interleaved. Existing onboard cargo is not reordered into a new stop list. `SEQUENCES_PER_SET=1`. The maximum number of route sequences in a search is `MAX_SETS_EVALUATED`, which is unset, and never `N!`.

Road calls for that one sequence, including the direct comparison leg used today, are `2*K + 2` when K is the additional-load count and both ends resolve (`direct`, then each hop of the chain). For K=1 that is the four calls `insertRoad` makes now. If the position is stale, routing calls stay 0 and the candidate is not feasible.

Window propagation uses the summed provider durations along that single chain, the same way `insertRoad` derives pickup and delivery instants today. A missing window stays indeterminate. Rehandling stays the existing policy (`REHANDLING_FORBIDDEN` or `ALLOWED`) inside the fingerprint. A new stop is planning-only. It does not create a `RouteStop`.

## Residual capacity

A multi-load set reuses `EvaluateGroupageItems` and `ResidualCapacitySnapshot`. There is no second compatibility engine.

Required facts stay: weight, volume, pallets, linear metres, height, temperature, ADR, food grade, loading and unloading access, odor, contamination, and catalog or rule compatibility. Aggregation is the existing groupage sum and intersection. An unknown fact stays `INDETERMINATE`. Unknown is never zero, never false, and never feasible.

## Problem classes

| Class | Decision | Bound |
| --- | --- | --- |
| `SAME_ORIGIN_SAME_DESTINATION` with more than 2 loads | `IN_0_3E` as `PLAN_ONLY` | lex sets, `execution_supported=false` |
| `CURRENT_TRIP_FILL` with more than 1 additional load | `IN_0_3E` as `PLAN_ONLY` | one canonical sequence, `execution_supported=false` |
| `MULTI_PICK_ONE_DROP` | `DEFER_TO_0_4` | needs `RoutePlan` and `RouteStop` |
| `ONE_PICK_MULTI_DROP` | `DEFER_TO_0_4` | same |
| `MULTI_PICK_MULTI_DROP` | `DEFER_TO_0_4` | same |

NLO-0.4 owns persistent `RoutePlan`, `RouteStop`, shared route legs, accepted plan activation, multi-stop shipment representation, and driver multi-stop tasks. NLO-0.3E does not create those rows.

## Resource exhaustion

A client `candidate_limit`, including 0 or a very large value, must not increase work. The server policy does.

Fail closed, with no feasible result invented from a partial search:

```text
POOL_LIMIT_EXCEEDED
SEARCH_BUDGET_EXCEEDED
ROUTING_BUDGET_EXCEEDED
```

Today those codes are not implemented. A large visible pool is still read in full, and a large same-origin group is still fully paired. That is the defect this bound is meant to close. It is not closed in this discovery.

Repeated identical searches each allocate a new search id and insert a new run. There is no cache. A cache is not authorized here. Freshness, catalog version, load version, shipment version, tracking, and routing fingerprint would all have to invalidate one. Audit prefers a new run over a reused body.

## Idempotency

Identical planning input creates a new search run. Candidate fingerprints are stable for identical inputs, which the benchmark checked. Reuse of a previous run is not the contract.
