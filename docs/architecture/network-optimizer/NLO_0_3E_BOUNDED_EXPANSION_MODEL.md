# NLO-0.3E bounded expansion model

Controller review R3 is a freeze candidate. It is not accepted and it does not implement a search.

```text
NLO03E_F001=CLOSED
NLO03E_F002=CLOSED
ADR_STATUS=PROPOSED
ARCHITECTURE_FREEZE_CANDIDATE=YES
TEST_STRATEGY_FREEZE_CANDIDATE=YES
ARCHITECTURE_FROZEN=NO
IMPLEMENTATION_AUTHORIZED=NO
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

| Field | Meaning | Value | Evidence | Safety margin | Failure | Owner |
| --- | --- | --- | --- | --- | --- | --- |
| `MAX_CANDIDATE_POOL` | global count of visible eligible loads | 10 | Postgres pool 10: p50 150 ms, observed max 196 ms, 136 inserted rows, 5 samples. 136 is the row count, not the latency | pool 50 size-2 observed max was 2846 ms; pool 100 was 10431 ms; pool 500 was 445323 ms | `POOL_LIMIT_EXCEEDED` before enumeration | server policy |
| `MAX_SET_SIZE` | largest cargo set | 3 | Explicit first-release bound. The pairwise benchmark measured 2-member groupage only. Size 4 is 375 sets and size 5 is 627; neither was timed | one step above pairs, below the unmeasured 4- and 5-member widths | generator does not emit a larger set | server policy |
| `MAX_SETS_EVALUATED` | assessments per search | 165 | `C(10,2)+C(10,3)=45+120` when every load shares one origin and destination. That is the worst case inside a global pool of 10 | not the pool-50 pair count of 1225, which this pool cannot reach | `SEARCH_BUDGET_EXCEEDED` before the next set | server policy |
| `MAX_ROUTING_CALLS` | `roadLeg` calls per NLO-0.3E search | 0 | Zero route sequences. Postgres routing calls were 0 | not applicable | this wave does not call a provider | server policy |
| `MAX_GROUPAGE_CALLS` | `EvaluateGroupageItems` calls per search | 330 | 165 sets times 2 contexts (capacity tenant and load tenant) | a single shared tenant uses fewer calls; 330 is the maximum | `SEARCH_BUDGET_EXCEEDED` before the next call | server policy |
| `TIME_BUDGET` | watchdog | 5s | Pool-50 size-2 observed max 2846 ms; pool-100 size-2 observed max 10431 ms | about 1.8 times the pool-50 maximum. Not a throughput promise and not an SLO | `SEARCH_BUDGET_EXCEEDED` | server policy |
| `DETERMINISTIC_ORDER` | generation and tie break | load id lexicographic, then status rank for the page | Fingerprints were stable across repeats | structural | a changed order changes the fingerprint | proposed |
| `ROUTE_SEQUENCES_EXPLORED` | stop orders per set | 0 | Same pickup and delivery location. F002 is closed | structural | a stop list is out of NLO-0.3E | proposed |

```text
POOL_BOUND_SCOPE=GLOBAL_VISIBLE_ELIGIBLE_POOL
BUDGETS_MUTUALLY_COHERENT=YES
TIME_BUDGET_IS_SLO=NO
NLO03E_I001=RESOLVED
NLO03E_I002=RESOLVED_AT_DESIGN_LEVEL
NLO03E_I003=RESOLVED_AT_DESIGN_LEVEL
IMPLEMENTATION_AUTHORIZED=NO
```

Structural counts are the primary budget. The 5 second watchdog does not raise the set count or the groupage count. Pool 50, 100, and 500 remain measurements of today's unbounded pairwise search. They are not the NLO-0.3E ceiling.

## N-member cost model

`TestNLO03ECostModel` (build tag `nlo03ediscovery`) asserts these counts. It does not generate production sets. Pool is 10. Two compatibility contexts are the maximum per set. One `EvaluateGroupageItems` call on K members is not treated as the same CPU work as a call on 2 members. Inside each context the current evaluator runs K cargo-versus-equipment checks and `C(K,2)` cargo-pair checks.

| Max set size | Sets | Candidate rows | Member rows | Persisted rows | Groupage calls | Inner checks | Estimated response bytes |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 3 | 165 | 165 | 450 | 616 | 330 | 1710 | 520455 |
| 4 | 375 | 375 | 1290 | 1666 | 750 | 5910 | 1491971 |
| 5 | 627 | 627 | 2550 | 3178 | 1254 | 13470 | 2949245 |

Persisted rows are 1 run plus candidates plus members. Inner checks are `sets * 2 * (K + C(K,2))` summed over K from 2 through the max. The response estimate scales the measured 104091 byte size-2 body by member count (90 members in that measurement). It is not a new timed payload.

`MAX_SET_SIZE=3` is the choice. It is a first-release product and safety bound. The pairwise benchmark did not time a 3-member `EvaluateGroupageItems` call, so it does not mathematically prove that 3 is the largest safe width. Size 3 keeps the audit at 616 rows and the inner checks at 1710. Size 4 multiplies inner checks by about 3.5 and size 5 by about 7.9, with a wider public member list and a harder explanation for an operator. The safety margin is refusing those unmeasured widths.

## Pool bound

`MAX_CANDIDATE_POOL` means `GLOBAL_VISIBLE_ELIGIBLE_POOL=10`. It is not a per-origin-destination cap.

The first bound is the repository read, before any set is built. The proposed operation, not implemented here, keeps the current visibility and opt-in predicates and adds a deterministic limit:

```text
ORDER BY pickup_location_id, delivery_location_id, id
LIMIT 11
```

Eleven is one more than the bound. If the eleventh row exists, the search returns `POOL_LIMIT_EXCEEDED` and discards the page. It does not evaluate candidates and it does not insert a completed run. If ten or fewer rows return, those rows are the whole eligible pool. Grouping them by canonical origin and destination cannot scan an unbounded marketplace, and it cannot create more than ten groups. The worst case is one group of ten, which is 165 sets. A split across groups produces fewer sets.

```text
UNBOUNDED_POOL_MATERIALIZATION=NO
UNBOUNDED_GROUP_SCAN=NO
POOL_BOUND_APPLIED_BEFORE_COMBINATORIAL_ENUMERATION=YES
```

## Budget checks and failure

`candidate_limit` is `RESPONSE_LIMIT_ONLY`. It does not change pool reads, sets generated, sets evaluated, groupage calls, or rows written. Limit 0 and a large limit are the same compute.

Before enumeration, the pool bound is checked. Before a set is evaluated, the search checks that one more set would still be within 165 and that the contexts that set would load would still be within 330. It does not start a set that would cross either count. Elapsed time is checked before each set. At or past 5 seconds the search does not start another set.

If a structural count is already exhausted, that failure wins over the watchdog. Both set count, groupage count, and the watchdog use `SEARCH_BUDGET_EXCEEDED`. Only the pool read uses `POOL_LIMIT_EXCEEDED`. `ROUTING_BUDGET_EXCEEDED` is not an NLO-0.3E code.

```text
PARTIAL_RESULT_PRESENTED_AS_COMPLETE=NO
BUDGET_FAILURE_ATOMICITY=ROLLED_BACK
```

Candidate, member, and run rows for the attempt stay in the search transaction and roll back. The current runs table allows only `status = 'COMPLETED'`, so a failed attempt is not stored as a completed search and is not stored as a partial candidate list. The HTTP result uses the existing validation error, HTTP 422, with `details.reason` set to the budget code. There is no 200 body of candidates.

## Provenance

An NLO-0.3E fingerprint covers capacity id and version, member load ids and versions, cargo or profile versions that the evaluation consumed, catalog versions, rule-set versions, compatibility fingerprints, policy version, algorithm and budget policy version, and the canonical origin, destination, and window inputs through their authoritative versions.

It does not depend on shipment version, tracking position, tracking freshness, ETA, a routing request fingerprint, routing proof, or a multi-stop rehandling sequence. Those belong to current-trip fill, which this wave does not extend. NLO-0.3D provenance is unchanged.

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

`POST /v1/network/consolidation/search` stays the only search route. A same-origin candidate lists 2 or 3 members (`minItems` 2, `maxItems` 3). `ordinal` is an integer from 1 through 3. A two-member response remains valid. `ordinal` is unique within the candidate, and so is `load_opportunity_id`. The existing primary key `(candidate_id, ordinal)`, unique `(candidate_id, load_opportunity_id)`, and candidate foreign key stay. `execution_supported` is false. The public body has no stop list.

Today the member check is `ordinal IN (1, 2)` and OpenAPI `maxItems` is 2. A later migration may replace the check with `ordinal BETWEEN 1 AND 3` and the generated contract may set `maxItems` to 3. This discovery does not add that migration and does not reserve `000084`.

The down migration must not delete ordinal-3 rows to force the old check. If any member ordinal is outside 1..2, the down migration fails closed and leaves the rows in place.

```text
DOWN_FAILS_CLOSED_IF_ROWS_OUTSIDE_OLD_BOUND=YES
MIGRATION_CREATED=NO
MIGRATION_RESERVED=NO
```

## Resource exhaustion

`candidate_limit` is a response cap only. It does not change pool reads, set generation, groupage calls, or persistence. The server pool, set, groupage, and time budgets do.

Fail closed. A partial candidate list is not returned as a complete search.

```text
POOL_LIMIT_EXCEEDED
SEARCH_BUDGET_EXCEEDED
```

`ROUTING_BUDGET_EXCEEDED` is not produced by NLO-0.3E. NLO-0.3D still calls the routing provider for one additional load. That behavior is unchanged.

These codes are not implemented yet. The current pool read has no limit. This discovery does not change that query.

Repeated identical searches each allocate a new search id. There is no cache. A cache is not authorized here. Invalidation for this wave is capacity, load, catalog, rule, policy, and budget-policy versions, plus the canonical origin, destination, and windows. Shipment version, tracking, and routing fingerprints are not NLO-0.3E inputs. Audit prefers a new run over a reused body.

## Idempotency

Identical planning input creates a new search run. Candidate fingerprints are stable for identical inputs, which the benchmark checked. Reuse of a previous run is not the contract.
