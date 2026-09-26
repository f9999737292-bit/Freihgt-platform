# NLO-0.3 current trip context

Baseline: `origin/main` `6d47cc92`.

## Semantic split

| Concept | Meaning | Runtime today |
| --- | --- | --- |
| `FUTURE_EMPTY_CAPACITY` | Space after the current shipment unloads | `PredictedCapacity`. Vehicle weight and volume copied in `predict/rule.go` |
| `CURRENT_TRIP_RESIDUAL_CAPACITY` | Space left while confirmed cargo is still on the vehicle | NOT stored. Not a `Capacity.source` |

ADR-NET-013 freezes `CurrentTripContext` as the planning aggregate and `ResidualCapacitySnapshot` as a child of that context. Do not add `CURRENT_TRIP_RESIDUAL` as a capacity source.

## CurrentTripContext fields

| Field | Required to plan | Owner today | If missing |
| --- | --- | --- | --- |
| `shipment_id`, `shipment_version`, `shipment_status` | YES | shipment-service | context cannot be built |
| `vehicle_id`, `vehicle_version` | YES | shipment-service | context cannot be built |
| `origin_location_id`, `destination_location_id` | YES | shipment columns | context cannot be built |
| current position and `observed_at` | YES for current-trip fill | tracking position, not read by BNO | `INDETERMINATE` for insertion; do not use the destination as "here" |
| next operational point | the shipment destination, because there are no stops | shipment | GAP if a real intermediate stop is required |
| ETA and `eta_observed_at` | YES when a window depends on arrival | tracking ETA lookup | time-window result `INDETERMINATE` |
| onboard cargo references and evidence state | YES | GAP. Status `LOADED` is not per-cargo proof | residual `UNKNOWN` |
| equipment capability used for the trip | YES | vehicle columns, NULL means unknown | compatibility `INDETERMINATE` where the fact is required |
| temperature setpoint | only when cargo requires temperature | vehicle mode plus cargo interval | do not invent a setpoint |
| capacity totals | YES per dimension that will be checked | vehicle | that residual dimension is `UNKNOWN` |
| occupied and residual snapshots | YES | derived only from confirmed onboard facts | see residual model |
| planned unload state | the single destination unload | shipment status | no intermediate unload sequence exists |

Fields with no owner stay GAP. The context does not invent them.

## Onboard evidence

Allowed evidence states for a cargo unit inside the context:

| State | Meaning |
| --- | --- |
| `PLANNED` | On the order or shipment cargo link. Not physical proof |
| `PICKED_UP` | A sourced pickup event names this cargo unit. Not yet frozen as a current event |
| `CONFIRMED_ONBOARD` | An execution fact names this unit as loaded and not yet unloaded |
| `UNLOADED` | Excluded from occupancy |

Today the shipment status list includes `LOADED` and `IN_TRANSIT`, and `cargo_items.quantity` is a planned quantity. Neither is `CONFIRMED_ONBOARD`. Until an execution owner emits a unit-level loaded fact, current-trip residual occupancy is `UNKNOWN` and current-trip fill cannot become `FEASIBLE`. That gap is NLO-0.3B's blocking read, not a reason to infer onboard cargo from the order.

## Freshness

Tracking-service already has policies. NLO-0.3 reuses them. It does not invent new legal minutes.

`LOCATION_FRESHNESS_POLICY` is `tracking-service/internal/domain/freshness.go`: fresh at most 10 minutes, stale at most 30, then lost. Null `recorded_at` is unknown.

`ETA_FRESHNESS_POLICY` is `tracking-service/internal/domain/eta_freshness.go`: fresh at most 15 minutes, stale at most 60, then expired. Null observation is unknown.

Current-trip fill treats location `stale`, `lost`, or `unknown` as not fresh enough to insert a stop. The result is `INDETERMINATE` with reason `LOCATION_STALE` or `LOCATION_UNKNOWN`, not a hard reject of the cargo and not a silent pass. ETA `stale`, `expired`, or `unknown` makes time-window feasibility `INDETERMINATE`.

BNO prediction keeps `BNO_PREDICTION_MAX_ETA_AGE` for future-empty capacity. Current-trip fill does not reuse that environment variable as the GPS policy.

## Route insertion, planning only

A one-load insertion, when position and windows are fresh, may be evaluated as:

`CURRENT_POSITION` → additional pickup → existing destination → additional delivery

or, when the additional load shares the existing destination and the pickup is still ahead:

`CURRENT_POSITION` → additional pickup → existing destination

Road kilometres and seconds come from the routing port. Haversine may prefilter. It is not road distance. The sequence is a planning assumption on the candidate. It does not write stops onto the shipment.

## Slots and city rules

Tracking owns `shipment_slot_state`. NLO does not book. A stop that requires a slot and has none is `SLOT_UNKNOWN` or `SLOT_REQUIRED`, which blocks `EXECUTABLE` and leaves the planning result `INDETERMINATE` or `PLAN_ONLY`. `SLOT_CONFIRMED` requires a tracking state that names the window. `SLOT_UNCONFIRMED` is a known window that tracking has not confirmed.

City access restrictions, if an executable urban plan needs them, go through a future `CityRulesProvider`. Unknown or stale city rules fail closed for an executable urban plan. This phase hardcodes no Moscow or Saint Petersburg constants.
