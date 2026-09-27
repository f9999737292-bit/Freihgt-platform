# NLO-0.3E bounded expansion model

Controller review R1 revised this model. It is not a frozen architecture. It does not implement a search.

```text
NLO03E_F001=ADDRESSED_IN_THIS_REVISION
NLO03E_F002=ADDRESSED_IN_THIS_REVISION
ARCHITECTURE_FROZEN=NO
```

```text
UNRESTRICTED_SUBSET_ENUMERATION=FORBIDDEN
UNRESTRICTED_ROUTE_PERMUTATION=FORBIDDEN
```

NLO-0.3E must not loop over every subset of N loads and must not loop over N! stop orders.

## Chosen algorithm

Option A: fixed maximum set size with incremental lexicographic extension.

Loads in one canonical origin-destination group are sorted by load id. Sets are generated in lexicographic order of those ids, from size 2 up through `MAX_SET_SIZE`. Generation stops when `MAX_SETS_EVALUATED` is reached or when the group is exhausted under the size cap. Each set is assessed once as cargo at that shared pickup and shared delivery. There is no stop order to search. Order of assessment is the generation order, then the existing status rank for the returned page. Status rank is not a commercial score.

Work is bounded by `MAX_SETS_EVALUATED`, not by `2^N`. Generating the next combination is linear in set size.

Option B, bounded beam or frontier expansion, is rejected. A beam needs a score to decide which partial sets survive. No consolidation score is authorized.

Option C, deterministic top-K seed expansion, is rejected for the same reason. Top-K without a score is an arbitrary cut. Lexicographic order already defines a deterministic cut without weights.

| Option | Complexity | Determinism | Audit | Routing cost | Compatibility cost | Privacy | Implementation |
| --- | --- | --- | --- | --- | --- | --- | --- |
| A lex extension | O(sets evaluated), sets capped | load-id order | each assessed set can be stored | no route-sequence search | one groupage pass per set, two contexts when two tenants | same fail-closed cross-shipper rule | extends the current pair loop |
| B beam | O(beam * set size) | only if the score is total | needs the score in the fingerprint | depends on the beam | depends on the beam | score must not leak private facts | needs an unapproved score |
| C top-K seeds | O(K * extensions) | only if the rank is total | same | depends on K | depends on K | same | needs an unapproved score |

No VRP, MILP, CP-SAT, LNS, genetic algorithm, or ML.

## Contract fields

`candidate_limit` stays the returned-result cap. It is applied after evaluation today. NLO-0.3E must not reuse it as a compute budget.

These server-owned policy fields are separate. The caller does not supply them.

| Field | Meaning | Proposed value | Evidence | Safety margin | Failure | Owner | Default until accepted |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `MAX_CANDIDATE_POOL` | eligible loads read before an N-member search stops | 10 | Postgres pool 10: observed max 136 ms, 136 inserted rows. Sizes 2 and 3 together are 165 sets, inside the 1225-set measurement | pool 50 is the next measured tier and cannot finish size 3 inside 1225 sets | `POOL_LIMIT_EXCEEDED`, no partial feasible claim | server policy | 10 |
| `MAX_SET_SIZE` | largest set the lex generator emits | 3 | Pairwise groupage is 2 loads. Size 3 is the only wider set whose full enumeration at pool 10 stays inside the measured call count. Size 4+ was not measured | call count capped at the pool-50 measurement | generator refuses a larger set | server policy | 3 |
| `MAX_SETS_EVALUATED` | assessments per search | 1225 | Postgres pool 50 evaluated 1225 pairs, observed max 2846 ms, 3676 inserted rows | pool 100 evaluated 4950 pairs, observed max 10431 ms, 14851 rows, and is not the default | `SEARCH_BUDGET_EXCEEDED` before a partial feasible claim | server policy | 1225 |
| `MAX_ROUTING_CALLS` | `roadLeg` calls per NLO-0.3E search | 0 | Same-origin expansion explores 0 route sequences. Postgres pairwise routing calls were 0 | not applicable | this wave does not call a provider | server policy | 0 |
| `MAX_GROUPAGE_CALLS` | `EvaluateGroupageItems` calls per search | 2450 | Postgres pool 50 loaded 2450 catalog contexts, one groupage evaluation each | pool 100 used 9900 and is not the default | `SEARCH_BUDGET_EXCEEDED` | server policy | 2450 |
| `TIME_BUDGET` | wall clock cap | 5s | Adopted comparison tier, pool 50, observed max 2846 ms on this disposable Postgres | about 1.8× that sample maximum; pool 100 observed max was 10431 ms; not a multi-host SLO | `SEARCH_BUDGET_EXCEEDED` | server policy | 5s |
| `DETERMINISTIC_ORDER` | generation and tie break | load id lexicographic, then status rank for the page | Current pairwise sort is status rank then load ids. Fingerprints were stable across repeats | structural | a changed order changes the fingerprint | proposed in ADR-NET-017, not accepted | required for a later implementation |
| `ROUTE_SEQUENCES_EXPLORED` | stop orders considered per set | 0 | Same-origin same-destination has one pickup location and one delivery location. F002: one order cannot prove every order infeasible | structural | emitting a stop list is out of NLO-0.3E | proposed in ADR-NET-017, not accepted | 0 |

```text
NLO03E_I001=ADDRESSED_IN_THIS_REVISION
NLO03E_MAX_ROUTING_CALLS=0
REAL_ROUTING_PROVIDER_BUDGET_REQUIRED_FOR_0_3E=NO
NLO03E_I002=CONTRACT_SHAPE_FROZEN_MIGRATION_NOT_WRITTEN
NLO03E_I003=OPEN
IMPLEMENTATION_AUTHORIZED=NO
```

The numbers are proposed server defaults for a later implementation. This revision does not change runtime behavior. `candidate_limit` is still not a compute budget.

A same-origin group larger than 10 loads is `POOL_LIMIT_EXCEEDED` for an N-member search. Sizes 2 and 3 of a group of 10 are 165 sets. That is inside the 1225 sets measured at pool 50. A group of 50 cannot be fully enumerated at set size 3 without exceeding that set budget, so it is not the N-member default. Pool 50 itself stayed under 5s (observed max 2846 ms) for pairs only. Pool 100 did not (observed max 10431 ms). Pool 500 took 445323 ms in one sample and inserted 374251 rows.

## Route sequence

NLO-0.3E explores zero route sequences (`NLO03E_F001`, `NLO03E_F002`).

A same-origin same-destination set uses the one pickup location and the one delivery location already shared by every member. Window overlap is the intersection of those known windows, as in pairwise search. It is not a permutation of stops. Rehandling is not evaluated as a new stop order. No `RouteStop` is planned.

`CURRENT_TRIP_FILL` with a second additional load would be a second pickup and a second delivery. That is multi-stop planning and belongs to NLO-0.4. NLO-0.3D stays at one additional load. The four `insertRoad` calls measured for that one load are NLO-0.3D behavior. They are not a template for a longer chain in NLO-0.3E.

`N!` enumeration stays forbidden. A cap of one canonical sequence is also rejected: failure of that sequence would not prove the set infeasible, and success would still be a multi-stop plan. Sequence-dependent failure in any later wave stays `INDETERMINATE` until an authorized bound is exhausted. Sequence-independent groupage failure may be `HARD_REJECT`. Unknown is not zero.

## Residual capacity

A multi-load set reuses `EvaluateGroupageItems` and `ResidualCapacitySnapshot`. There is no second compatibility engine.

Required facts stay: weight, volume, pallets, linear metres, height, temperature, ADR, food grade, loading and unloading access, odor, contamination, and catalog or rule compatibility. Aggregation is the existing groupage sum and intersection. An unknown fact stays `INDETERMINATE`. Unknown is never zero, never false, and never feasible.

## Problem classes

| Class | Decision | Bound |
| --- | --- | --- |
| `SAME_ORIGIN_SAME_DESTINATION` with more than 2 loads | `IN_0_3E` as cargo-set planning only | shared pickup and delivery, no stop list, `execution_supported=false` |
| `CURRENT_TRIP_FILL` with more than 1 additional load | `DEFER_TO_0_4` | a second insertion is multi-stop planning |
| `MULTI_PICK_ONE_DROP` | `DEFER_TO_0_4` | needs `RoutePlan` and `RouteStop` |
| `ONE_PICK_MULTI_DROP` | `DEFER_TO_0_4` | same |
| `MULTI_PICK_MULTI_DROP` | `DEFER_TO_0_4` | same |

NLO-0.4 owns persistent `RoutePlan`, `RouteStop`, shared route legs, accepted plan activation, multi-stop shipment representation, and driver multi-stop tasks. NLO-0.3E does not create those rows.

## N-member contract

This is the contract shape. It is not a migration and it is not a runtime change. `000084` is not created and is not reserved.

```text
NLO_0_3E_SCOPE=SAME_ORIGIN_SAME_DESTINATION_N_MEMBER_SETS_ONLY
CURRENT_TRIP_FILL_MAX_ADDITIONAL_LOADS=1
CURRENT_TRIP_FILL_GT_1=DEFER_TO_0_4
MULTI_PICK=DEFER_TO_0_4
MULTI_DROP=DEFER_TO_0_4
ROUTE_SEQUENCES_EXPLORED_BY_0_3E=0
MULTI_STOP_PLANNING_IN_0_3E=NO
MAX_SET_SIZE=3
```

`POST /v1/network/consolidation/search` stays the only search route. A same-origin candidate may list 2 or 3 members. `ordinal` is an integer from 1 through 3, unique within the candidate. `load_opportunity_id` stays unique within the candidate. `execution_supported` is false. The public body has no stop list.

Today `consolidation_candidate_members` checks `ordinal IN (1, 2)` and OpenAPI `members.maxItems` is 2. A later migration may widen the check to `ordinal BETWEEN 1 AND 3` and the generated contract may set `maxItems` to 3. Until that migration exists, N-member rows cannot be stored (`NLO03E_I002` remains the schema gap; the shape above is the freeze).

## Resource exhaustion

A client `candidate_limit`, including 0 or a very large value, must not increase work. The server policy does.

Fail closed, with no feasible result invented from a partial search:

```text
POOL_LIMIT_EXCEEDED
SEARCH_BUDGET_EXCEEDED
```

`ROUTING_BUDGET_EXCEEDED` is not produced by NLO-0.3E. NLO-0.3D still calls the routing provider for one additional load. That behavior is unchanged.

Today those codes are not implemented. A large visible pool is still read in full, and a large same-origin group is still fully paired. That is the defect this bound is meant to close. It is not closed in this discovery.

Repeated identical searches each allocate a new search id and insert a new run. There is no cache. A cache is not authorized here. Freshness, catalog version, load version, shipment version, tracking, and routing fingerprint would all have to invalidate one. Audit prefers a new run over a reused body.

## Idempotency

Identical planning input creates a new search run. Candidate fingerprints are stable for identical inputs, which the benchmark checked. Reuse of a previous run is not the contract.
