# TRACKING-KAFKA-REBALANCE-CONTRACT-0.1B

Status: R3 remediation after Agent J's R2 review. Production code, SQL, Kafka configuration, and migrations are unchanged. Implementation of 0.1B has not started. No broker test has passed the mechanism below, so it is not a safety proof.

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
| `TERMINAL` | Section 5 has proved `COMMITTED_STATE_EFFECT` or `VERIFIED_EQUIVALENT_PRIOR_EFFECT` for these bytes. A duplicate hash is not enough on its own. | Only after that proof is committed in PostgreSQL | Yes, once, if section 7 still accepts the hash and size and the prior state effect is still present |
| `NON_TERMINAL` | Invalid JSON, unknown type, missing `execution_id` or `event_sequence`, foreign-tenant occupation, a legitimate event whose effect was not stored, hash or size or epoch mismatch, 401, 403, or an unknown constraint | No | No |

Invalid JSON, an unknown type, and missing execution identity stay `NON_TERMINAL` even when the bytes are stored. Replay of those bytes does not commit. The partition stays parked under `PARTITION_POLICY=CONTIGUOUS_HOLD`.

A hash, size, or epoch mismatch does not overwrite the stored payload. It is `NON_TERMINAL`, alerts, and does not commit.

The consumer may insert and may read metadata, hash, size, epoch, class, and fence columns. It may not read the payload, update, or delete. An investigator reads the payload. Customer APIs have no grant. Opening writes one append-only audit row in the same transaction. Clearance without an audit row is forbidden. This stage does not implement clearance, replay tooling, or deletion.

## 5. Atomicity and terminal results

J-KRB-R2-001. `Apply` returning nil is not a terminal result. A missing change to `execution_tracking_state` is not successful processing unless this attempt proves a previously committed acceptable effect.

`TERMINAL = COMMITTED_STATE_EFFECT OR VERIFIED_EQUIVALENT_PRIOR_EFFECT`

Every other outcome is `NON_TERMINAL` until a check in this section proves one of those two. A quarantine row, an inbox insert, and a committed database transaction are not substitutes for that proof.

`COMMITTED_STATE_EFFECT` means this same transaction changed `execution_tracking_state` for the trusted owner and then committed. The change is an upsert, a `ClearExecutionTarget` that updates a row, or a `TouchExecutionSequence` that updates a row. `RowsAffected = 0` is not an effect. The proof is the locked row after the write, not the nil error.

`VERIFIED_EQUIVALENT_PRIOR_EFFECT` is the only legitimate no-op. It is terminal only when all of the following are true:

- this transaction does not change `execution_tracking_state`;
- a state row for this execution is already committed;
- its `OperatingTenantID` equals the trusted owner, and the payload claim equals that same owner;
- `last_execution_event_sequence` is already greater than or equal to this event's sequence;
- the existing inbox row for this `event_id` matches that owner, execution, sequence, and event type;
- section 7 accepts the stored hash and size for these exact bytes.

A new `event_id` whose sequence is already behind the cursor is not that no-op. The cursor does not prove that this payload was applied. That attempt rolls back, writes no inbox row, and does not commit the offset.

The business behavior that today commits an inbox row and leaves state unchanged for a stale sequence is not accepted as a terminal no-op. The only accepted no-op is the identity-matched redelivery above.

Current `Apply` branches in `services/tracking-service/internal/service/execution_consumer.go`, and the class this contract assigns. The code is unchanged; several branches still return nil without an effect.

| Lines | What returns nil today | Class |
|---|---|---|
| 104–106 | `event_id` already exists, so the insert reports not inserted and the transaction commits with no new state write | `NON_TERMINAL` until section 7 and the prior-effect checks above pass. Otherwise it is not a duplicate success. |
| 111–113 | State exists and `state.OperatingTenantID` differs from the payload tenant. The inbox insert is kept and state is not changed. | `NON_TERMINAL`. This is the cross-tenant commit. Section 6 forbids the insert. |
| 114–116 | State exists, the tenant comparison passed, and `event_sequence` is less than or equal to `last_execution_event_sequence`. The new inbox row is kept. | `NON_TERMINAL` unless the prior-effect checks pass for this same identity. A different `event_id` fails that test. |
| 118–120 | `route_stop.current` has no view, because the context was missing or the revision differed. The inbox row is kept and state is not changed. | `NON_TERMINAL`. |
| 121–125 | `UpsertExecutionTrackingState` runs and the transaction commits. | `COMMITTED_STATE_EFFECT` only if the upsert wrote the state row and the commit succeeds. |
| 126–129 | `route_stop.completed` finds no state row and commits the inbox. | `NON_TERMINAL`. |
| 130–132 | `ClearExecutionTarget` returns nil. The SQL updates a row only when the tenant matches and the sequence is newer. | `COMMITTED_STATE_EFFECT` only when the update changes a row. Zero rows is `NON_TERMINAL`. |
| 133–136 | Any other stop type finds no state row and commits the inbox. | `NON_TERMINAL`. |
| 137–141 | `TouchExecutionSequence` returns nil, then line 141 commits. The SQL updates a row only when the tenant matches and the sequence is newer. The same line 141 commit follows a successful upsert or clear. | `COMMITTED_STATE_EFFECT` only when that update changes a row and the commit succeeds. Zero rows is `NON_TERMINAL`. |

A known non-stop record is not an `Apply` result. `isExecutionStopEvent` returns false for a parsed `event_type` outside the five stop types, and the current consumer then commits that record without calling `Apply`. This contract still allows that one commit, under the name `VERIFIED_NON_STOP_SKIP`, because the bytes were classified before `Apply`. Invalid JSON takes the same false return today and must not use this exception: it stays `NON_TERMINAL`.

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
| Same `event_id`, and section 7 matches, and section 5 proves `VERIFIED_EQUIVALENT_PRIOR_EFFECT` | Redelivery of an event that already changed state | Duplicate. No new state change. `TERMINAL`. A hash match without that prior state effect stays `NON_TERMINAL`. |
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
| Duplicate `event_id` with a section 7 match and a section 5 prior state effect | Existing inbox row and existing state effect | Commit | Second apply does not change state |
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

## 9. Broker evidence

J-KRB-R2-002. A local lost set, an `OnPartitionsRevoked` or `OnPartitionsLost` callback, and `Client.GroupMetadata` are not proof of partition ownership. They are client memory. The acceptance rule is independent of them: an `OffsetCommit` that the broker accepts after that member has lost ownership fails the safety case, even if the callback already ran or the application refused a later call.

The ordinary Kafka API does not provide a historical snapshot of coordinator state at the moment each RPC was applied. `OffsetFetch` returns the latest committed offset. `DescribeGroups` returns the current members and assignment. Neither says which generation owned the partition when an earlier commit was stored. This contract does not pretend that they do.

What an isolated test can record, and only if it inserts a recording proxy on the client-to-broker connection:

- `OffsetCommit` request fields: group, generation, member id, topic, partition, and the committed offset (`record offset + 1`). The response error code per partition. Error `0` is acceptance.
- `SyncGroup` request and response on that same ordered connection: generation, member id, and assignment. A later `DescribeGroups` is a point-in-time check, not a substitute for the ordered log.
- The next `Fetch` after restart or rebalance: member id and the offset it asks to read. That is the first offset B reads. B's application log is not this evidence.

Ownership at acceptance is the proxy order. The commit was accepted after the loss if the response has error `0` and, earlier in that log, a completed sync or removal already took that partition away from that member id. If the proxy cannot order the response against that assignment change, the ownership criterion for that run is `NOT_PROVEN`. It is not filled in from the client.

Redpanda request logs are not the oracle unless the test enables a documented debug line that contains those same fields and the test captures it before rotation. Default broker logs are not assumed to contain them. A unit test that stubs `CommitRecords` is not a case in this section.

Separate from the ownership rule, every case also records:

- contiguous terminal prefix: the committed offset is the first offset that is not terminal, or the end of the log when every offset in the prefix is terminal. Committing past a `NON_TERMINAL` offset fails this check.
- commit order: the proxy shows one in-flight `OffsetCommit` for the partition, no second request before its response, and no later request that moves the cursor backward.
- B recovery: after restart or rebalance, B's first `Fetch` offset equals the `OffsetFetch` value taken before that fetch.
- no lost event: every offset from that fetch through the log end was applied by B, is still uncommitted, or was accepted by the broker while the proxy still showed A as the owner.

These four checks can pass or fail on their own evidence. A pass on them does not prove the ownership rule when the proxy order is missing.

## 10. Broker test matrix

These cases are not implemented. They are not authorized against staging or production. PR #372 does not satisfy them. One isolated topic, one partition, group `tracking-execution-stop`, autocommit disabled, and the proxy from section 9. A case that needs B and never moves the partition does not pass.

### 1. Commit during rebalance

- Setup: A has applied offset 10 with a `COMMITTED_STATE_EFFECT`. The proxy holds the `OffsetCommit` before the broker answers. B joins.
- A: one commit in flight. No second commit after the assignment changes.
- B: joins and waits for the held RPC to finish, then fetches.
- Broker evidence: ordered proxy log of A's commit response and the sync that moves the partition.
- Inbox: the effect row for offset 10 is committed. No second inbox row from B unless B actually fetches offset 10.
- Tracking: the state effect from A's transaction stays. B does not write a second effect if it does not read the record.
- Expected committed offset: `11` only if A's response is success and the proxy shows A still owned the partition at that response. Otherwise the cursor stays at the previous commit.
- Oracle: fail if the response is success after the sync removed A's ownership. Also fail the lost-event check if the cursor passes 10 and B never reads it.
- Limit: without proxy order, the ownership half is `NOT_PROVEN`. The prefix and B's first fetch can still be recorded.

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

- Setup: stop the process after a `VERIFIED_EQUIVALENT_PRIOR_EFFECT` row is committed and before `CommitRecords` returns. Restart the member. Repeat with invalid JSON, an unknown type, and a payload with no execution id.
- A: the restarted process is the only member.
- B: none until a second member is added. The first `Fetch` after restart is the recovery evidence.
- Broker evidence: `OffsetFetch` before the restart fetch, then the first `Fetch` offset.
- Inbox: the prior-effect row remains for the duplicate. The invalid records have a `NON_TERMINAL` quarantine row and no new inbox row.
- Tracking: unchanged by the replay.
- Expected committed offset: one step forward for the verified duplicate, once. Unchanged for the three `NON_TERMINAL` records, including on a second restart.
- Oracle: a second advance for the duplicate, or any advance for invalid JSON, unknown type, or missing identity, fails.
- Limit: the restart fetch shows where the member started. It does not reconstruct a commit response that was in flight at the crash. That part uses case 3.

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
- Limit: B's application counter is not the evidence. The `Fetch` offset on the proxy is.

## 11. Decisions still requiring approval

1. Accept or reject `SINGLE_FLIGHT_BLOCKED_COMMIT`. It is proposed, not proven. `GENERATION_FENCED_MANUAL_COMMIT` is not a sufficient substitute.
2. No implementation starts before section 10 has passed on an isolated broker, including the proxy order where a case needs it. A case left `NOT_PROVEN` does not open the gate. The production consumer stays blocked either way.
3. The 2 second commit budget and the 8 second poll window against the 45 second session timeout and the 60 second rebalance timeout.
4. Whether HTTP 404 stays the PR #346 inbox-commit behavior or becomes the non-insert, non-commit rule in section 6. This draft does not edit PR #346.
5. Investigator role, payload retention, and the later clearance stage. This stage does not include clearance or replay tooling.
6. The unresolved-age alert threshold.
7. Whether the nullable inbox hash columns are approved as a later migration. They are not created here.

## 12. Boundary

This document does not implement the consumer, the tables, the hash columns, the tests, or a rebalance hook. It does not change SQL, Kafka configuration, or migrations. It does not change PR #346 or PR #372. It does not close issue #344. Staging and production are unchanged. The consumer must not be started as a production reader on the strength of this proposal.
