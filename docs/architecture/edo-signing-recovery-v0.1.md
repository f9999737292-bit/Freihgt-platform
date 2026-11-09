# EDO signing recovery 0.1

Stage: `EDO-SIGNING-RECOVERY-0.1`

This stage returns a document from `SIGNING_IN_PROGRESS` to `READY_FOR_SIGNING` after the current signing attempt is persisted as `EXPIRED`. It does not create the next signing session. The next attempt is an ordinary `POST /v1/documents/{id}/signing-sessions`.

Internal `SIGNED` remains an internal document status. It is not confirmation of an external legally significant exchange.

`GAP-B-001` stays open. This stage does not authorize requirement-policy runtime or an EDO operator.

## HTTP availability

`PUBLIC_SIGNING_RECOVERY=DISABLED`

`EXPECTED_PUBLIC_RESPONSE=403`

Both of these routes reject the request and do not call recovery:

- API gateway: `POST /api/v1/documents/{id}/signing-recovery`
- document-service: `POST /v1/documents/{id}/signing-recovery`

`X-Tenant-ID`, a bearer token, a body `tenant_id`, and `X-Internal-Service-Token` do not authorize this operation. `RequireOperatorShipmentRead` is not document write authority. The document-service router returns `403` without reading those credentials and without calling `RecoverExpiredSigning`.

The OpenAPI operation is generated from `scripts/openapi/generate_openapi.py` with `PUBLIC_SIGNING_RECOVERY=DISABLED` and response `403`. It does not describe a successful recovery and does not treat `bearerAuth` as authority.

Shipper and carrier document routes stay read-only.

## Internal operation

`SigningService.RecoverExpiredSigning` and `SigningRepository.RecoverExpiredSigning` remain available to trusted in-process callers. They are not registered on the HTTP router.

The internal request identifies:

- `signing_session_id`
- `expected_version`, the stored document version
- the trusted tenant already held by the caller

A caller-supplied tenant is not the source of authority. When an HTTP handler is exercised directly in a unit test, a body `tenant_id` must equal that trusted tenant or the handler returns `401`. That unit test is not the document-service route.

A successful internal call returns the updated document and the unchanged expired session. It does not create a session.

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

## Write authority

No document write-authority contract is defined. This stage does not add one. `GAP-B-001` stays open.
