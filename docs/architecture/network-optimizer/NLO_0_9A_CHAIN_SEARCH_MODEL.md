# NLO-0.9A chain search model

This model names future discovery concepts. It does not create tables, endpoints, or runtime code.

```text
NLO_0_9A_MODE=DISCOVERY_DOCS_ONLY
CHAIN_RUNTIME_IMPLEMENTED=NO
IMPLEMENTATION_AUTHORIZED=NO
DEDICATED_CHAIN_IDENTITY_PRESENT=NO
ROUTE_PLAN_ID_IS_PRE_EVALUATION_CHAIN_IDENTITY=NO
CHAIN_DISCOVERY_ENGINE_PRESENT=NO
CHAIN_COMPARATIVE_RANKER_PRESENT=NO
GLOBAL_CHAIN_OPTIMUM_CLAIM_ALLOWED=NO
```

Inventory and the blocker register: [NLO_0_9A_CHAIN_OPTIMIZER_DISCOVERY.md](NLO_0_9A_CHAIN_OPTIMIZER_DISCOVERY.md).

## Conceptual objects

These names are architecture concepts. No table, column, or API field with these names is claimed to exist.

| Concept | Role |
| --- | --- |
| OptimizationSeed | The capacity context that starts a chain search: where and when the vehicle is available, and which capacity facts are pinned. |
| ChainSearch | One bounded server-side attempt to discover and compare two-load chains for one seed. |
| ChainCandidate | One ordered pair of future load subjects discovered by the server, before or aside from route-plan identity. |
| ChainLoadSubject | One future published load in that pair. The first later runtime uses exactly two: L1 and L2. |
| ChainEvaluation | The bounded route-plan evaluation of one discovered pair. `route_plan_id` can identify this evaluated result. |
| ChainExplanation | The privacy-safe record of why the pair was shortlisted, evaluated, ranked, or rejected. |

```text
DEDICATED_CHAIN_IDENTITY_PRESENT=NO
ROUTE_PLAN_ID_IS_PRE_EVALUATION_CHAIN_IDENTITY=NO
```

`route_plan_id` may identify a plan after evaluation. It cannot identify a discovery candidate that has not been evaluated. `match_candidates` cannot stand in for a chain candidate: each row has one `load_opportunity_id`.

## Relationship

```text
OptimizationSeed
    ↓
bounded first-load discovery
    ↓
bounded second-load discovery
    ↓
pair composition
    ↓
route-plan evaluation
    ↓
chain ranking
    ↓
privacy-safe result
```

One-load discovery is the first arrow only if a later design reuses its visibility and feasibility checks. The current next-load response is still one load. The route-plan call today starts at evaluation, and only when the caller already chose the load ids. The chain optimizer is the whole path, with the server choosing L1 and L2.

## First runtime recommendation

```text
RECOMMENDED_FIRST_CHAIN_RUNTIME_SHAPE=TWO_LOAD_CHAIN_TOTAL
IMPLEMENTATION_AUTHORIZED=NO
```

`TWO_LOAD_CHAIN_TOTAL` means:

- one optimization seed or context
- two future load subjects, L1 and L2, forming the chain
- the later server discovers both L1 and L2
- no arbitrary N-load search
- no recursive chain search
- no unbounded horizon
- no solver claim
- no global optimum claim
- no runtime authorization from this document

An array of caller-supplied `candidate_load_ids` is route-plan evaluation. It is not this chain optimizer, even when the array length is 2.

`FIRST_RUNTIME_SEED_SOURCE_POLICY=TBD`. The evidence below does not choose which seed sources ship in the first runtime. That choice belongs to the later architecture freeze.

## Seed evidence

```text
CHAIN_SEED_SOURCE_MANUAL_AVAILABLE=SUPPORTED
CHAIN_SEED_SOURCE_PREDICTED_CAPACITY=CONDITIONAL
CHAIN_SEED_SOURCE_CURRENT_TRIP_RESIDUAL=SUPPORTED
FIRST_RUNTIME_SEED_SOURCE_POLICY=TBD
```

`DEPOT_START` route-plan evaluation loads the caller’s own capacity by id and versions that capacity. A manual available capacity can therefore seed a supplied-load evaluation today. That is support for the seed source, not a chain search.

Next-load search reads `PredictedCapacity` for the effective available time. `DEPOT_START` plan preparation reads the capacity row and does not read the prediction row. Predicted capacity is a real one-load search input and is not yet a route-plan seed. The first runtime must not treat it as decided.

`CURRENT_TRIP` builds the start from a fresh tracking position, the end from the shipment destination, the initial capacity from the residual snapshot, and the onboard set from confirmed onboard cargo. That source can seed a supplied-load evaluation today. It does not discover L1 or L2.

## Horizon

`PRODUCT_SCOPE.md` problem class P3 names a configurable horizon of 24h, 48h, 72h, or N days. It does not choose one. Network-optimizer-service has no chain-horizon constant. Test fixtures that add 24 hours are local windows, not a chain policy.

```text
CHAIN_HORIZON_POLICY_ALREADY_FROZEN=NO
CHAIN_HORIZON_POLICY_VALUE=TBD
CHAIN_HORIZON_REQUIRES_POLICY_DECISION=YES
```

This discovery does not choose 24h, 48h, 72h, or N days.

## Future server-side pipeline

A later chain search, after policy values exist, follows this order:

1. Seed. Pin the chosen capacity context and its versions.
2. Visible first-load pool. Apply current publication and tenant rules. Do not scan another tenant’s unpublished loads.
3. First-load feasibility and pruning. Reuse existing hard rejects where they apply. Do not treat a one-load score as the chain objective.
4. Bounded first-load shortlist.
5. Second-load discovery for each shortlisted first load. The server selects L2. The caller does not pass the pair.
6. Visibility check for L2, including cross-shipper opt-in when the owners differ.
7. Compatibility precheck on the pair, fail closed when the result is unknown and the policy requires a known result.
8. Pair deduplication so the same ordered pair is evaluated once.
9. Bounded chain-candidate shortlist.
10. Route-plan evaluation of each shortlisted pair, using the existing bounded planner as a sequence evaluator.
11. Chain comparison and ranking under a frozen chain objective. Route-plan duration order is not that ranking.
12. Privacy-safe explanation. See [NLO_0_9A_CHAIN_SECURITY_PERSISTENCE_API.md](NLO_0_9A_CHAIN_SECURITY_PERSISTENCE_API.md).

The pipeline stops when a frozen global budget is exhausted. Those numbers are not set here.

## Future caps

The architecture requires distinct caps. Every value in this discovery is `TBD`. Reusing the next-load cap of 6 provider calls, or the route-plan cap of 300 provider calls, as the chain discovery budget is not accepted.

```text
FIRST_LOAD_SHORTLIST_CAP=TBD
SECOND_LOADS_PER_FIRST_CAP=TBD
CHAIN_PAIR_CAP=TBD
ROUTEPLAN_EVALUATION_CAP=TBD
CHAIN_GLOBAL_PROVIDER_CALL_CAP=TBD
CHAIN_GLOBAL_WALLCLOCK_BUDGET=TBD
ROUTEPLAN_BUDGET_IS_NOT_CHAIN_DISCOVERY_BUDGET=YES
```

`ROUTEPLAN_EVALUATION_CAP` limits how many shortlisted pairs may enter the existing planner. It is not permission to raise `MaxAdditionalLoads` above 2.

## Combinatorial risk

```text
COMBINATORIAL_EXPLOSION_RISK=HIGH
DISCOVERY_POOL_MAX=1000
ROUTED_CANDIDATE_MAX=25
MAX_ADDITIONAL_LOADS=2
```

A 1000-load discovery pool and a 25-load routing shortlist show the current one-load bounds. Pairing either set with a second load without a separate prune multiplies the work. Each pair may then spend the route-plan budget of 64 sequences, 300 leg evaluations, 300 provider calls, and 5 seconds. The first runtime shape stays at two loads so the fan-out is one pair layer, not an N-load recursion. The pair layer still needs its own caps before implementation.

## Route-plan reuse

The existing planner is the future bounded sequence evaluator for one discovered pair. It is not evidence that chain discovery exists.

```text
NEW_MULTI_LOAD_PLANNER_FROM_SCRATCH_REQUIRED=NO
CHAIN_DISCOVERY_ENGINE_PRESENT=NO
CHAIN_COMPARATIVE_RANKER_PRESENT=NO
```

What the planner already does is listed in the discovery inventory: placement enumeration, capacity and onboard evolution, groupage recheck, windows, service duration, road legs, fingerprints, and multi-load persistence.

What it does not do:

```text
CURRENT_ROUTEPLAN_LOAD_ORDER=DETERMINISTIC_UUID_SORT
CURRENT_ROUTEPLAN_INSERTION=SEQUENTIAL_GREEDY_INSERTION
GLOBAL_TWO_LOAD_ORDER_ENUMERATION_PRESENT=NO
GLOBAL_CHAIN_OPTIMUM_CLAIM_ALLOWED=NO
ROUTEPLAN_CHAIN_ECONOMIC_SCORE_PRESENT=NO
```

`BLOCKER_CHAIN_007=ROUTEPLAN_SELECTION_POLICY` stays open. A later freeze must say whether sequential greedy insertion is accepted for `TWO_LOAD_CHAIN_TOTAL`, or whether a bounded order comparison is required. This document does not change the planner and does not claim a global optimum.

## Objective

```text
ONE_LOAD_SCORE_PRESENT=YES
CHAIN_SCORE_PRESENT=NO
CHAIN_COMPARATIVE_RANKING_PRESENT=NO
CHAIN_OBJECTIVE_POLICY_PRESENT=NO
FREIGHT_COST_AVAILABLE_FOR_CHAIN_OBJECTIVE=PARTIAL
CHAIN_OBJECTIVE_POLICY=TBD
```

One-load profiles include deadhead, capacity utilization, return home, and revenue when currency is present. Revenue input exists for that one-load profile. It is partial evidence for a chain objective, not a chain objective. The one-load score must not be silently reused. Route-plan duration ordering must not be called a commercial chain ranking. This discovery does not choose the objective.

## Compatibility reuse

```text
TEMPERATURE_CHECK_PRESENT=YES
ADR_CHECK_PRESENT=YES
FOOD_GRADE_CHECK_PRESENT=YES
ODOR_CONTAMINATION_CHECK_PRESENT=YES
MULTI_LOAD_GROUPAGE_COMPATIBILITY_PRESENT=YES
STACKING_CHECK_PRESENT=NO
BLOCKER_CHAIN_006=STACKING_COMPATIBILITY
```

Temperature, ADR, food grade, odor, and contamination can be reused on the evolving onboard set. Stacking cannot. Until `STACKING_POLICY` is frozen, a later runtime must not claim that chain compatibility is complete.

Intermediate groupage already runs before each subsequent stop, on the cargo still onboard. A later chain test must show incompatibility between L1 and L2 while both are onboard, and must show capacity after L1 pickup, after L1 delivery, and before L2 pickup. Those tests are specified, not implemented, in [NLO_0_9A_CHAIN_TEST_AND_IMPLEMENTATION_ROADMAP.md](NLO_0_9A_CHAIN_TEST_AND_IMPLEMENTATION_ROADMAP.md).
