# TMS-MSTOP-0.1F status

SUCCESSOR_REVISION_IMPLEMENTED=YES
COMPLETED_HISTORY_IMMUTABLE=YES
COMPLETED_STOPS_REPARENTED=NO
COMPLETED_ACTIONS_REPARENTED=NO
OLD_REMAINING_STOPS_SUPERSEDED=YES
OLD_REMAINING_ACTIONS_SUPERSEDED=YES
SERVICE_STARTED_REPLAN_LOCK=YES
EXPECTED_REVISION_CONCURRENCY=YES
IDEMPOTENCY=YES
EXECUTION_PLAN_SUPERSEDED_PRODUCER=YES
CONTROL_TOWER_SUCCESSOR_PROJECTION=YES
SHIPMENT_FSM_CHANGED=NO
NLO_0_4D_STARTED=NO
READY_FOR_PRODUCTION_EXECUTION=NO

One `transport.transport_executions` row stays the execution root. `current_revision_id` moves to the new ACTIVE revision in the same transaction that marks the previous revision SUPERSEDED. `event_seq` is not reset.

Stop and action rows do not store a revision id. Revision membership is `transport.transport_execution_revision_stops` and `transport.transport_execution_revision_actions`. Completed, skipped, and cancelled stop rows are not updated. Their old membership stays, and the successor revision adds `INHERITED_COMPLETED`. The same rule applies to terminal actions. Open `PLANNED` and `ARRIVED` memberships become `SUPERSEDED`. Stop status and timestamps on those rows stay as they were. `ARRIVED` may be superseded. `SERVICE_STARTED` rejects the whole command with `EXECUTION_STOP_IN_SERVICE` before any revision, membership, outbox, or driver-task write.

Replacement stops and actions are new rows. Action ids are not reused, because the old action row remains the historical fact. New action shipment tenants are copied from the existing participant row. Participants are not inserted again and shipment status is not changed. The current stop is the first new `PLANNED` stop. The request cannot choose it.

No migration was added. Head remains `000092`. The 0.1A membership model already represents a successor.

The command emits `shipment.execution_plan.superseded` for the old revision, then `shipment.execution_plan.created` for introduced stops only, then `shipment.route_stop.current`. Control Tower keeps the first plan as history, marks old open stops `SUPERSEDED`, and applies a later plan-created event as the active revision. A late stop event whose revision is not active does not change the current stop or reopen a superseded stop. Approach updates do not touch completed or superseded stops.

Internal command path: `POST /internal/v1/transport-executions/{executionId}/successor-revision`. Operating tenant is the verified `X-Tenant-ID` header. A body tenant is ignored.
