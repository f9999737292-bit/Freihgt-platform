# ADR-TMS-005: Replan and successor semantics

## Status

Accepted. Architecture freeze, remediation R1. Implementation is not authorized.

## Context

ADR-NET-021 says replanning creates a successor and does not move shipment status backward. The first execution freeze said a completed stop keeps its id and is also represented on the successor while the stop's only parent was `execution_plan_id`. One row cannot be a child of two revisions. Remediation R1 uses direction A: the stop's parent is the long-lived `TransportExecution`. Revisions reference stops. They do not own them.

## Decision

A successor `EXECUTION_LINKED` activation creates a new `TransportExecutionRevision` on the same `TransportExecution`. In the same transaction:

| Piece | Rule |
| --- | --- |
| Completed stop | Same id. `execution_id` unchanged. New link `INHERITED_COMPLETED`. No update of the stop row |
| Stop in `ARRIVED` or `SERVICE_STARTED` | Same id. `execution_id` unchanged. New link `INHERITED_IN_SERVICE`. Status and timestamps unchanged |
| Remaining `PLANNED` stops | Status `CANCELLED`, reason `SUPERSEDED`, on the old revision only. New ids, still parented by the same route, linked `INTRODUCED` on the successor |
| In-service driver task | Stays on the preserved stop |
| Future driver tasks | Cancelled with the superseded planned stops. New tasks for introduced stops |
| Old revision | `SUPERSEDED` |
| Participating shipment status | Unchanged |

If the successor omits or relocates the in-service stop, projection returns `409 IN_SERVICE_STOP_CONFLICT` and leaves the old revision `ACTIVE`.

```text
REPLAN_SUCCESSOR_MODEL_FROZEN=YES
REPLAN_PERSISTENCE=ROUTE_OWNS_STOPS_REVISIONS_LINK_THEM
CURRENT_STOP_REPLAN_RULE=PRESERVE_IN_SERVICE_STOP
COMPLETED_STOP_ROW_REPARENTED=NO
IN_SERVICE_STOP_ROW_REPARENTED=NO
COMPLETED_HISTORY_IMMUTABLE=YES
REMAINING_STOPS_SUPERSEDED_SAFELY=YES
```

## Consequences

Two `ACTIVE` revisions of one `TransportExecution` are a failed transaction. A shipment participates in at most one active execution.
