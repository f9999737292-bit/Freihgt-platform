# ADR-TMS-005: Replan and successor semantics

## Status

Accepted. Architecture freeze. Implementation is not authorized.

## Context

ADR-NET-021 says replanning creates a successor, completed history is immutable, and current-trip activation does not move shipment status backward. NLO-0.4A leaves in-service stop behavior to execution. Today's driver model has no stop to preserve, so the rule has to be explicit before NLO-0.4D.

## Decision

The successor execution plan is created only from the successor's `EXECUTION_LINKED` activation. Completed stops stay as written. A stop in `ARRIVED` or `SERVICE_STARTED` stays until the driver or an operator finishes it, on the same shipment-owned id. If the successor omits or relocates that stop, projection returns `409 IN_SERVICE_STOP_CONFLICT`. Remaining `PLANNED` stops on the old plan become `CANCELLED` with reason `SUPERSEDED`. Their driver tasks are cancelled. New ids are allocated for the successor's new stops. The old plan becomes `SUPERSEDED` in the same transaction. Shipment status does not change.

```text
REPLAN_SUCCESSOR_MODEL_FROZEN=YES
CURRENT_STOP_REPLAN_RULE=PRESERVE_IN_SERVICE_STOP
COMPLETED_HISTORY_PRESERVED=YES
REMAINING_STOPS_SUPERSEDED_SAFELY=YES
```

## Consequences

Two active execution plans for one shipment are a failed transaction, not a supported state.
