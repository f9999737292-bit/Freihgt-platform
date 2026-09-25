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

The block above records the architecture freeze. BNO-0.1A later added the foundation runtime described in [BNO_0_1A_IMPLEMENTATION.md](BNO_0_1A_IMPLEMENTATION.md). BNO-0.1B added rule-based predicted capacity and the capability foundation in [BNO_0_1B_IMPLEMENTATION.md](BNO_0_1B_IMPLEMENTATION.md). BNO-0.1B2 added cargo and equipment reference data and the compatibility foundation in [BNO_0_1B2_REFERENCE_COMPATIBILITY.md](BNO_0_1B2_REFERENCE_COMPATIBILITY.md). Next-load matching and later roadmap stages remain not implemented.

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
| [adr/](adr/) | ADR-NET-001 … ADR-NET-012 |

ADR numbering follows the repository convention of a domain prefix (`ADR-EDO-*`, `ADR-RFX-*`, `ADR-PLAT-*`). These records live with the freeze pack and are **Proposed**, not Accepted, until controller review.

## Status labels used in discovery

`IMPLEMENTED`, `PARTIAL`, `DOCS_ONLY`, `PLANNED`, `NOT_FOUND`. A component is not treated as real because a document mentions it.
