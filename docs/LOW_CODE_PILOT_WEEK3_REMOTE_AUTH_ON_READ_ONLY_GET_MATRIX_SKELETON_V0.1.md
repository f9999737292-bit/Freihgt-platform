# Low-code Pilot Week-3 Remote Auth-On Read-only GET Matrix Skeleton v0.1

## Summary

This matrix will be executed only after sanitized staging server details are provided and explicit approval is given.

## Preconditions

- Staging server provisioned
- Staging domain/subdomain available
- HTTPS configured or explicitly noted as pending
- `LOW_CODE_ADMIN_AUTH_ENABLED=true`
- No secrets in docs
- Explicit approval to execute remote read-only GET checks

## Matrix

| ID | Check | Method | Endpoint Type | Expected | Evidence | Status |
|----|-------|--------|---------------|----------|----------|--------|
| RAG-001 | API gateway health | GET | health | 200/healthy | sanitized | PENDING |
| RAG-002 | Low-code service reachable via gateway | GET | runtime | reachable | sanitized | PENDING |
| RAG-003 | Admin route without token | GET | admin | 401/403 | sanitized | PENDING |
| RAG-004 | Admin route with non-admin | GET | admin | 403 | sanitized | PENDING |
| RAG-005 | Admin route with admin | GET | admin | 200 or expected admin response | sanitized | PENDING |
| RAG-006 | Runtime GET compatibility | GET | runtime | 200 or expected response | sanitized | PENDING |
| RAG-007 | No write operations | review | safety | yes | sanitized | PENDING |
| RAG-008 | No secrets captured | review | safety | yes | sanitized | PENDING |

## Forbidden During Execution

- POST
- PUT
- PATCH
- DELETE
- DB changes
- Migration execute
- Template publish/import/clone
- Production data use
- Storing tokens in docs
- Storing `.env` values in docs

## Decision

```text
REMOTE_AUTH_ON_GET_MATRIX_SKELETON_CREATED_PENDING_SERVER_DETAILS
```

Reference: `LOW_CODE_PILOT_WEEK3_STAGING_DETAILS_SANITIZED_INTAKE_TEMPLATE_V0.1.md`
