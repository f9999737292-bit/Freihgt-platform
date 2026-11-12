# EDO signing recovery 0.1 — discovery

Stage: `EDO-SIGNING-RECOVERY-0.1`

Base inspected: `origin/main` `3f306fc407aeda4bdc652d825954292f4a54fad6`

Mode: discovery. No service, migration, staging, or production change was made. Existing tests in the tree were not executed in this pass. Their result is `NOT_RUN`.

Canonical EDO documents already live as flat files under `docs/architecture/edo-*.md`, and ADRs live under `docs/adr/ADR-EDO-*.md`. This stage uses `docs/architecture/edo/` so it does not overwrite `docs/architecture/edo-signing-recovery-v0.1.md`.

## Confirmed facts

document-service owns the signing session and the signature row. There is no EDO operator client in the inspected service code. Qualified signature verification and an external operator exchange are `NOT_IMPLEMENTED`.

| Component | Location | Status |
| --- | --- | --- |
| document-service | `services/document-service` | Present |
| Signing session domain | `services/document-service/internal/domain/signing_session.go` | Present |
| In-process expired-session decision | `services/document-service/internal/domain/signing_recovery.go` | Present |
| Signing repository | `services/document-service/internal/repository/signing_repository.go` | Present |
| HTTP signing handlers | `services/document-service/internal/http/handlers/signing_handler.go` | Present |
| HTTP recovery route | `services/document-service/internal/http/router.go:59` | Registered as `RejectSigningRecovery` |
| Gateway recovery route | `services/api-gateway/internal/http/router.go:268` | Registered as `document.RejectSigningRecovery` before the document wildcard |
| Gateway recovery body | `services/api-gateway/internal/document/signing_recovery_gate.go:12` | Returns 403 and does not proxy |
| Attachment signature | `services/document-service/internal/service/attachment_service.go:285` | Present. Metadata and idempotency, not an operator signature |
| Attachment audit rows | `services/document-service/internal/repository/attachment_repository.go:216` | Present for attachment events |
| Signing recovery audit events | searched `services/document-service` for the proposed `SIGNING_*` names | `NOT_IMPLEMENTED` |
| EDO operator adapter | searched services for an operator client | `NOT_IMPLEMENTED` |
| Background signing worker | not found in document-service handlers or router | `NOT_FOUND_IN_INSPECTED_SCOPE` for a worker; no worker is registered on the router |

Document statuses in `document.go:14-22`: `DRAFT`, `READY_FOR_SIGNING`, `SIGNING_IN_PROGRESS`, `SIGNED`, `SENT_TO_OPERATOR`, `ACCEPTED`, `REJECTED`, `ARCHIVED`, `CANCELLED`.

Session statuses in `signing_session.go:13-18`: `CREATED`, `IN_PROGRESS`, `COMPLETED`, `EXPIRED`, `CANCELLED`, `FAILED`.

Signature types in `signing_session.go:20-22` include `ENHANCED_QUALIFIED`. Storing that label does not verify a qualified signature. `AddSignature` writes `verification_status` as `VALID` at `signing_repository.go:286`.

## Workflow inventory

| Workflow | Owner | Implementation | Recovery | Risk |
| --- | --- | --- | --- | --- |
| Create signing session | document-service | `SigningRepository.CreateSession` locks the document, requires `READY_FOR_SIGNING`, rejects another `CREATED` or `IN_PROGRESS` session, inserts `CREATED`, then sets the document to `SIGNING_IN_PROGRESS` and bumps `version` (`signing_repository.go:52-129`) | No resume of a partial insert. The transaction rolls back on failure before commit | A crash before commit leaves the previous document status. No external effect |
| Add internal signature | document-service | Locks document, locks session, reads `clock_timestamp()` after the lock, expires `CREATED`/`IN_PROGRESS` when `expires_at <= clock_timestamp()` without inserting a signature, otherwise inserts a signature and compare-and-sets the completed count (`signing_repository.go:205-339`) | Expiry is persisted and committed before the error returns. A failed count update does not leave a signature if the transaction rolls back | `verification_status` is set to `VALID` without cryptographic verification. No uniqueness on signer. No content hash |
| In-process recovery after persisted `EXPIRED` | document-service | `DecideSigningRecovery` (`signing_recovery.go:31-79`) and `RecoverExpiredSigning` (`signing_repository.go:342-413`) move the document from `SIGNING_IN_PROGRESS` to `READY_FOR_SIGNING` when the named session is the latest persisted `EXPIRED` session, the version matches, and no `COMPLETED`, `CREATED`, or `IN_PROGRESS` session exists. The session stays `EXPIRED` | This is the only implemented recovery. It does not create a session and does not call an operator | HTTP does not expose it. See the next row |
| HTTP signing recovery | document-service and api-gateway | Both routers call reject handlers. document-service returns `signing recovery is not available` (`signing_handler.go:141-143`). The gateway returns `signing recovery has no public write authority` (`signing_recovery_gate.go:12-14`) and does not proxy | No HTTP recovery mutation | A direct in-process caller can still run `RecoverExpiredSigning`. The router does not |
| Attachment signature metadata | document-service | `AttachSignature` requires a finalized attachment, a format in `CAdES`, `PKCS7`, `XMLDSIG`, `OTHER`, a reference, and rejects private-key material (`attachment_service.go:285-305`) | Idempotency key on the attachment signature input (`attachment_service.go:281`) | The reference is not proof of an operator acceptance. Verification of bytes is a separate EDO 0.3 design, not this recovery |
| Operator send / callback | none found | Document statuses `SENT_TO_OPERATOR`, `ACCEPTED`, and `REJECTED` exist. No adapter that sends them was found in the inspected services | `NOT_IMPLEMENTED` | External result cannot be reconciled because there is no provider reference |

## How the internal signature workflow behaves

Confirmed from the code above:

- The initiator of HTTP create and signature is whatever caller reaches the gateway wildcard or the document-service listener. When `AUTH_ENABLED=true`, the gateway document and signing wildcards use `RequireOperatorShipmentRead` (`router.go:269-272`). That guard is a tenant-global `PLATFORM_ADMIN` shipment read check. It is not a signer permission.
- Tenant for the handler is `trustedTenantID` (`signing_handler.go:49`, `:103`). That reads `X-Tenant-ID`. It does not verify a JWT inside document-service.
- Signer company and signer user are existence checks in the header tenant (`signing_service.go:112-126`). They are not a membership or a `document.sign` grant.
- Document version increments when a session is created, when a signature updates the document, and when in-process recovery returns the document to `READY_FOR_SIGNING`.
- The signed payload is a path pointer, `signature_payload_path` (`signing_session.go:51`). No document content hash is stored on the session signature in this repository method.
- The signature row is the stored result. There is no separate operator confirmation row.
- Session signature insert has no idempotency key in `AddSignatureInput` (`signing_session.go:65-72`).
- Timeout of the session is `expires_at`, compared with `clock_timestamp()` after the row lock. Equality is overdue (`signing_repository.go:242-258`).
- Restart recovery of an external operation is `NOT_IMPLEMENTED`. Restart during a database transaction follows ordinary rollback.
- Audit for signing recovery events is `NOT_IMPLEMENTED`. Attachment audit events exist for attachment actions.
- Repeating recovery is not an HTTP operation. In-process recovery refuses a version mismatch and refuses a document that is no longer `SIGNING_IN_PROGRESS` (`signing_recovery.go:54-65`).
- A second active session is rejected (`signing_repository.go:87-88`). A completed session blocks recovery (`signing_recovery.go:69-71`).

## Risks

- `SIGNED` is an internal document status after the required signer count is reached (`signing_session.go:138-141`). It is not evidence of a legally significant signature or of operator acceptance.
- HTTP recovery is fail-closed, so an operator cannot today resume an expired attempt through the API. The in-process function exists and is not the public route.
- Direct document-service writes still treat `X-Tenant-ID` as the tenant input. That gap is tracked by issue #343 and is outside this recovery design's implementation.
- No provider operation id exists, so F04, F05, and F13 have no stored external result to reconcile.

## Open questions

- Which bytes are the canonical signed payload for a future qualified signature? The session row stores a path, not a hash.
- Whether `GetSessionByID` without a tenant predicate (`signing_repository.go:132-140`) is reachable from production HTTP. The handler uses `GetSession` with the tenant (`signing_handler.go:84`).
- Whether attachment signature verification in EDO 0.3 I4 is in the runtime on this base or only specified. This pass did not treat design documents as runtime proof.

## Tests in this pass

`NOT_RUN`. No `go test` was started. Existing test names that cover the in-process rules include `services/document-service/internal/domain/signing_recovery_test.go` and `services/document-service/internal/integration/edo03schema/signing_recovery_integration_test.go`. Their last result is unknown to this pass.
