# NLO-0.3C current trip context

Status: IMPLEMENTED_IN_BRANCH / UNDER_REVIEW. This document describes the branch. It is not a merge closeout. NLO-0.3D is not started. NLO-0.3 is not complete.

## Authoritative evidence

shipment-service owns `transport.shipment_cargo_execution_evidence`. The table is append-only. A row is the unit execution fact. There is no mutable onboard boolean.

The current producer writes evidence only when an assigned authenticated driver completes a trusted operation and the shipment already has `cargo_id`:

- `PICKUP_COMPLETED` appends `CONFIRMED_ONBOARD`
- `DELIVERY_COMPLETED` appends `UNLOADED`

The cargo id is read from `transport.shipments.cargo_id` inside the status transaction. The driver request body has no cargo id. `state_version` and `shipment_version` are the shipment version produced by that transition. A replay of the same driver idempotency key does not insert another row. The unique key is `(shipment_id, cargo_id, state_version)`.

If the evidence insert fails, the shipment status transition rolls back with it. Status history and the shipment outbox stay in that same transaction.

`PLANNED` and `PICKED_UP` are valid states for a later producer. NLO-0.3C does not write them.

## No historical backfill

Existing `LOADED` and `IN_TRANSIT` shipments are not copied into `CONFIRMED_ONBOARD`. Shipment status is shipment-wide and does not prove a unit is onboard. A shipment with no evidence row stays `ONBOARD_CARGO_UNPROVEN`. Residual occupancy for that shipment is `UNKNOWN`, not zero.

A manual status change to `LOADED`, `IN_TRANSIT`, or `DELIVERED` does not write evidence. A shipment with `cargo_id` null can still change status through the existing driver path, and no cargo row is invented.

`UNLOADED` does not delete the earlier `CONFIRMED_ONBOARD` row. The latest row for a cargo, ordered by `state_version`, `occurred_at`, then `id`, is the current unit state. `UNLOADED` means the cargo is explicitly not onboard. No row means the state is unproven.

## Ownership

| Fact | Owner | Contract |
| --- | --- | --- |
| Shipment execution | shipment-service | `GET /internal/v1/shipments/{shipmentId}/execution-context` |
| Onboard evidence | shipment-service | `GET /internal/v1/shipments/{shipmentId}/onboard-cargo` |
| Cargo planning profile | transport-order-service | `GET /internal/v1/cargoes/{id}/planning-profile`, including `version` |
| Vehicle capability | shipment-service | existing `GET /internal/v1/vehicles/{id}/capability` |
| Position freshness and ETA freshness | tracking-service | existing state and ETA lookups |
| `CurrentTripContext` and `ResidualCapacitySnapshot` | network-optimizer-service | assembled from those reads |

BNO does not query `transport.shipments`, `transport.cargoes`, `transport.vehicles`, or tracking tables. It is not the system of record for whether cargo is physically onboard.

Internal shipment and cargo routes stay behind `X-Internal-Service-Token` and trusted `X-Tenant-ID`. A foreign id is `404`. These routes are not added to the API gateway. There is no public endpoint that accepts a caller-built context or residual snapshot.

Nullable cargo and vehicle facts stay null. Null weight, volume, pallet count, linear metres, or bool is not coerced to zero or false. Present vehicle database fields are `ASSET_CONFIRMED`. Missing fields stay `UNKNOWN`. A catalog nominal default is not a measured total.

## Residual formula

For payload, volume, pallet positions, and linear metres:

`remaining = vehicle total - sum(CONFIRMED_ONBOARD occupancy)`

only when the vehicle total is known and every required occupancy fact for that dimension is known. Otherwise `status=UNKNOWN` and `remaining` is empty. Known cargoes are not partially summed into an authoritative remainder.

Explicit `UNLOADED` contributes zero occupancy. That is different from missing evidence.

Pallet conversion uses only an explicit positive equivalence whose ownership is proven. EUR, EUR2, FIN, and US factors are not hardcoded. If ownership cannot be proven, pallet residual is `UNKNOWN`. Linear metres are not derived from pallets, weight, or volume.

Height is not subtractive. The snapshot compares vehicle `internal_height_mm` with the maximum confirmed onboard cargo height and reports `KNOWN_OK`, `KNOWN_EXCEEDED`, or `UNKNOWN`. It does not claim a remaining height and it does not place cargo in 3D.

If known occupancy exceeds the vehicle total, the dimension status is `CURRENT_OCCUPANCY_EXCEEDS_CAPACITY`. The remainder is not clamped to zero. That context is not safe for a later additional-load decision.

Temperature, ADR, food, odor, and contamination are carried facts. They are not capacity subtraction. More than one independently controlled zone is `MULTI_ZONE_ALLOCATION_REQUIRED` and is not assigned by this wave. Tracking freshness is stored as returned: `FRESH`, `STALE`, `LOST`, or `UNKNOWN`. BNO does not apply 10/30 or 15/60 minute thresholds. The NLO-0.2 predictor still reduces ETA to arrival and observed time for `PredictedCapacity`. Current-trip ETA keeps status, freshness, and source time. `PredictedCapacity` remains future empty capacity, not in-trip residual.

## Fingerprint

`CurrentTripContext` is an immutable value built on read. It is not stored in a new table. Migration `000082` is the evidence table. The input fingerprint covers shipment id, version, and status; vehicle id and version; ordered evidence state and cargo profile version; tracking freshness and recorded time; ETA status, freshness, and observed time; and residual status. It does not include `BuiltAt`, a random id, or a request id.

## Still off

`POST /v1/network/consolidation/search` with `pattern=CURRENT_TRIP_FILL` returns `PATTERN_NOT_IMPLEMENTED`. This wave does not search for another load, insert a route, assign, reserve, or offer capacity. There is no solver, MILP, CP-SAT, VRP, ML, or 3D packing.
