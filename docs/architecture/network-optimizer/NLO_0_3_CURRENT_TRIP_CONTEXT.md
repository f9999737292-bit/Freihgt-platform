# NLO-0.3 current trip context

Baseline: `origin/main` `6d47cc92`.

## Semantic split

| Concept | Meaning | Runtime today |
| --- | --- | --- |
| `FUTURE_EMPTY_CAPACITY` | Space after the current shipment unloads | `PredictedCapacity`. Vehicle weight and volume copied in `predict/rule.go` |
| `CURRENT_TRIP_RESIDUAL_CAPACITY` | Space left while confirmed cargo is still on the vehicle | NOT stored. Not a `Capacity.source` |

ADR-NET-013 records `CurrentTripContext` as the planning aggregate and `ResidualCapacitySnapshot` as a child of that context. Do not add `CURRENT_TRIP_RESIDUAL` as a capacity source. This package is FROZEN_ACCEPTED. Implementation is not authorized.

## Server-built context

`CurrentTripContextProvider` runs inside network-optimizer-service. A public request does not upload the context.

For `CURRENT_TRIP_FILL` the caller may send `shipment_id`, `pattern`, `policy`, and `candidate_limit`. The caller must not send residual payload, residual volume, the onboard cargo list, a tracking position, an ETA, vehicle totals, or temperature state. A `current_trip_context_id` is allowed only when the server already created that context.

The provider takes the trusted tenant from the gateway or internal service context and the owned shipment id. It reads `ShipmentExecutionProvider`, `ShipmentOnboardCargoProvider`, `VehicleCapabilityProvider`, `TrackingPositionProvider`, and `TrackingETAProvider`. It does not accept a browser tenant header and it does not scan another tenant.

Before assembly, the shipment must belong to the requesting capacity-owning tenant. A foreign shipment id is `NOT_FOUND`. The internal context binds `tenant_id`, `shipment_id`, `shipment_version`, `vehicle_id`, and `vehicle_version`. Public responses omit `tenant_id`.

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

Today the shipment status list includes `LOADED` and `IN_TRANSIT`, and `cargo_items.quantity` is a planned quantity. A cargo linked to the shipment is not onboard proof. Driver events in `shipment-service/internal/domain/driver_events.go` (`driver.location.updated`, `driver.arrived_at_pickup`, `driver.departed_pickup`, `driver.arrived_at_delivery`, `driver.delivery.completed`) name a shipment, not a cargo unit. None of them is unit-level `CONFIRMED_ONBOARD`.

`NEW_EXECUTION_EVIDENCE_REQUIRED=YES`. The owner of that evidence is shipment-service, the execution domain. Network-optimizer-service consumes it and is not the system of record for load or unload.

The future internal read is `ShipmentOnboardCargoProvider`. Input is the trusted tenant, `shipment_id`, and `shipment_version`. Output is cargo units with `cargo_id` or `cargo_unit_id`, cargo version, state, state version, observed or occurred time, source, and quantity facts. States are `PLANNED`, `PICKED_UP`, `CONFIRMED_ONBOARD`, and `UNLOADED`. Only `CONFIRMED_ONBOARD` enters authoritative residual occupancy. `PLANNED` does not. Until that read exists, current-trip fill cannot become `FEASIBLE`. That blocks NLO-0.3C and NLO-0.3D, not the pairwise first wave.

## Freshness

Tracking-service owns freshness. BNO consumes the status it returns. BNO does not copy thresholds and does not decide `age <= 10` or `age <= 30`.

Defaults only, from `domain/freshness.go` and `domain/eta_freshness.go`, overridden at process start by `services/tracking-service/internal/config/config.go`:

| Policy | Default | Runtime configuration |
| --- | --- | --- |
| Location fresh / stale | 10 / 30 minutes | `TRACKING_FRESH_THRESHOLD_MINUTES`, `TRACKING_STALE_THRESHOLD_MINUTES` |
| ETA fresh / stale | 15 / 60 minutes | `ETA_FRESH_THRESHOLD_MINUTES`, `ETA_STALE_THRESHOLD_MINUTES` |

`DEFAULTS_ONLY`. `RUNTIME_CONFIGURABLE`. `TRACKING_SERVICE_IS_POLICY_OWNER`. `BNO_CONSUMES_STATUS`.

The position lookup already returns `freshness.status`, `freshness.ageSeconds`, and `lastKnownPosition.recordedAt`. For current-trip fill:

| Returned location status | BNO result |
| --- | --- |
| `FRESH` | the position may be used |
| `STALE` | `INDETERMINATE` / `LOCATION_STALE` |
| `LOST` | `INDETERMINATE` / `LOCATION_LOST` |
| `UNKNOWN` | `INDETERMINATE` / `LOCATION_UNKNOWN` |

The ETA lookup already returns `status`, `freshnessStatus`, `ageSeconds`, `sourceObservedAt`, and `estimatedArrivalAt`. `FRESH` may be used. `STALE`, `EXPIRED`, and `UNKNOWN` must not prove a time window. The result is `INDETERMINATE` with an explicit reason.

`BNO_PREDICTION_MAX_ETA_AGE` remains the NLO-0.2 future-empty gate. It is not the current-trip GPS or ETA policy.

## Route insertion, planning only

A one-load insertion, when position and windows are fresh, may be evaluated as:

`CURRENT_POSITION` → additional pickup → existing destination → additional delivery

or, when the additional load shares the existing destination and the pickup is still ahead:

`CURRENT_POSITION` → additional pickup → existing destination

Road kilometres and seconds come from the routing port. Haversine may prefilter. It is not road distance. The sequence is a planning assumption on the candidate. It does not write stops onto the shipment.

## Slots and city rules

Tracking owns `shipment_slot_state`. NLO does not book. A stop that requires a slot and has none is `SLOT_UNKNOWN` or `SLOT_REQUIRED`, which blocks `EXECUTABLE` and leaves the planning result `INDETERMINATE` or `PLAN_ONLY`. `SLOT_CONFIRMED` requires a tracking state that names the window. `SLOT_UNCONFIRMED` is a known window that tracking has not confirmed.

City access restrictions, if an executable urban plan needs them, go through a future `CityRulesProvider`. Unknown or stale city rules fail closed for an executable urban plan. This phase hardcodes no Moscow or Saint Petersburg constants.
