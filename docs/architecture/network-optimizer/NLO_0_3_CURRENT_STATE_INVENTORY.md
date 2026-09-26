# NLO-0.3 current-state inventory

Baseline: `origin/main` `6d47cc92aa825abdd34ee223791ae13c83cf4b01`. Migration head is `000080_bno_match_score_topn_v0_1c2`. This document cites schema and code. It does not treat an architecture sentence as a runtime fact.

## Accepted BNO status

| Stage | Status | Evidence |
| --- | --- | --- |
| BNO-0.1A | CLOSED | Capacity and load publication, migration `000075` |
| BNO-0.1B | CLOSED | `PredictedCapacity`, migration `000076` |
| BNO-0.1B2 | CLOSED | Catalogs, rules, `EvaluateGroupage`, migration `000077` |
| BNO-0.1C0 | CLOSED | Location snapshots and routing port, migration `000078` |
| BNO-0.1C1 | CLOSED | Next-load hard search, migration `000079` |
| BNO-0.1C2 | CLOSED | Match score and top N, migration `000080`, merge `6d47cc92` |
| BNO-0.1C | CLOSED | C0, C1, and C2 are on main |

NLO-0.2 is IMPLEMENTED as the predictive next-load path. `predict.Evaluate` builds future availability from shipment destination, vehicle totals, and a tracking ETA. Search and top N consume that capacity as one vehicle plus one load. The open product gap is current-trip residual capacity, which is NLO-0.3, not a missing half of NLO-0.2.

Finding: prediction calls `POST /internal/v1/tracking/eta/lookup` through `sourceclient.HTTP.ETA`. It rejects the draft when `BNO_PREDICTION_MAX_ETA_AGE` is zero or the observation is older than that duration (`predict/rule.go`). GPS is not an NLO-0.2 input. Tracking-service defaults are 10/30 minutes for location and 15/60 minutes for ETA. Those numbers are defaults only. Runtime config is `TRACKING_FRESH_THRESHOLD_MINUTES`, `TRACKING_STALE_THRESHOLD_MINUTES`, `ETA_FRESH_THRESHOLD_MINUTES`, and `ETA_STALE_THRESHOLD_MINUTES`. NLO-0.3 consumes the returned freshness status and does not copy the thresholds.

## Shipment execution model

`transport.shipments` in `000003_create_transport_tables.up.sql` has exactly one `origin_location_id` and one `destination_location_id`, both `NOT NULL`. It has one nullable `transport_order_id` and one nullable `cargo_id`. There is no `transport.shipment_stops` table and no `transport.route_legs` table. Later migrations through `000080` do not add them.

Answers:

| Question | Answer |
| --- | --- |
| One origin and one destination | YES |
| Stop rows | NO |
| RouteLeg rows in the execution schema | NO. ADR-NET-005 names them as a future planning model only |
| Several transport orders on one shipment | NO. One nullable foreign key |
| Several independent cargo sets on one shipment | NO. One nullable `cargo_id` |
| Quantity physically onboard | NO. `cargo_items.quantity` is the item quantity on the cargo record. Shipment status `LOADED` is a shipment status, not a per-unit onboard proof |
| Residual payload, volume, pallet positions, linear metres | NO columns |
| Loading or unloading sequence | NO |
| Temperature zone assignment | NO |
| ADR segregation assignment | NO |

`ShipmentEligible` treats `LOADED` and `IN_TRANSIT` as eligible for future-empty prediction. That eligibility means "this shipment may produce a later empty capacity." It does not mean "this cargo is confirmed onboard for residual math."

No existing driver or shipment event qualifies as unit-level onboard evidence. `NEW_EXECUTION_EVIDENCE_REQUIRED=YES`. The future owner is shipment-service. Network-optimizer-service only consumes `ShipmentOnboardCargoProvider`. Only `CONFIRMED_ONBOARD` counts toward residual occupancy.

## Ownership matrix

| Fact | Owner | Source | Tenant boundary | Available | Freshness | BNO can read today | Gap |
| --- | --- | --- | --- | --- | --- | --- | --- |
| TransportOrder | transport-order-service | `transport.transport_orders` | `tenant_id` | YES | row version | ownership check only | no cargo-set read for consolidation |
| Cargo | shipment-service schema `transport.cargoes` | `000003` plus `000077` nullable planning columns | `tenant_id` | PARTIAL | row version | not via prediction input | prediction query omits cargo |
| CargoItem | same cargo aggregate | `transport.cargo_items` | through parent cargo | PARTIAL | no version column | NO | quantity is not onboard evidence |
| Shipment | shipment-service | `transport.shipments` | `tenant_id` on prediction input | YES for one O-D | `version` | YES, prediction input | no stops |
| Vehicle | shipment-service | `transport.vehicles` | `tenant_id` | YES weight/volume; PARTIAL pallets and linear metres | `version` | YES via `/internal/v1/vehicles/{id}/capability` | NULL capability stays unknown |
| PredictedCapacity | network-optimizer-service | `network_optimizer` prediction rows | owner tenant | YES | ETA observed_at plus BNO max age | YES, own tenant | future empty, not residual |
| Capacity | network-optimizer-service | `network_optimizer.capacities` | owner tenant | YES | capacity version | YES | manual or predicted empty |
| LoadOpportunity | network-optimizer-service | `network_optimizer.load_opportunities` | owner tenant; marketplace filter | YES | load version | YES for published scopes | opt-in booleans do not exist yet |
| Location | transport `locations`; BNO snapshots | location id on order/shipment; BNO snapshot | tenant on location join | YES | snapshot time in BNO | YES for search geography | not live GPS |
| Tracking ETA | tracking-service | `tracking.shipment_eta_state` | shipment binding | YES | `EvaluateETAFreshness` | YES via internal lookup | separate from BNO max age |
| Driver GPS | shipment-service event `driver.location.updated`; tracking ingest | tracking position state | shipment/driver tenant | YES as tracking state | status from tracking-service; defaults 10/30, runtime-configurable | NO dedicated BNO port | current-trip wave consumes status, not copied minutes |
| Slots | tracking-service | `tracking.shipment_slot_state` | shipment binding | PARTIAL | source observed_at | NO from BNO | NLO must not book |
| Compatibility catalogs and rules | network-optimizer-service | `000077` | platform plus tenant rule sets | YES | catalog and rule-set versions | YES | active set required |
| Pallet equivalences | network-optimizer-service | `network_optimizer.pallet_equivalences` | catalog version | YES when seeded | catalog version | YES inside `EvaluateGroupage` | only explicit positive factors |
| Routing provider | network-optimizer-service port | 2GIS when configured, else unconfigured | not a tenant table | PARTIAL | request time | YES for next-load road facts | no city-rule dataset |

## Human marketplace and internal scope

`ListMarketplaceLoads` returns `PUBLISHED` loads whose scope is `MARKETPLACE`, `ANONYMIZED_MARKETPLACE`, or `INVITED_CARRIERS` for the viewer's company. `PRIVATE` and `NETWORK_OPTIMIZATION_ONLY` are not in that predicate (`repository/postgres.go`). Next-load search calls that list (`service/search.go` `visibleLoads`). An internal consolidation pool must be a separate query. It must not widen the human marketplace.

## Groupage already in runtime

`POST /v1/network/compatibility/groupage/evaluate` calls `compat.EvaluateGroupage`. That function checks every cargo against equipment, every unordered cargo pair, aggregate capacity, temperature, and ADR, then resolves one status. NLO-0.3 reuses this function. It does not add a second engine.

## What is not in the repository

No residual capacity table. No onboard evidence state machine. No solver. No 3D packer. No EDO readiness proof inside BNO. Pairwise opt-in, search, and migration `000081` are the NLO-0.3B branch, not this frozen inventory.
