# BINTRANS Transportation & Network Optimizer v0.1

Architecture freeze for the BINTRANS network optimizer. This pack is documentation only.

```text
PRODUCT_CODE_CHANGE=NO
PRODUCTION_MIGRATION=NO
NEW_RUNTIME_SERVICE=NO
DEPLOYMENT_CHANGE=NO
STOP_AFTER_NETWORK_OPTIMIZER_V0_1_ARCHITECTURE_FREEZE=YES
IMPLEMENTATION_AUTHORIZED=NO
```

Baseline at freeze authoring: branch `discovery/bintrans-transport-network-optimizer-v0.1`, SHA `2c0ad2bd573ca78788d09d13cfe629b4023b88d9` (same as local `origin/main` after `git fetch origin` on 2026-09-24).

The block above records the original architecture freeze. Later accepted stages are BNO-0.1A, BNO-0.1B, BNO-0.1B2, BNO-0.1C0, BNO-0.1C1, and BNO-0.1C2. BNO-0.1C is closed on `origin/main` `6d47cc92`. NLO-0.3A is FROZEN_ACCEPTED. It records current-trip fill and consolidation and does not implement them. See [NLO_0_3_IMPLEMENTATION_ROADMAP.md](NLO_0_3_IMPLEMENTATION_ROADMAP.md).

## What this is

One optimization core with two policy modes:

- **MODE A — Marketplace Optimizer** (commercial priority): many shippers, many loads, many carriers, current and predicted capacity.
- **MODE B — Private Fleet Optimizer**: the same core with fleet objectives, explicit vehicle, driver, and trailer resources.

The product optimizes three kinds of unused capacity: empty kilometres, empty space, and empty time. The umbrella metric is `NETWORK_UTILIZATION`, always decomposable into primary measures.

## What this is not

Not a second freight exchange, not a second RFx engine, not a second freight-cost ledger, not a slot-booking owner, and not a Control Tower replacement. Existing Transport Order and Shipment contracts stay unchanged until a later, separately authorized implementation.

## Read order

| Document | Purpose |
|----------|---------|
| [CURRENT_STATE_INVENTORY.md](CURRENT_STATE_INVENTORY.md) | Code-backed status of existing capabilities |
| [PRODUCT_SCOPE.md](PRODUCT_SCOPE.md) | Modes, problem classes, MVP, non-goals |
| [DOMAIN_MODEL.md](DOMAIN_MODEL.md) | CargoUnit, LoadOpportunity, Capacity, route concepts |
| [ERD.md](ERD.md) | Entities and source-of-truth classification |
| [SERVICE_BOUNDARIES.md](SERVICE_BOUNDARIES.md) | Owners and forbidden duplication |
| [RATE_OWNERSHIP_MATRIX.md](RATE_OWNERSHIP_MATRIX.md) | Commercial calculation ownership |
| [CONSOLIDATION_MODEL.md](CONSOLIDATION_MODEL.md) | Hard constraints vs score |
| [OPTIMIZATION_MODEL.md](OPTIMIZATION_MODEL.md) | Matching, economics, solver evolution, failure modes |
| [REGIONAL_ROUTING.md](REGIONAL_ROUTING.md) | Regional profile |
| [URBAN_ROUTING.md](URBAN_ROUTING.md) | Urban profile and clustering |
| [CITY_RULES.md](CITY_RULES.md) | Versioned city rules; Moscow and Saint Petersburg profiles |
| [MARKETPLACE_DATA_VISIBILITY_MATRIX.md](MARKETPLACE_DATA_VISIBILITY_MATRIX.md) | Privacy matrix and publication policy |
| [SECURITY_THREAT_MODEL.md](SECURITY_THREAT_MODEL.md) | Threats mapped to the current JWT/tenant model |
| [EVENTS_CATALOG.md](EVENTS_CATALOG.md) | Event names aligned to ADR-EDO-006 |
| [STATE_MACHINES.md](STATE_MACHINES.md) | Lifecycles, races, reoptimization |
| [API_CONTRACTS.md](API_CONTRACTS.md) | Draft API classification |
| [contracts/network-optimizer-v0.1.openapi.yaml](contracts/network-optimizer-v0.1.openapi.yaml) | Skeleton only; not `packages/openapi` |
| [DIAGRAMS.md](DIAGRAMS.md) | Context, components, sequences |
| [SCENARIOS.md](SCENARIOS.md) | SCN-01 … SCN-12 |
| [OBSERVABILITY.md](OBSERVABILITY.md) | Future metrics |
| [TEST_STRATEGY.md](TEST_STRATEGY.md) | Future tests, including golden scenarios |
| [ROADMAP.md](ROADMAP.md) | NLO-0.1 … NLO-1.0 and FLEET-1.x |
| [BNO_0_1A_IMPLEMENTATION.md](BNO_0_1A_IMPLEMENTATION.md) | Implemented foundation subset and explicit non-goals |
| [OPEN_QUESTIONS.md](OPEN_QUESTIONS.md) | Unresolved items |
| [CONSISTENCY_REVIEW.md](CONSISTENCY_REVIEW.md) | Pre-commit architecture, ownership, security, contract, and roadmap review |
| [FINAL_ARCHITECTURE_REPORT.md](FINAL_ARCHITECTURE_REPORT.md) | Freeze verdict |
| [NLO_0_3_CURRENT_STATE_INVENTORY.md](NLO_0_3_CURRENT_STATE_INVENTORY.md) | Code-backed inventory for current-trip fill |
| [NLO_0_3_IMPLEMENTATION_ROADMAP.md](NLO_0_3_IMPLEMENTATION_ROADMAP.md) | Frozen waves, traceability, and acceptance |
| [NLO_0_4_IMPLEMENTATION_ROADMAP.md](NLO_0_4_IMPLEMENTATION_ROADMAP.md) | NLO-0.4B, NLO-0.4C, and NLO-0.4D are on main. NLO-0.4D is accepted and closed at `00b53de9db50cdc1e875df95b8ba7ef2daa0fe8a` |
| [NLO_0_4A_CURRENT_STATE_INVENTORY.md](NLO_0_4A_CURRENT_STATE_INVENTORY.md) | Route-plan discovery inventory |
| [adr/](adr/) | ADR-NET-001 … ADR-NET-024 |
| [NLO_0_5A_CURRENT_STATE_INVENTORY.md](NLO_0_5A_CURRENT_STATE_INVENTORY.md) | Routing and search inventory for backhaul |
| [NLO_0_5A_BACKHAUL_MODEL.md](NLO_0_5A_BACKHAUL_MODEL.md) | One-load backhaul |
| [NLO_0_5A_ROUNDTRIP_MODEL.md](NLO_0_5A_ROUNDTRIP_MODEL.md) | Bounded return toward the policy target |
| [NLO_0_5A_ROUTING_POLICY.md](NLO_0_5A_ROUTING_POLICY.md) | Road distance, cache, and fail-closed routing |
| [NLO_0_5_IMPLEMENTATION_ROADMAP.md](NLO_0_5_IMPLEMENTATION_ROADMAP.md) | NLO-0.5 waves after this freeze |
| [NLO_0_5A_TEST_STRATEGY.md](NLO_0_5A_TEST_STRATEGY.md) | NLO05-01 through NLO05-25 |
| [NLO_0_5B_IMPLEMENTATION_ROADMAP.md](NLO_0_5B_IMPLEMENTATION_ROADMAP.md) | Discovery cap, routing budget, and one-load feasibility |
| [NLO_0_5B_TEST_STRATEGY.md](NLO_0_5B_TEST_STRATEGY.md) | B2 routing-budget gates and B3 one-load feasibility |
| [NLO_0_5B0_BENCHMARK.md](NLO_0_5B0_BENCHMARK.md) | Local prefilter sample and uncapped call accounting |
| [NLO_0_4D_ARCHITECTURE_FREEZE.md](NLO_0_4D_ARCHITECTURE_FREEZE.md) | NLO-0.4D-R1 docs-only freeze for service duration and TMS acknowledgement |
| [SERVICE_DURATION_SOURCE.md](SERVICE_DURATION_SOURCE.md) | Operating-tenant service-duration policy |
| [EXECUTION_HANDOFF_ACK.md](EXECUTION_HANDOFF_ACK.md) | Synchronous projection acknowledgement |
| [ACTIVATION_STATE_MACHINE.md](ACTIVATION_STATE_MACHINE.md) | Activation statuses and link gates |

ADR numbering follows the repository convention of a domain prefix (`ADR-EDO-*`, `ADR-RFX-*`, `ADR-PLAT-*`). ADR-NET-001 through ADR-NET-012 remain Proposed. ADR-NET-013 through ADR-NET-021 are Accepted. ADR-NET-022 records the NLO-0.4D service-duration and execution-acknowledgement freeze. ADR-NET-023 is the NLO-0.5A backhaul and roundtrip architecture, merged in PR #201 at `4c8b22e8`. ADR-NET-024 accepts the bounded search policy: discovery 1000 is enforced by NLO-0.5B1, the routing budget is enforced by NLO-0.5B2, and one-load feasibility is evaluated by NLO-0.5B3. `BACKHAUL_RUNTIME_IMPLEMENTED=NO`. `ROUNDTRIP_RUNTIME_IMPLEMENTED=NO`. `EXTERNAL_PROVIDER_BENCHMARK=BLOCKED`. `PROVIDER_SLA_PROVEN=NO`. NLO-0.3 is complete. NLO-0.4A architecture is frozen. NLO-0.4B is IMPLEMENTED_CLOSED on main `0fc6a7979ca5ea0cbbbd50ff5770bbbf22304a5c` (PR #184, migration `000085_nlo_route_plan_bounded_planner_v0_4b`). `NLO_0_4_IMPLEMENTATION_STARTED=YES`. `NLO_0_4B_STATUS=IMPLEMENTED_CLOSED`. `NLO_0_4C_ACCEPTED=YES`. `NLO_0_4D_ACCEPTED=YES`. `NLO_0_4D_CLOSED=YES`. `NLO_0_4D_MERGE_SHA=00b53de9db50cdc1e875df95b8ba7ef2daa0fe8a`. `TMS_PROJECTION_HANDSHAKE_IMPLEMENTED=YES`. NLO-0.5A is merged at `4c8b22e8`. NLO-0.5B1 is merged at `349eedc89e0c47a71e3b23a107cc97fad5aebc91`.

## Status labels used in discovery

`IMPLEMENTED`, `PARTIAL`, `DOCS_ONLY`, `PLANNED`, `NOT_FOUND`. A component is not treated as real because a document mentions it.
