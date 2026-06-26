# Low-code Pilot Week-3 Tenant Isolation Read-only Test Plan v0.1

## Summary

Read-only test plan for tenant isolation verification (PR-GAP-006). **No writes** in this pack.

**Decision:** **TENANT_ISOLATION_READ_ONLY_TEST_PLAN_CREATED**

## Purpose

Define allowed read-only verification methods and test cases for tenant isolation without changing code or executing production/staging writes.

## Test Mode

**Read-only** — source inspection, docs inspection, optional local/dev GET only.

## Environments

| Environment | Use in this pack |
|-------------|------------------|
| Source tree | **yes** |
| Docs | **yes** |
| Local/dev | **optional** — sanitized GET only |
| Staging | **no** |
| Production | **no** |

## Allowed Operations

- Local source inspection
- Docs inspection
- Read-only GET checks in local/dev only
- Sanitized response code evidence
- Sanitized request ID evidence
- Sanitized endpoint evidence (no secrets in query/body)

## Forbidden Operations

- POST
- PUT
- PATCH
- DELETE
- Publish
- Import
- Migrate-to-active
- Batch execute
- Manual DB edits
- Production/staging writes
- Storing JWT/tokens/passwords in docs

## Test Cases

| ID | Test case | Method | Expected |
|----|-----------|--------|----------|
| TC-TENANT-001 | Active template cannot leak another tenant template | Source + optional local GET | Tenant A sees only tenant A active template |
| TC-TENANT-002 | Custom field values cannot leak another tenant entity values | Source + optional local GET | Values scoped to tenant + entity |
| TC-TENANT-003 | Audit events are tenant-bound | Source + optional local GET | Audit filter requires tenant; no cross-tenant rows |
| TC-TENANT-004 | Admin template list is tenant-bound | Source + optional local GET | Admin list filtered by tenant |
| TC-TENANT-005 | Migration preview is tenant-bound | Source inspection | Preview rejects tenant mismatch |
| TC-TENANT-006 | Migration execute path requires tenant-bound ownership | Source/docs only | Handler/service enforce tenant; no execute in pack |
| TC-TENANT-007 | Batch migration path requires tenant-bound ownership | Source/docs only | Batch handlers tenant-scoped; review pending |
| TC-TENANT-008 | Evidence redaction is applied | Docs review | No secrets/JWT/tokens/raw prod data in evidence |

## Evidence Format

| Field | Allowed |
|-------|---------|
| HTTP status code | yes |
| Request ID | yes |
| Endpoint path | yes (no secrets in query) |
| Tenant ID | sanitized/dev only |
| Response body | **no** raw payloads in docs |
| JWT/tokens | **never** |

## Decision

**TENANT_ISOLATION_READ_ONLY_TEST_PLAN_CREATED**

Reference: `LOW_CODE_PILOT_WEEK3_TENANT_ISOLATION_EVIDENCE_LOG_V0.1.md`
