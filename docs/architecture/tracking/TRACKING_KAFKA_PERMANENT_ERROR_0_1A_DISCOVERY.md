# TRACKING-KAFKA-PERMANENT-ERROR-0.1A — Discovery (R1)

Status: documentation only. Product code, migrations, Kafka settings, and deployment are unchanged.

Source: GitHub issue #344. Previous accepted stage: `TMS-STOP-EVENT-TRACKING-CONTRACT-0.9B` (PR #342). This revision applies independent-review findings `TKPE-I-001` through `TKPE-I-005`.

| Pin | Value |
|---|---|
| Baseline `origin/main` | `f58f13b5b4b539895673a77ff404ab47669dc96d` |
| Issue #344 | OPEN |
| Migration `000091` | Present. Not modified. No new migration file. |
| Delivery claim | At-least-once. Exactly-once is not claimed. |
| Production consumer | `PRODUCTION_CONSUMER_READINESS=BLOCKED` |
| Operator recovery | Not provided by 0.1A or by the 0.1B boundary in this document. |

Kafka transport for this consumer was not executed. Full Order → Delivery end-to-end remains unproven.

## Findings closure

| ID | Severity | Controller decision | R1 document | Proven by an integration test |
|---|---|---|---|---|
| `TKPE-I-001` | HIGH | `PARTITION_POLICY=CONTIGUOUS_HOLD` | Supersedes the earlier same-partition bypass. A later offset is not processed or committed while any earlier offset on that partition lacks a terminal result. | No. Required before implementation acceptance. |
| `TKPE-I-002` | HIGH | `PERSISTENT_FENCE=REQUIRED` | Fence is an open quarantine row. Clearance without audit is forbidden. Unknown execution identity parks the partition and does not commit. | No. Schema is documentation only. |
| `TKPE-I-003` | HIGH | `QUARANTINE_CONFLICT_POLICY=VERIFY_PAYLOAD` | Same source key compares SHA-256 and size. Mismatch parks. Original bytes are not overwritten. | No. |
| `TKPE-I-004` | MEDIUM | `CONTEXT_401_403_POLICY=PARK_AND_ALERT` | 401 and 403 do not quarantine-and-continue. 404 stays the current domain result. | No. |
| `TKPE-I-005` | MEDIUM | `PARK_SCOPE=PARTITION` plus the retry profile below | One parked partition does not stop the others. `REBALANCE_DESIGN=BLOCKED`. | No. |

Controller decisions are the five named policies, the retry numbers, and the rule that 0.1B does not delete quarantine rows or clear a fence automatically.

Project assumptions, not controller decisions:

- Overall processing deadline of 8 seconds, chosen so it sits inside the franz-go v1.21.5 default rebalance timeout of 60 seconds and session timeout of 45 seconds.
- Poll batch cap of 10 records while a rebalance design is still blocked.
- Leader epoch, when the fetched record has one, is an extra mismatch signal. It is not part of the idempotency key.
- The non-stop allowlist in section 7 is taken from the current shipment publisher. A newly introduced event type is unknown until this list is updated.

Invariants that remain unproven until a future integration run are marked in sections 6, 9, and 10. This document does not claim them as implemented.

## 1. Baseline

Repository: `f9999737292-bit/Freihgt-platform`.

`origin/main` at discovery equals the required base. PR #342 is merged. Its parents are `6e81da0541977c1e7811d8f6e382e37f86041b57` and `7dc6325a7aaed6c1f9d0c213d0e7f95d90d22f16`.

The execution consumer starts only when broker addresses and the shipment internal URL are both set (`services/tracking-service/cmd/server/main.go`). Compose files inspected for this discovery do not set `TRACKING_KAFKA_BROKERS` or `TRACKING_SHIPMENT_INTERNAL_URL`.

Client library: `github.com/twmb/franz-go v1.21.5`, with `kgo.DisableAutoCommit()`. `CommitRecords` commits `offset + 1`. Default group: `tracking-execution-stop`. Default topic: `shipment.status.v1`.

The shipment outbox publishes transport-execution events with the Kafka key set to the execution id. Shipment-status and execution events use the shipment status topic. Driver events are sent to `SHIPMENT_KAFKA_DRIVER_TOPIC`, default `driver.events.v1`, and are not part of the normal execution-topic stream. This discovery does not change those routes.

`TestStopEventTrackingContract09B` remains the proof that real stop outbox bytes can pass through `ExecutionConsumer.Apply`. It is not a Kafka consumer test. A later implementation must keep it passing.

Still open and untouched: issue #345, `CT_MUTATION_EQUALS_VIEW`, `SHIPMENT_TRUSTS_GATEWAY_TENANT_HEADER`.

## 2. Current failure mechanism

Observed path at the baseline SHA:

1. `Run` calls `PollFetches` and does not consult `fetches.Err()`.
2. `consumeBatch` groups by topic and partition, sorts each group by offset, and walks that order.
3. `isExecutionStopEvent` accepts five `shipment.route_stop.*` types. JSON parse failure returns false.
4. A false filter result is committed. A true result enters `applyUntil`, which retries `Apply` every 50ms until success or context cancellation.
5. One returned error ends `Run` for every assigned partition. There is no per-partition park state.
6. No `OnPartitionsRevoked`, `OnPartitionsLost`, or `BlockRebalanceOnPoll` is registered.

`Apply` opens one transaction after parsing. The inbox insert uses `ON CONFLICT (event_id) DO NOTHING`. `uq_execution_event_inbox_sequence` is `(execution_id, event_sequence)` and is not covered by that clause. The operating-tenant comparison runs only after the insert succeeds. A sequence conflict rolls the transaction back, leaves tracking state unchanged, and is retried forever. The offset stays uncommitted, and the process does not poll again.

Invalid JSON is a separate defect: the filter treats it as irrelevant and commits it.

`repository.IsUniqueViolation` matches every `23505` and ignores `ConstraintName`. The execution consumer does not call it. A later implementation must not use it as a reason to skip an event.

## 3. Error taxonomy

| Class | Recognition | Handling |
|---|---|---|
| `TRANSIENT` | Database connection, timeout, serialization `40001`, deadlock `40P01`; context network error, timeout, HTTP 408, 429, or 5xx | Retry inside the budget in section 11. No quarantine. No commit. No state change. |
| `DUPLICATE` | Inbox insert affects zero rows because `event_id` already exists | Terminal success. Commit that offset. Do not change tracking state. |
| `FILTER_SKIP` | Positively identified known non-stop event, section 7 | Terminal for the cursor. No `Apply`. No state change. |
| `PERMANENT` | Stop event with a known execution id and sequence, and a permanent validation or same-tenant sequence conflict | One durable quarantine row, fence `OPEN`, then commit only that offset. |
| `SECURITY_INVALID` | Same sequence owned by a different `operating_tenant_id`, or a forbidden private field on a parsed stop event | Same durable fence path as `PERMANENT`, plus a security alert. No state change. |
| `CONTEXT_AUTH` | Shipment context HTTP 401 or 403 | `PARK_AND_ALERT`. No quarantine. No commit. No state change. Not proof of a bad payload. |
| `UNKNOWN_IDENTITY` | Malformed JSON, ambiguous envelope, unknown type, or a stop event whose execution id or sequence cannot be parsed | Store original bytes when the quarantine insert succeeds. Do not commit. Park that partition. |
| `QUARANTINE_MISMATCH` | Same source key, different SHA-256, size, or leader epoch | Do not overwrite. Park. Alert. No commit. |
| `CANCELLED` | Consumer context done | Stop every partition without committing the in-flight offset. |
| `UNKNOWN` | Any other error, including `23505` on a constraint this contract does not name, and a sequence conflict whose inbox row is not visible after rollback | Budgeted retry, then park. No commit. Do not record it as permanent. |

`PERMANENT` and `SECURITY_INVALID` are the only `Apply` failures allowed to leave the stream, and only after the fence row commits and the execution id and sequence are both known.

Sequence conflict, after rollback, in a new read:

| Existing inbox row | Class |
|---|---|
| Same `event_id` | `DUPLICATE` |
| Same operating tenant, different `event_id` | `PERMANENT` |
| Different operating tenant | `SECURITY_INVALID` |
| No row | `UNKNOWN` |

Context HTTP 404 stays the current domain result: inbox commit, no state update, consumer success. It is not `CONTEXT_AUTH` and not `TRANSIENT`.

`CommitRecords`:

| Outcome | Policy |
|---|---|
| Nil error | This process observed a commit of `offset + 1`. Not exactly-once. |
| Definite broker or group error | Do not advance. Redelivery is expected. |
| Timeout or canceled call | Ambiguous. Do not start a later offset. Restart must be idempotent. |
| Commit of an older offset after another member committed a newer one | Forbidden. Cursor rewind is a defect. |

A budget expiry is `TRANSIENT_BUDGET_EXHAUSTED=PARK_AND_ALERT`. It does not reclassify the error as permanent and it does not commit.

## 4. Quarantine

Option A, a Kafka dead-letter topic, remains rejected as the primary store. It adds a shared-broker copy, it is not atomic with the source offset, and topic ACLs are a poor tenant boundary. No new topic is required.

Option B is the required store. The Control Tower table `control_tower.shipment_status_event_dead_letter` is the precedent for "commit only after the insert" and for a unique source position. Tracking does not copy its hash-only shape. Investigation needs the original bytes, under a different role from the consumer.

### 4.1 `tracking.execution_event_quarantine`

Not created in this stage.

| Column | Rule |
|---|---|
| `id` | Surrogate primary key. |
| `consumer_group`, `topic`, `partition_id`, `message_offset` | Idempotency key. Unique together. |
| `leader_epoch` | Nullable. Compared on replay. Not part of the unique key. |
| `payload` | Original bytes. Investigator role only. |
| `payload_sha256`, `payload_size` | Required. Consumer may read these. |
| `event_id`, `execution_id`, `event_sequence`, `event_type`, `operating_tenant_id` | Nullable claims. Not authorization. |
| `error_class`, `error_code`, `constraint_name` | Safe classification. |
| `fence_status` | `OPEN` or `CLEARED`. Null when this row is not an execution fence. |
| `fence_opened_at` | Set in the same insert that opens the fence. |
| `fence_reason`, `fence_source` | Why and which classifier path opened it. |
| `received_at` | UTC time of first acceptance of these bytes. |

Check: `fence_status`, `fence_opened_at`, `execution_id`, and `event_sequence` are all present, or all absent. A row with a parsed execution identity is a fence. A row without that identity is evidence for a parked partition, not a terminal result.

Index for the consumer check: `(execution_id, event_sequence)` where `fence_status = 'OPEN'`.

Opening insert is one transaction: the quarantine row with `fence_status = OPEN`, plus one append-only audit row. If either write fails, both roll back and the offset stays uncommitted.

### 4.2 Replay of the same key

1. Read the stored SHA-256, size, and leader epoch. Do not read `payload` with the consumer role.
2. Compare them with the fetched record.
3. Equal hash and size, and equal epoch when both sides have one: the stored row is the same record. Do not write again.
4. Commit only when that stored row is a terminal fence (`OPEN` or already `CLEARED`) for a known execution id and sequence, or when the original class was a verified duplicate or filter-skip that did not need a row. A matching unknown-identity row is not terminal: stay parked and do not commit.
5. Any difference: `QUARANTINE_MISMATCH`. Do not update `payload`, hash, size, or epoch. Park the partition, alert, and do not commit. An audit row may record the presented hash and size. If that audit insert fails, the park still stands.

Topic deletion and recreation can reuse partition offsets. The unique key then collides with the old row. Different bytes or a different leader epoch take the mismatch path and preserve the first payload. Byte-identical reuse is indistinguishable from a true replay; the epoch comparison is the additional signal when the broker supplies it. This collision behavior is not tested here.

### 4.3 Roles

| Role | Allowed |
|---|---|
| Consumer | `INSERT` of a new quarantine row and its opening audit row. `SELECT` of metadata, hash, size, epoch, fence status, execution id, and sequence. No `UPDATE`, no `DELETE`, no payload column. |
| Investigator | `SELECT` of `payload` for an authorized investigation. No clearance. |
| Customer API | No privilege on either table. |
| Clearance | Future. Not granted to the consumer. Inserts audit and changes `fence_status` only in the clearance transaction defined below. |

Automatic deletion is forbidden in 0.1B. Retention is an operator decision after a future clearance stage exists.

### 4.4 Audit

`tracking.execution_event_quarantine_audit`, append-only:

| Column | Rule |
|---|---|
| `id` | Surrogate key. |
| `quarantine_id` | The quarantine row. |
| `action` | `OPENED` in 0.1B. `CLEARED` and replay actions exist only in a later stage. |
| `actor_id` | Authorized operator for clearance. Opening uses a system actor recorded as such. |
| `acted_at` | UTC. |
| `reason` | Safe reason code. |
| `previous_state`, `new_state` | Fence status transition. Opening goes from empty to `OPEN`. |
| `investigation_ref` | Required for clearance. Null on system open. |

No `UPDATE` or `DELETE` on this table.

Clearance, when a future stage implements it, is one transaction: insert the audit row with the operator, reason, previous state, new state, and investigation reference; then set `fence_status = CLEARED`. If the audit insert fails, the fence update rolls back. Clearance without an audit row is forbidden. Automatic clearance is forbidden. 0.1B does not implement clearance or manual replay, so this document does not claim operator recovery.

## 5. Execution fence

`PERSISTENT_FENCE=REQUIRED`.

An open row whose `execution_id` and `event_sequence` are both non-null forbids `Apply` of any later event for that execution whose sequence is greater. The consumer runs this check before every `Apply`, including after restart.

The fence does not forbid a different execution. Contiguous hold, section 6, still forbids reaching that other execution by stepping over an unresolved offset.

Restart reads open fences from PostgreSQL before applying the record at the current Kafka cursor. An in-memory set is not the source of truth.

Unknown execution identity uses `UNKNOWN_EXECUTION_IDENTITY=PARK_PARTITION`. The original bytes may be stored. The offset is not committed, because a missing identity does not prove that a later record is independent.

## 6. Offset and partition contract

`PARTITION_POLICY=CONTIGUOUS_HOLD`.

A Kafka offset N may be committed only after every offset below N on that same topic and partition has a terminal result:

- confirmed `Apply` success, including the current HTTP 404 context result;
- verified `DUPLICATE`;
- verified `FILTER_SKIP`;
- confirmed durable quarantine for a parsed execution event that is allowed to leave the stream.

`CONTEXT_AUTH`, `UNKNOWN_IDENTITY`, `QUARANTINE_MISMATCH`, `UNKNOWN`, budget exhaustion, and an open fence blocking the current record are not terminal.

Example, one partition:

| Offset | Record | Result |
|---|---|---|
| 10 | Permanent sequence conflict, identity parsed | Quarantine and fence commit. Offset 10 is then committed. Cursor becomes 11. Tracking state unchanged. |
| 11 | Later sequence of the same execution | Fence check fails before `Apply`. No inbox write, no commit. |
| 12 | Different execution | Not applied and not committed. The consumer must not step over offset 11. |

Other partitions may continue.

Second order, for the test matrix: offset 10 terminal quarantine, offset 11 a different execution with no fence, offset 12 a greater sequence of the fenced execution. Offset 11 may reach its own terminal result and be committed. Offset 12 is then held. The cursor does not move past 12.

| Scenario | Committed cursor | PostgreSQL | Restart |
|---|---|---|---|
| Apply success | That offset plus one | Inbox and allowed state update | No redelivery if the commit sticks. Duplicate `event_id` if it does not. |
| Verified duplicate or filter-skip | That offset plus one | Duplicate leaves existing rows. Filter-skip writes nothing. | Same idempotent replay. |
| Terminal quarantine | That offset plus one, only after the fence insert commits | One open fence. Tracking state unchanged. | Hash match. Do not insert again. Commit if still uncommitted. |
| Fence blocks the next record | Previous terminal offset only | Fence remains `OPEN` | Replay starts at the blocked offset. Check the fence before `Apply`. Do not commit it. |
| Crash after fence insert, before commit | Previous cursor | Fence row present | Hash match, then commit. No second payload. |
| Unknown identity | Unchanged | Optional evidence row, no fence | Hash match does not authorize commit. Partition stays parked. |
| Hash or epoch mismatch | Unchanged | Original payload kept | Park and alert again. |
| 401 or 403 | Unchanged | No new tracking or quarantine row | Replay the same offset. Park again while the status remains. |
| Transient outage or budget exhaustion | Unchanged | No terminal row | Restart receives a new retry budget. Not marked permanent. |
| Commit timeout | Unknown | Apply or fence result already stored | Idempotent replay. Do not assume the broker stored the commit. |
| Context canceled | Unchanged for the in-flight record | No partial fence | Restart replays the uncommitted offset. |
| Rebalance | Only commits that finished while ownership was still held | Preserved | Uncommitted records are replayed by a current member. Rewind is forbidden. Not proven. |

`PARK_SCOPE=PARTITION`.

Each topic and partition has its own parked flag. A parked partition receives no `Apply` and no commit. Other assigned partitions continue. Cancellation clears in-flight work on all partitions and commits nothing that has not already returned from `CommitRecords`. The blocked set is a metric. After restart, open fences and non-terminal quarantine rows are read from PostgreSQL, and the Kafka cursor is the uncommitted offset. Transient parks are not durable: the uncommitted cursor is what causes the record to be seen again.

The current 50ms infinite loop is not the retry policy.

## 7. Shared-topic filter

The execution consumer reads `shipment.status.v1`. Publisher behavior at this baseline:

- Shipment status body is `ShipmentStatusEventEnvelope`: `eventType`, `eventId`, `tenantId`, camelCase. Header `event_type` carries the outbox type. Known types: `shipment.created`, `shipment.status.changed`, `shipment.cancelled`, `shipment.ready_for_billing`, `shipment.documents_completed`, `shipment.financially_closed`.
- Execution body uses snake_case `event_type`. The five types `Apply` accepts are `shipment.route_stop.current`, `arrived`, `service_started`, `completed`, and `sequence_overridden`.
- Other execution types published on this topic include `shipment.execution_plan.created`, `shipment.execution_plan.superseded`, and the delivery and cargo types accepted by `IsExecutionKafkaEventType`. They are known non-stop events.
- Driver envelopes use camelCase `eventType` and are published to `driver.events.v1`. Their appearance on `shipment.status.v1` is not a normal route.

A verified filter-skip requires all of:

1. The value is a JSON object.
2. Header `event_type` is present.
3. The body type field for that family agrees with the header. Shipment status uses `eventType`. Execution events use `event_type`.
4. The type is on the known non-stop list above.
5. The record is not one of the five stop types.

Malformed JSON, a non-object, a missing or disagreeing type, an unknown type, or a driver envelope on this topic is `UNKNOWN_IDENTITY`. Park the partition. Bytes may be stored. Do not commit. Do not treat the record as irrelevant.

The five stop types continue into classification and `Apply`. This section does not change shipment, driver, or execution publish routes.

## 8. Tenant isolation

A payload `operating_tenant_id` is a claim. It is not an authenticated tenant.

The existing `Apply` guard stays: a different operating tenant does not update `execution_tracking_state`. A sequence conflict never reaches that guard today. The quarantine path must not update state either.

A foreign tenant on the same sequence is `SECURITY_INVALID`: fence, alert, no state change, no customer-API payload. A foreign tenant with a free sequence still follows current `Apply` behavior. Changing that path is not part of this remediation.

Quarantine and audit are not a replay mechanism. Customer APIs receive no grant. Logs and metrics omit payload, secrets, coordinates, and tenant id labels.

## 9. Rebalance

`REBALANCE_DESIGN=BLOCKED`.

The preferred candidate is `BlockRebalanceOnPoll`, a short batch, and `AllowRebalance` on every exit. It is not accepted as safe. `franz-go` v1.21.5 documents the option and also documents that a member which blocks rebalance past the rebalance timeout can be removed. The library does not enforce an application deadline. Default session timeout is 45 seconds and default rebalance timeout is 60 seconds. A commit racing that removal, and a possible rewind of another member's cursor, is not disproven by the presence of the API. This stage runs no integration test.

Required properties, all unproven:

- The time a poll result may block rebalance has a hard upper bound.
- Database and HTTP calls observe that deadline and abort.
- `AllowRebalance` runs on every return, including errors.
- After the member no longer owns the partition, that member does not commit a new offset for it.
- Another group member continues from the last committed cursor.
- The committed cursor is not rewound.

Alternative, also unproven, and the design that stays on record while the candidate is blocked:

- Do not enable `BlockRebalanceOnPoll`.
- `OnPartitionsRevoked` and `OnPartitionsLost` cancel the context for the lost partitions and increment an assignment generation.
- `CommitRecords` is issued only for a record whose partition is still in the generation captured when it was polled, and only after a terminal result.
- The revoke and lost callbacks do not commit. Work that finished during the callback stays uncommitted and is replayed.
- A stale generation cannot commit. That removes the path where an old member writes an older offset over a newer owner's commit.

Neither design is cleared for 0.1B implementation until a targeted review accepts one of them. Retry bounds in section 11 are necessary for either design and are not sufficient proof.

## 10. Test matrix

No production tests are added in this stage. `TestStopEventTrackingContract09B` must remain successful later. Expected outcomes:

| Case | Apply | Quarantine | Commit | Fence | Restart |
|---|---|---|---|---|---|
| Poison 10, dependent 11, independent 12 | 10 no state change; 11 and 12 not called | 10 open fence | 10 only | Open for that execution | Cursor at 11; 11 and 12 still uncommitted |
| Poison 10, independent 11, dependent 12 | 10 no state change; 11 normal; 12 not called | 10 open fence | 10 and 11 | Open | Cursor at 12; 12 held |
| Crash between fence insert and commit | Not repeated as a new state change | One row | After restart, once | Open | Hash match, then commit |
| Restart with an open fence | Blocked before `Apply` for a greater sequence | Existing row | Not the blocked offset | Still open | Same hold |
| New offset, same execution sequence | No state change | Terminal fence after classification | That offset only | Open | Replay is hash match |
| Hash and size match | No | No second row | Only if the stored row is terminal | Unchanged | Idempotent |
| Hash or size mismatch | No | Original payload unchanged | No | Unchanged | Park and alert |
| Foreign operating tenant, same sequence | No state change | Security fence | That offset | Open | No foreign state |
| Invalid JSON, no execution identity | No | Evidence row allowed | No | None | Partition remains parked |
| Context 401 and 403 | No | None | No | None | Replay; park again if the status remains |
| Context 404 | Current inbox success | None | Yes | None | Duplicate path |
| PostgreSQL unavailable | Retried, then park | None | No | None | New budget; not permanent |
| Commit timeout | Unchanged by the commit call | Existing terminal row if already inserted | Unknown | Unchanged | Idempotent replay |
| Rebalance and partition loss | No commit after loss | Only rows already committed | No new offset by the old member | Unchanged | Other member continues. Not proven. |
| Two partitions, one parked | The other partition proceeds | Only on the parked cause | Only terminal offsets on the healthy partition | Per execution | Park is per partition |
| Transient budget exhausted | No | None | No | None | Not classified permanent |
| Fence clearance without an authorized audit | No | Unchanged | No | Stays `OPEN` | Still held |
| Audit insert failure while opening or clearing | No state change | Opening rolls back; clearance does not change status | No | Previous status | Retried or still open |
| Unresolved earlier offset | Later `Apply` not called | None for the later offset | Later offset not committed | Unchanged | Cursor remains at the gap |

## 11. Retry, metrics, and operations

Design defaults for a later compatibility check against processing time and rebalance. They are not a tuned production profile.

- At most 5 attempts for one record.
- Initial delay 200ms.
- Exponential backoff.
- Maximum delay 2 seconds.
- Jitter of ±20 percent.
- One overall deadline for that record's processing. The working assumption is 8 seconds.
- On expiry: park that partition and alert. Do not commit. Do not mark the error permanent.

There is no 50ms infinite loop.

Metrics, labels limited to `result`, `error_class`, `error_code`, and `topic`:

| Meter | Use |
|---|---|
| `tracking_execution_consumer_processed_total` | Terminal success, duplicate, or verified filter-skip |
| `tracking_execution_consumer_retry_total` | Transient attempts |
| `tracking_execution_consumer_permanent_error_total` | Terminal permanent and security classifications |
| `tracking_execution_consumer_quarantine_write_total` | Confirmed inserts, including idempotent matches |
| `tracking_execution_consumer_quarantine_write_failed_total` | Failed inserts |
| `tracking_execution_consumer_offset_commit_failed_total` | Commit errors |
| `tracking_execution_consumer_partition_blocked` | Gauge of parked topic partitions |
| `tracking_execution_consumer_oldest_unresolved_age_seconds` | Oldest uncommitted or unresolved record |

Do not add `event_id` or `tenant_id` labels. Do not reuse `tracking_ingest_*`. Lag remains optional until a low-cardinality source exists. The unresolved-age page threshold is an open operations number.

Alerts: quarantine insert failures, a parked partition, `SECURITY_INVALID`, `CONTEXT_AUTH`, hash mismatch, and continued commit failure page an operator. A single transient retry does not.

Logs: group, topic, partition, offset, class, code, constraint name, hash, and size. No payload and no credentials.

Operator actions that 0.1B does not implement: reading payload, clearing a fence, and replaying a record. Those require the clearance role, an investigation reference, and a successful audit insert. Until that stage exists, a parked partition stays parked.

## 12. Scope of 0.1B

A later implementation task may include:

- classifier, contiguous hold, and per-partition park;
- the quarantine and audit tables after a fresh migration-head check;
- hash, size, and epoch verification without overwrite;
- the execution-fence read before `Apply`;
- the shared-topic filter in section 7;
- 401/403 park-and-alert;
- the retry budget in section 11;
- metrics and safe logs;
- the tests in section 10, with `TestStopEventTrackingContract09B` still passing.

0.1B does not include:

- automatic or unaudited fence clearance;
- manual replay or a claim of operator recovery;
- automatic quarantine deletion;
- a new Kafka topic or a change to publisher routing;
- a change to `Apply`'s tenant guard or inbox SQL;
- issue #345;
- deployment, or enabling the production consumer;
- an accepted rebalance implementation while `REBALANCE_DESIGN=BLOCKED`.

Issue #344 stays open until the implementation is reviewed against this contract.

## 13. Production blockers

- `PRODUCTION_CONSUMER_READINESS=BLOCKED`.
- The baseline consumer still retries a sequence conflict forever and still commits invalid JSON.
- `REBALANCE_DESIGN=BLOCKED`.
- Persistent fence, hash check, and contiguous hold are specified and not implemented.
- Exactly-once is not claimed.
- No deployment is authorized.
