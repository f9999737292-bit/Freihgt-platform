# Low-code Pilot Week-3 Remote Auth-On Staging Repeat Plan v0.1

## Summary

Read-only verification plan for repeating local auth-on matrix on **remote staging** (PR-GAP-001). Prepared during preparation gate — **execution pending** staging details and explicit approval.

**Decision:** **REMOTE_AUTH_ON_STAGING_REPEAT_PLAN_CREATED_PENDING_EXECUTION**

Reference: `LOW_CODE_PILOT_WEEK3_REMOTE_STAGING_AUTH_ON_TEST_MATRIX_V0.1.md`, `LOW_CODE_PILOT_WEEK3_REMOTE_AUTH_ON_STAGING_PREREQUISITES_V0.1.md`

## Purpose

Define allowed read-only GET checks, forbidden write operations, evidence format, and stop conditions for Remote Auth-On Staging Repeat Pack v0.1.

## Preconditions

1. `LOW_CODE_PILOT_WEEK3_REMOTE_STAGING_DETAILS_VALIDATION_NOTE_V0.1.md` — decision **REMOTE_STAGING_DETAILS_VALIDATED_READY_FOR_AUTH_ON_REPEAT**
2. Staging web-admin URL and API gateway URL recorded (no credentials in docs)
3. `LOW_CODE_ADMIN_AUTH_ENABLED=true` on staging low-code-service
4. Admin and non-admin test users available via **secure channel** (not in repo)
5. Pilot tenant UUID available (UUID only in evidence)
6. Explicit approval for remote read-only GET execution
7. No SSH/deploy required for GET matrix if URLs are reachable from QA runner

## Allowed Checks

- Read-only GET health checks
- Read-only GET low-code active template checks
- Read-only GET audit events checks if endpoint is available and safe
- Admin vs non-admin access behavior check only if tokens are available **outside docs**
- Sanitized response code recording
- Sanitized endpoint recording
- Sanitized timestamp recording

## Forbidden Checks

- POST, PUT, PATCH, DELETE
- Template publish
- Import / export execute
- Migration execute / batch execute
- Manual DB edits
- Storing JWT/tokens/passwords in docs
- Storing `.env` values in docs
- Raw production/staging payload dumps

## Verification Matrix

| Check | Method | Expected | Evidence | Status |
|-------|--------|----------|----------|--------|
| API gateway health | GET | 200/healthy | sanitized | **PENDING** |
| low-code service reachable via gateway | GET | reachable | sanitized | **PENDING** |
| admin auth-on required for admin route | GET | 401/403 without admin | sanitized | **PENDING** |
| non-admin forbidden for admin route | GET | 403 | sanitized | **PENDING** |
| admin allowed for admin route | GET | 200 or expected admin response | sanitized | **PENDING** |
| runtime GET compatibility | GET | 200 or expected response | sanitized | **PENDING** |
| no secrets captured | docs review | yes | sanitized | **PASS** |
| no write operations | docs review | yes | sanitized | **PASS** |

## Evidence Format

Record per check:

- timestamp (UTC)
- endpoint path (no query secrets)
- HTTP status code
- pass/fail
- notes (sanitized — no headers with tokens, no response bodies with secrets)

Store evidence in repeat pack execution doc only — not in this plan.

## Stop Conditions

- Any unexpected write required
- Credentials would need to be pasted into repo/docs
- Staging returns 5xx across health checks
- Security owner requests STOP
- PR-GAP-001 closure attempted without full matrix PASS — **do not close gap**

## Decision

**REMOTE_AUTH_ON_STAGING_REPEAT_PLAN_CREATED_PENDING_EXECUTION**

Execution blocked 2026-06-23 — see `LOW_CODE_PILOT_WEEK3_REMOTE_AUTH_ON_STAGING_REPEAT_V0.1.md`.

**Decision:** **REMOTE_AUTH_ON_STAGING_REPEAT_PLAN_EXECUTION_BLOCKED**

## Next Pack

**Low-code Pilot Week-3 Remote Auth-On Staging Repeat Pack v0.1**
