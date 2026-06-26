# Low-code Pilot Week-3 Rollback Checklist v0.1

## Summary

Operational checklist for low-code rollback procedure execution. **Default status: PENDING** until incident or drill.

Reference: `LOW_CODE_PILOT_WEEK3_LOW_CODE_ROLLBACK_PROCEDURE_V0.1.md`

## Checklist

| Item | Status | Owner | Evidence | Notes |
|------|--------|-------|----------|-------|
| Rollback owner assigned | **PENDING** | Tech Lead / Ops | — | See owner note |
| Incident severity confirmed | **PENDING** | Rollback owner | — | P0/P1/P2 |
| Impacted tenant identified | **PENDING** | Ops | — | `X-Tenant-ID` |
| Impacted entity_type identified | **PENDING** | Ops | — | TO/SH/BR/… |
| Impacted template identified | **PENDING** | Ops | — | template_code + version |
| Last known safe template identified | **PENDING** | Ops | — | Published version |
| Risky admin actions paused | **PENDING** | Ops | — | No publish/migrate/import |
| Runtime GET verified | **PENDING** | QA | — | Read-only |
| Admin auth verified | **PENDING** | Security | — | 200/403/401 matrix |
| Audit events verified | **PENDING** | QA | — | GET audit-events |
| Communication sent | **PENDING** | PM | — | Operators notified |
| Resume decision captured | **PENDING** | Rollback owner | — | Resume or continue |
| No secrets committed | **PENDING** | All | — | No JWT/passwords in docs |
| No manual DB edits performed | **PENDING** | Ops / DBA | — | Unless emergency approved |

## Status Legend

| Status | Meaning |
|--------|---------|
| **PENDING** | Not started / awaiting incident or drill |
| **READY** | Prerequisites met to start procedure |
| **PASS** | Step verified |
| **FAIL** | Verification failed — escalate |
| **NOT_APPLICABLE** | e.g. auth rollback not needed |

## Usage

1. Rollback owner approves decision gate
2. Update rows during procedure
3. Attach evidence references (ticket IDs — not secrets)
4. On completion: link to incident record; update PR-GAP-003 only after owner approval pack

## Next Pack

**Low-code Pilot Week-3 Rollback Owner Approval Pack v0.1**
