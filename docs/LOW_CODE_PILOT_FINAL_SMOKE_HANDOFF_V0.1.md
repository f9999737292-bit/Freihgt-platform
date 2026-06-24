# Low-code Pilot Final Smoke & Handoff v0.1

## Summary

Final automated smoke and handoff pack before pilot Day-1. All evidence documents are present. Automated checks (health, seed, integration smoke, npm build, go test, low-code API smoke) **passed** with no hard blockers.

**Decision: GO_WITH_CONDITIONS** — pilot may proceed on staging under narrow scope. Hand off to operator with runbook, checklist, and short handoff note.

**This is not a production launch.**

## Current Commit

| Field | Value |
|-------|-------|
| HEAD | `168e2f9` — `docs: add low-code pilot release readiness package` |
| Final smoke date | 2026-06-24 |
| Branch | `main` |
| Working tree at smoke | **clean** |
| Backend / frontend code changed in this pack | **no** |

## Decision

| Field | Value |
|-------|-------|
| **Recommended decision** | **GO_WITH_CONDITIONS** |
| Hard blockers found in final smoke | **None** |
| P0 remaining | **0** |
| Production rollout | **Not approved** |

## Evidence Documents

| Document | Status | Last commit |
|----------|--------|-------------|
| `LOW_CODE_PILOT_GO_NO_GO_REVIEW_V0.1.md` | **Found** | `54e57b9` |
| `LOW_CODE_PILOT_LAUNCH_RUNBOOK_V0.1.md` | **Found** | `54e57b9` |
| `LOW_CODE_PILOT_LAUNCH_REHEARSAL_V0.1.md` | **Found** | `4958db0` |
| `LOW_CODE_PILOT_RELEASE_PACKAGE_V0.1.md` | **Found** | `168e2f9` |
| `LOW_CODE_PILOT_OPERATOR_CHECKLIST_V0.1.md` | **Found** | `168e2f9` |
| `LOW_CODE_PILOT_RELEASE_NOTES_V0.1.md` | **Found** | `168e2f9` |
| `LOW_CODE_PILOT_MANUAL_UI_VERIFICATION_V0.1.md` | **Found** | `168e2f9` |
| `LOW_CODE_PILOT_FIX_POLISH_SPRINT_V0.1.md` | **Found** | `168e2f9` |
| `LOW_CODE_STAGING_AUTH_ON_VERIFICATION_V0.1.md` | **Found** | `9afb85c` |
| `LOW_CODE_RUNTIME_PILOT_STAGING_CHECKLIST_V0.1.md` | **Found** | `da5af8e` |
| `LOW_CODE_PERMISSIONS_MATRIX_V0.1.md` | **Found** | `c68412b` |

**Missing evidence docs:** none.

## Final Smoke Results

| Check | Result | Notes |
|-------|--------|-------|
| `make health-check` | **PASS** | All 9 services OK |
| `make seed-lowcode-demo` | **PASS** | 6 PUBLISHED templates; demo values present |
| `make integration-smoke-test` | **PASS** | Run ID `TEST-20260624145002` |
| `go test ./...` (low-code-service) | **PASS** | All packages OK (cached) |
| Working tree clean | **PASS** | No uncommitted code |

## API Smoke Results

Tenant: `74519f22-ff9b-4a8b-8fff-a958c689682f` (dev demo). Gateway: `http://localhost:8080/api/v1`.

| Endpoint | Method | HTTP | Result |
|----------|--------|------|--------|
| `/low-code/form-templates/active?entity_type=TRANSPORT_ORDER&template_code=transport_order_default` | GET | **200** | PASS |
| `/low-code/custom-field-values?entity_type=TRANSPORT_ORDER&entity_id=2db04b49-665c-469f-bcb1-ffeb1274fedb&template_code=transport_order_default` | GET | **200** | PASS |
| `/low-code/admin/form-templates` | GET | **200** | PASS |
| `/low-code/admin/form-templates/b1111111-1111-4111-8111-111111111102/export` | GET | **200** | PASS |
| `/low-code/admin/form-templates/import-preview` | POST | **200** | PASS (valid export payload) |
| `/low-code/admin/custom-field-values/migration-preview` | POST | **200** | PASS |
| `/low-code/audit-events?limit=10` | GET | **200** | PASS |

**No execute/import publish/batch execute** was run in this smoke (safe GET/preview only).

## UI Build Result

| Check | Result |
|-------|--------|
| `npm run build` (web-admin) | **PASS** |
| Client build | ~12s |
| Server build | ~12s |
| Blocking errors | **None** |
| Non-blocking | Node DEP0155 deprecation warning (Vue shared exports) |

## Pilot Scope

| Dimension | Phase 1 |
|-----------|---------|
| Tenants | **One** isolated pilot tenant |
| Entity type | **TRANSPORT_ORDER** first |
| Template code | `transport_order_default` |
| Admin operations | `PLATFORM_ADMIN` only (auth-on in staging) |
| Runtime users | Per permissions matrix |
| Batch execute | Only after **clean preview**; max **100** |
| Manual DB edits | **Prohibited** |
| Auto-publish from import | **Never** (DRAFT only) |

## Ready Capabilities

- Active template runtime (`GET .../form-templates/active`)
- Custom field values GET/PUT (tenant-scoped)
- `validation_context` on entity panels
- Entity panels (TO Phase 1; SH/BR Phase 2)
- Audit log (GET + UI)
- Admin template lifecycle (list, builder, clone, publish)
- Template import/export (preview + execute → DRAFT)
- Migration preview/execute (guarded)
- Batch migration preview/execute (guarded, max 100)
- Permissions matrix + UI guardrails
- Auth-on verified locally (401/403/200)
- Pilot runbook, operator checklist, release notes

## Restricted Capabilities

- No mobile driver app in this pilot
- No ЭТрН/ЭПД integration in this pilot
- No production batch >100
- No auto rollback UI
- No automatic template migration on publish
- No automatic import publish
- Batch execute deferred in Phase 1 until clean preview sign-off

## Required Staging Settings

On `low-code-service` only:

```yaml
LOW_CODE_ADMIN_AUTH_ENABLED: "true"
IDENTITY_SERVICE_URL: http://identity-service:8081
LOG_LEVEL: info
```

- Apply via deployment config or **gitignored** override — **do not commit** auth-on to tracked dev compose.
- Gateway must forward `X-User-ID` for admin UI sessions.
- Repeat auth-on verification on real staging (see `LOW_CODE_STAGING_AUTH_ON_VERIFICATION_V0.1.md`).

## Access and Roles

| Role | Admin UI/API | Runtime custom fields |
|------|--------------|------------------------|
| `PLATFORM_ADMIN` | **Yes** (auth-on) | Yes |
| `SHIPPER_LOGIST` / `SHIPPER_ADMIN` | **No** (403) | Yes (TO Phase 1) |
| `CARRIER_*`, `FINANCE_*` | **No** | Per matrix |
| `DRIVER` | **No** | No runtime edit in UI v0.1 |

Dev reference (replace with staging values):

| Item | Value |
|------|-------|
| Tenant | `74519f22-ff9b-4a8b-8fff-a958c689682f` |
| Admin login | `admin@7rights.local` / `Admin123456!` |
| Admin user ID | `8541a3a3-bde7-4fed-9501-37b9953bf904` |
| TO template ID | `b1111111-1111-4111-8111-111111111102` |
| Demo entity DEMO-TO-001 | `2db04b49-665c-469f-bcb1-ffeb1274fedb` |

## Operator Handoff

**Primary docs for operators:**

1. `LOW_CODE_PILOT_HANDOFF_NOTE_V0.1.md` — short 1–2 page summary
2. `LOW_CODE_PILOT_LAUNCH_RUNBOOK_V0.1.md` — step-by-step launch
3. `LOW_CODE_PILOT_OPERATOR_CHECKLIST_V0.1.md` — daily and change checklists
4. `LOW_CODE_PILOT_RELEASE_NOTES_V0.1.md` — what's included

**Launch sequence (staging):**

1. Deploy current `main` (`168e2f9` or later)
2. Enable auth-on on staging low-code-service (gitignored override)
3. Run staging auth verification curls (401/403/200)
4. Confirm one pilot tenant + `transport_order_default` PUBLISHED
5. Operator 15-min browser walkthrough (admin templates, custom values, audit)
6. Open pilot to limited users (TO only)

## Daily Checks

- `make health-check` — all services green
- Audit events review (writes, admin actions)
- low-code-service error logs (no repeated 5xx)
- Active template still `transport_order_default` PUBLISHED
- Spot-check 1–2 pilot entities — custom values correct
- No unapproved DRAFT published

See `LOW_CODE_PILOT_OPERATOR_CHECKLIST_V0.1.md`.

## Stop Conditions

Stop pilot and escalate immediately if:

- Admin endpoint accessible by non-admin (auth-on staging)
- Tenant isolation issue (cross-tenant data visible)
- Wrong entity/tenant writes
- Audit missing for write/admin operations
- `make health-check` failure (low-code-service down)
- Repeated low-code-service 5xx
- Import/export unexpected template version/status
- Migration execute outcome differs from preview expectation

## Rollback Summary

1. **Stop pilot access** — disable pilot users / revert feature flag if used
2. **Keep previous PUBLISHED template active** — do not publish bad DRAFT
3. **Do not publish bad DRAFT** — archive or delete after review
4. **Use audit** to inspect writes (`/low-code/audit-events`)
5. **DB rollback** only via backup/DBA procedure — no manual SQL edits unless emergency DBA process
6. **Restore auth-off** in dev/local if testing — staging keeps auth-on for pilot duration

Full procedure: `LOW_CODE_PILOT_LAUNCH_RUNBOOK_V0.1.md` → Rollback Procedure.

## Known Limitations

- Auth-on verified in **local Docker only** — must repeat on real staging
- Browser DevTools walkthrough not captured in agent session (operator sign-off pending)
- Non-admin UI login negative test deferred (API 403 evidence exists)
- SHIPMENT / BILLING_REGISTER entity panels ready but Phase 2 scope
- No automatic rollback UI
- Vitest unit tests not part of pilot gate

## Open Follow-ups

| Item | Owner | When |
|------|-------|------|
| Staging auth-on repeat verification | DevOps / Security | Before pilot users |
| Operator browser sign-off (15 min) | Pilot lead | Day 0 |
| Day-1 monitoring pack | Platform ops | After handoff |
| Phase 2 entity expansion (SH/BR) | Product | Post-pilot week 2+ |

## Final Recommendation

**GO_WITH_CONDITIONS** — proceed to pilot Day-1 on staging with:

- One tenant, TRANSPORT_ORDER, `transport_order_default`
- Auth-on enabled and verified on staging
- PLATFORM_ADMIN for admin ops
- Preview gates before any execute/publish
- Operator uses handoff note + runbook + checklist

No code changes required from this final smoke.

## Next Action

**Low-code Pilot Day-1 Monitoring Pack v0.1**

## Verification Commands

```powershell
cd D:\Projects\freight-platform
make health-check
make seed-lowcode-demo
make integration-smoke-test

cd apps\web-admin
npm run build

cd ..\..\services\low-code-service
go test ./...

# Low-code API smoke (safe GET/preview)
$T = "74519f22-ff9b-4a8b-8fff-a958c689682f"
curl.exe -H "X-Tenant-ID: $T" "http://localhost:8080/api/v1/low-code/form-templates/active?entity_type=TRANSPORT_ORDER&template_code=transport_order_default"
curl.exe -H "X-Tenant-ID: $T" "http://localhost:8080/api/v1/low-code/custom-field-values?entity_type=TRANSPORT_ORDER&entity_id=2db04b49-665c-469f-bcb1-ffeb1274fedb&template_code=transport_order_default"
curl.exe -H "X-Tenant-ID: $T" "http://localhost:8080/api/v1/low-code/admin/form-templates"
curl.exe -H "X-Tenant-ID: $T" "http://localhost:8080/api/v1/low-code/admin/form-templates/b1111111-1111-4111-8111-111111111102/export"
curl.exe -X POST -H "Content-Type: application/json" -H "X-Tenant-ID: $T" --data-binary "@scripts/dev/payloads/template-import-export-edge-cases/valid_transport_order_export_v1.json" http://localhost:8080/api/v1/low-code/admin/form-templates/import-preview
curl.exe -X POST -H "Content-Type: application/json" -H "X-Tenant-ID: $T" --data-binary "@scripts/dev/payloads/lowcode_migration_preview_transport_order.json" http://localhost:8080/api/v1/low-code/admin/custom-field-values/migration-preview
curl.exe -H "X-Tenant-ID: $T" "http://localhost:8080/api/v1/low-code/audit-events?limit=10"
```
