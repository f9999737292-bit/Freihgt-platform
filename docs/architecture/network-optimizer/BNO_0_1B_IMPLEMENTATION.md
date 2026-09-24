# BNO-0.1B implementation

Rule-based predicted capacity and the equipment, loading, and temperature capability foundation. This is not next-load matching.

## Predicted capacity

Predicted capacity is a forecast of when an assigned vehicle becomes free after its current shipment is unloaded. It is not the vehicle master, not a manual capacity publication, not current-trip residual space, and not a marketplace advertisement.

The release location is the shipment destination location. Current GPS is not used. No intermediate stops are created.

```text
service_start = max(ETA, planned_delivery_at when a planned delivery exists)
predicted_available_at = service_start + BNO_PREDICTION_DEFAULT_UNLOAD_DURATION
availability_window = predicted_available_at ± BNO_PREDICTION_AVAILABILITY_UNCERTAINTY
```

The unload duration and the uncertainty are planning assumptions from configuration. They are stored with the prediction as `unload_policy_source=BNO_PREDICTION_DEFAULT_UNLOAD_DURATION` and rule version `bno-predict-0.1b.1`. If either duration is unset, or the ETA freshness limit `BNO_PREDICTION_MAX_ETA_AGE` is unset, no actionable prediction is created.

ETA comes from tracking-service delivery ETA. Both the arrival instant and `sourceObservedAt` are required. There is no substitution of the current time, pickup time, or a computed drive time. An observation older than `BNO_PREDICTION_MAX_ETA_AGE` is `ETA_STALE`.

## Full future capacity

When the shipment is a single origin-destination movement and the repository can prove the vehicle has no other active assignment, the snapshot copies the vehicle nominal weight and volume. Current shipment cargo is not subtracted.

```text
NEXT_LOAD_FUTURE_CAPACITY = vehicle nominal capability
CURRENT_TRIP_RESIDUAL_CAPACITY is not implemented
```

If another active assignment exists, the result is `VEHICLE_FUTURE_AVAILABILITY_AMBIGUOUS`. Full future capacity is not claimed.

## Confidence

`confidence` is a deterministic rule score in `[0, 1]`, rule version `bno-predict-0.1b.1`. It is not a calibrated probability.

Start at 1 and subtract only:

- 0.15 when no planned delivery is known
- 0.10 when destination coordinates are missing
- 0.05 when normalized body type is unknown
- 0.05 when nominal weight is unknown
- 0.05 when nominal volume is unknown
- 0.10 when ETA age / max age is greater than 0.25 and at most 0.50
- 0.20 when ETA age / max age is greater than 0.50

The result is clamped to `[0, 1]`. Activation from `PREDICTED` to `AVAILABLE` requires confidence at or above `BNO_PREDICTION_CONFIDENCE_FLOOR` (default 0.5). `BNO_PREDICTION_AUTO_ACTIVATE` defaults to false. Activation does not publish the capacity.

## Lifecycle

Generated predictions are `status=PREDICTED` and `visibility=PRIVATE`. Marketplace reads still require `status=AVAILABLE`, so a predicted row is not found there. The same input fingerprint returns the current prediction and does not emit another `network.capacity.predicted` event. A changed fingerprint supersedes the previous current prediction. A superseded prediction cannot be activated. Cancellation withdraws the current predicted capacity with `SHIPMENT_CANCELLED`.

Same effective inputs do not create a duplicate. The fingerprint covers the shipment, vehicle capability, destination, ETA and its observation time, delivery window, unload duration, uncertainty, max ETA age, and rule version.

## Capability model

The vehicle master remains `transport.vehicles` in shipment-service. Network optimizer stores a snapshot. There is no second vehicle master and no trailer master.

Combination type and body type are separate. Canonical combination values are `TRUCK`, `TRACTOR_SEMITRAILER`, `TRUCK_TRAILER`, `ROAD_TRAIN`, and `OTHER`. Canonical body values are `TENT`, `CONTAINER`, `ISOTHERMAL`, `REFRIGERATOR`, `BOX`, `PLATFORM`, `LOWBED`, `TANK`, `TIPPER`, `CAR_CARRIER`, `TIMBER`, and `OTHER`.

`TENT` does not imply loading access. Loading and unloading are independent arrays of `REAR`, `SIDE`, and `TOP`.

`ISOTHERMAL` is not `REFRIGERATOR`. A refrigerator does not receive a fabricated range. Temperature control mode is `NONE`, `PASSIVE`, or `ACTIVE`. Passive isothermal equipment does not satisfy cargo that requires active control (`TEMPERATURE_CONTROL_REQUIRED`).

The vehicle capability range is not the current trip setpoint. This release does not invent preconditioning time. If a future cargo needs a temperature the current setpoint does not already satisfy, compatibility stays indeterminate with `TEMPERATURE_PRECONDITIONING_REQUIRED`.

Container size tokens `20FT`, `40FT`, `40HC`, `45FT`, and `REEFER_CONTAINER` can be stored. They are not used to optimize container moves.

Load opportunity cargo can carry `required_body_types`, `required_loading_access`, `required_unloading_access`, `temperature_required`, temperature bounds, and an optional preferred setpoint. Legacy free-text `body_type` and `equipment_type` are not guessed. An exact canonical token is the only legacy mapping.

## Unknown

NULL means unknown. An empty access array means a known empty set. Unknown is not zero, not false, and not supported. Compatibility returns `INDETERMINATE` when a required fact is unknown, and `INCOMPATIBLE` only when the known facts conflict.

Single-zone co-load requires a non-empty temperature intersection. `+2..+8` against `-25..-18` is `TEMPERATURE_RANGES_INCOMPATIBLE`. Multi-zone optimization is not implemented. A single-zone vehicle is not treated as able to carry both intervals at once.

## Sources

- Shipment prediction input: `GET /internal/v1/shipments/{id}/prediction-input` on shipment-service, tenant-scoped, internal service token required. It returns status, version, vehicle id, destination id and coordinates, planned delivery, and the count of other active assignments. It does not return cargo weight, driver identity, or contacts.
- Vehicle capability: `GET /internal/v1/vehicles/{id}/capability` on the same service.
- ETA: existing `POST /internal/v1/tracking/eta/lookup`. Tracking remains the ETA owner. The client keeps only arrival and `sourceObservedAt`.

Eligible shipment statuses are `VEHICLE_ASSIGNED`, `DRIVER_ASSIGNED`, `PICKUP_SLOT_BOOKED`, `DELIVERY_SLOT_BOOKED`, `IN_PICKUP`, `LOADED`, `IN_TRANSIT`, `ARRIVED_AT_CONSIGNEE`, and `UNLOADING`.

## API

Owner routes, also exposed through the gateway:

- `POST /v1/network/shipments/{shipmentId}/predicted-capacity`
- `POST /v1/network/predicted-capacities/{id}/refresh`
- `GET /v1/network/predicted-capacities/{id}`
- `GET /v1/network/predicted-capacities`
- `POST /v1/network/predicted-capacities/{id}/activate`

There is no `/matches`, `/next-load`, `/recommendations`, or `/top-n` route.

The new event is `network.capacity.predicted`. Activation emits `network.capacity.updated`. Cancellation emits `network.capacity.withdrawn`. Match, chain, route, and offer events are not emitted. Event payloads omit coordinates, vehicle id, payload, tokens, and contact data.

Migration `000076` evolves the capacity source and status checks and adds `network_optimizer.predicted_capacities` plus nullable vehicle capability columns. Migration `000075` is unchanged.

## Not in this release

Next-load search, match candidates, match scores, ranking, top N, deadhead, road distance, backhaul, consolidation, current-trip fill, multi-stop execution, regional or city routing, and machine learning.
