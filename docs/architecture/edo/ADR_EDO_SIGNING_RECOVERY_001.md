# ADR-EDO-SIGNING-RECOVERY-001

Status: proposed. Not accepted. Agent J has not reviewed it.

Date: 2026-11-12

Base: `3f306fc407aeda4bdc652d825954292f4a54fad6`

This file lives under `docs/architecture/edo/` because this stage is one package. Historical EDO decisions remain in `docs/adr/ADR-EDO-001` through `ADR-EDO-012`. This number does not replace `ADR-EDO-001`, which is canonical document ownership.

## Decision

1. document-service owns the signing operation row and the session. api-gateway authenticates the public caller and does not decide the signature result.
2. The source of truth for an internal signature is the committed `documents.signatures` row and the session status updated in the same transaction.
3. The source of truth for an external operation is provider evidence tied to the operation id. Until that adapter exists, there is no external source of truth. A timeout is not a failure.
4. Bytes are immutable after preparation. Recovery does not edit them, does not move old signatures, and does not clear an old `completed_signers_count`.
5. Idempotency is the server operation id plus tenant, document, version, content hash, and signer. The hash alone is not the operation id. HTTP replay returns the stored result.
6. Reconciliation is a read of provider evidence followed by one allowed transition out of `RESULT_UNKNOWN`. It is not implemented.
7. Retry of the same operation id does not create a second provider effect while the outcome is unknown, unless that provider's contract proves idempotency. A new attempt uses a new session, which matches today's `CreateSession` after the document is `READY_FOR_SIGNING`.
8. Tenant isolation is a predicate on every read and write. A foreign tenant is not found.
9. Manual intervention resolves `MANUAL_REVIEW_REQUIRED`. It does not sign as the party. `PLATFORM_ADMIN` is not that right.
10. The audit trail is an append-only event list. It stores identifiers, states, and reason codes. It does not store private keys.
11. After a restart the service reads committed rows. It does not sign during startup.
12. An operator integration requires a named provider, a statement of whether the operation id is idempotent, and a separate legal and security approval. Stage 0.5 is that work.
13. Open items are listed below.
14. Alternatives that were not chosen: treating `SIGNED` as operator confirmation, using the document hash as the only idempotency key, marking timeouts as `FAILED`, and exposing `RecoverExpiredSigning` on HTTP in this stage.

## Context

The current internal flow is real and limited. `CreateSession` and `AddSignature` are transactional. Expiry uses `clock_timestamp()` after the session lock and commits `EXPIRED` without a signature. In-process recovery returns a document from `SIGNING_IN_PROGRESS` to `READY_FOR_SIGNING` only for the latest persisted `EXPIRED` session. Both HTTP recovery routes return 403 and do not call that function.

No operator client, no provider operation id, and no signing-session audit event were found.

## Consequences

Later stages add an operation record rather than overloading `document_status`. HTTP recovery stays closed until a stage explicitly opens a narrow, authorized transition. Internal `SIGNED` remains an internal count.

Legal effect depends on the signature type, the operator, and the applicable rules. Those are not established by this ADR. `ENHANCED_QUALIFIED` as a stored label is not that establishment.

## Open questions

- The canonical byte range that a future hash must cover.
- Whether session expiry and signer revocation are the same control. Today only the timestamp is enforced.
- Who may hold the reviewer permission.
- Which operator, if any, is in scope for 0.5.
- Whether `GetSessionByID` without a tenant filter is used outside tests.
