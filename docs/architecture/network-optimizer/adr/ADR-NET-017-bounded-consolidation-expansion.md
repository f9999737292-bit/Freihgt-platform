# ADR-NET-017: NLO-0.3E bounded consolidation expansion

Status: Proposed. Controller review R3 freeze candidate. Not accepted. Implementation is not authorized.

```text
ADR_STATUS=PROPOSED
NLO03E_F001=CLOSED
NLO03E_F002=CLOSED
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

`MAX_CANDIDATE_POOL=10` is the global visible eligible pool, applied by a repository read of at most 11 rows ordered by pickup location, delivery location, and id, before any set is built. `MAX_SET_SIZE=3`. `MAX_SETS_EVALUATED=165`. `MAX_GROUPAGE_CALLS=330`. `TIME_BUDGET=5s` is a watchdog, not an SLO. `NLO03E_MAX_ROUTING_CALLS=0`. `NLO03E_ROUTING_CALLS_PER_SET=0`. `REAL_ROUTING_PROVIDER_BUDGET_REQUIRED_FOR_0_3E=NO`. A 2GIS load test is not an NLO-0.3E gate. NLO-0.3D keeps its existing routing calls for one additional load.

Route sequences explored by NLO-0.3E: 0. Same-origin same-destination has no alternate stop order. `N!` permutation is forbidden. NLO-0.3E does not choose a canonical multi-stop sequence. See F001 and F002 below.

Cross-shipper policy: `CROSS_SHIPPER_N_WAY_FEASIBLE_ALLOWED=NO` for two shippers and for three or more. Result stays `INDETERMINATE` / `MULTI_PARTY_REFERENCE_CONTEXT_UNAVAILABLE`. One tenant's catalog overlay is not applied to another tenant's cargo.

Ranking: no. `MATCH_SCORE_REUSED=NO`. `ConsolidationPlanningScore` is not introduced. Tie break is load-id order, then the existing status rank for the returned page. Weights stay unset. `MAX_CONTRIBUTION` stays reserved.

`candidate_limit` is a returned-result cap only. It does not bound pool reads, set generation, set evaluation, groupage calls, or writes. Server budgets are checked before that work. A budget failure rolls the attempt back and returns HTTP 422 with `details.reason` of `POOL_LIMIT_EXCEEDED` or `SEARCH_BUDGET_EXCEEDED`. It does not return a partial candidate list.

Audit: a successful search commits one run after the bounded enumeration finishes. A budget failure rolls that transaction back. The runs table currently allows only `COMPLETED`, so failure is not stored as a completed search. Fingerprints cover capacity and load versions, consumed cargo or profile versions, catalog and rule versions, compatibility fingerprints, policy version, and the budget policy version. They do not cover shipment version, tracking, ETA, or routing proof. No cache.

Invalidation: a change of capacity, load, consumed cargo or profile, catalog, rule, policy, or budget policy changes the candidate fingerprint even when the status does not. Canonical origin, destination, and windows do too, through their authoritative versions.

Failure modes for NLO-0.3E: `POOL_LIMIT_EXCEEDED` and `SEARCH_BUDGET_EXCEEDED`. Fail closed. `ROUTING_BUDGET_EXCEEDED` is not an NLO-0.3E result. Unknown is not zero and not feasible.

Problem classes in this revision: same-origin same-destination sets larger than 2, as cargo-set planning only. Current-trip fill stays at one additional load, which is NLO-0.3D. A second additional load, multi-pick, multi-drop, and multi-pick-multi-drop wait for NLO-0.4.

Persistence (`NLO03E_I002`): the current member check is `ordinal IN (1, 2)`. The design widens a later migration to `ordinal BETWEEN 1 AND 3`, keeps the primary key, the load unique key, and the candidate foreign key, and sets OpenAPI `minItems` 2 and `maxItems` 3. A down migration fails closed if any ordinal is outside 1..2. This ADR does not add the migration and does not reserve `000084`.

Public API: reuse `POST /v1/network/consolidation/search`. No new route in this decision.

## Controller findings

### NLO03E_F001 — scope contradiction, multi-stop planning versus execution

The first revision put current-trip sets of more than one additional load in NLO-0.3E and defined a pickup and delivery order for them, while deferring multi-pick and multi-drop to NLO-0.4. That order is multi-stop planning. `execution_supported=false` does not make it a cargo-only decision. NLO-0.4 owns `RoutePlan`, `RouteStop`, shared legs, and multi-stop shipment representation.

Revised scope:

- `SAME_ORIGIN_SAME_DESTINATION` with more than 2 loads stays the NLO-0.3E candidate. All members share one canonical pickup location and one canonical delivery location. Planning is residual capacity and compatibility of the cargo set. It does not emit a stop list.
- `CURRENT_TRIP_FILL` with more than 1 additional load is `DEFER_TO_0_4`. NLO-0.3D remains `MAX_ADDITIONAL_LOADS=1`.
- `MULTI_PICK_ONE_DROP`, `ONE_PICK_MULTI_DROP`, and `MULTI_PICK_MULTI_DROP` stay `DEFER_TO_0_4`.

### NLO03E_F002 — one sequence cannot prove global infeasibility

A single explored stop order can show that order is feasible. It cannot show that every order is infeasible. Labeling the set `HARD_REJECT` because one lexicographic order missed a window, a rehandling rule, or a detour would be a false global proof.

NLO-0.3E explores no alternate route sequences, so it does not emit a sequence verdict. Sequence-independent groupage failures may still be `HARD_REJECT`. Any later wave that explores stop orders must keep order-dependent failure as `INDETERMINATE` unless the authorized sequence bound is actually exhausted. A bound of one sequence is not that exhaustion.

### Design status after R3

F001 and F002 are closed. The Postgres baseline and the routing scope are accepted. I001, I002, and I003 are resolved at design level in this candidate. They are not a runtime implementation. The ADR remains proposed.

- `NLO03E_I001`: global pool 10, set size 3, 165 sets, 330 groupage calls, 5 second watchdog, 0 routing calls.
- `NLO03E_I002`: ordinal 1..3, existing keys preserved, down migration fails closed, no migration file.
- `NLO03E_I003`: `candidate_limit` is response-only. Work budgets are prechecked. Failure rolls back.

## Consequences

- NLO-0.3 stays incomplete until a later implementation is accepted and merged.
- NLO-0.4 is not started. Multi-stop planning is not pulled forward into NLO-0.3E.
- This revision is a freeze candidate. `ARCHITECTURE_FROZEN=NO` until the controller accepts it.
- Solver techniques remain out: VRP, MILP, CP-SAT, LNS, genetic algorithms, ML, and 3D bin packing.
- Backhaul, round trip, city rules, and network-wide optimization remain out.
