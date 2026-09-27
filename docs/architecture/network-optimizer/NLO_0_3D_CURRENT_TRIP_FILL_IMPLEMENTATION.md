# NLO-0.3D current trip fill

Status: IMPLEMENTED_CLOSED. Merged to main in PR #178. Feature head `f61d64f8c34fa8cafa6bee30d44d8646c7c30655`. Merge SHA `ef3722db92fd1c115f6539cc491df3eabd57affa`. CI run `36307221142` succeeded on that feature head. Migration `000083_nlo_current_trip_fill_v0_3d`. NLO-0.3C stays IMPLEMENTED_CLOSED. NLO-0.3E is NOT_STARTED. NLO-0.3 is not complete. NLO-0.4 is not started.

```text
NLO_0_3C_STATUS=IMPLEMENTED_CLOSED
NLO_0_3D_STATUS=IMPLEMENTED_CLOSED
NLO_0_3D_MERGED=YES
NLO_0_3D_MERGE_SHA=ef3722db92fd1c115f6539cc491df3eabd57affa
CURRENT_TRIP_FILL_PUBLIC_ENABLED=YES
CURRENT_TRIP_FILL_MODE=PLANNING_ONLY
MAX_ADDITIONAL_LOADS=1
EXECUTION_SUPPORTED=NO
SHIPMENT_MUTATION=NO
ORDER_MUTATION=NO
ASSIGNMENT=NO
RESERVATION=NO
CARRIER_OFFER=NO
SLOT_BOOKING=NO
DRIVER_TASK=NO
NLO_0_3_COMPLETE=NO
NLO_0_3E_STATUS=NOT_STARTED
NLO_0_4_STATUS=NOT_STARTED
```

`CURRENT_TRIP_FILL_PUBLIC_ENABLED=YES` means the planning search API exists. It does not mean route execution, a multi-stop shipment, assignment, or reservation exists.

Controller remediation R1 is closed. `F001_CAPACITY_CONTEXT=CLOSED`. `F002_AUDIT_FINGERPRINT=CLOSED`.

## What this wave answers

`POST /v1/network/consolidation/search` with `pattern=CURRENT_TRIP_FILL` and `shipment_id` asks whether one published load can be planned onto the vehicle's current trip. The server builds `CurrentTripContext` and `ResidualCapacitySnapshot` through the NLO-0.3C provider. The caller cannot submit residual weight, volume, pallets, onboard cargo, GPS, ETA, vehicle capacity, temperature, or a context object. A foreign shipment is `NOT_FOUND`.

Each candidate is the confirmed onboard set plus exactly one additional published load. The search does not enumerate two or more extra loads. `max_additional_loads=1`. `execution_supported=false`.

## Proof

A candidate is `FEASIBLE` only when every mandatory fact is known and satisfied: residual weight, volume, linear metres, and pallet positions when the additional load states them; height fit; groupage compatibility; a `FRESH` tracking position and a `FRESH` ETA; a routing-provider road insertion; pickup and delivery windows; and a placement result of `SEQUENCE_OK` or `REHANDLE_REQUIRED`. Unknown stays `INDETERMINATE`. A proven miss is `HARD_REJECT`. Current occupancy that already exceeds capacity is never `FEASIBLE`.

Road distance and duration come from the routing provider. Haversine is not road distance. Tracking-service owns position and ETA freshness. This wave does not hard-code a freshness threshold and does not reuse `BNO_PREDICTION_MAX_ETA_AGE`.

Groupage reuses the existing B2 engine. Pallet conversion requires an explicit positive ownership-proven equivalence. Placement is a planning sequence check, not 3D packing. A forbidden rehandle is `REHANDLING_CONFLICT`.

## Persistence

Migration `000083_nlo_current_trip_fill_v0_3d` widens the NLO-0.3B consolidation audit. A `SAME_ORIGIN_SAME_DESTINATION` run requires `capacity_id` and `capacity_version`. A `CURRENT_TRIP_FILL` run stores both as NULL and does not publish a zero UUID or a zero version. The public current-trip response omits both fields and includes `shipment_id` and `shipment_version`. The internal audit trace keeps the compatibility fingerprint, rule-set and catalog versions, policy, shipment and vehicle versions, onboard cargo and profile versions, evidence-state versions, position and ETA freshness and time, residual provenance, routing provenance, and the candidate fingerprint. The public marketplace response does not include that trace.

## Still out

No shipment, transport order, assignment, reservation, carrier offer, slot, or driver task is written. No route plan is activated. No second additional load, solver, or NLO-0.3E ranking is included.
