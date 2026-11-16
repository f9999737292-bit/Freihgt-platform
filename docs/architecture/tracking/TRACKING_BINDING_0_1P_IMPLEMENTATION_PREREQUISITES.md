# Implementation prerequisites — 0.1P

Status: not scheduled. Nothing in this file is applied. No migration number is reserved. `000103`, `000104`, and `000105` stay as they are. PostgreSQL tests for this list are `NOT_RUN`.

P2 from `TRACKING_BINDING_0_1P_TRUSTED_ISSUER_CONTRACT.md` is the proposal. P1 is recorded there and is not the selected boundary.

## Future privilege shape

These statements are a checklist for a later change. They are not SQL to run now.

| Item | Future state |
| --- | --- |
| `binding_authority_issuer` | `LOGIN`, non-superuser, `NOINHERIT`, `NOBYPASSRLS`. Password only on the issuer process. |
| `binding_authority_owner` | `NOLOGIN`, non-superuser. Owns functions, ticket table, and audit table. Does not own the execution or active-link tables. |
| `binding_authority_caller` | `LOGIN`, non-superuser, `NOINHERIT`. The shipment writer pool. |
| Issuer `EXECUTE` | Granted to `binding_authority_issuer` only. Revoked from `PUBLIC`, `freight`, `shipment_runtime`, and `binding_authority_caller`. |
| Consume `EXECUTE` | Granted to `binding_authority_caller` only. |
| Membership | No `GRANT issuer TO caller`, no `GRANT owner TO caller`, no `GRANT issuer TO freight`. |
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

| Test | Required result |
| --- | --- |
| Writer calls the issuer function | `42501` |
| Writer `SET ROLE binding_authority_issuer` | denied |
| Ticket for another tenant | consume denied, no write |
| Ticket for another presenter | consume denied. A shared caller that only knows `ticket_id` does not pass |
| Two consumes, one committed ticket | the second denied |
| Expired ticket and revoked ticket | both denied |
| Create ticket names E1, writer inserts E2 | denied |
| Writer sets a session variable or SQL argument to another tenant | no ticket and no write |
| Writer `INSERT` or `UPDATE` on the active link, the ticket, or `owner_epoch` | `42501` for that non-owner, non-superuser login |
| Consume error after the audit insert | no execution change, no link change, no audit row, ticket still `ISSUED` |

A passing test on a superuser connection does not count. The session under test is the writer login.

## Still closed

`J-DML-001` is not closed. Tenant isolation is not implemented by this document. Presenter identity is `NOT_PROVEN`. Decision consumption stays closed. Broker safety is not proved. B3 stays `PROPOSED_NOT_APPROVED`. The implementation gate stays closed.
