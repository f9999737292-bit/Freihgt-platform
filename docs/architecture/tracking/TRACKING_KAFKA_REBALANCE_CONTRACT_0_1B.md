# TRACKING-KAFKA-REBALANCE-CONTRACT-0.1B

Status: R4 remediation after Agent J's R3 review. Production code, SQL, Kafka configuration, and migrations are unchanged. Implementation of 0.1B has not started. Broker safety is not proven. `SHIPMENT_OWNER_RACE=OPEN`. The mixed-version gate is specified and not enabled.

| Pin | Value |
|---|---|
| Base | `origin/main` `3f306fc407aeda4bdc652d825954292f4a54fad6` |
| Issue | #344, remains OPEN |
| This draft | PR #357 |
| Discovery draft | PR #346, left unchanged: OPEN, draft, head `9b480f17118985c8b4791c66039b2a0525afdb98`, base `f58f13b5b4b539895673a77ff404ab47669dc96d` |
| Current-consumer observation | PR #372, test only. It observed the unmodified consumer. It is not evidence for the mechanism in this document. |
| Library | `github.com/twmb/franz-go v1.21.5`, classic `consumer` protocol |
| `REBALANCE_DESIGN` | `SINGLE_FLIGHT_BLOCKED_COMMIT`, proposed below. `GENERATION_FENCED_MANUAL_COMMIT` is withdrawn as a sufficient control. Not proven. |
| `PRODUCTION_CONSUMER_READINESS` | `BLOCKED` |

PR #346 is not modified. Where this document disagrees with it, both drafts stay open until a controller chooses one.

## 1. Current consumer-group model

`NewExecutionEventConsumer` builds one `kgo.Client` with `SeedBrokers`, `ConsumerGroup` default `tracking-execution-stop`, `ConsumeTopics` default `shipment.status.v1`, and `DisableAutoCommit`. It does not set a balancer, session timeout, rebalance timeout, heartbeat, `OnPartitionsAssigned`, `OnPartitionsRevoked`, `OnPartitionsLost`, `OnPartitionsCallbackBlocked`, or `BlockRebalanceOnPoll`.

Library defaults at v1.21.5:

- Protocol `consumer`. Balancer `CooperativeStickyBalancer`.
- Session timeout 45s, rebalance timeout 60s, heartbeat 3s.
- Autocommit interval 5s, which does not run because autocommit is disabled.

`defaultRevoke` commits fetched offsets only when autocommit is enabled. With `DisableAutoCommit` it returns without a commit. After the callback, the library removes lost partitions from its uncommitted map. A revoke does not by itself acknowledge records the application has not passed to `CommitRecords`.

`CommitRecords` sends `offset + 1`. Its own comment says a rebalance before or during the call can duplicate records, and a later commit of an older offset can rewind a cursor another member has already advanced. `DisableAutoCommit` repeats that rewind case: member A can commit offset 30 after member B has committed offset 50.

`Run` commits a record after `applyUntil` returns nil, or immediately when `isExecutionStopEvent` returns false. `applyUntil` retries every `Apply` error every 50ms until the context ends. One error from `consumeBatch` returns from `Run` for every assigned partition. Fetch errors are not classified.

PR #372 ran this unmodified consumer against an isolated broker. A permanent `23505` was retried until cancel. The group cursor stayed uncommitted. Cooperative sticky left the only partition on the blocked member while that member was still in the group. After that member closed, the second member fetched the same record and also did not commit. That run did not issue an in-flight or slow `CommitRecords`, so it does not show what a late commit does.

Exactly-once is not claimed.

## 2. What v1.21.5 actually synchronizes

This is source reading of `pkg/kgo/consumer_group.go` and `config.go` at v1.21.5. It is not a broker result.

`CommitRecords` calls `CommitOffsetsSync`. That waits for `noCommitDuringJoinAndSync`'s read lock, then takes `syncCommitMu` so one synchronous commit's callback finishes before the next synchronous commit starts. `commit()` then runs under that read lock and under `g.mu`.

`joinAndSync` takes the write side of `noCommitDuringJoinAndSync` for the whole join and sync. A commit that already holds the read lock delays `JoinGroup`. A commit that arrives during join and sync waits. The function comment says this ordering is there to avoid `REBALANCE_IN_PROGRESS` for a commit that lands between join and sync, and `ILLEGAL_GENERATION` for a commit that was in flight with the old generation before sync finished. The comment also says a cooperative rebalance can still happen at any other moment, and that only `BlockRebalanceOnPoll` blocks the revoke side.

The generation on the `OffsetCommit` request is copied from `memberGen` inside `commit()`, while `g.mu` is held, not at the earlier application check. `Client.GroupMetadata` at v1.21.5 returns `(memberID string, generation int32)`. Outside a group those values are `""` and `-1`. The method does not return the group name. `groupMemberGen.load` is the same pair. Reading it before `CommitRecords` does not attach that pair to the RPC, and it is not proof that this member owns a partition.

On the classic protocol this client uses, `commit()` does not drop partitions that are no longer in `nowAssigned`. The filter that removes lost partitions and retries with the new epoch is the KIP-848 `STALE_MEMBER_EPOCH` path. This consumer is not on that path.

`CommitOffsets` does not cancel a prior in-flight commit. The comment says canceling the request kills the TCP connection, and the broker can then apply a later commit from a new connection before the original. A hung commit must not be canceled to make room for a replacement.

`updateCommitted` returns without updating the local map when `req.Generation` differs from the current generation. That ignores the response locally. It does not remove an offset the broker already stored.

`BlockRebalanceOnPoll` gates polls and revoke, lost, and assigned callbacks on `pollWaitMu`. A non-empty poll holds the low half of `pollWaitState` until `AllowRebalance`. A rebalance callback holds the high half and waits until that poll count is zero. `OnPartitionsCallbackBlocked` runs in a goroutine when a callback is waiting. The library text says this lets the member commit records it currently owns, and that blocking past the rebalance timeout can remove the member. The gate is local. It does not stop the coordinator from ending the session.

`defaultRevoke` remains a non-commit while autocommit is disabled. Revoke and lost callbacks in this proposal also do not commit.

## 3. Late commit

J-KRB-001 rejects a generation comparison as a sufficient mechanism. `GENERATION_FENCED_MANUAL_COMMIT` is withdrawn. Comparing the generation at poll with the generation just before `CommitRecords` misses the value the library stamps inside `commit()`, and it does not cover an RPC that is already on the wire when the member is removed.

The candidate is `SINGLE_FLIGHT_BLOCKED_COMMIT`. It is not implemented and not proven. It prevents a new commit after the application has recorded the loss. It does not retract an RPC that was already sent. v1.21.5 has no API that both leaves that RPC in place and guarantees the broker will ignore it.

### Operations

1. The client is constructed with `BlockRebalanceOnPoll` and `DisableAutoCommit`. One poll batch is in flight. The next poll waits until this batch has either finished its single commit or abandoned it, and `AllowRebalance` has been called.
2. Processing records the partitions owned at that poll. `OnPartitionsRevoked` and `OnPartitionsLost` only add those partitions to a lost set and cancel the processing context. They do not call `CommitRecords`.
3. A commit starts only when all of the following are true: the result is terminal under section 5; the partition is in the poll ownership set; the lost set does not contain it; no commit is already in flight for this client.
4. The application calls `CommitRecords` before `AllowRebalance`. A nil return is the only local acknowledgement. The generation may be logged. It is not an acceptance condition.
5. `AllowRebalance` runs after that single RPC has returned, or the client is closing. It does not run while the application intends to start a commit.
6. After the lost set contains the partition, the application does not start a commit for it. A commit that is already inside `commit()` is the hung-commit case below, not a second call.

### Locks

- Library read lock `noCommitDuringJoinAndSync` is held for the commit RPC and delays `JoinGroup`.
- Library `syncCommitMu` is exclusive for the synchronous commit callback.
- The library waits for an older in-flight commit instead of canceling it.
- The application holds one mutex across process, the commit decision, and `AllowRebalance`, so two goroutines cannot commit the same poll.
- The lost set is written by the revoke and lost callbacks and read before a commit is started. Those callbacks cannot run until `AllowRebalance` because of `BlockRebalanceOnPoll`.

### Time limits

These are design bounds, not measured coordinator behavior.

- Commit RPC budget: 2 seconds from the call. If it has not returned, the application marks that partition `commit_unknown`. It does not cancel the context and does not start another commit.
- Poll window: 8 seconds from poll to `AllowRebalance`, including the wait for an RPC that already started. The window is meant to sit inside the 45 second session timeout and the 60 second rebalance timeout. It is not approval that the coordinator will wait.
- `OnPartitionsCallbackBlocked` is a signal to abandon non-terminal work and reach the commit decision. It does not extend ownership.

### Hung commit

If the RPC is still running at 2 seconds, the database result is not treated as a Kafka acknowledgement. The event is not terminal. The in-flight request is left to finish because canceling it can reorder a later commit on a new connection.

When the response arrives:

- A per-partition group error, including `ILLEGAL_GENERATION`, `UNKNOWN_MEMBER_ID`, and `REBALANCE_IN_PROGRESS`, leaves the cursor unknown to this process. Do not retry those offsets.
- A nil error means the broker accepted the request that was already sent. This proposal does not call that safe. The isolated test in section 8 has to show whether that acceptance happened after the member lost the partition.
- If the client is closed first, the offset is unknown. Close is not followed by a replacement commit.

`updateCommitted` dropping a stale generation does not repair a stored broker offset. The next owner reads the committed cursor from the broker. Safety stays unproven until that test passes.

## 4. Quarantine classes

PostgreSQL is the quarantine store. No new Kafka topic is proposed. The table is not created here.

`tracking.execution_event_quarantine` keeps the unique key `consumer_group`, `topic`, `partition_id`, `message_offset`. It stores the original payload, SHA-256, size, and leader epoch. Epoch is compared and is not part of the key. Claimed `event_id`, `execution_id`, `event_sequence`, `event_type`, and `operating_tenant_id` are nullable and are not authorization.

J-KRB-003 splits every quarantine row into one class:

| Class | When | Commit on first sight | Commit on replay of the same hash, size, and epoch |
|---|---|---|---|
| `TERMINAL` | Section 5 has proved `COMMITTED_STATE_EFFECT` in this transaction, or `VERIFIED_EQUIVALENT_PRIOR_EFFECT` once `last_event_id` stores that proof. A duplicate hash is not enough. | Only after the in-transaction proof commits | Not until `last_event_id` is present and matches. Until then replay is `NON_TERMINAL` |
| `NON_TERMINAL` | Invalid JSON, unknown type, missing `execution_id` or `event_sequence`, foreign-tenant occupation, a legitimate event whose effect was not stored, hash or size or epoch mismatch, 401, 403, or an unknown constraint | No | No |

Invalid JSON, an unknown type, and missing execution identity stay `NON_TERMINAL` even when the bytes are stored. Replay of those bytes does not commit. The partition stays parked under `PARTITION_POLICY=CONTIGUOUS_HOLD`.

A hash, size, or epoch mismatch does not overwrite the stored payload. It is `NON_TERMINAL`, alerts, and does not commit.

The consumer may insert and may read metadata, hash, size, epoch, class, and fence columns. It may not read the payload, update, or delete. An investigator reads the payload. Customer APIs have no grant. Opening writes one append-only audit row in the same transaction. Clearance without an audit row is forbidden. This stage does not implement clearance, replay tooling, or deletion.

## 5. Atomicity and terminal results

J-KRB-R2-001. `Apply` returning nil is not a terminal result. A missing change to `execution_tracking_state` is not successful processing unless this attempt proves a previously committed acceptable effect.

`TERMINAL = COMMITTED_STATE_EFFECT OR VERIFIED_EQUIVALENT_PRIOR_EFFECT`

Every other outcome is `NON_TERMINAL` until a check in this section proves one of those two. A quarantine row, an inbox insert, and a committed database transaction are not substitutes for that proof.

J-KRB-R3-003. `COMMITTED_STATE_EFFECT` is a new effect in this transaction. All of the following are required:

1. The transaction commits. A rollback is not an effect.
2. The locked `execution_tracking_state` row changes in a way this event's operation was supposed to change it.
3. `RowsAffected > 0` for that update or upsert. `RowsAffected = 0` is `NON_TERMINAL`, including when `Apply` returns nil.
4. After the write, and before commit, the same transaction reads the locked row again.
5. That re-read shows `last_execution_event_sequence` equal to this event's sequence. A greater value does not count. A greater stored sequence does not prove that this event was applied.
6. The re-read tenant equals the trusted owner, the payload claim equals that owner, and the inbox row written in this same transaction has this `event_id`, execution, sequence, and event type. A tenant or identity conflict is `NON_TERMINAL`.

`VERIFIED_EQUIVALENT_PRIOR_EFFECT` is the only replay no-op that can be terminal. It requires a full identity match: tenant, execution, sequence, event type, a section 7 hash, and `len(raw)`. It also requires proof that a state effect for this event was stored earlier. `last_execution_event_sequence` being equal to this sequence is necessary and not sufficient. The current state row does not store the `event_id` that wrote it, so a later process cannot tell this event from another writer that left the same sequence number.

The required future column is `tracking.execution_tracking_state.last_event_id`, written in the same transaction as the state effect and equal to this `event_id`. It is not added here. Until that column exists and is non-null, a replay cannot prove the prior effect. Replay stays `NON_TERMINAL`. This document does not call that proof implemented.

A new `event_id` whose sequence is behind the cursor is not a prior effect. The higher cursor is not terminal for that event. The attempt rolls back, writes no inbox row, and does not commit the offset.

The current behavior that commits an inbox row and leaves state unchanged for a stale sequence is not a terminal no-op.

Current `Apply` branches in `services/tracking-service/internal/service/execution_consumer.go`, and the class this contract assigns. The code is unchanged; several branches still return nil without an effect.

| Lines | What returns nil today | Class |
|---|---|---|
| 104–106 | `event_id` already exists, so the insert reports not inserted and the transaction commits with no new state write | `NON_TERMINAL` on replay until `last_event_id` can prove the prior effect. A hash match without that column is not enough. |
| 111–113 | State exists and `state.OperatingTenantID` differs from the payload tenant. The inbox insert is kept and state is not changed. | `NON_TERMINAL`. This is the cross-tenant commit. Section 6 forbids the insert. |
| 114–116 | State exists, the tenant comparison passed, and `event_sequence` is less than or equal to `last_execution_event_sequence`. The new inbox row is kept. | `NON_TERMINAL`. A different `event_id` is not this event. A higher stored sequence is not proof. |
| 118–120 | `route_stop.current` has no view, because the context was missing or the revision differed. The inbox row is kept and state is not changed. | `NON_TERMINAL`. |
| 121–125 | `UpsertExecutionTrackingState` runs and the transaction commits. | `COMMITTED_STATE_EFFECT` only when `RowsAffected > 0`, the re-read sequence equals this event's sequence, and the tenant and `event_id` checks pass. |
| 126–129 | `route_stop.completed` finds no state row and commits the inbox. | `NON_TERMINAL`. |
| 130–132 | `ClearExecutionTarget` returns nil. The SQL updates a row only when the tenant matches and the sequence is newer. | `COMMITTED_STATE_EFFECT` only when `RowsAffected > 0` and the re-read sequence equals this event. Zero rows is `NON_TERMINAL`. |
| 133–136 | Any other stop type finds no state row and commits the inbox. | `NON_TERMINAL`. |
| 137–141 | `TouchExecutionSequence` returns nil, then line 141 commits. The same commit follows a successful upsert or clear. | `COMMITTED_STATE_EFFECT` only when `RowsAffected > 0`, the re-read sequence equals this event, and the commit succeeds. Zero rows is `NON_TERMINAL`. |

J-KRB-R3-001. A parsed `event_type` outside the five stop types is not a known non-stop event. `isExecutionStopEvent` returns false for invalid JSON and for every other string, and the current consumer then commits without `Apply`. That commit is not accepted here.

`INVALID_JSON` is `NON_TERMINAL`. `UNKNOWN_EVENT` is `NON_TERMINAL`. Absence from the allowlist does not permit an offset commit. A syntactically present `event_type` is not a business contract.

`VERIFIED_NON_STOP_SKIP` applies only to a type on the allowlist below. The allowlist is empty. No type is authorized to skip.

The producer does publish other types on `shipment.status.v1`. `resolveKafkaTopic` sends every non-driver event to that topic. `isKnownKafkaEventType` accepts the execution types in `IsExecutionKafkaEventType` and the shipment outbox types in `outbox.go`. Those lists are a publish gate. They are not a tracking decision that state may stay unchanged. Execution payloads from `emitExecutionEvent` carry `event_type`. Shipment status payloads are `ShipmentStatusEventData` and do not carry that field; the type is the outbox event type and the header `eventType`, which this consumer does not read. A body with no `event_type` is not on the allowlist.

Observed on the topic, and not authorized to skip:

| Type | Where it is defined | Why it is not a skip |
|---|---|---|
| `shipment.execution_plan.created`, `shipment.execution_plan.superseded` | `transport_execution_commands.go` | Execution events. No tracking contract says they have no state effect. |
| `shipment.delivery.partially_rejected`, `shipment.delivery.rejected`, `shipment.cargo.disposition_pending`, `shipment.cargo.return_authorized`, `shipment.cargo.redirect_authorized`, `shipment.cargo.return_completed`, `shipment.cargo.redirect_completed` | `transport_execution_disposition.go`, included by `IsExecutionKafkaEventType` | Same. They are execution events, not the five stop types. |
| `shipment.created`, `shipment.status.changed`, `shipment.cancelled`, `shipment.ready_for_billing`, `shipment.documents_completed`, `shipment.financially_closed` | `outbox.go` | Shipment lifecycle events. No tracking contract says a missing state effect is success. |

Open question: a controller may later add one of these types to the allowlist, one type at a time, with a written business rule that tracking must not change state for it. Until that rule exists, the partition parks on the record under `CONTIGUOUS_HOLD`. Parking is the specified behavior, not permission to commit.

The proposed order is not implemented:

1. Parse the payload.
2. Open one database transaction, take the execution advisory lock, and lock the state row, as section 6 requires, before any inbox insert.
3. In that same transaction, either write a `COMMITTED_STATE_EFFECT`, or prove `VERIFIED_EQUIVALENT_PRIOR_EFFECT`. Otherwise roll back. A `NON_TERMINAL` quarantine row and its audit row are a separate transaction and do not update `execution_tracking_state`.
4. One Kafka commit, only for a `TERMINAL` result, and only under section 3.

`Apply` today keeps the inbox insert and the state change in one transaction. Failure rolls both back. A `23505` on `uq_execution_event_inbox_sequence` aborts that transaction, so that attempt does not store tracking state. After the rollback, the classification in section 6 runs. It does not turn the nil-free error into a terminal result.

Not terminal, and not committed on replay: an `Apply` nil from a branch in the table that has no effect, HTTP 401 or 403, invalid JSON, unknown type, missing execution identity, `OWNER_STATE_MISMATCH`, foreign-tenant occupation, a sequence slot held by a different event, a hash or size that fails section 7, quarantine insert failure, a transient failure, an exhausted retry budget, `commit_unknown`, and a canceled context.

`PARTITION_POLICY=CONTIGUOUS_HOLD` remains. A later offset on the same partition is not committed while an earlier offset lacks a terminal result. Other partitions may continue.

There is no database transaction that contains the Kafka offset. Crash windows are replayed from the last broker cursor. Exactly-once is not claimed.

## 6. Tenant occupation of an execution sequence

J-KRB-002. The current insert accepts a foreign tenant's sequence before it looks at the owner.

`InsertExecutionInbox` is `INSERT ... ON CONFLICT (event_id) DO NOTHING`. `uq_execution_event_inbox_sequence` is unique on `(execution_id, event_sequence)` only. The tenant is a column, not part of either key. `IsUniqueViolation` matches every `23505` and ignores the constraint name. It must not be used as a skip signal.

The occupation path today:

1. The payload carries a new `event_id` and a free `(execution_id, event_sequence)`.
2. The insert happens before the tenant comparison. The `event_id` conflict does not fire.
3. If a state row exists and its tenant differs, `Apply` commits the inbox and does not change state.
4. If no state row exists, `route_stop.current` with a missing context and the other stop types also commit the inbox without a state write.
5. The sequence slot is then taken. The legitimate event, with a different `event_id`, gets `23505`, rolls back, and never becomes the stored row.

The current context call does not close this. `Get` is used for `route_stop.current` only, and the tenant argument is the payload claim. A 404 still falls through to the inbox insert. This contract does not treat that call as the ownership check.

The owner comparison that decides an insert happens inside the database transaction, after the locks below and before the insert. A lookup that ran before `Begin` is only a fast reject. It is not the decision.

Transaction order:

1. `BEGIN`.
2. `pg_advisory_xact_lock` on a 64-bit hash of `execution_id`. This is what covers the missing-state case. `SELECT ... FOR UPDATE` locks nothing when the state row is absent, so two transactions could otherwise both observe no row and both insert.
3. `SELECT` the state row `FOR UPDATE` when it exists.
4. Load the trusted owner again by execution id, still before the insert, while those locks are held. The payload tenant is not the lookup key.
5. Decide, then insert or roll back. `COMMIT` only for a section 5 terminal result. Any other decision is `ROLLBACK`.

If the locked `state.OperatingTenantID` differs from that trusted owner:

- roll back, so there is no new inbox insert and no update of an existing inbox row;
- do not change `execution_tracking_state`;
- do not commit the Kafka offset;
- record reason code `OWNER_STATE_MISMATCH` and nothing else. The code is not accompanied by either tenant id, the payload claim, or the state tenant. Customer-visible quarantine columns do not receive those ids. An application log line for this decision is the code alone. The investigator payload store, where it already exists for other quarantine rows, remains the only place the raw bytes are kept, and this stage does not widen that grant.

The advisory lock serializes tracking writers for one execution. The second transaction blocks until the first commits or rolls back, then reads the state row that the first transaction left. The unique key is still the backstop if a writer skips the lock. That closes the tracking-service race between the owner check and the insert.

It does not close a shipment change that happens after the in-transaction lookup returns and before `COMMIT`. Shipment is another service and is not in this transaction. A later event that locks the state row and sees a different trusted owner takes `OWNER_STATE_MISMATCH` and does not commit. This contract does not call the first insert atomic with shipment.

Other pre-insert results, also inside this transaction:

- The owner cannot be loaded. HTTP 401 and 403 are `NON_TERMINAL` and alert. HTTP 404 and an unparsed execution id are missing identity, also `NON_TERMINAL`. HTTP 408, 429, 5xx, and network errors stay transient and do not commit. None of these insert.
- The payload claim differs from the trusted owner. Roll back, no insert, reason code `CLAIM_OWNER_MISMATCH`, with the same prohibition on writing either tenant id. `NON_TERMINAL`.
- Both match, and the state row is absent or has the same owner. The insert may proceed, and it is terminal only under section 5.

This disagrees with the unchanged PR #346 rule that a 404 may commit the inbox. Both drafts stay open.

Existing conflicting row, read only after the failed insert has rolled back:

| Stored row | Incoming event | Result |
|---|---|---|
| Same `event_id`, and section 7 matches | Redelivery | `NON_TERMINAL` until `last_event_id` proves `VERIFIED_EQUIVALENT_PRIOR_EFFECT`. A hash match without that column does not commit. |
| Same authoritative tenant, different `event_id` | Legitimate event that was not stored | The slot is already used by a different event. `NON_TERMINAL`. Do not commit. Do not call the incoming event handled. |
| Tenant is not the authoritative owner | Legitimate event that was not stored | Stolen sequence. `NON_TERMINAL`. Do not commit. Do not delete or overwrite the stored row in this stage. Alert. |
| Authoritative owner | Foreign claim | Foreign event was not stored. `NON_TERMINAL`. Do not commit. The stored legitimate row stays. |
| No row, or any other constraint name | Anything | Unknown. `NON_TERMINAL`. Do not commit. Do not call it permanent. |

A payload tenant never authorizes a state update.

Migration compatibility, with no migration in this change:

- Keep the `event_id` primary key and `uq_execution_event_inbox_sequence`.
- Do not drop that unique key, and do not add the tenant into it in the same step. Rows that already occupy a sequence must remain readable. Rewriting them is not part of this stage.
- A later migration may add nullable `payload_sha256` and `payload_size` on `tracking.execution_event_inbox`, and nullable `last_event_id` and `owner_epoch` on `tracking.execution_tracking_state`. None of those columns are created here. Old rows stay null and fail closed.
- The quarantine table, if it is added later, is a new table. It does not replace the inbox unique key.

## 7. Repeated event_id

J-KRB-004. `ON CONFLICT (event_id) DO NOTHING` plus a commit is not an identity check. The inbox row stores `event_id`, `operating_tenant_id`, `execution_id`, `event_sequence`, `event_type`, and `processed_at`. It does not store the payload.

A repeated `event_id` can be `VERIFIED_EQUIVALENT_PRIOR_EFFECT` only when the stored tenant, execution, sequence, and event type match and the hash rule below accepts the bytes. The hash match is not by itself terminal. Section 5 still requires the prior state effect.

The digest is SHA-256 over the exact raw Kafka record value, the `[]byte` passed to `Apply`, in order, with no JSON re-serialization and no whitespace normalization. The stored form is the lowercase hexadecimal encoding of that digest. The stored size is `len(raw)` in bytes. Those two values are what a later delivery compares.

A hash is usable only when it matches `^[0-9a-f]{64}$`. `NULL`, an empty string, a missing column, uppercase hex, a different length, non-hex characters, and any other string are not hashes. Two empty strings are not the same payload. The digest of a zero-length byte slice is a real 64-character value, and an empty stored string is not that value.

`NULL`, an empty string, an invalid format, a size that is null or unequal to `len(raw)`, and any mismatch of a well-formed hash are `NON_TERMINAL`. The stored inbox row is left unchanged. The offset is not committed.

Historical inbox rows have no hash and no size. They are incomplete, not identical to each other and not identical to a new delivery. This stage does not backfill them. Every replay of an `event_id` whose stored hash or size is missing or not well-formed stays `NON_TERMINAL`.

On any mismatch: leave the stored row as it is, write a `NON_TERMINAL` quarantine row for this offset, alert, and do not commit. The mismatch is not a duplicate success.

## 8. Retry, redelivery, restart, and commit failure

Retry budget for transient and unknown errors: 5 attempts, 200ms initial delay, exponential backoff, 2 second cap, jitter ±20%. The poll window is 8 seconds. Budget expiry is `NON_TERMINAL` and alerts. It does not commit and does not relabel the error as permanent. The current 50ms infinite loop is not the policy.

| Event | Stored | Offset | After restart or redelivery |
|---|---|---|---|
| Duplicate `event_id` with a section 7 match | Existing inbox row | Uncommitted until `last_event_id` proves the prior effect | Replay stays `NON_TERMINAL`. A hash match alone does not commit |
| Duplicate `event_id` with a null hash or any mismatch | Existing inbox row, plus a `NON_TERMINAL` quarantine row | Uncommitted | Stay on that offset |
| Legitimate event whose effect was not stored | `NON_TERMINAL` row, state unchanged | Uncommitted | Do not treat it as handled |
| Foreign occupation | No new inbox row, `NON_TERMINAL` quarantine | Uncommitted | Stay on that offset |
| Invalid JSON, unknown type, or missing execution identity | `NON_TERMINAL` bytes | Uncommitted | Same hash still does not commit |
| Crash before the database transaction commits | Nothing new | Uncommitted | Classify again |
| Crash after a state-effect transaction commits and before `CommitRecords` returns | The state row, still without `last_event_id` in this stage | Unknown until `OffsetFetch` | Replay stays `NON_TERMINAL` until `last_event_id` matches. Do not commit from a hash match alone |
| Crash after a `NON_TERMINAL` row commits | One non-terminal row | Uncommitted | Do not commit |
| `CommitRecords` error, timeout, or `commit_unknown` | Whatever database transaction already committed | Unknown to this process | Read the broker cursor. Do not start the next offset. Do not send a second commit for this one. |
| Context cancel before a commit starts | No partial terminal row | Uncommitted | Replay |
| Revoke or loss before a commit starts | Only transactions that already committed | Last broker commit | The new owner reads that cursor. The old member does not start a commit. |

Redelivery of the same bytes is safe for a matched `event_id` and for the quarantine key. Different bytes at the same group, topic, partition, and offset are a mismatch: keep the first payload, park, and do not commit. Topic recreation can reuse offsets. That case is specified here and is not tested.

## 9. Broker evidence

J-KRB-R2-002 and J-KRB-R3-002. A local lost set, a revoke or lost callback, and `Client.GroupMetadata` are not proof of partition ownership. An `OffsetCommit` the broker accepts after the member has lost ownership fails the safety case even if the callback already ran.

`PROXY_FRAME_ORDER_IS_BROKER_ACCEPTANCE_TIME=NO`. The order of frames at a proxy is not the order in which the coordinator applies RPCs. These times are different, and a test must name which one it observed:

| Moment | What can observe it |
|---|---|
| Client sends the request | The proxy sees the request bytes arrive from the client. |
| Request is delivered to the broker | The proxy releases the bytes toward the broker. A held request has not been delivered. |
| Coordinator processes the request | Not visible on the ordinary Kafka API, and not visible from proxy frame order. |
| Client receives the response | The proxy sees the response bytes. A delayed response can arrive after a rebalance even when the coordinator processed the request before that rebalance. |
| Committed offset changes | A later `OffsetFetch` shows the stored offset. It does not timestamp the apply. |
| Ownership of the partition changes | `DescribeGroups` or a completed sync shows the assignment at the sample time. It is not a history of the coordinator at each RPC. |

A successful response that is observed after a rebalance does not prove the commit was applied after the rebalance. `DELAYED_RESPONSE_IS_NOT_PROOF=YES`.

The ordinary API still has no historical coordinator snapshot. Default broker logs are not that snapshot. A unit test that stubs `CommitRecords` is not a case here.

### Delayed-request test

`DELAYED_REQUEST_TEST_DEFINED=YES`. This is the controlled experiment. It is not implemented.

1. Consumer A builds one `OffsetCommit`.
2. The proxy holds that request and does not deliver it to the coordinator.
3. The test confirms A is gone from the assignment and the partition is assigned to B. The confirmation is `DescribeGroups` or the completed sync sampled while the request is still held.
4. Only after that confirmation, the proxy delivers the held request.
5. The test records the coordinator response.
6. A separate `OffsetFetch` reads the committed offset.
7. B's first `Fetch` is recorded from the proxy.
8. The unprocessed event is not past the cursor unless B read it or A had a terminal result whose commit was delivered while A still owned the partition.

What this experiment can show: the request was not delivered until after the sampled assignment change. If the response is then success and `OffsetFetch` moves to A's offset, the broker accepted a commit that had not been delivered until after that sample. That fails the safety case. If the response is a group error and `OffsetFetch` does not move to A's offset, the broker rejected this held request. That result covers this shape only.

What it cannot show: the coordinator's apply instant. It cannot show that a response which was merely slow, after a request already delivered, was applied after the rebalance. A request that reached the broker before the rebalance is a different test. Its response time is not apply time. That run is `NOT_TESTABLE_WITH_CURRENT_ORACLE` for the apply-versus-loss question. `BROKER_SAFETY_PROVEN=NO`.

### Late commit rewind

`LATE_COMMIT_REWIND_RISK=DOCUMENTED`. `CommitRecords` stores `offset + 1`. The library documents that member A can commit offset 30 after member B has committed offset 50, and the cursor moves back to 30. Classic `OffsetCommit` stores the offset in the request. It does not reject a lower offset because a higher one is already stored. The rewind is possible when a commit carrying 30 is accepted after a commit carrying 50, including when A's request was held and delivered late, and including when A's request was already at the broker before B's commit and the coordinator applies A's request second.

`SINGLE_FLIGHT_BLOCKED_COMMIT` stops a new local commit after the application has recorded the loss, and it does not cancel an RPC already sent. It does not stop the coordinator from applying a request that is in flight or held. It is not protection against every rewind. The delayed-request test fails the mechanism if `OffsetFetch` moves from 50 back to 30. No other local callback repairs that.

Separate checks, which do not prove apply time:

- contiguous terminal prefix, from `OffsetFetch` and the classified records;
- commit order, from requests the proxy actually delivered, not from response arrival;
- B's first `Fetch` against the `OffsetFetch` taken before it;
- no skipped unprocessed event on that fetch.

## 10. Broker test matrix

These cases are not implemented. They are not authorized against staging or production. PR #372 does not satisfy them. One isolated topic, one partition, group `tracking-execution-stop`, autocommit disabled, and the proxy from section 9. A case that needs B and never moves the partition does not pass. Each case is `TESTABLE`, `PARTIALLY_TESTABLE`, or `NOT_TESTABLE_WITH_CURRENT_ORACLE`. A delayed response is not classified as proof of apply time.

### 1. Commit during rebalance

- Setup: A has a state effect for offset 10. The proxy holds the request so the coordinator has not received it. This is the delayed-request shape, not a delayed response. B joins.
- A: one commit in flight. No second commit after the assignment changes.
- B: joins. The partition must be B's before the proxy releases A's request.
- Broker evidence: held request, sampled assignment, then the released request, its response, `OffsetFetch`, and B's first `Fetch`.
- Inbox: the effect row for offset 10 may already be committed in PostgreSQL. No second inbox row unless B reads the record.
- Tracking: A's state effect stays. B does not write another effect unless it reads the record.
- Expected committed offset: the previous cursor, unless the response is success for a request released while the sample still showed A as owner. A success after the sample showed B fails the case.
- Oracle: fail if `OffsetFetch` advances on a request that was released only after B owned the partition. A slow response to a request already delivered is not this oracle.
- Limit: `PARTIALLY_TESTABLE`. The hold proves delivery time. It does not prove the coordinator's apply instant. `BROKER_SAFETY_PROVEN=NO`.

### 2. Slow in-flight commit

- Setup: the same held `OffsetCommit`, held past the 2 second budget and into the rebalance timeout. B joins while it is held.
- A: does not cancel the RPC and does not send a replacement. The local result is `commit_unknown`, not terminal, while the response is absent.
- B: does not treat the record as skipped.
- Broker evidence: the response time and error code on the proxy, ordered against the sync.
- Inbox: A's state-effect transaction may already be committed. That does not mark the Kafka result terminal.
- Tracking: unchanged by the slow RPC itself.
- Expected committed offset: unchanged until a response with error `0` is ordered while A still owns the partition.
- Oracle: a success response after the ownership loss fails the case. A canceled RPC also fails it, because this contract forbids the cancel.
- Limit: the same proxy-order limit as case 1.

### 3. Unknown commit result

- Setup: the proxy drops the `OffsetCommit` response after the broker may have stored it. A crashes or closes without a second commit.
- A: no replacement commit. The result stays `commit_unknown`.
- B: starts after the group releases the partition and reads from `OffsetFetch`.
- Broker evidence: `OffsetFetch` before B's first `Fetch`. The lost response means the proxy cannot say whether the broker stored the offset. That gap is `NOT_PROVEN` for acceptance, not a pass.
- Inbox: whatever transaction committed before the RPC. No new terminal classification is added because the response was lost.
- Tracking: the stored effect stays.
- Expected committed offset: whatever `OffsetFetch` returns. The test records it and does not invent the missing response.
- Oracle: fail if A sends a second commit. Fail the lost-event check if B's first fetch skips an offset B did not apply and the proxy cannot show a prior accepted commit by the owner.
- Limit: a dropped response cannot be reconstructed from `GroupMetadata` or the lost set.

### 4. Partition handover A to B

- Setup: A polls offset 10 and does not commit. B joins. The test does not pass if the assignment never changes.
- A: after the sync removes the partition, A attempts a late commit and the application must not send it.
- B: receives the assignment and fetches offset 10.
- Broker evidence: sync assignment for B, absence of an `OffsetCommit` from A after that sync, then B's `Fetch` at 10.
- Inbox: no row until the member who still owns the partition applies it.
- Tracking: no state change until that apply commits a state effect.
- Expected committed offset: unchanged while A has not sent a commit. B may commit `11` only after its own terminal result.
- Oracle: any `OffsetCommit` from A accepted after the sync fails the case, including one the local lost set claimed to have blocked if the proxy still shows the request.
- Limit: point-in-time `DescribeGroups` shows the handover only if it is sampled while B owns the partition. The proxy log is the record.

### 5. Cross-tenant inbox conflict

- Setup: one execution. A legitimate state row and inbox row exist for sequence 1. A second record uses another tenant claim, the same execution and sequence, and a new `event_id`. A separate subcase has no legitimate row yet and a foreign claim on a free sequence.
- A: processes each record under section 6.
- B: not required. If the partition moves, B must see the same uncommitted record.
- Broker evidence: no `OffsetCommit` for that offset. `OffsetFetch` stays below it.
- Inbox: no new row. The existing row is unchanged.
- Tracking: the legitimate state row is unchanged. The free-sequence subcase still has no state row.
- Expected committed offset: the offset before the foreign record.
- Oracle: an insert, an updated inbox row, a state change, or a commit of that offset fails the case.
- Limit: this case does not need the coordinator history. The database rows and `OffsetFetch` are the evidence. It still is not a unit test of a mocked repository.

### 6. Replay after restart

- Setup: stop the process after a state-effect transaction commits and before `CommitRecords` returns. Restart. Repeat with invalid JSON, an unknown type, and a payload with no execution id. `last_event_id` does not exist yet.
- A: the restarted process is the only member.
- B: none until a second member is added. The first `Fetch` after restart is the recovery evidence.
- Broker evidence: `OffsetFetch` before the restart fetch, then the first `Fetch` offset.
- Inbox: the existing row remains. The invalid records have a `NON_TERMINAL` quarantine row and no new inbox row.
- Tracking: unchanged by the replay. The state row has a sequence and no `last_event_id`.
- Expected committed offset: unchanged for the duplicate and for the three invalid records. Replay is `NON_TERMINAL` until `last_event_id` matches.
- Oracle: any advance on these records fails, including a hash match without `last_event_id`.
- Limit: `TESTABLE` for the refusal to commit. `NOT_TESTABLE_WITH_CURRENT_ORACLE` for a future durable prior-effect proof, because the column is not in the schema. The in-flight response uses case 3.

### 7. Same event_id, different SHA-256

- Setup: the stored row has a well-formed 64-character lowercase hash of the original raw bytes. The redelivered record uses the same `event_id` and a different raw value, so the computed hash differs.
- A: compares section 7 and does not commit.
- B: sees the same record if the partition moves.
- Broker evidence: no `OffsetCommit` for this offset.
- Inbox: the original row is unchanged, including its hash.
- Tracking: unchanged.
- Expected committed offset: unchanged.
- Oracle: a commit, an overwritten hash, or a terminal classification fails.
- Limit: database columns plus `OffsetFetch`. No coordinator history required.

### 8. Same event_id, different payload size

- Setup: the stored hash is well-formed and the stored size equals the original `len(raw)`. The new record has the same `event_id` and a different byte length. The hash may also differ. Either difference is enough.
- A: section 7 rejects the size and does not commit.
- B: same as case 7.
- Broker evidence: no `OffsetCommit` for this offset.
- Inbox: original size and hash unchanged.
- Tracking: unchanged.
- Expected committed offset: unchanged.
- Oracle: a commit or an updated size fails. An empty stored size compared with an empty new size is not a pass; both are `NON_TERMINAL`.
- Limit: same as case 7.

### 9. Nil Apply without a persisted state effect

- Setup: one record for each nil branch in section 5 that does not write state: existing `event_id` without a prior state effect, sequence already behind the cursor for a new `event_id`, `route_stop.current` with no view, `route_stop.completed` with no state, another stop type with no state, and a touch or clear whose update matches zero rows.
- A: runs the real `Apply` against PostgreSQL.
- B: not required.
- Broker evidence: no `OffsetCommit` for that offset. `Apply == nil` is recorded and is not the oracle.
- Inbox: no new row for a branch that this contract rolls back. An existing row is unchanged.
- Tracking: unchanged. A zero-row update stays zero rows.
- Expected committed offset: unchanged.
- Oracle: classifying the outcome terminal, or committing the offset, because `Apply` returned nil, fails.
- Limit: this is still a broker test because the offset assertion is `OffsetFetch`. A Go test with no broker does not replace it.

### 10. Owner lookup and state mismatch

- Setup: the trusted shipment owner and `state.OperatingTenantID` differ. Two consumers start the check together so one waits on the advisory lock.
- A: takes the lock, re-reads the owner, sees the mismatch, and rolls back.
- B: blocks on the same execution lock, then reads the state A left behind and also stops without an insert.
- Broker evidence: no `OffsetCommit` from either member for this offset.
- Inbox: no new row and no change to the existing row.
- Tracking: unchanged.
- Expected committed offset: unchanged.
- Oracle: an insert, an inbox update, a log or quarantine reason that contains either tenant id, or a commit, fails. The reason code is `OWNER_STATE_MISMATCH` only.
- Limit: the lock order is visible in PostgreSQL lock waits. The shipment change after the in-transaction lookup is the residual window in section 6 and is not claimed closed by this case.

### 11. Late commit after loss of ownership

- Setup: A has a terminal result and the proxy has already recorded a sync that assigns the partition to B. A then sends `OffsetCommit` for that partition, including a request the application was supposed to suppress.
- A: the sent request is the subject. A local refusal that still produces no request passes this case only when the proxy shows no request.
- B: owns the partition and may commit a newer offset first.
- Broker evidence: A's request generation and member id, the earlier sync, and A's response error code. B's later commit, if any, is the cursor that must not move backward.
- Inbox: no extra row from the late request.
- Tracking: unchanged by the late request.
- Expected committed offset: B's cursor if B committed while it owned the partition, otherwise the last commit from before the sync. Not A's late offset.
- Oracle: response error `0` on A's late request fails the case even if A's callback said the partition was lost. A response that rewinds B also fails the commit-order check.
- Limit: if the response cannot be ordered after the sync, this case is `NOT_PROVEN`, not a pass.

### 12. Redelivery of an uncommitted legitimate event

- Setup: A applied a legitimate event, the state effect is committed, and the `OffsetCommit` was not accepted. The process restarts or B takes the partition.
- A: if it restarts, its first fetch is the uncommitted offset.
- B: if it takes the partition, its first `Fetch` is that same offset, and it applies the record.
- Broker evidence: `OffsetFetch` still at the old cursor, then the first `Fetch` at the legitimate record's offset.
- Inbox: the original effect row stays. Section 7 matches the redelivered bytes. No second state write is required for `VERIFIED_EQUIVALENT_PRIOR_EFFECT`.
- Tracking: the existing effect stays. It is not rolled back and not duplicated.
- Expected committed offset: advances by one only after the member who reads the record commits under section 3, and only because the prior effect verifies.
- Oracle: a first fetch that starts past the record fails the lost-event check. A commit that happens without the prior-effect proof also fails.
- Limit: B's application counter is not the evidence. The `Fetch` offset on the proxy is. Replay is `NON_TERMINAL` until `last_event_id` exists, so this case does not advance the cursor on restart today.

### 13. Known non-stop event

- Preconditions: the allowlist in section 5 is empty. A record uses one of the producer types listed there, which is not a stop type.
- Stimulus: the consumer reads it.
- Expected changes: no inbox insert and no state write. Class `NON_TERMINAL`.
- Expected committed offset: unchanged.
- Replay: still `NON_TERMINAL`. The same type does not become a skip on the second read.
- Observable evidence: `OffsetFetch` and the unchanged tables.
- Limit: `TESTABLE` for the refusal to commit. `NOT_TESTABLE_WITH_CURRENT_ORACLE` for a future allowlist entry, because no business rule authorizes one.

### 14. Unknown parsed event type

- Preconditions: the JSON parses and `event_type` is a non-empty string outside the five stop types and outside the empty allowlist.
- Stimulus: the consumer reads it.
- Expected changes: no state effect. `UNKNOWN_EVENT` is `NON_TERMINAL`.
- Expected committed offset: unchanged.
- Replay: unchanged and still uncommitted.
- Observable evidence: parsed type, `OffsetFetch`, empty state change.
- Limit: `TESTABLE`. The producer publish gate is not evidence that this type may be skipped.

### 15. Invalid JSON

- Preconditions: the record value is not JSON.
- Stimulus: the consumer reads it. `isExecutionStopEvent` returns false today and the current code would commit. This contract must not.
- Expected changes: no inbox row and no state row.
- Expected committed offset: unchanged.
- Replay: still `NON_TERMINAL`.
- Observable evidence: `OffsetFetch` stays below the record.
- Limit: `TESTABLE`.

### 16. Delayed request versus delayed response

- Preconditions: two runs of the same `OffsetCommit`. Run R holds the request before delivery. Run S delivers the request immediately and holds only the response.
- Stimulus: B receives the partition during the hold.
- Expected changes: run R is case 1. Run S records the response time and does not call it the apply time.
- Expected committed offset: run R as in case 1. Run S reports `OffsetFetch` without claiming when the coordinator stored it.
- Replay: neither run classifies the event terminal because a response arrived late.
- Observable evidence: proxy hold point, assignment sample, response, `OffsetFetch`, B's first `Fetch`.
- Limit: run R is `PARTIALLY_TESTABLE`. Run S is `NOT_TESTABLE_WITH_CURRENT_ORACLE` for apply-versus-loss. `DELAYED_RESPONSE_IS_NOT_PROOF=YES`.

### 17. Offset rewind from 50 to 30

- Preconditions: B has a stored commit at 50. A's `OffsetCommit` carries 30 and is held until after that store, then released. This is the delayed-request shape.
- Stimulus: release A's request.
- Expected changes: the safety case fails if the cursor moves from 50 to 30. `SINGLE_FLIGHT_BLOCKED_COMMIT` is not a pass by itself.
- Expected committed offset: remains 50. A move to 30 fails the case.
- Replay: B's next fetch must not start at 30 unless 30 was the last accepted owner commit, which this setup says it was not.
- Observable evidence: `OffsetFetch` before and after the release, and B's first `Fetch`.
- Limit: `PARTIALLY_TESTABLE` for a request held until after B's commit. A pair of requests already inside the broker is `NOT_TESTABLE_WITH_CURRENT_ORACLE` for which one the coordinator applied last.

### 18. Stale sequence and a different event_id

- Preconditions: state `last_execution_event_sequence` is 5 for the trusted owner. The new record has `event_sequence` 2 and a new `event_id`.
- Stimulus: `Apply`.
- Expected changes: rollback, no new inbox row, no state change. The higher sequence is not terminal for this event.
- Expected committed offset: unchanged.
- Replay: the same record stays `NON_TERMINAL`.
- Observable evidence: row counts, sequence still 5, `OffsetFetch`.
- Limit: `TESTABLE` against PostgreSQL and `OffsetFetch`. It is not a unit test with a fake repository.

### 19. RowsAffected = 0

- Preconditions: `TouchExecutionSequence` or `ClearExecutionTarget` matches no row. The SQL returns success with zero rows.
- Stimulus: the call and the following commit of the database transaction, as the current code does.
- Expected changes: no state change. Class `NON_TERMINAL` even though the error is nil.
- Expected committed offset: unchanged.
- Replay: still `NON_TERMINAL`.
- Observable evidence: `RowsAffected`, the re-read sequence, `OffsetFetch`.
- Limit: `TESTABLE`. `Apply == nil` is not the oracle.

### 20. Shipment owner epoch changed

- Preconditions: the in-transaction lookup returns epoch 4, then a second lookup before the insert returns epoch 5. No shared transaction with shipment exists.
- Stimulus: the consumer reaches the insert decision.
- Expected changes: rollback, reason `OWNER_EPOCH_CHANGED`, no tenant ids in the reason, no insert.
- Expected committed offset: unchanged.
- Replay: `NON_TERMINAL` while the epochs differ.
- Observable evidence: the two epoch values and the unchanged tables. The shipment write that happened between them is outside this database.
- Limit: `PARTIALLY_TESTABLE` if the test controls both lookup results. The real race remains `NOT_TESTABLE_WITH_CURRENT_ORACLE` as a closed proof, and `SHIPMENT_OWNER_RACE=OPEN`.

### 21. Mixed-version consumer group

- Preconditions: member A advertises the old contract and member B advertises the new one. The old member does not take the advisory lock.
- Stimulus: both remain in the group, and the old member commits an `Apply` nil with no state effect.
- Expected changes: the new mode does not start. B does not commit. A's already stored offset is not rewritten by B.
- Expected committed offset: whatever A stored, left in place. B does not advance it and does not move it backward.
- Replay: historical conflicts stay for the later recovery plan. They are not auto-cleared.
- Observable evidence: `DescribeGroups` member metadata and `OffsetFetch` before and after B starts.
- Limit: `TESTABLE` as a refusal to enter the new mode. `NOT_TESTABLE_WITH_CURRENT_ORACLE` as a claim that the two versions are safe together. They are not compatible.

## 11. Shipment owner race

`SHIPMENT_OWNER_RACE=OPEN`. Section 6 locks tracking writers. It does not lock the shipment service. A second read of the owner, without a transaction that contains both databases, can still be stale before the tracking commit.

The candidate fence is a monotonic ownership epoch minted by shipment. It is not implemented, and this stage adds no migration.

- Source: shipment, at execution creation and at each change of operating owner. The counter increases by one and is never reused. The payload is not the source.
- Change: only shipment advances it, in the same shipment transaction that changes the owner.
- Read: the tracking lookup returns the owner and the epoch together. Tracking stores neither value as authority until the checks below pass.
- Identity: the epoch is not part of `event_id`. An event does not become a different event because the epoch changed. The epoch fences the write.
- Check: inside the tracking transaction, after the advisory lock and before the insert, read the epoch again. If a state row exists, its stored epoch must equal that second read, and the second read must equal the first read in this decision. Otherwise roll back.
- Concurrent owner change: two tracking transactions can both read epoch 4, shipment can move to epoch 5, and both can still write. The second read does not serialize with shipment. That is why the race stays open.
- Source unavailable: `NON_TERMINAL`, reason `OWNER_EPOCH_UNAVAILABLE`, no insert and no offset commit.
- Epoch changed between the two reads, or the stored state epoch differs: `NON_TERMINAL`, reason `OWNER_EPOCH_CHANGED`. The reason carries no tenant id.

A future column `owner_epoch` on `execution_tracking_state` would record the epoch that matched at the write. It is not created here. Re-reading the epoch is not a sufficient fence. `SHIPMENT_OWNER_RACE=OPEN`.

## 12. Mixed consumer deployment

`MIXED_CONSUMER_DEPLOYMENT_GATE=CLOSED`. Old and new members are not compatible, and this stage does not roll anything out.

- The old consumer does not take the execution advisory lock.
- The old consumer can still commit an `Apply` nil that has no state effect, and it can still commit an offset for invalid JSON or an unknown type.
- The new consumer does not repair an offset the old consumer has already committed. It does not move that cursor backward to hide the old commit.
- Two versions in one group are not treated as compatible because they share a group id.

The gate, when a later stage builds it, has all of these conditions:

1. Contract version on each member, carried in client identity the group coordinator returns. `Client.GroupMetadata` is not the roster.
2. `DescribeGroups` lists every active member and the version it advertises.
3. The new mode stays off until every member advertises the new version and none advertises the old one.
4. Before the mode is allowed, read `OffsetFetch` and the inbox. An offset already past a `NON_TERMINAL` record is a historical conflict.
5. If a mixed member appears, stop. The new member does not poll for a commit and does not stay in the new mode. It alerts and leaves or refuses the assignment.
6. Recovery of those historical conflicts is a later clearance plan. It is not automatic and it is not this stage.

Until that gate exists and passes, the new binary must not run in a group where an old binary can still join.

## 13. Decisions still requiring approval

1. Accept or reject `SINGLE_FLIGHT_BLOCKED_COMMIT`. It is not sufficient against every rewind. `GENERATION_FENCED_MANUAL_COMMIT` remains withdrawn.
2. No implementation starts before section 10 has run on an isolated broker. A case that is `NOT_TESTABLE_WITH_CURRENT_ORACLE` or `PARTIALLY_TESTABLE` for apply time does not open the gate. `BROKER_SAFETY_PROVEN=NO`. The production consumer stays blocked.
3. The 2 second commit budget and the 8 second poll window.
4. Whether HTTP 404 stays the PR #346 inbox-commit behavior or the non-insert rule in section 6. This draft does not edit PR #346.
5. Whether any producer type in section 5 may later be added to the empty non-stop allowlist. None are authorized now.
6. Investigator role, payload retention, and clearance. Not in this stage.
7. The unresolved-age alert threshold.
8. Later migrations for the inbox hash columns, `last_event_id`, and `owner_epoch`. None are created here.
9. `SHIPMENT_OWNER_RACE` stays open until a fence that serializes with shipment exists. A second read is not that fence.
10. The mixed-version gate stays closed until the six conditions in section 12 are implemented and observed. This stage does not enable it.

## 14. Boundary

This document does not implement the consumer, the tables, the hash columns, `last_event_id`, `owner_epoch`, the tests, or a rebalance hook. It does not change SQL, Kafka configuration, or migrations. It does not change PR #346 or PR #372. It does not close issue #344. It does not roll out a mixed consumer group. Staging and production are unchanged. The consumer must not be started as a production reader on the strength of this proposal.
