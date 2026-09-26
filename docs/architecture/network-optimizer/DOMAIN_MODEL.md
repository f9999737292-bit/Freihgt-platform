# Domain model

Names below are planning concepts. Where the repository already has an aggregate, this model extends it by projection. It does not rename public APIs.

## 7.1 CargoUnit

Smallest physical object the consolidation engine may co-load. It is a **planning projection** of `transport.cargoes` / `transport.cargo_items`, plus attributes the current cargo aggregate does not store.

| Field | v0.1 | Source today |
|-------|------|----------------|
| `cargo_unit_id` | required | new id; maps to cargo and/or cargo item |
| `tenant_id` | required | cargo tenant |
| `transport_order_id` | required | owning order |
| `weight` | required when known | `gross_weight` or item weight |
| `volume` | required when known | `volume` |
| `pallet_count` | optional | nullable `transport.cargoes.pallet_count`; unknown stays null |
| `linear_meters` | optional | nullable `transport.cargoes.linear_meters` |
| `dimensions` | partial | `max_loaded_height_mm` only; no 3D packing |
| `stackable` | optional | nullable cargo fact; stacking does not invent positions |
| `fragile` | optional | nullable cargo fact; no automatic reject |
| `cargo_type` | required | free-text `cargo_type` plus optional `cargo_type_code` |
| `commodity` | optional | description / item name |
| `temperature_min` / `temperature_max` | optional | cargo fields |
| `adr_class` | optional | item `hazard_class`; cargo has `dangerous_goods_flag` |
| `loading_method` / `unloading_method` | partial | required and allowed access lists on the load projection; no separate method master |
| `can_co_load` | not a runtime column | NLO-0.3A replaces this name. See the opt-in fields below |
| `pickup_location` / `delivery_location` | required | order origin/destination today (single pair) |
| `pickup_window` / `delivery_window` | required | requested/planned timestamps; not a full window model |

Unknown physical attributes fail closed for constraints that need them. Missing pallet counts and linear metres stay `UNKNOWN`. Never coerce an unknown value to zero. The match is `INDETERMINATE` for that constraint unless policy explicitly allows weight/volume-only feasibility.

NLO-0.3A correction, verified on `origin/main` `6d47cc92`: `transport.cargoes` gained nullable `pallet_count`, `pallet_type_code`, `linear_meters`, height, stackable, fragile, food-grade, odor, and contamination columns in migration `000077`. Those columns describe the cargo record. They do not prove the cargo is physically onboard, and they are not residual vehicle capacity. `can_co_load` is not a database column. Cross-shipper consolidation requires explicit `cross_shipper_consolidation_allowed=false` by default on the published load. Same-owner consolidation requires explicit `consolidation_allowed=false` by default. Details are in [NLO_0_3_PRIVACY_TENANCY.md](NLO_0_3_PRIVACY_TENANCY.md) and [ADR-NET-015](adr/ADR-NET-015-cross-shipper-opt-in-privacy.md).

### CargoCompatibility and CargoIncompatibility

Separate from score.

- `CargoCompatibility` records allowed co-load pairs or classes (same temperature band, shipper opt-in).
- `CargoIncompatibility` is a hard rule: ADR class conflict, temperature conflict, shipper denial, fragile vs heavy, documented customer exclusion.

A violated incompatibility is `HARD_REJECT`, not a penalty. See [CONSOLIDATION_MODEL.md](CONSOLIDATION_MODEL.md). BNO-0.1B2 rules also distinguish a `SOFT` deny, which is a warning, and `REQUIRE_SEPARATION` or `REQUIRE_CONDITION`, which stay indeterminate until a later allocator. That behavior is specified in [BNO_0_1B2_REFERENCE_COMPATIBILITY.md](BNO_0_1B2_REFERENCE_COMPATIBILITY.md). It does not add a consolidation solver.

## Load opportunity

Safe marketplace representation. **Not** another tenant's shipment row.

```text
load_opportunity_id
owner_tenant_id          # internal; not shown to other shippers
publication_scope        # PRIVATE | INVITED_CARRIERS | MARKETPLACE | ANONYMIZED_MARKETPLACE | NETWORK_OPTIMIZATION_ONLY
pickup / pickup_window
delivery / delivery_window
weight / volume / pallets / linear_meters
body_type
equipment_requirements
cargo_requirements       # temperature, ADR, loading — no free-text customer PII
offered_rate OR commercial_mode
required_carrier_attributes
status
source_transport_order_id   # internal reference, stripped from foreign views
```

| Viewer | Sees |
|--------|------|
| Matching engine | Fields allowed by scope, plus internal owner keys inside the platform trust zone |
| Carrier in scope | Published operational and commercial fields for that scope |
| Another shipper | Nothing, unless a later consolidation share policy explicitly releases operational facts |
| Owning shipper | Full own opportunity, not other shippers' rates |

`NETWORK_OPTIMIZATION_ONLY` is visible to the optimizer and to the owning tenant. It is not a carrier advertisement.

Publication is explicit. Creating a transport order does not publish a load.

## Capacity

First-class time-bounded resource. A vehicle master row is not capacity.

```text
capacity_id
owner_tenant_id
carrier_id
vehicle_id          # optional in marketplace until assignment
trailer_id          # optional; no trailer master today
driver_id           # optional in marketplace; required for private-fleet execution
location
available_from / available_until
source              # see Capacity source
confidence          # 0..1
body_type / equipment
payload_remaining / volume_remaining
pallet_positions_remaining / linear_meters_remaining
temperature_capability / adr_capability
preferred_destinations / excluded_destinations
max_deadhead / max_wait
home_base
commercial_preferences
publication_scope   # PRIVATE | SHIPPER_NETWORK | MARKETPLACE | ANONYMIZED
status
version
```

### Capacity source

`MANUAL`, `CURRENT_SHIPMENT_PREDICTION`, `TELEMATICS`, `DRIVER_APP`, `CARRIER_TMS`, `API`, `FLEET_PLAN`.

The architecture must not assume the carrier typed the empty truck in by hand. v0.1 implementation may start with `MANUAL` and `CURRENT_SHIPMENT_PREDICTION` only.

### PredictedCapacity

```text
predicted_capacity_id
capacity_id
vehicle_label            # display only; plate visibility follows the privacy matrix
predicted_location
available_from
confidence
availability_window      # e.g. 16:10–17:15
prediction_method        # RULE_BASED | STATISTICAL | ML | EXTERNAL
input_snapshot_ids       # shipment, ETA, slot, dwell assumption
generated_at
```

Example (illustrative, not a live record):

```text
vehicle A123BC
predicted_location: Kazan
available_from: 2026-09-24 16:35
confidence: 0.87
availability_window: 16:10–17:15
```

Inputs the rule-based provider may use when present: current shipment ETA, remaining distance, driver events, slot window, expected unload duration, facility dwell history, telematics. This freeze does not implement ML. Missing ETA means the provider returns no prediction above the policy confidence floor.

## Vehicle capacity model

`payload = 20t` is not sufficient. The domain reserves:

```text
payload_kg
volume_m3
pallet_positions
linear_meters
internal_dimensions
body_type
rear_loading / side_loading / top_loading
temperature_zones
adr_capability
floor_constraints
vehicle_height / vehicle_width / vehicle_length
gross_weight_class
```

Only `capacity_weight` and `capacity_volume` exist on `transport.vehicles` today. The model must not block the other fields. v0.1 feasibility uses the fields that are actually populated and marks the rest `UNKNOWN`.

## Route and movement

Today: `TransportOrder` → single `Shipment` with one origin and one destination. That compatibility stays.

Target planning shape:

```text
TransportOrder
    ↓
CargoUnit
    ↓
RoutePlan
    ├── RouteLeg     # shareable by many cargo units
    └── Stop         # PickupStop, DeliveryStop, HubStop, DepotStop, BreakStop
```

Several orders may share one linehaul leg:

```text
Order A ─┐
Order B ─┼── RouteLeg
Order C ─┘
```

`RoutePlan` is optimizer source of truth for a **proposal**. It does not replace `Shipment`. Accepted single-leg OD plans later use the existing shipment creation path. Multi-order shared legs stay `PROPOSED` until an authorized execution model exists. See ADR-NET-005.

### Stop

```text
sequence
location
arrival_window
service_duration
cargo_actions      # load / unload which cargo units
constraints
stop_kind          # PICKUP | DELIVERY | HUB | DEPOT | BREAK
```

### ChainPlan

Ordered list of route plans or legs for one capacity over a horizon, with slack between legs.

## Hub and cross-dock

The plan model can represent a future path without executing it:

```text
Origin → Pickup → Origin Hub → Consolidation → Linehaul → Destination Hub → Deconsolidation → Urban Delivery
```

`Hub`, `CrossDock`, and `Depot` are roles of a location (`HubStop`, dwell, cargo actions). They are not new masters and not a second slot service. A hub stop may require a slot query to tracking. Automatic cross-dock execution is out of the first implementation. Discovery did not find a facility table; `facility_id` on slot rows is only a UUID.

## Match and decision records

- `MatchCandidate` — feasible or rejected pair/set, with hard-reject reasons.
- `MatchScore` — deterministic components plus `score_explanation`.
- `ConsolidationCandidate` — set of cargo units that passed hard constraints.
- `CarrierOffer` — commercial proposal to a carrier. Not an RFx response line.
- `OptimizationJob` — one run, inputs, profile, status.
- `OptimizationDecision` — immutable audit snapshot of eligibility, rejections, scores, rule versions, rate snapshot ids, route assumptions, capacity snapshot, actor.

## Facility roles (future)

`Facility`, `Hub`, `CrossDock`, and `Depot` are roles. Today the closest data is `transport.locations.location_type` plus an unbound `facility_id` on slots. Do not pretend a facility master exists. A later attribute `facility_role` may classify a location. Slot booking stays owned by tracking. A stop may **require** a slot; it does not book one inside the optimizer.

## Commercial modes on an opportunity

`DIRECT_ACCEPT`, `FIXED_PRICE_OFFER`, `COUNTER_OFFER` stay on `CarrierOffer`. `MINI_TENDER`, `RFQ`, and `AUCTION` are requests the optimizer hands to `rfx-service`. See ADR-NET-007 and [RATE_OWNERSHIP_MATRIX.md](RATE_OWNERSHIP_MATRIX.md).
