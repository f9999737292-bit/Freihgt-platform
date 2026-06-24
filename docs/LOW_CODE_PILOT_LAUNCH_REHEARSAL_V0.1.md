# Low-code Pilot Launch Rehearsal v0.1

## Summary

Dry-run rehearsal of the low-code staging pilot launch per `docs/LOW_CODE_PILOT_LAUNCH_RUNBOOK_V0.1.md`. Executed in **Accelerated AI Team Workflow** (PM → DevOps → QA → Security → Docs). **Safe operations only** — no import/migration/batch execute, no publish, no DB edits, no destructive Docker commands.

**Result:** All rehearsal checks passed. **GO_WITH_CONDITIONS** reaffirmed — ready for real staging pilot after operator repeats auth-on on staging environment.

## Current Commit

| Field | Value |
|-------|-------|
| Baseline commit | `a2069a1` — `docs: add virtual ai team workflow` |
| Runbook reference | `54e57b9` — decision + launch runbook |
| Rehearsal date | 2026-06-24 |
| Branch | `main` |
| Working tree at start | clean |

## Rehearsal Scope

### Allowed (performed)

- `make health-check`, `make seed-lowcode-demo`, `make integration-smoke-test`
- `npm run build` (web-admin)
- Runtime GET: active template, public list, custom values, audit
- Admin GET: template list, export
- Admin POST preview-only: import-preview, migration-preview, batch-migration-preview
- Temporary auth-on via gitignored compose override; restored default-off

### Not performed (deferred by design)

- Import execute, migration execute, batch migration execute
- Template publish / delete / archive
- Manual DB edits
- Browser manual UI walkthrough (documented as recommended before real pilot)
- Production or non-demo tenant writes

## AI Team Roles Used

| Role | Contribution |
|------|--------------|
| **PM** | Scope, success/stop criteria, final report |
| **DevOps** | Health checks, safe auth-on override, default-off restore |
| **QA** | Baseline smoke, API curl matrix, integration-smoke-test |
| **Security** | Auth 401/403/200, export safety, tenant/runtime policy review |
| **Docs** | This report, NEXT_COMMANDS update |

### PM plan (executed)

**Success criteria:**

- Default-off baseline 200 on active + admin list
- All safe API smoke endpoints 200
- Export: `schema_version: lowcode.template.export.v1`, no custom values
- Auth-on: 401 / 403 / 200; runtime unchanged
- Default-off restored; integration-smoke-test PASS
- No hard blockers

**Stop conditions (none triggered):**

- Admin accessible by non-admin with auth-on
- Health-check failure
- Preview endpoints failing
- Default-off not restorable

## Default-off Baseline Results

| Check | HTTP | Result |
|-------|------|--------|
| `make health-check` | — | OK |
| `make seed-lowcode-demo` | — | OK |
| `make integration-smoke-test` | — | PASSED (TEST-20260624132325) |
| Active template GET | 200 | PASS |
| Admin list GET (no `X-User-ID`) | 200 | PASS |
| After auth-on restore: admin list | 200 | PASS |
| Post-restore integration smoke | — | PASSED (TEST-20260624132428) |

Tenant: `74519f22-ff9b-4a8b-8fff-a958c689682f`

## API Smoke Results

Gateway: `http://localhost:8080/api/v1/low-code/...`

| Endpoint | Method | HTTP | Notes |
|----------|--------|------|-------|
| `/form-templates/active?entity_type=TRANSPORT_ORDER&template_code=transport_order_default` | GET | 200 | PUBLISHED active |
| `/admin/form-templates` | GET | 200 | default-off |
| `/admin/form-templates/b1111111-1111-4111-8111-111111111102/export` | GET | 200 | See export checks |
| `/admin/form-templates/import-preview` | POST | 200 | `new_version_request.json` |
| `/admin/custom-field-values/migration-preview` | POST | 200 | `lowcode_migration_preview_transport_order.json` |
| `/admin/custom-field-values/batch-migration-preview` | POST | 200 | `lowcode_batch_migration_preview_transport_order.json` |
| `/custom-field-values?entity_type=TRANSPORT_ORDER&entity_id=2db04b49-...` | GET | 200 | DEMO-TO-001 |
| `/audit-events?entity_type=TRANSPORT_ORDER&entity_id=...&limit=10` | GET | 200 | Events present |
| `/form-templates?entity_type=TRANSPORT_ORDER&limit=5` | GET | 200 | Public list |

### Export checks

| Check | Result |
|-------|--------|
| `schema_version` | `lowcode.template.export.v1` |
| Top-level `custom_values` | **Absent** |
| Audit logs in export | **Absent** (only `source`, `template`, `metadata`) |
| Checksum in metadata | Present |

## Auth-on Rehearsal Results

**Method:** Gitignored `docker-compose.override.yml` (not committed), recreate `low-code-service` only, deleted after test.

| Check | HTTP | Result |
|-------|------|--------|
| Admin list, no `X-User-ID` | 401 | PASS |
| Admin list, shipper (`008e1462-...`, SHIPPER_LOGIST) | 403 | PASS |
| Admin list, PLATFORM_ADMIN (`8541a3a3-...`) | 200 | PASS |
| Runtime active template (no user, auth-on) | 200 | PASS — not admin-guarded |
| `make health-check` during auth-on | OK | PASS |

**Restore:** Override deleted → `make platform-up-no-build` → default-off admin 200 → integration-smoke-test PASS.

Consistent with `docs/LOW_CODE_STAGING_AUTH_ON_VERIFICATION_V0.1.md`.

## Manual UI Rehearsal

**Status:** Not executed in browser during this pack (CLI/docs rehearsal).

**Recommended before real pilot** (dev/staging):

| Page | Check |
|------|-------|
| `/low-code` | Hub, service status |
| `/low-code/custom-field-values` | Entity lookup |
| `/low-code/audit` | Events list |
| `/low-code/admin/form-templates` | List, import wizard |
| `/low-code/admin/form-templates/[id]` | Export JSON |
| `/transport-orders/2db04b49-665c-469f-bcb1-ffeb1274fedb` | Custom fields panel |
| `/shipments/[id]`, `/billing-registers/[id]` | Phase 2 — optional in Phase 1 |

Login: `admin@7rights.local` / `Admin123456!` (dev only)

Build verification (proxy for UI compile health): `npm run build` — **PASS**

## Security Review

| Item | Status | Evidence |
|------|--------|----------|
| Admin requires PLATFORM_ADMIN (auth-on) | PASS | 401/403/200 rehearsal |
| Non-admin rejected | PASS | SHIPPER_LOGIST → 403 |
| Runtime not admin-guarded (by design) | PASS | Active template 200 without user |
| Tenant isolation | PASS | Integration smoke + tenant-scoped APIs |
| Import ignores source tenant for writes | PASS | Documented in hardening; preview-only here |
| No auto-publish | PASS | Execute not run; design DRAFT-only |
| No custom values in export | PASS | Export JSON inspected |
| Audit for writes/admin ops | PASS | Audit GET 200; events for demo entity |
| No `v-html` in low-code components | PASS | Grep: no matches in `components/low-code/` |

## Audit Review

- `GET /audit-events` → **200** for TRANSPORT_ORDER demo entity
- Historical value/migration/import events visible in dev tenant
- Rehearsal did not perform new writes (preview-only admin POSTs)
- Batch-level audit row deferred — entity-level + `batch_id` filter per design

## Stop Conditions Tested

| Stop condition | Tested | Triggered |
|----------------|--------|-----------|
| Non-admin admin access (auth-on) | Yes | **No** (403 correct) |
| Health-check failure | Yes | **No** |
| Preview endpoint failure | Yes | **No** (all 200) |
| Tenant isolation breach | Indirect (smoke) | **No** |
| Repeated 5xx | Not observed | **No** |

## Issues Found

| Issue | Severity | Action |
|-------|----------|--------|
| Manual UI not browser-tested in rehearsal | Low | Run UI checklist on staging before pilot users |
| Auth-on on real staging not re-verified here | Medium | Repeat curls on staging deploy |
| No DRIVER demo user in dev seed | Low | Create for staging negative tests |

**Hard blockers:** none

## Blockers

**None.** Launch rehearsal **READY** for staging execution per runbook.

## Go/No-Go Update

| Field | Value |
|-------|-------|
| Prior decision | GO_WITH_CONDITIONS (`LOW_CODE_PILOT_GO_NO_GO_REVIEW_V0.1.md`) |
| Rehearsal outcome | **Reaffirmed GO_WITH_CONDITIONS** |
| Launch status | **READY** (not BLOCKED) |
| Real staging pilot | Proceed when ops complete runbook Pre-launch Checklist |

## Recommended Next Steps

1. **Low-code Pilot Release Package Pack v0.1** — release notes, operator handoff, staging env checklist
2. On staging: enable auth-on, repeat auth curls with staging tenant/users
3. Manual UI walkthrough (admin + TO entity panel)
4. Phase 1 pilot: TRANSPORT_ORDER only, preview-before-execute discipline

If staging finds regressions → **Low-code Runtime Pilot Fix Pack v0.1**.

## Verification Commands

```powershell
cd D:\Projects\freight-platform

git status --short
git log --oneline -15

make health-check
make seed-lowcode-demo
make integration-smoke-test

cd apps\web-admin
npm run build
```

### Rehearsal curl (default-off)

```powershell
$T = "74519f22-ff9b-4a8b-8fff-a958c689682f"
$GW = "http://localhost:8080/api/v1"

curl.exe -H "X-Tenant-ID: $T" `
  "$GW/low-code/form-templates/active?entity_type=TRANSPORT_ORDER&template_code=transport_order_default"

curl.exe -H "X-Tenant-ID: $T" `
  "$GW/low-code/admin/form-templates"
```

References: `docs/LOW_CODE_PILOT_LAUNCH_RUNBOOK_V0.1.md`, `docs/ai-team/ACCELERATED_WORKFLOW.md`.
