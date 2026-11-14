# 0.1O direct DML privilege matrix

Status: `PROPOSED_NOT_APPROVED`. This document does not change grants, roles, or schema.

Audit base: `origin/main` `3f306fc407aeda4bdc652d825954292f4a54fad6`. Reference draft PR #384 remains at `6228504ec2c7a270f40c1b8b058d7d53e8c17860`. Platform migration head observed in this checkout is `000105`. No later migration number is assigned here.

Absence of a Go `UPDATE` is not absence of an access path. A role, a table owner, a superuser, or a `SECURITY DEFINER` function can still write.

## Roles

| Role | Attributes in `000103:17-31` | Membership in migrations | Meaning |
| --- | --- | --- | --- |
| `shipment_runtime` | `NOLOGIN NOSUPERUSER NOINHERIT NOBYPASSRLS` | none | Privileges apply only after `SET ROLE` or through a member whose own role inherits. |
| `freight_migrations` | same shape | none | Owns the pin function and the pin tables. It is not given ownership of the active-link table. |
| `company_runtime` | same shape | none | Relationship administration. No grant on the active-link table was found. |

`000103:6-8` says `freight` stays the shared login until a later role split. `000105:4-5` does not alter that login. The unapplied gate `infrastructure/postgres/tms_activation_enforcement_gate_v0.1.sql:6-8` refuses to install while that login is superuser, bypasses row security, owns the protected tables, or can write pins. An integration test grants `shipment_runtime` to `freight` (`activation_runtime_integration_test.go:607`). That membership is not a migration.

`NOINHERIT` is on these three roles. It does not stop a superuser, and it does not stop a member role that itself has `INHERIT` once a deployment grants membership. No such membership is in the numbered migrations. Deployment membership is `UNKNOWN`.

## Table ownership

`transport.transport_execution_active_shipments` is created at `000088:76-81`. No `ALTER TABLE ... OWNER TO` for that table was found. Ownership stays with the role that applied `000088`. That role is not named in the migration. The owner and any superuser retain `INSERT`, `UPDATE`, and `DELETE` regardless of `REVOKE` from `shipment_runtime`.

`freight_migrations` owns `transport.transport_execution_carrier_relationship_pins` (`000103:176`) and `transport.propagate_execution_relationship_pins(uuid)` (`000103:1150`).

## Sensitive grants

Classification:

- `REQUIRED_BY_PRODUCT` — a product Go statement in `services/` uses this privilege.
- `UNUSED_IN_OBSERVED_CODE` — the grant exists and no product Go statement was found.
- `PRIVILEGED_BYPASS` — the privilege can change authority state outside the intended writer.
- `UNKNOWN` — the path depends on a role or deployment fact this checkout does not fix.

| Object | Role | Privilege | Source | Product use | Class |
| --- | --- | --- | --- | --- | --- |
| `transport.transport_execution_active_shipments` | `shipment_runtime` | `SELECT, INSERT, UPDATE` | `000104:18` | `INSERT` in `transport_execution_repository.go:335-340` | `INSERT` is `REQUIRED_BY_PRODUCT`. `UPDATE` is `UNUSED_IN_OBSERVED_CODE` and `PRIVILEGED_BYPASS`. |
| same table | `shipment_runtime` | `DELETE` | not granted by `000104:18` | none found | not a runtime grant. Owner and superuser remain `PRIVILEGED_BYPASS`. |
| same table | table owner, superuser, shared `freight` login if it is either | all | ownership not reassigned | none named | `PRIVILEGED_BYPASS` |
| `transport.transport_executions` | `shipment_runtime` | `SELECT, INSERT, UPDATE` | `000103:1225` | `INSERT` at repository `:295`. `UPDATE current_revision_id` at `:92-96` and successor `:172-176`. `UPDATE event_seq` at successor `:457-459` and command `:1224-1227`, `:1342-1345` | `REQUIRED_BY_PRODUCT` for those columns. The grant is table-level, so `owner_epoch` does not exist yet and `operating_tenant_id`, `driver_id`, `vehicle_id`, and `id` are still writable. That width is `PRIVILEGED_BYPASS`. |
| same table | `freight_migrations` | `SELECT` | `000103:1223` | pin function reads the execution | `REQUIRED_BY_PRODUCT` for the definer read |
| `transport.transport_execution_revisions` | `shipment_runtime` | `SELECT, INSERT` plus column `UPDATE` including `id`, `execution_id`, `operating_tenant_id`, `status` | `000103:1226-1244` | successor updates `status` at `:151-156` | `REQUIRED_BY_PRODUCT` for `status` and `updated_at`. Identity-column `UPDATE` is `PRIVILEGED_BYPASS`. `DELETE` is revoked at `:1244`. |
| `transport.shipments` | `shipment_runtime` | `SELECT, UPDATE` | `000104:10` | AssignDriver `:433`, AssignVehicle `:473`, status `:521`, command `:1177` | `REQUIRED_BY_PRODUCT`. Table-level `UPDATE` can also change tenant and company columns. That width is `PRIVILEGED_BYPASS`. |
| same table | `shipment_runtime` | `INSERT` | `000105:43` | `CreateShipment` `:108` and award execution `:539` | `REQUIRED_BY_PRODUCT` |
| `transport.transport_execution_carrier_relationship_pins` | `shipment_runtime` | `INSERT, UPDATE, DELETE` revoked | `000103:1245` | no direct Go `INSERT` | direct DML is closed for this role |
| same table | `freight_migrations` through `propagate_execution_relationship_pins` | `INSERT` and revision fingerprint `UPDATE` | function body `:981-1001`, `SECURITY DEFINER` at `:920` | successor calls it at `transport_execution_successor_repository.go:169` | `REQUIRED_BY_PRODUCT` when the function is present. It is a deliberate bypass of the runtime revoke. It does not write the active-link table. |

`PUBLIC` is not granted DML on the active-link table by these migrations. `000104:18` does not grant `DELETE` to `shipment_runtime`.

## What product code writes

| Operation | Statement | Transaction | Locks before the write |
| --- | --- | --- | --- |
| New execution | `INSERT` `:295` | `Project` `:31-126` | none on an existing execution. Flag-on first locks shipment, cargo, driver, and vehicle (`transport_execution_activation.go:263-326`). |
| New active link | `INSERT` `:335` | same `Project` transaction | no `FOR UPDATE` of the link row. A second link for the same shipment fails the primary key. |
| Existing active-link `execution_id` | none | none | `UPDATE` remains possible for `shipment_runtime` without an execution lock |
| Shipment driver or vehicle | `UPDATE` shipments | AssignDriver `:426-454`, AssignVehicle `:467` | no execution lock and no link lock |
| Revision pointer | `UPDATE current_revision_id` | successor `:64-73`, after `FOR UPDATE` of the execution at `:85-89` | execution first |
| Pin copy | `SECURITY DEFINER` function | same successor transaction, after the new revision insert and before the pointer update | caller already holds the execution. The function body has no `FOR UPDATE`. |

## Dependents of the current grants

These product paths fail if `shipment_runtime` loses the listed privilege without a replacement:

- `Project` needs `INSERT` on executions, revisions, and the active-link table.
- Successor and command event emission need `UPDATE` of `current_revision_id`, `event_seq`, and `updated_at` on executions.
- Successor needs `UPDATE` of revision `status`.
- AssignDriver, AssignVehicle, status, accept, cancel, and command shipment transitions need `UPDATE` on shipments.
- Successor needs `EXECUTE` on `transport.propagate_execution_relationship_pins(uuid)` (`000103:1208`). The Go probe treats a missing function as a no-op (`transport_execution_activation.go:367-368`). On a database migrated through `000103`, the function is present and the call runs.

Tracking consumption and binding-decision consumption are not writers of this table. They are outside this matrix and stay closed.
