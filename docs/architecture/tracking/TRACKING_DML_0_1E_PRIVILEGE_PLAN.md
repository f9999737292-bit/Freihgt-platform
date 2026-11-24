# Tracking DML 0.1E — privilege revocation plan

Status: sequence only. No role, grant, or owner was changed.
`DB_ROLES_CHANGED=NO`.
`PUBLIC_EXECUTE_AUDIT=REQUIRED`.
`SHARED_FREIGHT_RISK=OPEN`.

## 1. Owner and inheritance today

No migration through `000105` assigns an owner for `transport.transport_executions`, `tracking.execution_tracking_state`, `tracking.execution_event_inbox`, or `tracking.execution_stop_eta_state`. `000103:174-176` assigns `freight_migrations` as owner of the pin tables and the activation type. `000103:1138-1152` assigns that role as owner of the activation functions. Those statements do not cover the four tables in this plan.

`000103:6-7` says `freight` remains the shared login. The tracking default login name is `freight` at `services/tracking-service/internal/config/config.go:87`. If the migration session was `freight`, `freight` owns the four tables. The live owner was not queried. `LIVE_TABLE_OWNER=NOT_QUERIED`.

`shipment_runtime` and `company_runtime` are `NOLOGIN` and `NOINHERIT` (`000103:24-32`). No production Go path executes `SET ROLE`, so those roles do not inherit into the running session. `000103:1225` still grants `shipment_runtime` table-level `UPDATE` on `transport.transport_executions`. That grant includes `operating_tenant_id`. It is unused by the current binary and available to any session that can become `shipment_runtime`.

`freight` receives no separate `GRANT` in `000103` or `000105` because the design treated it as the shared login rather than as a limited role. A table owner does not need that grant.

## 2. Public execute

Schema `USAGE` lets a role look up a function. It does not grant `EXECUTE`. A new function receives `EXECUTE` for `PUBLIC` unless it is revoked. After `USAGE` is granted, every function that still has `PUBLIC` `EXECUTE` becomes callable.

The up migrations through `000105` were read. `pg_proc.proacl` was not. `PUBLIC_EXECUTE_AUDIT=REQUIRED`.

`000103:1186-1198` includes `REVOKE ALL ... FROM PUBLIC` for the transport functions that migration creates. Lines 1207-1213 and 1215 grant `EXECUTE` on transport functions to `shipment_runtime` or `freight_migrations`. `tracking_runtime` is not a grantee.

These earlier functions return `trigger` and have no `REVOKE ... FROM PUBLIC` in the migration that creates them:

- `transport.deny_transport_order_rate_snapshot_mutation()` in `000051`
- `transport.deny_shipment_cargo_execution_evidence_mutation()` in `000082`
- `transport.reject_transport_execution_stop_reparent()` and `transport.reject_transport_execution_revision_mismatch()` in `000088`
- `transport.reject_completed_transport_execution_stop_mutation()` and `transport.reject_completed_transport_execution_action_mutation()` in `000089`
- `transport.deny_delivery_disposition_append_only()` and `transport.reject_delivery_disposition_fact_rewrite()` in `000093`
- `transport.reject_pod_requirement_decision_mutation()` and `transport.validate_pod_requirement_decision_insert()` in `000098`

A direct call of a trigger function fails because it is a trigger function. That failure is not a revoke. The new protected functions must revoke `PUBLIC` in the same migration that creates them. The catalog audit is a gate, not a result of this file read.

## 3. Revocation sequence

This order is a design. A later migration number is not chosen and is not reserved.

1. Controller approves `DB_ROLE_SPLIT_APPROVED` and the ownership move. Until that approval, stop.
2. Create `tracking_table_owner` as `NOLOGIN`, `NOSUPERUSER`, `NOBYPASSRLS`, with no membership granted to `freight` or to `tracking_service_login`. Move ownership of the three tracking tables, and of `transport.transport_executions` only if the controller also approves that move, to `tracking_table_owner`. The execution-table move also affects shipment. It is a separate controller decision from the tracking-table move. If `freight` remains the owner of the tracking tables, the later revoke does not fence `freight`.
3. Create `tracking_state_fence`, `tracking_runtime`, and `tracking_service_login` as specified in the protected-write contract. Create the functions. Revoke `PUBLIC`. Grant `EXECUTE` only to `tracking_runtime`.
4. Replace `000103:1225` for a future migration with column-level `UPDATE` on `transport.transport_executions` that excludes `operating_tenant_id` and `owner_epoch`. Grant `EXECUTE` on the separate epoch function to `shipment_runtime` only. Do not grant that function to `freight`.
5. `REVOKE INSERT, UPDATE, DELETE` on the three tracking tables from `PUBLIC`, from `tracking_service_login`, and from `freight`. Also revoke them from `shipment_runtime` if any later grant added them. No such tracking grant exists through `000105`.
6. Change the tracking service login to `tracking_service_login`. That change is a deployment action. This document does not write a connection string.
7. Start the new binary only through the gate in section 5. Do not start the old binary.

ETA and approach cannot keep working after step 5 if they still issue W4 and W7. Those statements lose their privilege. They work after step 5 only when their call sites use the protected functions and those functions exist. Shipping step 5 before the new binary is a deliberate outage of those writes, which is the fail-closed side of the cutover. It is not a partial mode.

The old consumer can continue a direct `UPDATE` when any of these is true: it still connects as the table owner, its login still has `UPDATE`, or it is a superuser. The function does not remove a privilege from a binary that is already running. Stopping that binary is an operator action. `OLD_CONSUMER_CAN_BE_STARTED_AGAIN=YES` until the gate below is an enforced deployment control.

## 4. What freight keeps

Until step 2, `freight` keeps owner rights. After step 2, `freight` keeps whatever explicit grants it still has. The design gives it no `EXECUTE` and no DML on the tracking tables. Owner rights are gone only if step 2 actually moved the tables and a catalog query confirms the new owner. This stage did not run that query.

`freight` is not a member of `tracking_runtime` in this design. A later membership would make `USAGE` true and would defeat the caller check. That membership is forbidden by the contract and is not yet prevented by a server constraint.

## 5. Fail-closed rollout gate

`MIXED_CONSUMER_GATE=CLOSED`. The new process does not join `tracking-execution-stop` until all of the following are true. None of them is implemented.

- The process `session_user` is `tracking_service_login`, not `freight`.
- `has_table_privilege` for that session is false for `INSERT`, `UPDATE`, and `DELETE` on `tracking.execution_tracking_state` and `tracking.execution_event_inbox`.
- `EXECUTE` on the protected functions is true.
- Coordinator membership for group `tracking-execution-stop` shows no member on the old contract. `Client.GroupMetadata` does not return the roster. The gate uses a full group description. PR #357 remains the reference for that limit and is not edited.
- A member that appears without the new contract version causes this process to leave the group and stop. It does not commit offsets for that member.
- The controller has set `DIRECT_DML_ENFORCEMENT_APPROVED` and `ALL_WRITERS_MIGRATION_PLAN_APPROVED`. Both are `NO` in this stage.

A failed check exits the process. It does not fall back to direct SQL. An old image is not started by this gate. Preventing an operator from starting the old image is a deployment control outside PostgreSQL. The database side is the revoke, and it does not apply to the table owner.

## 6. Rollback

The down migration restores the previous grants and the previous owner. Restoring DML while a consumer is running reopens direct writes. The down migration is therefore allowed only after every tracking consumer, ETA writer, and approach writer has stopped. It does not drop historical rows and it does not rewrite epochs. If any new epoch column is non-null, the down migration stops, matching the historical-column rule already recorded for the future proof columns.

Rollback is fail-open for the fence. It is not a routine recovery from a bad event. A bad event rolls back its own transaction and leaves the grants in place.

## 7. Privilege matrix

Designed cells are `DESIGNED_NOT_APPLIED`. Cells marked as installed are current grants and are not new rights. The installed grant that this matrix replaces is `000103:1225`: `shipment_runtime` has table-level `SELECT, INSERT, UPDATE` on `transport.transport_executions`. `freight_migrations` has installed `SELECT` on that table at `000103:1223` and schema `USAGE` at `000103:50`. No installed grant on schema `tracking` was found through `000105`.

`tracking_service_login` has no direct privilege. It inherits `tracking_runtime` and therefore has only the `EXECUTE` and schema `USAGE` of that role.

| Role | Schema | Object | SELECT | INSERT | UPDATE | DELETE | EXECUTE |
|---|---|---|---|---|---|---|---|
| `tracking_runtime` | `tracking` | `execution_tracking_state` | no | no | no | no | no |
| `tracking_runtime` | `tracking` | `execution_event_inbox` | no | no | no | no | no |
| `tracking_runtime` | `tracking` | `execution_stop_eta_state` | no | no | no | no | no |
| `tracking_runtime` | `transport` | `transport_executions` | no | no | no | no | no |
| `tracking_runtime` | `tracking`, `transport` | read and write functions in the protected-write contract | no | no | no | no | yes |
| `tracking_state_fence` | `tracking` | `execution_tracking_state`, `execution_event_inbox` | yes | yes | yes | no | no |
| `tracking_state_fence` | `tracking` | `execution_stop_eta_state` | yes | yes | yes | no | no |
| `tracking_state_fence` | `transport` | `transport_executions` | `id`, `operating_tenant_id`, `owner_epoch` | no | `updated_at` only | no | no |
| `tracking_read_fence` | `tracking` | the columns read by `GetState`, `ListActiveTargetsForActors`, and `GetExecutionStopETA` | yes | no | no | no | no |
| `tracking_read_fence` | `tracking` | `execution_event_inbox` | no | no | no | no | no |
| `tracking_read_fence` | `transport` | `transport_executions` | no | no | no | no | no |
| `tracking_execution_fence` | `transport` | `transport_executions` | `id`, `operating_tenant_id`, `owner_epoch` | no | `updated_at` only | no | no |
| `tracking_execution_fence` | `tracking` | the three tracking tables | no | no | no | no | no |
| `shipment_runtime` | `transport` | `transport_executions` | yes | yes | column list excluding `operating_tenant_id` and `owner_epoch` | no | epoch function only |
| `shipment_runtime` | `tracking` | the three tracking tables | no | no | no | no | no |
| `freight` | `tracking`, `transport` | the four tables and the new functions | no designed grant | no designed grant | no designed grant | no designed grant | no |
| `freight_migrations` | `transport` | `transport_executions` | yes, as installed at `000103:1223` | no | no | no | activation function only, as installed |
| `freight_migrations` | `tracking` | the three tracking tables | no | no | no | no | no |

Schema `USAGE` is designed for `tracking_runtime`, `tracking_state_fence`, and `tracking_read_fence` on `tracking` and, for the first two, on `transport`. `tracking_read_fence` does not need `transport`. `CREATE` is granted to none of them. `freight` receives no designed membership in `tracking_runtime`. If `freight` owns a table, the `no` cells do not bind it. `SHARED_FREIGHT_RISK=OPEN`.

`tracking_execution_fence` is the lock-only role from PR #375. Option B's write functions already lock the execution row, so this role is not the Go read path and it is not given tracking DML. It remains in the matrix so the lock role does not silently gain the state table.
