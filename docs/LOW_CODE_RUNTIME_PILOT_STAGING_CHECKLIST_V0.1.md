# Low-code Runtime Pilot Staging Checklist v0.1

## Summary

Final staging/pilot checklist for the low-code runtime layer: environment flags, data safety, API smoke commands, manual UI verification, observability, risk register, rollback plan, and go/no-go criteria. **Docs-only pack** — no backend, frontend, API, or migration changes.

Use this document before enabling low-code in a staging or pilot environment. Follow **Low-code Staging Auth-On Verification Pack v0.1** immediately after completing the auth-on section here.

## Current Commit

| Field | Value |
|-------|-------|
| Commit | `b7707bf` |
| Message | `chore: harden low-code template import export` |
| Branch | `main` |
| Checklist date | 2026-06-24 |

Related baseline docs: `LOW_CODE_RUNTIME_PILOT_READINESS_V0.1.md`, `LOW_CODE_PERMISSIONS_MATRIX_V0.1.md`, `LOW_CODE_TEMPLATE_IMPORT_EXPORT_HARDENING_V0.1.md`.

## Scope

**In scope**

- Staging environment preparation
- Auth-on pilot toggle checklist
- API smoke (read-only + preview where safe)
- Manual UI walkthrough
- Risk register and rollback
- Recommended phased pilot scope

**Out of scope**

- Code changes, new features, migrations
- Production cutover automation
- Live compose changes in this pack (manual operator steps documented)

## Pilot-ready Capabilities

### Runtime

| Capability | Status | Reference |
|------------|--------|-----------|
| Active template API | Ready | `GET /api/v1/low-code/form-templates/active` |
| Custom field values GET/PUT | Ready | `LOW_CODE_CUSTOM_FIELD_VALUES_API_V0.1.md` |
| `validation_context` on PUT | Ready | `LOW_CODE_ENTITY_INTEGRATION_V0.2.md` |
| Conditional required validation | Ready | `LOW_CODE_CONDITIONAL_REQUIRED_VALIDATION_V0.1.md` |
| Entity panels: TRANSPORT_ORDER | Ready | `/transport-orders/[id]` |
| Entity panels: SHIPMENT | Ready | `/shipments/[id]` |
| Entity panels: BILLING_REGISTER | Ready | `/billing-registers/[id]` |
| Audit events GET | Ready | `LOW_CODE_AUDIT_LOG_V0.1.md` |
| Public templates PUBLISHED only | Ready | No DRAFT in public list |

### Admin

| Capability | Status | Reference |
|------------|--------|-----------|
| Form template admin list/detail | Ready | `/low-code/admin/form-templates` |
| Builder / editor / drag-drop | Ready | `LOW_CODE_DRAG_AND_DROP_FORM_BUILDER_V0.1.md` |
| Preview / diff / publish | Ready | `LOW_CODE_FORM_TEMPLATE_VERSION_COMPARE_V0.1.md` |
| Clone published → DRAFT | Ready | `LOW_CODE_CLONE_PUBLISHED_TEMPLATE_TO_DRAFT_V0.1.md` |
| Migration preview / execute | Ready | `LOW_CODE_MIGRATE_TO_ACTIVE_*` |
| Batch migration preview / execute | Ready | Max 100 entities |
| Template import / export | Ready | Import creates DRAFT only; no auto-publish |

### Security

| Capability | Status | Reference |
|------------|--------|-----------|
| Permissions matrix | Ready | `LOW_CODE_PERMISSIONS_MATRIX_V0.1.md` |
| Admin guard (`LOW_CODE_ADMIN_AUTH_ENABLED`) | Ready (default-off) | `LOW_CODE_PERMISSIONS_ADMIN_GUARDRAILS_V0.1.md` |
| UI `useLowCodePermissions()` | Ready | Admin + runtime write gates |
| Default-off dev compatibility | Ready | Tenant header only |
| Pilot auth-on checklist | Documented | See below |

### Observability

| Capability | Status | Reference |
|------------|--------|-----------|
| `make health-check` | Ready | All services including low-code |
| Audit events | Ready | Value, migration, import/export |
| Prometheus metrics | Ready | `/metrics` on low-code-service |
| Batch migration metrics | Ready | Bounded labels (no tenant_id in labels) |
| Import/export audit | Ready | `FORM_TEMPLATE_*` event kinds |

## Staging Environment Checklist

### Environment

- [ ] Platform up: `make platform-up-no-build` (or staging equivalent)
- [ ] `make health-check` — all services **OK**, including `low-code-service`
- [ ] Database migrations applied: `make migrate-up` (staging)
- [ ] Gateway routes low-code: `http://{gateway}/api/v1/low-code/*` → low-code-service
- [ ] `low-code-service` direct health: `http://localhost:8088/health` (dev) or staging port
- [ ] **Pilot only:** set `LOW_CODE_ADMIN_AUTH_ENABLED=true` on `low-code-service` (see Auth-on Checklist)
- [ ] `IDENTITY_SERVICE_URL` set when auth-on enabled
- [ ] Confirm `LOW_CODE_ADMIN_AUTH_ENABLED` **not** left `true` in local dev compose unless intentional

### Tenant and users

| Item | Dev/staging demo value |
|------|------------------------|
| Tenant ID | `74519f22-ff9b-4a8b-8fff-a958c689682f` |
| Platform admin email | `admin@7rights.local` |
| Platform admin password | `Admin123456!` (dev only) |
| Platform admin user ID (curl auth-on) | `8541a3a3-bde7-4fed-9501-37b9953bf904` |
| Published TO template ID | `b1111111-1111-4111-8111-111111111102` |
| Template code | `transport_order_default` |

- [ ] Pilot tenant ID documented and isolated from production tenants
- [ ] `PLATFORM_ADMIN` role assigned for pilot admin user(s)
- [ ] Non-admin pilot users (e.g. `SHIPPER_LOGIST`, `DRIVER`) available for negative tests

### Template runtime rules

- [ ] `GET /api/v1/low-code/form-templates` returns **PUBLISHED only** (no DRAFT)
- [ ] Active template resolves: `GET .../form-templates/active?entity_type=TRANSPORT_ORDER&code=transport_order_default`
- [ ] Response includes `is_active: true` for current published version
- [ ] No accidental publish of test DRAFT during staging prep

## Auth-on Checklist

**When:** Staging/pilot only. **Rollback:** set `LOW_CODE_ADMIN_AUTH_ENABLED=false` and restart `low-code-service`.

1. [ ] Backup DB before toggle
2. [ ] Set on `low-code-service`:
   ```yaml
   LOW_CODE_ADMIN_AUTH_ENABLED: "true"
   IDENTITY_SERVICE_URL: http://identity-service:8081
   ```
3. [ ] Restart `low-code-service` only: `docker compose ... restart low-code-service`
4. [ ] Confirm web-admin / gateway sends `X-User-ID` from authenticated session
5. [ ] **Positive:** Admin curl with tenant + admin user ID → 200
   ```powershell
   curl.exe -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" `
     -H "X-User-ID: 8541a3a3-bde7-4fed-9501-37b9953bf904" `
     "http://localhost:8080/api/v1/low-code/admin/form-templates?limit=5"
   ```
6. [ ] **Negative:** Same without `X-User-ID` → `401 UNAUTHORIZED`
7. [ ] **Negative:** Non-admin user on admin endpoint → `403 FORBIDDEN`
8. [ ] **Runtime unchanged:** Custom field GET/PUT still works with tenant header only (no admin middleware)
9. [ ] UI: non-`PLATFORM_ADMIN` cannot open `/low-code/admin/*` (middleware `low-code-admin`)
10. [ ] Rollback tested: flag `false` → admin open with tenant only (dev behavior)

See `docs/LOW_CODE_PERMISSIONS_ADMIN_GUARDRAILS_V0.1.md`.

## Data Safety Checklist

- [ ] **Backup DB** before pilot (full snapshot or tenant-scoped export)
- [ ] **Export templates** before structural changes:
  ```powershell
  curl.exe -H "X-Tenant-ID: {tenant}" `
    "http://localhost:8080/api/v1/low-code/admin/form-templates/b1111111-1111-4111-8111-111111111102/export" `
    -o backup-transport_order_default.json
  ```
- [ ] Confirm `make seed-demo-data` / `make seed-lowcode-demo` are **dev-only** — never run against production
- [ ] Demo entities (DEMO-TO-001, etc.) are not treated as production data
- [ ] Tenant isolation: all API calls use pilot `X-Tenant-ID`; never trust `source.tenant_id` from import files
- [ ] Migration/batch **execute** only on test entities in staging until go/no-go sign-off
- [ ] Import **execute** only by `PLATFORM_ADMIN`; creates DRAFT only — verify before manual publish

## API Smoke Checklist

**Prerequisites:** `make health-check`, `make seed-dev-admin`, `make seed-demo-data`, `make seed-lowcode-demo` (dev/staging test tenant only).

```powershell
cd D:\Projects\freight-platform
$T = "74519f22-ff9b-4a8b-8fff-a958c689682f"
$TO = "2db04b49-665c-469f-bcb1-ffeb1274fedb"   # DEMO-TO-001; resolve via API if IDs differ
$GW = "http://localhost:8080/api/v1"
```

### Platform smoke

```powershell
make health-check
make seed-lowcode-demo
make integration-smoke-test
```

### Low-code API smoke (non-destructive)

| # | Check | Command | Expected |
|---|-------|---------|----------|
| 1 | Active template | `curl.exe -H "X-Tenant-ID: $T" "$GW/low-code/form-templates/active?entity_type=TRANSPORT_ORDER&code=transport_order_default"` | 200, PUBLISHED |
| 2 | Public list no DRAFT | `curl.exe -H "X-Tenant-ID: $T" "$GW/low-code/form-templates?entity_type=TRANSPORT_ORDER"` | 200, all PUBLISHED |
| 3 | Custom values GET | `curl.exe -H "X-Tenant-ID: $T" "$GW/low-code/custom-field-values?entity_type=TRANSPORT_ORDER&entity_id=$TO"` | 200 |
| 4 | Custom values PUT (no context) | POST/PUT per `LOW_CODE_CUSTOM_FIELD_VALUES_API_V0.1.md` | 200 |
| 5 | Custom values PUT (with context) | Include `validation_context` in body | 200 |
| 6 | Audit events | `curl.exe -H "X-Tenant-ID: $T" "$GW/low-code/audit-events?entity_type=TRANSPORT_ORDER&limit=10"` | 200 |
| 7 | Migration preview | `curl.exe -X POST -H "Content-Type: application/json" -H "X-Tenant-ID: $T" --data-binary "@scripts/dev/payloads/lowcode_migration_preview_transport_order.json" "$GW/low-code/admin/custom-field-values/migration-preview"` | 200 |
| 8 | Batch migration preview | `curl.exe -X POST -H "Content-Type: application/json" -H "X-Tenant-ID: $T" --data-binary "@scripts/dev/payloads/lowcode_batch_migration_preview_transport_order.json" "$GW/low-code/admin/custom-field-values/batch-migration-preview"` | 200 |
| 9 | Template export | `curl.exe -H "X-Tenant-ID: $T" "$GW/low-code/admin/form-templates/b1111111-1111-4111-8111-111111111102/export"` | 200, `schema_version: lowcode.template.export.v1` |
| 10 | Import preview | `curl.exe -X POST -H "Content-Type: application/json" -H "X-Tenant-ID: $T" --data-binary "@scripts/dev/payloads/template-import-export-edge-cases/new_version_request.json" "$GW/low-code/admin/form-templates/import-preview"` | 200 or WARNING |

**Avoid on shared staging unless test tenant:**

- Migration **execute**
- Batch migration **execute**
- Import **execute**
- Publish DRAFT

### validation_context helper (automated)

```powershell
node scripts/dev/verify_lowcode_validation_context.mjs
```

Expected: **OK**

## Manual UI Checklist

**Login (dev/staging only):** http://localhost:3000/login  
`admin@7rights.local` / `Admin123456!`  
Tenant: `74519f22-ff9b-4a8b-8fff-a958c689682f`

| Page | Checks |
|------|--------|
| `/low-code` | Hub loads; service status online; admin links visible for platform admin |
| `/low-code/custom-field-values` | Entity lookup; migration preview modal opens |
| `/low-code/audit` | Events list; filters; migration/import categories |
| `/low-code/admin/form-templates` | List; Import wizard opens; filters work |
| `/low-code/admin/form-templates/[id]` | Metadata; export JSON; clone/publish (draft only) |
| `/transport-orders/[id]` | Custom fields panel; save; validation_context sent |
| `/shipments/[id]` | Panel renders; route context |
| `/billing-registers/[id]` | Panel renders; finance fields |

**Functional checks**

- [ ] Entity panels render without console errors
- [ ] Save custom values succeeds; toast confirmation
- [ ] Network tab: PUT includes `validation_context` when panel supports it
- [ ] Admin template detail: Export JSON downloads/copies; schema_version correct
- [ ] Import wizard: invalid JSON shows error; valid JSON preview works; execute creates DRAFT
- [ ] Migration preview modal: SAFE/WARNING/BLOCKED states; warnings checkbox before execute
- [ ] Batch migration wizard: preview step; max entity count respected in UI
- [ ] Audit page shows recent value/migration/import events
- [ ] Publish remains manual after import (no auto-publish)
- [ ] Non-admin user: no access to `/low-code/admin/*`

## Observability Checklist

- [ ] `make health-check` — all green
- [ ] `curl http://localhost:8088/health` — low-code-service OK
- [ ] `curl http://localhost:8088/metrics` — scrape OK; batch migration counters present
- [ ] Audit: value updates logged after custom field save
- [ ] Audit: `FORM_TEMPLATE_EXPORTED` after export
- [ ] Audit: `FORM_TEMPLATE_IMPORT_PREVIEWED` / `FORM_TEMPLATE_IMPORTED_AS_DRAFT` after import flow
- [ ] Logs: no raw `value_json` in application logs (policy)
- [ ] Metrics: no high-cardinality labels (`tenant_id`, `entity_id`, `batch_id` forbidden)

## Risk Register

| Risk | Severity | Mitigation | Owner / action |
|------|----------|------------|----------------|
| Auth-on not live-toggled in compose yet | Medium | Follow Auth-on Checklist; verify 401/403; rollback flag documented | Platform ops |
| No Vitest frontend automated tests | Low | Manual UI checklist + `npm run build`; pilot limited scope | Frontend lead |
| Batch jobs >100 not implemented | Medium | Enforce max 100 in UI/API; split large batches | Admin operators |
| Batch-level audit deferred | Low | Use entity-level audit + `batch_id` filter | Support / audit review |
| Import/export max fields = 200 (500 deferred) | Low | Validate large templates in preview; split sections if needed | Template admin |
| REPLACE_EXISTING_DRAFT without DRAFT → 400 | Medium | Use NEW_VERSION or clone-to-draft first; document in runbook | Template admin |
| Checksum mismatch = warning only | Low | Review warnings in import wizard before execute | Template admin |
| `validation_context` advisory only | Medium | Do not use for financial authorization; document policy | Product / compliance |
| No auto-rollback UI for migrated values | High | Preview mandatory; audit inspection; DBA rollback from backup | Ops + DBA |
| Template import never auto-publishes | Low | Explicit publish step; train admins | Template admin |
| Default-off admin open in dev | High (if mis-deployed) | Enable auth-on in staging/prod; verify middleware | Platform ops |
| Demo seed in production | Critical | Never run demo seeds in prod; separate tenant | Release manager |

## Rollback Plan

### Admin access issues

1. Set `LOW_CODE_ADMIN_AUTH_ENABLED=false` on `low-code-service`
2. Restart service
3. Confirm admin UI/API accessible with tenant header (staging) or restore auth config

### Template issues

1. Keep previous **PUBLISHED** template active (do not publish bad DRAFT)
2. Use **clone-to-draft** for safe edits from last good published version
3. **Re-export** good template before any import experiment
4. Bad import DRAFT: **do not publish**; discard via admin archive path when available, or leave unpublished
5. Never manually DELETE template rows in DB unless emergency DBA procedure

### Custom field / migration issues

1. **Do not** manually delete custom field values in DB
2. Inspect **audit events** (`entity_type`, `entity_id`, `batch_id`)
3. If batch migration partially completed: filter audit by `batch_id`; re-run preview before any retry
4. Restore from DB backup if widespread bad writes

### Import/export

1. Export current state before import
2. Import creates DRAFT only — rollback = do not publish imported DRAFT
3. Active published template unchanged by import execute

### Emergency

- DB restore from pre-pilot backup
- Disable low-code admin UI routes at gateway (last resort)
- No manual SQL unless approved DBA runbook

## Recommended Pilot Scope

### Phase 1 (recommended start)

| Dimension | Scope |
|-----------|-------|
| Entity type | **TRANSPORT_ORDER** only |
| Tenants | **1** pilot tenant |
| Template | **1** published code: `transport_order_default` |
| Users | Platform admin + shipper logist (runtime edit) |
| Operations | Custom values read/edit; migration **preview only** |
| Template import/export | **PLATFORM_ADMIN** only; staging test tenant |
| Batch execute | **Disabled** until Phase 1 sign-off |

### Phase 2 (after Phase 1 go/no-go)

| Dimension | Scope |
|-----------|-------|
| Entity types | SHIPMENT, BILLING_REGISTER |
| Operations | Single-entity migration execute with preview gate |
| Batch migration | Preview + execute, max 100 entities |
| Import/export | Between dev and staging for template promotion |

## Verification Commands

```powershell
cd D:\Projects\freight-platform

git status --short
git log --oneline -3

make health-check
make seed-lowcode-demo
make integration-smoke-test

cd apps\web-admin
npm run build
```

**Checklist verification date:** run above before pilot go/no-go meeting.

## Go / No-Go Criteria

### GO (all required)

- [ ] `make health-check` OK
- [ ] `make integration-smoke-test` PASSED
- [ ] `npm run build` PASSED (web-admin)
- [ ] Active template resolves for pilot entity type(s)
- [ ] Public API returns PUBLISHED only
- [ ] Custom field GET/PUT smoke OK on pilot test entity
- [ ] Audit events retrievable
- [ ] DB backup completed
- [ ] Auth-on verified **or** explicitly waived for dev-only pilot with signed risk acceptance
- [ ] Rollback plan communicated to ops
- [ ] Phase 1 scope agreed (tenant, entity type, users)

### NO-GO (any triggers hold)

- [ ] low-code-service unhealthy or migration not applied
- [ ] DRAFT visible in public runtime template API
- [ ] Custom field PUT fails on pilot test entity
- [ ] Auth-on enabled but admin UI broken for platform admin
- [ ] No DB backup before template/migration changes
- [ ] Production tenant used for destructive migration/import tests

## Next Action

**Low-code Staging Auth-On Verification Pack v0.1** — live toggle verification, 401/403 matrix, and rollback drill on staging.

Related: `LOW_CODE_STAGING_AUTH_ON_VERIFICATION_V0.1.md` (to be created in next pack).
