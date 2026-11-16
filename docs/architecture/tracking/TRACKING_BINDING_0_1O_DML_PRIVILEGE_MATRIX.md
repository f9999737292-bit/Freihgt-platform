# 0.1O direct DML privilege matrix

Status: `B3_REDESIGNED_PROPOSED_NOT_APPROVED`. This document does not change grants, roles, or schema.

Audit base: `origin/main` `3f306fc407aeda4bdc652d825954292f4a54fad6`. Reference draft PR #384 remains at `6228504ec2c7a270f40c1b8b058d7d53e8c17860`. Platform migration head observed in this checkout is `000105`. No later migration number is assigned here.

Absence of a Go `UPDATE` is not absence of an access path. A role, a table owner, a superuser, or a `SECURITY DEFINER` function can still write.

## Roles observed in migrations

| Role | Attributes in `000103:17-31` and `000105:3` | Membership in migrations | Meaning |
| --- | --- | --- | --- |
| `shipment_runtime` | `NOLOGIN NOSUPERUSER NOINHERIT NOBYPASSRLS` | none | Privileges apply only after `SET ROLE` or through a member whose own role inherits. |
| `freight_migrations` | same shape | none | Owns the pin function and the pin tables. It is not given ownership of the active-link table. |
| `company_runtime` | same shape | none | Relationship administration. No grant on the active-link table was found. |
| `freight` | not altered by `000105:5` | not granted in a numbered migration | Shared login. `000103:6-8` says it stays until a later role split. |

`000105:8-9` does not create an award-link lock function and says that function stays out until a server-owned caller identity exists. The unapplied gate `infrastructure/postgres/tms_activation_enforcement_gate_v0.1.sql` refuses to install while that login is superuser, bypasses row security, owns the protected tables, or can write pins. An integration test grants `shipment_runtime` to `freight` (`activation_runtime_integration_test.go:607`). That membership is not a migration.

`NOINHERIT` is on these three created roles. It does not stop a superuser, and it does not stop a member role that itself has `INHERIT` once a deployment grants membership. No such membership is in the numbered migrations. Deployment membership is `UNKNOWN`.

## Designed roles, not created

| Role | Attributes | Why it exists in the design |
| --- | --- | --- |
| `binding_authority_owner` | `NOLOGIN NOSUPERUSER NOINHERIT NOBYPASSRLS` | Owns the fencing functions and the audit table. `SECURITY DEFINER` makes this the role that writes the link and the epoch. It does not own the active-link table. |
| `binding_authority_caller` | `NOLOGIN NOSUPERUSER NOINHERIT` | The only role granted `EXECUTE` on the fencing functions. It has no `INSERT`, `UPDATE`, or `DELETE` on the link table. |
| `binding_authority_issuer` | `NOLOGIN NOSUPERUSER NOINHERIT` | The only role that may insert an authority ticket. The runtime is not a member and is not granted `EXECUTE` on the issuer. |

`shipment_runtime` and `freight` are not members of these roles and cannot `SET ROLE` to them. The roles are not in a migration. The shared login cannot be substituted for the issuer. `TRUSTED_TENANT_AUTHORITY=DESIGN_BLOCKED_BY_SHARED_LOGIN`.

## Table ownership

`transport.transport_execution_active_shipments` is created at `000088:76-81`. No `ALTER TABLE ... OWNER TO` for that table was found. `ACTIVE_LINK_TABLE_OWNER=NOT_PROVEN`. The owner and any superuser retain `INSERT`, `UPDATE`, and `DELETE` regardless of `REVOKE` from `shipment_runtime`.

`freight_migrations` owns `transport.transport_execution_carrier_relationship_pins` (`000103:176`) and `transport.propagate_execution_relationship_pins(uuid)` (`000103:1150`). That ownership is not reused for the active link.

## Sensitive grants observed today

Classification:

- `REQUIRED_BY_PRODUCT` — a product Go statement in `services/` uses this privilege.
- `UNUSED_IN_OBSERVED_CODE` — the grant exists and no product Go statement was found.
- `PRIVILEGED_BYPASS` — the privilege can change authority state outside the intended writer.
- `UNKNOWN` — the path depends on a role or deployment fact this checkout does not fix.

| Object | Role | Privilege | Source | Product use | Class |
| --- | --- | --- | --- | --- | --- |
| `transport.transport_execution_active_shipments` | `shipment_runtime` | `SELECT, INSERT, UPDATE` | `000104:18` | `INSERT` in `transport_execution_repository.go:335-340` | `INSERT` is `REQUIRED_BY_PRODUCT` today and is also `PRIVILEGED_BYPASS`: the foreign key only requires the execution to exist. `UPDATE` is `UNUSED_IN_OBSERVED_CODE` and `PRIVILEGED_BYPASS`. |
| same table | `shipment_runtime` | `DELETE` | not granted by `000104:18` | none found | not a runtime grant. Owner and superuser remain `PRIVILEGED_BYPASS`. |
| same table | table owner, superuser, shared `freight` login if it is either | all | ownership not reassigned | none named | `PRIVILEGED_BYPASS`. Owner is `NOT_PROVEN`. |
| `transport.transport_executions` | `shipment_runtime` | `SELECT, INSERT, UPDATE` | `000103:1225` | `INSERT` at repository `:295`. `UPDATE current_revision_id` at `:92-96` and successor `:172-176`. `UPDATE event_seq` at successor `:459` and command `:1226`, `:1344` | `REQUIRED_BY_PRODUCT` for the columns `Project` writes today. The `INSERT` is table-level, so a future `owner_epoch` can be set to a non-zero value in that `INSERT`. A column-level `INSERT` grant does not override this table grant. That width is `PRIVILEGED_BYPASS`. It is not epoch fencing. |
| same table | `freight_migrations` | `SELECT` | `000103:1223` | pin function reads the execution | `REQUIRED_BY_PRODUCT` for the definer read |
| `transport.transport_execution_revisions` | `shipment_runtime` | `SELECT, INSERT` plus column `UPDATE` including `id`, `execution_id`, `operating_tenant_id`, `status` | `000103:1226-1244` | successor updates `status` at `:151-156` | `REQUIRED_BY_PRODUCT` for `status` and `updated_at`. Identity-column `UPDATE` is `PRIVILEGED_BYPASS`. `DELETE` is revoked at `:1244`. |
| `transport.shipments` | `shipment_runtime` | `SELECT, UPDATE` | `000104:10` | AssignDriver `:433`, AssignVehicle `:473`, status `:521`, command `:1177` | `REQUIRED_BY_PRODUCT`. Table-level `UPDATE` can also change tenant and company columns. That width is `PRIVILEGED_BYPASS`. |
| same table | `shipment_runtime` | `INSERT` | `000105:43` | `CreateShipment` `:108` and award execution `:539` | `REQUIRED_BY_PRODUCT` |
| `transport.transport_execution_carrier_relationship_pins` | `shipment_runtime` | `INSERT, UPDATE, DELETE` revoked | `000103:1245` | no direct Go `INSERT` | direct DML is closed for this role |
| same table | `freight_migrations` through `propagate_execution_relationship_pins` | `INSERT` and revision fingerprint `UPDATE` | function body `:981-1001`, `SECURITY DEFINER` at `:920` | successor calls it at `transport_execution_successor_repository.go:169` | `REQUIRED_BY_PRODUCT` when the function is present. It is a deliberate bypass of the runtime revoke. It does not write the active-link table. |

`PUBLIC` is not granted DML on the active-link table by these migrations. `000104:18` does not grant `DELETE` to `shipment_runtime`. PostgreSQL does grant `EXECUTE` on a new function to `PUBLIC` until `REVOKE ALL ON FUNCTION ... FROM PUBLIC`. The pin functions receive that revoke at `000103:1184-1198`.

## Proposed privilege target

This target is not applied. It is the privilege shape of the redesigned B3 contract.

| Privilege | `shipment_runtime` | `binding_authority_owner` | `binding_authority_caller` |
| --- | --- | --- | --- |
| Active-link `INSERT` | removed | held, and used only by the create function | absent |
| Active-link `UPDATE` and `DELETE` | removed | held, and used only by the retarget function | absent |
| Execution table-level `INSERT` | removed. A remaining table grant would let runtime set `owner_epoch` | held, and used only by the create function, which sets the epoch from the committed ticket | absent |
| Execution column `INSERT` of `owner_epoch` | absent. Not a control while the table-level grant remains | held with the table insert inside the function | absent |
| Execution `UPDATE` of `current_revision_id`, `event_seq`, `updated_at` | kept for successor and commands | not the command path | absent |
| Execution `UPDATE` of `owner_epoch`, `operating_tenant_id`, `id`, `driver_id`, `vehicle_id` | absent | `owner_epoch` only, inside the fencing function | absent |
| Fencing function `EXECUTE` | absent | owner, not a caller | granted |
| Issuer function `EXECUTE` | absent | absent | absent |
| Audit `INSERT` | absent | held, inside the same transaction as the change. The required field is `audit.ticket_id` | absent |

A reader `FOR SHARE` on PostgreSQL 16 still needs `UPDATE` on at least one column of each locked table. That column is not `execution_id` and not `owner_epoch`. A grant on `updated_at` leaves a direct update of that column possible. The fencing function does not close that hole.

`REVOKE UPDATE` on the active-link table does not close the boundary while `INSERT` remains, and it does not close the boundary for a superuser or for the unproven table owner. A rollback that grants the previous `UPDATE` or `INSERT` returns to this open state. It is not a safe default.

## What product code writes

| Operation | Statement | Transaction | Locks before the write |
| --- | --- | --- | --- |
| New execution | `INSERT` `:295` | `Project` `:31-126` | none on an existing execution. Flag-on first locks shipment, cargo, driver, and vehicle (`transport_execution_activation.go:67-71` and `:259-326`). |
| New active link | `INSERT` `:335` | same `Project` transaction | no `FOR UPDATE` of the link row. A second link for the same shipment fails the primary key. The foreign key key-shares `execution_id`. Today that can be an execution the statement did not create. |
| Existing active-link `execution_id` | none | none | `UPDATE` remains possible for `shipment_runtime` without an execution lock. The foreign key would then key-share the destination. |
| Shipment driver or vehicle | `UPDATE` shipments | AssignDriver `:426-454`, AssignVehicle `:467` | shipment row lock from the `UPDATE`. No execution lock and no link lock. The redesign does not add an execution lock after this shipment lock. |
| Revision pointer | `UPDATE current_revision_id` | successor `:64-73`, after `FOR UPDATE` of the execution at `:85-89` | execution first |
| Pin copy | `SECURITY DEFINER` function | same successor transaction, after the new revision insert and before the pointer update | caller already holds the execution. The function body has no `FOR UPDATE`. Inserts key-share the revision, the relationship version, and the activation subject. |

## Dependents of the current grants

These product paths fail if `shipment_runtime` loses the listed privilege without a replacement:

- `Project` needs a replacement before it loses `INSERT` on executions. The create function is that replacement and is also the only writer of `owner_epoch` on insert. Removing the table `INSERT` before the function exists stops `Project`. The function replaces `INSERT` on the active-link table for the same reason. Removing that `INSERT` before the function exists stops `Project`.

The future denial tests, not run here, are the proof table in `TRACKING_BINDING_0_1O_WRITER_FENCING_CONTRACT.md`. They include a committed ticket consumed by a later transaction, a double consume, an expired or revoked ticket, another tenant or principal, a runtime `INSERT` of `owner_epoch = 5`, direct link `INSERT` and `UPDATE`, retarget against a command, and pin propagation. They also include a `CREATE_ACTIVE_LINK` ticket fixed to execution E1, a null execution id, a writer-selected E2, an execution that already exists, the wrong shipment or tenant, a second consume after commit, rollback with the ticket left `ISSUED`, and a different execution id from the same top-level transaction. `audit.ticket_id` is the audit field. Presenting the ticket does not prove presenter identity.
- Successor and command event emission need `UPDATE` of `current_revision_id`, `event_seq`, and `updated_at` on executions.
- Successor needs `UPDATE` of revision `status`.
- AssignDriver, AssignVehicle, status, accept, cancel, and command shipment transitions need `UPDATE` on shipments. Assignment also needs the execution-first function before a linked assignment can bump an epoch. The current statements do not bump one.
- Successor needs `EXECUTE` on `transport.propagate_execution_relationship_pins(uuid)` (`000103:1208`). The Go probe treats a missing function as a no-op (`transport_execution_activation.go:367-368`). On a database migrated through `000103`, the function is present and the call runs.

Tracking consumption and binding-decision consumption are not writers of this table. They are outside this matrix and stay closed.
