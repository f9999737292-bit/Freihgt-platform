# Trusted issuer contract — 0.1P

Status: `PROPOSED_NOT_APPROVED`. This does not approve B3, does not close `J-DML-001`, and does not install a role. Properties marked `PROPOSED` are not implemented.

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

- Its password lives only with the issuer process. Shipment-service does not have it. `PROPOSED`.
- Principal and tenant come from the access token the issuer validates itself. A tenant or principal in the request JSON is not that source. The shared `X-Internal-Service-Token` is not the caller proof. See the authentication section.
- Operation, shipment, and execution are taken from the signed gateway request and checked against the principal. The operation check is `NOT_PROVEN` until the issuer can read the same role decision the gateway uses. A JSON operation field alone is not that decision.
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

`PROPOSED_NOT_APPROVED`. A separate issuer process is the only holder of the issuer database login. Shipment-service keeps one writer pool and only consumes. The call from the gateway is not the existing internal-token check.

`packages/shared-go/internalauth/auth.go:16-25` accepts any caller that presents `X-Internal-Service-Token`. Gateway, shipment-service, and the other services that read `INTERNAL_SERVICE_TOKEN` can present it. `integration_verify_handler.go:42-44` is that same check before an API-key verification. It does not identify the gateway, and it does not validate a human access token. Ticket issuance rejects a request whose only caller proof is this shared token.

#### P2-A — issuer validates the access token

The issuer repeats the checks the gateway already performs, on the token bytes, not on a JSON copy of the claims.

| Check | Today | Issuance rule |
| --- | --- | --- |
| Signature and `HS256` only | Implemented. `auth.go:90-91`, identity `jwt.go:48` and `:59-60` | Required. Another algorithm fails. |
| Expiry | `exp` is set (`jwt.go:43`) and `token.Valid` rejects an expired token (`auth.go:100`) | Required. The issuer clock decides. |
| Principal | `sub` (`jwt.go:42`) | Required. This is the only principal written onto the ticket. |
| Tenant | `tenant_id` (`jwt.go:38`) | Required. A different tenant in the body fails. |
| `iss` and `aud` | Not set in `jwt.go:41-45` | `PROPOSED`. `NOT_PROVEN` until tokens carry them and the issuer rejects a mismatch. |
| Revocation | No `jti` and no denylist on this path | `NOT_PROVEN`. A token that still verifies is accepted until `exp`. |
| Operation scope | Not a claim. Gateway asks identity for roles (`routeauth/guard.go:44-56`) | `NOT_PROVEN` at the issuer. The issuer must perform that role check with the verified token or refuse. |

P2-A does not prove the HTTP client is the gateway. The user holds the access token, and the gateway forwards `Authorization` (`routeauth/client.go:71-72`). Shipment-service can present that token.

#### P2-B — the caller is the gateway

`PROPOSED`. The issuer accepts an issuance request only when a gateway-only key signs it. Shipment-service and the other internal services do not hold that key. mTLS with a client certificate issued only to the gateway is an acceptable transport binding of the same fact. The signed body is still required, because a captured body can be sent again on a later connection.

The signature covers the access token, the operation, the shipment, the execution scope, `request_id`, and the gateway key id. The issuer verifies the signature against the pinned gateway key, checks the key id is still allowed, and rejects every other key. A request signed by another service fails. A request that presents `X-Internal-Service-Token` and no gateway signature fails.

The issuer has no route on the public gateway. It is reachable only from the gateway on the internal network. That absence of a route is `PROPOSED`; no issuer process exists to publish one today.

Rotation is `PROPOSED`. A new gateway key id overlaps the previous id for one window. After that window the issuer rejects the old id. A revoked key fails closed.

#### Selected combination

P2 uses both. P2-B proves the caller is the gateway. P2-A proves the principal and the tenant. Neither one is deployed. The shared internal token is not a substitute for P2-B.

| Question | Assessment |
| --- | --- |
| Forged tenant | The ticket tenant is the `tenant_id` claim from the token the issuer validated. A body tenant, a header, or a SQL argument that differs is denied. The writer login cannot insert the ticket. |
| Bearer ticket | Open. Gateway authentication does not identify the later consume session. See the presenter document. |
| Atomic consume | The writer transaction locks the committed ticket, writes, audits, and sets `CONSUMED` in one commit. |
| Gateway | The gateway still validates the user token (`auth.go:29-66`). It then signs the issuance request with its own key. It does not send `X-Internal-Service-Token` as that proof. |
| Pools | Issuer pool and writer pool are in different processes. Writer pool size stays the current default of 25 until measured (`pool_config.go:20`). |
| Go | New issuer process, gateway signer, and writer consume calls. No number is reserved for the migration. |
| Migrations | Later, after the issuer login exists. Do not edit `000103`, `000104`, or `000105`. |
| Rollback | Stop signing new requests. Issued tickets expire or are revoked. Restoring the `freight` DSN returns the open login. |
| Privileged bypass | Superuser, table owner, and a session that still connects as `freight` while that login owns the tables or is superuser. The shared login is still the compose pool. Isolation is not in force. |

`DB_ROLE_SEPARATION_FEASIBLE=DESIGN_ONLY`. `P2_DB_ROLE_SEPARATION=DESIGNED_NOT_IMPLEMENTED`. `SHARED_FREIGHT_LOGIN=OPEN`.

## Issuance request and replay

`PROPOSED`. The issuer does not trust a timestamp supplied only by the caller.

The signed payload contains:

- `request_id`, unique for one authorization
- the gateway key id
- the access token
- operation, tenant claim, shipment, and execution scope
- `signed_at` and a short signature expiry

The issuer checks the signature expiry against its own clock. A stale signature fails even when `request_id` is new. The access-token `exp` is a second, independent check.

The idempotency key is `(gateway_key_id, request_id)`. PostgreSQL enforces it with a unique constraint. Go checking the key before the insert does not. Both columns are `NOT NULL`. A `NULL` is not a key, and it must not be stored. A missing `gateway_key_id` or `request_id` is `ISSUE_DENIED_AUTH` and writes nothing.

The idempotency row stores `ticket_id`, which is `NOT NULL` and references that ticket. The ticket insert and the idempotency insert are one transaction. A deferred check at commit rejects a ticket that has no idempotency row. A failure rolls both back. No migration number is reserved for this table.

The row stays after `REVOKED`, `EXPIRED`, and `CONSUMED`. Those status changes do not delete it. It remains at least until the ticket `expires_at`. A purge before that time is not part of this contract. While the row exists, the unique key still blocks a second ticket for that pair.

`SELECT ... FOR UPDATE` does not lock a row that is not there. Two first issuances of the same pair can both see no row. The unique constraint is what stops the second commit.

The issuer still checks the gateway signature, the access token, and the operation scope before it writes. A new `request_id` passes those checks again. It is not a shortcut. An expired signature fails closed and does not return an older ticket. The lost-response retry does not insert another ticket.

Transaction A, the first commit:

1. Check the gateway signature, the access token, and the operation scope.
2. Begin.
3. Insert the ticket and the idempotency row for that `ticket_id`.
4. Commit both.
5. Send the HTTP response after that commit. The response is not part of the commit.

Transaction B, the same `gateway_key_id` and `request_id`, runs its own checks and then inserts the same unique key.

1. If A commits, B's insert conflicts. PostgreSQL raises `unique_violation` and aborts B. B does not run another statement in that transaction.
2. B rolls back.
3. B begins a new transaction, locks the committed idempotency row `FOR UPDATE`, then locks that ticket `FOR UPDATE`.
4. B compares the payload hash and reads `status` and `expires_at` under those locks. It uses the result codes below. It does not insert a second ticket.
5. If A rolled back, the new transaction finds no row. B may insert the one ticket and its idempotency row only after repeating the signature, token, and operation checks. The aborted transaction is not reused.

`INSERT ... ON CONFLICT` is not this path. A conflict action that succeeded before reading the ticket status could return a ticket that is `REVOKED`, `EXPIRED`, or `CONSUMED`.

A committed idempotency row is read only in a transaction that did not just receive `unique_violation`. Under that lock the issuer compares the payload hash, then the ticket:

The result codes stay the ones already fixed. They apply in the new transaction after the unique conflict, and on a later retry. A lost response uses the same path. It does not mint a replacement ticket.

The application result codes do not add a second SQLSTATE. The conflicting insert is PostgreSQL `unique_violation`, and that error aborts the transaction. The function is not written. A future run records the SQLSTATE it actually raises.

| Condition under the lock | Result code | Ticket id in the response |
| --- | --- | --- |
| No stored row, checks passed | `ISSUE_OK` | The new id |
| Same hash, `ISSUED`, `expires_at` still later than `clock_timestamp()` | `ISSUE_REPLAY_ISSUED` | The same id |
| Same hash, `REVOKED` | `ISSUE_DENIED_REVOKED` | Absent |
| Same hash, status `EXPIRED`, or `ISSUED` with `expires_at` not later than `clock_timestamp()` | `ISSUE_DENIED_EXPIRED` | Absent |
| Same hash, `CONSUMED` | `ISSUE_DENIED_CONSUMED` | Absent. This is not a new grant |
| Same hash, any other status | `ISSUE_DENIED_UNKNOWN_STATUS` | Absent |
| Same `request_id`, different payload hash | `ISSUE_DENIED_PAYLOAD_MISMATCH` | Absent |
| Signature, token, key, or role check failed | `ISSUE_DENIED_AUTH` | Absent |

A denial does not include another principal's ticket id, status, tenant, or shipment. `ISSUE_DENIED_AUTH` is the code for a failed check and for a request the issuer will not describe further.

Revocation locks only the ticket `FOR UPDATE`. Consume locks the ticket `FOR UPDATE` before the execution, shipment, and link locks. After a committed idempotency row exists, a retry locks that row `FOR UPDATE` and then the ticket. Revocation and consume do not lock the idempotency row, so they do not take the opposite order. Two retries of a committed row take the idempotency row, then the ticket.

If revocation commits first, the retry's ticket lock sees `REVOKED` and returns `ISSUE_DENIED_REVOKED`. If the retry commits `ISSUE_REPLAY_ISSUED` first, revocation waits, then sets `REVOKED`. The HTTP response is written after the database commit. Those two steps are not one atomic operation. A ticket returned by that commit can be revoked before the caller uses it. Consume of a `REVOKED` ticket still fails.

Two first inserts do not lock a missing row. The unique index admits one commit. The other transaction is aborted by `unique_violation` and is rolled back. Its follow-up transaction then uses the status rules above. A failed first insert leaves neither the ticket nor the idempotency row.

## Who decides

| Step | Who | Evidence |
| --- | --- | --- |
| User token | Gateway, then the issuer again | `auth.go:43-66`. Issuer revalidation is `PROPOSED` |
| Tenant membership | The `tenant_id` claim in that token | `jwt.go:38`. The issuer ignores a substituted body tenant |
| Operation | Gateway role policy today | `routeauth/guard.go:44-56`. Issuer operation check is `NOT_PROVEN` |
| Stored result | The committed ticket row | `PROPOSED`. It stores the validated principal, tenant, operation, and scope |
| Trusted claims versus JSON | Claims are read from the validated token. JSON that disagrees fails | `PROPOSED` |
| Later consume | The ticket fields, not a new tenant argument | Presenter binding stays `NOT_PROVEN` |

## Failure

Each row fails closed when the authority is missing.

| Event | Result |
| --- | --- |
| Issuer unavailable | The gateway returns failure. No ticket and no binding write |
| Gateway unavailable | Nothing is signed. The issuer inserts nothing |
| Expired or revoked gateway key | Denied |
| Replay of a different scope | Denied |
| Forged tenant in the body | Denied. The token claim is the tenant |
| Writer process compromised | It can use its own login. It cannot sign an issuance, cannot `SET ROLE` to the issuer, and cannot `EXECUTE` the issuer. A stolen `ticket_id` is still the presenter gap |
| Issuer process compromised | It can insert tickets for identities it can validate. Recovery is to revoke its login and the gateway key. Tickets already committed stay until expiry or revocation |
| Issue commits, response is lost | The same signed request runs the transaction again. It returns the same id only for `ISSUE_REPLAY_ISSUED`. It does not insert a second ticket |
| Consume rolls back | The ticket stays `ISSUED`. The write and the audit row are gone |
| Two consumes | The ticket lock admits one commit. The other sees `CONSUMED` or waits and then fails |

## What P2 does not claim

- The gateway signature, the issuer revalidation, and the request-id store are `PROPOSED`. They are not running.
- A stored ticket is returned only for `ISSUE_REPLAY_ISSUED`. `REVOKED`, `EXPIRED`, `CONSUMED`, an elapsed `expires_at`, and an unknown status are denials.
- A lock of an absent idempotency row is not the first-issuance barrier. The unique constraint on `(gateway_key_id, request_id)` is.
- `X-Internal-Service-Token` is not gateway authentication.
- Sharing the `freight` password with a second process and naming that process "issuer" does not split the principal. Compose still uses that login.
- `iss`, `aud`, token revocation, and the issuer's operation check are `NOT_PROVEN`.
- `ticket_id` does not prove the presenter. A one-time bearer ticket is not a cryptographic binding to a consume session.
- Pin propagation stays `NOT_PROVEN`.
- Direct DML by the current login stays open. `J-DML-001` stays open.
