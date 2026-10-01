# TMS-MSTOP-0.1E status

CONTROL_TOWER_OWNER=control-tower-read-model-service
EXECUTION_PROJECTION_IMPLEMENTED=YES
STOP_PROJECTION_IMPLEMENTED=YES
ACTION_PROJECTION_IMPLEMENTED=YES
EXECUTION_EVENT_INBOX_IMPLEMENTED=YES
EXECUTION_EVENT_GAP_HANDLING_IMPLEMENTED=YES
EXECUTION_PLAN_CREATED_IMPLEMENTED=YES
EXECUTION_PLAN_SUPERSEDED_PRODUCER_IMPLEMENTED=NO
EXECUTION_PLAN_SUPERSEDED_CONSUMER_READY=YES
TRACKING_APPROACH_CONSUMER_IMPLEMENTED=YES
DRIVER_DELAY_STOP_CONTEXT_IMPLEMENTED=YES
DRIVER_PROBLEM_STOP_ACTION_CONTEXT_IMPLEMENTED=YES
DRIVER_EXECUTION_CONTEXT_SERVER_VALIDATED=YES
CLIENT_EXECUTION_STOP_ID_TRUSTED=NO
CLIENT_ACTION_ID_TRUSTED=NO
FOREIGN_STOP_CONTEXT_REJECTED=YES
FOREIGN_ACTION_CONTEXT_REJECTED=YES
CROSS_SHIPPER_PRIVACY=PASS
CONTROL_TOWER_WRITES_EXECUTION=NO
CONTROL_TOWER_WRITES_SHIPMENT_STATUS=NO
TMS_MSTOP_0_1F_STARTED=NO
NLO_0_4D_STARTED=NO
READY_FOR_PRODUCTION_EXECUTION=NO

Execution projection rebuild is event replay only. Shipment status snapshot rebuild is unchanged.

`shipment.execution_plan.created` is emitted in the same transaction as the initial TransportExecution projection. Replay of that activation does not emit a second created event. `shipment.execution_plan.superseded` is not produced.

Tracking approach events stay on `TRACKING_KAFKA_TOPIC`. Control Tower consumes them with group `control-tower-tracking-approach-v1` only when `CONTROL_TOWER_TRACKING_KAFKA_TOPIC` is set. Approach updates advisory fields and does not mark a stop arrived.

Shipper reads use the existing shipment ownership check. Carrier execution rows do not store `shipment_tenant_id`.

Driver delay and problem requests may carry `executionStopId` and `actionId` only as hints. Shipment-service publishes those fields after `ResolveDriverExecutionContext` matches the authenticated operating tenant, assigned driver, shipment participant, current active revision, stop, and action. A syntactically valid identifier that fails that match is rejected and creates no driver outbox event. Requests without those identifiers stay valid. Control Tower `LinkDriverContext` remains defense-in-depth.
