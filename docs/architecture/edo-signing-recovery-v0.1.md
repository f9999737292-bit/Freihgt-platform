# EDO signing recovery 0.1

Stage: `EDO-SIGNING-RECOVERY-0.1`

This stage returns a document from `SIGNING_IN_PROGRESS` to `READY_FOR_SIGNING` after the current signing attempt is persisted as `EXPIRED`. It does not create the next signing session. The next attempt is an ordinary `POST /v1/documents/{id}/signing-sessions`.

Internal `SIGNED` remains an internal document status. It is not confirmation of an external legally significant exchange.

`GAP-B-001` stays open. This stage does not authorize requirement-policy runtime or an EDO operator.

## Endpoint

Document service:

`POST /v1/documents/{id}/signing-recovery`

OpenAPI path, generated from `scripts/openapi/generate_openapi.py` in the same shape as the other signing routes:

`POST /api/v1/documents/{id}/signing-recovery`

The method is `POST`. `GET` does not recover a document.

Request body:

- `signing_session_id` (required)
- `expected_version` (required, the stored document version)
- `tenant_id` (optional echo)

Authority is the trusted `X-Tenant-ID` header established by the gateway. A body or query `tenant_id` is not the source of authority. When the body contains `tenant_id`, it must equal the trusted header; otherwise the response is `401`.

Success is `200` with the updated document and the unchanged expired session. The response does not contain a new session.

## Allowed transition

All of the following are read from stored rows inside one transaction, after the document row and that document's signing sessions are locked:

1. The document and the named session belong to the trusted tenant, and the session belongs to that document.
2. The stored session status is `EXPIRED`. A client status, a client `expires_at`, or an overdue `CREATED`/`IN_PROGRESS` row is not sufficient. Recovery does not expire a session.
3. The stored document status is `SIGNING_IN_PROGRESS`.
4. The stored document version equals `expected_version`.
5. The named session is the latest session for that document (`created_at`, then `id`).
6. No session for that document is `COMPLETED`, `CREATED`, or `IN_PROGRESS`.

The update sets `document_status` to `READY_FOR_SIGNING` and increments `version` by one. The named session stays `EXPIRED`. Its `expires_at`, `completed_signers_count`, and signature rows stay unchanged.

`SIGNED`, `ACCEPTED`, `ARCHIVED`, `CANCELLED`, `SENT_TO_OPERATOR`, `READY_FOR_SIGNING`, `DRAFT`, and `REJECTED` are not recoverable through this action. `ValidateReadyForSigningStatus` is unchanged.

If the stored rows do not identify exactly one allowed recovery, the transaction returns an error and commits nothing.

## Repeat

There is no recovery idempotency key, so a repeat is not a replay of the previous body.

- The same `expected_version` after a successful recovery no longer matches the stored version. The response is `409` and `version` is not incremented again.
- A later call that presents the post-recovery version sees `READY_FOR_SIGNING` and is rejected without a further version increment.
- After a new `CreateSession`, the document is `SIGNING_IN_PROGRESS` again. Recovery with the old session id is rejected because that session is no longer the current attempt or because a new active session exists.

## New attempt

`CreateSession` rechecks, inside its own transaction and under the same document-then-sessions lock order, that the document is `READY_FOR_SIGNING` and that no `CREATED` or `IN_PROGRESS` session exists. The new session has a new id and `completed_signers_count = 0`. It does not receive the previous signatures.

An `expires_at` that is not in the future is rejected, and no session is inserted.

## Errors

| Condition | HTTP |
| --- | --- |
| Missing or malformed trusted tenant, or body tenant mismatch | 401 |
| Session missing for the trusted tenant and document | 404 |
| Session is not persisted `EXPIRED`, or document status is not `SIGNING_IN_PROGRESS` | 400 |
| Stale `expected_version`, active session, completed attempt, or an older expired session | 409 |

## Public gateway

`POST /api/v1/documents/{id}/signing-recovery` is registered before the document wildcard and returns `403`. `RequireOperatorShipmentRead` is not a document write-authority contract, and this stage does not publish the mutation through that wildcard. Shipper and carrier document routes stay read-only.

`AUTHORIZATION_BLOCKER`: no document write-authority contract is available for a public recovery route.
