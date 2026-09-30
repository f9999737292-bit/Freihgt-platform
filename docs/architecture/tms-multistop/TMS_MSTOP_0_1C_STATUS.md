# TMS-MSTOP-0.1C status

Driver stop tasks and the assigned-driver stop API. This note records the 0.1C implementation. It does not change the frozen architecture from PR #186.

```text
DRIVER_STOP_TASK_IMPLEMENTED=YES
DRIVER_STOP_TASK_SEPARATE_MODEL=YES
NOTICE_DRIVER_TASK_REUSED_AS_STOP_TASK=NO
CURRENT_NEXT_API_IMPLEMENTED=YES
DRIVER_NEXT_STOP_CONTRACT=CURRENT_AND_NEXT_ONLY
DRIVER_AUTHORIZATION=ASSIGNED_DRIVER_ONLY
SERVER_IDEMPOTENCY=YES
OFFLINE_CLIENT_QUEUE_IMPLEMENTED=NO
TRACKING_NOT_STARTED=YES
CONTROL_TOWER_NOT_STARTED=YES
SUCCESSOR_REPLAN_NOT_STARTED=YES
NLO_0_4D_NOT_STARTED=YES
NLO_0_4D_AUTHORIZED=NO
READY_FOR_PRODUCTION_EXECUTION=NO
```

`transport.driver_stop_tasks` is a separate table from notice `transport.driver_task`. One row is created per driver-visible execution stop in the same transaction as the transport-execution projection. Status and version mirror the authoritative stop. A position anchor is not tasked. An end stop is tasked only when its location differs from the last cargo stop. `shipment_id` is set only when every cargo action on the stop belongs to one shipment.

`GET /v1/driver/me/stops` returns at most the current open stop and the following driver-visible stop on the active revision. Mutating routes are `arrive`, `start-service`, `complete`, action `confirm`, and action `fail`. They call the existing 0.1B command service. The client does not send execution, revision, tenant, or driver identity. Confirm resolves pickup or delivery on the server. Fail uses the existing driver exception category catalogue.

Gateway routes are `/api/v1/driver/me/stops` and the matching commands. They reuse the driver handler, client, and DRIVER role check. With auth enabled, identity comes from the verified token. Mutating calls require `Idempotency-Key` and use the 0.1B database command idempotency record.

`POST /v1/driver/me/shipments/{shipmentId}/events` with `DEPARTED_PICKUP` dispatches the in-process departure command when the shipment is an active participant of an execution assigned to the resolved driver in the verified operating tenant. A shipment that is not a participant keeps the existing single-leg status update. There is no new depart route.

No route-level driver or vehicle reassignment command exists, so planned-task transfer is not implemented. Route-level participant cancellation is not implemented. Completed task rows are retained. Notice task routes, the notice table, and the notice acknowledgement states are unchanged.

Migration: `000090_tms_driver_stop_tasks_v0_1c`. Down drops only `transport.driver_stop_tasks`.

Tracking, Control Tower execution projection, successor replan, and NLO-0.4D are not part of this wave.
