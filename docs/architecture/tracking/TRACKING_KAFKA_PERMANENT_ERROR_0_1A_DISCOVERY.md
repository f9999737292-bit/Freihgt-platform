# TRACKING-KAFKA-PERMANENT-ERROR-0.1A — Discovery

Status: architecture discovery only. No product code, migration, Kafka setting, or deployment is changed by this document.

Source: GitHub issue #344. Previous accepted stage: `TMS-STOP-EVENT-TRACKING-CONTRACT-0.9B` (PR #342).

| Pin | Value |
|---|---|
| Baseline `origin/main` | `f58f13b5b4b539895673a77ff404ab47669dc96d` |
| Issue #344 | OPEN |
| Migration `000091` | Present. Not modified. |
| Delivery claim | At-least-once. Exactly-once is not claimed. |
| Production consumer | `PRODUCTION_CONSUMER_READINESS=BLOCKED` |

Kafka transport for this consumer was not executed in this stage. Full Order → Delivery end-to-end remains unproven.

## 1. Baseline

Repository: `f9999737292-bit/Freihgt-platform`.

`origin/main` at discovery start equals the required base. PR #342 is merged. The merge commit parents are `6e81da0541977c1e7811d8f6e382e37f86041b57` and `7dc6325a7aaed6c1f9d0c213d0e7f95d90d22f16`.

The execution consumer is wired only when both broker addresses and the shipment internal URL are configured (`services/tracking-service/cmd/server/main.go`). Compose files inspected for this discovery do not set `TRACKING_KAFKA_BROKERS` or `TRACKING_SHIPMENT_INTERNAL_URL`. The shipment publisher and the Control Tower consumer do use Redpanda and `shipment.status.v1`. That does not prove this tracking consumer has run against that broker.

Client library: `github.com/twmb/franz-go v1.21.5`, manual commit via `kgo.DisableAutoCommit()`.

Default group: `tracking-execution-stop`. Default execution topic: `shipment.status.v1`. That topic is shared with shipment status, driver events, and execution events. The shipment outbox sets the Kafka key to the execution id for transport-execution events (`AggregateID` is `scope.cmd.ExecutionID` in `emitExecutionEvent`).

Existing tests of the consumer loop are unit tests in `execution_event_consumer_test.go`. They cover in-offset-order retry, refusal to commit a later offset while an earlier offset is unresolved, and a second apply of the same payload bytes. They do not cover PostgreSQL `23505`, quarantine, poll errors, rebalance, or invalid JSON.

`TestStopEventTrackingContract09B` remains the proof that real shipment stop outbox bytes can be applied through `ExecutionConsumer.Apply`. It is not a Kafka consumer test. A future implementation must keep that test passing.

Adjacent issues that this stage does not close:

- #344 — permanent inbox-sequence retry. This document defines the fix contract. It does not implement it.
- #345 — test-only shipment module dependency. Out of scope.
- `CT_MUTATION_EQUALS_VIEW` — open, untouched.
- `SHIPMENT_TRUSTS_GATEWAY_TENANT_HEADER` — open, untouched.

## 2. Current failure mechanism

Observed path:

1. `Run` calls `PollFetches`.
2. `fetches.EachRecord` collects records. `fetches.Err()` is not consulted.
3. `consumeBatch` groups by topic and partition, sorts each group by offset, and walks that order.
4. `isExecutionStopEvent` accepts five `shipment.route_stop.*` types. Any other outcome, including JSON parse failure, returns false.
5. Accepted records call `applyUntil`, which calls `Apply` until it returns nil or the consumer context ends. The delay between attempts is a fixed 50ms.
6. `CommitRecords` runs only after `applyUntil` returns nil, or immediately when the filter rejects the record.
7. `CommitRecords` commits `offset + 1` for that record (`franz-go` v1.21.5). One failed commit returns from `consumeBatch`, and `Run` then leaves the loop. The process does not resume polling until it is started again.
8. There is no `OnPartitionsRevoked`, `OnPartitionsLost`, or `BlockRebalanceOnPoll`.

`Apply` (`execution_consumer.go`):

- Parses the payload before any database transaction.
- For `shipment.route_stop.current` only, calls shipment tracking context before `Begin`.
- Opens one transaction, inserts the inbox row, then updates state, then commits.
- `defer tx.Rollback` runs on every return. A failed statement aborts that transaction. Inbox and tracking state from that attempt are not committed.
- Duplicate `event_id` uses `ON CONFLICT (event_id) DO NOTHING`. Zero rows affected is treated as success and the transaction commits with no state change.
- The unique key `uq_execution_event_inbox_sequence` is `(execution_id, event_sequence)`. It is not covered by the `event_id` conflict clause.
- The operating-tenant comparison runs only after the inbox insert succeeds. A sequence conflict therefore never reaches the tenant guard.
- A foreign operating tenant with a free sequence inserts the inbox row and commits without changing tracking state.
- A sequence less than or equal to `last_execution_event_sequence` commits the inbox row and does not move state.
- Context HTTP 404 becomes `ContextNotFoundError`, clears the view, and still commits the inbox row without a state update.
- Any other context error is returned before `Begin` and is retried forever by `applyUntil`.

`repository.IsUniqueViolation` matches every PostgreSQL `23505` and ignores `ConstraintName`. The execution consumer does not call it. It must not be reused as a reason to skip an event.

Root cause confirmed for issue #344:

A second payload with the same `execution_id` and `event_sequence` but a different `event_id` fails the inbox insert with `23505` on `uq_execution_event_inbox_sequence`. Rollback leaves inbox and tracking state unchanged. `applyUntil` has no error class, so the same record is retried indefinitely. Its offset is not committed. Because `Run` does not poll again until `consumeBatch` returns, every partition assigned to that process stops moving, not only the partition that holds the conflicting record.

Invalid JSON is a separate current defect. `isExecutionStopEvent` returns false when `json.Unmarshal` fails, and `consumeBatch` then commits that offset. A corrupt stop event is acknowledged as if it were irrelevant. A well-formed stop event with a bad UUID, a private field, or an incomplete body does enter `Apply` and is retried forever. Those two outcomes must not be collapsed into one policy.

Records whose JSON parses and whose `event_type` is outside the five stop types are committed without `Apply`. On the shared topic this includes shipment status, driver events (`eventType` is camelCase, so `event_type` is absent), and `shipment.execution_plan.created`. That filter is current behavior. It is safe only when the body is valid JSON and the type is actually not a stop event.

## 3. Error taxonomy

This is the contract for a later implementation. It is not the current code.

Unknown errors are fail-closed: no offset commit, no tracking-state write, no quarantine record that pretends the error is understood.

| Class | How it is recognized | Required handling |
|---|---|---|
| `TRANSIENT` | Database connection, pool, timeout, serialization `40001`, deadlock `40P01`; context client network error, timeout, HTTP 408/429/5xx | Retry with bounded backoff. Do not quarantine. Do not commit. |
| `DUPLICATE` | Inbox insert affects zero rows because `event_id` already exists | Idempotent success. Commit the Kafka offset. Do not change tracking state. |
| `PERMANENT` | Stop-event type with invalid JSON, invalid UUID, incomplete event, or a forbidden private field | Durable quarantine, then offset commit only after that write is confirmed. Do not change tracking state. |
| `PERMANENT_CANDIDATE` | `23505` whose `ConstraintName` is `uq_execution_event_inbox_sequence` | After rollback, read the existing inbox row in a new transaction. Classify from that row. Do not skip on SQLSTATE alone. |
| `SECURITY_INVALID` | Sequence owner has a different `operating_tenant_id`, or the payload contains a forbidden private field that carries another tenant's data | Quarantine, alert, no state change, no API exposure of the foreign payload. Distinct from an ordinary permanent validation error. |
| `CANCELLED` | Consumer context canceled or process shutdown | Return without committing the in-flight offset. Redelivery is required and safe. |
| `UNKNOWN` | Any other `Apply`, database, or client error, including `23505` on a constraint this contract does not name | Bounded retry, then stop the execution fence and the partition cursor. Do not commit. Do not quarantine as a resolved permanent error. |

`PERMANENT_CANDIDATE` decision after reading the existing inbox row:

| Existing row | Classification |
|---|---|
| Same `event_id` as the incoming payload | `DUPLICATE`. The `23505` path should be rare here because `ON CONFLICT (event_id)` already covers it. |
| Same `operating_tenant_id`, different `event_id` | `PERMANENT`. Sequence is already owned. Quarantine. Do not treat it as a duplicate and do not advance `last_execution_event_sequence`. |
| Different `operating_tenant_id` | `SECURITY_INVALID`. |
| No row visible after rollback | `UNKNOWN`. Another transaction may be racing. Fail closed and retry. Do not commit. |

Context HTTP 404 stays the current domain result: inbox commit, no state update, consumer success. It is not `TRANSIENT` and not `PERMANENT`.

Context HTTP 401 and 403 are not transient. They are `PERMANENT_CANDIDATE` configuration or trust-boundary failures: quarantine only after the safe error code is `CONTEXT_UNAUTHORIZED` or `CONTEXT_FORBIDDEN`, alert, and do not commit before the quarantine write. Do not log the response body.

`CommitRecords` has its own contract:

| Commit outcome | Policy |
|---|---|
| Returned nil | The client observed a successful offset commit for `record.Offset+1`. Processing may move to the next offset. This is still not exactly-once. |
| Returned a definite broker or group error | Do not advance. Redelivery is expected. The next attempt must tolerate a duplicate `event_id` or a duplicate quarantine row. |
| Timeout or canceled context with no response | Ambiguous. Do not start the next offset in this process. After restart, both "already committed" and "redelivered" are acceptable. Both must be idempotent. |
| Commit of an offset while a later offset on the same partition was already committed by another member | Forbidden. `franz-go` documents that an older commit can rewind a newer one. |

No path may commit an offset because a message was hard to classify.

## 4. DLQ and quarantine alternatives

Both options sit outside the aborted `23505` transaction. PostgreSQL aborts that transaction. The quarantine write is a new operation after rollback.

### Option A — Kafka dead-letter topic

Produce the original bytes to a dedicated topic and wait for a broker acknowledgement before `CommitRecords` on the source topic.

| Question | Assessment |
|---|---|
| Atomicity | No transaction covers the source offset and the dead-letter produce. Crash between acknowledgement and source commit redelivers the source record. |
| Duplicate quarantine | A second produce is required to be idempotent. `franz-go` idempotent production reduces duplicates for a live producer id, but a process restart can still append a second copy unless the dead-letter consumer deduplicates on source position. |
| Quarantine unavailable | Produce error, unknown topic, or timeout. Source offset stays uncommitted. Correct fail-closed behavior, at the cost of blocking that partition. |
| Ambiguous acknowledgement | Same as an ambiguous source commit. The operator can see two copies. Replay must key off source position, not arrival order. |
| Operations | New topic, retention, ACLs, and a separate reader. The raw payload is visible to anyone who can read the topic. |
| Tenant isolation | Broker ACLs are coarse. A shared dead-letter topic mixes tenants. Logs and metrics must still omit payload and tenant labels. |
| Investigation | Good transport fidelity. Poor structured query next to `execution_event_inbox`. |
| Restart | Redelivery is safe only when the dead-letter identity is the source position. |
| Manual replay | A replay consumer is a new trust boundary and needs its own authorization. Reading the topic is not authorization to reapply. |

### Option B — PostgreSQL durable quarantine

Insert one row in a new tracking-owned table, then commit the source offset only after that insert commits.

Precedent already in this repository: `control_tower.shipment_status_event_dead_letter` is unique on `(topic, partition_id, message_offset)`, stores a payload hash rather than the raw body, and the Control Tower consumer commits the offset only after that insert succeeds. Tracking cannot copy the hash-only table. Issue #344 requires the message itself to remain available for investigation. The raw bytes belong in a restricted table, not in logs or metrics.

Proposed table, not created in this stage:

`tracking.execution_event_quarantine`

| Column | Rule |
|---|---|
| `id` | Surrogate primary key. |
| `consumer_group`, `topic`, `partition_id`, `message_offset` | Source identity. Unique together. |
| `payload` | Original record bytes. Restricted grants. Not selected by ordinary service APIs. |
| `payload_sha256` | Safe correlation value for logs and tickets. |
| `event_id`, `execution_id`, `event_sequence`, `event_type`, `operating_tenant_id` | Nullable. Filled only when parsing succeeds. Untrusted claims, not authorization. |
| `error_class`, `error_code`, `constraint_name` | Safe classification. `constraint_name` only for a known constraint. |
| `received_at` | Consumer clock, UTC. |

Idempotency key: `(consumer_group, topic, partition_id, message_offset)`.

`event_id` is not the key. Invalid JSON has none, and two offsets can repeat the same claimed id. A replay of the same Kafka position inserts zero new rows and is success. A different offset is a different source record even when the bytes match.

| Question | Assessment |
|---|---|
| Atomicity | The failed `Apply` transaction and the quarantine insert are separate. The offset commit is a third step. Idempotent insert covers the crash between insert and commit. |
| Quarantine unavailable | Insert error leaves the offset uncommitted. The same database outage also prevents a successful `Apply`, so holding the cursor does not create a second failure domain. |
| Duplicate write | `ON CONFLICT (consumer_group, topic, partition_id, message_offset) DO NOTHING`, then commit the offset. |
| Restart | Redelivery finds the row and commits the offset without a second investigation record. |
| Tenant isolation | Row stores the claimed tenant for the investigator. Application reads of tracking state stay on the existing tenant guards. Quarantine payloads are not returned by customer APIs. |
| Retention | No automatic deletion in 0.1B. Deletion is an operator decision after the investigation record is closed. |
| Operations | One new migration and one restricted table. No new topic. Queryable beside the inbox. |
| Payload exposure | Database role separation is required. Logs store hash, class, code, topic, partition, and offset only. |

### Recommendation

Use option B as the quarantine backend.

Option A keeps a second copy on the same shared broker that already carries mixed tenant traffic, and it still cannot be atomic with the source offset. Option B matches the Control Tower commit gate, keeps source identity in PostgreSQL, and fails closed when the database that owns tracking state is unavailable. The extra requirement over the Control Tower table is retained payload bytes under a restricted grant.

A later export from the table to a topic is optional and is not part of 0.1B.

## 5. Recommended architecture

1. Keep `ExecutionConsumer.Apply` as the state transition. Do not weaken its tenant comparison, its `event_id` idempotency, or its rollback.
2. Add a classifier in the consumer, not a blanket `23505` skip. Read `pgconn.PgError.ConstraintName` only after the failed transaction has rolled back.
3. Replace the unbounded `applyUntil` loop with class handling: retry `TRANSIENT`, succeed `DUPLICATE`, quarantine `PERMANENT` and `SECURITY_INVALID`, and park `UNKNOWN`.
4. Parse JSON before the relevance filter. Invalid JSON on this consumer is `PERMANENT`, not an irrelevant record.
5. Keep the current skip-and-commit behavior only for valid JSON whose `event_type` is present and is not one of the five stop types.
6. Quarantine insert uses the idempotency key above. Offset commit follows a confirmed insert or a confirmed duplicate insert.
7. Persist an execution fence: an unresolved quarantine row for `(execution_id, event_sequence)` blocks later events of that same execution whose sequence is greater. Other execution ids on the same partition may continue after the quarantined offset is committed.
8. Register a revoke/lost callback or `BlockRebalanceOnPoll`, and call `AllowRebalance` after the batch. Do not commit a partition the member no longer owns.
9. Classify `PollFetches` errors the way the Control Tower consumer already does. Back off. Do not spin and do not commit.
10. Add consumer metrics beside the existing ingest metrics. Do not reuse `tracking_ingest_*` for this path.

Retry budget numbers are an open controller decision. The shape is fixed: exponential backoff with jitter, a cap, and no quarantine while the class remains `TRANSIENT`. The current 50ms infinite loop is not the policy.

`UNKNOWN` after the bounded budget parks the partition: the process stays up, reports `consumer_partition_blocked`, and does not commit that offset. Parking is fail-closed. An infinite tight loop is not an acceptable form of fail-closed behavior because it hides the record and stalls every assigned partition.

## 6. Offset safety invariants

Invariant: a Kafka offset is committed only after `Apply` has succeeded, or after a permanent or security-invalid record has been durably inserted into quarantine. Duplicate success counts as `Apply` success. Filter-only commit is allowed only for valid non-stop JSON.

| Scenario | Stored | Offset committed | Redelivered | Redelivery safe | Tracking state | Recovery |
|---|---|---|---|---|---|---|
| 1. Apply success | Inbox and any allowed state update | Yes, that record only | Only if commit is ambiguous or a rebalance rewinds | Yes, duplicate `event_id` | Updated only by the successful transaction | None |
| 2. Transient Apply error | Nothing from the failed attempt | No | Yes | Yes | Unchanged | Retry |
| 3. Permanent Apply error, before quarantine | Nothing | No | Yes | Yes | Unchanged | Quarantine, then commit |
| 4. Quarantine insert committed | Quarantine row | Not yet | Yes if the process stops here | Yes, conflict-do-nothing | Unchanged | Commit offset |
| 5. Quarantine insert failed | Nothing new | No | Yes | Yes | Unchanged | Retry the insert. Do not commit. |
| 6. Crash before quarantine | Nothing | No | Yes | Yes | Unchanged | Process from the last committed offset |
| 7. Crash after quarantine, before commit | Quarantine row | No | Yes | Yes | Unchanged | See the row, commit |
| 8. Commit error | Apply or quarantine result already stored | Unknown | Yes when the broker did not commit | Yes | Unchanged by the commit attempt | Retry commit or reprocess idempotently |
| 9. Restart | Prior committed quarantine and inbox rows | Only previously acknowledged offsets | Uncommitted tail | Yes | Preserved | Replay from committed offset |
| 10. Rebalance during processing | Only work finished before revoke | Only records whose commit completed while the member owned the partition | Uncommitted records | Yes, if a late commit cannot rewind a newer owner | Preserved | New owner reads the last committed offset |

`CommitRecords` retries retryable group errors inside the client and returns an unretryable error. The application must still treat a returned error as "not acknowledged by this process."

Exactly-once is not provided. The group consumer is at-least-once. Idempotent inbox and quarantine keys are the recovery mechanism.

## 7. Partition ordering

`consumeBatch` already sorts one partition by offset and commits a record only after the previous record in that batch has been handled. It does not commit offset 11 while offset 10 is still inside `applyUntil`. That local property is necessary and stays.

It is not sufficient:

- A permanent error currently never returns, so later records in the same batch and later polls do not run.
- Partition map iteration stops the whole `Run` on the first returned error. One commit error stops every assigned partition.
- Records for different executions share a partition. The Kafka key is the execution id, so one execution stays on one partition while many executions can share it.
- A later event for the same execution, applied after a quarantined predecessor, can advance `last_execution_event_sequence`. The predecessor can then be inserted only as a stale inbox row and can no longer repair state. That is uncontrolled progress.

Contract:

- Do not commit a higher offset in a partition while a lower offset is neither applied nor quarantined.
- Do not drop a permanent record by committing it as irrelevant.
- Quarantine stores the source offset so an investigator can reconstruct the skipped position.
- After a quarantined event with a parsed `execution_id` and `event_sequence`, hold every later record of that execution with a greater sequence. Quarantine those too, or leave them uncommitted behind the fence. Do not call the normal state update.
- Records on the same partition for a different `execution_id` may be processed once the quarantined offset itself is committed.
- An `UNKNOWN` error, or a quarantine store that is down, blocks that partition cursor. Continuation is not allowed.
- Operator clearance of the fence is an audited action. Automatic clearance is not part of this contract.

A stale predecessor whose sequence is already stored and whose `event_id` matches remains the current duplicate or stale-success path. It is not a new gap.

## 8. Tenant isolation

Issue #344 example: one `execution_id`, one `event_sequence`, two `event_id` values, and possibly two `operating_tenant_id` values.

Observed today:

- The tenant comparison does not run when the sequence unique key fails.
- A foreign tenant with a different sequence is stored in the inbox and does not change tracking state.
- Customer tracking-context reads are a separate shipment handler. This consumer must not grow an API that returns quarantine payloads.

Contract:

- A payload `operating_tenant_id` is a claim. It is not an authenticated tenant by itself.
- Never update `execution_tracking_state` for a different operating tenant. The existing guard stays.
- Never return another tenant's payload, coordinates, or participant identity through an API as part of quarantine.
- Quarantine is not a replay path. Replay is a separate authorized and audited operation.
- `SECURITY_INVALID` alerts are security incidents. A same-tenant sequence conflict is a data-integrity incident. They do not share one silent skip.
- Logs and metrics do not contain raw payload, secrets, or tenant id labels.

This discovery does not change the foreign-tenant free-sequence behavior. Whether that path should become `SECURITY_INVALID` instead of an inbox commit is an open decision. 0.1B does not change it unless a later task says so.

## 9. Failure and restart matrix

| Case | Consumer result | State | Offset | Next process start |
|---|---|---|---|---|
| Exact duplicate `event_id` | Success | Unchanged | Commit | No redelivery when commit sticks |
| Same sequence, different `event_id`, same tenant | Quarantine | Unchanged | Commit after insert | Fence holds later sequences |
| Same sequence, foreign tenant | Security quarantine | Unchanged | Commit after insert | Alert remains until an operator closes it |
| Invalid JSON | Quarantine | Unchanged | Commit after insert | No state dependency if no execution id parsed |
| Valid non-stop type | Skip | Unchanged | Commit | Current shared-topic behavior |
| Transient database or context outage | Retry | Unchanged | Uncommitted | Continues when the dependency returns |
| Quarantine database outage | Retry insert | Unchanged | Uncommitted | Partition stays blocked |
| Crash before insert | Nothing stored | Unchanged | Uncommitted | Redeliver and classify again |
| Crash after insert | One quarantine row | Unchanged | Uncommitted | Insert is a no-op, then commit |
| Commit timeout | Ambiguous | Unchanged by the commit | Unknown | Idempotent redelivery |
| Context canceled in `Apply` or sleep | `CANCELLED` | Unchanged | Uncommitted | Restart replays |
| Rebalance before commit | No late commit after revoke | Unchanged | Uncommitted | Other member replays |
| Two partitions, poison on one | The other partition may proceed only after the poisoned offset is safely quarantined or the implementation parks only that partition | The other execution is untouched | Only safe offsets | A process-wide `Run` return is not the target design |
| Stale lower sequence after a newer applied event | Current inbox success, no state regression | Unchanged | Commit | Existing `Apply` behavior |
| Later valid event after a quarantined predecessor of the same execution | Held or quarantined as dependent | Unchanged | Not committed as a normal apply | Operator decision |
| Later valid event of another execution after a committed quarantine | Normal apply | That execution only | Commit when apply succeeds | No cross-execution fence |

Restart recovery depends on the quarantine unique key and the inbox `event_id` conflict. It does not depend on the process remembering an in-memory set.

## 10. Test plan

`TestStopEventTrackingContract09B` must stay successful. New tests do not replace it.

### Unit

- Classifier: invalid JSON, invalid UUID, private field, incomplete event, unknown stop-adjacent type, valid non-stop type.
- `23505` with `ConstraintName=uq_execution_event_inbox_sequence` is not a duplicate.
- `23505` with another constraint name is `UNKNOWN`.
- Same `event_id` is `DUPLICATE` without reading it as a sequence conflict.
- Same sequence and same tenant is `PERMANENT`.
- Same sequence and other tenant is `SECURITY_INVALID`.
- Missing inbox row after rollback is `UNKNOWN`.
- Offset commit is not called for transient, unknown, cancelled, or failed quarantine.
- Offset commit is called once after successful apply, duplicate, or confirmed quarantine.
- A later offset in the same partition is not committed first.
- A later event with the same `execution_id` is not applied while the fence exists.
- A later event with another `execution_id` is applied after the quarantined offset commits.
- Context cancellation leaves the offset uncommitted.
- `CommitRecords` error does not mark the record done.
- Invalid JSON is not passed through `isExecutionStopEvent` into a commit.

### PostgreSQL integration

- Real `23505` on `uq_execution_event_inbox_sequence`, rollback, empty state, then one quarantine row.
- Repeat of the same source position does not add a second quarantine row.
- Foreign-tenant sequence conflict does not change `execution_tracking_state`.
- Exact `event_id` replay does not change state or add a quarantine row.
- Quarantine insert failure, injected by a broken statement, leaves the offset helper uncalled.
- Transaction abort of `Apply` and the following quarantine transaction are separate.

### Kafka / Redpanda integration

Use an existing test broker pattern. Do not point tests at staging or production.

- One poison record then a valid record on the same partition: poison quarantined, valid record of another execution applied, commits in order.
- Same execution after the poison record stays unapplied.
- Two partitions: poison on partition 0 does not block partition 1 once the implementation parks per partition.
- Consumer stop after quarantine insert and before commit, then a new process: one quarantine row and a committed offset.
- Broker commit error: record is not acknowledged by the test committer.
- Group rebalance while a record is in flight: the losing member does not commit that partition afterward.
- Poll error is retried with backoff and commits nothing.
- Valid shipment-status JSON on `shipment.status.v1` is committed without `Apply`.
- Invalid JSON is quarantined rather than committed as irrelevant.

### Restart and security

- Kill the process at the points in the matrix above.
- Quarantine read API, if any is added, rejects a caller from another tenant and does not return payload bytes to an ordinary role.
- Manual replay without the authorized audit entry is rejected.
- Logs of a failed record contain the hash and source position and do not contain the payload.

Tests not run in this discovery: all of the above, plus the existing contract test. This stage did not start PostgreSQL, Redpanda, or the consumer.

## 11. Operational monitoring

Existing `tracking_ingest_*`, `eta_ingest_*`, and `slot_ingest_*` meters describe telemetry ingestion. They are not execution-consumer meters. Do not double-count this path there.

New meters, labels limited to `result`, `error_class`, `error_code`, and `topic`:

| Meter | Meaning |
|---|---|
| `tracking_execution_consumer_processed_total` | Records that reached a terminal success, duplicate, or filter-skip |
| `tracking_execution_consumer_retry_total` | Transient attempts |
| `tracking_execution_consumer_permanent_error_total` | Permanent classifications, including security |
| `tracking_execution_consumer_quarantine_write_total` | Confirmed inserts, including idempotent conflicts |
| `tracking_execution_consumer_quarantine_write_failed_total` | Insert attempts that failed |
| `tracking_execution_consumer_offset_commit_failed_total` | `CommitRecords` errors |
| `tracking_execution_consumer_partition_blocked` | Gauge of partitions parked on unknown or quarantine outage |
| `tracking_execution_consumer_lag` | Group lag when the client can report it without per-event series |
| `tracking_execution_consumer_oldest_unresolved_age_seconds` | Age of the oldest uncommitted or unresolved quarantined record |

No `event_id` or `tenant_id` label.

Alerts:

| Condition | Severity | Action |
|---|---|---|
| Quarantine write failures continuing | Page | Partition is stuck. Restore the database. Do not skip the offset. |
| Partition blocked on `UNKNOWN` | Page | Inspect constraint name and safe error code. Do not delete the record. |
| `SECURITY_INVALID` increment | Page | Security incident. Do not replay. |
| Permanent sequence conflict increment | Ticket | Data-integrity investigation. Fence stays until a decision. |
| Offset commit failures continuing | Page | Check group membership and coordinator. Assume redelivery. |
| Oldest unresolved age above the agreed threshold | Page | Consumer is not making progress. |
| Single transient retry | None | Expected during a short outage. |

The age threshold is an open operations decision.

Log fields: group, topic, partition, offset, error class, error code, constraint name, payload sha256, payload size. Omit payload, tokens, coordinates, and names.

Operator runbook:

1. Read the quarantine row by group, topic, partition, and offset.
2. Put the payload hash in the ticket. Do not paste the payload.
3. For a sequence conflict, compare the inbox owner in an authorized admin query. Check `event_id` and operating tenant. Do not expose that row through a customer API.
4. Decide whether the source event is a producer defect, a replay with a regenerated id, or a cross-tenant write.
5. Replay only through a separate authorized action that writes an audit row. Consuming the quarantine table is not replay.
6. Clear the execution fence only after that decision. Do not clear it by committing a later offset first.

## 12. Open questions

1. Transient attempt budget, initial delay, cap, and jitter. The current 50ms loop is evidence of the defect, not a chosen budget.
2. Unresolved-age page threshold.
3. Whether a foreign tenant with a free sequence should remain an inbox commit without a state update.
4. Which database role may read `payload`, and how long rows are retained.
5. The role and audit record required before manual replay.
6. `BlockRebalanceOnPoll` versus a revoke callback. Blocking is safer for commit ownership and can exceed the rebalance timeout if a database outage lasts. The implementation has to bound poll size and allow rebalance while a transient retry is waiting.
7. Whether a dependent later event is written to quarantine immediately or left uncommitted behind the fence. Both hold state. Only the first gives the operator a list of the held messages if the process is restarted after their offsets were not committed. Preferred direction: leave them uncommitted until the fence is cleared, so they can still be applied in order. Confirm with the controller.
8. HTTP 401 and 403 from the context client are proposed as permanent configuration failures. Confirm they must not be retried as transient.
9. Lag metric source. `franz-go` does not make group lag a counter that already exists in tracking. The lag series stays optional until a low-cardinality source is chosen.

## 13. Scope of 0.1B

In scope when a later task authorizes implementation:

- Error classifier and replacement of unbounded `applyUntil`.
- Invalid JSON no longer committed as irrelevant.
- New migration for `tracking.execution_event_quarantine` only, after the current migration head is read again.
- Idempotent quarantine insert and the offset invariant.
- Execution fence for a quarantined predecessor.
- Poll error backoff, commit error handling, and rebalance ownership.
- Metrics, safe logs, and the tests in section 10.
- Keep `TestStopEventTrackingContract09B` passing.
- Leave issue #344 open until that implementation is reviewed. This discovery does not close it.

Out of scope for 0.1B unless a separate task says otherwise:

- Changing `ExecutionConsumer.Apply` tenant guards or inbox SQL.
- Changing the shipment producer, event ids, or sequence allocation.
- Issue #345 and the test-only module dependency.
- A new Kafka topic, broker ACL, or compose runtime change.
- Production or staging deployment, and starting this consumer against a live broker.
- Control Tower mutation rules and shipment trust of the gateway tenant header.
- Automatic replay.

## 14. Production blockers

- `PRODUCTION_CONSUMER_READINESS=BLOCKED`.
- `KAFKA_PERMANENT_RETRY_RISK=OPEN` until 0.1B is implemented and the integration tests have actually run.
- The consumer still retries a sequence conflict forever at the baseline SHA.
- Invalid JSON is still acknowledged.
- No revoke hook is registered, so a late commit can rewind another member. That hazard is documented by `franz-go` v1.21.5 and is not tested here.
- Exactly-once is not claimed.
- No deployment is authorized by this discovery.
