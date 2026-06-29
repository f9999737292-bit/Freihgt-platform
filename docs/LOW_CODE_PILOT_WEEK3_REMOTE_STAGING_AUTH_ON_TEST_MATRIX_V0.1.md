# Low-code Pilot Week-3 Remote Staging Auth-On Test Matrix v0.1

## Summary

Read-only test matrix for **remote staging** auth-on verification (PR-GAP-001). Execute only after staging inputs are **PROVIDED** per preparation checklist.

**Not executed yet** — staging details missing; repeat pack blocked 2026-06-23.

Reference: `LOW_CODE_PILOT_WEEK3_REMOTE_STAGING_PREPARATION_CHECKLIST_V0.1.md`

## Scope

**In scope:** GET requests; HTTP status evidence; tenant-scoped runtime compatibility.

**Out of scope:** Any write, publish, migrate, import, batch execute, DB edits.

## Preconditions

- [ ] Remote Staging Preparation Checklist Pack v0.1 completed
- [ ] Staging URL + API URL provided
- [ ] `LOW_CODE_ADMIN_AUTH_ENABLED=true` confirmed + service restarted
- [ ] Admin and non-admin test users identified (UUID and/or JWT flow)
- [ ] Tenant ID provided
- [ ] Read-only permission **yes**
- [ ] Credentials handled **outside repo**

## Test Matrix

| Test ID | Actor | Endpoint Type | Method | Expected Result | Evidence |
|---------|-------|---------------|--------|-----------------|----------|
| AUTH-STG-001 | Admin | admin low-code templates (`GET /admin/form-templates`) | GET | **200 OK** or allowed response | Response code only — no secrets |
| AUTH-STG-002 | Non-admin | admin low-code templates | GET | **403 Forbidden** | Response code only |
| AUTH-STG-003 | Anonymous / no token | admin low-code templates | GET | **401** or **403** | Response code only |
| AUTH-STG-004 | Admin | runtime active templates (`GET /form-templates/active?...`) | GET | **200 OK** | Response code only |
| AUTH-STG-005 | Non-admin | runtime active templates | GET | **200 OK** if runtime GET compatible | Response code only |
| AUTH-STG-006 | Wrong tenant | tenant-bound endpoint | GET | **403** / **404** / no cross-tenant data | Response code only |
| AUTH-STG-007 | Admin | audit-events (`GET /audit-events?limit=20`) | GET | **200 OK** or allowed admin response | Response code only |
| AUTH-STG-008 | Non-admin | audit-events | GET | **403 Forbidden** if admin-only; else document actual policy | Response code only |

### Example endpoints (adjust base URL from Ops)

```text
GET {API_BASE}/admin/form-templates
GET {API_BASE}/form-templates/active?entity_type=SHIPMENT&template_code=shipment_default
GET {API_BASE}/custom-field-values?entity_type=SHIPMENT&entity_id={ID}&template_code=shipment_default
GET {API_BASE}/audit-events?limit=20
```

## Expected Results

| Test ID | Pass |
|---------|------|
| AUTH-STG-001 | Admin can list admin templates |
| AUTH-STG-002 | Non-admin blocked from admin routes |
| AUTH-STG-003 | Unauthenticated blocked from admin routes |
| AUTH-STG-004 | Runtime active template works with auth-on |
| AUTH-STG-005 | Runtime GET not broken for non-admin |
| AUTH-STG-006 | No cross-tenant data leak |
| AUTH-STG-007 | Audit read behaves per staging policy |
| AUTH-STG-008 | Non-admin audit access matches policy |

Align with local baseline: `LOW_CODE_PILOT_WEEK3_REMOTE_AUTH_ON_REPEAT_V0.1.md`.

## Forbidden Actions

During remote staging verification:

- POST / PUT / PATCH / DELETE
- Template publish / clone / create / update
- Migration preview execute / batch execute
- Import preview execute / import execute
- Manual DB writes
- Committing credentials or full response bodies with tokens

## Evidence Format

Future evidence doc (`REMOTE_AUTH_ON_EVIDENCE` pack) should contain:

| Column | Content |
|--------|---------|
| Test ID | AUTH-STG-00x |
| Date | ISO date |
| HTTP status | e.g. 200, 403 |
| Pass/Fail | PASS / FAIL |
| Notes | Non-secret only |

**Do not store:** passwords, JWT, Authorization headers, full error stacks with secrets.

## Pass / Fail Criteria

**Matrix PASS:** AUTH-STG-001 through AUTH-STG-005 **PASS**; AUTH-STG-006 **PASS** or documented waiver with Security approval; AUTH-STG-007/008 match documented staging policy.

**Matrix FAIL / STOP:** Non-admin **200** on admin list (AUTH-STG-002); runtime GET broken with auth-on (AUTH-STG-004/005); cross-tenant leak (AUTH-STG-006).

## Next Decision

After successful remote matrix (future pack):

| Decision | Condition |
|----------|-----------|
| `AUTH_ON_REMOTE_VERIFIED` | All required tests PASS on staging |
| `AUTH_ON_REMOTE_PARTIAL` | Some tests PASS; waivers documented |
| `AUTH_ON_REMOTE_NOT_READY` | Staging reachable but checks fail |
| PR-GAP-001 **CLOSED** | Only when remote repeat PASS + evidence doc approved |

Current: **wait** — staging inputs **MISSING**.
