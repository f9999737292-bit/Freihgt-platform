# ADR-NET-013: Current-trip residual capacity semantics

Status: Accepted. NLO-0.3A is FROZEN_ACCEPTED. Implementation is not authorized.

## Decision

`PredictedCapacity` remains future empty capacity after the current shipment unloads. It is not remaining space while cargo is still on the vehicle.

In-trip space is a separate planning object, `ResidualCapacitySnapshot`, carried inside `CurrentTripContext`. It is not a new `Capacity.source` value and it is not a second meaning of `CURRENT_SHIPMENT_PREDICTION`.

`CurrentTripContextProvider` builds that context on the server from the trusted tenant and an owned `shipment_id`. A public request does not upload residual capacity, onboard cargo, GPS, ETA, vehicle capability, or shipment version. A foreign shipment id is `NOT_FOUND`. Public responses omit the internal tenant id.

## Why this shape

`network-optimizer-service/internal/predict/rule.go` copies `vehicle.CapacityWeightKg` and `vehicle.CapacityVolumeM3` onto the prediction and sets availability to ETA plus unload. `shipment-service` `PredictionInput` returns shipment status, vehicle id, and destination. It does not return onboard cargo or occupied weight. Search and MatchScore then treat that capacity as the vehicle that will be empty at the destination.

Reusing `Capacity.source = CURRENT_TRIP_RESIDUAL` would make next-load search and current-trip fill share one row with two meanings. A standalone snapshot without the trip context would drop shipment version, location freshness, and the evidence that cargo is onboard.

## Consequences

- Future empty capacity and in-trip residual capacity are never the same field.
- A residual fact is `UNKNOWN` when the vehicle total or the confirmed onboard occupancy for that dimension is unknown. Partial subtraction is not authoritative.
- No migration in this ADR.
