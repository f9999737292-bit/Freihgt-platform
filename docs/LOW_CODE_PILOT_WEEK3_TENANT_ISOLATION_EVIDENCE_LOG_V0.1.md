# Low-code Pilot Week-3 Tenant Isolation Evidence Log v0.1

## Summary

Evidence log for tenant isolation (PR-GAP-006). Initial entries from **read-only source inspection**; **final review pending**.

**Decision:** **TENANT_ISOLATION_EVIDENCE_LOG_CREATED_PENDING_REVIEW**

## Evidence Collection Status

| Metric | Value |
|--------|-------|
| Collection method | read-only source inspection |
| Runtime GET checks | **not executed** in this pack |
| Write operations | **none** |
| Secrets captured | **no** |
| Production data captured | **no** |
| Final review | **pending** |

## Evidence Entries

### TENANT-EVIDENCE-001

- **area:** runtime active templates
- **method:** source inspection
- **environment:** local/source
- **status:** **PASS**
- **evidence:** `services/low-code-service/internal/repository/form_template_repository.go` — active template queries filter `tenant_id`; publish validates template tenant matches request tenant
- **sensitive_data_captured:** no
- **secrets_captured:** no
- **production_data_captured:** no
- **notes:** Runtime negative cross-tenant GET not run in this pack

### TENANT-EVIDENCE-002

- **area:** custom field values
- **method:** source inspection
- **environment:** local/source
- **status:** **PASS**
- **evidence:** `custom_field_value_service_preview_test.go` — `TenantMismatch` when source/target tenant differs; repository uses tenant predicates
- **sensitive_data_captured:** no
- **secrets_captured:** no
- **production_data_captured:** no
- **notes:** Runtime GET matrix pending optional follow-up

### TENANT-EVIDENCE-003

- **area:** audit events
- **method:** source inspection
- **environment:** local/source
- **status:** **PASS**
- **evidence:** `audit_service.go` — returns `TenantRequired` when filter tenant nil; aligns with audit retention policy tenant-scoping
- **sensitive_data_captured:** no
- **secrets_captured:** no
- **production_data_captured:** no
- **notes:** Audit handler routes reviewed; no audit payload logged

### TENANT-EVIDENCE-004

- **area:** admin templates
- **method:** source inspection
- **environment:** local/source
- **status:** **PASS**
- **evidence:** `admin_form_template_repository.go` — list/detail/update/delete/publish SQL includes `WHERE tenant_id = $1`; clone/import paths tenant-scoped
- **sensitive_data_captured:** no
- **secrets_captured:** no
- **production_data_captured:** no
- **notes:** Admin auth middleware uses `HeaderTenantID` in tests

### TENANT-EVIDENCE-005

- **area:** migration preview
- **method:** source inspection
- **environment:** local/source
- **status:** **PASS**
- **evidence:** Migration preview service tests validate tenant on source/target templates
- **sensitive_data_captured:** no
- **secrets_captured:** no
- **production_data_captured:** no
- **notes:** Preview-only; no POST executed

### TENANT-EVIDENCE-006

- **area:** migration execute docs/source review
- **method:** source inspection
- **environment:** docs/source
- **status:** **PASS**
- **evidence:** `admin_custom_field_value_handler_edge_cases_test.go` — `TestAdminMigrateToActiveTenantRequired`; execute path not invoked in pack
- **sensitive_data_captured:** no
- **secrets_captured:** no
- **production_data_captured:** no
- **notes:** Execute remains blocked in pilot without separate approval

### TENANT-EVIDENCE-007

- **area:** batch migration docs/source review
- **method:** source inspection
- **environment:** docs/source
- **status:** **PENDING**
- **evidence:** Batch handler test files present; formal tenant isolation entry not completed
- **sensitive_data_captured:** no
- **secrets_captured:** no
- **production_data_captured:** no
- **notes:** Review in Evidence Review Pack v0.1

## Blockers

| # | Blocker | Impact |
|---|---------|--------|
| 1 | Final security review not completed | PR-GAP-006 remains open |
| 2 | Cross-tenant negative runtime matrix not run | TC-TENANT-001–002 pending optional GET |
| 3 | `tenant.go` query param fallback | Review whether client-only tenant source is acceptable |

## Risks

| Risk | Severity | Mitigation |
|------|----------|------------|
| Tenant ID from query param fallback | P2 | Review in Evidence Review Pack; prefer header-only in production |
| No staging cross-tenant test | P2 | Remote staging repeat + tenant matrix when staging available |
| Batch migration tenant rules incomplete in log | P3 | Complete TENANT-EVIDENCE-007 in review pack |

## Decision

**TENANT_ISOLATION_EVIDENCE_LOG_CREATED_PENDING_REVIEW**

## Next Steps

1. **Tenant Isolation Evidence Review Pack v0.1**
2. Optional sanitized two-tenant local GET matrix
3. Do **not** close PR-GAP-006 until review sign-off

Reference: `LOW_CODE_PILOT_WEEK3_TENANT_ISOLATION_DECISION_NOTE_V0.1.md`
