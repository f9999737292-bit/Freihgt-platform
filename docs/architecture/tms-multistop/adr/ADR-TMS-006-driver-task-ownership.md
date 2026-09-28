# ADR-TMS-006: Driver task ownership

## Status

Accepted. Architecture freeze. Implementation is not authorized.

## Context

`transport.driver_task` is an inbox of confirmation notices with an acknowledgement lifecycle. Driver milestones already move shipment status and write cargo evidence. ADR-NET-021 says stop tasks, if built, are shipment-owned projections and that today's notice types stay.

## Decision

`shipment-service` owns driver stop work. Notice tasks are unchanged. A `DriverStopTask` references `execution_stop_id` and mirrors stop status. The stop is authoritative. The driver API shows the current open stop and the next stop, not the optimizer candidate. Offline retries use the existing idempotency record. The server applies commands in stop order when they arrive.

```text
DRIVER_TASK_OWNER=shipment-service
DRIVER_TASK_REUSE=PARTIAL
DRIVER_NEXT_STOP_CONTRACT=CURRENT_AND_NEXT_ONLY
OFFLINE_MODEL=CLIENT_QUEUE_SERVER_IDEMPOTENCY
```

## Consequences

A new task type is not added to the notice check constraint. Stop execution does not become a second inbox product.
