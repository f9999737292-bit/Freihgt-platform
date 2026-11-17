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

Rotation is `PROPOSED`. A new gateway key id overlaps the previous id for one window. After that window the issuer rejects the old id. A revoked key fails closed. Ending the overlap does not delete a tombstone. A new key does not create a second ticket for a `request_id` that already has one. Retired key ids stay recorded.

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

- `request_id`, one ticket inside the issuer namespace
- the gateway key id
- the access token
- operation, tenant claim, shipment, and execution scope
- `signed_at` and a short signature expiry

The issuer checks the signature against its own clock. `signed_at` in the future by more than the clock-skew allowance fails. `signed_at` older than the signature lifetime plus that allowance fails, including when `request_id` has not been stored yet. The caller's timestamp is not enough. The access-token `exp` is a separate check. `ticket.expires_at` is the lifetime of an issued authority. It is not the signature lifetime and it is not how long the idempotency key is kept.

`issuer_namespace_id` is the permanent name of one issuance history. It is read from the issuer's trusted configuration and checked against the tombstones and the external witness. A namespace in the request is not that value. A mismatch closes `ISSUER_ISSUANCE_GATE` and is `ISSUE_DENIED_AUTH`. `gateway_key_id` identifies the signing key. It is not a second permission namespace, and rotating it does not change `issuer_namespace_id`. Restarting the issuer does not change it. Replacing the database credentials does not change it. Restoring a database does not allocate a new one.

A configuration value that differs from the namespace on a tombstone or in the witness leaves the gate CLOSED. Creating another namespace is a separate administrative workflow, not an edit of this value. That workflow is `PROPOSED_NOT_IMPLEMENTED`. It starts only when a named administrator is authorized by a check that is not the issuance request. Issuance on the old namespace is closed first. The witness for that namespace is read, and if it cannot be read the transition stops and the old gate stays CLOSED. Old tombstones and old witness records stay. The transition is appended to the witness and does not lower its checkpoint. The new namespace opens only by a later, separate decision. Rollback returns to the old namespace with the same tombstones and the same witness. It does not restore a database that omits tombstones, and it does not delete witness records. A new namespace that cannot show its `request_id` values are disjoint from the old history does not treat an old `request_id` as a new authority. Changing the namespace identifier does not by itself do that.

The idempotency key is `(issuer_namespace_id, request_id)`. PostgreSQL enforces it with a unique constraint. Go checking the key before the insert does not. Both columns are `NOT NULL`. A `NULL` is not a key. A missing `issuer_namespace_id`, `gateway_key_id`, or `request_id` is `ISSUE_DENIED_AUTH` and writes nothing. One `request_id` in that namespace has one ticket, including after the signing key changes. An earlier draft used `(gateway_key_id, request_id)`. No such rows exist. A future migration, with its number chosen at implementation time and not reserved here, checks any stored rows first. It copies `issuer_namespace_id` only when the issuer configuration and the witness name the same value, and it keeps the original `gateway_key_id`, `ticket_id`, and `payload_hash`. A different configuration value does not open issuance. It creates and validates the new unique constraint before it drops the old one. If one `request_id` already belongs to two keys or two tickets, that proof fails, the old constraint stays, and issuance stays closed.

The committed tombstone stores `issuer_namespace_id`, `request_id`, the `gateway_key_id` of the first signature, `ticket_id`, `payload_hash`, status, and `expires_at`. `ticket_id` and `payload_hash` are `NOT NULL`. The ticket insert and the tombstone insert are one transaction. A deferred check at commit rejects a ticket that has no tombstone. A failure rolls both back.

The canonical payload is UTF-8 text, one field per line, in this order: `issuer_namespace_id`, `request_id`, operation, principal from the validated `sub`, tenant from the validated `tenant_id` claim, shipment id, execution ids in ascending UUID order, expected epoch or an empty field, expected revision or an empty field. Signature bytes, `signed_at`, `gateway_key_id`, and the raw access token are not in that text. `payload_hash` is SHA-256 of that text. This encoding is `PROPOSED_NOT_IMPLEMENTED`.

`(issuer_namespace_id, request_id)` is single-use for the life of the issuer. A new signature of the same `request_id`, with a new `signed_at` or a new `gateway_key_id`, does not receive a second ticket. The old signature's lifetime does not make the key reusable. Retired key ids stay recorded after the rotation overlap ends. A signature from a retired key after that overlap is `ISSUE_DENIED_AUTH`. The tombstone for the request stays.

Two retention shapes were compared. A bounded row, deleted when the signature window ends, is not enough. After the delete, the issuer cannot tell a new `request_id` from one it has already used, and a new signature of the old `request_id` would pass the signature check and insert a second ticket. `ticket.expires_at` is a shorter clock than that window whenever the ticket expires first, so it is not the retention bound either.

The selected record is a tombstone. After `REVOKED`, `EXPIRED`, or `CONSUMED`, the unique key, the original `gateway_key_id`, `ticket_id`, `payload_hash`, status, and `expires_at` stay. Status changes do not delete them and do not set a terminal status back to `ISSUED`. Cleanup may drop the access token after the signature window. It does not delete or null `payload_hash`. A missing or unreadable digest is `ISSUE_DENIED_AUTH` and the response has no ticket id. A different canonical hash is `ISSUE_DENIED_PAYLOAD_MISMATCH`. A full delete is not part of this contract. These columns and this cleanup are `PROPOSED_NOT_IMPLEMENTED`.

While a request with that `request_id` can still pass the signature, token, and operation checks, the row that blocks a second ticket stays. That includes a ticket that is already expired and a signature that is still inside its window. The stored outcomes are:

| Stored state | Result |
| --- | --- |
| Ticket expired, tombstone present | `ISSUE_DENIED_EXPIRED`. No second ticket |
| Ticket `REVOKED` or `CONSUMED`, tombstone present | `ISSUE_DENIED_REVOKED` or `ISSUE_DENIED_CONSUMED` |
| Ticket expired and the row removed while the old signature is still acceptable | Forbidden. The row is not removed. Without it the issuer cannot refuse and avoid a second insert |
| Same `request_id`, including a new signing key, and the same canonical hash, ticket still `ISSUED` and unexpired | `ISSUE_REPLAY_ISSUED`. The original ticket. No second ticket |
| Same `request_id` and a different canonical hash | `ISSUE_DENIED_PAYLOAD_MISMATCH` |
| Digest missing or unreadable | `ISSUE_DENIED_AUTH`. No ticket id |

Cleanup locks the idempotency row `FOR UPDATE` and then the ticket, the same order as a retry. It does not lock the ticket first. Revocation locks only the ticket, so cleanup waits for that lock and then leaves the tombstone, including `payload_hash`. A cleanup that deleted the key or the digest would hide a prior request. This cleanup does not. The unique index still conflicts for that `request_id` under a later signing key. A concurrent first insert of a new `request_id` is unchanged: the unique constraint admits one commit. A retry after a lost HTTP response reads the tombstone in a new transaction and does not insert another ticket. Key rotation does not delete these rows. Cleanup does not delete or rewrite a witness record. A witness cleanup that would remove proof of an earlier issuance is forbidden.

The PostgreSQL tombstones are not proof that this database is the latest history. A restored copy can agree with itself and still omit committed tickets. `SELECT COUNT(*)` and the existence of the table are not that proof. The proposed check is an append-only witness outside the PostgreSQL data directory. Restoring the database does not restore the witness and does not rewind it. No storage product is selected. The document-service object store, whether local or S3, stores documents. It is not this witness, and it is not an authority boundary for issuance. The witness is `PROPOSED_NOT_IMPLEMENTED`. It is not installed in BINTRANS.

Each witness record carries a sequence assigned by the witness, the previous sequence, `issuer_namespace_id`, `request_id`, the reservation state, the fencing epoch that created the reservation, `payload_hash`, and, once a ticket exists, `ticket_id` and the original `gateway_key_id`. The sequence has no gaps. One `(issuer_namespace_id, request_id)` has one reservation, whichever signing key is presented. The issuer cannot submit a lower checkpoint or a lower epoch. The witness rejects a delete or a rewrite. Dropping an access token does not remove the witness record. No storage product is selected for these properties. The document-service object store does not provide them.

The future issuer appends a reservation or an outcome, with a credential that is not the `freight` login, the gateway signing key, or the writer login. That credential does not exist. The issuer and a separate recovery operator may read. The operator cannot append a reservation or an outcome. The operator can request the epoch increment. The writer and the gateway cannot append. The operator records the highest witness sequence already accepted. That record sits outside the database and outside the witness. The issuer cannot write it, and the procedure has no step that lowers it. A witness sequence below that record is a rewind. The gate stays `CLOSED`, and history is not complete. If that record is missing while either store has issuance records, history is not complete. The record is `PROPOSED_NOT_IMPLEMENTED` and is not a selected storage product.

The witness reservation and the PostgreSQL commit are two commits. They are not one atomic operation. This design has no distributed transaction. The reservation is appended first. PostgreSQL is started only after that append is durable. The outcome is appended after PostgreSQL ends. A local flag in the issuer is not any of these steps.

The witness keeps one monotonic fencing epoch for each `issuer_namespace_id`. A new reservation is accepted only from the current epoch, and only from the one holder of that epoch. The recovery operator, not the old issuer, asks the witness to increment the epoch. The witness then refuses new reservations from the old epoch. That increment does not abort a PostgreSQL transaction the old issuer has already started. A lease timeout is not evidence that the old transaction will fail to `COMMIT`. The old epoch may still append the outcome of a reservation it already holds, so a commit is not forgotten. It cannot open a second reservation.

The PostgreSQL check that would make the old `COMMIT` fail is not installed. The required shape is one epoch row per namespace, locked `FOR UPDATE` for the whole issuance transaction, with the transaction's epoch equal to that locked row. A boolean in the issuer process is not that lock. `freight` can still insert without it. Whether that login is superuser is `NOT_PROVEN`. The table owner and a superuser can disable a trigger, and `session_replication_role = replica` can skip one. `OLD_PRIMARY_FENCING=NOT_PROVEN`. Until a check inside PostgreSQL rejects that `COMMIT`, promotion does not open `ISSUER_ISSUANCE_GATE`.

A reservation is one of `RESERVED`, `COMMIT_CONFIRMED`, `ABORT_CONFIRMED`, `IN_DOUBT`, or `PERMANENTLY_DENIED`. `RESERVED` follows a durable unique append. `COMMIT_CONFIRMED` follows an outcome that cites that same reservation and a committed ticket. `ABORT_CONFIRMED` follows proof that the PostgreSQL transaction rolled back and that no commit of that reservation exists. A timeout is not that proof, and a missing HTTP response is not that proof. `IN_DOUBT` follows a crash or an unknown outcome. `PERMANENTLY_DENIED` follows an unknown fate that cannot be proven. `RESERVED` and `IN_DOUBT` are never deleted and never returned to an empty key. `COMMIT_CONFIRMED` and `PERMANENTLY_DENIED` do not change. `IN_DOUBT` becomes `COMMIT_CONFIRMED` only when the matching tombstone is found. The same reservation, after a proven abort, may be resumed into the one PostgreSQL insert. That resume is not a new reservation.

| Window | What can be checked | What enforces the result |
| --- | --- | --- |
| Reservation durable, PostgreSQL not begun | One `RESERVED` record for that namespace and `request_id` | The witness unique append. A second reserve is rejected |
| Reservation durable, PostgreSQL insert not started | The same `RESERVED` record, and no tombstone | Resume that reservation, or leave it. A timeout does not free it and does not create a ticket |
| PostgreSQL `COMMIT` done, outcome not appended | The tombstone and the still-open reservation | The reservation blocks another reserve. The HTTP response grants nothing until the outcome is `COMMIT_CONFIRMED` |
| PostgreSQL `ROLLBACK` done, witness still `RESERVED` | The reservation, and no commit record for it | No automatic new ticket. `ABORT_CONFIRMED` only with proof of the rollback. Otherwise the state becomes `IN_DOUBT` |
| HTTP response lost | The reservation state | No new reservation. A ticket id is returned only for `COMMIT_CONFIRMED`, and only under the replay rules below |
| PostgreSQL restored from a stale backup | Witness reservations that the backup does not contain | Those reservations stay. A missing tombstone is not a free `request_id`. The gate stays `CLOSED` |
| Witness unavailable | No readable reservation log | The gate stays `CLOSED`. No `ISSUE_OK` |
| Witness and PostgreSQL disagree | Reservation state against the tombstone | The gate stays `CLOSED`. An unknown commit stays `IN_DOUBT` or becomes `PERMANENTLY_DENIED`. No second ticket |
| Reservation belongs to an older epoch | The epoch stored on that reservation | The old epoch cannot reserve again. It may append the outcome of the reservation it holds. Its later PostgreSQL `COMMIT` is not proven to fail |

History is not complete while any reservation is `RESERVED` or `IN_DOUBT`, while the witness cannot be read, or while its sequence is below the accepted mark. A database that matches itself is not that answer. Reconciliation copies a witness reservation onto the existing tombstone, or stores a denial tombstone that is not `ISSUED`. It does not insert a second ticket. The recovery operator records the comparison and cannot lower the accepted sequence and cannot append a reservation. A person's approval does not replace that comparison. If the proof is missing, the gate stays `CLOSED`.

Restore and failover stay `PROPOSED_NOT_IMPLEMENTED`.

1. Close issuance before recovery starts. Closing it on the new process does not, by itself, stop the old process.
2. Compare the restored rows with the witness that this backup did not replace.
3. Check one namespace, one current epoch, a contiguous reservation sequence, and a terminal witness state for every `request_id` the witness holds.
4. A reservation the database lacks is not issued again. Its key stays reserved, denied, or matched to a restored tombstone.
5. Do not treat the old primary as fenced. `OLD_PRIMARY_FENCING=NOT_PROVEN`. The new epoch does not open the gate.
6. Two timelines do not both receive `COMMIT_CONFIRMED` for one `request_id`. The witness has one reservation. They can still both write through `freight`. That bypass is open.
7. The gate stays `CLOSED` until the reservation history and a PostgreSQL epoch check are both proven. Neither is proven. The issuer does not open the gate because a person approved it or because the table is present.

Promoting a lagging replica closes the gate. Split-brain closes both sides for new reservations. A lost, rewound, or unreachable witness leaves the gate `CLOSED`. A configuration change after the restore does not open it.

`SELECT ... FOR UPDATE` does not lock a row that is not there. The witness reservation is what stops a second first issuance, including after the database rows are gone. The PostgreSQL unique constraint still stops a second insert when the row is present. It does not survive a restore that omits the row.

The issuer still checks the gateway signature, the access token, the operation scope, the namespace, the open gate, and the current epoch before it reserves. A new `request_id` passes those checks again. It is not a shortcut. An expired signature fails closed and does not return an older ticket. The lost-response retry does not create a reservation.

Transaction A, the first issuance:

1. Check the gateway signature, the access token, the issuer operation scope, and `issuer_namespace_id`.
2. Check that the issuance gate is open and that this process holds the current fencing epoch. A closed gate stops the transaction here.
3. Append one reservation for `(issuer_namespace_id, request_id)` to the witness.
4. Continue only after that append is durable and the state is `RESERVED`.
5. Begin the PostgreSQL transaction.
6. Insert the ticket and the idempotency tombstone together.
7. In that same transaction, lock the epoch row and require it to equal the reservation epoch. A local flag is not this step. This lock is `PROPOSED_NOT_IMPLEMENTED` and is not a proven fence.
8. `COMMIT` the ticket and the tombstone.
9. Append the outcome. `COMMIT_CONFIRMED` cites that reservation and that `ticket_id`.
10. Send the HTTP response only after the outcome is `COMMIT_CONFIRMED`. If the outcome is not confirmed, the response has no ticket id and is not a new grant.

Transaction B uses the same `issuer_namespace_id` and `request_id`, including a different `gateway_key_id`. It tries to reserve the same key.

1. The witness accepts one reservation. The other caller does not begin PostgreSQL.
2. If A reaches `COMMIT_CONFIRMED`, B locks the tombstone `FOR UPDATE`, then the ticket, and uses the result codes below. It does not insert a second ticket.
3. If A's PostgreSQL transaction is aborted by `unique_violation`, that transaction is rolled back. The follow-up reads the committed row in a new transaction.
4. If A rolls back and the reservation remains `RESERVED` or becomes `IN_DOUBT`, B does not create a ticket. A proven `ABORT_CONFIRMED` may resume the same reservation. It does not allocate another one.

`INSERT ... ON CONFLICT` is not this path. A conflict action that succeeded before reading the ticket status could return a ticket that is `REVOKED`, `EXPIRED`, or `CONSUMED`.

A committed idempotency row is read only in a transaction that did not just receive `unique_violation`. Under that lock the issuer compares the payload hash, then the ticket:

The result codes stay the ones already fixed. They apply in the new transaction after the unique conflict, and on a later retry. A lost response uses the same path. It does not mint a replacement ticket.

The application result codes do not add a second SQLSTATE. The conflicting insert is PostgreSQL `unique_violation`, and that error aborts the transaction. The function is not written. A future run records the SQLSTATE it actually raises.

| Condition under the lock | Result code | Ticket id in the response |
| --- | --- | --- |
| Durable `RESERVED`, current epoch, gate open, no tombstone, checks passed, outcome then `COMMIT_CONFIRMED` | `ISSUE_OK` | The new id, only after that outcome |
| No tombstone and no durable reservation | No issue | Absent |
| `RESERVED` or `IN_DOUBT`, outcome unknown | No new ticket | Absent |
| Stale fencing epoch or closed gate | `ISSUE_DENIED_AUTH` | Absent |
| Same hash, witness `COMMIT_CONFIRMED`, `ISSUED`, `expires_at` still later than `clock_timestamp()` | `ISSUE_REPLAY_ISSUED` | The same id |
| Same hash, `REVOKED` | `ISSUE_DENIED_REVOKED` | Absent |
| Same hash, status `EXPIRED`, or `ISSUED` with `expires_at` not later than `clock_timestamp()` | `ISSUE_DENIED_EXPIRED` | Absent |
| Same hash, `CONSUMED` | `ISSUE_DENIED_CONSUMED` | Absent. This is not a new grant |
| Same hash, any other status | `ISSUE_DENIED_UNKNOWN_STATUS` | Absent |
| Same `request_id`, different payload hash | `ISSUE_DENIED_PAYLOAD_MISMATCH` | Absent |
| Digest missing or unreadable | `ISSUE_DENIED_AUTH` | Absent |
| Signature, token, key, or role check failed | `ISSUE_DENIED_AUTH` | Absent |

A denial does not include another principal's ticket id, status, tenant, or shipment. `ISSUE_DENIED_AUTH` is the code for a failed check and for a request the issuer will not describe further.

Revocation locks only the ticket `FOR UPDATE`. Consume locks the ticket `FOR UPDATE` before the execution, shipment, and link locks. After a committed idempotency row exists, a retry locks that row `FOR UPDATE` and then the ticket. Revocation and consume do not lock the idempotency row, so they do not take the opposite order. Two retries of a committed row take the idempotency row, then the ticket.

If revocation commits first, the retry's ticket lock sees `REVOKED` and returns `ISSUE_DENIED_REVOKED`. If the retry commits `ISSUE_REPLAY_ISSUED` first, revocation waits, then sets `REVOKED`. The HTTP grant is written only after the witness outcome is `COMMIT_CONFIRMED`. That grant is still not atomic with a later revocation. A ticket returned by that commit can be revoked before the caller uses it. Consume of a `REVOKED` ticket still fails.

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
| Issuer process compromised | It can insert tickets for identities it can validate, and it can append witness records until its witness credential is revoked. It cannot lower the accepted sequence. A witness record with no ticket forbids that `request_id` and closes the gate. It does not authorize a second ticket. Recovery revokes the issuer login, the witness credential, and the gateway key. Tickets already committed stay until expiry or revocation |
| Issue commits, response is lost | The same request is retried. It returns the same id only when the witness is `COMMIT_CONFIRMED` and the locked ticket is still `ISSUED` and unexpired. It does not create a reservation |
| Consume rolls back | The ticket stays `ISSUED`. The write and the audit row are gone |
| Two consumes | The ticket lock admits one commit. The other sees `CONSUMED` or waits and then fails |

## What P2 does not claim

- The gateway signature, the issuer revalidation, and the request-id store are `PROPOSED`. They are not running.
- A stored ticket is returned only for `ISSUE_REPLAY_ISSUED`. `REVOKED`, `EXPIRED`, `CONSUMED`, an elapsed `expires_at`, and an unknown status are denials.
- A missing idempotency row is not first issuance. The witness reservation is. The unique constraint on `(issuer_namespace_id, request_id)` remains for a database that still has the row. `gateway_key_id` is not that namespace.
- That unique key, the original signing key, and `payload_hash` are a tombstone. They are not deleted at `ticket.expires_at`, at the end of the signature window, or when the signing key rotates. One `request_id` is not a second ticket.
- Issuer restart, failover, and restore handling are `PROPOSED_NOT_IMPLEMENTED`. A stale restore closes issuance. The external witness is not installed. The document object store is not that witness. Two commits are not one atomic operation. A reservation is not released on a timeout.
- `OLD_PRIMARY_FENCING=NOT_PROVEN`. A lease timeout is not proof that an old transaction will fail its `COMMIT`. A person's approval does not open the gate. Direct SQL as `freight` bypasses an uninstalled epoch lock.
- `issuer_namespace_id` is not a switch. A new identifier does not make a used `request_id` a new authority.
- `X-Internal-Service-Token` is not gateway authentication.
- Sharing the `freight` password with a second process and naming that process "issuer" does not split the principal. Compose still uses that login.
- `iss`, `aud`, token revocation, and the issuer's operation check are `NOT_PROVEN`.
- `ticket_id` does not prove the presenter. A one-time bearer ticket is not a cryptographic binding to a consume session.
- Pin propagation stays `NOT_PROVEN`.
- Direct DML by the current login stays open. `J-DML-001` stays open.
