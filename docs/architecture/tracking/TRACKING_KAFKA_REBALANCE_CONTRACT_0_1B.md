# TRACKING-KAFKA-REBALANCE-CONTRACT-0.1B

Status: design proposal only. Production code, SQL, Kafka configuration, and migrations are unchanged. Implementation of 0.1B has not started.

| Pin | Value |
|---|---|
| Base | `origin/main` `3f306fc407aeda4bdc652d825954292f4a54fad6` |
| Issue | #344, remains OPEN |
| Discovery draft | PR #346, left unchanged: OPEN, draft, head `9b480f17118985c8b4791c66039b2a0525afdb98`, base `f58f13b5b4b539895673a77ff404ab47669dc96d` |
| Library | `github.com/twmb/franz-go v1.21.5` |
| `REBALANCE_DESIGN` | Proposed below. Not proven. Production stays blocked. |
| `PRODUCTION_CONSUMER_READINESS` | `BLOCKED` |

PR #346 is not modified by this document. Its base is behind current `origin/main`. The consumer files are unchanged between those two commits.

## 1. Current consumer-group model

`NewExecutionEventConsumer` builds one `kgo.Client` with:

- `SeedBrokers` from `TRACKING_KAFKA_BROKERS`;
- `ConsumerGroup` default `tracking-execution-stop`;
- `ConsumeTopics` default `shipment.status.v1`;
- `DisableAutoCommit`.

The client does not set a balancer, session timeout, rebalance timeout, heartbeat, `OnPartitionsAssigned`, `OnPartitionsRevoked`, `OnPartitionsLost`, or `BlockRebalanceOnPoll`.

Observed library defaults at v1.21.5:

- Protocol `consumer`.
- Balancer `CooperativeStickyBalancer`.
- Session timeout 45s, rebalance timeout 60s, heartbeat 3s.
- Autocommit interval 5s, which does not run because autocommit is disabled.

`defaultRevoke` commits fetched offsets only when autocommit is enabled. With `DisableAutoCommit` that function returns without a commit. After the callback, the library removes lost partitions from its uncommitted map. A revoke therefore does not by itself acknowledge records the application has not passed to `CommitRecords`.

`CommitRecords` sends `offset + 1` for the records it is given. The library retries retryable group errors. It does not, in the tracking consumer, check that the member still owns the partition. The library documents that a commit during or after a rebalance can duplicate processing and can rewind a cursor another member has already advanced.

`Run` polls, sorts each topic partition by offset, and commits a record after `applyUntil` returns nil, or immediately when `isExecutionStopEvent` returns false. `applyUntil` retries every `Apply` error every 50ms until the context ends. A commit error or a canceled context returns from `Run` and stops every assigned partition. Fetch errors are not classified.

This is at-least-once only while the application does not commit an offset it no longer owns. Exactly-once is not claimed. No broker test has observed a rebalance.

## 2. Partition ownership

A member owns a topic partition only while that partition is in the assignment for its current group generation.

Cooperative sticky assignment moves only the partitions that must move. Partitions that stay assigned keep their owner and their committed cursor. A partition that moves is owned by the new member only after assignment, and that member fetches from the last committed offset, not from the previous owner's local memory.

Ownership ends when `OnPartitionsRevoked` or `OnPartitionsLost` runs for that partition, or when the member is removed because heartbeats or the rebalance timeout lapse. After that, the previous member has no right to advance the cursor.

The in-flight record is not owned work that can be finished after that signal. Its offset stays uncommitted unless a `CommitRecords` call completed while the generation that polled it still owned the partition.

## 3. Late-commit protection

Proposed rule, name `GENERATION_FENCED_MANUAL_COMMIT`. This is not implemented and not tested.

1. When a batch is polled, record the member id, generation, and the set of owned partitions.
2. Register `OnPartitionsRevoked` and `OnPartitionsLost`. Both cancel the context for every listed partition and mark that generation's ownership closed for those partitions. Neither callback commits.
3. `CommitRecords` is called only when all of the following are true:
   - the record has a terminal result from section 5;
   - the partition is still in the owned set;
   - the generation captured at poll equals the generation now;
   - the caller is not inside a revoke or lost callback.
4. A failed or timed-out commit does not advance the local cursor. The same record is replayed from the last acknowledged offset.
5. Do not commit from `defaultRevoke`. Leaving autocommit disabled preserves the current library behavior that a revoke does not acknowledge unprocessed fetches.
6. Do not treat `BlockRebalanceOnPoll` as the safety mechanism. It can keep ownership only until the rebalance timeout, and the library does not enforce an application deadline. A commit that races member removal is not disproven.

The application must refuse the late commit itself. Source reading does not show the broker rejecting a commit for a partition this member no longer owns, and the library explicitly warns that such a commit can rewind another member.

## 4. Durable quarantine

PostgreSQL is the quarantine store. No new Kafka topic is proposed.

`tracking.execution_event_quarantine`, not created here:

- Unique key: `consumer_group`, `topic`, `partition_id`, `message_offset`.
- Original payload, SHA-256, and size. Leader epoch is stored and compared, and is not part of the key.
- Claimed `event_id`, `execution_id`, `event_sequence`, `event_type`, and `operating_tenant_id` are nullable and are not authorization.
- `fence_status` is `OPEN` or `CLEARED` only when both `execution_id` and `event_sequence` are present. That open row is the execution fence.
- Replay with the same hash, size, and epoch does not insert again and does not overwrite. A difference parks the partition, alerts, and does not commit.
- Unknown execution identity may store bytes and must not commit. The partition stays parked.
- The consumer may insert and may read metadata, hash, size, epoch, and fence columns. It may not read the payload, update, or delete.
- An investigator role reads the payload. Customer APIs have no grant.
- Opening writes one append-only audit row in the same transaction. Clearance without an audit row is forbidden. 0.1B does not implement clearance, replay, or deletion.

## 5. Atomicity

`Apply` keeps the inbox insert and the tracking-state change in one database transaction. Failure rolls both back. A `23505` on `uq_execution_event_inbox_sequence` aborts that transaction. Tracking state from that attempt is not stored.

Quarantine is a later transaction. It does not update `execution_tracking_state`. The offset commit is a third step and happens only after the quarantine transaction has committed for a terminal fence, or after `Apply` itself has succeeded.

Terminal results that may commit one offset:

- `Apply` success, including the current HTTP 404 path and an `event_id` duplicate;
- a verified filter-skip of a known non-stop event;
- a committed open fence for a parsed execution id and sequence.

Not terminal: HTTP 401 or 403, invalid JSON, unknown type, hash mismatch, quarantine insert failure, transient failure, an exhausted retry budget, and a canceled context. Those do not commit.

`PARTITION_POLICY=CONTIGUOUS_HOLD` remains: a later offset on the same partition is not committed while an earlier offset lacks a terminal result. Another execution on that partition is not reached by stepping over the gap. Other partitions may continue.

There is no database transaction that contains the Kafka offset. Crash windows are covered by replay and the quarantine unique key, not by exactly-once delivery.

## 6. PostgreSQL 23505

`InsertExecutionInbox` conflicts only on `event_id`. The unique key `uq_execution_event_inbox_sequence` is `(execution_id, event_sequence)`. A different `event_id` for that pair returns `23505`, the transaction rolls back, and the tenant comparison inside `Apply` does not run.

`IsUniqueViolation` matches every `23505` and ignores the constraint name. It must not be used as a reason to skip.

After rollback, a new read classifies the existing inbox row:

| Existing row | Result |
|---|---|
| Same `event_id` | Duplicate success. Commit. No state change. |
| Same operating tenant, different `event_id` | Permanent. Open a fence, then commit that offset. Do not advance tracking state. |
| Different operating tenant | Security invalid. Open a fence, alert, no state change, then commit that offset. |
| No row, or any other constraint name | Unknown. Park. Do not commit. Do not call it permanent. |

A payload tenant id is a claim. It does not authorize a state update. The existing free-sequence path, where a different tenant inserts an inbox row and does not change state, stays as it is until a controller changes it.

## 7. Retry, redelivery, restart, and commit failure

Retry budget for transient and unknown errors: 5 attempts, 200ms initial delay, exponential backoff, 2s cap, jitter ±20%. The working processing deadline is 8 seconds and is not a measured rebalance limit. Budget expiry parks the partition and alerts. It does not commit and does not relabel the error as permanent. The 50ms infinite loop is not the policy.

| Event | Stored | Offset | After restart or redelivery |
|---|---|---|---|
| Duplicate `event_id` | Existing inbox row | Commit | Second apply does not change state |
| Permanent or security conflict | One open fence, state unchanged | Commit only after the fence transaction commits | Hash match, no second payload, commit if still uncommitted |
| Crash before the quarantine insert | Nothing new | Uncommitted | Classify again |
| Crash after the insert, before commit | One fence | Uncommitted | Hash match, then commit |
| Quarantine insert or its commit fails | No new fence | Uncommitted | Stay on that offset |
| `CommitRecords` error or timeout | Terminal row already stored, if that step finished | Unknown to this process | Replay is idempotent. Do not start the next offset. |
| Context cancel | No partial fence | Uncommitted | Replay |
| Revoke or loss before commit | Only transactions that already committed | Last successful commit | The new owner reads that cursor. The old member does not commit. |

Redelivery of the same bytes is safe for `event_id` and for the quarantine key. Redelivery of different bytes at the same group, topic, partition, and offset is a mismatch: keep the first payload, park, and do not commit. Topic recreation can reuse offsets; that case is specified and not tested.

## 8. Two-consumer ownership test

This test is a proposal. It is not authorized to run against staging or production, and it does not exist yet.

Setup: one test topic, two partitions, group `tracking-execution-stop`, two members using the proposed generation fence, autocommit disabled. Isolated test broker only.

1. Member A owns both partitions and has polled offset 10 on partition 0. The record is in flight and is not committed. Offset 9 is the last commit.
2. Start member B in the same group so cooperative sticky moves partition 0, or stop member A after the poll if the assignor does not move it. The test must observe an actual ownership change. A run that never moves the partition does not pass.
3. Member A's revoke or lost callback cancels partition 0. A then attempts `CommitRecords` for offset 10.
4. Expected: the group cursor for partition 0 stays at the last commit from before the move. Member B fetches offset 10. Member A does not move the cursor forward, and a later commit by B is not rewound by A.
5. Repeat with a fence insert that committed in PostgreSQL before the revoke, and with `CommitRecords` still in flight at the revoke. Expected: one quarantine row, and the Kafka cursor advances only if that commit completed while A still owned the partition. Otherwise B replays, the hash matches, and B commits.
6. A second case parks partition 0 and continues partition 1 on the member that still owns partition 1.

A passing result requires the assertions above. Calling the franz-go API is not the pass condition.

## 9. Decisions still requiring approval

1. Accept or reject `GENERATION_FENCED_MANUAL_COMMIT`. It is proposed, not proven.
2. Whether any implementation may start before the two-consumer test has passed. This proposal says no rebalance code ships before that test, and the production consumer stays blocked either way.
3. The 8 second deadline against the 45 second session timeout and 60 second rebalance timeout.
4. Whether a foreign tenant with a free sequence remains an inbox commit without a state update.
5. Investigator role, payload retention, and the later clearance stage. 0.1B does not include clearance or replay.
6. The unresolved-age alert threshold.
7. What to do with draft PR #346. This change does not edit it.

## 10. Boundary

This document does not implement the consumer, the tables, the test, or a rebalance hook. It does not change SQL, Kafka configuration, or migrations. It does not close issue #344. Staging and production are unchanged. The consumer must not be started as a production reader on the strength of this proposal.
