# Presenter binding — 0.1P

Status: discovery. No test in this document was run.

`ticket_id` is a random primary key. Knowing it is not a presenter identity. `TICKET_ID_IS_IDENTITY=NO`.

## What must be true together

A consume is accepted only when every row below matches the locked ticket.

| Property | Rule |
| --- | --- |
| Presenter | The session is a principal the issuer recorded, proved by a check the writer cannot forge. |
| Tenant | The tenant on the ticket, copied from the gateway token by the issuer. |
| Operation | The function's operation equals the ticket operation. |
| Shipment | The locked shipment equals `shipment_id`. |
| Execution | Create uses the preassigned UUID. Retarget uses the ticket's source and destination. Any other UUID fails. |
| One use | Status moves from `ISSUED` to `CONSUMED` in the same commit as the write. A later transaction sees `CONSUMED` and fails. |
| Expiry | `clock_timestamp()` at or after `expires_at` fails while the status is still `ISSUED`. |
| Revoke | The issuer sets `REVOKED` under a ticket-only lock. The writer then fails. |
| Other connection | A second pool connection that only knows `ticket_id` fails the presenter check. |

Rollback of the consume transaction returns the ticket to `ISSUED`. That is not a second successful consume.

## Current sessions

The shipment pool, the tracking pool, and the other compose services authenticate as the same login. `current_user` does not change per request. `packages/shared-go/database/postgres.go:12-21` never sets a role from the gateway user id.

A second connection from that login can present any `ticket_id` the first connection wrote. PostgreSQL has no column that distinguishes those two connections as different people.

`PRESENTER_IDENTITY_BOUND=NOT_PROVEN`.

`binding_authority_caller`, once it exists, is still one role. Every shipment-service connection would share it. Role split stops the writer from issuing tickets. It does not, by itself, stop one writer connection from consuming another principal's ticket.

## Options that are not built

### Rejected: `ticket_id` as the bearer

Possession of the id is the only check. Any connection that can call consume and can read or guess the id is the presenter. This is the gap 0.1O marked `NOT_PROVEN`. It stays rejected.

### Rejected: one database role per user

The pool default is 25 connections (`pool_config.go:20`). A role per principal cannot be the pool login without opening a new authenticated connection per request and giving shipment-service a password for every user. That mechanism does not exist. It is not a prerequisite for P2.

### Candidate: signed presenter assertion

This mechanism is not in the repository. The trust order, if it is built later, is:

1. The gateway verifies the access token and holds `AuthContext` (`auth.go:43-66`).
2. The issuer, called by the gateway, writes the ticket and signs a separate assertion that names `ticket_id`, `principal_id`, `tenant_id`, and the operation.
3. The signing key is readable by `binding_authority_owner` inside the consume function and not by `binding_authority_caller`.
4. The writer passes the assertion into consume. The function checks the signature, then compares the fields to the locked ticket, then writes.
5. A writer that can `SELECT` the key, or that connects as the owner or as a superuser, can forge the assertion. The grant design has to make that `SELECT` fail for the writer login.

The signature is the presenter check. The ticket row remains the one-use, expiry, and scope check. Neither one replaces the other.

Until that key and that function exist, presenter binding stays `NOT_PROVEN`. A header the writer sets, including `X-User-ID`, is not step 3.

## Bearer risk after gateway authentication

P2-B proves the issuance caller is the gateway. P2-A copies principal and tenant from the validated access token onto the ticket. Neither fact is checked again from a writer connection at consume time. A writer session that learns `ticket_id` still presents a bearer.

These consume-time facts are different things:

| Fact | What it binds |
| --- | --- |
| Writer database role | One shared login for every shipment-service connection. It is not a person. |
| Gateway service identity | The issuance request only. It is gone before consume. |
| Ticket scope | Operation, tenant, shipment, and execution. A different scope fails. |
| Where `ticket_id` is stored | `PROPOSED` that the writer receives it only inside the user request that was authorized. A copy in another session is not prevented by that storage rule. |
| Expiry and one use | A second committed consume fails. Rollback leaves the ticket `ISSUED`, so a stolen id can still be used once before expiry. An issuance retry returns that id only while the locked ticket is `ISSUED` and unexpired. A lost response does not insert a second ticket. The unique key is `(issuer_namespace_id, request_id)` and remains after expiry, revocation, consumption, and signing-key rotation. `payload_hash` remains with it. The first insert is limited by that key, not by a lock on a missing row. |
| Another writer session | The shared caller role cannot tell the sessions apart. |

A one-time bearer ticket is not a cryptographic binding to a presenter. The signed presenter assertion above would be that binding. It is not built. Gateway authentication does not supply it.

`PRESENTER_IDENTITY_BOUND=NOT_PROVEN`.

`PRESENTER_BINDING_FEASIBLE=NOT_ON_SHARED_LOGIN`.

If a future test claims presenter binding, a stolen ticket presented by another writer session must be denied. Until that check exists, the test is not claimed as passing.
