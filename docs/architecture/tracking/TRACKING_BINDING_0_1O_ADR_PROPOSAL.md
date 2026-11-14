# ADR proposal — mandatory writer fencing

Status: `PROPOSED_NOT_APPROVED`.

Base: `origin/main` `3f306fc407aeda4bdc652d825954292f4a54fad6`. Reference draft PR #384 at `6228504ec2c7a270f40c1b8b058d7d53e8c17860` is unchanged. This proposal does not implement the decision and does not close `J-DML-001`.

## Problem

`shipment_runtime` may `UPDATE transport.transport_execution_active_shipments` (`000104:18`). Product Go inserts that table and does not update it. The privilege remains an access path. The same role has table-level `UPDATE` on `transport.transport_executions` (`000103:1225`). There is no `owner_epoch` column. A reader lock ends when its transaction ends, so a later direct update is not fenced.

The shared `freight` login is still the deployment login. Table ownership of the active-link table was never transferred. Either fact bypasses a revoke aimed only at `shipment_runtime`.

## Decision

Adopt option B3 from `TRACKING_BINDING_0_1O_ROLLOUT_OPTIONS.md`, subject to a later approval.

Keep `Project`, successor, return, redirect, and command writes in shipment-service. Take `UPDATE` and `DELETE` of an existing active-link row away from `shipment_runtime`. Perform that replacement in one `SECURITY DEFINER` function that locks the existing execution, then the shipment, then the link, checks the locked tenant, the active revision, and a non-null matching `owner_epoch`, writes one change, and increments the epoch.

When AssignDriver or AssignVehicle changes a shipment that already has an active link, that transaction locks the linked execution first and bumps the epoch, then updates the shipment. It does not lock the execution after the shipment. Activation continues to insert a new execution after its shipment locks and does not call the retarget function.

Narrow execution `UPDATE` to `current_revision_id`, `event_seq`, and `updated_at`.

## Why B3

B1 moves every writer at once and couples `Project`, commands, and assignment to one cutover. B2 puts the writes in another process but cannot make two service commits one database transaction. B3 removes the observed link-update grant and adds the epoch on the two changes that replace assignment identity, without reversing the execution-first command order or the shipment-first activation insert.

## Why this is not approval

The following are not done:

- A non-superuser, non-owner session has not been shown `42501` for direct link `UPDATE` and direct epoch `UPDATE`.
- The shared login has not been split and does not stop being superuser or table owner by this proposal.
- `owner_epoch` is not in the schema. Existing rows are not given a value.
- Tenant match against a locked row is not a tenant-isolation implementation.
- Presenter identity, decision consumption, and broker safety stay unimplemented.
- `transport.propagate_execution_relationship_pins(uuid)` exists in `000103:917-1014`, is granted to `shipment_runtime` at `:1208`, and is called from successor at `transport_execution_successor_repository.go:169` after the new revision insert and before the pointer update. The caller already holds the execution `FOR UPDATE`. The function reads the revision and execution, updates `pin_set_fingerprint`, and inserts pin rows. It has no `FOR UPDATE` and does not write the active link. That is not a safety proof. An isolated interleaving of this call is required before implementation. `0.1N` did not install the function.

## Migration rule

Head observed here is `000105`. Do not allocate the next number in this proposal. Count the migration directory again at implementation time. Do not revoke grants in the same migration that only adds the nullable column.

## Review questions for Agent J

- Does any proposed edge lock an existing execution after the shipment is held?
- Does B3 leave `Project` `INSERT` intact while removing link `UPDATE` from `shipment_runtime`?
- Does the epoch model treat null as absent rather than as `1`?
- Does the proposal still depend on the shared login and the untransferred table owner?
- Is pin propagation marked unproven rather than safe?
- Does the document close `J-DML-001`? It must not.
