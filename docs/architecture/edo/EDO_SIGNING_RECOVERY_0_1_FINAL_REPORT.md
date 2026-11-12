# EDO signing recovery 0.1 — final report

## Facts

On `origin/main` `3f306fc407aeda4bdc652d825954292f4a54fad6`, document-service implements an internal signing session. Creating a session and adding a signature are single transactions with document-then-session locks. An overdue session becomes `EXPIRED` without a signature. In-process recovery can return the document to `READY_FOR_SIGNING` and does not create a session. HTTP recovery is 403 at document-service and at the gateway. No EDO operator adapter was found. No signing-recovery audit event was found. Tests were not executed.

## Proposed decisions

Use a separate operation record for `PREPARED`, `RESULT_UNKNOWN`, and `MANUAL_REVIEW_REQUIRED`. Do not treat a missing provider response as failure. Do not use the document hash as the only operation id. Do not let a platform administrator sign as a party. Keep the current expiry and in-process recovery rules. Keep HTTP recovery closed until a later authorized stage.

## Risks

Internal `SIGNED` can be mistaken for a legally significant or operator-confirmed signature. A repeated session-signature request has no idempotency key. Direct service calls still accept `X-Tenant-ID` as the tenant input. External failure modes F03, F04, F05, and F13 have nowhere to store a provider result.

## Readiness

The documents are ready for Agent J. This pass does not say that the independent review has started, and it does not say the design is accepted.

```text
AGENT=B
PROJECT=BINTRANS
STAGE=EDO-SIGNING-RECOVERY-0.1
MODE=DISCOVERY_ARCHITECTURE
BASE_SHA=3f306fc407aeda4bdc652d825954292f4a54fad6
FINAL_HEAD=c76d6b09701e1155d3bd02bcc14711e7fbe63ae2
ORIGIN_MAIN=3f306fc407aeda4bdc652d825954292f4a54fad6
BRANCH=docs/edo-signing-recovery-v0.1
WORKTREE=D:/Projects/freight-platform-wt/edo-signing-recovery-docs-v0.1
WORKTREE_CLEAN=YES
SIGNING_WORKFLOW_DISCOVERED=YES
SIGNING_OWNER_IDENTIFIED=document-service
CURRENT_RECOVERY_IMPLEMENTED=PARTIAL_IN_PROCESS_HTTP_403
IDEMPOTENCY_IMPLEMENTED=PARTIAL_ATTACHMENT_ONLY
TENANT_ISOLATION_VERIFIED=NOT_VERIFIED
DOCUMENT_IMMUTABILITY_VERIFIED=NOT_VERIFIED
AUDIT_TRAIL_VERIFIED=NOT_VERIFIED
FAILURE_SCENARIOS_ANALYZED=F01-F15
STATE_MACHINE_DOCUMENTED=YES
RETRY_POLICY_DOCUMENTED=YES
RECONCILIATION_POLICY_DOCUMENTED=YES
SECURITY_MODEL_DOCUMENTED=YES
TEST_MATRIX_COMPLETED=YES
ADR_CREATED=YES
ROADMAP_CREATED=YES
DOCUMENTATION_FILES_CHANGED=docs/architecture/edo/*
PRODUCT_CODE_CHANGED=NO
MIGRATION_CREATED=NO
STAGING_CHANGED=NO
PRODUCTION_CHANGED=NO
EXTERNAL_EDO_CALLS=NO
REAL_SIGNING_PERFORMED=NO
PR_NUMBER=373
PR_STATE=DRAFT
CI_RESULT=NOT_RUN
INDEPENDENT_REVIEW=NOT_STARTED
BLOCKERS=NONE
OPEN_QUESTIONS=canonical signed bytes; signer revocation versus session expiry; reviewer permission; operator selection; unscoped GetSessionByID callers
NEXT_RECOMMENDED_STAGE=EDO-SIGNING-RECOVERY-0.2
FINAL_VERDICT=READY_FOR_INDEPENDENT_REVIEW
```

`FINAL_HEAD` in the block above is the documentation commit. A following commit may record the pull request number. `CI_RESULT` stays `NOT_RUN` until a run for the branch tip is observed. The file cannot contain the hash of the commit that changes this sentence.
