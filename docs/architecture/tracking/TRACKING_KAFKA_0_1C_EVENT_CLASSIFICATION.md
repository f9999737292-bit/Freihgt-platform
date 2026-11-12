# Tracking Kafka 0.1C — event classification

Status: allowlist empty.
`NON_STOP_ALLOWLIST=EMPTY`.
`VERIFIED_NON_STOP_SKIP` applies to no type.
`UNKNOWN_EVENT=NON_TERMINAL`.
`INVALID_JSON=NON_TERMINAL`.

A type is eligible for a later allowlist entry only when a written business rule says that tracking state may stay unchanged and the offset may advance. Presence in `isKnownKafkaEventType` is a publish gate (`services/shipment-service/internal/outbox/kafka_publisher.go`). It is not that business rule. This document adds no type from that gate.

Absence from the allowlist does not authorize an offset commit. The partition stays on the record under contiguous hold.

## 1. Topic and consumers

`resolveKafkaTopic` sends every non-driver event to the configured shipment topic. The shipment default is `shipment.status.v1` (`services/shipment-service/internal/config/kafka.go`). The tracking consumer default group is `tracking-execution-stop` on that topic (`services/tracking-service/internal/config/config.go`). The control-tower status consumer default group is `control-tower-shipment-status-v1` on the same default topic (`services/control-tower-read-model-service/internal/config/config.go`). Those are different groups. A tracking decision does not commit the control-tower offset, and a control-tower commit does not commit the tracking offset.

Driver events use `driver.events.v1` by default. They are listed in section 4 so a misrouted record is not treated as an allowlist candidate. The tracking parser reads JSON `event_type`. A driver envelope uses `eventType` (`services/shipment-service/internal/domain/driver_events.go`). A body without `event_type` is not on the allowlist.

Invalid JSON does not yield an event type. `isExecutionStopEvent` in the current consumer returns false for a parse failure (`services/tracking-service/internal/service/execution_event_consumer.go`). This classification still requires `NON_TERMINAL` and forbids the commit. The current consumer is not changed here.

## 2. Records on the default shipment topic

Tracking state effect means a change to `tracking.execution_tracking_state` that section 4 of `TRACKING_KAFKA_0_1C_PERSISTENCE.md` can prove. "Ignore" means commit the offset with no such effect. No row below has ignore allowed.

| Event type | Producer evidence | Schema | Tracking state effect | Ignore | If the offset advanced anyway |
|---|---|---|---|---|---|
| `shipment.route_stop.current` | `EventRouteStopCurrent` and `IsExecutionKafkaEventType` in `transport_execution_commands.go`. Payload from `emitExecutionEvent` in `transport_execution_command_repository.go`. | JSON `event_type`, `event_id`, `operating_tenant_id`, `execution_id`, `revision_id`, `event_sequence`, `stop_id`. Header `eventType` is separate. Outbox id differs from payload `event_id`. | Required when the shipment context matches. A nil context is not a proven skip. | No | A current stop can be lost while the cursor moves. |
| `shipment.route_stop.arrived` | `EventRouteStopArrived`. Same emitter. | Same execution payload. | Required only when a proven sequence touch or equivalent state effect exists. The current nil-state path is not that proof. | No | Arrival is unmarked and the cursor moves. |
| `shipment.route_stop.service_started` | `EventRouteStopServiceStarted`. Same emitter. | Same execution payload. | Same as arrived. | No | Service start is unmarked and the cursor moves. |
| `shipment.route_stop.completed` | `EventRouteStopCompleted`. Same emitter. | Same execution payload. | Required when a proven clear of the matching stop exists. Zero rows is not success. | No | A completed stop can remain the target. |
| `shipment.route_stop.sequence_overridden` | `EventRouteStopSequenceOverridden`. Same emitter. | Same execution payload, including `skipped_ordinals`. | Required only with a proven state effect. No such rule is specified for the skipped ordinals. | No | Override details are passed without a state proof. |
| `shipment.execution_plan.created` | `EventExecutionPlanCreated`. `emitExecutionPlanCreated` writes a wider payload. | Execution identity plus stops and actions. | Not specified. No allowlist rule says the plan may be ignored. | No | Tracking can keep a previous target across a new plan. |
| `shipment.execution_plan.superseded` | `EventExecutionPlanSuperseded`. `emitExecutionPlanSuperseded`. | Execution identity plus old and new revision. | Not specified. | No | Tracking can keep a superseded target. |
| `shipment.delivery.partially_rejected` | `transport_execution_disposition.go`, included by `IsExecutionKafkaEventType`. | Disposition payload from that producer. | Not specified. | No | A rejection is passed without a tracking decision. |
| `shipment.delivery.rejected` | Same disposition constants. | Same family. | Not specified. | No | Same as a partial rejection. |
| `shipment.cargo.disposition_pending` | Same file. | Same family. | Not specified. | No | Pending cargo work is passed. |
| `shipment.cargo.return_authorized` | Same file. | Same family. | Not specified. | No | An authorization is passed. |
| `shipment.cargo.redirect_authorized` | Same file. | Same family. | Not specified. | No | An authorization is passed. |
| `shipment.cargo.return_completed` | Same file. | Same family. | Not specified. | No | A completed return is passed. |
| `shipment.cargo.redirect_completed` | Same file. | Same family. | Not specified. | No | A completed redirect is passed. |
| `shipment.created` | `OutboxEventTypeCreated` in `outbox.go`. `MapOutboxEventType` selects it when `fromStatus` is absent. | `ShipmentStatusEventData`: `fromStatus`, `toStatus`, actor and time fields. The body has no `event_type`. | Not a tracking execution effect. Control tower is a consumer of this family. | No | Tracking would commit a record it did not classify. Control tower's own group is independent. |
| `shipment.status.changed` | `OutboxEventTypeStatusChanged`. | Same status body. | Not specified for tracking state. | No | A status change is passed by tracking. |
| `shipment.cancelled` | `OutboxEventTypeCancelled` when `toStatus` is cancelled. | Same status body. | Not specified. | No | A cancellation is passed by tracking. |
| `shipment.ready_for_billing` | `OutboxEventTypeReadyForBilling`. | Same status body. | Not specified. | No | A billing-readiness record is passed. |
| `shipment.documents_completed` | `OutboxEventTypeDocumentsCompleted`. | Same status body. | Not specified. | No | A document record is passed. |
| `shipment.financially_closed` | `OutboxEventTypeFinanciallyClosed`. | Same status body. | Not specified. | No | A financial-close record is passed. |

## 3. Parser outcomes

| Input | Class | Offset |
|---|---|---|
| Invalid JSON | `NON_TERMINAL` | Unchanged |
| JSON object with no `event_type`, including `ShipmentStatusEventData` | `NON_TERMINAL` | Unchanged |
| `event_type` present and not on the allowlist | `NON_TERMINAL` | Unchanged |
| One of the five stop types | Eligible for a state-effect attempt. Terminal only after the persistence proof. | Unchanged until that proof commits |
| A future allowlist entry | `VERIFIED_NON_STOP_SKIP` only for that one type, after a controller writes the business rule | May advance only for that type |

An unknown type stays `NON_TERMINAL` on every replay. Storing the bytes in a future quarantine row does not change that class.

## 4. Driver topic, not an allowlist source

`IsDriverKafkaEventType` includes `driver.location.updated`, `driver.arrived_at_pickup`, `driver.departed_pickup`, `driver.arrived_at_delivery`, `driver.delivery.completed`, `driver.delay.reported`, `driver.problem.reported`, `driver.documents.uploaded`, `driver.tracking.lost`, `driver.tracking.restored`, `driver.exception_reported`, and `driver.shipment_event_recorded`. The publisher sends those to the driver topic. They are not approved skips if they appear on `shipment.status.v1`. The tracking parser does not read `eventType`, so the body is an unknown record and stays `NON_TERMINAL`.

## 5. Open question

A controller may add one type at a time. The addition needs the producer, the schema, the consumers, the state effect that is intentionally absent, and the consequence that was accepted. Until that written rule exists, the allowlist stays empty.

## 6. Acceptance

- `KNOWN_NON_STOP_EVENT` has no current member.
- `UNKNOWN_EVENT=NON_TERMINAL`.
- `INVALID_JSON=NON_TERMINAL`.
- `PUBLISH_GATE_IS_SKIP_AUTHORITY=NO`.
- `ALLOWLIST_COUNT=0`.
