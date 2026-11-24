# Tracking DML 0.1E — owner epoch enforcement

Status: designed, not implemented. No column and no trigger are created.
`OWNER_EPOCH_MONOTONICITY_DESIGNED=YES`.
`OWNER_EPOCH_MONOTONICITY=SPECIFIED_NOT_ENFORCED`.
`HISTORICAL_DATA_POLICY_APPROVED=NO`.

A second read is not this mechanism. The current consumer can read a tenant at `execution_consumer.go:111` and commit. That read is not a lock on `transport.transport_executions`, and the column it would compare does not exist.

## 1. Stored value

`transport.transport_executions` has no `owner_epoch` column at `000088` or at migration head `000105`. Existing rows are not backfilled. A later column is nullable. Rows present at creation stay null. Null is not treated as `1`, and it is not a reason to commit the Kafka offset.

The shipment insert at `transport_execution_repository.go:296-301` is the only creation path found. The design sets `owner_epoch = 1` in that same `INSERT`. The value is not taken from the HTTP caller, the Kafka payload, or `event_seq`.

## 2. What may change it

A later authorized owner change, which does not exist in this tree, runs in one shipment transaction:

1. `SELECT ... FROM transport.transport_executions WHERE id = $1 FOR UPDATE`.
2. If `owner_epoch` is null, raise. The statement does not fill in a historical null.
3. `UPDATE ... SET owner_epoch = owner_epoch + 1` on that locked row, and no other assignment.
4. Commit. Tracking state is not rewritten. An effect that already committed at the old epoch stays.

The function that contains those statements is `SECURITY DEFINER`, owned by a `NOLOGIN` role that is not `tracking_state_fence` and not the table owner. `shipment_runtime` receives `EXECUTE`. `freight` does not. The body checks `pg_has_role(session_user, 'shipment_runtime', 'USAGE')` and does not read `current_user` as the caller. `SET ROLE` safety is not claimed without a test.

W10, W11, W12, W13, and W14 assign `current_revision_id` or `event_seq`. They must not assign `owner_epoch`. `event_seq` stays the event counter.

`shipment_runtime` today has table-level `UPDATE` at `000103:1225`, so a session that can become that role can assign `owner_epoch` once the column exists. The column-level replacement in the privilege plan is what removes that assignment. Until that replacement is installed, the function is not the only writer. `ALL_WRITERS_FENCED=NO`.

## 3. Trigger

A `BEFORE INSERT OR UPDATE` trigger on `transport.transport_executions` is the backstop, not the business command.

- On insert, `owner_epoch` must be `1`. Any other value, including null, raises.
- On update, if `OLD.owner_epoch` is null, `NEW.owner_epoch` must stay null.
- On update, if `OLD.owner_epoch` is not null, `NEW.owner_epoch` must not be null and must not be lower.
- An increase of more than one is rejected on this table. The authorized command increases by exactly one. A trigger that allowed any higher value would accept a jumped epoch from any role that still has column `UPDATE`.
- The trigger rejects the statement. It does not rewrite the value.

The trigger sees the new row. It does not know which function issued the statement. A role that still has `UPDATE` on `owner_epoch` can still increase it by one without the business function. The trigger closes decrease, nulling, and a jump. The revoke closes the other writers. Both are required. Neither exists today.

A trigger is preferred to a second lookup because it runs in the same statement as the write. A lookup in another statement can be skipped by a direct `UPDATE`.

The trigger is not a substitute for the tracking function. A tracking write still has to lock the execution row and compare the event epoch with the locked value. The trigger does not see the Kafka event.

## 4. Stale epoch and a later owner change

The tracking function writes only when the event epoch equals the locked epoch. An event that carries 4 does not write after the locked row shows 5. A higher event epoch is not authority.

Under `READ COMMITTED`, the `FOR UPDATE` wait returns the latest committed row. A tracking transaction that committed while it held epoch 4 keeps that effect after a later commit of epoch 5. This design does not undo it. A tracking transaction that starts after epoch 5 has committed cannot commit an epoch-4 effect.

A state write after the owner change without taking the execution lock is the current W1 through W4 path. After option B, that direct write is revoked for the runtime login. It remains possible for the table owner, a superuser, and a session that can become `tracking_state_fence`.

## 5. Historical rows

Events and state rows that have no epoch stay `NON_TERMINAL`. Forbidden repairs:

- setting null epochs to `1` because the row exists;
- copying `event_seq` into `owner_epoch`;
- choosing the highest inbox sequence as `last_event_id`.

`HISTORICAL_DATA_POLICY_APPROVED=NO`. No backfill is authorized by this document.
