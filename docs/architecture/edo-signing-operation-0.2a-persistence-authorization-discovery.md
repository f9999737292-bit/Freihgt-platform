# EDO signing operation 0.2A — persistence and authorization discovery

Stage: `EDO-SIGNING-RECOVERY-0.2A-RUNTIME-PERSISTENCE-DISCOVERY`

Mode: documentation. This file proposes a PostgreSQL shape for a later stage. It does not add a migration, a route, a permission grant, a provider client, or a signature.

Inspected baseline: `origin/main` `a9ef8f40bf0e7f75cb4aee3794e7841723347e4c` (`Merge pull request #367`).

Related implementation: Draft PR #397, head `7fe59bf090e9ab53c8eaaf833eff5bcdc014ca80`, base `3f306fc407aeda4bdc652d825954292f4a54fad6`. That pull request stays unchanged.

Product tests in this pass: `NOT_RUN`. Statements about current behavior come from reading the cited source. They are not a fresh `go test` result.

## 1. Current implementation

`document-service` owns the internal signing session and the signature row. On `origin/main` the operation model from PR #397 is absent. `git diff --stat 3f306fc407aeda4bdc652d825954292f4a54fad6 origin/main` for `services/document-service`, `services/api-gateway/internal/http/router.go`, and `services/api-gateway/internal/document` was empty, so the signing files cited below match both SHAs.

| Piece | Location on the inspected tree | What it does |
| --- | --- | --- |
| Session and document statuses | `services/document-service/internal/domain/signing_session.go`, `document.go` | Session: `CREATED`, `IN_PROGRESS`, `COMPLETED`, `EXPIRED`, `CANCELLED`, `FAILED`. Document: `DRAFT` through `CANCELLED`, including `SIGNED`, `SENT_TO_OPERATOR`, `ACCEPTED`, `REJECTED` |
| Create session | `signing_repository.go` `CreateSession` | One transaction. Locks the document for the tenant, rejects a second `CREATED` or `IN_PROGRESS` session, inserts `CREATED`, then sets the document to `SIGNING_IN_PROGRESS` and increments `version` |
| Add signature | `signing_repository.go` `AddSignature` | Locks document, then session. Overdue `CREATED`/`IN_PROGRESS` becomes `EXPIRED` with no signature insert. Otherwise inserts `documents.signatures` with `verification_status = VALID` and compare-and-sets the completed count |
| In-process expiry recovery | `signing_recovery.go` `DecideSigningRecovery`, `signing_repository.go` `RecoverExpiredSigning` | One transition: document `SIGNING_IN_PROGRESS` to `READY_FOR_SIGNING` for the latest persisted `EXPIRED` session. The session stays `EXPIRED`. No new session is created |
| Public recovery | document-service `router.go` `RejectSigningRecovery`; gateway `router.go` `POST /api/v1/documents/{id}/signing-recovery` | Both return 403 and do not call `RecoverExpiredSigning` |
| Operation domain | PR #397 only: `signing_operation.go`, `signing_operation_test.go` | In-memory guards. No repository, no HTTP registration |
| `document.sign` | `infrastructure/migrations/000008_seed_permissions.up.sql` | Permission code exists. No use of that code was found under `services/document-service` or `services/api-gateway` on `origin/main` |
| Migration head | `infrastructure/migrations/000105_platform_postgres_role_foundation_v0_2_i1.up.sql` | Highest `*.up.sql` observed on this baseline. This document does not reserve the next number |

`documents.documents.version` is an integer incremented by session creation, signature completion, and in-process recovery. `documents.document_versions` stores payload paths. `documents.document_files.checksum_sha256` exists. This pass did not prove that the checksum column is the canonical byte range a future qualified signature must cover.

`documents.signatures` has no uniqueness on signer, operation id, or payload hash. `AddSignatureInput` has no idempotency key. Attachment signature upload uses `Idempotency-Key` and is a separate contract. It does not identify a signing operation.

`SigningService.AddSignature` checks that the signer user and signer company exist in the header tenant. Existence is not a `document.sign` grant. `trustedTenantID` reads `X-Tenant-ID`. The gateway document and signing wildcards use `RequireOperatorShipmentRead`. That guard is not company signing authority. Shipper and carrier document routes used by the gateway handlers inspected here are GET routes.

`docs/architecture/edo-signing-recovery-v0.1.md` records `PUBLIC_SIGNING_RECOVERY=DISABLED` and `EXPECTED_PUBLIC_RESPONSE=403`. ADR-EDO-001 keeps canonical document ownership in document-service. ADR-EDO-004 and ADR-EDO-008 keep an external operator behind a port that is not implemented here. ADR-EDO-010 and ADR-EDO-011 separate attachment verification and a production GOST verifier from this recovery path. ADR-EDO-007 leaves legal retention open. ADR-EDO-012 is the durable object-storage contract and is not changed here.

Internal document status `SIGNED` means the required signer count was reached inside `ResolveSigningAfterSignature`. It is not operator acceptance and it is not a legally sufficient signature. The five technical operation states below are not that document status.

## 2. Domain state and recovery contract

PR #397 defines these technical states and no others:

| State | Holds the tenant+document scope | Meaning in the domain function |
| --- | --- | --- |
| `PREPARED` | Yes | Identity accepted. Optional local bookkeeping marker may be present |
| `RESULT_UNKNOWN` | Yes | External outcome was not stored. Absence of a response is the evidence for entering this state |
| `MANUAL_REVIEW_REQUIRED` | Yes | Stored contradiction. A reviewer path is not implemented |
| `FAILED_RETRYABLE` | No | Terminal for that operation id in the domain table. A later new operation id can be planned |
| `FAILED_FINAL` | No | Terminal for that operation id |

Allowed transitions encoded in `signingOperationTransitions`:

| From | To | Evidence value the function requires |
| --- | --- | --- |
| `PREPARED` | `RESULT_UNKNOWN` | `EXTERNAL_RESPONSE_ABSENT` |
| `RESULT_UNKNOWN` | `MANUAL_REVIEW_REQUIRED` | `CONTRADICTION` |
| `RESULT_UNKNOWN` | `FAILED_RETRYABLE` | `PROVIDER_REJECTION_CONFIRMED` |
| `RESULT_UNKNOWN` | `FAILED_FINAL` | `PROVIDER_REJECTION_CONFIRMED` |

`PlanLocalSignatureRecord` stays on `PREPARED`, sets `LOCAL_ATTEMPT_RECORDED`, and writes once. A second call with the same identity returns the stored marker and requests no write. There is no transition from `PREPARED` to a failure state. There is no transition out of `MANUAL_REVIEW_REQUIRED`. There is no transition into a success state. Reaching `FAILED_*` with empty evidence or `EXTERNAL_RESPONSE_ABSENT` returns a conflict and leaves the state unchanged.

`PlanPrepare` order, read from PR #397 `signing_operation.go`:

1. `ValidateSigningOperationIdentity`
2. `validateDocumentObservation`
3. `observationMatches` of the presented observation against the presented identity
4. If the same `operation_id` is already in the supplied slice, `matchSigningOperationIdentity`; a match returns that stored operation and requests no write
5. If another row for the same tenant and document is in a blocking state, conflict and no write
6. Otherwise the plan is a new `PREPARED` row and one write

The existing unit tests cover identical replay, identity mismatches, a second active operation, observation mismatch when no row is supplied, cross-tenant not-found, and the refusal to treat a missing external response as failure. They do not include a case that calls `PlanPrepare` with the same operation id and the same stored identity while the document observation version or hash differs from that identity. Because step 3 runs before step 4, that call returns the observation conflict and does not return the stored row. This discovery does not call that order a contract violation. The required replay contract for a changed document, and whether step 3 should run only when no stored operation exists, is an open question for independent review of PR #397.

The in-memory blocking check is not a PostgreSQL row lock. `FAILED_RETRYABLE` and `FAILED_FINAL` do not hold the scope in the domain function. A persistence stage still must not attach old signature rows to a new operation id.

## 3. Proposed persistence schema

Proposed only. Not a migration. The next implementation re-reads the migration head before choosing a number. Head observed here is `000105`.

Two tables in schema `documents`, owned by document-service:

`documents.signing_operations`

| Column | Rule |
| --- | --- |
| `operation_id` | UUID primary key. Generated by the server for a new attempt |
| `tenant_id` | Required. Part of every read and write predicate |
| `document_id` | Required. Same tenant as the locked document row |
| `idempotency_key` | Required. Caller key, unique per tenant |
| `signer_user_id` | Required |
| `signer_company_id` | Required |
| `expected_document_version` | Integer `>= 1`. Copied from the locked document at insert |
| `content_sha256` | 64 lowercase hex characters. Server observation at insert |
| `state` | One of the five technical states. No `SIGNED`, `ACCEPTED`, or `CONFIRMED` value |
| `evidence_kind` | One of the domain evidence values, or empty on a fresh `PREPARED` row |
| `evidence_source` | `SERVER`, `AUTHENTICATED_PROVIDER`, or `REVIEWER` |
| `evidence_reference` | Provider message id or review id. Empty until such a record exists. No private key, token, or certificate private material |
| `local_attempt` | Integer `0` or `1` |
| `row_version` | Integer `>= 1`. Optimistic lock, incremented on every state or evidence change |
| `created_at`, `updated_at` | `timestamptz` |

`documents.signing_operation_events`

Append-only. Columns: event id, `tenant_id`, `operation_id`, `row_version`, actor kind (`SERVICE` or `REVIEWER`), actor id, previous state, next state, evidence kind, evidence source, reason code, correlation id, `created_at`. No update column. Application code inserts and does not update or delete event payloads. Whether a database role already enforces that was not queried. `NOT_VERIFIED`.

Unique constraints:

- Primary key `operation_id`.
- Unique `(tenant_id, idempotency_key)`.
- Partial unique index on `(tenant_id, document_id)` where `state` is `PREPARED`, `RESULT_UNKNOWN`, or `MANUAL_REVIEW_REQUIRED`.

Checks keep `state` and `evidence_source` inside the lists above and reject any other document status being stored in `state`.

The operation row does not replace `documents.signing_sessions` or `documents.signatures`. It does not change `documents.documents.document_status`.

Retention for the first persistence stage is keep-forever inside the database. No purge job. Legal retention stays with ADR-EDO-007 and is not decided here. Recovery does not delete signature rows or operation rows.

## 4. Transaction and locking model

Lock order matches the current signing repository: the document row first, then operation rows.

```text
BEGIN
  SELECT document_status, version
  FROM documents.documents
  WHERE id = $document AND tenant_id = $tenant AND deleted_at IS NULL
  FOR UPDATE

  SELECT ... FROM documents.signing_operations
  WHERE tenant_id = $tenant AND document_id = $document
  ORDER BY operation_id
  FOR UPDATE
COMMIT or ROLLBACK
```

A provider socket call, if a later stage is authorized to make one, stays outside this transaction. The transaction that records `RESULT_UNKNOWN` commits before that call.

Row locking:

- `FOR UPDATE` on the document serializes prepare, local-attempt recording, and recovery for that tenant document with `CreateSession`, `AddSignature`, and `RecoverExpiredSigning`.
- `row_version` compare-and-set is the stale-writer check: `UPDATE ... WHERE operation_id = $id AND tenant_id = $tenant AND row_version = $expected`. Zero rows is a conflict and the transaction rolls back.
- The partial unique index is the backstop when two transactions pass the in-memory-style check. The loser maps to a conflict and inserts nothing.

Tenant-scoped reads use `operation_id` plus `tenant_id`. A missing row and a row whose tenant differs both surface as not-found with the same error. The handler does not fall back to `GetSessionByID`-style lookup without a tenant predicate.

Conflict detection inside the transaction:

- Presented identity differs from the stored identity for that operation id: conflict, no update.
- Presented observation differs from the locked document version or from the server content hash, and no operation row exists yet: conflict, no insert.
- A blocking row already exists for another operation id: conflict, no insert.
- `row_version` mismatch: conflict, no update.
- Unique violation on the partial index: conflict, no insert.

Crash before commit leaves the previous committed rows. The in-memory plan is discarded.

## 5. Replay and idempotency matrix

This is the proposed persistence contract. It is the contract a later repository must implement. It is not a verdict that PR #397 already fails a requirement.

| Scenario | Stored row | Proposed result | Writes |
| --- | --- | --- | --- |
| Same idempotency key, same operation identity, same server observation | Yes | Return the stored operation, including a non-`PREPARED` state | 0 |
| Same operation id and same identity, document version or content hash now differs | Yes | Return the stored operation. Do not rewrite `expected_document_version` or `content_sha256`. Do not start another external attempt | 0 |
| Same idempotency key or same operation id, same tenant, different document, version, hash, or signer | Yes | Conflict. Do not return the stored operation as a successful replay | 0 |
| Same operation id, other tenant | Yes, for the other tenant | Not-found, same as a missing id | 0 |
| No row, observation matches the locked document, scope free | No | Insert `PREPARED` | 1 |
| No row, observation version or hash differs from the locked document or the server hash | No | Conflict | 0 |
| Two concurrent prepares for one tenant document | Race | One insert. The other conflict from the partial unique index | 1 total |
| Repeat after process restart | Whatever committed | Read the row. Do not sign during startup. A `RESULT_UNKNOWN` row is not a new external call | 0 |

The server generates `operation_id` when the idempotency key is first seen. The key is stored with a hash of the identity fields so a reused key with a different body conflicts. The content hash is one identity field. It is not the operation id by itself.

A new signing attempt after `FAILED_RETRYABLE` or `FAILED_FINAL` uses a new operation id and a new idempotency key, and it rechecks authorization. The old session and old signature rows stay on their original ids.

Question for Agent J on PR #397: `PlanPrepare` compares the presented observation with the presented identity before it looks up an existing operation. A persistence implementation that returns a stored row when the live document has moved would therefore differ from that current function order. Review should decide which order is binding before a repository copies the function literally.

## 6. Trusted evidence model

`PROVIDER_REJECTION_CONFIRMED` in the domain function is a classification argument. Stored by itself, that string does not prove that a provider sent an authentic rejection.

| Evidence | Who may record it | What it authorizes |
| --- | --- | --- |
| `LOCAL_ATTEMPT_RECORDED` | Server, in the same transaction as the local bookkeeping update | One local marker on `PREPARED`. It does not set document `SIGNED` and it does not set cryptographic verification |
| `EXTERNAL_RESPONSE_ABSENT` | Server, after it has committed the intention to treat the outcome as unknown, and before or when it loses the response | `PREPARED` to `RESULT_UNKNOWN` only. It does not authorize `FAILED_RETRYABLE`, `FAILED_FINAL`, or a success status |
| Authenticated provider acceptance | Adapter after it verifies the provider transport or callback authenticity and stores the provider reference | Not mapped to any of the five states as success. A later stage needs an explicit decision before any document status changes. This discovery does not add `CONFIRMED` |
| Authenticated provider rejection | Same adapter, with `evidence_source = AUTHENTICATED_PROVIDER` and a non-empty `evidence_reference` | `RESULT_UNKNOWN` to `FAILED_RETRYABLE` or `FAILED_FINAL` |
| `CONTRADICTION` | Server when two authenticated records disagree, or a reviewer when the records are incomplete | `RESULT_UNKNOWN` to `MANUAL_REVIEW_REQUIRED` |
| Failed signature check | Server verifier result. The current `AddSignature` write of `VALID` is not this result | Does not set `verification_status` to `VALID`, does not set document `SIGNED`, and does not by itself move `RESULT_UNKNOWN` to `FAILED_FINAL`. The safe holding state is `MANUAL_REVIEW_REQUIRED` until a verifier stage defines a narrower code |
| Client-supplied evidence string | Caller body or header | Stored as a claim only. It cannot set `evidence_source` to `AUTHENTICATED_PROVIDER` or `REVIEWER` |

Absence of a response is never converted into success and never converted into `FAILED_RETRYABLE` or `FAILED_FINAL`. A timeout and a lost response both remain `RESULT_UNKNOWN`.

A repeated callback with the same provider reference replays the stored event and inserts no second signature. A callback whose tenant, operation id, document version, or content hash does not match is an audit insert at most: if the operation is `RESULT_UNKNOWN` and the mismatch is authenticated, the state may move to `MANUAL_REVIEW_REQUIRED`; otherwise the state stays. A callback for another operation id does not change this operation.

## 7. Tenant and signer authorization

Reads and writes include the trusted tenant. The trusted tenant for any future HTTP surface has to be the tenant the gateway established. A body `tenant_id` that disagrees is rejected. This discovery does not change `trustedTenantID`, and it does not treat `X-Tenant-ID` on a direct document-service listener as a completed authentication design. Draft PR #360 remains the open caller-authentication contract.

Signer authorization for a new operation:

- The signer user id is the authenticated subject.
- The signer company holds a future `document.sign` grant for that company.
- An active membership without that grant is not sufficient under the policy recorded as Policy B in Draft PR #371.
- PR #371 still marks every `document.sign` cell `SEPARATE_AUTHORITY_REQUIRED`. No role grant is proposed here.
- `PLATFORM_ADMIN` reaching `RequireOperatorShipmentRead` is not authority to sign as a party.

Creating a session and adding a signature today remain on their current routes and their current existence checks. This document does not tighten or loosen those routes.

A reviewer of `MANUAL_REVIEW_REQUIRED` uses a separate reviewer permission. The reviewer does not apply the party's key and does not receive `document.sign`. That permission is not named and not seeded here.

Public recovery stays closed:

- `POST /v1/documents/{id}/signing-recovery` in document-service remains `RejectSigningRecovery`.
- `POST /api/v1/documents/{id}/signing-recovery` in the gateway remains `RejectSigningRecovery`.
- No new route is registered.
- No background worker, timer, or operator client is added.
- Object storage and `documents.document_files` are unchanged.
- No new legal document status is added.

A future read of an operation, if a later stage is authorized, is a tenant-scoped GET that returns not-found for both a missing id and a foreign tenant. A future prepare command stays behind the same closed decision until controller authorization names the route. This document does not specify that route as open.

## 8. Crash recovery

Startup reads committed rows. It does not insert a signature, does not call a provider, and does not move `RESULT_UNKNOWN` to a failure or success state.

| Case | Committed state after the crash | Next allowed action |
| --- | --- | --- |
| Crash after `PREPARED` commit, before any local marker | `PREPARED`, `local_attempt = 0` | Replay returns that row. One local marker may still be recorded in a later authorized transaction. No external call |
| Crash after the local marker and any local signature insert commit together | `PREPARED`, `local_attempt = 1`, signature row present | Replay returns the marker. No second local insert |
| Crash after the transaction rolled back | Previous document and session rows | No operation row. A later prepare is a new decision under the current lock |
| Operator timeout | `RESULT_UNKNOWN`, `EXTERNAL_RESPONSE_ABSENT`, `evidence_source = SERVER` | No second provider call. Wait for authenticated reconciliation or review |
| Lost response after a call may have been accepted | Same `RESULT_UNKNOWN` | No second provider call. Do not infer rejection |
| Repeated callback with the same reference | Unchanged operation | Append nothing new when the stored event already matches. No second signature |
| Callback that contradicts the stored hash or version | `MANUAL_REVIEW_REQUIRED` when the callback is authenticated and the row is `RESULT_UNKNOWN`; otherwise unchanged | No document status change |
| Document version differs from `expected_document_version` | Stored operation unchanged | Do not sign the new bytes under this operation id |
| Cross-tenant read or callback | No change to the victim row | Not-found |
| Replay with a different signer, hash, or version in the identity | Stored operation unchanged | Conflict |
| Reviewer action and callback at the same time | One `row_version` winner | The loser rolls back. A callback does not leave `MANUAL_REVIEW_REQUIRED`, because no such transition exists. The reviewer transition itself is not defined in this stage |

If the local signature insert and the operation marker cannot share one transaction, the persistence stage stops and reports that split rather than writing one of them alone.

## 9. Negative test matrix

For a later authorized persistence stage. Every row here is `NOT_RUN`. The tests use an isolated database or a repository fake that counts commits. They do not call an operator, and they do not use a private key.

| Id | Case | Expected |
| --- | --- | --- |
| N1 | Crash before prepare commit | No operation row |
| N2 | Replay of the same key and identity | Same `operation_id`, commit count unchanged |
| N3 | Same key, different signer or hash | Conflict, no update |
| N4 | Same operation id, other tenant | Not-found identical to a missing id, no update |
| N5 | Two concurrent prepares | One row, one conflict |
| N6 | Observation version mismatch before insert | Conflict, no row |
| N7 | Stored operation, live document version changed | Stored version and hash unchanged, no new operation while the state blocks the scope |
| N8 | `EXTERNAL_RESPONSE_ABSENT` to `FAILED_RETRYABLE` and to `FAILED_FINAL` | Conflict, state stays `RESULT_UNKNOWN` |
| N9 | Client body field `PROVIDER_REJECTION_CONFIRMED` | Rejected as authenticated evidence, no failure transition |
| N10 | Authenticated rejection with provider reference | One transition to the requested failure state, one event |
| N11 | Second local attempt | Replay, no second signature insert |
| N12 | Restart with `RESULT_UNKNOWN` | No signature insert and no provider call |
| N13 | Callback for another operation id | This operation unchanged |
| N14 | Reviewer and callback race | One commit, the other conflict, document status unchanged |
| N15 | Direct call toward the public recovery route | 403, `RecoverExpiredSigning` not used |

`TestDecideSigningRecovery` stays as the regression for the existing in-process session recovery and is not a substitute for N1–N15.

## 10. Dependency on PR #397

PR #397 is the domain source for the five states, the evidence names, the blocking scope, and the pure transition table. It is a draft. Its base `3f306fc407aeda4bdc652d825954292f4a54fad6` is not current `origin/main`. The signing paths compared in section 1 had an empty diff between those SHAs. This pass did not merge #397 and did not re-run its tests.

Before a repository copies `PlanPrepare`, Agent J reviews:

- the observation-match step that runs before the existing-operation lookup
- the fact that `PROVIDER_REJECTION_CONFIRMED` is an argument, not authenticated provider proof
- the decision that `FAILED_RETRYABLE` releases the tenant-document scope

This discovery does not edit #397, #360, #369, #371, or #373.

## 11. Proposed implementation stages

Each stage needs its own controller authorization.

| Stage | Scope | Still closed |
| --- | --- | --- |
| 0.2A, this document | Persistence and authorization design | Migration, HTTP, provider, grants |
| 0.3, only after review of this file and of the PR #397 question | One new migration after a fresh head check, repository, append-only events, isolated database tests N1–N8 and N11 | Provider, new routes, `document.sign` grants, opening recovery |
| 0.4 | Mock adapter that can return accepted, rejected, timeout, and contradiction. Timeout stays `RESULT_UNKNOWN` | Real operator network |
| Later | Named provider, legal signature decision, reviewer permission, any public route | Production and unattended enablement |

0.3 does not set `documents.signatures.verification_status` to `VALID` and does not change `document_status` to `SIGNED`.

## 12. Go / No-Go

No-Go for persistence implementation, a migration, a new route, a permission change, provider traffic, real signing, staging, and production.

Go for independent architecture review of this document.

Open decisions that block 0.3 even after a favorable review:

- Binding replay order for a stored operation when the live document version or hash has changed.
- Which stored bytes the server hashes, given `checksum_sha256` is not established as that canonical range.
- Caller authentication from PR #360 and the ungranted `document.sign` matrix from PR #371.
- Reviewer permission name.
- Operator selection and callback authenticity.
- Legal retention period.

```text
STAGE=EDO-SIGNING-RECOVERY-0.2A-RUNTIME-PERSISTENCE-DISCOVERY
BASE_SHA=a9ef8f40bf0e7f75cb4aee3794e7841723347e4c
RELATED_PR=397
RELATED_HEAD=7fe59bf090e9ab53c8eaaf833eff5bcdc014ca80
MIGRATION_CREATED=NO
HTTP_RECOVERY=CLOSED
DOCUMENT_SIGN_GRANTED=NO
PROVIDER_CALLS=NO
IMPLEMENTATION_GO=NO
ARCHITECTURE_REVIEW=REQUESTED
```
