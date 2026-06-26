# Low-code Pilot Week-3 Staging Input Form v0.1

## Purpose

Handoff form for Ops/Security to complete **after** staging deploy. Used to unblock **PR-GAP-001** and trigger **Remote Auth-On Staging Repeat Pack v0.1**.

**Do not commit passwords, JWT, or tokens in this file.**

## How To Fill This Form

1. Deploy staging per `LOW_CODE_PILOT_WEEK3_STAGING_DEPLOY_RUNBOOK_V0.1.md`
2. Complete readiness checklist `LOW_CODE_PILOT_WEEK3_STAGING_READINESS_CHECKLIST_V0.1.md`
3. Copy template below into secure ticket / PM channel (or commit **sanitized** copy with UUIDs only)
4. Provide passwords via **separate secure channel**

## Required Inputs

| # | Field | Required |
|---|-------|----------|
| 1 | Staging URL | yes |
| 2 | API URL | yes |
| 3 | Auth-on confirmed | yes |
| 4 | Service restarted | yes |
| 5–8 | Admin/non-admin email + UUID | yes |
| 9 | Tenant ID | yes |
| 10–11 | Login endpoint + JWT flow | yes if JWT required |
| 13 | Read-only permission | yes — must be **yes** |

## Optional Inputs

| # | Field |
|---|-------|
| 12 | Service account required |
| 14–16 | Ops owner, Security owner, Notes |
| Deploy commit SHA | Recommended |
| Wrong-tenant UUID | For AUTH-STG-006 |

## Credentials Handling

| Secret | Rule |
|--------|------|
| Passwords | Provided separately — **not in repo** |
| JWT | Obtained at test time — **not stored in repo** |
| Tokens | **Not stored in repo** |
| Docs | May only say: *credentials provided separately / not stored* |
| UUIDs / URLs | May be recorded after Ops confirmation |

## Filled Form Template

```text
=== Low-code Pilot Week-3 Staging Input Form ===
Date: YYYY-MM-DD
Deploy commit SHA: <git-sha>
Pack baseline: STAGING_DEPLOY_RUNBOOK_CREATED

1. Staging URL:
2. API URL:
3. LOW_CODE_ADMIN_AUTH_ENABLED=true confirmed: yes/no
4. Service restarted after auth-on change: yes/no
5. Admin user email:
6. Admin user UUID:
7. Non-admin user email:
8. Non-admin user UUID:
9. Tenant ID:
10. Login endpoint:
11. JWT flow required: yes/no
12. Service account required: yes/no
13. Read-only verification allowed: yes/no
14. Ops owner:
15. Security owner:
16. Notes:

Credentials: provided separately / not stored
```

## Ready-To-Run Criteria

Remote Auth-On Staging Repeat Pack may start when:

- Items 1–4, 5–9, 13 are complete
- Item 3 = **yes**, Item 4 = **yes**, Item 13 = **yes**
- Item 11 documented if **yes**
- Readiness checklist required rows = **READY**
- No secrets pasted into committed docs

## Next Pack

**Low-code Pilot Week-3 Remote Auth-On Staging Repeat Pack v0.1**

Test matrix: `LOW_CODE_PILOT_WEEK3_REMOTE_STAGING_AUTH_ON_TEST_MATRIX_V0.1.md`

Alternative (if approved): **Temporary Tunnel Auth-On Matrix Pack v0.1** — see `LOW_CODE_PILOT_WEEK3_TEMPORARY_TUNNEL_OPTION_V0.1.md`
