# EDO signing recovery 0.2 — policy contract freeze

Stage: `EDO-SIGNING-RECOVERY-0.2-R3-POLICY-CONTRACT-FREEZE`

Mode: documentation. This file records the controller decision for `J-EDO02A-003`. It does not add a state, a transition, a migration, a route, a permission grant, a provider client, or a signature.

This document is not an Agent J closure. `J-EDO02A-003` stays open until Agent J reviews this contract.

## Baseline

| Ref | SHA | Role |
| --- | --- | --- |
| PR #397 head | `a8adb02ec3e39d558bbb7e80dae4efee2ba59faf` | Domain behavior this freeze describes |
| PR #397 branch | `feat/edo-signing-recovery-domain-v0.2` | Draft. Files before this document: `signing_operation.go`, `signing_operation_test.go` |
| PR #397 base | `3f306fc407aeda4bdc652d825954292f4a54fad6` | Older than current `main` |
| PR #401 head | `fe5f61192b1bc62e76e275877f79ccd81976c7ce` | Persistence discovery. Not edited by this freeze |
| `origin/main` | `892361db87985f35132aef86550a025909af648a` | Checked at freeze time. Not the PR #397 base |

PR #397 CI run `44939415732` on `a8adb02ec3e39d558bbb7e80dae4efee2ba59faf` completed with conclusion `success` after one failed-job rerun. The first attempt failed in `execution-tracking-pod-integration` because the runner reported `start postgres: no version found matching 16.4.0`. That job is outside this domain.

GitHub shows no pull-request review, review comment, or issue comment on PR #397. The findings `J-EDO02A-001`, `J-EDO02A-002`, `J-EDO02A-003`, and `J-EDO02A-010` are the controller-supplied Agent J audit. R2 recorded `001` and `002` and `010` as fixed in `a8adb02` and left `003` as `BLOCKED_POLICY_DECISION`. This file is the policy record for that finding. It does not mark the finding closed.

`docs/architecture/edo/` is absent on `a8adb02`. The 0.1 documents under that directory live on another branch and are not edited here. `docs/architecture/edo-signing-recovery-v0.1.md` remains the document-status recovery note: public recovery stays `403`, and internal document status `SIGNED` is not an external legal signature.

## 1. Controller Decision D1–D8

| Id | Decision | Effect on this stage |
| --- | --- | --- |
| D1 | `APPROVED_HYBRID_C_FAIL_CLOSED` | Hybrid C is the contract. Automatic behavior is limited to the technical transitions already implemented, and only with the evidence that code already requires |
| D2 | `NO_PREPARED_CANCELLATION` | No cancellation of `PREPARED` until a later decision names the actor, the evidence, and the destination |
| D3 | `NO_MANUAL_REVIEW_EXIT` | No exit from `MANUAL_REVIEW_REQUIRED` until a later decision names the reviewer authority and the destination |
| D4 | `NO_PROVIDER_ACCEPTED_STATE` | No `CONFIRMED`, `ACCEPTED`, `SIGNED`, or other provider-acceptance state |
| D5 | `DEFERRED` | `FAILED_FINAL` and the tenant+document blocking rule stay as implemented. Section 8 records the consequences of a possible future hold. It does not implement that hold |
| D6 | `PROVIDER_NOT_FOUND_NOT_CANCELLATION` | A provider "not found" after a call may have been sent proves neither cancellation nor rejection |
| D7 | `ONE_BLOCKING_OPERATION_PER_TENANT_DOCUMENT` | One blocking operation for each tenant and document. Signer identity stays inside the operation. Scope is not split per signer |
| D8 | `NO_PROVIDER_STATUS_QUERY` | No automatic or manual provider status-read integration in this stage |

`EXTERNAL_RESPONSE_ABSENT` remains evidence for `PREPARED` → `RESULT_UNKNOWN` only. It is not cancellation, rejection, or acceptance.

## 2. Current state machine

This is the signing-operation state machine in `services/document-service/internal/domain/signing_operation.go` at PR #397 head. It is not the document status machine and it is not the signing-session status machine.

Document statuses (`DRAFT`, `READY_FOR_SIGNING`, `SIGNING_IN_PROGRESS`, `SIGNED`, and the rest) stay on the document row. Session statuses (`CREATED`, `IN_PROGRESS`, `COMPLETED`, `EXPIRED`, `CANCELLED`, `FAILED`) stay on the signing session. Neither list is a value of `SigningOperation.State`.

Internal document status `SIGNED` means the required signer count was reached. It is not operator acceptance and it is not a legally sufficient signature. None of the five operation states below is that document status.

| Operation state | Holds tenant+document scope today | Meaning |
| --- | --- | --- |
| `PREPARED` | Yes | Identity accepted. At most one local bookkeeping marker |
| `RESULT_UNKNOWN` | Yes | The external outcome was not stored |
| `MANUAL_REVIEW_REQUIRED` | Yes | A contradiction was stored. No exit is implemented |
| `FAILED_RETRYABLE` | No | Terminal for this operation id in the current function |
| `FAILED_FINAL` | No | Terminal for this operation id in the current function. D5 does not change this |

The domain package does not persist rows, does not allocate `row_version`, and does not take a PostgreSQL lock. Where a transition below names concurrency, the current mechanism is the in-memory slice passed by the caller. A future repository, if separately authorized, still has to lock the document row before operation rows and compare-and-set a stored version. That repository is not created here. PR #401's proposed tables are not current schema.

Server-owned authority for every current transition is document-service code. A client body cannot choose `evidence_source`. Tenant scope is the trusted tenant already on the operation identity. A foreign tenant is not a source of authority.

### PREPARED → PREPARED with one local marker

| Item | Current behavior |
| --- | --- |
| Function | `PlanLocalSignatureRecord` |
| Evidence kind | `LOCAL_ATTEMPT_RECORDED` |
| Evidence source | `SERVER` |
| Predicates | Presented identity matches the stored identity. Stored state is `PREPARED`. `DocumentObservation` matches stored tenant, document, expected version, and content hash. Marker is not yet recorded |
| Authority | Server-owned snapshot. The function comment requires the observation to come from trusted storage. A client body is not that snapshot |
| Tenant | Compared as part of identity and observation |
| Concurrency | No `row_version`. A second call that finds `Recorded == true` returns the clone and requests no write |
| Side effects | No signature insert, no `verification_status`, no document status change, no provider call |
| Replay | The recorded marker is returned with write `false` |
| Refusal | Identity mismatch returns not-found or conflict and no write. Wrong state, or an observation mismatch, returns the unchanged operation and no write |

### PREPARED → RESULT_UNKNOWN

| Item | Current behavior |
| --- | --- |
| Function | `PlanExternalResultUnknown` |
| Evidence kind | `EXTERNAL_RESPONSE_ABSENT` |
| Evidence source | `SERVER` |
| Evidence reference | Empty |
| Predicates | Identity matches. The only allowed edge is `PREPARED` to `RESULT_UNKNOWN` with this kind |
| Authority | The server records that it does not have the external outcome |
| Tenant | Identity match includes tenant |
| Concurrency | A repeat of the same state and kind returns the clone and requests no write |
| Side effects | No provider call, no signature, no document status change. Scope stays held |
| Replay | Same state and same kind: write `false` |
| Refusal | Any other destination or kind returns conflict and the unchanged operation |

### RESULT_UNKNOWN → MANUAL_REVIEW_REQUIRED

| Item | Current behavior |
| --- | --- |
| Function | `PlanManualReview` |
| Evidence kind | `CONTRADICTION` |
| Evidence source | The function stores the kind and does not assign a source |
| Predicates | Identity matches. The transition table allows this pair only with `CONTRADICTION` |
| Authority | Server-side classification of contradictory evidence. A reviewer permission is not consulted because no reviewer exit exists |
| Tenant | Identity match includes tenant |
| Concurrency | Repeat of the same state and kind: write `false` |
| Side effects | No document status change. Scope stays held. No path out of this state |
| Replay | Same state and same kind: write `false` |
| Refusal | A missing response does not select this edge. Any other evidence returns conflict and the unchanged operation |

### RESULT_UNKNOWN → FAILED_RETRYABLE

| Item | Current behavior |
| --- | --- |
| Function | `PlanFailure` |
| Evidence kind | `PROVIDER_REJECTION_CONFIRMED` |
| Evidence source | `AUTHENTICATED_PROVIDER` |
| Evidence reference | Non-empty after trim |
| Predicates | Target is exactly `FAILED_RETRYABLE`. Identity matches. `authenticatedProviderRejection` accepts only that kind, that source, and a non-empty reference together. The transition table allows the edge only from `RESULT_UNKNOWN` |
| Authority | The source value is a claim the caller supplies to the domain function. The function does not contact a provider and does not prove transport authenticity. Section 3 states the trust rule a future adapter must meet before it is allowed to supply this claim |
| Tenant | Identity match includes tenant |
| Concurrency | The same target plus the same authenticated claim returns the clone and requests no write |
| Side effects | No signature and no document status change. After the transition, `SigningOperationStateBlocksDocument` is false for this state. D5 does not change that |
| Replay | Matching stored failure claim: write `false` |
| Refusal | Empty kind, `EXTERNAL_RESPONSE_ABSENT`, kind without authenticated source and reference, or a target of `FAILED_FINAL` from an existing `FAILED_RETRYABLE` row: conflict, no write |

### RESULT_UNKNOWN → FAILED_FINAL

| Item | Current behavior |
| --- | --- |
| Function | `PlanFailure` |
| Evidence kind | `PROVIDER_REJECTION_CONFIRMED` |
| Evidence source | `AUTHENTICATED_PROVIDER` |
| Evidence reference | Non-empty after trim |
| Predicates | Same authenticity predicate as `FAILED_RETRYABLE`. The target is a different state. The two failure states do not transition into each other |
| Authority | Same claim boundary as the retryable edge. The kind string alone is not proof |
| Tenant | Identity match includes tenant |
| Concurrency | Matching stored claim: write `false` |
| Side effects | No signature and no document status change. The state does not hold tenant+document scope. D5 leaves this unchanged |
| Replay | Matching stored claim: write `false` |
| Refusal | Absence, an unauthenticated claim, or a request to move an existing `FAILED_FINAL` row to `FAILED_RETRYABLE`: conflict, no write |

### Edges that are not in the machine

The transition table contains only the four edges above, plus the in-place local marker on `PREPARED`. The following are absent and stay absent under D2, D3, D4, and D8:

- any destination out of `MANUAL_REVIEW_REQUIRED`
- any cancellation of `PREPARED`
- `PREPARED` to either failure state
- either failure state back to `PREPARED`, `RESULT_UNKNOWN`, or `MANUAL_REVIEW_REQUIRED`
- `FAILED_RETRYABLE` ↔ `FAILED_FINAL`
- a provider-accepted operation state
- a provider status query

## 3. Evidence trust boundary

1. `evidence_kind`, `evidence_source`, and `evidence_reference` do not prove authenticity by themselves.
2. `AUTHENTICATED_PROVIDER` may be assigned only by a trusted server adapter after that adapter has checked the proof independently of the caller. This freeze does not add that adapter.
3. A client request cannot assign `evidence_source`.
4. `EXTERNAL_RESPONSE_ABSENT` does not prove rejection, cancellation, or acceptance.
5. A future authentic callback, if a later stage is authorized to receive one, has to match tenant, operation id, document version, and content hash of the stored operation. A mismatch does not rebind the operation.
6. `RESULT_UNKNOWN`, a lost response, and a provider "not found" after a possible send do not authorize another signing command for the same operation.
7. `DOCUMENT_SIGN_GRANTED=NO`.
8. An operation state is not a legally significant signature.

`LOCAL_ATTEMPT_RECORDED` with source `SERVER` authorizes one local bookkeeping marker on `PREPARED`. It does not set document status `SIGNED` and it does not set cryptographic verification.

D6: a provider response of "not found" after a call may have been sent is not `PROVIDER_REJECTION_CONFIRMED` and it is not cancellation. The operation stays in its stored state unless an already allowed edge applies. `EXTERNAL_RESPONSE_ABSENT` is the loss signal. It is not the "not found" signal rewritten into a failure.

## 4. Allowed and forbidden transitions

| From | To | Verdict under D1–D8 |
| --- | --- | --- |
| `PREPARED` | `PREPARED` with one `LOCAL_ATTEMPT_RECORDED` / `SERVER` marker | Allowed by current code |
| `PREPARED` | `RESULT_UNKNOWN` with `EXTERNAL_RESPONSE_ABSENT` / `SERVER` | Allowed by current code |
| `RESULT_UNKNOWN` | `MANUAL_REVIEW_REQUIRED` with `CONTRADICTION` | Allowed by current code |
| `RESULT_UNKNOWN` | `FAILED_RETRYABLE` with authenticated rejection claim | Allowed by current code |
| `RESULT_UNKNOWN` | `FAILED_FINAL` with authenticated rejection claim | Allowed by current code |
| `PREPARED` | cancellation | Forbidden until a later decision names actor, evidence, and destination |
| `MANUAL_REVIEW_REQUIRED` | any other state | Forbidden until a later decision names reviewer authority and destination |
| any state | `CONFIRMED`, `ACCEPTED`, `SIGNED`, or another acceptance state | Forbidden. Those names are not operation states |
| `PREPARED` or `RESULT_UNKNOWN` | failure or cancellation because the response is absent or the provider says not found | Forbidden |
| `FAILED_RETRYABLE` | `FAILED_FINAL` and the reverse | Forbidden |
| any blocking state | a second blocking operation for the same tenant and document | Forbidden |
| any state | a provider status read | Forbidden in this stage |

A repeat of an already stored allowed edge is replay. It is not a second transition and it requests no write.

## 5. Idempotency and replay

`PlanPrepare` order at PR #397 head:

1. Validate the presented operation identity.
2. Look up the operation id inside the caller's tenant.
3. If that row exists and the full identity matches, return the stored operation with write `false`. A newer live document version or content hash does not rewrite `expected_version` or `content_hash` and does not start another external attempt.
4. If the row exists and the identity differs, return conflict and do not write.
5. If the same operation id exists only for another tenant, return not-found, the same message as a missing id, and do not create a row.
6. If this tenant has no row, validate the server-owned observation and require it to match the presented identity.
7. If another row for this tenant and document is in a blocking state, return conflict and do not write.
8. Otherwise return a new `PREPARED` operation and request one write.

Replay does not create a signature, does not call a provider, and does not update the stored operation. A new attempt after `FAILED_RETRYABLE` or `FAILED_FINAL` is a new operation id. It is not a replay of the old id. D5 does not add a block on that new id.

`PlanLocalSignatureRecord` refuses a new marker when the observation disagrees with the stored identity. The duplicate marker path is read-only.

## 6. Tenant isolation

- Lookup filters by tenant before a missing id is treated as absent.
- A foreign operation id and a missing operation id both surface as `signing operation not found`.
- The foreign lookup does not insert a row for the caller.
- Presented tenant is part of identity match. A mismatch of tenant with the stored row is not-found, not a rewrite.
- This package does not read `X-Tenant-ID` and does not accept a query-parameter tenant.
- A future HTTP surface, if separately authorized, still has to use the gateway-established tenant. That surface is not opened here.
- Public signing recovery stays `RejectSigningRecovery` with expected response `403`.

## 7. Multi-signer scope

D7 keeps one blocking operation for the pair `(tenant_id, document_id)`.

`SigningOperationStateBlocksDocument` is true only for `PREPARED`, `RESULT_UNKNOWN`, and `MANUAL_REVIEW_REQUIRED`. Signer user and signer company belong to the operation identity. A different signer on the same operation id is an identity conflict. A different operation id for the same tenant and document is a conflict while a blocking row exists.

Changing the required signer set does not split the scope and does not rewrite the stored identity. If that change alters the canonical bytes, the stored content hash stays the hash captured for this operation. New bytes are not signed under this operation id. This freeze does not decide a per-signer uniqueness key.

`FAILED_RETRYABLE` and `FAILED_FINAL` do not hold the scope. A later new operation id must not attach old signature rows to itself. The domain function does not move signature rows. There is no signature insert in this package.

## 8. Deferred D5

D5 defers any change to `FAILED_FINAL` and to the blocking rule. Today's machine leaves both failure states outside the blocking set. That is the behavior Agent J has to review. It is not a new release rule.

If a later controller decision ordered `FAILED_FINAL` to hold tenant+document scope, the consequences would be:

- `SigningOperationStateBlocksDocument` would return true for `FAILED_FINAL`.
- `PlanPrepare` would reject another operation id for that tenant and document until a separately approved release.
- A persistence index that encodes the blocking set would have to include `FAILED_FINAL`. No such index is created here.
- Documents could stay blocked after a final rejection until a named actor releases them.
- The current function would then be wrong, because it treats `FAILED_FINAL` as non-blocking. That code change is not part of D5.

The risk of leaving the current rule in place is also recorded: after `FAILED_FINAL`, a new operation id can be planned. That new id still requires its own identity, its own observation match, and its own authorization. It must not reuse the rejected operation's signature rows. D5 does not choose between these outcomes. It forbids implementing the hold in this stage.

`FAILED_RETRYABLE` remains non-blocking as well. D5 does not split the two failure states.

## 9. Production blockers

| Blocker | Current fact |
| --- | --- |
| Operation model is not on `main` | PR #397 is Draft. Base `3f306fc` is behind `origin/main` `892361db` |
| No operation persistence | The domain package is in-memory. PR #401 is a discovery document, not a migration |
| No reviewer authority | Exit from `MANUAL_REVIEW_REQUIRED` is not defined |
| No `PREPARED` cancellation | D2 |
| No provider adapter and no status query | D4 and D8. `PROVIDER_SIGNING_ENABLED` is not granted |
| `document.sign` | The permission code exists in an older seed. This stage does not grant it |
| `AddSignature` | Existing runtime can store `verification_status = VALID` without cryptographic verification. This operation machine does not call that function and does not make that write a provider proof |
| Public recovery | Both recovery routes return `403` and do not call `RecoverExpiredSigning` |
| Legal signature | Operation state, session completion, and internal `SIGNED` are separate from a legally sufficient signature. ADR-EDO-007 retention and the canonical signed byte range stay outside this freeze |
| D5 | Failure states release the in-memory scope. A hold is not implemented |

`EDO_0_3_AUTHORIZED=NO`.

## 10. Future test matrix

These rows are the regression set for a later authorized change. They are not a test run of this documentation stage. Existing domain tests at `a8adb02` already cover replay, observation mismatch, cross-tenant not-found, absence versus failure, and distinct failure states. This table does not claim a new `go test` result.

| Id | Case | Expected while D1–D8 stand |
| --- | --- | --- |
| T1 | Replay same identity after version or hash change | Stored operation, write false |
| T2 | New local marker after version or hash change | Unchanged operation, write false |
| T3 | Duplicate local marker | Write false |
| T4 | `EXTERNAL_RESPONSE_ABSENT` | Only `PREPARED` → `RESULT_UNKNOWN` |
| T5 | Absence, empty claim, or provider "not found" used as failure or cancellation | Conflict, no write |
| T6 | Rejection kind without `AUTHENTICATED_PROVIDER` and a non-empty reference | Conflict, no write |
| T7 | `FAILED_RETRYABLE` to `FAILED_FINAL` and the reverse | Conflict, no write |
| T8 | Any transition out of `MANUAL_REVIEW_REQUIRED` | No allowed edge |
| T9 | Foreign operation id and missing operation id | Same not-found, no insert |
| T10 | Second blocking operation for one tenant and document | Conflict, no insert |
| T11 | Same provider reference repeated | No second signature |
| T12 | Client body sets `AUTHENTICATED_PROVIDER` or `REVIEWER` | Rejected by a future trusted boundary. The domain function does not authenticate the caller |
| T13 | Public recovery route | `403`, `RecoverExpiredSigning` not called |
| T14 | `FAILED_FINAL` blocking bit | Remains false until a decision replaces D5 |

## 11. Remaining controller decisions

D1–D8 are decided for this stage. The following are still open and are not decided by this file:

| Topic | Why it remains open |
| --- | --- |
| Actor, evidence code, and destination for a future `PREPARED` cancellation | D2 forbids inventing them here |
| Reviewer permission name and the only legal destination out of `MANUAL_REVIEW_REQUIRED` | D3. The permission is not seeded |
| A non-legal holding state for authentic provider acceptance | D4 forbids adding it here |
| Whether `FAILED_FINAL` should hold scope | D5 explicitly defers the choice |
| Provider transport authenticity and callback signing rules | No adapter is authorized |
| A status-read integration | D8 |
| Per-signer lock scope | D7 keeps the current pair |
| Canonical bytes covered by a future qualified signature | Not proved by `document_files.checksum_sha256` in the inspected discovery |
| Legal retention | ADR-EDO-007 remains the open record |
| Grant of `document.sign` | Not granted |
| Merge of PR #397 or PR #401 | Not authorized |

## 12. Agent J review checklist

- The five operation states match `signing_operation.go`, and no sixth state appears in this document as an implemented state.
- Operation state is not described as document status `SIGNED` or as a legally sufficient signature.
- Allowed edges match `signingOperationTransitions` plus the single local marker.
- `EXTERNAL_RESPONSE_ABSENT` and provider "not found" are not cancellation or rejection.
- `PlanPrepare` at `a8adb02` looks up the tenant-scoped operation before it checks a live observation. Section 13 records the older order in PR #401. This file does not adopt that older order as current behavior.
- D5 does not change `SigningOperationStateBlocksDocument` for `FAILED_FINAL`.
- D7 does not split scope by signer.
- No migration, table, unique index, route, provider client, or `document.sign` grant is introduced.
- PR #401 is not modified.
- `J-EDO02A-003` is not marked closed.

## 13. PR #397 and PR #401

PR #397 at `a8adb02` is the current domain behavior: replay of a matching identity returns the stored row before live observation is consulted.

PR #401 at `fe5f611` describes persistence for a later stage and still quotes the pre-R2 `PlanPrepare` order from head `7fe59bf`. That discovery is historical for the order question. It is not edited here, and its proposed tables are not created.

| Topic | PR #401 text at `fe5f611` | Current PR #397 behavior at `a8adb02` |
| --- | --- | --- |
| `PlanPrepare` order | Observation match is described before stored-operation lookup | Tenant lookup and identity match run first. Observation is checked only when this tenant has no stored row |
| Replay when version or hash changed | Called an open review question | Matching identity returns the stored row and requests no write |
| Tests for that replay | Discovery says those cases were not in the tests it read | R2 added them on this head |
| Cited PR #397 head | `7fe59bf090e9ab53c8eaaf833eff5bcdc014ca80` | `a8adb02ec3e39d558bbb7e80dae4efee2ba59faf` |
| `FAILED_FINAL` scope | Documents the current non-blocking behavior | Unchanged. D5 does not implement a hold |
| Schema | Proposes `signing_operations`, an event table, and a partial unique index | Not present in this domain commit and not added by this freeze |
| `CONTRADICTION` source | Discovery allows server or reviewer | `PlanManualReview` stores the kind and does not set a source. No reviewer transition exists |

No new table, migration, or unique index is specified for implementation by this document.
