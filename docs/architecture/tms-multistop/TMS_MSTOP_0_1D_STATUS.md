# TMS-MSTOP-0.1D status

Execution-stop tracking. This note records the 0.1D implementation. It does not change the frozen architecture from PR #186.

```text
TRACKING_OWNER=tracking-service
EXECUTION_STOP_TARGET_IMPLEMENTED=YES
EXECUTION_EVENT_CONSUMER_IMPLEMENTED=YES
EXECUTION_STOP_ETA_IMPLEMENTED=YES
PLANNED_ARRIVAL_PRESERVED=YES
APPROACH_EVENT_IMPLEMENTED=YES
TRACKING_OWNED_OUTBOX_IMPLEMENTED=YES
AUTO_ARRIVE_STOP=NO
TRACKING_MUTATES_SHIPMENT_STATUS=NO
TRACKING_MUTATES_EXECUTION_STOP=NO
TRACKING_WRITES_CARGO_EVIDENCE=NO
CONTROL_TOWER_NOT_STARTED=YES
REPLAN_NOT_STARTED=YES
NLO_0_4D_NOT_STARTED=YES
NLO_0_4D_AUTHORIZED=NO
READY_FOR_PRODUCTION_EXECUTION=NO
```

Tracking owns the route target in `tracking.execution_tracking_state`. That table is a tracking projection, not a copy of `TransportExecution`. The parent is the transport execution. An execution-stop ETA does not require a participant shipment id.

Shipment-service emits `shipment.route_stop.current` for the initial open stop in the projection transaction. Replay does not emit it again. A normal ArriveStop does not emit a new current event. A controlled ArriveStop override emits `shipment.route_stop.current` when the current stop changes. Completing the last stop does not invent a current stop. Event payloads include `operating_tenant_id` and omit participant shipment tenant, price, rate, capacity, optimizer payload, and leg geometry.

Shipment-service publishes the five `shipment.route_stop.*` events on the existing shipment status topic. They are not driver events and they are not rewritten into a shipment status envelope. Unknown execution event types stay rejected.

The tracking consumer reads `shipment.status.v1` with the existing franz-go consumer settings. It starts only when `TRACKING_KAFKA_BROKERS` and `TRACKING_SHIPMENT_INTERNAL_URL` are set. A failed execution record is retried on that partition before any later offset is applied or committed. Shipment stop commands do not call tracking. Duplicate events are stored in `tracking.execution_event_inbox`. Older `event_sequence` values do not move the target. `revision_version` is not the order key.

The live ETA target is the current canonical non-START stop. A START position anchor can be the route position and is not an approach target. Coordinates come from the shipment-service internal read `GET /internal/v1/transport-executions/{executionId}/stops/{stopId}/tracking-context`. Unknown coordinates stay null. The read requires `X-Internal-Service-Token` and the operating tenant from the event. It returns the active revision only.

`target_type=execution_stop` is stored with `execution_id` and `execution_stop_id`. Pickup and delivery still require `shipment_id` and a null `execution_stop_id`. Database checks enforce that shape. Current execution-stop ETA lives in `tracking.execution_stop_eta_state`. `tracking.shipment_eta_state` stays shipment-scoped. Live ETA does not update `planned_arrival`. Provider ETA for a non-current, foreign, or ambiguous device-to-execution match is rejected. Freshness and quality reuse the existing evaluators.

`tracking.stop.approaching` is advisory. An accepted location on the approach path writes `tracking.location_event`, `tracking.shipment_tracking_state`, the approach marker, and `tracking.event_outbox` in one transaction. The event is not written to `transport.shipment_event_outbox`. The payload has no raw coordinates and no shipment tenant. `TRACKING_STOP_APPROACH_ENABLED` defaults to false. When it is true, `TRACKING_STOP_APPROACH_RADIUS_METERS` must be greater than 0 or config load fails. The existing tracking-loss path that writes driver events to the shipment outbox is unchanged.

Execution-stop ETA observation and current state commit in one transaction. The transaction locks `tracking.execution_tracking_state` and revalidates the live stop before insert. Current state changes only when `repository.ShouldReplaceETAObservation` selects the incoming observation. An older unique observation stays in history and does not move current state. Pickup and delivery ETA ingestion is unchanged.

The tracking outbox publisher uses the same header contract as the shipment status publisher (`event_type`, `schema_version`, `source_event_id`, `content_type`). It starts only when brokers and `TRACKING_KAFKA_TOPIC` are set. It does not configure Kafka TLS or SASL. Real Kafka was not executed in this wave. `services/tracking-service` is in the `backend-go-check` matrix.

The public shipment tracking APIs are unchanged. No execution-stop route was added to the API gateway. The OpenAPI generator does not own tracking-service or internal routes, so the canonical specs were not extended.

Migration: `000091_tms_execution_stop_tracking_v0_1d`. Down removes only the 0.1D tracking tables, execution-stop ETA columns, and the execution-stop target check.

Control Tower execution projection, successor replan, route deviation, slot-booking changes, driver mobile, and NLO-0.4D are not part of this wave.
