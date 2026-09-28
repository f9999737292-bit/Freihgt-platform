# Implementation roadmap

Architecture accepted. No wave below is authorized.

```text
CONTROLLER_VERDICT=ACCEPT_TMS_MULTISTOP_ARCHITECTURE_R2
CONTROLLER_REVIEW_PENDING=NO
ARCHITECTURE_ACCEPTED=YES
IMPLEMENTATION_AUTHORIZATION_PENDING=YES
IMPLEMENTATION_AUTHORIZED=NO
TMS_MULTISTOP_IMPLEMENTATION_STARTED=NO
NLO_0_4D_STARTED=NO
NLO_0_4D_AUTHORIZED=NO
AGENT_C_MIGRATION_CREATED=NO
AGENT_C_MIGRATION_RESERVED=NO
NEXT_TMS_MIGRATION=UNRESERVED
```

Agent D may need migration `000086` for NLO-0.4C. This pack does not reserve a number. The implementation wave fetches `origin/main` and takes the next free number.

NLO-0.4D in the optimizer roadmap is the shipment and driver integration. This architecture is accepted. NLO-0.4D does not start here and is not authorized. A later Agent D task may start it only after Agent D implements the `PENDING_EXECUTION` projection command and the later `network.route_plan.execution_linked` post-link fact (`IMPLEMENTED_TODAY=NO`), and the NLO service-duration gate is open. The event is not the projection trigger. TMS-MSTOP-0.1A remains the next proposed execution wave and is not started.

```text
NLO_0_4C_ACTIVATION_RELEASE_BLOCKED_UNTIL_SERVICE_DURATION_SOURCE=YES
TMS_EXECUTION_DOES_NOT_WEAKEN_NLO_ACTIVATION_GATE=YES
```

No production execution activation is authorized until an authoritative or versioned service-duration source exists. Accepting this architecture does not bypass that gate. A wave that writes execution rows in production stays blocked while the gate is closed.

## Waves

The suggested split matches the repository: persistence before commands, commands before driver projection, tracking and Control Tower as consumers, replan last because it needs the plan and the stop lock.

| Wave | Scope | Depends on |
| --- | --- | --- |
| TMS-MSTOP-0.1A | `TransportExecution` with `operating_tenant_id`, revision, participant with `shipment_tenant_id`, stop, revision-stop link, action, and revision-action link. `CreateExecutionProjectionFromActivation` while activation is `PENDING_EXECUTION`. Idempotency on `activation_id`. Reject unresolved load opportunities. No driver UI | Accepted freeze. Agent D contract is the trusted synchronous command, then `EXECUTION_LINKED`, then `network.route_plan.execution_linked` (`IMPLEMENTED_TODAY=NO`). NLO service-duration gate still closed for production |
| TMS-MSTOP-0.1B | Stop and action commands, cargo evidence writes, shipment status alignment, completed-stop immutability | 0.1A |
| TMS-MSTOP-0.1C | `DriverStopTask` for current and next. Driver API. Offline queue still server-ordered | 0.1B |
| TMS-MSTOP-0.1D | Tracking target `execution_stop`. `planned_arrival` left intact. Advisory approach event | 0.1B |
| TMS-MSTOP-0.1E | Control Tower execution projection and stop id on delay and problem events | 0.1B |
| TMS-MSTOP-0.1F | Successor revision, inherited links, in-service lock, supersede of remaining planned stops without re-parenting completed rows | 0.1A and 0.1B. 0.1C before drivers see the switch |

0.1D and 0.1E can proceed in parallel after 0.1B. They do not block each other. 0.1F is not folded into 0.1A, because the first projection must be correct for a single plan before successor locking is added.

## Out of scope for every wave until a later ADR

A new shipment status, a slot-booking service, optimizer writes from shipment-service, geofence-authoritative arrival, and using RoutePlan stop ids as shipment primary keys.

## Single-leg shipments

Waves must leave the current milestone map in place when a shipment is not a participant of an `ACTIVE` `TransportExecution`.
