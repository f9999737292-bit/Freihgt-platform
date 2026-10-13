# NLO-0.9A chain optimizer discovery

This document records the current source boundary. It does not implement a chain, freeze a horizon, freeze a search budget, or freeze an objective.

```text
NLO_0_9A_MODE=DISCOVERY_DOCS_ONLY
NLO_0_9A_STATUS=DISCOVERY_ONLY
CHAIN_RUNTIME_IMPLEMENTED=NO
IMPLEMENTATION_AUTHORIZED=NO
NEXT_STAGE_AUTHORIZED=NO
NLO_0_6_CANONICAL_SCOPE=REGIONAL_ROUTING_OPEN_AND_CLOSED
NLO_0_9_CANONICAL_SCOPE=CHAIN_OPTIMIZER_AND_AT_RISK_REOPTIMIZATION
NLO_0_9A_INCLUDES_AT_RISK_REOPTIMIZATION=NO
```

Base of this reading: `7dc1e9afd1b1012f6f0fdcfdae12ca771de3384c`, tree `0ca50c9ea4bf0dd09306a05db83cd0e446b58781`.

NLO-0.9B supersedes the unfrozen policy values in this discovery. Source facts in this document remain the inventory. See [NLO_0_9B_CHAIN_ARCHITECTURE_FREEZE.md](NLO_0_9B_CHAIN_ARCHITECTURE_FREEZE.md).

```text
NLO_0_9A_POLICY_VALUES_SUPERSEDED_BY=NLO-0.9B
RUNTIME_IMPLEMENTATION_AUTHORIZED=NO
```

Related discovery:

- [NLO_0_9A_CHAIN_SEARCH_MODEL.md](NLO_0_9A_CHAIN_SEARCH_MODEL.md)
- [NLO_0_9A_CHAIN_SECURITY_PERSISTENCE_API.md](NLO_0_9A_CHAIN_SECURITY_PERSISTENCE_API.md)
- [NLO_0_9A_CHAIN_TEST_AND_IMPLEMENTATION_ROADMAP.md](NLO_0_9A_CHAIN_TEST_AND_IMPLEMENTATION_ROADMAP.md)

`docs/architecture/network-optimizer/ROADMAP.md` keeps NLO-0.6 as regional routing, open and closed, and NLO-0.9 as the chain optimizer and `AT_RISK` reoptimization. This discovery does not rename NLO-0.6. `NLO_0_5_IMPLEMENTATION_ROADMAP.md` places chain search and live reoptimization in NLO-0.9. `PRODUCT_SCOPE.md` problem class P3 describes a load chain over a configurable horizon and does not select one horizon.

## Three different capabilities

```text
ONE_LOAD_DISCOVERY
!=
MULTI_LOAD_ROUTEPLAN_EVALUATION
!=
CHAIN_OPTIMIZER
```

| Capability | What the current source does | What it does not do |
| --- | --- | --- |
| One-load discovery | `SearchNextLoad` selects visible published loads and returns one-load candidates. Corridor and ellipse feasibility judge one load toward a target. | It does not select a second load or compose a chain. |
| Multi-load route-plan evaluation | `POST` route-plan evaluate accepts `candidate_load_ids` of length 1 or 2. The caller supplies the ids. The bounded planner inserts those loads. | Caller-selected ids are not server-side chain discovery. |
| Chain optimizer | Not present. A later chain must discover both future load subjects on the server, then evaluate the pair. | This discovery does not authorize that runtime. |

```text
NEXT_LOAD_SEARCH_PRESENT=YES
NEXT_LOAD_SEARCH_RETURNS_ONE_LOAD_CANDIDATES=YES
SERVER_SIDE_SECOND_LOAD_DISCOVERY=NO
SERVER_SIDE_CHAIN_COMPOSER=NO
CHAIN_SEARCH_MODE_PRESENT=NO
CHAIN_HORIZON_PRESENT=NO
ROUTEPLAN_INPUT_ACCEPTS_MULTIPLE_LOADS=YES
ROUTEPLAN_MAX_ADDITIONAL_LOADS=2
CALLER_CAN_PASS_TWO_LOAD_IDS=YES
CHAIN_LOAD_IDS_SERVER_DISCOVERED=NO
CHAIN_DISCOVERY_MISSING=YES
CHAIN_DISCOVERY_ENGINE_PRESENT=NO
```

## Current one-load search

`services/network-optimizer-service/internal/domain/geo.go` accepts only `RADIUS`, `DIRECTIONAL_CORRIDOR`, and `ROUTE_ELLIPSE`. `services/network-optimizer-service/internal/domain/backhaul_feasibility.go` states that a second load is a separate call and that `EvaluateOneLoadBackhaul` does not search, rank, or compose a second load. Corridor requires delivery-to-target road kilometres to be strictly smaller than pickup-to-target. Ellipse uses the four-leg insertion. A nil leg is unknown road distance. `RADIUS` is not a backhaul result.

`services/network-optimizer-service/internal/domain/discovery_bound.go` sets `CandidateDiscoveryCap` to 1000. `services/network-optimizer-service/internal/domain/routing_bound.go` sets the routing slice and provider budget below. `match_candidates.load_opportunity_id` stores one load per row (`infrastructure/migrations/000079_bno_next_load_candidate_search_v0_1c1.up.sql`).

Current-trip fill in `services/network-optimizer-service/internal/service/current_trip_fill.go` uses its own `MaxAdditionalLoads = 1`. That search is one additional load on an in-progress trip. It is not a chain composer. Pairwise and N-member consolidation remain same-origin / same-destination planning searches with `execution_supported=false`. They are not a geographic load chain.

## Current route-plan evaluation

`services/network-optimizer-service/internal/service/route_plan.go` rejects `candidate_load_ids` unless the array length is 1 or 2 and the ids are distinct. The server loads those ids. It does not search the marketplace for them.

`services/network-optimizer-service/internal/routeplan/policy.go`:

```text
MAX_ROUTE_LOAD_SUBJECTS=4
MAX_ADDITIONAL_LOADS=2
MAX_STOPS=8
MAX_ACTIONS_PER_STOP=4
MAX_SEQUENCE_CANDIDATES=64
MAX_ROUTE_LEG_EVALUATIONS=300
MAX_ROUTING_PROVIDER_CALLS=300
MAX_GROUPAGE_EVALUATIONS=600
ROUTEPLAN_TIME_BUDGET=5s
ALGORITHM_POLICY_VERSION=BOUNDED_INCREMENTAL_HEURISTIC/v0.4b
```

`Plan` rejects the input when `BaseSubjects + len(Loads)` exceeds 4 or `len(Loads)` exceeds 2. On `CURRENT_TRIP`, `BaseSubjects` is the count of confirmed onboard cargo units. Two additional loads therefore fit only when that onboard count leaves room inside the subject cap. `DEPOT_START` does not set `BaseSubjects`, so the additional-load cap is the binding limit for that mode.

The planner already provides, for a caller-supplied set:

- multi-load input
- deterministic load-id ordering
- pickup and delivery placement enumeration for each inserted load
- capacity decrease on pickup and restoration on delivery
- onboard cargo added on pickup and removed on delivery
- groupage recheck on the onboard set before each subsequent stop
- time-window validation
- service-duration handling, with unknown duration becoming indeterminate
- road-leg routing, with provider failure dropping the sequence
- version and fingerprint pinning, including every action subject version
- persistence of stops, actions, legs, and capacity snapshots for a two-load plan

```text
NEW_MULTI_LOAD_PLANNER_FROM_SCRATCH_REQUIRED=NO
CHAIN_COMPARATIVE_RANKER_PRESENT=NO
ROUTEPLAN_BUDGET_IS_NOT_CHAIN_DISCOVERY_BUDGET=YES
```

The route-plan budget bounds one evaluation of a supplied set. It is not a budget for discovering pairs from the marketplace. Numeric chain-search limits are not assigned in this discovery. See [NLO_0_9A_CHAIN_SEARCH_MODEL.md](NLO_0_9A_CHAIN_SEARCH_MODEL.md).

`routeplan.Plan` sets `ExecutionSupported=false` on every outcome. Migration `000085_nlo_route_plan_bounded_planner_v0_4b.up.sql` checks `execution_supported = false`. Later route-plan migrations do not relax that check.

## Current bounds

One-load and backhaul search, from `discovery_bound.go` and `routing_bound.go`:

```text
DISCOVERY_POOL_MAX=1000
ROUTED_CANDIDATE_MAX=25
NEXT_LOAD_PROVIDER_CALL_CAP=6
NEXT_LOAD_HARD_BUDGET=30s
```

`MaxProviderCallsTotal` is 6. That total covers at most 4 matrix calls and 2 route calls. Provider retry count is 0. Search parallelism is 1.

Route-plan evaluation, from `routeplan/policy.go`:

```text
MAX_ROUTE_LOAD_SUBJECTS=4
MAX_ADDITIONAL_LOADS=2
MAX_STOPS=8
MAX_ACTIONS_PER_STOP=4
MAX_SEQUENCE_CANDIDATES=64
MAX_ROUTE_LEG_EVALUATIONS=300
MAX_ROUTING_PROVIDER_CALLS=300
MAX_GROUPAGE_EVALUATIONS=600
ROUTEPLAN_TIME_BUDGET=5s
ROUTEPLAN_BUDGET_IS_NOT_CHAIN_DISCOVERY_BUDGET=YES
```

No chain shortlist cap, pair cap, provider cap, or wall-clock budget is frozen here. Those names stay `TBD` in the search model.

## Combinatorial risk

```text
COMBINATORIAL_EXPLOSION_RISK=HIGH
```

The marketplace discovery pool can contain 1000 loads. The next-load routing shortlist can contain 25. A naive expansion that pairs each shortlisted first load with another load from that pool multiplies the candidate count. Each surviving pair can then call the bounded route-plan evaluator, whose own caps are 64 sequences, 300 legs, 300 provider calls, and 5 seconds per evaluation. This discovery records the risk. It does not benchmark it and it does not state a production latency target.

## Planner selection limitation

```text
CURRENT_ROUTEPLAN_LOAD_ORDER=DETERMINISTIC_UUID_SORT
CURRENT_ROUTEPLAN_INSERTION=SEQUENTIAL_GREEDY_INSERTION
GLOBAL_TWO_LOAD_ORDER_ENUMERATION_PRESENT=NO
GLOBAL_CHAIN_OPTIMUM_CLAIM_ALLOWED=NO
ROUTEPLAN_CHAIN_ECONOMIC_SCORE_PRESENT=NO
```

`Plan` copies the supplied loads and sorts them by UUID string. It then inserts each load into the current winner. `insertLoad` enumerates pickup and delivery placements for that one load. `better` prefers a feasible result, then a shorter road duration, then a lower pickup ordinal, then a lower delivery ordinal.

The planner does not enumerate permutations of the two-load order. It does not claim a global optimum. Duration ordering is not a commercial chain ranking. This limitation is `BLOCKER_CHAIN_007` below. This discovery does not change planner code.

## Ranking and objective

```text
ONE_LOAD_SCORE_PRESENT=YES
CHAIN_SCORE_PRESENT=NO
CHAIN_COMPARATIVE_RANKING_PRESENT=NO
CHAIN_OBJECTIVE_POLICY_PRESENT=NO
FREIGHT_COST_AVAILABLE_FOR_CHAIN_OBJECTIVE=PARTIAL
CHAIN_OBJECTIVE_POLICY=TBD
```

`domain.KnownObjectiveProfile` recognizes one-load profiles, including `MAX_REVENUE` when a ranking currency is present. `ObjectiveExecutable` excludes `MAX_CONTRIBUTION`. That score applies to one next-load candidate. It is not a chain objective. Route-plan `better` does not read revenue, margin, or utilization. A later freeze must choose the chain objective. This discovery does not choose revenue, margin, kilometres, utilization, duration, or a weighted mix.

## Compatibility already present

Groupage evaluation is reused while the planner walks the stop sequence. The onboard set changes after each pickup and delivery. Temperature, ADR, and food-grade checks are prefix matches on groupage reason codes. Odor and contamination are existing cargo-to-cargo rules (`ODOR_EMISSION_CLASS`, `ODOR_SENSITIVE`, `CONTAMINATION_CLASS`).

```text
TEMPERATURE_CHECK_PRESENT=YES
ADR_CHECK_PRESENT=YES
FOOD_GRADE_CHECK_PRESENT=YES
ODOR_CONTAMINATION_CHECK_PRESENT=YES
MULTI_LOAD_GROUPAGE_COMPATIBILITY_PRESENT=YES
STACKING_CHECK_PRESENT=NO
```

`compat.Cargo.Stackable` is stored. The groupage evaluator does not apply a stacking compatibility rule. The existing test `BNO113_STACKABLE_DOES_NOT_INVENT_EXTRA_SPACE` records that a stackable flag does not create extra pallet space. Groupage support does not close the stacking gap. `BLOCKER_CHAIN_006` stays open. This discovery does not implement stacking. Generalized chain compatibility is not complete while that blocker is open.

A new groupage engine is not required for the checks that already exist. Cross-shipper evaluation still fails closed when the multi-party reference context is unavailable. That is a privacy and reference-context rule, recorded in [NLO_0_9A_CHAIN_SECURITY_PERSISTENCE_API.md](NLO_0_9A_CHAIN_SECURITY_PERSISTENCE_API.md).

## Seed, horizon, identity, and handoff

Seed evidence and the unset first-runtime seed policy are in the search model:

```text
CHAIN_SEED_SOURCE_MANUAL_AVAILABLE=SUPPORTED
CHAIN_SEED_SOURCE_PREDICTED_CAPACITY=CONDITIONAL
CHAIN_SEED_SOURCE_CURRENT_TRIP_RESIDUAL=SUPPORTED
FIRST_RUNTIME_SEED_SOURCE_POLICY=TBD
CHAIN_HORIZON_POLICY_ALREADY_FROZEN=NO
CHAIN_HORIZON_POLICY_VALUE=TBD
CHAIN_HORIZON_REQUIRES_POLICY_DECISION=YES
DEDICATED_CHAIN_IDENTITY_PRESENT=NO
```

Execution handoff evidence:

```text
MULTI_LOAD_ROUTE_PLAN_ACCEPT_SUPPORTED=YES
MULTI_LOAD_ROUTE_PLAN_ACTIVATE_SUPPORTED=CONDITIONAL
MULTI_LOAD_EXECUTION_HANDOFF_SUPPORTED=CONDITIONAL
TMS_CANONICAL_EXECUTION_OWNER=YES
CHAIN_DISCOVERY_SUCCESS_DOES_NOT_PROVE_EXECUTION_HANDOFF=YES
```

Accepting a feasible route plan does not require a one-load count. Activation projection requires a current-trip shipment context. `DEPOT_START` plans have no shipment id, so that projection path does not apply to them. Projected execution subjects are confirmed shipment cargo. Additional load-opportunity actions are copied onto the projection, and they are not execution subjects. `execution_supported` remains false. TMS remains the execution owner. A future chain discovery result does not prove that path.

## AT_RISK boundary

```text
AT_RISK_CHAIN_REOPTIMIZATION_PRESENT=NO
ACTIVE_EXECUTION_CHAIN_REPLAN_PRESENT=NO
NLO_0_9A_INCLUDES_AT_RISK_REOPTIMIZATION=NO
```

No `AT_RISK` chain reoptimization and no active-execution chain replan exist in network-optimizer-service. The canonical NLO-0.9 roadmap names both the chain optimizer and `AT_RISK` reoptimization. This discovery covers chain discovery only. `AT_RISK` remains a later NLO-0.9 substage, after a bounded chain exists.

## Analytics boundary

From `docs/architecture/analytics/ANALYTICS_0_1A_SOURCE_CONTRACT_GAPS.md`. This discovery does not edit that document or analytics code.

```text
GAP_D_001=OPEN
GAP_D_003=OPEN
GAP_D_004=OPEN
GAP_D_005=OPEN
CHAIN_0_9A_MUST_CLOSE_ANALYTICS_GAPS=NO
```

`GAP-D-001` is a planned deadhead-reduction fact. `GAP-D-003` is a backhaul opportunity and acceptance fact. `GAP-D-004` is a search-to-accept link. `GAP-D-005` is persisted `route_increase_km`. A later chain may become a source for those facts. Closing them is not part of NLO-0.9A.

## Blocker register

All seven blockers stay open. None blocks writing this discovery. Each can block a later production or general chain runtime until the named decision is frozen. Owner of the decision record is Agent D. NLO-0.9B must freeze the decisions before any runtime authorization. See [NLO_0_9A_CHAIN_TEST_AND_IMPLEMENTATION_ROADMAP.md](NLO_0_9A_CHAIN_TEST_AND_IMPLEMENTATION_ROADMAP.md).

### BLOCKER_CHAIN_001=HORIZON_POLICY

```text
CATEGORY=HORIZON_POLICY
SOURCE_EVIDENCE=PRODUCT_SCOPE P3 names 24h, 48h, 72h, or N days. No chain-horizon constant exists in network-optimizer-service.
BLOCKS_DISCOVERY=NO
BLOCKS_IMPLEMENTATION=YES
OWNER=D
REQUIRED_DECISION=CHAIN_HORIZON_POLICY
```

### BLOCKER_CHAIN_002=SEARCH_BUDGET

```text
CATEGORY=SEARCH_BUDGET
SOURCE_EVIDENCE=Next-load budget is 6 provider calls and 30s. Route-plan budget is 64 sequences, 300 legs, 300 provider calls, and 5s for one supplied set. No chain-search budget exists.
BLOCKS_DISCOVERY=NO
BLOCKS_IMPLEMENTATION=YES
OWNER=D
REQUIRED_DECISION=CHAIN_DISCOVERY_BUDGET
```

### BLOCKER_CHAIN_003=CHAIN_IDENTITY_AND_DISCOVERY_PERSISTENCE

```text
CATEGORY=CHAIN_IDENTITY
SOURCE_EVIDENCE=No chain_id, load_chain_id, chain_candidate_id, parent_candidate_id, predecessor_load_id, or successor_load_id. match_candidates stores one load_opportunity_id. route_plan_id identifies an evaluated plan only.
BLOCKS_DISCOVERY=NO
BLOCKS_IMPLEMENTATION=YES
OWNER=D
REQUIRED_DECISION=CHAIN_IDENTITY and CHAIN_DISCOVERY_PERSISTENCE
```

### BLOCKER_CHAIN_004=CHAIN_RANKING_AND_OBJECTIVE

```text
CATEGORY=RANKING
SOURCE_EVIDENCE=One-load score exists. Route-plan selection uses feasibility, duration, and stop ordinals. No chain score and no chain objective policy exist.
BLOCKS_DISCOVERY=NO
BLOCKS_IMPLEMENTATION=YES
OWNER=D
REQUIRED_DECISION=CHAIN_OBJECTIVE_POLICY
```

### BLOCKER_CHAIN_005=CROSS_SHIPPER_PRIVACY

```text
CATEGORY=PRIVACY
SOURCE_EVIDENCE=Individual marketplace visibility is not joint-chain disclosure. routeLoadVisible requires cross-shipper consolidation opt-in for a foreign load. Multi-owner reference context is unavailable, so the plan stays indeterminate.
BLOCKS_DISCOVERY=NO
BLOCKS_IMPLEMENTATION=YES
OWNER=D
REQUIRED_DECISION=CHAIN_PRIVACY_SURFACE
```

### BLOCKER_CHAIN_006=STACKING_COMPATIBILITY

```text
CATEGORY=COMPATIBILITY
SOURCE_EVIDENCE=Stackable is a cargo field. Groupage does not evaluate a stacking rule. Stackable does not invent extra space.
BLOCKS_DISCOVERY=NO
BLOCKS_IMPLEMENTATION=YES
BLOCKS_GENERAL_CHAIN_IMPLEMENTATION=YES
OWNER=D
REQUIRED_DECISION=STACKING_POLICY
```

### BLOCKER_CHAIN_007=ROUTEPLAN_SELECTION_POLICY

```text
CATEGORY=ROUTEPLAN_SELECTION_POLICY
SOURCE_EVIDENCE=Plan sorts loads by UUID string and inserts each load into the current winner. It does not enumerate two-load order permutations. better does not apply a commercial score.
BLOCKS_DISCOVERY=NO
BLOCKS_IMPLEMENTATION=YES
OWNER=D
REQUIRED_DECISION=ROUTEPLAN_SELECTION_POLICY
```

## What this discovery authorizes

It authorizes the four NLO-0.9A documents only. It does not authorize runtime code, a migration, an OpenAPI change, a gateway change, staging, a benchmark, an ADR, or a roadmap edit.

```text
CHAIN_IMPLEMENTATION_CAN_START_NOW=NO
CHAIN_RUNTIME_IMPLEMENTED=NO
```
