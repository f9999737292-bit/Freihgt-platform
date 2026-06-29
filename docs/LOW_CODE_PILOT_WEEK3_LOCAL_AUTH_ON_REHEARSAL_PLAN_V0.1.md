# Low-code Pilot Week-3 Local Auth-On Rehearsal Plan v0.1

## Summary

This document defines a local-only rehearsal for auth-on behavior.

Important:
Local rehearsal is useful preparation, but it is **not** remote staging evidence and does **not** close PR-GAP-001.

## Preconditions

- Local project root: `D:\Projects\freight-platform`
- No remote server required
- No production data
- No secrets in docs
- `LOW_CODE_ADMIN_AUTH_ENABLED=true` may be used locally only if already supported by local env setup

## Allowed Local Checks

- Local health check
- Local gateway reachability
- Local runtime GET compatibility check
- Local admin route unauthorized check
- Local non-admin forbidden check if local test token exists outside docs
- Local admin allowed check if local test token exists outside docs

## Forbidden

- Production writes
- Staging writes
- Remote SSH
- Remote deploy
- Storing JWT/tokens/passwords in docs
- Storing `.env` values in docs
- POST/PUT/PATCH/DELETE
- Migration execute
- Template publish/import/clone unless explicitly authorized later

## Local Evidence Rule

Record only sanitized:

- Command name
- Endpoint path
- HTTP status
- Timestamp
- Pass/fail
- No payload dumps
- No tokens

## Decision

```text
LOCAL_AUTH_ON_REHEARSAL_PLAN_CREATED_NOT_REMOTE_EVIDENCE
```

## PR-GAP-001 Status

```text
BLOCKED_WAITING_FOR_STAGING_SERVER_DETAILS
```

Reference: `LOW_CODE_PILOT_WEEK3_PR_GAP_001_NO_SERVER_CONTINUATION_STATUS_V0.1.md`
