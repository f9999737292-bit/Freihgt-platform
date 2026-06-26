# Low-code Pilot Week-3 Remote Staging Ops Request v0.1

## Request Summary

Formal request to **Ops**, **Security**, and **Product/PM** for staging inputs needed to close **PR-GAP-001** (Remote Auth-On Repeat on remote staging).

**Status:** **WAIT_FOR_REMOTE_STAGING**

**Local auth-on:** `AUTH_ON_REPEAT_LOCAL_VERIFIED` — remote repeat **blocked**.

Ops/Security must provide staging URL, auth-on confirmation, service restart confirmation, admin/non-admin test users, tenant ID, and login/JWT flow. Credentials must be provided **separately** and must **not** be committed to git.

## Needed From Ops

| # | Item | Notes |
|---|------|-------|
| 1 | Staging URL | e.g. `https://staging.example.com` |
| 2 | API base URL | e.g. `{staging}/api/v1/low-code` |
| 3 | Enable `LOW_CODE_ADMIN_AUTH_ENABLED=true` on low-code-service | Deployment config only — not git |
| 4 | Confirm low-code-service **restarted** after config change | yes/no + date |
| 5 | Health-check or gateway reachability confirmation | read-only |
| 6 | Rollback contact | who can disable auth-on if tests fail |

## Needed From Security

| # | Item | Notes |
|---|------|-------|
| 1 | Admin test user | `PLATFORM_ADMIN` — email + UUID |
| 2 | Non-admin test user | e.g. `SHIPPER_LOGIST` — email + UUID |
| 3 | Tenant ID for pilot scope | `X-Tenant-ID` |
| 4 | Login/JWT flow | If gateway requires Bearer token — endpoint + header format |
| 5 | Read-only test approval | **yes** required before remote GET matrix |
| 6 | Optional wrong-tenant UUID | For AUTH-STG-006 |

## Needed From Product / PM

| # | Item | Notes |
|---|------|-------|
| 1 | Confirm controlled pilot scope on staging tenant | Align with scope charter |
| 2 | Named Ops/Security owner for staging handoff | TBD in owner matrix |
| 3 | Priority / target date for staging inputs | Optional |

## Credentials Handling

| Secret type | Rule |
|-------------|------|
| Passwords | **Not stored in repo** — provide via secure channel |
| JWT / Bearer tokens | **Not stored in repo** — obtain at test time only |
| Service tokens | **Not stored in repo** |
| Docs | May only say: *credentials provided separately / not stored* |
| User UUIDs / tenant ID | May be recorded in evidence doc **after** Ops confirms (non-secret identifiers) |

## Read-Only Rule

Remote verification pack runs **GET only**. No staging writes, publish, migrate, import, or batch execute.

Ops must confirm **read-only permission: yes** before **Remote Auth-On Staging Repeat Pack v0.1**.

## Deadline / Priority

| Field | Value |
|-------|-------|
| Priority | **P3** — PR-GAP-001 blocker for production readiness |
| Parallel work | Controlled pilot **continues**; other gap packs may proceed |
| Blocking | Production go/no-go remains blocked until PR-GAP-001 closed |

## Next Pack After Inputs

**Low-code Pilot Week-3 Remote Auth-On Staging Repeat Pack v0.1**

**Trigger:** Remote staging details provided (all **MISSING** checklist items → **PROVIDED**)

**Test matrix:** `LOW_CODE_PILOT_WEEK3_REMOTE_STAGING_AUTH_ON_TEST_MATRIX_V0.1.md`

**Checklist:** `LOW_CODE_PILOT_WEEK3_REMOTE_STAGING_PREPARATION_CHECKLIST_V0.1.md`

Reference: `LOW_CODE_PILOT_WEEK3_PRODUCTION_READINESS_GAP_TRACKER_V0.1.md` (PR-GAP-001)
