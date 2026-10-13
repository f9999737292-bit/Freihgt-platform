# NLO-0.9A chain test and implementation roadmap

These tests are specified for a later runtime. None are implemented in NLO-0.9A. None have been executed. The status of every row is `FUTURE_REQUIRED`.

```text
NLO_0_9A_MODE=DISCOVERY_DOCS_ONLY
CHAIN_RUNTIME_IMPLEMENTED=NO
IMPLEMENTATION_AUTHORIZED=NO
NEXT_STAGE_AUTHORIZED=NO
TESTS_IMPLEMENTED_IN_0_9A=NO
```

Evidence for the cases: [NLO_0_9A_CHAIN_OPTIMIZER_DISCOVERY.md](NLO_0_9A_CHAIN_OPTIMIZER_DISCOVERY.md), [NLO_0_9A_CHAIN_SEARCH_MODEL.md](NLO_0_9A_CHAIN_SEARCH_MODEL.md), and [NLO_0_9A_CHAIN_SECURITY_PERSISTENCE_API.md](NLO_0_9A_CHAIN_SECURITY_PERSISTENCE_API.md).

## Future acceptance tests

| ID | Case | Status |
| --- | --- | --- |
| CHAIN-01 | Deterministic pair discovery. The same seed and the same visible loads produce the same ordered L1 and L2 pair. | FUTURE_REQUIRED |
| CHAIN-02 | Candidate bounds. First-load shortlist, second loads per first load, pair cap, and route-plan evaluation cap stop expansion. The numeric caps are the values frozen later, not values invented here. | FUTURE_REQUIRED |
| CHAIN-03 | Global budget exhaustion. Provider-call and wall-clock budgets stop the search with an explicit budget result. | FUTURE_REQUIRED |
| CHAIN-04 | Horizon boundary. A load outside the frozen horizon is excluded. A load inside it remains eligible for discovery. | FUTURE_REQUIRED |
| CHAIN-05 | Cross-tenant deny. A tenant cannot read another tenant’s unpublished load or another tenant’s chain search. | FUTURE_REQUIRED |
| CHAIN-06 | Cross-shipper visibility. A foreign load without cross-shipper opt-in is not placed in a chain result. Individual visibility alone is not enough. | FUTURE_REQUIRED |
| CHAIN-07 | Unknown compatibility fails closed. Missing multi-party reference context does not become a feasible chain. | FUTURE_REQUIRED |
| CHAIN-08 | Stacking policy behavior. The frozen stacking policy accepts or rejects the pair. The current absence of a stacking check is not treated as a pass. | FUTURE_REQUIRED |
| CHAIN-09 | Capacity after L1 pickup and after L1 delivery. Pickup decreases remaining capacity. Delivery restores it. Unknown capacity does not become zero. | FUTURE_REQUIRED |
| CHAIN-10 | Capacity before L2 pickup. L2 is checked against the capacity that remains after the preceding actions, not against the seed capacity alone. | FUTURE_REQUIRED |
| CHAIN-11 | Intermediate groupage incompatibility. L1 and L2 onboard together are rejected when groupage returns a hard incompatibility. | FUTURE_REQUIRED |
| CHAIN-12 | Window feasibility. An arrival after the stop window drops that sequence. Waiting until a window opens is allowed only inside the window. | FUTURE_REQUIRED |
| CHAIN-13 | Unknown service duration. Missing service duration yields an indeterminate plan and blocks activation while the production service-duration gate stays on. | FUTURE_REQUIRED |
| CHAIN-14 | Routing provider failure. A failed or invalid road leg does not become a feasible chain and is not replaced by straight-line distance. | FUTURE_REQUIRED |
| CHAIN-15 | Duplicate chain candidate. The same ordered pair is evaluated once. | FUTURE_REQUIRED |
| CHAIN-16 | Version drift. A changed load version or capacity version changes the evaluation fingerprint and does not replay the old result as current. | FUTURE_REQUIRED |
| CHAIN-17 | Policy version drift. A changed service-duration policy, catalog, rule set, or algorithm policy version changes the fingerprint. | FUTURE_REQUIRED |
| CHAIN-18 | Two-load route persistence. An evaluated pair stores two load subjects, their versions, stops, actions, and road legs. | FUTURE_REQUIRED |
| CHAIN-19 | Ranking determinism. Equal inputs and the same frozen chain objective produce the same order. Duration-only planner order is not substituted for that objective. | FUTURE_REQUIRED |
| CHAIN-20 | No raw foreign commercial data. A pre-accept chain result omits the other shipper’s commercial terms, contracts, customer identity, and internal shipment ids. | FUTURE_REQUIRED |
| CHAIN-21 | Activation and handoff contract. Accept, activate, and projection match the frozen execution-support boundary. Today’s planner result stays `ExecutionSupported=false` until that proof exists. | FUTURE_REQUIRED |
| CHAIN-22 | Idempotent execution projection. The same activation key and body replays. A reused key with a different body conflicts. | FUTURE_REQUIRED |

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

## Decisions NLO-0.9B must freeze

NLO-0.9B cannot authorize implementation until each of these has a value. NLO-0.9A leaves them unset where the source does not already decide them.

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
| API_RESOURCE_SHAPE | SEPARATE_CHAIN_SEARCH_RESOURCE, path TBD |
| EXECUTION_SUPPORT_BOUNDARY | TBD |

`API_RESOURCE_SHAPE` records the discovery recommendation only. The path and the public-versus-internal choice remain open. `CHAIN_DISCOVERY_PERSISTENCE` records that route-plan storage can hold an evaluated two-load plan and that match-candidate rows cannot hold a chain. It does not authorize a migration.

## Blockers carried forward

All remain open:

```text
BLOCKER_CHAIN_001=HORIZON_POLICY
BLOCKER_CHAIN_002=SEARCH_BUDGET
BLOCKER_CHAIN_003=CHAIN_IDENTITY_AND_DISCOVERY_PERSISTENCE
BLOCKER_CHAIN_004=CHAIN_RANKING_AND_OBJECTIVE
BLOCKER_CHAIN_005=CROSS_SHIPPER_PRIVACY
BLOCKER_CHAIN_006=STACKING_COMPATIBILITY
BLOCKER_CHAIN_007=ROUTEPLAN_SELECTION_POLICY
BLOCKS_DISCOVERY=NO
BLOCKS_IMPLEMENTATION=YES
```

Detail and source evidence are in [NLO_0_9A_CHAIN_OPTIMIZER_DISCOVERY.md](NLO_0_9A_CHAIN_OPTIMIZER_DISCOVERY.md).
