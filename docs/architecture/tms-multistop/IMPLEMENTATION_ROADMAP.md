# Implementation roadmap

Architecture only. No wave below is authorized.

```text
IMPLEMENTATION_AUTHORIZED=NO
TMS_MULTISTOP_IMPLEMENTATION_STARTED=NO
NLO_0_4D_STARTED=NO
AGENT_C_MIGRATION_CREATED=NO
AGENT_C_MIGRATION_RESERVED=NO
NEXT_TMS_MIGRATION=UNRESERVED
```

Agent D may need migration `000086` for NLO-0.4C. This pack does not reserve a number. The implementation wave fetches `origin/main` and takes the next free number.

NLO-0.4D in the optimizer roadmap is the shipment and driver integration. It does not start here. It starts only after this freeze is accepted and the Agent D activation contract is actually emitted.

## Waves

The suggested split matches the repository: persistence before commands, commands before driver projection, tracking and Control Tower as consumers, replan last because it needs the plan and the stop lock.

| Wave | Scope | Depends on |
| --- | --- | --- |
| TMS-MSTOP-0.1A | Execution plan, stop, and action tables. `CreateExecutionProjectionFromActivation`. Idempotency on `activation_id`. No driver UI | Accepted freeze. Agent D `network.route_plan.execution_linked` |
| TMS-MSTOP-0.1B | Stop and action commands, cargo evidence writes, shipment status alignment, completed-stop immutability | 0.1A |
| TMS-MSTOP-0.1C | `DriverStopTask` for current and next. Driver API. Offline queue still server-ordered | 0.1B |
| TMS-MSTOP-0.1D | Tracking target `execution_stop`. `planned_arrival` left intact. Advisory approach event | 0.1B |
| TMS-MSTOP-0.1E | Control Tower execution projection and stop id on delay and problem events | 0.1B |
| TMS-MSTOP-0.1F | Successor transaction, in-service lock, supersede of remaining stops | 0.1A and 0.1B. 0.1C before drivers see the switch |

0.1D and 0.1E can proceed in parallel after 0.1B. They do not block each other. 0.1F is not folded into 0.1A, because the first projection must be correct for a single plan before successor locking is added.

## Out of scope for every wave until a later ADR

A new shipment status, a slot-booking service, optimizer writes from shipment-service, geofence-authoritative arrival, and using RoutePlan stop ids as shipment primary keys.

## Single-leg shipments

Waves must leave the current milestone map in place when no `ACTIVE` execution plan exists.
