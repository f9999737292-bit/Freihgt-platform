# NLO-0.3D current trip fill

Status: IMPLEMENTED_IN_BRANCH / UNDER_REVIEW. This branch does not merge. NLO-0.3C stays IMPLEMENTED_CLOSED. NLO-0.3E is NOT_STARTED. NLO-0.3 is not complete. NLO-0.4 is not started.

```text
NLO_0_3C_STATUS=IMPLEMENTED_CLOSED
NLO_0_3D_STATUS=UNDER_REVIEW
NLO_0_3_COMPLETE=NO
NLO_0_3E_STARTED=NO
CURRENT_TRIP_FILL=planning only
EXECUTION_SUPPORTED=false
MAX_ADDITIONAL_LOADS=1
SHIPMENT_MUTATION=NO
```

## What this wave answers

`POST /v1/network/consolidation/search` with `pattern=CURRENT_TRIP_FILL` and `shipment_id` asks whether one published load can be planned onto the vehicle's current trip. The server builds `CurrentTripContext` and `ResidualCapacitySnapshot` through the NLO-0.3C provider. The caller cannot submit residual weight, volume, pallets, onboard cargo, GPS, ETA, vehicle capacity, temperature, or a context object. A foreign shipment is `NOT_FOUND`.

Each candidate is the confirmed onboard set plus exactly one additional published load. The search does not enumerate two or more extra loads. `max_additional_loads=1`. `execution_supported=false`.

## Proof

A candidate is `FEASIBLE` only when every mandatory fact is known and satisfied: residual weight, volume, linear metres, and pallet positions when the additional load states them; height fit; groupage compatibility; a `FRESH` tracking position and a `FRESH` ETA; a routing-provider road insertion; pickup and delivery windows; and a placement result of `SEQUENCE_OK` or `REHANDLE_REQUIRED`. Unknown stays `INDETERMINATE`. A proven miss is `HARD_REJECT`. Current occupancy that already exceeds capacity is never `FEASIBLE`.

Road distance and duration come from the routing provider. Haversine is not road distance. Tracking-service owns position and ETA freshness. This wave does not hard-code a freshness threshold and does not reuse `BNO_PREDICTION_MAX_ETA_AGE`.

Groupage reuses the existing B2 engine. Pallet conversion requires an explicit positive ownership-proven equivalence. Placement is a planning sequence check, not 3D packing. A forbidden rehandle is `REHANDLING_CONFLICT`.

## Persistence

Migration `000083_nlo_current_trip_fill_v0_3d` widens the NLO-0.3B consolidation audit so a current-trip row can omit `capacity_id`. Provenance stays in the candidate trace and fingerprint. The response summary is output only.

## Still out

No shipment, transport order, assignment, reservation, carrier offer, slot, or driver task is written. No route plan is activated. No second additional load, solver, or NLO-0.3E ranking is included.
