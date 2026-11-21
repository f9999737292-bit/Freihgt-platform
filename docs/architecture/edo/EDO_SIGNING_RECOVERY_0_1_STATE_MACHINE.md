# EDO signing recovery 0.1 — state machine

This is a design model. It is not a database migration.

## Map onto statuses that already exist

| Proposed recovery state | Existing status it must not duplicate | Meaning |
| --- | --- | --- |
| `CREATED` | `documents.signing_sessions.status = CREATED` | Session row committed. No signature yet |
| `PREPARED` | No column | Bytes and hash frozen for this attempt. Not stored today |
| `SIGNING` | `IN_PROGRESS` | At least one internal signature has been accepted, or a provider call has started. Today `IN_PROGRESS` means the completed count is still below the required count (`signing_session.go:138-142`) |
| `RESULT_UNKNOWN` | No column. Do not reuse `FAILED` | An external call may have been accepted and the response was not stored |
| `SIGNED` | Document `SIGNED` and session `COMPLETED` | Internal count reached. Not operator confirmation and not legal sufficiency |
| `CONFIRMED` | Document `ACCEPTED` is the closest legal-facing status and is unused by the signing repository | Operator or verifier has confirmed this attempt. `NOT_IMPLEMENTED` |
| `FAILED_RETRYABLE` | No column. `EXPIRED` is narrower: the technical window elapsed | The attempt ended and a new attempt is allowed only under the recovery rules |
| `FAILED_FINAL` | Session `FAILED` or `CANCELLED`. Neither is written by `AddSignature` or `RecoverExpiredSigning` | No further attempt for this operation id |
| `MANUAL_REVIEW_REQUIRED` | No column | Evidence is incomplete or contradictory |

Document status and session status stay the legal and workflow record. Recovery states that are only technical (`PREPARED`, `RESULT_UNKNOWN`, `MANUAL_REVIEW_REQUIRED`) need their own operation row in a later stage. They must not be stuffed into `document_status`, because `SIGNED` would then be confused with an unknown provider call.

In-process recovery today does one document transition: `SIGNING_IN_PROGRESS` to `READY_FOR_SIGNING`, and only from a persisted `EXPIRED` session (`signing_recovery.go:28-30`). The session remains `EXPIRED`. A later `CreateSession` starts a new id. That behavior stays. This model does not replace it with an automatic retry.

## Allowed transitions

| From | To | Owner | Guard |
| --- | --- | --- | --- |
| none | `CREATED` | document-service `CreateSession` | Document is `READY_FOR_SIGNING`. No `CREATED` or `IN_PROGRESS` session. Same tenant |
| `CREATED` | `PREPARED` | future preparation step | Content hash and version stored. No provider call |
| `PREPARED` | `SIGNING` | signature attempt | Signer is still authorized. Hash matches |
| `CREATED` or `IN_PROGRESS` | `EXPIRED` | `AddSignature` when overdue | `clock_timestamp()` after the session lock. No signature insert. This exists today |
| `SIGNING` | `SIGNED` | local count reached | Compare-and-set on the locked count. Internal status only |
| `SIGNING` | `RESULT_UNKNOWN` | provider adapter | Timeout, lost response, or lost commit acknowledgement |
| `RESULT_UNKNOWN` | `SIGNED` or `CONFIRMED` | reconciler | Provider evidence matches operation id, tenant, version, and hash |
| `RESULT_UNKNOWN` | `FAILED_RETRYABLE` | reconciler | Provider evidence says the call was not accepted |
| `RESULT_UNKNOWN` | `MANUAL_REVIEW_REQUIRED` | reconciler | Evidence is missing or contradicts the stored hash |
| `EXPIRED` plus document `SIGNING_IN_PROGRESS` | document `READY_FOR_SIGNING` | in-process recovery | Rules in `DecideSigningRecovery`. HTTP stays 403 until a later authorized stage |
| `READY_FOR_SIGNING` | new `CREATED` | `CreateSession` | New session id. Previous session stays `EXPIRED` |
| `SIGNED` | `CONFIRMED` | operator adapter | Separate from the internal count. Not implemented |
| `MANUAL_REVIEW_REQUIRED` | `FAILED_FINAL` or back to a proven state | an authorized reviewer | Not a platform admin signing as the party |

## Forbidden transitions

- `RESULT_UNKNOWN` to `FAILED_RETRYABLE` or `FAILED_FINAL` only because the response is missing.
- `EXPIRED` to a new provider signature without a new session created under the current authorization.
- `COMPLETED` or document `SIGNED` back to `READY_FOR_SIGNING` through recovery. `DecideSigningRecovery` already rejects a `COMPLETED` session.
- Changing signed bytes, moving historical signatures onto a new session, or clearing `completed_signers_count` on an old session.
- `CONFIRMED` to `SIGNING` because a late callback failed for another attempt.
- Any transition that reads or writes a different tenant.

## Owners

document-service owns session and document rows. api-gateway may authenticate the caller and must not invent the provider result. A future operator adapter owns only the external request and the evidence it returns. A reviewer owns `MANUAL_REVIEW_REQUIRED` decisions and does not hold the party's signing key.

## Invariants

- The bytes identified by the prepared hash do not change after `PREPARED`.
- A signature row, once committed, is not deleted or rebound by recovery.
- `clock_timestamp()` used for expiry stays a separate statement after the session lock. It is not replaced by application time taken before the lock.
- Equality of `expires_at` and that timestamp stays overdue.
- HTTP `POST /v1/documents/{id}/signing-recovery` stays 403 until an implementation stage is explicitly authorized. The in-process function is not that HTTP route.
- `SIGNED` does not mean `CONFIRMED`.

## Terminal states

`SIGNED` is terminal for the internal count of that session. `CONFIRMED` is terminal for operator acceptance of that attempt. `FAILED_FINAL` is terminal for that operation id. `EXPIRED` is terminal for that session and is not terminal for the document: the document may return to `READY_FOR_SIGNING` and a new session may start. `MANUAL_REVIEW_REQUIRED` is terminal until a person resolves it.

## Retry

Retry of the same operation id replays the stored result. It does not call the provider again while the state is `RESULT_UNKNOWN` unless that provider's contract proves the operation id is idempotent. A new signing operation requires a new operation id, a current authorization, and the document version that is still the prepared version. The existing recovery path does not itself create that new session.

## Reconciliation

Reconciliation reads provider evidence and the local operation row. It is allowed to move `RESULT_UNKNOWN` to a proven state. It is not allowed to create a signature the provider did not report. Without a provider, reconciliation is `NOT_IMPLEMENTED`.

## Crash

Before commit, the database is the previous committed state. After commit, the row is the state. If the process dies between a provider acceptance and the local commit, the next start must load `RESULT_UNKNOWN` rather than start a second provider call. That row does not exist yet.
