# NLO-0.3E bounded expansion model

Controller acceptance is recorded. This document does not implement a search.

```text
NLO_0_3E_ARCHITECTURE=FROZEN_ACCEPTED
NLO03E_F001=CLOSED
NLO03E_F002=CLOSED
NLO03E_F005=CLOSED
NLO03E_F006=CLOSED
ADR_STATUS=ACCEPTED
ARCHITECTURE_FROZEN=YES
TEST_STRATEGY_FROZEN=YES
CONTROLLER_ACCEPTANCE=PASS
BLOCKING_FINDINGS=0
IMPLEMENTATION_AUTHORIZED=NO
NLO_0_3E_IMPLEMENTATION_STARTED=NO
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
| A lex extension | O(sets evaluated), sets capped | load-id order | each assessed set can be stored | no route-sequence search | one groupage pass per set, two contexts when two tenants | same fail-closed cross-shipper rule | new N-member pattern only; the legacy pair loop stays pairwise |
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

Structural counts are the primary budget. The 5 second watchdog does not raise the set count or the groupage count. These ceilings apply only to `SAME_ORIGIN_SAME_DESTINATION_N_MEMBER`. Pool 50, 100, and 500 remain measurements of today's unbounded pairwise search. They are not the N-member ceiling, and NLO-0.3E does not apply the pool of 10 to the legacy pairwise pattern.

```text
LEGACY_PAIRWISE_UNBOUNDED_BEHAVIOR=UNCHANGED_BY_0_3E
```

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

`MAX_CANDIDATE_POOL` means `GLOBAL_VISIBLE_ELIGIBLE_POOL=10` on `SAME_ORIGIN_SAME_DESTINATION_N_MEMBER` only. It is not a per-origin-destination cap. It is not applied to legacy `SAME_ORIGIN_SAME_DESTINATION`.

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

On the N-member pattern, `candidate_limit` is `RESPONSE_LIMIT_ONLY`. It does not change pool reads, sets generated, sets evaluated, groupage calls, or rows written. Limit 0 and a large limit are the same compute. These prechecks are not a change to legacy pairwise search.

Before enumeration, the pool bound is checked. Before a set is evaluated, the search checks that one more set would still be within 165 and that the contexts that set would load would still be within 330. It does not start a set that would cross either count. Elapsed time is checked before each set. At or past 5 seconds the search does not start another set.

If a structural count is already exhausted, that failure wins over the watchdog. Both set count, groupage count, and the watchdog use `SEARCH_BUDGET_EXCEEDED`. Only the pool read uses `POOL_LIMIT_EXCEEDED`. `ROUTING_BUDGET_EXCEEDED` is not an NLO-0.3E code.

```text
PARTIAL_RESULT_PRESENTED_AS_COMPLETE=NO
BUDGET_FAILURE_ATOMICITY=ROLLED_BACK
```

Candidate, member, and run rows for the attempt stay in the search transaction and roll back. The current runs table allows only `status = 'COMPLETED'`, so a failed attempt is not stored as a completed search and is not stored as a partial candidate list. The HTTP result uses the existing validation error, HTTP 422, with `details.reason` set to the budget code. There is no 200 body of candidates.

## Provenance

An N-member fingerprint covers `pattern=SAME_ORIGIN_SAME_DESTINATION_N_MEMBER`, capacity id and version, member load ids and versions, cargo or profile versions that the evaluation consumed, catalog versions, rule-set versions, compatibility fingerprints, policy version, algorithm and budget policy version, and the canonical origin, destination, and window inputs through their authoritative versions. The pattern value keeps a two-member N-member candidate from colliding with a legacy pairwise candidate.

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
| `SAME_ORIGIN_SAME_DESTINATION` | unchanged pairwise mode | set size 2 only; NLO-0.3E does not widen it |
| `SAME_ORIGIN_SAME_DESTINATION_N_MEMBER` | `IN_0_3E` as cargo-set planning only | shared pickup and delivery, 2 or 3 members, no stop list, `execution_supported=false` |
| `CURRENT_TRIP_FILL` with more than 1 additional load | `DEFER_TO_0_4` | a second insertion is multi-stop planning |
| `MULTI_PICK_ONE_DROP` | `DEFER_TO_0_4` | needs `RoutePlan` and `RouteStop` |
| `ONE_PICK_MULTI_DROP` | `DEFER_TO_0_4` | same |
| `MULTI_PICK_MULTI_DROP` | `DEFER_TO_0_4` | same |

NLO-0.4 owns persistent `RoutePlan`, `RouteStop`, shared route legs, accepted plan activation, multi-stop shipment representation, and driver multi-stop tasks. NLO-0.3E does not create those rows.

## Public patterns

The legacy pairwise pattern stays pairwise. The N-member pattern is additive. NLO-0.3E does not begin returning three-member candidates from `SAME_ORIGIN_SAME_DESTINATION`, and it does not apply the global pool of 10 to that pattern. Hardening the legacy pairwise resource limits needs its own compatibility decision.

```text
LEGACY_PAIRWISE_PATTERN=SAME_ORIGIN_SAME_DESTINATION
LEGACY_PAIRWISE_SET_SIZE=2
PAIRWISE_ONLY=YES
N_MEMBER_PATTERN=SAME_ORIGIN_SAME_DESTINATION_N_MEMBER
N_MEMBER_PATTERN_ADDITIVE=YES
NEW_PUBLIC_ENDPOINT=NO
NEW_PATTERN=YES
LEGACY_PAIRWISE_UNBOUNDED_BEHAVIOR=UNCHANGED_BY_0_3E
NLO_0_3E_SCOPE=SAME_ORIGIN_SAME_DESTINATION_N_MEMBER_SETS_ONLY
CURRENT_TRIP_FILL_MAX_ADDITIONAL_LOADS=1
CURRENT_TRIP_FILL_GT_1=DEFER_TO_0_4
MULTI_PICK=DEFER_TO_0_4
MULTI_DROP=DEFER_TO_0_4
ROUTE_SEQUENCES_EXPLORED_BY_0_3E=0
MULTI_STOP_PLANNING_IN_0_3E=NO
MAX_SET_SIZE=3
```

`POST /v1/network/consolidation/search` stays the only search route.

| Request | Before | After | Change |
| --- | --- | --- | --- |
| Existing `SAME_ORIGIN_SAME_DESTINATION` | pairwise, set size 2 | pairwise, set size 2 | `BREAKING=NO` |
| Existing `CURRENT_TRIP_FILL` | max additional loads 1 | max additional loads 1 | `BREAKING=NO` |
| New `SAME_ORIGIN_SAME_DESTINATION_N_MEMBER` | unsupported | bounded 2..3 member planning | `ADDITIVE=YES` |

Proposed N-member request: `pattern=SAME_ORIGIN_SAME_DESTINATION_N_MEMBER`, an owned `capacity_id`, the existing policy when it applies, and optional `candidate_limit`. The caller does not send `max_set_size`, `max_pool_size`, `max_sets`, `max_groupage_calls`, or `time_budget`. Those stay server policy for this pattern only.

Proposed response schemas stay separate. `PairwiseConsolidationSearchResponse` keeps `evaluated_pair_count` and `PairwiseConsolidationCandidate` with `members.minItems=2` and `members.maxItems=2`. `NMemberConsolidationSearchResponse` is not that schema. It carries `search_id`, `capacity_id`, `capacity_version`, `pattern`, `pool_load_count`, `evaluated_set_count`, `feasible_candidate_count`, `indeterminate_candidate_count`, `hard_reject_candidate_count`, `returned_candidate_count`, and `candidates`. The set counter is `evaluated_set_count`, not `evaluated_pair_count`. `NMemberConsolidationCandidate` has `members.minItems=2` and `members.maxItems=3`. Member `ordinal` is an integer from 1 through 3. `CURRENT_TRIP_FILL` stays `CurrentTripFillSearchResponse`.

Proposed discriminator, not generated in this revision:

```text
SAME_ORIGIN_SAME_DESTINATION -> PairwiseConsolidationSearchResponse
SAME_ORIGIN_SAME_DESTINATION_N_MEMBER -> NMemberConsolidationSearchResponse
CURRENT_TRIP_FILL -> CurrentTripFillSearchResponse
```

Existing pairwise clients keep receiving two-member candidates. `execution_supported` is false. The N-member body has no stop list. `ordinal` is unique within the candidate, and so is `load_opportunity_id`.

```text
PairwiseConsolidationSearchResponse.evaluated_pair_count
    ↔ consolidation_search_runs.evaluated_pair_count
NMemberConsolidationSearchResponse.evaluated_set_count
    ↔ consolidation_search_runs.evaluated_set_count
```

The response name and the stored column use the same meaning. An N-member count is not written into `evaluated_pair_count`.

## Current database constraints

Migrations `000081` and `000083` are the live schema. This revision does not change them.

`000081` creates `consolidation_search_runs` with `pattern text NOT NULL` and `evaluated_pair_count integer NOT NULL`. Its original pattern check allowed only `SAME_ORIGIN_SAME_DESTINATION`. `000083` replaces that check:

```text
consolidation_search_runs_pattern_chk:
pattern IN ('SAME_ORIGIN_SAME_DESTINATION', 'CURRENT_TRIP_FILL')
```

`000083` also adds `consolidation_search_runs_context_chk`:

```text
(pattern = 'SAME_ORIGIN_SAME_DESTINATION'
 AND capacity_id IS NOT NULL
 AND capacity_version IS NOT NULL)
OR
(pattern = 'CURRENT_TRIP_FILL'
 AND capacity_id IS NULL
 AND capacity_version IS NULL)
```

`consolidation_candidate_members` still has `CHECK (ordinal IN (1, 2))`, primary key `(candidate_id, ordinal)`, unique `(candidate_id, load_opportunity_id)`, and a foreign key on `candidate_id`. `consolidation_candidates.pattern` is tied to the run by `consolidation_candidates_run_fk` on `(search_run_id, tenant_id, capacity_id, pattern)`. For a capacity-backed run that foreign key requires `candidate.pattern = search_run.pattern`.

`searchCurrentTripFill` sets `EvaluatedPairCount` to the number of opted-in loads it assessed and stores that integer in `evaluated_pair_count`. The run leaves `capacity_id` and `capacity_version` null. NLO-0.3E does not rename that current-trip field and does not move it to `evaluated_set_count`.

## Future migration shape

Not created. Not reserved. `000084` is not used. `MIGRATION_CREATED=NO`. `MIGRATION_RESERVED=NO`.

Pattern check, and no other pattern:

```text
pattern IN (
  'SAME_ORIGIN_SAME_DESTINATION',
  'SAME_ORIGIN_SAME_DESTINATION_N_MEMBER',
  'CURRENT_TRIP_FILL'
)
```

Context check. Both same-origin modes require a capacity. Current trip stays without one:

```text
(
  pattern IN (
    'SAME_ORIGIN_SAME_DESTINATION',
    'SAME_ORIGIN_SAME_DESTINATION_N_MEMBER'
  )
  AND capacity_id IS NOT NULL
  AND capacity_version IS NOT NULL
)
OR
(
  pattern = 'CURRENT_TRIP_FILL'
  AND capacity_id IS NULL
  AND capacity_version IS NULL
)
```

Audit counts. Add `evaluated_set_count integer NULL` and drop `NOT NULL` from `evaluated_pair_count`. The count check is:

```text
(
  pattern = 'SAME_ORIGIN_SAME_DESTINATION'
  AND evaluated_pair_count IS NOT NULL
  AND evaluated_set_count IS NULL
)
OR
(
  pattern = 'SAME_ORIGIN_SAME_DESTINATION_N_MEMBER'
  AND evaluated_pair_count IS NULL
  AND evaluated_set_count IS NOT NULL
)
OR
(
  pattern = 'CURRENT_TRIP_FILL'
  AND evaluated_pair_count IS NOT NULL
  AND evaluated_set_count IS NULL
)
```

```text
PAIRWISE_RUN_HAS_EVALUATED_PAIR_COUNT=YES
PAIRWISE_RUN_HAS_EVALUATED_SET_COUNT=NO
N_MEMBER_RUN_HAS_EVALUATED_SET_COUNT=YES
N_MEMBER_RUN_HAS_EVALUATED_PAIR_COUNT=NO
CURRENT_TRIP_AUDIT_FIELD=evaluated_pair_count
```

A pairwise row and an N-member row cannot both carry a pair count and a set count. Current-trip rows keep the column they already use.

Member check becomes `ordinal BETWEEN 1 AND 3`. The primary key, the load unique key, and the candidate foreign key stay. The composite run foreign key stays, so an N-member candidate keeps the run's pattern. A two-member N-member candidate is still N-member mode. Member count is not the discriminator.

Existing pairwise rows and existing current-trip rows stay valid: their pattern, capacity nullability, and `evaluated_pair_count` are unchanged, and `evaluated_set_count` is null. Ordinals 1 and 2 still satisfy `BETWEEN 1 AND 3`.

The down migration checks before it restores the old constraints. If any run has `pattern = 'SAME_ORIGIN_SAME_DESTINATION_N_MEMBER'`, or any member has `ordinal = 3`, it raises an error and aborts. It does not delete those rows, does not rewrite the pattern to pairwise, and does not copy `evaluated_set_count` into `evaluated_pair_count`.

```text
DOWN_MIGRATION_DESTRUCTIVE_COERCION=NO
DOWN_MIGRATION_FAIL_CLOSED=YES
DOWN_FAILS_CLOSED_IF_ROWS_OUTSIDE_OLD_BOUND=YES
```

## Resource exhaustion

On the N-member pattern, `candidate_limit` is a response cap only. It does not change pool reads, set generation, groupage calls, or persistence. The server pool, set, groupage, and time budgets do. Those budgets and the two codes below belong only to `SAME_ORIGIN_SAME_DESTINATION_N_MEMBER`. This ADR does not add them to legacy pairwise search.

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
