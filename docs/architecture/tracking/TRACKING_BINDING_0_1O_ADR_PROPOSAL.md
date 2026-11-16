# ADR proposal — mandatory writer fencing

Status: `B3_REDESIGNED_PROPOSED_NOT_APPROVED`.

Base: `origin/main` `3f306fc407aeda4bdc652d825954292f4a54fad6`. Reference draft PR #384 at `6228504ec2c7a270f40c1b8b058d7d53e8c17860` is unchanged. This proposal does not implement the decision and does not close `J-DML-001`.

## Problem

`shipment_runtime` may `UPDATE` and `INSERT` `transport.transport_execution_active_shipments` (`000104:18`). Product Go inserts that table and does not update it. Both privileges remain access paths. The foreign key `transport_execution_active_shipments_execution_id_fkey` (`000088:79`) only requires the named execution to exist, and an update of `execution_id` takes `FOR KEY SHARE` on the destination.

The same role has table-level `INSERT` and `UPDATE` on `transport.transport_executions` (`000103:1225`). There is no `owner_epoch` column. After that column exists, the table-level `INSERT` can store a non-zero epoch. A column grant does not stop that while the table grant remains, and a default of `NULL` does not stop an explicit value. A reader lock ends when its transaction ends, so a later direct update is not fenced. A retained wide `UPDATE` would not fence the column either.

The shared `freight` login is still the deployment login (`000103:6-8`, `000105:5`). Table ownership of the active-link table was never transferred. `ACTIVE_LINK_TABLE_OWNER=NOT_PROVEN`. Either fact bypasses a revoke aimed only at `shipment_runtime`. A SQL function cannot see the Go `AuthContext`, so a tenant argument or a session variable is not the gateway decision.

## Decision

Adopt the redesigned option B3 from `TRACKING_BINDING_0_1O_ROLLOUT_OPTIONS.md` as a proposal. The decision label is `B3_REDESIGNED_PROPOSED_NOT_APPROVED`.

The write role is a designed `NOLOGIN` non-superuser, `binding_authority_owner`. It owns the fencing functions and does not own the active-link table. The runtime is not a member of that role and cannot `SET ROLE` to it. Each function is `SECURITY DEFINER`, sets `search_path` to `pg_catalog`, uses schema-qualified names, loses `PUBLIC` execute in the creating transaction, and is executable only by `binding_authority_caller`.

Tenant authority is a ticket that the issuer commits in one session and the writer consumes in a later session. The writer does not require `ticket.xact_id = pg_current_xact_id()`. The issuer locks only the ticket when it revokes an `ISSUED` row. The writer locks the ticket before the business rows. The current shared login cannot be that issuer and the writer at the same time. `TRUSTED_TENANT_AUTHORITY=DESIGN_BLOCKED_BY_SHARED_LOGIN`. This proposal does not treat the designed issuer as a boundary that is already in force.

`Project` loses direct execution `INSERT` and direct active-link `INSERT`. The create function, in the transaction after the committed ticket, inserts the execution with `owner_epoch = 1` and inserts the link. `xmin` may confirm that the execution row belongs to that consume transaction, including a failure when a subtransaction xid differs. It is not the authority check. `INITIAL_INSERT_AUTHORITY=NOT_PROVEN` while the issuer is blocked by the shared login.

Retarget locks the ticket, then the source execution and the destination execution in ascending uuid order, then the shipment, then the active-link row. It rechecks tenant, revision, and epoch under those locks. A stale source rolls back. The destination is not first locked by the foreign-key check after the shipment. AssignDriver and AssignVehicle lock the ticket before either the execution or the shipment. They do not lock the shipment and then lock an execution.

`owner_epoch` is `bigint NULL`. `NULL` means unconfirmed and rejects every execution-targeted change. The create function is the initial writer. Later increments happen only in the fencing transaction, together with the link or shipment change and one audit row. Rollback restores the previous row and drops the audit insert.

The epoch trigger, when enabled, runs for a table owner and for a superuser. An owner can drop or disable it. `session_replication_role = replica` can skip it where that setting is permitted. The trigger is not the authority boundary.

`transport.propagate_execution_relationship_pins(uuid)` is unchanged. Its compatibility is `NOT_PROVEN`.

## Why this B3

The earlier B3 locked one existing execution and left the destination execution to the foreign-key check. That check runs when the link row is updated, after the shipment lock, and cycles with an execution-first command on the destination. The redesigned order locks both executions first.

The earlier B3 also left `Project` its direct `INSERT`. That privilege can attach a shipment to an execution the transaction did not create. The redesign moves that insert into the function and removes the runtime grant.

B1 still moves every writer at once. B2 still cannot make two service commits one database transaction. Neither one removes the shared-login blocker.

## Why this is not approval

The following are not done:

- A non-superuser, non-owner session has not been shown `42501` for direct link `INSERT`, direct link `UPDATE`, and direct epoch `UPDATE`.
- The shared login has not been split. The issuer role does not exist. Tenant authority remains `DESIGN_BLOCKED_BY_SHARED_LOGIN`.
- The active-link table owner is `NOT_PROVEN`.
- `owner_epoch` is not in the schema. Existing rows are not given a value. `NULL` is a refusal, not `1`.
- The retarget deadlock test has not been run. `DEADLOCK_FREE` is not claimed.
- Pin propagation against a revision, a relationship version, an activation subject, an execution-first command, and a retarget has not been run.
- Presenter identity, decision consumption, and broker safety stay unimplemented.
- `transport.propagate_execution_relationship_pins(uuid)` exists in `000103:917-1014`, is granted to `shipment_runtime` at `:1208`, and is called from successor at `transport_execution_successor_repository.go:169`. The function reads the successor and source revisions, updates `pin_set_fingerprint`, and inserts pin rows. Those inserts key-share the revision, the relationship version, and the activation subject. The function does not write the active link. That is not a safety proof.

## Migration rule

Head observed here is `000105`. Do not allocate the next number in this proposal. Count the migration directory again at implementation time. Do not revoke grants in the same migration that only adds the nullable column. Do not edit `000103`, `000104`, or `000105` as part of this design.

## Review questions for Agent J

- Does retarget lock the destination execution before the shipment, with the foreign-key edge included?
- Does a stale source roll back instead of continuing?
- Does any path lock an existing execution after the shipment is held?
- Is direct active-link `INSERT` removed from `shipment_runtime` in the proposal?
- Is the function owner a different role from the unproven table owner, with `PUBLIC` execute revoked?
- Does the epoch model treat null as absent, with no backfill, and is a table-level `INSERT` excluded from setting a non-zero epoch?
- Is the ticket committed by the issuer before the writer consumes it, without requiring one transaction id across two sessions?
- Does a second committed consume fail, and does rollback of the consume transaction leave the ticket `ISSUED`?
- Does the trigger text say that an enabled trigger runs for the table owner and for a superuser?
- Is trusted tenant authority marked `DESIGN_BLOCKED_BY_SHARED_LOGIN` rather than accepted from a function argument?
- Is pin propagation marked `NOT_PROVEN` rather than safe?
- Does the document close `J-DML-001`? It must not.
