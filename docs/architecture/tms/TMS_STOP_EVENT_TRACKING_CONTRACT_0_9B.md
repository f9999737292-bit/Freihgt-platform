# TMS stop-event tracking contract 0.9B

Stage: `TMS-STOP-EVENT-TRACKING-CONTRACT-0.9B`

Mode: `LOCAL_CONTRACT_TESTS_ONLY`

This stage proves one pickup-and-delivery execution: shipment-service writes the execution outbox, and those payload bytes are passed to tracking `ExecutionConsumer.Apply`. No Kafka broker, live stand, new migration, deployment, or merge is part of this proof.

## Baseline

| Field | Value |
| --- | --- |
| Branch | `test/tms-stop-event-tracking-contract-v0.9b` |
| BASE_SHA | `1902a2b4a77e9f2dcf823dae34f85328e8903117` |
| EXPECTED_BASE | `1902a2b4a77e9f2dcf823dae34f85328e8903117` |
| MAIN_AT_LOCAL_START | `1902a2b4a77e9f2dcf823dae34f85328e8903117` |
| MAIN_AT_CLOSEOUT | `67d2f394aa838ff259b65c628713cdf87ffb22d4` |
| REBASE | NO |
| KAFKA | NOT_STARTED |
| NEW_MIGRATION | NO |
| DEPLOYMENT | NOT_RUN |
| MERGE | NOT_RUN |

The isolated test database applies the existing `000091_tms_execution_stop_tracking_v0_1d.up.sql` after the shipment harness migrations. That is the current tracking inbox and execution-target schema. It is not a new migration.

## Identifier semantics

`payload.event_id` and the outbox identity are different values.

`emitExecutionEvent` puts `uuid.NewString()` in the JSON field `event_id`. It then creates a second UUID and stores that second value as both the outbox row `id` and `source_event_id`. The contract test reads both columns and the payload. It records `payload_event_id_equals_outbox_row_id=false` and `source_event_id_equals_outbox_row_id=true`. Tracking deduplicates on `payload.event_id`, not on the outbox row id.

- `event_sequence` is `transport_executions.event_seq` after that statement increments it. The first row for a new execution is sequence 1 and its type is `shipment.execution_plan.created`, not the pickup current event.
- `aggregate_id` for these execution events is the execution id. `aggregate_version` is the revision version.
- Stop-event payloads also contain `operating_tenant_id`, `execution_id`, `revision_id`, `revision_version`, `stop_id`, `action_id`, `occurred_at`, and `skipped_ordinals`.
- They do not contain `shipment_tenant_id`, `price`, `rate`, or `capacity_snapshot`.
- `shipment.execution_plan.created` has no `stop_id`. The tracking consumer filter does not pass it to `Apply`. The contract test leaves it out of the tracking inbox.

`driver.problem.reported` is a different envelope. Its JSON uses `eventId`, `eventType`, `tenantId`, and `reasonCode`. Its outbox aggregate is the shipment, not the execution.

## What the test calls

`services/shipment-service/internal/integration/transportexecution/stop_event_tracking_contract_test.go` creates the execution with the existing projection fixture and the real command service. After each command it reads `transport.shipment_event_outbox.payload` and passes those bytes to `ExecutionConsumer.Apply`.

The stop context is the shipment HTTP handler `GET /internal/v1/transport-executions/{executionId}/stops/{stopId}/tracking-context`, served by `httptest` from the same database. `Apply` and the event parser are not stubbed.

Go does not allow the shipment module to import `tracking-service/internal`. `services/tracking-service/executioncontract` is a direct call to `NewExecutionConsumer` and `Apply`. The tracking server does not import that package. It does not add an event type.

The domain fixture has a position-anchor START stop before pickup. The real operator `SKIP_STOP` with reason `ROUTE_BLOCKED` runs before the driver can arrive at pickup. Pickup and delivery then follow ARRIVE, START_STOP_SERVICE, confirm, and COMPLETE_STOP. No numeric ETA is required. The assertion is the live ETA stop id on `tracking.execution_tracking_state`.

## Contract test

Command:

`go test -tags=integration -count=1 -timeout 20m -v -run TestStopEventTrackingContract09B ./internal/integration/transportexecution`

Working directory: `services/shipment-service`

Exit code: 0

| Subtest | Result |
| --- | --- |
| PICKUP_DELIVERY_SEQUENCE | PASS |
| OUT_OF_ORDER_PREDECESSOR | PASS |
| FAIL_DELIVERY_ACTION | PASS |
| COMMAND_OUTBOX_ATOMICITY | PASS |

Observed behavior:

- In-order stop payloads are accepted. After pickup completion, the current stop and live ETA stop are the delivery stop. The delivery `shipment.route_stop.completed` payload clears that target before a later current event can select the next open stop. The initial inbox row remains.
- Re-applying the same delivery-current bytes does not change state, sequence, or inbox count.
- Replaying the pickup arrive command with the same idempotency key does not increment `event_seq` or add an outbox row.
- Foreign tenant: a synthetic copy keeps the real `execution_id` and `event_sequence`, replaces `event_id`, and sets another `operating_tenant_id`. The inbox unique key is `(execution_id, event_sequence)`, not the tenant. The insert fails with `uq_execution_event_inbox_sequence` before `Apply` reaches its operating-tenant comparison. The transaction rolls back. State and inbox are unchanged. The foreign tracking-context request returns 404 and does not include the participant tenant, `pointKind`, or `liveEta`. This does not prove JWT to gateway to shipment-service.
- Out of order: a real older pickup-current payload is applied after newer events from the same execution. Its sequence is not already in the inbox, so the inbox insert succeeds. `Apply` then sees `event_sequence` below `last_execution_event_sequence` and commits without changing the current stop, the live ETA stop, or the sequence. Nothing queues that predecessor for later application. There is no reorder buffer.
- `FAIL_ACTION` with reason `CARGO_ISSUE` stores `driver.problem.reported` and `reasonCode`. It does not emit `shipment.route_stop.completed`, does not confirm the action, and does not complete the stop. The driver envelope uses `eventType`, not `event_type`. The Kafka filter `isExecutionStopEvent` reads only `event_type` and accepts only the five `shipment.route_stop.*` types, so it would not deliver this payload to `Apply`. The contract test calls `Apply` directly. Rejection is inside `parseExecutionEvent`, which requires `event_id`, `operating_tenant_id`, `execution_id`, `event_sequence`, and `stop_id`. The test log is `PROBLEM_PAYLOAD_APPLY_ERROR=true`. Tracking state does not change.
- A later `COMPLETE_STOP` is allowed by the existing domain rule. The stop becomes `COMPLETED` with reason `PARTIAL`, and only then is `shipment.route_stop.completed` written. Tracking clears the delivery target when that completed payload is applied.
- A context-source error before the tracking transaction writes neither state nor inbox. A malformed payload does not change an existing ETA target.
- A forced execution-outbox insert failure rolls back the arrive command. The stop stays `PLANNED`, and `event_seq` does not advance. This uses the existing `SetExecutionOutboxInsertForTest` hook.

## NOT_RUN

| Check | Result |
| --- | --- |
| Mid-transaction fault injection after a successful inbox insert | NOT_RUN. `Apply` uses one transaction and rolls back on error. No existing hook fails the state write after the inbox insert, and incomplete coordinates are dropped before that write. No production fault hook was added. |
| `services/shipment-service/internal/integration/outbox` | NOT_RUN. That suite covers the generic shipment outbox worker, not this execution-stop payload. |
| Kafka broker, live stand, and Kafka configuration | NOT_RUN. No broker was started and no Kafka configuration file was edited. |

## Open risks

These are not closed by this test:

| Risk | State |
| --- | --- |
| `CT_MUTATION_EQUALS_VIEW` | OPEN. Control Tower view roles can still acknowledge, assign, and resolve. No RBAC change in this stage. |
| `SHIPMENT_TRUSTS_GATEWAY_TENANT_HEADER` | OPEN. Shipment-service still trusts gateway-supplied `X-Tenant-ID`. The foreign-tenant case above is a synthetic payload and an internal tracking-context request. |

## Regression

| Command | Exit | Result |
| --- | --- | --- |
| `go test -tags=integration -count=1 -timeout 40m -run "TestTransportExecutionCommands\|TestExecutionStopTrackingEvents\|TestDriverStopTasks\|TestMigration000091" ./internal/integration/transportexecution` | 0 | PASS |
| `go test -tags=integration -count=1 -timeout 40m -run TestExecutionStopTracking ./internal/integration/driverlocation` | 0 | PASS |

The first command covers execution commands, the execution-outbox payload tests, driver stop tasks, and the existing 000091 up/down test. The second covers the tracking execution consumer, including its duplicate-event and tenant-mismatch cases. Those tracking cases still use the existing hand-built payload. They are regression coverage, not this stage's producer proof.

No Kafka broker was started. Kafka configuration files were not edited. The old driver shipment-event route, POD logic, Control Tower summary, and order-execution execute route were not edited.

At closeout, `origin/main` is `67d2f394aa838ff259b65c628713cdf87ffb22d4`. The commits after the base are customer-portal docs and e2e specs. They do not touch the files in this stage. The branch was not rebased.

CI for the added `TestStopEventTrackingContract09B` hook is recorded by the closeout run. Until that run finishes, its result is `NOT_RUN`.
