# Tracking Kafka 0.1C — persistence design

Status: specified, not created.
`PERSISTENCE_MODEL_DESIGNED=YES`.
`LAST_EVENT_ID_SPECIFIED=YES`.
`OWNER_EPOCH_SPECIFIED=YES`.
`HASH_AND_SIZE_SPECIFIED=YES`.
`HISTORICAL_REPLAY_POLICY=NON_TERMINAL_NO_BACKFILL`.

Platform migration head at base `3f306fc407aeda4bdc652d825954292f4a54fad6` is `000105_platform_postgres_role_foundation_v0_2_i1`. This stage does not add a migration and does not reserve the next number. A later task must read the head again before choosing a number.

The existing unique key `tracking.execution_event_inbox (execution_id, event_sequence)` and the primary key `event_id` stay as they are until a separate migration decision. This design does not replace either key.

## 1. Current rows

`000091_tms_execution_stop_tracking_v0_1d.up.sql` creates:

- `tracking.execution_event_inbox`: `event_id`, `operating_tenant_id`, `execution_id`, `event_sequence`, `event_type`, `processed_at`. There is no payload, hash, size, or epoch.
- `tracking.execution_tracking_state`: `last_execution_event_sequence` and `operating_tenant_id`. There is no `last_event_id` and no `owner_epoch`. The primary key is `execution_id`.

`InsertExecutionInbox` uses `ON CONFLICT (event_id) DO NOTHING` and does not store bytes (`services/tracking-service/internal/repository/execution_tracking_repository.go`).

`UpsertExecutionTrackingState` in that file lists `operating_tenant_id = EXCLUDED.operating_tenant_id` in the `SET` clause. The `WHERE` clause admits the update only when the stored `operating_tenant_id` already equals `EXCLUDED.operating_tenant_id` and the stored sequence is older than the candidate. A candidate for tenant B therefore changes zero rows of a state row owned by tenant A. The stored tenant stays A. `ClearExecutionTarget` and `TouchExecutionSequence` use the same tenant equality predicate on `UPDATE`. This stage does not change that SQL.

A conflict update that changes zero rows is not a state effect. `RowsAffected = 0` does not prove that the event was processed. The current Go functions return the driver error and ignore `RowsAffected`, so a nil error with zero rows is still `NON_TERMINAL` under this contract. The offset stays uncommitted.

The `WHERE` predicate applies to an existing `execution_id`. An insert of a missing `execution_id` stores the candidate tenant because there is no stored row to compare. That insert still needs the mandatory epoch check in section 4 before it can be `COMMITTED_STATE_EFFECT`.

## 2. Future columns

Added nullable, with no backfill:

| Location | Column | Rule |
|---|---|---|
| `tracking.execution_event_inbox` | `payload_sha256` | Lowercase hex of SHA-256 over the exact Kafka record value. Null on every existing row. |
| `tracking.execution_event_inbox` | `payload_size` | `len(raw)` as a non-negative integer. Null on every existing row. |
| `tracking.execution_tracking_state` | `last_event_id` | The `event_id` of the event whose committed effect last changed this row. Null on every existing row. |
| `tracking.execution_tracking_state` | `owner_epoch` | The shipment epoch copied from the event that wrote the row. Null on every existing row. |

Checks, applied only when the new value is present:

- `payload_sha256` matches `^[0-9a-f]{64}$`.
- `payload_size` is greater than or equal to zero.
- `payload_sha256` and `payload_size` are both null or both present.
- `owner_epoch` is greater than zero.
- `last_event_id` references `tracking.execution_event_inbox.event_id` when it is not null. The inbox insert happens earlier in the same transaction, so the reference exists before commit.
- A non-null `last_event_id` requires the inbox row's `execution_id` to equal the state row's `execution_id`. That check is a later constraint or a transaction assertion. It is not created here.

The unique sequence key stays. A second `event_id` for the same `(execution_id, event_sequence)` still raises unique violation `23505` on `uq_execution_event_inbox_sequence`. That violation remains `NON_TERMINAL`. The new columns do not make it a skip.

## 3. Hash and size

The digest is SHA-256 of the `[]byte` passed to `Apply`, in order, with no JSON rewrite and no whitespace change. The stored form is lowercase hex. The size is the length of that slice.

A usable pair is a 64-character lowercase hex digest plus an equal size. Null, empty string, uppercase hex, a wrong length, and a size that differs from `len(raw)` are unusable. Two empty strings are not a match. The digest of a zero-length slice is a real 64-character value and is still not a business skip.

The consumer compares a redelivery with the stored pair. Any unusable or unequal pair is `NON_TERMINAL`. The stored row is left unchanged.

## 4. State-effect rule for a new event

`OWNER_EPOCH_MANDATORY=YES`. Every new `COMMITTED_STATE_EFFECT` includes a proven `owner_epoch`. There is no intermediate mode in which the inbox stores `event_id` while the epoch check is still absent. A transaction that fails the epoch check rolls back the inbox insert and the state change together.

A new effect is `COMMITTED_STATE_EFFECT` only when one transaction does all of the following:

1. The event carries `owner_epoch`, and the value is an integer greater than zero.
2. The authoritative epoch is read from shipment ownership under the rule in `TRACKING_KAFKA_0_1C_OWNER_FENCE.md`.
3. The event epoch equals that authoritative epoch.
4. A mismatch, a missing epoch, or an unavailable authoritative read updates no state row. The attempt is `NON_TERMINAL`. It is not terminal, and it does not commit a Kafka offset.
5. The inbox insert stores this `event_id`, tenant, execution, sequence, event type, hash, size, and the same epoch. That insert remains only when this transaction commits.
6. The locked state row changes in the way the operation required, including a non-null `owner_epoch` equal to the authoritative epoch.
7. `RowsAffected` for that mutation is greater than zero. Zero rows is `NON_TERMINAL` even when the driver returns nil. A tenant mismatch is this case for an existing row.
8. The same transaction reads the locked row again before commit.
9. The re-read `last_execution_event_sequence` equals this event's sequence. A greater value is a different event.
10. The re-read `last_event_id` equals this `event_id`.
11. The re-read tenant equals the trusted owner, and the payload claim equals that owner.
12. The re-read `owner_epoch` equals the event epoch and the authoritative epoch.
13. The transaction commits.

A second read of the epoch, without a lock held in the same transaction as the shipment epoch row through commit, is not a cross-service fence. Section 9 of the owner-fence document records that limit.

A tenant conflict or an event-identity conflict rolls back. No new inbox row is kept, and the offset stays uncommitted.

## 5. Replay proof

`VERIFIED_EQUIVALENT_PRIOR_EFFECT` requires every item below. A subset is not enough:

- the same `event_id`, tenant, execution, sequence, and event type;
- a usable SHA-256 and an equal size;
- `last_event_id` on the state row equals this `event_id`;
- `last_execution_event_sequence` equals this event's sequence;
- `owner_epoch` is present on the event and on the state row, and both equal the authoritative epoch. A missing epoch fails the replay.

Sequence equality is necessary and not sufficient. A higher sequence with a different `event_id` is `NON_TERMINAL`. The attempt rolls back, writes no new inbox row, and does not commit the offset.

Until `last_event_id` exists and is non-null for that execution, replay stays `NON_TERMINAL`. This document does not declare the proof implemented.

## 6. Historical compatibility

Existing inbox and state rows remain valid under the old columns. New columns stay null. A null proof column means the historical effect is not proven.

Forbidden backfill:

- computing a hash for a row whose original bytes were not stored;
- setting `last_event_id` to the inbox `event_id` that shares the highest sequence;
- copying `event_seq` into `owner_epoch`;
- treating a unique-key match as proof that the state row belongs to that event.

Any of those writes can mark an event terminal when a different event, or no durable payload, produced the state. Historical replay therefore stays `NON_TERMINAL`. The partition remains on that offset under the contiguous-hold rule from PR #357.

Issue #344 is a rejected second insert. The losing event is not an inbox row. An inbox scan cannot reconstruct it. Recovery reads the uncommitted Kafka record.

## 7. Migration and rollback plan

A later migration, numbered only after a fresh head check:

- adds the four nullable columns and the checks in section 2;
- does not update existing rows;
- does not drop or replace `uq_execution_event_inbox_sequence` or the `event_id` primary key;
- does not grant payload reads to customer roles.

Down migration:

- stops when any new column is non-null;
- drops only the new columns and their checks when every value is null;
- does not delete inbox rows, state rows, or offsets.

Nulling a proof column in order to force a down migration is a separate controller decision. It is not part of this down plan. A consumer that has written a non-null proof column has left the reversible window.

## 8. Acceptance

- `EXISTING_SEQUENCE_UNIQUE_KEY_PRESERVED=YES`.
- `BACKFILL=FORBIDDEN`.
- `NULL_PROOF_IS_NON_TERMINAL=YES`.
- `ROWS_AFFECTED_REQUIRED=YES`.
- `ROWS_AFFECTED_ZERO_IS_NOT_TERMINAL=YES`.
- `UPSERT_TENANT_TRANSFER=NO`.
- `SEQUENCE_EQUALITY_REQUIRED=YES`.
- `LAST_EVENT_ID_EQUALITY_REQUIRED=YES`.
- `OWNER_EPOCH_MANDATORY=YES`.
- `EPOCH_MISMATCH_COMMITS_OFFSET=NO`.
- `SECOND_READ_IS_CROSS_SERVICE_FENCE=NO`.
- `MIGRATION_CREATED=NO`.
- `MIGRATION_HEAD_UNCHANGED=000105`.
