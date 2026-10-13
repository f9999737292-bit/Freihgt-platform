# NLO-0.9A chain test and implementation roadmap

These tests are specified for a later runtime. None are implemented in NLO-0.9A. None have been executed. The status of every row is `FUTURE_REQUIRED`.

NLO-0.9B supersedes the unfrozen policy references below. Current-source facts in the 0.9A discovery stay in force. Concrete expectations now come from [NLO_0_9B_CHAIN_ARCHITECTURE_FREEZE.md](NLO_0_9B_CHAIN_ARCHITECTURE_FREEZE.md) and [ADR-NET-025](adr/ADR-NET-025-bounded-two-load-chain.md).

```text
NLO_0_9A_MODE=DISCOVERY_DOCS_ONLY
NLO_0_9A_POLICY_VALUES_SUPERSEDED_BY=NLO-0.9B
CHAIN_RUNTIME_IMPLEMENTED=NO
IMPLEMENTATION_AUTHORIZED=NO
NEXT_STAGE_AUTHORIZED=NO
TESTS_IMPLEMENTED_IN_0_9A=NO
CHAIN_TESTS_IMPLEMENTED=NO
```

Evidence for the cases: [NLO_0_9A_CHAIN_OPTIMIZER_DISCOVERY.md](NLO_0_9A_CHAIN_OPTIMIZER_DISCOVERY.md), [NLO_0_9A_CHAIN_SEARCH_MODEL.md](NLO_0_9A_CHAIN_SEARCH_MODEL.md), and [NLO_0_9A_CHAIN_SECURITY_PERSISTENCE_API.md](NLO_0_9A_CHAIN_SECURITY_PERSISTENCE_API.md).

## Future acceptance tests

| ID | Expected behavior under the 0.9B freeze | Status |
| --- | --- | --- |
| CHAIN-01 | The same seed, visible loads, and policy version produce the same selected order. A new request without the same idempotency key receives a new `chain_search_id`. | FUTURE_REQUIRED |
| CHAIN-02 | Expansion stops at 8 first loads, 2 second loads per first load, 4 pairs, and 2 route-plan evaluations. | FUTURE_REQUIRED |
| CHAIN-03 | The 25th provider call or the 120s wall clock returns `CHAIN_BUDGET_EXHAUSTED`, `search_complete=false`, and no global-optimum flag. | FUTURE_REQUIRED |
| CHAIN-04 | A load whose pickup starts at or after seed availability plus 48h is excluded. A load inside that fixed UTC window can remain eligible. Caller N-day overrides are rejected. | FUTURE_REQUIRED |
| CHAIN-05 | A tenant cannot read another tenant’s unpublished load or another tenant’s chain search. | FUTURE_REQUIRED |
| CHAIN-06 | A foreign load without `cross_shipper_consolidation_allowed` is not placed in a returned pair. | FUTURE_REQUIRED |
| CHAIN-07 | Missing multi-party reference context does not become a feasible chain. | FUTURE_REQUIRED |
| CHAIN-08 | Overlapping onboard time with unknown or false `stackable` is rejected. `stackable=true` adds no pallet space. Non-overlapping loads are not rejected for stacking. | FUTURE_REQUIRED |
| CHAIN-09 | L1 pickup decreases remaining capacity and L1 delivery restores it. Unknown capacity is not stored as zero. | FUTURE_REQUIRED |
| CHAIN-10 | L2 pickup is checked against capacity after the preceding actions. | FUTURE_REQUIRED |
| CHAIN-11 | L1 and L2 onboard together are rejected when groupage returns a hard incompatibility. | FUTURE_REQUIRED |
| CHAIN-12 | Arrival after the stop window drops that sequence. Waiting until the window opens is allowed only inside the window and inside the 48h horizon. | FUTURE_REQUIRED |
| CHAIN-13 | Unknown service duration yields an indeterminate plan. NLO-0.9C still returns `execution_supported=false`. | FUTURE_REQUIRED |
| CHAIN-14 | A failed road leg excludes the pair. Straight-line distance is not a substitute and is not zero. | FUTURE_REQUIRED |
| CHAIN-15 | The same unordered load pair and versions in one search produce one `chain_candidate_id`. Both action orders are evaluations of that candidate. | FUTURE_REQUIRED |
| CHAIN-16 | A changed load or capacity version changes the evaluation fingerprint and does not replay the previous result as current. | FUTURE_REQUIRED |
| CHAIN-17 | A changed service-duration policy, catalog, rule set, or algorithm policy version changes the fingerprint. | FUTURE_REQUIRED |
| CHAIN-18 | A finished evaluation stores two load subjects, their versions, stops, actions, and road legs on the route plan. The chain shortlist stores the candidate separately. | FUTURE_REQUIRED |
| CHAIN-19 | Equal inputs rank by empty road kilometres, then duration, then known utilization, then target distance, then authoritative revenue, then load-id tie-break. Missing revenue is omitted, not zero. One-load score and greedy duration order are not the chain rank. | FUTURE_REQUIRED |
| CHAIN-20 | The pre-accept result omits the other shipper’s identity, customer, contract, rate, and internal shipment id, including inside reason text. | FUTURE_REQUIRED |
| CHAIN-21 | NLO-0.9C returns `execution_supported=false` and does not activate. Accept, activate, and TMS proof belong to NLO-0.9D and are not enabled by this freeze. | FUTURE_REQUIRED |
| CHAIN-22 | The same chain-search idempotency key and body hash replay the same search id. A reused key with a different hash conflicts. | FUTURE_REQUIRED |

```text
CHAIN_TESTS_PASSED_IN_0_9A=0
CHAIN_TESTS_STATUS=FUTURE_REQUIRED
```

## Recommended later stages

These names are recommendations. This document does not authorize any of them.

| Stage | Role |
| --- | --- |
| NLO-0.9B-CHAIN-ARCHITECTURE-FREEZE | Freeze the decisions listed below. Still no runtime until a later authorization. |
| NLO-0.9C-CHAIN-RUNTIME-FOUNDATION | Possible first runtime of `TWO_LOAD_CHAIN_TOTAL` after 0.9B. Not authorized. |
| NLO-0.9D-CHAIN-EXECUTION-ACCEPTANCE | Possible proof of accept, activate, and TMS acknowledgement for that bounded chain. Not authorized. |

`AT_RISK` reoptimization stays a separate NLO-0.9 substage after the first bounded chain. NLO-0.9A does not include it. NLO-0.6 remains regional routing, open and closed, and is not a chain stage.

```text
NEXT_RECOMMENDED_STAGE=NLO-0.9B-CHAIN-ARCHITECTURE-FREEZE
NEXT_STAGE_AUTHORIZED=NO
NLO_0_9A_INCLUDES_AT_RISK_REOPTIMIZATION=NO
CHAIN_IMPLEMENTATION_CAN_START_NOW=NO
```

## Decisions NLO-0.9A left open

NLO-0.9A left these unset. NLO-0.9B supersedes this table in [NLO_0_9B_CHAIN_ARCHITECTURE_FREEZE.md](NLO_0_9B_CHAIN_ARCHITECTURE_FREEZE.md). The 0.9A column is the discovery record, not the current policy.

| Decision | NLO-0.9A value |
| --- | --- |
| CHAIN_HORIZON_POLICY | TBD |
| CHAIN_DISCOVERY_BUDGET | TBD |
| CHAIN_OBJECTIVE_POLICY | TBD |
| CHAIN_IDENTITY | TBD |
| CHAIN_DISCOVERY_PERSISTENCE | CONDITIONAL, schema not designed |
| CHAIN_PRIVACY_SURFACE | Current gap recorded, surface not frozen |
| STACKING_POLICY | TBD |
| ROUTEPLAN_SELECTION_POLICY | Current behavior recorded, acceptance not frozen |
| FIRST_RUNTIME_SEED_SOURCE | TBD |
| API_RESOURCE_SHAPE | SEPARATE_CHAIN_SEARCH_RESOURCE, path unset in 0.9A |
| EXECUTION_SUPPORT_BOUNDARY | unset in 0.9A |

The 0.9A path and execution boundary were unset. NLO-0.9B freezes both. `CHAIN_DISCOVERY_PERSISTENCE` in 0.9A recorded that route-plan storage can hold an evaluated two-load plan and that match-candidate rows cannot hold a chain. NLO-0.9B selects search-run and shortlist persistence for a later migration. It does not authorize that migration.

## Blockers carried forward

NLO-0.9A recorded these as open. NLO-0.9B closes each one in the architecture freeze. Closure is not runtime implementation.

```text
BLOCKER_CHAIN_001=HORIZON_POLICY
BLOCKER_CHAIN_002=SEARCH_BUDGET
BLOCKER_CHAIN_003=CHAIN_IDENTITY_AND_DISCOVERY_PERSISTENCE
BLOCKER_CHAIN_004=CHAIN_RANKING_AND_OBJECTIVE
BLOCKER_CHAIN_005=CROSS_SHIPPER_PRIVACY
BLOCKER_CHAIN_006=STACKING_COMPATIBILITY
BLOCKER_CHAIN_007=ROUTEPLAN_SELECTION_POLICY
NLO_0_9A_BLOCKS_DISCOVERY=NO
NLO_0_9B_BLOCKER_STATUS=CLOSED
```

Detail and source evidence are in [NLO_0_9A_CHAIN_OPTIMIZER_DISCOVERY.md](NLO_0_9A_CHAIN_OPTIMIZER_DISCOVERY.md).
