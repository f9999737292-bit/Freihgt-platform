# Roadmap

Each stage uses the same core. Later stages do not replace earlier contracts; they add problem classes. Implementation of any stage needs a new authorization.

BNO-0.1A implements the foundation subset of NLO-0.1: manual capacity, explicit load publication, visibility, persistence, API, outbox, and audit. It does not implement hard feasibility, persisted match candidates, or explanations.

BNO-0.1B is IMPLEMENTED: rule-based `PredictedCapacity`, normalized body and combination capability, independent loading and unloading access, temperature capability and cargo requirements, and compatibility primitives.

BNO-0.1B2 is IMPLEMENTED: versioned cargo, equipment, pallet, and packaging catalogs, tenant compatibility rule sets, and explainable cargo-equipment and groupage evaluation. It does not implement next-load search, ranking, or a physical trailer asset master.

BNO-0.1C0 is IMPLEMENTED: location snapshots, search and display geography, the routing port, and search-policy primitives. It does not search loads.

BNO-0.1C1 is IMPLEMENTED: a carrier-owned available capacity can search visible published loads and persist hard-feasible match candidates.

BNO-0.1C2 is IMPLEMENTED on `origin/main` `6d47cc92`: deterministic match score, stored system objective profiles, and top N after hard feasibility. BNO-0.1C is CLOSED.

NLO-0.2 is IMPLEMENTED as code on that same main. Rule-based `PredictedCapacity` reads a tenant-scoped shipment prediction input and a tracking ETA, then publishes future empty capacity after unload. BNO-0.1C1 search and BNO-0.1C2 top N rank that capacity against one load. Prediction fails closed when `BNO_PREDICTION_MAX_ETA_AGE` is unset or the ETA is older than that policy. That policy is not the tracking-service location freshness (10/30 minutes) or ETA freshness (15/60 minutes). NLO-0.2 does not compute in-trip residual capacity. GPS position is not an input.

NLO-0.3 architecture is FROZEN by NLO-0.3A and is NOT IMPLEMENTED. The first authorized product wave, when a controller authorizes it, is current-trip context and residual capacity, then pairwise same-origin/same-destination feasibility and one additional current-trip load. Unrestricted N-load search, a solver, and multi-stop shipment execution stay out. NLO-0.4 owns execution of shared stops and legs.

| Stage | Content | Depends on |
|-------|---------|------------|
| NLO-0.1 | Capacity and load foundation: manual capacity, explicit load publication, tenant projection, hard feasibility, persisted candidates and explanations | Gateway tenant model, order/cargo/location reads |
| NLO-0.2 | Predictive next load: rule-based `PredictedCapacity` from shipment plus tracking ETA, top N, no ML | NLO-0.1, tracking ETA read |
| NLO-0.3 | Current-trip residual context, pairwise same-origin/same-destination feasibility, and one additional current-trip load. Planning only | NLO-0.2, B2 groupage, explicit publication |
| NLO-0.4 | Persistent route execution: multi-stop shipment, shared legs, accepted plan activation, driver multi-stop tasks | NLO-0.3 planning proof, ADR-NET-005 |
| NLO-0.5 | Backhaul and roundtrip with corridor search and road distance | Routing provider port |
| NLO-0.6 | Regional routing, open and closed | NLO-0.4 plan model |
| NLO-0.7 | Urban Moscow profile populated from sourced rules, not from solver branches | city-rules data ownership |
| NLO-0.8 | Urban Saint Petersburg as a second profile | NLO-0.7 profile mechanism |
| NLO-0.9 | Chain optimizer and `AT_RISK` reoptimization | NLO-0.2 and NLO-0.5 |
| NLO-1.0 | Network optimizer: measured large jobs, network value beyond the zero fallback | Benchmarks from earlier stages |
| FLEET-1.x | Private fleet objectives and explicit vehicle, driver, depot assignment on the same core | NLO-0.1 core, fleet masters |

NLO-0.7 and NLO-0.8 are empty of legal constants until operations load versioned rules with sources. The architecture for both profiles is already defined.

## Recommended next implementation

NLO-0.1 and NLO-0.2 are already on main. The next product work, only after a separate authorization, is NLO-0.3B then NLO-0.3C and NLO-0.3D. See [NLO_0_3_IMPLEMENTATION_ROADMAP.md](NLO_0_3_IMPLEMENTATION_ROADMAP.md).

Why that bound: `transport.shipments` still has one origin, one destination, one optional transport order, and one optional cargo. There is no stop or route-leg table. `PredictedCapacity` copies vehicle payload and volume as capacity after unload. It is not residual space during the trip. B2 `EvaluateGroupage` already evaluates a cargo set. The first consolidation waves reuse that evaluator for a pair or for one confirmed onboard projection plus one published load. They do not enumerate `2^N` subsets and they do not activate a shipment.

Dependencies: trusted tenant headers; read APIs for shipment, tracking ETA, location, cargo, vehicle; explicit publication; optional rate snapshot for priced ranking. Kafka is not required for the first synchronous path. Events in the catalog are the later integration. The document registry may be read later for a document constraint. The EDO operator runtime is not an implementation dependency for NLO-0.1 or NLO-0.2.

Planning contracts for multi-stop routes may exist from NLO-0.4. Production multi-stop execution stays gated until the execution domain supports stops and legs.

Risks: builders may treat Haversine as road distance; a query might scan shipments across tenants; multi-stop UI might be promised before execution can store stops; missing pallet data might be treated as zero.
