# Low-code Pilot Go/No-Go Review v0.1

## Summary

Final go/no-go decision for a **limited pilot/staging** launch of the low-code runtime layer. Evidence spans runtime readiness, staging checklist, live auth-on verification, permissions matrix, entity integration, template import/export hardening, batch migration, audit/metrics, and automated verification on 2026-06-24.

**Decision: GO_WITH_CONDITIONS** — no hard blockers. Pilot may proceed under narrow scope (single tenant, TRANSPORT_ORDER first) with auth-on enabled in staging, mandatory preview gates, and documented rollback.

## Current Commit

| Field | Value |
|-------|-------|
| Commit | `9afb85c` |
| Message | `docs: add low-code staging auth verification` |
| Branch | `main` |
| Review date | 2026-06-24 |
| Working tree at review | clean |

## Decision

| Field | Value |
|-------|-------|
| **Recommended decision** | **GO_WITH_CONDITIONS** |
| Hard blockers | **None** |
| Production rollout | **Not recommended** — staging/pilot only |
| Default-off dev compose | **Unchanged** — remains `LOW_CODE_ADMIN_AUTH_ENABLED=false` |

## Evidence Reviewed

| Document | Role in decision |
|----------|------------------|
| `LOW_CODE_RUNTIME_PILOT_READINESS_V0.1.md` | Runtime API, entity panels, validation_context, migration preview, observability baseline |
| `LOW_CODE_RUNTIME_PILOT_STAGING_CHECKLIST_V0.1.md` | Staging env, risk register, phased scope, rollback plan |
| `LOW_CODE_STAGING_AUTH_ON_VERIFICATION_V0.1.md` | Live auth-on: 401/403/200 verified; runtime unchanged; default-off restored |
| `LOW_CODE_PERMISSIONS_MATRIX_V0.1.md` | Roles × actions; admin vs runtime enforcement model |
| `LOW_CODE_ENTITY_INTEGRATION_V0.2.md` | validation_context wiring on TO/SH/BR panels |
| `LOW_CODE_TEMPLATE_IMPORT_EXPORT_HARDENING_V0.1.md` | Import/export limits, checksum, no auto-publish |
| `NEXT_COMMANDS.md` | Pack sequencing and verification commands |

Supporting references: `LOW_CODE_PERMISSIONS_ADMIN_GUARDRAILS_V0.1.md`, `LOW_CODE_BATCH_MIGRATION_HARDENING_V0.1.md`, `LOW_CODE_AUDIT_LOG_V0.1.md`, `LOW_CODE_RUNTIME_READINESS_REVIEW_V0.1.md`.

## Go/No-Go Criteria

| # | Criterion | Status | Evidence |
|---|-----------|--------|----------|
| 1 | Runtime API compatibility | **GO** | Gateway smoke 200; integration smoke passed (TEST-20260624130646) |
| 2 | Auth default-off compatibility | **GO** | Compose default `false`; admin + runtime without `X-User-ID` → 200 |
| 3 | Auth-on admin guard | **GO_WITH_CONDITIONS** | Local Docker verified 401/403/200; **must repeat in real staging** |
| 4 | PLATFORM_ADMIN access | **GO** | Admin endpoints 200 with `8541a3a3-bde7-4fed-9501-37b9953bf904` |
| 5 | Non-admin rejection | **GO** | SHIPPER_LOGIST → 403 on admin routes |
| 6 | Runtime public/read endpoints | **GO** | Active template, public list/detail, custom values GET — 200 with auth-on |
| 7 | Entity panels readiness | **GO** | TO/SH/BR wired; unavailable fallback; build OK |
| 8 | validation_context | **GO** | v0.2 wiring; PUT backward compatible; advisory-only policy |
| 9 | Template lifecycle | **GO** | Public API PUBLISHED only; clone-to-draft; explicit publish |
| 10 | Import/export safety | **GO_WITH_CONDITIONS** | Hardening + DRAFT-only import; max 200 fields; export before changes |
| 11 | Migration safety | **GO_WITH_CONDITIONS** | Preview mandatory; execute only after clean preview |
| 12 | Batch migration safety | **GO_WITH_CONDITIONS** | Max 100; preview gate; batch execute deferred in Phase 1 |
| 13 | Audit readiness | **GO** | Value/migration/import events; entity-level audit with `batch_id` |
| 14 | Metrics/logs readiness | **GO** | `/metrics` OK; bounded labels; batch counters present |
| 15 | Permissions matrix | **GO** | Documented; UI gates + admin API guard when auth-on |
| 16 | Staging checklist | **GO** | Complete operator checklist available |
| 17 | Rollback plan | **GO** | Auth flag, template, migration, import, DB backup documented |
| 18 | Known limitations | **GO_WITH_CONDITIONS** | Documented below; accepted for narrow pilot |

**Totals:** 11 GO / 7 GO_WITH_CONDITIONS / 0 NO_GO

## Pilot Scope

### Recommended narrow pilot (Phase 1)

| Dimension | Scope |
|-----------|-------|
| **Tenant** | One isolated pilot tenant only (not production data) |
| **Entity type** | **TRANSPORT_ORDER** first |
| **Template code** | `transport_order_default` (single published active template) |
| **Admin roles** | `PLATFORM_ADMIN` only — template admin, import/export, migration preview/execute |
| **Runtime edit roles** | Per permissions matrix: `SHIPPER_LOGIST`, `SHIPPER_ADMIN`, etc.; **not** `DRIVER` in UI v0.1 |

### Features enabled (Phase 1)

- Active template read (`GET .../form-templates/active`)
- Custom field values read/edit (`GET/PUT .../custom-field-values`)
- `validation_context` on entity detail save
- Audit events read (`GET .../audit-events`)
- Template export (admin, auth-on)
- Import preview (admin, auth-on)
- Import execute → **DRAFT only** on pilot test tenant
- Single-entity migration **preview**
- Clone published → DRAFT for safe edits

### Features restricted (Phase 1)

- **Batch migration execute** — disabled until Phase 1 sign-off and clean batch preview
- **Import auto-publish** — never (import creates DRAFT only)
- **Batch size** — max 100 entities (API + UI enforced)
- **Manual DB edits** — prohibited except emergency DBA procedure
- **Demo seeds** — never in production
- **Multi-entity-type rollout** — defer SHIPMENT/BILLING_REGISTER to Phase 2

### Phase 2 (after Phase 1 sign-off)

- SHIPMENT and BILLING_REGISTER entity panels
- Single-entity migration execute with preview gate
- Batch migration preview + execute (max 100)
- Template promotion between environments via export/import

## Required Conditions

Before pilot go-live in staging:

1. Set `LOW_CODE_ADMIN_AUTH_ENABLED=true` on staging `low-code-service` (not in tracked dev compose)
2. Set `IDENTITY_SERVICE_URL` to staging identity service
3. Verify pilot `PLATFORM_ADMIN` user exists and has role for pilot tenant
4. Run `make health-check` + `make integration-smoke-test` on staging
5. Re-run auth-on negative/positive curls (401 without user, 403 non-admin, 200 admin)
6. Start with **TRANSPORT_ORDER only** — one tenant, one template code
7. **Export templates** before any import or publish change
8. Run migration/batch **preview** before any execute; do not execute batch unless preview is clean
9. Monitor audit events, `/metrics`, and service logs during pilot
10. Accept rollback plan (auth flag off, no publish bad DRAFT, DB backup)
11. Confirm demo seeds are **not** run against production
12. Ensure gateway/web-admin forwards `X-User-ID` for admin UI sessions

## Blockers

### Hard blockers

**None.** All automated verification passed; auth-on behavior verified locally; no open defects blocking a controlled staging pilot.

### Conditional items (not blockers)

| Item | Impact | Mitigation |
|------|--------|------------|
| Auth-on must be toggled and verified in **real staging** | Medium | Repeat verification pack steps; rollback flag documented |
| No frontend Vitest coverage | Low | Manual UI checklist + `npm run build` + verification script |
| Runtime PUT not admin-guarded (tenant-scoped by design) | Medium | Accept for pilot; UI gates operator roles; future Runtime Write RBAC pack |
| Batch-level audit deferred | Low | Entity-level audit + `batch_id` filter |
| Import/export max 200 fields (500 deferred) | Low | Preview large templates; split sections |
| `REPLACE_EXISTING_DRAFT` requires existing DRAFT | Medium | Use `NEW_VERSION` or clone-to-draft first |
| Checksum mismatch = warning only on import preview | Low | Review warnings before execute |
| No dedicated DRIVER user in dev demo seed | Low | Create driver user for staging negative tests |
| `validation_context` advisory only — not financial auth | Medium | Policy: do not use for billing/UPD gates |

## Known Limitations

- Service-level RBAC on runtime GET/PUT **not implemented** — tenant header only at API; UI gates runtime write
- FR/DOCUMENT/RFX entity panels lack v0.2 `validationContext` — out of Phase 1 scope
- Core BFF does not forward `validation_context` server-side — frontend sidecar pattern sufficient for pilot
- Client `validation_context.role` is not a trust boundary — soft validation only
- Default-off admin routes open in dev — **must not** deploy staging without auth-on
- Import never auto-publishes — explicit publish step required
- No auto-rollback UI for migrated values — audit + DB backup required
- Batch execute at scale capped at 100 — split large batches
- Vitest/component tests not present for low-code UI

## Operational Guardrails

| Area | Guardrail |
|------|-----------|
| Auth | Auth-on in staging; default-off in dev compose only |
| Templates | Export before change; clone-to-draft; no accidental DRAFT publish |
| Import | Preview → review warnings → execute to DRAFT → manual publish |
| Migration | Preview mandatory; SAFE/warnings acknowledged before execute |
| Batch | Max 100; preview gate; Phase 1 = preview only |
| Data | No manual SQL; no demo seed in prod; tenant isolation via `X-Tenant-ID` |
| Observability | Health check daily; audit review; metrics scrape; no `value_json` in logs |
| Financial | Do not use low-code `validation_context` for UPD/billing authorization |

## Rollback Plan

| Scenario | Action |
|----------|--------|
| Admin auth blocks operators | `LOW_CODE_ADMIN_AUTH_ENABLED=false` → restart `low-code-service` |
| Bad template published | Keep previous PUBLISHED active; clone-to-draft from last good version |
| Bad import DRAFT | Do not publish; discard or leave unpublished |
| Bad migration execute | Audit inspect by `entity_id` / `batch_id`; DB restore if widespread |
| Low-code service down | Entity panels show unavailable state; core entities unaffected |
| Emergency | DB restore from pre-pilot backup; gateway block admin routes (last resort) |

Full detail: `LOW_CODE_RUNTIME_PILOT_STAGING_CHECKLIST_V0.1.md` → Rollback Plan section.

## Final Recommendation

**Proceed with a controlled staging pilot under GO_WITH_CONDITIONS.**

The low-code runtime layer is sufficiently mature for a **narrow, monitored pilot**: one tenant, TRANSPORT_ORDER, auth-on enabled in staging, PLATFORM_ADMIN for admin operations, preview-before-execute discipline, and accepted known limitations. Do not promote to production-wide rollout until Phase 1 sign-off and Phase 2 scope review.

If staging finds regressions → **Low-code Runtime Pilot Fix Pack v0.1**.  
If Phase 1 succeeds → **Low-code Pilot Launch Runbook Pack v0.1**.

## Verification Commands

Pre-flight and evidence (2026-06-24):

```powershell
cd D:\Projects\freight-platform

git status --short          # clean
git log --oneline -15       # 9afb85c at HEAD

make health-check           # OK
make seed-lowcode-demo      # OK
make integration-smoke-test # PASSED (TEST-20260624130646)

cd apps\web-admin
npm run build               # OK

cd ..\..
node scripts/dev/verify_lowcode_validation_context.mjs  # optional
```

Auth-on reference (staging repeat):

```powershell
# See docs/LOW_CODE_STAGING_AUTH_ON_VERIFICATION_V0.1.md
curl.exe -i -H "X-Tenant-ID: {tenant}" `
  "http://{gateway}/api/v1/low-code/admin/form-templates"
# Expect 401 without X-User-ID when auth-on
```

## Next Action

**Low-code Pilot Launch Runbook Pack v0.1** — operational runbook for staging pilot execution (env setup, day-0 checklist, monitoring, escalation).
