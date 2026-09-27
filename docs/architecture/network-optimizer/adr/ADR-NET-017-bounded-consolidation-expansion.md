# ADR-NET-017: NLO-0.3E bounded consolidation expansion

Status: Proposed. Controller review R5 freeze candidate. Not accepted. Implementation is not authorized.

```text
ADR_STATUS=PROPOSED
NLO03E_F001=CLOSED
NLO03E_F002=CLOSED
NLO03E_F005=CLOSED
NLO03E_F006=ADDRESSED_IN_THIS_REVISION
POSTGRES_BASELINE=ACCEPTED
ROUTING_SCOPE=ACCEPTED
NLO03E_I001=RESOLVED
NLO03E_I002=RESOLVED_AT_DESIGN_LEVEL
NLO03E_I003=RESOLVED_AT_DESIGN_LEVEL
ARCHITECTURE_FROZEN=NO
ARCHITECTURE_FREEZE_CANDIDATE=YES
TEST_STRATEGY_FREEZE_CANDIDATE=YES
IMPLEMENTATION_AUTHORIZED=NO
```

Baseline: `origin/main` `6576034a4f39b451341ad72667c5fc25f6562658`. Measurements: `NLO_0_3E_MEASUREMENTS.md`.

## Decision

NLO-0.3E, when later authorized, extends planning search only. `execution_supported=false`. It does not activate a route, mutate a shipment or order, or create an assignment, reservation, offer, slot, or driver task.

Algorithm: lexicographic incremental extension of load ids inside one canonical origin-destination group, from size 2 through a server-owned `MAX_SET_SIZE`, stopping at a server-owned `MAX_SETS_EVALUATED`. Every load in the set shares that one pickup location and that one delivery location. The set does not add a stop and does not order stops. Unrestricted subset enumeration is forbidden. Beam search and top-K seeding are rejected because they need a score.

Those budgets apply only to the additive pattern `SAME_ORIGIN_SAME_DESTINATION_N_MEMBER`. `MAX_CANDIDATE_POOL=10` is the global visible eligible pool for that pattern, applied by a repository read of at most 11 rows ordered by pickup location, delivery location, and id, before any set is built. `MAX_SET_SIZE=3`. `MAX_SETS_EVALUATED=165`. `MAX_GROUPAGE_CALLS=330`. `TIME_BUDGET=5s` is a watchdog, not an SLO. `NLO03E_MAX_ROUTING_CALLS=0`. `NLO03E_ROUTING_CALLS_PER_SET=0`. `REAL_ROUTING_PROVIDER_BUDGET_REQUIRED_FOR_0_3E=NO`. A 2GIS load test is not an NLO-0.3E gate. NLO-0.3D keeps its existing routing calls for one additional load. Legacy `SAME_ORIGIN_SAME_DESTINATION` stays pairwise with set size 2. `LEGACY_PAIRWISE_UNBOUNDED_BEHAVIOR=UNCHANGED_BY_0_3E`. This ADR does not apply the pool of 10 to that pattern.

Route sequences explored by NLO-0.3E: 0. Same-origin same-destination has no alternate stop order. `N!` permutation is forbidden. NLO-0.3E does not choose a canonical multi-stop sequence. See F001 and F002 below.

Cross-shipper policy: `CROSS_SHIPPER_N_WAY_FEASIBLE_ALLOWED=NO` for two shippers and for three or more. Result stays `INDETERMINATE` / `MULTI_PARTY_REFERENCE_CONTEXT_UNAVAILABLE`. One tenant's catalog overlay is not applied to another tenant's cargo.

Ranking: no. `MATCH_SCORE_REUSED=NO`. `ConsolidationPlanningScore` is not introduced. Tie break is load-id order, then the existing status rank for the returned page. Weights stay unset. `MAX_CONTRIBUTION` stays reserved.

On the N-member pattern, `candidate_limit` is a returned-result cap only. It does not bound pool reads, set generation, set evaluation, groupage calls, or writes. Server budgets for that pattern are checked before that work. A budget failure rolls the attempt back and returns HTTP 422 with `details.reason` of `POOL_LIMIT_EXCEEDED` or `SEARCH_BUDGET_EXCEEDED`. It does not return a partial candidate list. Those codes are not claimed for legacy pairwise search.

Audit: a successful N-member search commits one run after the bounded enumeration finishes. A budget failure rolls that transaction back. The runs table currently allows only `COMPLETED`, so failure is not stored as a completed search. The persisted `pattern` is `SAME_ORIGIN_SAME_DESTINATION` or `SAME_ORIGIN_SAME_DESTINATION_N_MEMBER`. Mode is not inferred from member count. N-member fingerprints include that pattern, capacity and load versions, consumed cargo or profile versions, catalog and rule versions, compatibility fingerprints, policy version, and the algorithm and budget policy version. They do not cover shipment version, tracking, ETA, or routing proof. No cache.

Invalidation: a change of capacity, load, consumed cargo or profile, catalog, rule, policy, or budget policy changes the candidate fingerprint even when the status does not. Canonical origin, destination, and windows do too, through their authoritative versions.

Failure modes for the N-member pattern only: `POOL_LIMIT_EXCEEDED` and `SEARCH_BUDGET_EXCEEDED`. Fail closed. This ADR does not add those codes to legacy pairwise search. `ROUTING_BUDGET_EXCEEDED` is not an NLO-0.3E result. Unknown is not zero and not feasible.

Problem classes in this revision: the additive pattern `SAME_ORIGIN_SAME_DESTINATION_N_MEMBER`, as cargo-set planning of 2 or 3 members. The existing pattern `SAME_ORIGIN_SAME_DESTINATION` stays pairwise. Current-trip fill stays at one additional load, which is NLO-0.3D. A second additional load, multi-pick, multi-drop, and multi-pick-multi-drop wait for NLO-0.4.

Persistence (`NLO03E_I002`): live constraints are migration `000081` plus `000083`. `pattern` is `NOT NULL`. `evaluated_pair_count` is `integer NOT NULL`. The pattern check allows only `SAME_ORIGIN_SAME_DESTINATION` and `CURRENT_TRIP_FILL`. The context check requires a capacity for pairwise runs and null capacity columns for `CURRENT_TRIP_FILL`. Members are `ordinal IN (1, 2)`, with primary key `(candidate_id, ordinal)`, unique `(candidate_id, load_opportunity_id)`, and a foreign key on `candidate_id`. `consolidation_candidates_run_fk` on `(search_run_id, tenant_id, capacity_id, pattern)` keeps `candidate.pattern` equal to `search_run.pattern` when capacity is present. Mode is not inferred from member count.

`searchCurrentTripFill` writes the number of assessed additional loads into `evaluated_pair_count` and leaves both capacity columns null. That storage stays. NLO-0.3E does not rename it.

The later migration, not created and not reserved as `000084`, widens the pattern check to those two values plus `SAME_ORIGIN_SAME_DESTINATION_N_MEMBER` and no other pattern. The context check requires non-null `capacity_id` and `capacity_version` for both same-origin patterns, and null capacity columns for `CURRENT_TRIP_FILL`. It adds nullable `evaluated_set_count` and allows `evaluated_pair_count` to be null only where the count check says so: pairwise and current-trip rows keep a pair count and a null set count; an N-member row keeps a set count and a null pair count. Both counts are never set together on those modes. Member ordinal becomes `BETWEEN 1 AND 3`. The primary key, load unique key, candidate foreign key, and composite run foreign key stay.

The down migration raises and aborts when any run pattern is `SAME_ORIGIN_SAME_DESTINATION_N_MEMBER` or any member ordinal is 3. It does not delete those rows, rewrite the pattern to pairwise, or copy `evaluated_set_count` into `evaluated_pair_count`. `DOWN_MIGRATION_DESTRUCTIVE_COERCION=NO`. `DOWN_MIGRATION_FAIL_CLOSED=YES`. `MIGRATION_CREATED=NO`. `MIGRATION_RESERVED=NO`.

Public API counts use the same meaning as the columns: `PairwiseConsolidationSearchResponse.evaluated_pair_count` is `consolidation_search_runs.evaluated_pair_count`, and `NMemberConsolidationSearchResponse.evaluated_set_count` is `consolidation_search_runs.evaluated_set_count`.

Public API: reuse `POST /v1/network/consolidation/search`. No new route. `NEW_PATTERN=YES`. Proposed discriminator: `SAME_ORIGIN_SAME_DESTINATION` maps to `PairwiseConsolidationSearchResponse` with `evaluated_pair_count` and members fixed at 2; `SAME_ORIGIN_SAME_DESTINATION_N_MEMBER` maps to `NMemberConsolidationSearchResponse` with `evaluated_set_count` and members from 2 through 3; `CURRENT_TRIP_FILL` maps to `CurrentTripFillSearchResponse`. Generated OpenAPI is not edited in this revision. The N-member request sends pattern, owned `capacity_id`, existing policy when it applies, and optional `candidate_limit`. It does not send pool, set, groupage, or time budgets.

## Controller findings

### NLO03E_F001 — scope contradiction, multi-stop planning versus execution

The first revision put current-trip sets of more than one additional load in NLO-0.3E and defined a pickup and delivery order for them, while deferring multi-pick and multi-drop to NLO-0.4. That order is multi-stop planning. `execution_supported=false` does not make it a cargo-only decision. NLO-0.4 owns `RoutePlan`, `RouteStop`, shared legs, and multi-stop shipment representation.

Revised scope:

- `SAME_ORIGIN_SAME_DESTINATION` stays pairwise, set size 2. NLO-0.3E does not widen that pattern.
- `SAME_ORIGIN_SAME_DESTINATION_N_MEMBER` is the additive NLO-0.3E pattern. All members share one canonical pickup location and one canonical delivery location. Planning is residual capacity and compatibility of the cargo set. It does not emit a stop list.
- `CURRENT_TRIP_FILL` with more than 1 additional load is `DEFER_TO_0_4`. NLO-0.3D remains `MAX_ADDITIONAL_LOADS=1`.
- `MULTI_PICK_ONE_DROP`, `ONE_PICK_MULTI_DROP`, and `MULTI_PICK_MULTI_DROP` stay `DEFER_TO_0_4`.

### NLO03E_F002 — one sequence cannot prove global infeasibility

A single explored stop order can show that order is feasible. It cannot show that every order is infeasible. Labeling the set `HARD_REJECT` because one lexicographic order missed a window, a rehandling rule, or a detour would be a false global proof.

NLO-0.3E explores no alternate route sequences, so it does not emit a sequence verdict. Sequence-independent groupage failures may still be `HARD_REJECT`. Any later wave that explores stop orders must keep order-dependent failure as `INDETERMINATE` unless the authorized sequence bound is actually exhausted. A bound of one sequence is not that exhaustion.

### NLO03E_F005 — public pairwise contract must stay pairwise

Widening `SAME_ORIGIN_SAME_DESTINATION` in place would return three-member candidates to existing pairwise clients and would apply the N-member pool bound to a mode this ADR does not redefine. The legacy pattern stays `PAIRWISE_ONLY` with set size 2 and its existing response, including `evaluated_pair_count`. The N-member search is a new pattern on the same endpoint. Its response is `NMemberConsolidationSearchResponse` with `evaluated_set_count`. Persisted `pattern` distinguishes the modes. `LEGACY_PAIRWISE_UNBOUNDED_BEHAVIOR=UNCHANGED_BY_0_3E`.

### NLO03E_F006 — persistence checks do not yet admit the N-member pattern

The live pattern check, context check, and `evaluated_pair_count integer NOT NULL` cannot store `SAME_ORIGIN_SAME_DESTINATION_N_MEMBER` without either rejecting the row or overloading the pair-count column. Member `ordinal IN (1, 2)` cannot store a third member. This revision documents those live checks and the future checks. It does not add the migration.

### Design status after R5

F001, F002, and F005 stay closed. F006 is addressed in this revision and is not a controller acceptance. The Postgres baseline and the routing scope are accepted. I001, I002, and I003 are resolved at design level in this candidate. They are not a runtime implementation. The ADR remains proposed.

- `NLO03E_I001`: global pool 10, set size 3, 165 sets, 330 groupage calls, 5 second watchdog, 0 routing calls. Those budgets stay on the N-member pattern only.
- `NLO03E_I002`: pattern check, context check, exclusive pair/set counts, ordinal 1..3, existing keys preserved, down migration fails closed without deleting or rewriting N-member rows. No migration file.
- `NLO03E_I003`: `candidate_limit` is response-only. Work budgets are prechecked. Failure rolls back.

## Consequences

- NLO-0.3 stays incomplete until a later implementation is accepted and merged.
- NLO-0.4 is not started. Multi-stop planning is not pulled forward into NLO-0.3E.
- This revision is a freeze candidate. `ARCHITECTURE_FROZEN=NO` until the controller accepts it.
- Solver techniques remain out: VRP, MILP, CP-SAT, LNS, genetic algorithms, ML, and 3D bin packing.
- Backhaul, round trip, city rules, and network-wide optimization remain out.
