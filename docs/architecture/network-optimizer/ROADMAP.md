# Roadmap

Each stage uses the same core. Later stages do not replace earlier contracts; they add problem classes. Implementation of any stage needs a new authorization.

BNO-0.1A implements the foundation subset of NLO-0.1: manual capacity, explicit load publication, visibility, persistence, API, outbox, and audit. It does not implement hard feasibility, persisted match candidates, or explanations.

BNO-0.1B is IMPLEMENTED: rule-based `PredictedCapacity`, normalized body and combination capability, independent loading and unloading access, temperature capability and cargo requirements, and compatibility primitives.

BNO-0.1B2 is IMPLEMENTED: versioned cargo, equipment, pallet, and packaging catalogs, tenant compatibility rule sets, and explainable cargo-equipment and groupage evaluation. It does not implement next-load search, ranking, or a physical trailer asset master.

BNO-0.1C0 is IMPLEMENTED: location snapshots, search and display geography, the routing port, and search-policy primitives. It does not search loads.

BNO-0.1C1 is IMPLEMENTED in this line of work: a carrier-owned available capacity can search visible published loads and persist hard-feasible match candidates. Match score, weighted ranking, and top N are not implemented. BNO-0.1C is not complete.

NLO-0.2 is PARTIAL. The predicted-capacity half is in BNO-0.1B. Top-N next-load matching is not implemented.

NLO-0.3 remains NOT IMPLEMENTED.

| Stage | Content | Depends on |
|-------|---------|------------|
| NLO-0.1 | Capacity and load foundation: manual capacity, explicit load publication, tenant projection, hard feasibility, persisted candidates and explanations | Gateway tenant model, order/cargo/location reads |
| NLO-0.2 | Predictive next load: rule-based `PredictedCapacity` from shipment plus tracking ETA, top N, no ML | NLO-0.1, tracking ETA read |
| NLO-0.3 | Current-trip fill and consolidation patterns that current cargo attributes can prove | NLO-0.1, residual capacity |
| NLO-0.4 | Multi-stop plans. Execution still blocked until shipment can represent stops | NLO-0.3, ADR-NET-005 follow-up |
| NLO-0.5 | Backhaul and roundtrip with corridor search and road distance | Routing provider port |
| NLO-0.6 | Regional routing, open and closed | NLO-0.4 plan model |
| NLO-0.7 | Urban Moscow profile populated from sourced rules, not from solver branches | city-rules data ownership |
| NLO-0.8 | Urban Saint Petersburg as a second profile | NLO-0.7 profile mechanism |
| NLO-0.9 | Chain optimizer and `AT_RISK` reoptimization | NLO-0.2 and NLO-0.5 |
| NLO-1.0 | Network optimizer: measured large jobs, network value beyond the zero fallback | Benchmarks from earlier stages |
| FLEET-1.x | Private fleet objectives and explicit vehicle, driver, depot assignment on the same core | NLO-0.1 core, fleet masters |

NLO-0.7 and NLO-0.8 are empty of legal constants until operations load versioned rules with sources. The architecture for both profiles is already defined.

## Recommended first implementation

`NLO-0.1` + `NLO-0.2` only.

Why: the repository can already supply one active shipment, an ETA, a location, cargo weight and volume, and a vehicle capacity. It cannot yet supply legs, trailers, pallets, a road network, or a legal city-rule dataset. Next-load ranking with hard feasibility and explanations proves the core without a VRP solver.

Dependencies: trusted tenant headers; read APIs for shipment, tracking ETA, location, cargo, vehicle; explicit publication; optional rate snapshot for priced ranking. Kafka is not required for the first synchronous path. Events in the catalog are the later integration. The document registry may be read later for a document constraint. The EDO operator runtime is not an implementation dependency for NLO-0.1 or NLO-0.2.

Planning contracts for multi-stop routes may exist from NLO-0.4. Production multi-stop execution stays gated until the execution domain supports stops and legs.

Risks: builders may treat Haversine as road distance; a query might scan shipments across tenants; multi-stop UI might be promised before execution can store stops; missing pallet data might be treated as zero.
