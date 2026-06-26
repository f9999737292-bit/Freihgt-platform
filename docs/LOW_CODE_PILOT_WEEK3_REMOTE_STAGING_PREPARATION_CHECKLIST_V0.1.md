# Low-code Pilot Week-3 Remote Staging Preparation Checklist v0.1

## Summary

Ops/Security/Product checklist for inputs required before **Remote Auth-On Staging Repeat** (PR-GAP-001).

**Current status:** **WAIT_FOR_REMOTE_STAGING**

**Decision:** **REMOTE_STAGING_PREPARATION_CHECKLIST_CREATED**

**PR-GAP-001:** **BLOCKED_WAITING_FOR_REMOTE_STAGING**

**Production-ready not claimed.** **Controlled pilot continues** under `CONTROLLED_PILOT_APPROVED`.

**Docs-only pack** — no code, env, secrets, or staging writes.

## Current Status

| Field | Value |
|-------|-------|
| HEAD | `3594b8c` — `docs: add week 3 remote auth-on repeat verification` |
| Local auth-on decision | `AUTH_ON_REPEAT_LOCAL_VERIFIED` (2026-06-23) |
| Remote staging | **not available** |
| PR-GAP-001 | **BLOCKED_WAITING_FOR_REMOTE_STAGING** |
| Mode | **WAIT_FOR_REMOTE_STAGING** |

## Why This Checklist Exists

Local auth-on repeat **PASS** (`LOW_CODE_PILOT_WEEK3_REMOTE_AUTH_ON_REPEAT_V0.1.md`). Remote staging verification **cannot complete** without:

- Staging URL and API base
- Auth-on deployment config confirmation
- Admin and non-admin test identities
- Tenant ID
- Login/JWT flow (if gateway requires token)
- Read-only test permission

This checklist tells Ops/Security/Product **what to provide** and **what must not go into git**.

## Required Staging Inputs

| Item | Required Value | Status | Notes |
|------|----------------|--------|-------|
| Staging URL | `https://...` | **MISSING** | Required before remote verification |
| API URL | `https://.../api/v1/low-code` | **MISSING** | Required before curl/browser checks |
| Auth-on flag | `LOW_CODE_ADMIN_AUTH_ENABLED=true` | **MISSING** | Must be confirmed by Ops |
| Service restart confirmation | yes/no | **MISSING** | Required after auth-on config |
| Admin user email | provided separately | **MISSING** | Must not be stored with password |
| Admin user UUID | provided separately | **MISSING** | Needed if gateway supports header-based test |
| Non-admin user email | provided separately | **MISSING** | Must not be stored with password |
| Non-admin user UUID | provided separately | **MISSING** | Needed for forbidden-route verification |
| Tenant ID | `X-Tenant-ID` | **MISSING** | Required for tenant-bound APIs |
| Login endpoint | `POST /api/v1/auth/login` or equivalent | **MISSING** | Required if JWT is used |
| Read-only permission | yes/no | **MISSING** | Required before remote test |
| Smoke service account | yes/no | **TBD** | Optional, depends on staging auth model |

**Reason:** Remote staging verification cannot be completed because staging URL, auth-on config, admin/non-admin users, tenant ID, and login flow are **not provided yet**.

## Required Auth Configuration

| Item | Requirement |
|------|-------------|
| Flag | `LOW_CODE_ADMIN_AUTH_ENABLED=true` on **low-code-service** only |
| Method | Deployment config / env override — **not** tracked compose |
| Identity | `IDENTITY_SERVICE_URL` reachable from low-code-service |
| Restart | low-code-service restarted after flag change |
| Rollback | Ops can set flag `false` and restart if verification fails |

Reference: `LOW_CODE_PILOT_WEEK3_AUTH_ON_STAGING_RUNBOOK_V0.1.md`

## Required Users

| Role | Purpose | Stored in repo |
|------|---------|----------------|
| `PLATFORM_ADMIN` | Admin route **200** test | UUID/email only — **no password** |
| Non-admin (e.g. `SHIPPER_LOGIST`) | Admin route **403** test | UUID/email only — **no password** |
| Wrong-tenant user (optional) | Tenant boundary test AUTH-STG-006 | UUID only if provided |

**Dev reference (local only — staging values may differ):**

| Role | Email | User ID |
|------|-------|---------|
| PLATFORM_ADMIN | `admin@7rights.local` | `8541a3a3-bde7-4fed-9501-37b9953bf904` |
| SHIPPER_LOGIST | `shipper@7rights.local` | `008e1462-6f67-4246-b7dc-4aae1669c0c5` |

## Required Tenant Data

| Item | Requirement |
|------|-------------|
| Pilot tenant UUID | `X-Tenant-ID` for all low-code calls |
| Demo entities (optional) | TO/SH/BR demo IDs if runtime GET tests need known entities |
| Cross-tenant test tenant | Second tenant UUID for AUTH-STG-006 if available |

## Required Login / JWT Flow

Document **one** of:

| Model | What Ops provides |
|-------|-------------------|
| Header passthrough | Admin/non-admin **UUIDs**; curl uses `X-Tenant-ID` + `X-User-ID` |
| JWT gateway | Login endpoint, token header name; credentials **outside repo** |
| Service account | Scoped read-only token for smoke — **not stored in docs** |

If JWT: provide login URL and confirm QA can obtain token without committing it.

## Read-Only Verification Rules

| Rule | Value |
|------|-------|
| Methods allowed | **GET only** |
| Writes | **Forbidden** — no POST/PUT/PATCH/DELETE |
| Template publish | **Forbidden** |
| Migration/import/batch | **Forbidden** |
| Manual DB | **Forbidden** |
| Evidence | HTTP status codes + non-secret summaries only |

Ops must confirm **read-only permission: yes** before remote pack runs.

## Security Rules

1. **Do not commit** passwords, JWT, service tokens, or `.env` staging files.
2. Docs may state: *credentials provided separately / not stored*.
3. **Do not** leave auth-on enabled in tracked compose.
4. **Do not** claim production-ready from staging prep alone.
5. Stop and open **Runtime Pilot Fix Pack** on P0/P1 (e.g. non-admin **200** on admin routes).

## Evidence To Capture

After staging repeat (future pack):

| Evidence | Format |
|----------|--------|
| Per-test HTTP status | Table in evidence doc — no response bodies with secrets |
| Auth-on config confirmation | Ops sign-off (yes/no + date) |
| Service restart | Ops sign-off |
| Rollback confirmation | Default-off or documented exception |

See `LOW_CODE_PILOT_WEEK3_REMOTE_STAGING_AUTH_ON_TEST_MATRIX_V0.1.md`.

## Blocking Conditions

| Condition | Effect |
|-----------|--------|
| No staging URL | **BLOCKED** — cannot run remote matrix |
| Auth-on not enabled on staging | **BLOCKED** |
| No admin/non-admin identities | **BLOCKED** |
| No tenant ID | **BLOCKED** |
| Read-only permission **no** | **BLOCKED** |
| JWT required but login flow unknown | **BLOCKED** |

## Ready-To-Test Criteria

All **MISSING** items in Required Staging Inputs become **PROVIDED** (externally):

- [ ] Staging URL + API URL
- [ ] `LOW_CODE_ADMIN_AUTH_ENABLED=true` + restart confirmed
- [ ] Admin email/UUID + non-admin email/UUID
- [ ] Tenant ID
- [ ] Login/JWT flow documented (if needed)
- [ ] Read-only permission **yes**
- [ ] Remote Staging Preparation Checklist Pack v0.1 **completed** (this doc)

## Next Pack

**Low-code Pilot Week-3 Remote Auth-On Staging Repeat Pack v0.1**

Trigger: **Remote staging details provided**

Reference: `LOW_CODE_PILOT_WEEK3_REMOTE_STAGING_OPS_REQUEST_V0.1.md`

## Verification Commands

```powershell
cd D:\Projects\freight-platform
git status --short
make health-check
```

Local reference runbook: `LOW_CODE_PILOT_WEEK3_AUTH_ON_STAGING_RUNBOOK_V0.1.md`
