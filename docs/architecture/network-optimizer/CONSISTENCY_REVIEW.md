# Pre-commit review

Reviewed on 2026-09-24 against the code at `2c0ad2bd573ca78788d09d13cfe629b4023b88d9` and this documentation tree. No product code, migration, runtime, or deployment file is part of the change.

## 1. Architecture consistency

PASS. One core, two modes. Marketplace is the primary mode. Empty kilometres, empty space, and empty time roll up to `NETWORK_UTILIZATION` without hiding the parts. CargoUnit through OptimizationDecision are in the domain model and ERD. SCN-01 through SCN-12 are in `SCENARIOS.md`. Hub and cross-dock are plan roles, not a new execution service. Solver choice stays a staged comparison, not a global-optimum v0.1.

Fix applied before commit: cross-shipper text now states Shipper A, B, and C on one vehicle, not only a pair.

## 2. Ownership

PASS. Transport order, shipment, RFx, contract rate, freight-cost ledger, slot intelligence, documents, and settlement stay with their current services. BNO owns plans, projections, offers, and decision snapshots. Planning contribution is not a `CostEntry`. Competitive procedures hand off to RFx. Multi-stop plans cannot become `ACTIVE` until shipment execution can represent them.

## 3. Tenant security

PASS. Matching reads published projections, not `transport.shipments` across tenants. The visibility matrix states the owning shipper, another shipper, candidate carrier, assigned carrier, driver, and the BNO internal service. Out-of-scope reads hide existence. Gateway JWT remains the identity source.

## 4. Event and API consistency

PASS. New events use `network.*` past tense per ADR-EDO-006. Existing `shipment.*` and `driver.*` names are consumed, not renamed. `shipment.eta.updated` is not claimed as a live event. Draft routes use `/api/v1/network/...` and live only under `contracts/`. `packages/openapi` is unchanged. Public APIs do not return foreign commercial fields or raw decision internals.

## 5. Roadmap dependencies

PASS. NLO-0.2 depends on NLO-0.1 and a tracking ETA read. NLO-0.4 plans exist before execution is legal. NLO-0.5 needs a routing port. NLO-0.7 and NLO-0.8 need sourced city-rule data and do not fork the solver. NLO-1.0 waits on measured earlier stages. FLEET-1.x uses the same core. First implementation recommendation remains NLO-0.1 plus NLO-0.2, and it is not authorized by this freeze.

## 6. Git diff

PASS for scope. The expected diff is `docs/architecture/README.md` plus `docs/architecture/network-optimizer/**`. No service, app, migration, compose, or production OpenAPI path.

## Verdict

Documentation gates are covered. `IMPLEMENTATION_AUTHORIZED=NO`.
