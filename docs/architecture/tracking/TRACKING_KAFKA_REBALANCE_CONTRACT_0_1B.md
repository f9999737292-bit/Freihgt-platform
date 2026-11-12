# TRACKING-KAFKA-REBALANCE-CONTRACT-0.1B

Status: R2 remediation of the design proposal. Production code, SQL, Kafka configuration, and migrations are unchanged. Implementation of 0.1B has not started. No broker test has passed the mechanism below, so it is not a safety proof.

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

The generation on the `OffsetCommit` request is copied from `memberGen` inside `commit()`, while `g.mu` is held, not at the earlier application check. `GroupMetadata` in this version returns the group and the generation only. Reading it before `CommitRecords` does not attach that value to the RPC.

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
| `TERMINAL` | Parsed `execution_id` and `event_sequence`, authoritative tenant confirmed, and the row records a duplicate whose stored identity and payload hash already match section 7. The effect is already stored. | Only after that row is committed in PostgreSQL | Yes, once, if the hash, size, and epoch still match |
| `NON_TERMINAL` | Invalid JSON, unknown type, missing `execution_id` or `event_sequence`, foreign-tenant occupation, a legitimate event whose effect was not stored, hash or size or epoch mismatch, 401, 403, or an unknown constraint | No | No |

Invalid JSON, an unknown type, and missing execution identity stay `NON_TERMINAL` even when the bytes are stored. Replay of those bytes does not commit. The partition stays parked under `PARTITION_POLICY=CONTIGUOUS_HOLD`.

A hash, size, or epoch mismatch does not overwrite the stored payload. It is `NON_TERMINAL`, alerts, and does not commit.

The consumer may insert and may read metadata, hash, size, epoch, class, and fence columns. It may not read the payload, update, or delete. An investigator reads the payload. Customer APIs have no grant. Opening writes one append-only audit row in the same transaction. Clearance without an audit row is forbidden. This stage does not implement clearance, replay tooling, or deletion.

## 5. Atomicity

`Apply` today keeps the inbox insert and the tracking-state change in one database transaction. Failure rolls both back. A `23505` on `uq_execution_event_inbox_sequence` aborts that transaction, so that attempt does not store tracking state.

The proposed order is different and is not implemented:

1. Parse the payload.
2. Resolve the execution owner before any inbox insert, as section 6 requires.
3. One database transaction for the inbox insert and the state change, or for a quarantine row and its audit row. Quarantine does not update `execution_tracking_state`.
4. One Kafka commit, only for a terminal result, and only under section 3.

Terminal results that may commit one offset:

- `Apply` succeeded and the state change, or the legitimate no-op, is committed in PostgreSQL.
- A verified filter-skip of a known non-stop event.
- A `TERMINAL` quarantine row that has committed, for a duplicate that already matched section 7.

A legitimate event whose processing did not commit is not terminal. Opening a quarantine row for it does not make it terminal.

Not terminal, and not committed on replay: HTTP 401 or 403, invalid JSON, unknown type, missing execution identity, foreign-tenant occupation, a sequence slot held by a different event, hash mismatch, quarantine insert failure, a transient failure, an exhausted retry budget, `commit_unknown`, and a canceled context.

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

Proposed check, before `Begin` and before the inbox insert:

1. A trusted shipment lookup loads the execution owner by execution id. The payload `operating_tenant_id` is only a claim. It is not the lookup key.
2. If the owner cannot be loaded, including HTTP 404, 401, 403, 408, 429, 5xx, and network errors, do not insert. 401 and 403 are `NON_TERMINAL` and alert. 404 and a payload that does not parse to an execution id are missing identity, also `NON_TERMINAL`. 408, 429, 5xx, and network errors stay transient and do not commit.
3. If the claim differs from the owner, do not insert. `NON_TERMINAL`. Do not change state. Do not commit.
4. Only a matching owner may enter the inbox transaction.

This disagrees with the unchanged PR #346 rule that a 404 may commit the inbox. Both drafts stay open.

Existing conflicting row, read only after the failed insert has rolled back:

| Stored row | Incoming event | Result |
|---|---|---|
| Same `event_id`, and section 7 matches | Redelivery of that event | Duplicate. No state change. `TERMINAL` only because the effect is already stored. |
| Same authoritative tenant, different `event_id` | Legitimate event that was not stored | The slot is already used by a different event. `NON_TERMINAL`. Do not commit. Do not call the incoming event handled. |
| Tenant is not the authoritative owner | Legitimate event that was not stored | Stolen sequence. `NON_TERMINAL`. Do not commit. Do not delete or overwrite the stored row in this stage. Alert. |
| Authoritative owner | Foreign claim | Foreign event was not stored. `NON_TERMINAL`. Do not commit. The stored legitimate row stays. |
| No row, or any other constraint name | Anything | Unknown. `NON_TERMINAL`. Do not commit. Do not call it permanent. |

A payload tenant never authorizes a state update.

Migration compatibility, with no migration in this change:

- Keep the `event_id` primary key and `uq_execution_event_inbox_sequence`.
- Do not drop that unique key, and do not add the tenant into it in the same step. Rows that already occupy a sequence must remain readable. Rewriting them is not part of this stage.
- A later migration may add nullable `payload_sha256` and `payload_size` on `tracking.execution_event_inbox`, filled in the same transaction as a new insert. Existing rows stay null. A null hash fails closed under section 7.
- The quarantine table, if it is added later, is a new table. It does not replace the inbox unique key.

## 7. Repeated event_id

J-KRB-004. `ON CONFLICT (event_id) DO NOTHING` plus a commit is not an identity check. The inbox row stores `event_id`, `operating_tenant_id`, `execution_id`, `event_sequence`, `event_type`, and `processed_at`. It does not store the payload.

A repeated `event_id` is terminal only when every one of these matches the stored row:

- `operating_tenant_id`
- `execution_id`
- `event_sequence`
- `event_type`
- payload SHA-256
- payload size

The hash and size are the payload check. The current inbox cannot supply them. Until the nullable columns in section 6 exist and the stored hash is non-null, a repeated `event_id` is `NON_TERMINAL`: do not commit, do not change state, and do not overwrite the row. A null hash on an old row fails closed the same way.

On any mismatch: leave the stored row as it is, write a `NON_TERMINAL` quarantine row for this offset, alert, and do not commit. The mismatch is not a duplicate success.

A match does not advance tracking state. It may commit that offset only as a duplicate of an effect that is already stored.

## 8. Retry, redelivery, restart, and commit failure

Retry budget for transient and unknown errors: 5 attempts, 200ms initial delay, exponential backoff, 2 second cap, jitter ±20%. The poll window is 8 seconds. Budget expiry is `NON_TERMINAL` and alerts. It does not commit and does not relabel the error as permanent. The current 50ms infinite loop is not the policy.

| Event | Stored | Offset | After restart or redelivery |
|---|---|---|---|
| Duplicate `event_id` with a full section 7 match | Existing inbox row | Commit | Second apply does not change state |
| Duplicate `event_id` with a null hash or any mismatch | Existing inbox row, plus a `NON_TERMINAL` quarantine row | Uncommitted | Stay on that offset |
| Legitimate event whose effect was not stored | `NON_TERMINAL` row, state unchanged | Uncommitted | Do not treat it as handled |
| Foreign occupation | No new inbox row, `NON_TERMINAL` quarantine | Uncommitted | Stay on that offset |
| Invalid JSON, unknown type, or missing execution identity | `NON_TERMINAL` bytes | Uncommitted | Same hash still does not commit |
| Crash before the database transaction commits | Nothing new | Uncommitted | Classify again |
| Crash after a `TERMINAL` row commits and before `CommitRecords` returns | One terminal row | Unknown until the broker cursor is read | Hash match, then one commit if the cursor is still below this offset |
| Crash after a `NON_TERMINAL` row commits | One non-terminal row | Uncommitted | Do not commit |
| `CommitRecords` error, timeout, or `commit_unknown` | Whatever database transaction already committed | Unknown to this process | Read the broker cursor. Do not start the next offset. Do not send a second commit for this one. |
| Context cancel before a commit starts | No partial terminal row | Uncommitted | Replay |
| Revoke or loss before a commit starts | Only transactions that already committed | Last broker commit | The new owner reads that cursor. The old member does not start a commit. |

Redelivery of the same bytes is safe for a matched `event_id` and for the quarantine key. Different bytes at the same group, topic, partition, and offset are a mismatch: keep the first payload, park, and do not commit. Topic recreation can reuse offsets. That case is specified here and is not tested.

## 9. Isolated tests

These tests are a proposal. They are not implemented. They are not authorized against staging or production. PR #372 does not satisfy them. Calling the franz-go API is not a pass. A run that never moves the partition does not pass the handoff case.

Setup for every case: one isolated test topic, one partition so the record has to move, group `tracking-execution-stop`, autocommit disabled, two members built with the proposed client options. The broker is the test broker only.

1. Rebalance during an in-flight commit. A has processed offset 10 and called `CommitRecords`. Hold that `OffsetCommit` before the broker answers. Move the partition to B. A must not start a second commit after revoke or lost. Pass only if B either reads offset 10, or the cursor advanced solely because A's RPC completed while A still owned the partition and B starts at the following offset. Fail if the cursor moves past offset 10 and neither member applied it or left it explicitly uncommitted.
2. Slow commit. The same RPC is slower than the 2 second budget and is still running as the rebalance timeout approaches. A must not cancel it and must not send a replacement. The event must not be marked terminal while the response is unknown. When the response finally arrives, record whether the broker stored the offset. If it stored the offset after A had lost the partition, this mechanism fails and stays unproven.
3. Two-consumer handoff. A polls the record and does not commit. B joins. The test must observe B's assignment. A then attempts a late commit and the application must refuse to send it. B processes the record. A's refusal must not rewind B's cursor.
4. Cross-tenant sequence. A foreign payload with a free `(execution_id, event_sequence)` must not insert an inbox row and must not commit. Separately, a stored foreign row already occupying that sequence must leave the legitimate event uncommitted and must not mark that legitimate event terminal. State for the legitimate tenant stays unchanged.
5. Replay after restart. Kill the process after a `TERMINAL` duplicate row commits and before `CommitRecords` returns. Restart. A full section 7 match may commit once, and a second restart must not move the cursor again. Repeat with invalid JSON, an unknown type, and a payload that has no execution id. Those `NON_TERMINAL` rows must still be uncommitted after restart.
6. B does not lose the event. In each case, B's fetch position is the broker's committed offset. Every offset at or after that position is either processed by B, still uncommitted, or was committed by A while A owned it and the response was success before the loss. An offset that the committed cursor has passed, and that neither member processed nor parked without committing, is a lost event and a failed test.

## 10. Decisions still requiring approval

1. Accept or reject `SINGLE_FLIGHT_BLOCKED_COMMIT`. It is proposed, not proven. `GENERATION_FENCED_MANUAL_COMMIT` is not a sufficient substitute.
2. No implementation starts before section 9 has passed on an isolated broker. The production consumer stays blocked either way.
3. The 2 second commit budget and the 8 second poll window against the 45 second session timeout and the 60 second rebalance timeout.
4. Whether HTTP 404 stays the PR #346 inbox-commit behavior or becomes the non-insert, non-commit rule in section 6. This draft does not edit PR #346.
5. Investigator role, payload retention, and the later clearance stage. This stage does not include clearance or replay tooling.
6. The unresolved-age alert threshold.
7. Whether the nullable inbox hash columns are approved as a later migration. They are not created here.

## 11. Boundary

This document does not implement the consumer, the tables, the hash columns, the tests, or a rebalance hook. It does not change SQL, Kafka configuration, or migrations. It does not change PR #346 or PR #372. It does not close issue #344. Staging and production are unchanged. The consumer must not be started as a production reader on the strength of this proposal.
