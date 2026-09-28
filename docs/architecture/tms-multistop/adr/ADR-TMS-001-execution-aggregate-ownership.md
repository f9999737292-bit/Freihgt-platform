# ADR-TMS-001: Execution aggregate ownership

## Status

Accepted. Architecture freeze. Implementation is not authorized.

## Context

`shipment-service` already owns shipment status, driver and vehicle assignment, driver operations, cargo execution evidence, and the shipment outbox. `network-optimizer-service` owns RoutePlan rows and does not set `execution_supported`. ADR-NET-021 assigns driver stop tasks and shipment status to shipment-service. The shipment row is one origin and one destination, so it cannot store an ordered stop list by itself.

## Decision

`shipment-service` owns execution. The shipment remains the aggregate root. `ShipmentExecutionPlan`, `ShipmentExecutionStop`, and `ShipmentStopAction` are children of that aggregate. No new execution service is created. The optimizer does not gain execution tables.

```text
EXECUTION_AGGREGATE_OWNER=shipment-service
NEW_AGGREGATE_REQUIRED=NO
REUSE_EXISTING_SHIPMENT_AGGREGATE=YES
```

## Consequences

NLO-0.4D, when later authorized, writes shipment-owned rows from an activated plan. It does not move stop completion into the optimizer.
