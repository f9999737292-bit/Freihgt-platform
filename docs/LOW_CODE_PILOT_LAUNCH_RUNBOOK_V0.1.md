# Low-code Pilot Launch Runbook v0.1

## Summary

Practical operator runbook for launching a **limited low-code staging/pilot**: one tenant, TRANSPORT_ORDER first, auth-on enabled in staging, default-off preserved in dev/local. Assumes **GO_WITH_CONDITIONS** decision from `LOW_CODE_PILOT_GO_NO_GO_REVIEW_V0.1.md`.

**Launch status:** **READY** (no hard blockers). If stop conditions trigger during launch, mark pilot **BLOCKED** and follow Rollback Procedure.

## Current Commit

| Field | Value |
|-------|-------|
| Baseline commit | `9afb85c` — `docs: add low-code staging auth verification` |
| Sprint commit | (this pack) — decision + runbook |
| Branch | `main` |
| Runbook date | 2026-06-24 |

## Launch Decision

| Field | Value |
|-------|-------|
| Go/No-Go decision | **GO_WITH_CONDITIONS** |
| Launch allowed | **Yes** — staging/pilot only |
| Production rollout | **No** |
| Hard blockers | **None** |
| Launch status | **READY** |

See `docs/LOW_CODE_PILOT_GO_NO_GO_REVIEW_V0.1.md` for full criteria matrix.

## Pilot Scope

| Dimension | Phase 1 scope |
|-----------|---------------|
| Tenants | **One** isolated pilot tenant |
| Entity type | **TRANSPORT_ORDER** only |
| Template code | `transport_order_default` |
| Admin operations | `PLATFORM_ADMIN` only |
| Runtime | Custom values read/edit per permissions matrix (UI-gated) |
| Batch execute | **Disabled** until clean preview + Phase 1 sign-off |
| Batch size | Max **100** entities (never exceed) |
| DB edits | **Prohibited** except emergency DBA procedure |
| Import | Creates **DRAFT only** — never auto-publishes |

### Dev reference values (do not use in production)

| Item | Dev demo value |
|------|----------------|
| Tenant ID | `74519f22-ff9b-4a8b-8fff-a958c689682f` |
| Platform admin | `admin@7rights.local` / `Admin123456!` |
| Admin user ID (curl) | `8541a3a3-bde7-4fed-9501-37b9953bf904` |
| Non-admin (negative test) | `shipper@7rights.local` → `008e1462-6f67-4246-b7dc-4aae1669c0c5` |
| Published TO template ID | `b1111111-1111-4111-8111-111111111102` |
| Demo entity DEMO-TO-001 | `2db04b49-665c-469f-bcb1-ffeb1274fedb` |

Replace with **staging pilot tenant** values before launch.

## Roles and Access

| Role | Admin `/low-code/admin/*` | Runtime custom fields |
|------|---------------------------|------------------------|
| `PLATFORM_ADMIN` | **Yes** (auth-on) | Yes (UI + API tenant-scoped) |
| `SHIPPER_LOGIST` / `SHIPPER_ADMIN` | **No** (403) | Yes (UI runtime edit) |
| `CARRIER_*`, `FINANCE_*`, etc. | **No** | Per entity type (Phase 1: TO only) |
| `DRIVER` | **No** | **No** runtime edit in UI v0.1 |
| Unauthenticated curl (default-off dev) | Open with tenant | Open with tenant |
| Unauthenticated curl (staging auth-on) | **401** admin; runtime unchanged | Tenant header only |

**Staging rule:** non-admin users **must not** access admin endpoints (403) or admin UI routes (`/low-code/admin/*`).

## Environment Setup

### Staging (pilot)

On `low-code-service` only:

```yaml
LOW_CODE_ADMIN_AUTH_ENABLED: "true"
IDENTITY_SERVICE_URL: http://identity-service:8081   # staging internal URL
LOG_LEVEL: info
```

Apply via deployment config or **gitignored** `docker-compose.override.yml` — **do not** commit auth-on to tracked dev compose.

Restart:

```powershell
# Example: recreate low-code-service with override
docker compose -f infrastructure/docker-compose/docker-compose.yml `
  -f infrastructure/docker-compose/docker-compose.override.yml up -d --no-build low-code-service
```

Verify gateway routes: `{gateway}/api/v1/low-code/*` → low-code-service.

Ensure web-admin / gateway forward **`X-User-ID`** from authenticated sessions for admin UI.

### Dev / local (unchanged)

Tracked compose keeps:

```yaml
LOW_CODE_ADMIN_AUTH_ENABLED: "false"
```

Default-off preserves smoke scripts and curl without `X-User-ID`.

## Pre-launch Checklist

- [ ] Go/No-Go review accepted: `LOW_CODE_PILOT_GO_NO_GO_REVIEW_V0.1.md`
- [ ] **Database backup** taken and verified (staging)
- [ ] Pilot **tenant ID** documented and isolated from production
- [ ] `PLATFORM_ADMIN` assigned for pilot admin user(s)
- [ ] Non-admin test user available for negative auth check
- [ ] Migrations applied: `make migrate-up` (staging equivalent)
- [ ] Auth-on env set on staging `low-code-service`
- [ ] `make health-check` — all services OK
- [ ] Auth-on curls: 401 (no user), 403 (non-admin), 200 (admin)
- [ ] Active template exists: `transport_order_default` PUBLISHED
- [ ] Export current template JSON **before any change**
- [ ] Rollback plan reviewed with ops/DBA
- [ ] Demo seeds **not** run on production

## Launch Steps

Execute in order. Stop and escalate if any step fails (see Stop Conditions).

| Step | Action | Pass criteria |
|------|--------|---------------|
| 1 | Verify DB backup | Backup ID recorded; restore tested or approved |
| 2 | Verify tenant ID | Pilot tenant documented; no prod tenant overlap |
| 3 | Enable auth-on in staging | `LOW_CODE_ADMIN_AUTH_ENABLED=true`; service restarted |
| 4 | Verify PLATFORM_ADMIN | Admin user has role; admin curl → 200 |
| 5 | Run health-check | All services OK including low-code |
| 6 | Run low-code API smoke | See API Smoke Commands — runtime + admin (with headers) |
| 7 | Export current template | JSON saved; `schema_version: lowcode.template.export.v1` |
| 8 | Verify active template | `GET .../active` → 200, `is_active: true`, PUBLISHED |
| 9 | Verify TO custom fields | GET values for pilot entity → 200; UI panel loads |
| 10 | Verify audit events | GET audit → 200; value events visible after test save |
| 11 | Start limited pilot | Enable pilot users; monitor daily checklist |

Optional automated checks:

```powershell
make integration-smoke-test          # platform baseline
node scripts/dev/verify_lowcode_validation_context.mjs
```

## API Smoke Commands

Replace `$T`, `$GW`, `$ADMIN`, `$TO` with staging values.

```powershell
cd D:\Projects\freight-platform
$T = "{pilot_tenant_id}"
$GW = "http://{gateway}/api/v1"
$ADMIN = "{platform_admin_user_id}"
$TO = "{pilot_transport_order_id}"
$SHIPPER = "{non_admin_user_id}"
$TPL = "{published_template_id}"
```

### Runtime (no admin guard — tenant only)

```powershell
# Active template
curl.exe -s -o NUL -w "active %{http_code}`n" `
  -H "X-Tenant-ID: $T" `
  "$GW/low-code/form-templates/active?entity_type=TRANSPORT_ORDER&template_code=transport_order_default"

# Custom values GET
curl.exe -s -o NUL -w "values GET %{http_code}`n" `
  -H "X-Tenant-ID: $T" `
  "$GW/low-code/custom-field-values?entity_type=TRANSPORT_ORDER&entity_id=$TO"

# Audit events
curl.exe -s -o NUL -w "audit %{http_code}`n" `
  -H "X-Tenant-ID: $T" `
  "$GW/low-code/audit-events?entity_type=TRANSPORT_ORDER&limit=10"
```

### Admin auth-on (staging)

```powershell
# Negative: no user → 401
curl.exe -s -o NUL -w "admin no-user %{http_code}`n" `
  -H "X-Tenant-ID: $T" `
  "$GW/low-code/admin/form-templates"

# Negative: non-admin → 403
curl.exe -s -o NUL -w "admin shipper %{http_code}`n" `
  -H "X-Tenant-ID: $T" -H "X-User-ID: $SHIPPER" `
  "$GW/low-code/admin/form-templates"

# Positive: PLATFORM_ADMIN → 200
curl.exe -s -o NUL -w "admin platform %{http_code}`n" `
  -H "X-Tenant-ID: $T" -H "X-User-ID: $ADMIN" `
  "$GW/low-code/admin/form-templates"

# Export (admin)
curl.exe -s -o NUL -w "export %{http_code}`n" `
  -H "X-Tenant-ID: $T" -H "X-User-ID: $ADMIN" `
  "$GW/low-code/admin/form-templates/$TPL/export"

# Migration preview (admin, read-only)
curl.exe -s -o NUL -w "migration-preview %{http_code}`n" `
  -X POST -H "Content-Type: application/json" `
  -H "X-Tenant-ID: $T" -H "X-User-ID: $ADMIN" `
  --data-binary "@scripts/dev/payloads/migration-edge-cases/safe_transport_order.json" `
  "$GW/low-code/admin/custom-field-values/migration-preview"

# Import preview (admin, read-only)
curl.exe -s -o NUL -w "import-preview %{http_code}`n" `
  -X POST -H "Content-Type: application/json" `
  -H "X-Tenant-ID: $T" -H "X-User-ID: $ADMIN" `
  --data-binary "@scripts/dev/payloads/template-import-export-edge-cases/new_version_request.json" `
  "$GW/low-code/admin/form-templates/import-preview"
```

**Expected (auth-on):** runtime → 200; admin no-user → 401; non-admin → 403; admin → 200.

Full auth verification reference: `docs/LOW_CODE_STAGING_AUTH_ON_VERIFICATION_V0.1.md`.

## Manual UI Checks

Login: `{web-admin}/login` as PLATFORM_ADMIN for pilot tenant.

| # | Page | Check |
|---|------|-------|
| 1 | `/transport-orders/{id}` | Custom fields panel loads; save succeeds |
| 2 | Network tab on save | PUT includes `validation_context` |
| 3 | `/low-code/audit` | Recent value update event after save |
| 4 | `/low-code/admin/form-templates` | List loads; Import wizard opens |
| 5 | Template detail | Export JSON works; checksum present |
| 6 | Import wizard | Preview shows warnings/errors; execute → DRAFT only |
| 7 | Non-admin login | `/low-code/admin/*` blocked (UI + 403 API) |
| 8 | `/low-code` hub | Service status online |

Phase 1: **do not** test SHIPMENT/BILLING_REGISTER panels in pilot unless explicitly expanded.

## Import/Export Procedure

### Before any template change

1. **Export** current published template (admin, auth-on).
2. Store JSON with date + operator name.
3. Confirm `schema_version: lowcode.template.export.v1`.

### Import (staging test tenant only)

1. Open Import wizard or POST `import-preview`.
2. Review preview: errors block; warnings require acknowledgment.
3. Checksum mismatch → **WARNING** — confirm before execute.
4. Execute import → creates **DRAFT only**.
5. Review DRAFT in admin editor.
6. **Do not publish** until template review complete.
7. Active published template **unchanged** until explicit publish.

### Restrictions

- Max **200 fields**, 50 sections, 512 KB payload.
- `REPLACE_EXISTING_DRAFT` requires existing DRAFT — use `NEW_VERSION` or clone-to-draft first.
- No custom values import; no auto-publish.

See `docs/LOW_CODE_TEMPLATE_IMPORT_EXPORT_HARDENING_V0.1.md`.

## Migration Procedure

### Single-entity (Phase 1: preview preferred)

1. Identify entity IDs and target template code.
2. POST `migration-preview` (admin, auth-on).
3. Review per-entity status: SAFE / WARNING / BLOCKED.
4. **Execute only if SAFE** or warnings explicitly accepted.
5. Verify audit event for migration.
6. Re-read custom values on entity.

### Batch (Phase 2 or after Phase 1 sign-off)

1. POST `batch-migration-preview` — max **100** entity IDs.
2. Review summary: safe / warning / blocked counts.
3. **Do not execute** unless preview is clean and approved.
4. Filter audit by `batch_id` after execute.

**Phase 1 policy:** batch **execute disabled** — preview only.

## Audit Verification

After pilot writes, confirm:

```powershell
curl.exe -H "X-Tenant-ID: $T" `
  "$GW/low-code/audit-events?entity_type=TRANSPORT_ORDER&entity_id=$TO&limit=20"
```

| Event kind | When expected |
|------------|---------------|
| Value update | After custom field save |
| Migration | After migration execute |
| `FORM_TEMPLATE_EXPORTED` | After export |
| `FORM_TEMPLATE_IMPORT_*` | After import preview/execute |

Batch-level audit row deferred — use entity events + `batch_id` metadata filter.

## Monitoring Checklist

| Signal | Command / location | Frequency |
|--------|-------------------|-----------|
| Platform health | `make health-check` | Daily + after deploys |
| Low-code health | `GET /health` on low-code-service | Daily |
| Metrics | `GET /metrics` — batch counters, no tenant_id labels | Daily scrape |
| Audit | Admin audit page or API | After writes + daily review |
| Logs | low-code-service logs — no raw `value_json` | Daily error scan |
| 5xx rate | Gateway / service logs | Continuous during pilot |

Alert on: repeated 5xx, health-check failure, missing audit after writes.

## Rollback Procedure

| Trigger | Action |
|---------|--------|
| Admin auth blocks all operators (approved) | Set `LOW_CODE_ADMIN_AUTH_ENABLED=false`; restart `low-code-service` |
| Bad template published | Do **not** use bad version; keep previous PUBLISHED active; clone-to-draft from export |
| Bad import DRAFT | Do **not** publish; leave DRAFT unpublished |
| Bad migration writes | Audit inspect by `entity_id` / `batch_id`; no manual DELETE |
| Widespread bad data | **DB restore** from pre-pilot backup (DBA only) |
| Disable pilot access | Remove pilot user roles / block admin routes at gateway (last resort) |
| Emergency | DBA backup restore; escalate to platform ops |

**Never:** manual SQL on `lowcode.*` except approved DBA emergency process.

## Stop Conditions

**Stop pilot immediately** if any occur:

| # | Condition |
|---|-----------|
| 1 | Admin endpoint accessible by non-admin (403 expected, got 200) |
| 2 | Tenant isolation failure (cross-tenant read/write) |
| 3 | Custom values written to wrong entity or tenant |
| 4 | Migration execute outcome differs materially from preview |
| 5 | Audit missing for confirmed writes |
| 6 | `make health-check` failure (any service) |
| 7 | Repeated 5xx from low-code-service |
| 8 | Template import creates unexpected DRAFT/version or touches published template |

On stop: mark pilot **BLOCKED** → Rollback Procedure → open **Low-code Runtime Pilot Fix Pack v0.1**.

## Daily Pilot Checklist

Run each business day during pilot:

- [ ] `make health-check` — all green
- [ ] Review audit events (value updates, admin actions)
- [ ] Scan low-code-service error logs
- [ ] Confirm low-code `/health` OK
- [ ] Verify active template still `transport_order_default` PUBLISHED
- [ ] Spot-check recent custom values on 1–2 pilot entities
- [ ] Review admin changes (exports, imports, DRAFTs — none published without approval)
- [ ] Note issues/blockers in pilot log

## Post-launch Review

After Phase 1 window (recommended: 1–2 weeks):

| Question | Record |
|----------|--------|
| Auth-on stable in staging? | Yes/No |
| Any stop conditions triggered? | List |
| Custom field saves reliable? | Yes/No |
| Audit complete for writes? | Yes/No |
| Template changes needed? | Export/import used? |
| Expand to SHIPMENT/BILLING_REGISTER? | Go/No-Go Phase 2 |
| Production promotion? | **Defer** unless explicit new review |

Deliverable: short pilot retrospective → update Go/No-Go or Fix Pack.

## Next Action

**Low-code Pilot Launch Rehearsal Pack v0.1** — dry-run launch steps on staging before live pilot users.

If launch blocked → **Low-code Runtime Pilot Fix Pack v0.1**.
