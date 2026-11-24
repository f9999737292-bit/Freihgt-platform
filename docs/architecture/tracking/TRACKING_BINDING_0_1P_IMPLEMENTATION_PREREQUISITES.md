# Implementation prerequisites — 0.1P

Status: not scheduled. Nothing in this file is applied. No migration number is reserved. `000103`, `000104`, and `000105` stay as they are. PostgreSQL tests for this list are `NOT_RUN`.

P2 from `TRACKING_BINDING_0_1P_TRUSTED_ISSUER_CONTRACT.md` is `PROPOSED_NOT_APPROVED`. The caller proof is a gateway-only signature. The principal and tenant come from the access token the issuer validates. `X-Internal-Service-Token` is not that proof. P1 is recorded there and is not the selected boundary. The shared `freight` login is still the compose pool, so the role split is not in force.

## Future privilege shape

These statements are a checklist for a later change. They are not SQL to run now.

| Item | Future state |
| --- | --- |
| `binding_authority_issuer` | `LOGIN`, non-superuser, `NOINHERIT`, `NOBYPASSRLS`. Password only on the issuer process. |
| `binding_authority_owner` | `NOLOGIN`, non-superuser. Owns functions, ticket table, and audit table. Does not own the execution or active-link tables. |
| `binding_authority_caller` | `LOGIN`, non-superuser, `NOINHERIT`. The shipment writer pool. |
| `binding_authority_fencing` | Future `LOGIN`, non-superuser, `NOINHERIT`. Not created. Not the issuer, the owner, the caller, or `freight`. Runs handover only. |
| Issuer `EXECUTE` | Granted to `binding_authority_issuer` only. Revoked from `PUBLIC`, `freight`, `shipment_runtime`, and `binding_authority_caller`. |
| Consume `EXECUTE` | Granted to `binding_authority_caller` only. |
| Membership | No `GRANT issuer TO caller`, no `GRANT owner TO caller`, no `GRANT issuer TO freight`, and no `GRANT` of `binding_authority_fencing` to those roles. |
| `SET ROLE` | Caller is not a member, so `SET ROLE` to the issuer or the owner fails. |
| Ticket and audit DML | Writer has no `INSERT` or `UPDATE`. Issuer inserts tickets. The owner function updates the ticket to `CONSUMED` and inserts `audit.ticket_id`. |
| Execution and link DML | Table `INSERT` and the sensitive `UPDATE` leave `shipment_runtime` only when the consume function replaces those writes. Removing them first stops `Project`. |
| `freight` | Stays until the new logins are proven. Dropping it is not part of the first privilege change. |

`freight_migrations` is not the function owner. It already owns the pin function (`000103:1150`).

A `BEFORE INSERT` or `BEFORE UPDATE` trigger may reject a bad epoch. The table owner and a superuser can drop it, and `session_replication_role = replica` can skip it. The login split is the boundary. The trigger is not.

## Existing sessions that depend on `freight`

Replacing the shared login breaks every pool that uses it before that pool has its own login. The inventory lists them: identity, company, transport-order, rfx, shipment, document, billing-register, payment, low-code, and the control-tower read model. Tracking-service uses the same fallback user and is not a binding-table writer.

`shipment_runtime` cannot take over those pools. It is `NOLOGIN` (`000103:31` and `:48`). The production connect path does not `SET ROLE`.

Ownership of `transport.transport_execution_active_shipments` is `NOT_PROVEN` (`000088:76` creates it and does not name an owner). A later owner change has to name the live owner first. This discovery does not.

## Order

1. Inventory the live owner of the execution and active-link tables, and whether `freight` is superuser or bypasses row security. Do not invent those facts from compose.
2. Add the issuer process and its login in a disposable database. The gateway is the only caller. Shipment-service has no issuer password.
3. Prove the writer login cannot `EXECUTE` the issuer, cannot `SET ROLE` to it, and cannot insert a ticket.
4. Only then point shipment-service at the writer login and replace the binding writes with consume.
5. Leave `freight` able to connect until step 4 has passed. Restoring the old DSN is a return to the shared principal, not the recovery path for a bad ticket.

## Isolated tests, not run

The witness reservation, the fencing epoch, the namespace transition, and the unique index on `(issuer_namespace_id, request_id)` are `PROPOSED_NOT_IMPLEMENTED`. `OLD_PRIMARY_FENCING=NOT_PROVEN`. Holding an epoch row `FOR UPDATE` does not reject the holder's `COMMIT`. `WITNESS_FAILURE_DOMAIN=NOT_PROVEN`. No storage product is selected for the witness. The document object store is not that witness. The rows below are not executed.

| Test | Required result |
| --- | --- |
| `X-Internal-Service-Token` and no gateway signature | Denied. No ticket |
| Body principal or tenant different from the validated token | Denied |
| Signature by a key that is not the gateway key | Denied |
| Gateway signature, valid token, and a completed role check | One ticket |
| Expired gateway key or expired access token | Denied |
| Same signed request, ticket still `ISSUED` and unexpired | `ISSUE_REPLAY_ISSUED`. The same id. No second ticket |
| Same signed request, ticket `REVOKED` | `ISSUE_DENIED_REVOKED` |
| Same signed request, ticket `EXPIRED` | `ISSUE_DENIED_EXPIRED` |
| Status still `ISSUED`, `expires_at` not later than `clock_timestamp()` | `ISSUE_DENIED_EXPIRED` |
| Same signed request, ticket `CONSUMED` | `ISSUE_DENIED_CONSUMED`. The id is not returned as a new grant |
| Same `request_id`, different payload | `ISSUE_DENIED_PAYLOAD_MISMATCH` |
| Two concurrent first inserts, same payload | The witness accepts one reservation. The other caller does not start PostgreSQL. One ticket when that reservation reaches `COMMIT_CONFIRMED` |
| Two concurrent first inserts, different payload | One committed ticket. The other, in a new transaction, gets `ISSUE_DENIED_PAYLOAD_MISMATCH` |
| The first transaction rolls back | The other may insert the single ticket after repeating the signature, token, and operation checks |
| `unique_violation` | That transaction is aborted and rolled back. The status read runs in a new transaction |
| Missing `gateway_key_id` or `request_id` | `ISSUE_DENIED_AUTH`. No row. `NULL` is not inserted |
| Lost response, then the same signed request | No new reservation. Return the id only when the witness is `COMMIT_CONFIRMED` and the locked ticket is still `ISSUED` and unexpired |
| Retry waits with a revocation | The retry does not return the ticket after the revocation commit |
| Unknown ticket status | `ISSUE_DENIED_UNKNOWN_STATUS` |
| Insert error other than the unique conflict | Rollback. No ticket and no idempotency row |
| Same `request_id` with a different operation | `ISSUE_DENIED_PAYLOAD_MISMATCH` |
| Retry after `REVOKED`, `EXPIRED`, or `CONSUMED` | Denied. The tombstone remains. No second ticket |
| Retry after `ticket.expires_at` while the signature window is still open | `ISSUE_DENIED_EXPIRED`. The row is still there |
| Retry after the signature window | Denied. The tombstone remains |
| Same `request_id` signed again, including with a new gateway key | No second ticket. The same ticket only when the canonical hash matches and the ticket is still `ISSUED` and unexpired. Otherwise denied |
| Missing or unreadable `payload_hash` | `ISSUE_DENIED_AUTH`. No ticket id |
| Cleanup after the signature window | The access token may be dropped. `payload_hash`, the unique key, and the terminal status remain |
| First issue with key K1 and request R | One ticket |
| Repeat K1 and R, same canonical payload, ticket still valid | `ISSUE_REPLAY_ISSUED` |
| Repeat K1 and R, different canonical payload | `ISSUE_DENIED_PAYLOAD_MISMATCH` |
| Key K2 signs request R after rotation | No second ticket |
| Retired key K1 and existing R | The tombstone and the retired-key record remain |
| A new `request_id` | Full authorization. One ticket if it commits |
| Two concurrent first inserts under two active signing keys and one `request_id` | One ticket |
| Issuer process restart with the committed history present | The history is still there. No rebuilt ticket. The witness is read before the gate can open |
| Failover that still has the confirmed history | The gate stays closed. A person's approval is not the reservation proof and is not a fence |
| Restore from a backup that can omit a tombstone | `ISSUER_ISSUANCE_GATE=CLOSED`. The witness reservations remain. No new ticket |
| History gap | A skipped reservation sequence, or a ticket with no reservation. Issuance stays closed. A row count is not the detector |
| Reconciliation | Does not mint a ticket. `RESERVED` and `IN_DOUBT` are not released on a timeout |
| Witness reserve before PostgreSQL begins | One `RESERVED` record. No second reservation. No ticket yet |
| Reservation durable, PostgreSQL insert not started | No second ticket. A timeout does not free the key |
| Crash before PostgreSQL `COMMIT` | The reservation remains. No automatic new ticket |
| PostgreSQL `COMMIT` before the witness outcome | The reservation blocks a second ticket. No HTTP grant until `COMMIT_CONFIRMED` |
| Crash after PostgreSQL `COMMIT` | Outcome becomes `IN_DOUBT` or `COMMIT_CONFIRMED`. No second ticket |
| Epoch 10 transaction still open | Its `COMMIT` is not rejected by the epoch-row lock. The gate stays `CLOSED` |
| Handover requested | State `HANDOVER_PENDING`. Neither epoch receives a new reservation. The gate stays `CLOSED` |
| Backend count before the reconnect ban | Quiesce does not start. Gate `CLOSED` |
| `REVOKE CONNECT` without a failed login | Not a confirmed ban. Gate `CLOSED` |
| Ban confirmed, then the old identity connects | Stay in `HANDOVER_PENDING`. No new epoch |
| `freight` connects during handover | The ban is not confirmed. Gate `CLOSED`. `OLD_PRIMARY_FENCING` stays `NOT_PROVEN` |
| Old backend termination requested | The request is not proof the backend is gone. The gate stays `CLOSED` until a later observation |
| Open transaction ends by rollback | The reservation stays. No second ticket |
| Old credential connects again | Connection denied only when that denial is observed. Until then the gate stays `CLOSED` |
| New epoch issues | Only after the old backends are gone and the old credential cannot connect. A larger epoch number is not enough |
| PostgreSQL epoch update crashes | `HANDOVER_PENDING` remains. The gate stays `CLOSED` |
| Witness epoch update crashes | `HANDOVER_PENDING` remains. The two writes are not one transaction |
| Two handover requests | One `HANDOVER_PENDING`. The second does not open a second transition |
| Old primary still running | Its new reservation is rejected. Its in-flight `COMMIT` is `NOT_PROVEN` to fail |
| Lagging replica promoted | Gate `CLOSED`. Old reservations are not new authority |
| Two timelines issue at once | One reservation per `request_id`. Both cannot reach `COMMIT_CONFIRMED` for that request |
| Stale fencing epoch at the witness | No new reservation |
| Stale writer commits in PostgreSQL | Not proven blocked while `freight` can write. Gate stays `CLOSED` |
| Witness unavailable | Gate `CLOSED`. No `ISSUE_OK` |
| Witness and PostgreSQL disagree | Gate `CLOSED`. No second ticket |
| Namespace in configuration differs from the witness | Gate `CLOSED` |
| Gateway key rotation | The namespace is unchanged. No second ticket |
| Namespace changed without the approved workflow | Gate `CLOSED`. Old `request_id` values are not a new authority |
| Same `request_id` after recovery | No second ticket |
| Witness retains a `request_id` whose tombstone is missing | The key stays forbidden. No new ticket |
| Replay after the access token was dropped | The digest and the witness record remain. The same ticket only while it is still `ISSUED` and unexpired. Otherwise denied |
| Cleanup runs during a retry | No second ticket. Cleanup does not delete the unique key and locks the idempotency row before the ticket |
| Cleanup runs during a first insert of that same key | The unique key, once committed, still conflicts. Cleanup does not remove it |
| Tombstone still stored | A later insert of that `request_id` does not commit a new ticket |
| Retry, revocation, and cleanup together | Cleanup locks the idempotency row before the ticket and keeps the key and the digest. Revocation locks only the ticket. No second ticket |
| Writer calls the issuer function | `42501` |
| Writer `SET ROLE binding_authority_issuer` | Denied |
| Ticket for another tenant | Consume denied, no write |
| Stolen `ticket_id` from another writer session | Denied only when presenter binding is claimed. It is not claimed. `PRESENTER_IDENTITY_BOUND=NOT_PROVEN` |
| Two consumes, one committed ticket | The second denied |
| Expired ticket and revoked ticket | Both denied |
| Create ticket names E1, writer inserts E2 | Denied |
| Writer sets a session variable or SQL argument to another tenant | No ticket and no write |
| Writer `INSERT` or `UPDATE` on the active link, the ticket, or `owner_epoch` | `42501` for that non-owner, non-superuser login |
| Consume error after the audit insert | No execution change, no link change, no audit row, ticket still `ISSUED` |

A passing test on a superuser connection does not count. The session under test is the writer login. Issuance-retry tests use the issuer login. No SQLSTATE is required by this list. The proof records the SQLSTATE the function raises and the result code above. The HTTP response is not part of the database commit.

## Still closed

`J-DML-001` is not closed. Tenant isolation is not implemented by this document. Presenter identity is `NOT_PROVEN`. Decision consumption stays closed. Broker safety is not proved. B3 stays `PROPOSED_NOT_APPROVED`. The implementation gate stays closed.
