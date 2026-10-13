# NLO-0.9B chain architecture freeze

This document freezes the decisions NLO-0.9A left open. It does not implement a chain search, a ranker, stacking, persistence, or an endpoint.

```text
NLO_0_9B_MODE=DOCS_ONLY_ARCHITECTURE_FREEZE
NLO_0_9B_STATUS=ARCHITECTURE_FROZEN
RUNTIME_IMPLEMENTATION_AUTHORIZED=NO
IMPLEMENTATION_AUTHORIZED=NO
CHAIN_RUNTIME_IMPLEMENTED=NO
NLO_0_9C_STARTED=NO
NLO_0_9D_STARTED=NO
RECOMMENDED_FIRST_CHAIN_RUNTIME_SHAPE=TWO_LOAD_CHAIN_TOTAL
GLOBAL_CHAIN_OPTIMUM_CLAIM_ALLOWED=NO
OPENAPI_CHANGED_IN_0_9B=NO
MIGRATION_CREATED_IN_0_9B=NO
```

Base: `69371e34303b92cab188d89110c58c96156c69d8`.

NLO-0.9A remains the source inventory. Where this document names a policy value, it supersedes the matching `TBD` in the 0.9A discovery. It does not supersede a source fact. One-load search still returns one load. The server still does not discover a chain. Route-plan evaluate still accepts one or two caller-selected load ids. `match_candidates` is still one load per row. The current planner still sorts by UUID and inserts greedily. `ExecutionSupported=false` remains the current planner result.

Decision record: [ADR-NET-025](adr/ADR-NET-025-bounded-two-load-chain.md).

## Shape

`TWO_LOAD_CHAIN_TOTAL` is one optimization seed plus exactly two future load subjects, L1 and L2. The server discovers both. The caller does not submit the pair. There is no third load, no recursive extension, and no N-load search. Comparing the two orders of the same pair is not an N-load search and is not a global optimum.

## A. Horizon

```text
CHAIN_HORIZON_POLICY=FIXED_CLOCK_48H
CHAIN_HORIZON_CLOCK=UTC_INSTANT
CHAIN_HORIZON_BUSINESS_CALENDAR=NO
CHAIN_HORIZON_CALLER_OVERRIDE=NO
CHAIN_HORIZON_MAXIMUM=48h
```

This supersedes `CHAIN_HORIZON_POLICY_VALUE=TBD` and `CHAIN_HORIZON_POLICY_ALREADY_FROZEN=NO` from NLO-0.9A.

`PRODUCT_SCOPE` P3 listed 24h, 48h, 72h, and N days without choosing. A 24h window is not selected: a delivered L1 and a later L2 pickup plus deadhead commonly cross one calendar day, and a test fixture that adds 24 hours is not a policy. 72h and N days are not selected: the marketplace pool is already up to 1000 loads, and a longer horizon widens that pool while the first runtime has no measured chain budget. 48h is one fixed planning window. It is long enough for two sequential loads and short of a regional tour, which stays NLO-0.6.

The horizon is clock time, not a business calendar. No carrier working-calendar source is frozen. Instants are UTC. A window that is not an absolute instant fails closed and does not inherit a local zone.

Horizon start is the seed availability instant:

- Manual available capacity: `available_from` when it is present, otherwise the search clock. Horizon end is `start + 48h`, and also not later than `available_until` when that bound is present.
- Current-trip residual: the search clock at the fresh position used as the route start. Predicted ETA is not the start.

A load is outside the horizon when its pickup window ends before the start, or its pickup window starts at or after the end, or its delivery window starts at or after the end. A load whose delivery window continues past the end stays eligible only when a measured schedule completes that delivery departure at or before the end. A sequence that cannot prove that completion is rejected. The horizon is not truncated into a partial load.

## B. Discovery budget

These caps are the chain-search budget. They are not the one-load cap of 6 provider calls and 30s, and they are not the route-plan cap of 300 provider calls and 5s. Those remain the budgets of their own operations. NLO-0.9A evidence used here is the pool of 1000 and the routed shortlist of 25, which show why the chain shortlist must be smaller than 25.

```text
FIRST_LOAD_SHORTLIST_CAP=8
SECOND_LOADS_PER_FIRST_CAP=2
CHAIN_PAIR_CAP=4
ROUTEPLAN_EVALUATION_CAP=2
CHAIN_ORDERS_PER_PAIR=2
CHAIN_SEQUENCES_PER_ORDER_CAP=16
CHAIN_LEGS_PER_ORDER_CAP=6
CHAIN_GLOBAL_PROVIDER_CALL_CAP=24
CHAIN_GLOBAL_WALLCLOCK_BUDGET=120s
CHAIN_PARALLELISM=1
CHAIN_PROVIDER_CALL_TIMEOUT=5s
CHAIN_PROVIDER_RETRY_COUNT=0
```

The 5s call timeout and the zero retry are the existing provider limits, applied per call. They are not the global chain budget.

Worst-case expansion, with parallelism 1:

```text
raw_pairs = FIRST_LOAD_SHORTLIST_CAP × SECOND_LOADS_PER_FIRST_CAP = 8 × 2 = 16
pairs_retained = min(raw_pairs, CHAIN_PAIR_CAP) = 4
plans_evaluated = min(pairs_retained, ROUTEPLAN_EVALUATION_CAP) = 2
planner_invocations = plans_evaluated × CHAIN_ORDERS_PER_PAIR = 2 × 2 = 4
provider_calls <= planner_invocations × CHAIN_LEGS_PER_ORDER_CAP = 4 × 6 = 24
provider_calls <= CHAIN_GLOBAL_PROVIDER_CALL_CAP = 24
wall_clock <= provider_calls × CHAIN_PROVIDER_CALL_TIMEOUT = 24 × 5s = 120s
wall_clock <= CHAIN_GLOBAL_WALLCLOCK_BUDGET = 120s
```

The products are equal, so neither cap is decorative. An inner route-plan invocation still stops at its own 64 sequences, 300 calls, and 5s if those trip first. For a chain-owned invocation, the chain wrapper stops that invocation at 16 sequences and 6 legs, and stops the whole search at 24 calls or 120s. The inner 5s budget is not the chain completion budget. Caller-supplied route-plan evaluate keeps its own 5s budget.

Shortlist construction is a prune, not the chain score. First loads are visible loads that pass horizon and hard feasibility, ordered by measured road deadhead ascending and then load id ascending, cut at 8. Unknown road distance is not zero and does not enter the shortlist. For each retained first load, at most 2 later loads are kept by measured road distance from L1 delivery to L2 pickup, then load id. The pair cap then keeps 4 pairs. At most 2 of those pairs are sent to the planner.

```text
CHAIN_BUDGET_EXHAUSTED_STATUS=CHAIN_BUDGET_EXHAUSTED
CHAIN_BUDGET_EXHAUSTED_SEARCH_COMPLETE=false
CHAIN_BUDGET_EXHAUSTED_GLOBAL_OPTIMUM=false
```

Exhaustion returns only pairs whose selected order finished before the stop. Those pairs are ranked among themselves and labeled partial. An empty finished set is still `CHAIN_BUDGET_EXHAUSTED`, not a completed finding that no chain exists. The result is not a successful complete search and not a global optimum.

## C. Objective

```text
CHAIN_OBJECTIVE_POLICY=LEXICOGRAPHIC_EMPTY_ROAD_THEN_DURATION
ONE_LOAD_SCORE_REUSED_AS_CHAIN_SCORE=NO
CHAIN_OBJECTIVE_COMPONENTS=FEASIBILITY_GATE,INCREMENTAL_EMPTY_ROAD_KM,TOTAL_PLANNED_ROAD_DURATION_SECONDS,KNOWN_CAPACITY_UTILIZATION,TARGET_ROAD_DISTANCE_KM,AUTHORITATIVE_REVENUE
CHAIN_OBJECTIVE_ORDER=ASC_EMPTY_KM,ASC_DURATION,DESC_KNOWN_UTILIZATION,ASC_TARGET_KM,DESC_AUTHORITATIVE_REVENUE
CHAIN_OBJECTIVE_TIE_BREAK=ASC_MIN_LOAD_ID,ASC_MAX_LOAD_ID,ASC_SELECTED_ORDER_CODE
```

This supersedes `CHAIN_OBJECTIVE_POLICY=TBD`.

`ONE_LOAD_SCORE` may order the discovery prune. It is not `CHAIN_SCORE`. Route-plan duration order inside one greedy insertion is not the chain rank.

A pair that is not feasible is excluded before ranking. Feasible pairs then compare lexicographically:

1. Incremental empty road kilometres, ascending. Empty means a measured road leg with neither L1 nor L2 onboard. Current-trip residual cargo does not count as a chain load. Unknown road kilometres exclude the pair. They are not zero.
2. Total planned road duration of the selected order, ascending, using measured leg durations only.
3. Known capacity utilization, descending, only when every dimension the plan consumed is `KNOWN`. If any consumed dimension is unknown, this component is omitted for every pair in that comparison. It is not zero.
4. Target road distance from the chain end to the seed target, ascending, only when the seed has an explicit target. Manual capacity without a target, and a comparison where the target leg is unknown, omit the component. Omission is not zero.
5. Authoritative revenue, descending, only when both loads expose a comparable amount in the same currency from an authoritative commercial fact. If either load lacks that fact, or the currencies differ, the component is omitted for that comparison. Missing revenue is not zero, and the priced load does not win by default.

No separate risk weight is frozen. No authoritative chain risk input exists, so a risk penalty is omitted rather than invented.

Tie-break is the lower load id, then the higher load id, then the selected order code. `L1_BEFORE_L2` sorts before `L2_BEFORE_L1` only when the scored components are equal. Load ids are UUID strings.

## D. Identity

```text
CHAIN_SEARCH_IDENTITY=SERVER_UUID_TENANT_SCOPED
CHAIN_CANDIDATE_IDENTITY=SERVER_UUID_BEFORE_EVALUATION
CHAIN_EVALUATION_IDENTITY=ROUTE_PLAN_ID_AFTER_EVALUATION_ONLY
CLIENT_SUPPLIED_CHAIN_ID_ACCEPTED=NO
ROUTE_PLAN_ID_IS_PRE_EVALUATION_CHAIN_IDENTITY=NO
```

This supersedes the 0.9A statement that chain identity was absent as a future model. The source still has no chain id column. The runtime that creates one is not this stage.

A chain search id is a UUID generated by the server inside the caller tenant. The client cannot choose it. The same `Idempotency-Key` and the same request hash replay that id and body. The same key with a different hash conflicts. A new key allocates a new search id even when the seed and policy match. Result order is deterministic for the same inputs and policy version; the search UUID is not a hash of those inputs.

A chain candidate id is a UUID generated by the server when the pair is materialized, before route-plan evaluation. One candidate is one unordered pair of two load ids and their versions inside one search. The dedup key is tenant, search id, lower load id, higher load id, and both versions. The two action orders are evaluations of that one candidate, not two candidates. `route_plan_id` is null until an evaluation is stored.

The evaluation identity is the existing `route_plan_id`, and only after that plan is stored. It identifies the selected order's plan. It is not the search id and not the candidate id.

## E. Persistence

```text
CHAIN_DISCOVERY_PERSISTENCE=SEARCH_RUN_AND_SHORTLIST
MATCH_CANDIDATES_REDEFINED_AS_CHAIN=NO
MIGRATION_REQUIRED_IN_FUTURE_RUNTIME=YES
MIGRATION_CREATED_IN_0_9B=NO
MIGRATION_NUMBER_ASSIGNED=NO
```

Evaluated choices:

| Choice | Decision |
| --- | --- |
| Ephemeral discovery only | Not selected. A budget-exhausted or rejected shortlist would have no tenant-scoped audit. |
| Search run only | Not sufficient. The ranked pair and its explanation would not be replayable. |
| Search run and shortlist | Selected. At most `CHAIN_PAIR_CAP` pair rows, with reason codes, versions, and fingerprints. |
| Shortlist plus every rejected marketplace row | Not selected. A 1000-load rejection table repeats foreign load detail and is not required for the first audit. |

Non-shortlisted loads contribute aggregate counts by reason code only. Those counts do not store the other shipper's identity or commercial fields. The shortlist stores the candidate id, load ids and versions that the carrier is already allowed to know, the selected order, the objective components that were present, the omitted components, and reject codes.

`match_candidates` stays one `load_opportunity_id` per row. Route-plan tables continue to own the evaluated plan, its stops, actions, legs, and capacity snapshots. Chain tables, when a later runtime adds them, reference `route_plan_id` only after evaluation. This stage assigns no migration number and creates no migration. Retention follows deletion of the owning tenant. No new TTL is invented.

## F. Privacy

```text
CHAIN_PRE_ACCEPT_PRIVACY_SURFACE=INDIVIDUAL_PUBLICATION_SCOPE_PLUS_OWN_ROUTE_METRICS
CHAIN_POST_ACCEPT_PRIVACY_SURFACE=EXISTING_ACCEPTED_ROUTE_PLAN_AND_TMS
DISCOVERY_RESULT_VISIBILITY_EQUALS_ACCEPTED_EXECUTION_VISIBILITY=NO
TENANT_ISOLATION=FAIL_CLOSED
CROSS_SHIPPER_VISIBILITY=EXPLICIT_ONLY
CLIENT_SUPPLIED_TENANT_TRUST=NO
RAW_FOREIGN_COMMERCIAL_DATA_PRE_ACCEPT=NO
FAIL_CLOSED_ON_UNKNOWN_REFERENCE_CONTEXT=YES
```

This supersedes the unfrozen privacy surface. It does not invent a publication scope. ADR-NET-015 remains the cross-shipper opt-in rule: a foreign load enters a pair only when that load is published to this carrier and `cross_shipper_consolidation_allowed` is true. Same-owner loads still require `consolidation_allowed`. The carrier cannot set either flag on the request.

Before accept, each load contributes only the fields its existing publication scope already allows this carrier to see. A chain does not upgrade an anonymized load to an exact address, an exact window, a customer name, or a shipper name. The chain-level fields are the carrier's own seed reference, the server candidate id, measured empty kilometres, measured duration, feasibility and reject codes, and the objective components that were present or omitted. Reason codes must not embed the other shipper's name, customer, contract, rate, or internal shipment id.

Pre-accept response:

| Field | Exposed |
| --- | --- |
| Other shipper identity | NO |
| Customer or consignee identity | NO |
| Internal shipment id of another party | NO |
| Exact pickup or delivery address beyond that load's own publication scope | NO |
| Time window beyond that load's own publication scope | NO |
| Commercial terms, rate, revenue, contract | NO |
| Cargo attributes beyond that load's own visible snapshot | NO |
| Approximate geography already allowed by that load's publication scope | YES |
| Own-route empty kilometres and duration | YES |
| Feasibility and rejection codes without foreign commercial text | YES |

After accept, visibility is the existing accepted route-plan and the TMS execution projection. That later surface is NLO-0.9D, not this stage. It still does not add the other shipper's commercial terms to the chain-search response. Unknown multi-party reference context does not become a feasible chain and is not returned as a selectable pair.

## G. Stacking

```text
STACKING_POLICY=OVERLAP_REQUIRES_KNOWN_STACKABLE_NO_EXTRA_SPACE
STACKING_UNKNOWN_BEHAVIOR=REJECT
STACKING_CHECK_PRESENT_IN_CURRENT_SOURCE=NO
STACKING_RUNTIME_IMPLEMENTED=NO
FULL_CHAIN_COMPATIBILITY_CLAIMED=NO
```

This supersedes the open stacking gap as a policy. The source still has no stacking check. `stackable=true` still does not create pallet space.

When L1 and L2 would be onboard at the same time:

- a null or unknown `stackable` flag rejects the pair
- `stackable=false` on either load rejects the pair
- both flags true allows the overlap for the stacking question only; weight, volume, pallets, linear metres, and height still use remaining known capacity

When L1 is fully delivered before L2 is picked up, stacking is not applied. No layout engine is claimed, so physical separation of non-stackable cargo is not an acceptance path. Temperature, ADR, food grade, odor, and contamination remain the existing groupage checks.

## H. Route-plan selection

```text
ROUTEPLAN_SELECTION_POLICY=BOUNDED_TWO_ORDER_COMPARISON
CURRENT_PLANNER_BEHAVIOR_UNCHANGED_BY_0_9B=YES
GLOBAL_TWO_LOAD_ORDER_ENUMERATION_PRESENT_IN_CURRENT_PLANNER=NO
GLOBAL_CHAIN_OPTIMUM_CLAIM_ALLOWED=NO
```

NLO-0.9A recorded deterministic UUID sort and sequential greedy insertion, with no comparison of load order. The first runtime does not accept that single order as the chain result. It also does not enumerate every stop interleaving beyond the existing placement search.

For each evaluated pair the later runtime runs the existing insertion planner twice: once with the lower load id inserted first, and once with the higher load id inserted first. Each run keeps that planner's placement enumeration, capacity updates, groupage recheck, windows, service duration, and road legs. Each run stops at 16 sequences and 6 legs. The chain objective compares the two finished feasible orders. There is no claim that another interleaving would be better.

The deterministic order of work is the lower-id-first invocation, then the reverse. The tie-break in section C decides equal scores.

## I. Seed source

```text
FIRST_RUNTIME_SEED_SOURCE=MANUAL_AVAILABLE_AND_CURRENT_TRIP_RESIDUAL
PREDICTED_CAPACITY_IN_FIRST_RUNTIME=NO
```

This supersedes `FIRST_RUNTIME_SEED_SOURCE_POLICY=TBD`. Manual available capacity and current-trip residual remain supported seeds because depot-start and current-trip evaluation already pin them. Predicted capacity stays conditional in the source: next-load search reads it for available time, and route-plan depot start does not. The first runtime rejects a predicted-capacity seed. It does not treat that seed as manual capacity.

## J. API

```text
CHAIN_API_RESOURCE_SHAPE=SEPARATE_CHAIN_SEARCH_RESOURCE
CHAIN_API_PATH=POST /api/v1/network/chain-searches
CHAIN_API_REPLAY_PATH=GET /api/v1/network/chain-searches/{chain_search_id}
CHAIN_API_VISIBILITY=PUBLIC_AUTHENTICATED
OPENAPI_CHANGED_IN_0_9B=NO
PUBLIC_CHAIN_ENDPOINT_IMPLEMENTED=NO
CLIENT_SELECTED_CHAIN_PAIR=NO
```

This supersedes `CHAIN_API_PATH=TBD` and the unfrozen public-versus-internal choice. The path is conceptual. No OpenAPI file changes in this stage.

The resource is separate so next-load search stays a one-load response and route-plan evaluate stays a caller-selected evaluation. The caller sends the seed only: `MANUAL_CAPACITY` with `capacity_id`, or `CURRENT_TRIP` with `shipment_id`, plus an optional idempotency key. A body that includes `candidate_load_ids`, L1, or L2 is rejected. That request is not rewritten into a route-plan evaluate.

The response contains `chain_search_id`, result status, `search_complete`, and candidates. Each candidate contains `chain_candidate_id`, load references allowed by section F, the selected order code, present and omitted objective components, reason codes, and `execution_supported=false`.

Visibility is the existing gateway JWT boundary used for next-load search. The carrier roles already allowed to search are the roles allowed to call this resource. Platform admin remains the existing admin path. No new role and no anonymous access are added. The service remains unexposed except through that gateway.

## K. Execution boundary

```text
EXECUTION_SUPPORT_BOUNDARY=DISABLED_THROUGH_0_9C_UNTIL_0_9D_PROOF
NLO_0_9C_EXECUTION_SUPPORTED=false
NLO_0_9D_AUTHORIZED=NO
TMS_CANONICAL_EXECUTION_OWNER=YES
CURRENT_PLANNER_EXECUTION_SUPPORTED=false
```

NLO-0.9C, when later authorized, may discover, evaluate, rank, persist the search run and shortlist, and return the chain result. Every such result has `execution_supported=false`. It does not accept, activate, or project to TMS.

NLO-0.9D, when later authorized, is the first stage that may accept and activate a chain plan. A chain is executable only after that stage proves feasible status, known service duration, the stacking policy, the existing activation gates, TMS projection, and idempotent acknowledgement. TMS remains the execution owner. This stage does not enable execution. Current accept of a caller-supplied feasible route plan is unchanged and is not chain discovery.

## Blockers

| Blocker | Resolution |
| --- | --- |
| BLOCKER_CHAIN_001=HORIZON_POLICY | CLOSED by `FIXED_CLOCK_48H` |
| BLOCKER_CHAIN_002=SEARCH_BUDGET | CLOSED by the caps in section B |
| BLOCKER_CHAIN_003=CHAIN_IDENTITY_AND_DISCOVERY_PERSISTENCE | CLOSED by sections D and E |
| BLOCKER_CHAIN_004=CHAIN_RANKING_AND_OBJECTIVE | CLOSED by section C |
| BLOCKER_CHAIN_005=CROSS_SHIPPER_PRIVACY | CLOSED by section F |
| BLOCKER_CHAIN_006=STACKING_COMPATIBILITY | CLOSED by section G as a reject-unknown policy, not as a layout engine |
| BLOCKER_CHAIN_007=ROUTEPLAN_SELECTION_POLICY | CLOSED by `BOUNDED_TWO_ORDER_COMPARISON` |

```text
READY_FOR_NLO_0_9C=YES
NLO_0_9C_STARTED=NO
IMPLEMENTATION_AUTHORIZED=NO
```

`READY_FOR_NLO_0_9C=YES` means the architecture blockers above are closed. It does not authorize the runtime.

## Future tests

CHAIN-01 through CHAIN-22 stay `FUTURE_REQUIRED`. Their concrete expectations are in [NLO_0_9A_CHAIN_TEST_AND_IMPLEMENTATION_ROADMAP.md](NLO_0_9A_CHAIN_TEST_AND_IMPLEMENTATION_ROADMAP.md), superseded by this freeze. No test is implemented here.

```text
CHAIN_TESTS_IMPLEMENTED=NO
```
