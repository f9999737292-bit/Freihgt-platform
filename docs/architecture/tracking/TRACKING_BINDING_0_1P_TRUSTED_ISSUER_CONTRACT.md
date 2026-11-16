# Trusted issuer contract — 0.1P

Status: `DESIGN_ONLY`. This does not approve B3, does not close `J-DML-001`, and does not install a role.

The 0.1O ticket lifecycle stays as specified on draft PR #385: the issuer commits the ticket, and a later transaction consumes it. This document answers only how those two sessions become different database principals.

## The boundary

PostgreSQL can see `current_user`, role membership, and grants. It cannot see the gateway `AuthContext`.

These are not a tenant boundary:

- a tenant argument to a SQL function
- a custom GUC set by the same session
- `caller_tenant_id = locked_row.tenant_id`
- a shared login that can both insert the ticket and perform the write

The check that works is a login the writer process cannot authenticate as. Only that login may insert a ticket. The writer login is not a member of it and is not granted its privileges, so `SET ROLE` to it fails.

`TRUSTED_TENANT_AUTHORITY` stays `DESIGN_BLOCKED_BY_SHARED_LOGIN` until that login split is installed and tested. The roles below are not in the migrations.

## Issuer

`binding_authority_issuer` is a future `LOGIN` role: `NOSUPERUSER`, `NOCREATEDB`, `NOCREATEROLE`, `NOINHERIT`, `NOREPLICATION`, `NOBYPASSRLS`.

- Its password lives only with the issuer process.
- The issuer process receives tenant, principal, and operation from the gateway after the gateway has verified the access token (`auth.go:43-66`). It does not accept those fields from shipment-service as proof.
- It checks operation, shipment, execution, and scope against that verified identity.
- For `CREATE_ACTIVE_LINK` and `CREATE_EXECUTION` it allocates the execution UUID before commit. A null execution id is not issued. This repeats the 0.1O-R4 rule and does not change it.
- It inserts one `ISSUED` ticket and commits. The ticket table has no foreign key to executions, shipments, or links.
- It does not `GRANT` its role to the writer and does not put its password in the writer configuration.

Revocation is a later transaction by the same issuer login. It locks only the ticket and sets `REVOKED` when the status is `ISSUED`.

## Function owner

`binding_authority_owner` is a future `NOLOGIN` non-superuser. It owns the ticket table, the audit table, and the fencing functions. It does not own `transport.transport_execution_active_shipments` or `transport.transport_executions`. `freight_migrations` is not reused. That role already owns the pin function (`000103:1150`).

Each function is `SECURITY DEFINER`, sets `search_path` to `pg_catalog`, uses schema-qualified names, and loses `PUBLIC` execute in the creating transaction before commit. The owner is not a login the writer can open.

## Writer

`binding_authority_caller` is a future `LOGIN` role with the same non-superuser attributes. It is the shipment-service pool user for consume.

- `EXECUTE` on the consume functions only.
- No `EXECUTE` on the issuer function.
- No membership in `binding_authority_issuer` or `binding_authority_owner`.
- `NOINHERIT`, so a mistaken membership still requires `SET ROLE`, and `SET ROLE` to a role it is not a member of fails.
- No table `INSERT` or `UPDATE` on the ticket table, the audit table, the active-link table, or `owner_epoch`.
- Consume, the business write, and the audit insert commit in one transaction. A failure rolls all of them back. The ticket then stays `ISSUED` until expiry or revocation.

`shipment_runtime` remains `NOLOGIN`. It is not this caller. Production today does not `SET ROLE` to it (`packages/shared-go/database/postgres.go:12-21`).

## Two options

### P1 — two logins inside shipment-service

Shipment-service holds two pools: an issuer pool and a writer pool.

| Question | Assessment |
| --- | --- |
| Forged tenant | The issuer function can ignore a writer SQL argument only if the writer cannot connect as the issuer. Two pools in one process fail that test once the process can read both passwords. |
| Bearer ticket | Unchanged. The writer pool can present any `ticket_id` it can read. |
| Atomic consume | Yes, on the writer pool, one transaction. |
| Gateway | The gateway still stops at shipment-service. The issuer decision stays inside the writer process. |
| Pools | A second pool, separate max connections, no shared `AfterConnect`. |
| Go | Pool split, call split, and a hard ban on using the issuer pool from write handlers. |
| Migrations | Later. Logins, grants, functions. No number is reserved here. |
| Rollback | Removing the second pool returns the service to one login. |
| Privileged bypass | The process that holds the issuer password is the issuer. A superuser or table owner still bypasses grants. |

P1 is not the selected boundary. A process that can open the issuer login can issue a ticket for any tenant it chooses.

### P2 — issuer process, writer login

A separate issuer process is the only holder of the issuer login. The gateway calls it after JWT verification, over the existing internal-auth pattern (`integration_verify_handler.go:42-44` is the shape: the callee authorizes the caller before it trusts the body). Shipment-service keeps one writer pool and only consumes.

| Question | Assessment |
| --- | --- |
| Forged tenant | The writer has no issuer login, no issuer `EXECUTE`, and no membership. A tenant in its SQL is not consulted. The issuer takes tenant from the gateway call, and that call is authenticated as the gateway. |
| Bearer ticket | Still open until presenter binding in the companion document. Role split alone does not name the presenter. |
| Atomic consume | Yes. The writer transaction locks the committed ticket, writes, audits, and sets `CONSUMED` in one commit. |
| Gateway | Fits the current gateway. Auth stays at the gateway (`auth.go:29-66`). The gateway gains one issuer call. Shipment-service does not become the identity source. |
| Pools | Issuer pool and writer pool are in different processes. They do not share connections. Writer pool size stays the current default of 25 until measured (`pool_config.go:20`). |
| Go | New issuer process, a gateway client, and writer calls that use consume functions instead of direct binding DML. The role and function change is smaller than moving every writer. |
| Migrations | Later, and only after the issuer login exists. Do not edit `000103`, `000104`, or `000105`. Do not allocate the next number in this document. |
| Rollback | Stop routing new issues to the issuer and leave issued tickets to expire. Restoring the `freight` DSN returns the open login. It is not a safe default. |
| Privileged bypass | Superuser, table owner, and any session that still connects as `freight` while `freight` owns the tables or is superuser. `session_replication_role = replica` can skip a normal trigger. The trigger is not this boundary. |

P2 is the proposal. It is not installed. `DB_ROLE_SEPARATION_FEASIBLE=DESIGN_ONLY`.

## What P2 does not claim

- The designed issuer is not a boundary that already runs.
- Sharing the `freight` password with a second process and naming that process "issuer" does not split the principal.
- `ticket_id` does not prove the presenter. See the presenter document.
- Pin propagation stays `NOT_PROVEN`.
- Direct DML by the current login stays open. `J-DML-001` stays open.
