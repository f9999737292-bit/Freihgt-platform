# Low-code Pilot Week-3 Auth-On Staging Verification v0.1

## Summary

Week-3 auth-on verification for low-code **admin** operations. Confirmed that with `LOW_CODE_ADMIN_AUTH_ENABLED=true`:

- **PLATFORM_ADMIN** receives HTTP **200** on admin list endpoints
- **SHIPPER_LOGIST** (non-admin) receives HTTP **403 FORBIDDEN**
- Requests **without `X-User-ID`** receive HTTP **401 UNAUTHORIZED**
- **Runtime GET** endpoints remain compatible (HTTP **200** without admin user context)
- **Default-off** dev mode restored and re-verified after temporary auth-on

**Verification decision: AUTH_ON_PARTIAL_VERIFIED**

All auth-on checks passed via **local Docker temporary override** (gitignored, not committed). **Live remote staging** was not available in this pack — repeat on deployment staging when ops config is ready.

**Docs-only pack** — no backend, frontend, API contract, migration, env, or secret changes committed.

## Current Commit

| Field | Value |
|-------|-------|
| HEAD | `891d7bb` — `docs: add week 3 monitoring evidence` |
| Verification date | 2026-06-24 |
| Branch | `main` |
| Backend / frontend changed in this pack | **no** |

## Scope

**In scope**

- Pre-flight git/health baseline
- Default-off compatibility (runtime + admin list without `X-User-ID`)
- Temporary auth-on via gitignored Docker Compose override
- Admin endpoint positive/negative checks (read-only)
- Runtime GET compatibility with auth-on enabled
- Audit GET (runtime route) compatibility
- Permission matrix / code reference review (read-only)
- Rollback to default-off + integration smoke
- Documentation and runbook

**Out of scope**

- Committing env / compose override / secrets
- Permanent local auth-on left enabled
- Production or remote staging deployment changes
- Admin write endpoints (create, publish, import, migration execute)
- PUT/save on custom-field-values
- Backend / frontend code changes

## Evidence Documents

| Document | Found | Purpose |
|----------|-------|---------|
| `LOW_CODE_PILOT_WEEK3_MONITORING_EVIDENCE_V0.1.md` | **yes** | Week-3 monitoring baseline |
| `LOW_CODE_PILOT_WEEK3_MONITORING_BASELINE_REPORT_V0.1.md` | **yes** | Day 0/1 monitoring report |
| `LOW_CODE_PILOT_WEEK3_MONITORING_RUNBOOK_V0.1.md` | **yes** | Daily monitoring procedures |
| `LOW_CODE_PILOT_WEEK3_EXECUTION_PLAN_V0.1.md` | **yes** | Week-3 workstream 2 |
| `LOW_CODE_PERMISSIONS_MATRIX_V0.1.md` | **yes** | Role × action matrix |
| `AUTH_RBAC.md` | **yes** | Web-admin auth reference |
| `LOW_CODE_RUNTIME_PILOT_STAGING_CHECKLIST_V0.1.md` | **yes** | Staging handoff |
| `LOW_CODE_RUNTIME_PILOT_READINESS_V0.1.md` | **yes** | Runtime readiness |
| `LOW_CODE_STAGING_AUTH_ON_VERIFICATION_V0.1.md` | **yes** | Prior local auth-on pack (reference) |

**Missing critical evidence docs:** none.

## Environment

| Item | Value |
|------|-------|
| Gateway | `http://localhost:8080/api/v1` |
| Tenant ID | `74519f22-ff9b-4a8b-8fff-a958c689682f` |
| PLATFORM_ADMIN user ID | `8541a3a3-bde7-4fed-9501-37b9953bf904` (`admin@7rights.local`) |
| Non-admin user ID | `008e1462-6f67-4246-b7dc-4aae1669c0c5` (`shipper@7rights.local`, `SHIPPER_LOGIST`) |
| Tracked compose default | `LOW_CODE_ADMIN_AUTH_ENABLED: "false"` |
| Live remote staging | **not available** in this pack |
| Auth-on method | Local gitignored `docker-compose.override.yml` (temporary) |

## Default-off Baseline Checks

Pre-flight:

```text
git status --short   → clean
git log --oneline    → 891d7bb at HEAD
make health-check    → OK (all 9 services)
make seed-lowcode-demo → OK
```

Default-off curl (tenant header only):

| Endpoint | HTTP | Result |
|----------|------|--------|
| `GET .../form-templates/active?entity_type=SHIPMENT&template_code=shipment_default` | **200** | **PASS** |
| `GET .../custom-field-values?entity_type=SHIPMENT&entity_id=14d405e2-0152-4030-b356-eec464a3cc66&template_code=shipment_default` | **200** | **PASS** |
| `GET .../admin/form-templates` (no `X-User-ID`) | **200** | **PASS** — default-off open admin |

Default-off compatibility **preserved**.

## Auth-on Verification Method

1. Created **gitignored** temporary override: `infrastructure/docker-compose/docker-compose.override.yml`
2. Set `LOW_CODE_ADMIN_AUTH_ENABLED: "true"` on `low-code-service` only
3. Restarted: `docker compose -f infrastructure/docker-compose/docker-compose.yml -f infrastructure/docker-compose/docker-compose.override.yml up -d --no-build low-code-service`
4. `make health-check` → OK
5. Executed read-only admin/runtime curl checks
6. **Rollback:** deleted override file; `make platform-up-no-build`; verified default-off admin **200** without user
7. `make integration-smoke-test` → **PASS** (`TEST-20260624193511`)

Override file **not committed** (listed in `.gitignore`).

## PLATFORM_ADMIN Result

```powershell
curl.exe -i `
  -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" `
  -H "X-User-ID: 8541a3a3-bde7-4fed-9501-37b9953bf904" `
  "http://localhost:8080/api/v1/low-code/admin/form-templates"
```

| Field | Value |
|-------|-------|
| HTTP | **200** |
| Body | **9** template items returned |
| Result | **PASS** — admin endpoint accessible for PLATFORM_ADMIN |

## Non-admin Forbidden Result

```powershell
curl.exe -i `
  -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" `
  -H "X-User-ID: 008e1462-6f67-4246-b7dc-4aae1669c0c5" `
  "http://localhost:8080/api/v1/low-code/admin/form-templates"
```

| Field | Value |
|-------|-------|
| HTTP | **403** |
| Body code | `FORBIDDEN` |
| Message | `low-code admin access required` |
| Role tested | `SHIPPER_LOGIST` (non-admin) |
| Result | **PASS** — non-admin cannot access admin endpoint |

## Missing User Result

```powershell
curl.exe -i `
  -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" `
  "http://localhost:8080/api/v1/low-code/admin/form-templates"
```

| Field | Value |
|-------|-------|
| HTTP | **401** |
| Body code | `UNAUTHORIZED` |
| Message | `authenticated user is required for low-code admin operations` |
| Result | **PASS** — missing user blocked in auth-on mode |

## Runtime GET Compatibility Result

With auth-on enabled, runtime routes (outside `RequireLowCodeAdmin` guard):

| Endpoint | HTTP (no `X-User-ID`) | Result |
|----------|----------------------|--------|
| `GET .../form-templates/active?entity_type=SHIPMENT&template_code=shipment_default` | **200** | **PASS** |
| `GET .../custom-field-values?entity_type=SHIPMENT&entity_id=14d405e2-0152-4030-b356-eec464a3cc66&template_code=shipment_default` | **200** | **PASS** |
| `GET .../audit-events?limit=20` | **200** | **PASS** — runtime/read route unchanged |

Runtime GET remains **tenant-scoped only** — no admin user context required. Matches `LOW_CODE_PERMISSIONS_MATRIX_V0.1.md` and prior `LOW_CODE_STAGING_AUTH_ON_VERIFICATION_V0.1.md`.

## Admin Endpoint Coverage

Safe read-only admin checks performed:

| Endpoint | No user | Non-admin | PLATFORM_ADMIN |
|----------|---------|-----------|----------------|
| `GET /low-code/admin/form-templates` | **401** | **403** | **200** |

Not executed (forbidden in this pack):

- POST create/update template
- POST publish / clone
- POST import-preview / import execute
- POST migration-preview / migration execute
- POST batch-migration execute

Optional export GET deferred — template list sufficient for Week-3 admin read coverage.

## Permission Matrix Review

References reviewed (read-only, no code changes):

| Source | Finding |
|--------|---------|
| `docs/LOW_CODE_PERMISSIONS_MATRIX_V0.1.md` | Admin `/admin/*` requires `PLATFORM_ADMIN` when auth-on; runtime GET/PUT tenant-scoped |
| `docs/AUTH_RBAC.md` | Dev admin seed; `usePermissions()` + `useLowCodePermissions()` documented |
| `packages/shared-go/auth/lowcode_admin.go` | `LowCodeAdminRoleCodes = ["PLATFORM_ADMIN"]`; `HasLowCodeAdminRole()` |
| `apps/web-admin/composables/useLowCodePermissions.ts` | Admin UI gates: `canAccessLowCodeAdmin()` → PLATFORM_ADMIN; runtime write UI-gated by operator roles; DRIVER excluded from runtime write |

**Documented guardrails:**

- PLATFORM_ADMIN required for admin low-code operations (auth-on)
- Non-admin forbidden on admin routes
- Runtime panel: tenant header; UI gates for write by operator roles
- DRIVER: no admin; no runtime write in UI v0.1
- Default-off: admin routes open with tenant only (dev smoke compatible)

## Security Review

| Check | Result |
|-------|--------|
| Secrets committed | **no** |
| Env files committed | **no** |
| Auth-on config committed | **no** — override gitignored and deleted |
| Production writes | **no** |
| Admin write endpoints executed | **no** |
| PUT/save executed | **no** |
| Non-admin forbidden | **yes** — HTTP 403 |
| Missing user blocked | **yes** — HTTP 401 |
| PLATFORM_ADMIN allowed | **yes** — HTTP 200 |
| Runtime compatibility | **yes** — HTTP 200 |
| Rollback to default-off verified | **yes** — admin without user → **200**; smoke **PASS** |

## Issues Found

None blocking pilot auth-on.

| Note | Severity |
|------|----------|
| Live remote staging not verified in this pack | Informational — use deployment config when available |
| No dedicated DRIVER user in demo seed — SHIPPER_LOGIST used for negative test | Informational — same as prior pack |
| Export GET not re-run (list coverage sufficient) | Informational |

## Blockers

**None (P0).** Auth-on behavior verified locally.

## Decision

**AUTH_ON_PARTIAL_VERIFIED**

Rationale:

- All auth-on curl checks **pass** via safe temporary local override
- Default-off baseline and rollback **pass**
- Permission matrix and prior staging verification doc **align**
- **Live remote staging** environment was **not available** — cannot claim full `AUTH_ON_VERIFIED` for deployment staging

Alternative decisions **not** selected:

- **AUTH_ON_VERIFIED** — rejected: remote staging not exercised
- **AUTH_ON_NOT_READY** — rejected: local verification complete, no failures
- **STOPPED** — rejected: no P0 security failures

## Conditions

1. Repeat auth-on matrix on **deployment staging** when ops enables `LOW_CODE_ADMIN_AUTH_ENABLED=true` via deployment config (not tracked compose).
2. Ensure gateway/web-admin forwards `X-User-ID` from authenticated sessions for admin UI.
3. Do **not** commit auth-on to tracked `docker-compose.yml`.
4. Collect operator feedback (Week-3 next pack).
5. Maintain default-off for local dev smoke unless intentional pilot session.

## Recommended Next Steps

1. **Low-code Pilot Week-3 Operator Feedback Collection Pack v0.1**
2. Ops: apply auth-on on staging via deployment override when ready; re-run runbook commands.
3. Continue daily monitoring per `LOW_CODE_PILOT_WEEK3_MONITORING_RUNBOOK_V0.1.md`.

## Verification Commands

```powershell
cd D:\Projects\freight-platform

# Pre-flight
git status --short
make health-check
make seed-lowcode-demo

# Default-off baseline
$T = "74519f22-ff9b-4a8b-8fff-a958c689682f"
curl.exe -i -H "X-Tenant-ID: $T" "http://localhost:8080/api/v1/low-code/form-templates/active?entity_type=SHIPMENT&template_code=shipment_default"
curl.exe -i -H "X-Tenant-ID: $T" "http://localhost:8080/api/v1/low-code/custom-field-values?entity_type=SHIPMENT&entity_id=14d405e2-0152-4030-b356-eec464a3cc66&template_code=shipment_default"
curl.exe -i -H "X-Tenant-ID: $T" "http://localhost:8080/api/v1/low-code/admin/form-templates"

# Auth-on (temporary override — see runbook; NOT committed)
# After restart with LOW_CODE_ADMIN_AUTH_ENABLED=true:
curl.exe -i -H "X-Tenant-ID: $T" -H "X-User-ID: 8541a3a3-bde7-4fed-9501-37b9953bf904" "http://localhost:8080/api/v1/low-code/admin/form-templates"
curl.exe -i -H "X-Tenant-ID: $T" -H "X-User-ID: 008e1462-6f67-4246-b7dc-4aae1669c0c5" "http://localhost:8080/api/v1/low-code/admin/form-templates"
curl.exe -i -H "X-Tenant-ID: $T" "http://localhost:8080/api/v1/low-code/admin/form-templates"
curl.exe -i -H "X-Tenant-ID: $T" "http://localhost:8080/api/v1/low-code/form-templates/active?entity_type=SHIPMENT&template_code=shipment_default"
curl.exe -i -H "X-Tenant-ID: $T" "http://localhost:8080/api/v1/low-code/custom-field-values?entity_type=SHIPMENT&entity_id=14d405e2-0152-4030-b356-eec464a3cc66&template_code=shipment_default"
curl.exe -i -H "X-Tenant-ID: $T" "http://localhost:8080/api/v1/low-code/audit-events?limit=20"

# Rollback + regression
make platform-up-no-build
make health-check
make integration-smoke-test

# Frontend
cd apps\web-admin
npm run build
```
