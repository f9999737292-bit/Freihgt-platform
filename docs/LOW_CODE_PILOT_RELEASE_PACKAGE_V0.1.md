# Low-code Pilot Release Package v0.1

## Summary

Final **release package** for a limited low-code staging/pilot launch. Consolidates evidence from readiness reviews, auth verification, go/no-go decision, launch runbook, and launch rehearsal into one operator handoff document.

**Release decision: GO_WITH_CONDITIONS** — no hard blockers. Pilot may proceed on **one tenant**, **TRANSPORT_ORDER first**, with auth-on in staging and documented guardrails.

**This is not a production launch.** Use for staging/pilot handoff only.

## Release Decision

| Field | Value |
|-------|-------|
| Decision | **GO_WITH_CONDITIONS** |
| Hard blockers | **None** |
| Launch rehearsal | **PASS** (`LOW_CODE_PILOT_LAUNCH_REHEARSAL_V0.1.md`) |
| Production rollout | **Not approved** |
| Package date | 2026-06-24 |

## Release Scope

| Dimension | Phase 1 pilot |
|-----------|---------------|
| Tenants | **One** isolated pilot tenant |
| Entity type | **TRANSPORT_ORDER** first |
| Template code | `transport_order_default` |
| Admin operations | `PLATFORM_ADMIN` only (auth-on in staging) |
| Runtime users | Per permissions matrix (UI-gated) |
| Batch execute | Only after **clean preview**; max **100** entities |
| DB edits | **Prohibited** except emergency DBA |
| Auto-publish | **Never** (import → DRAFT only) |

## Current Commit

| Field | Value |
|-------|-------|
| Package baseline | `4958db0` — `docs: add low-code pilot launch rehearsal` |
| Evidence chain | `54e57b9` (runbook) → `40ccf4c` (go/no-go) → `9afb85c` (auth-on) |
| Branch | `main` |
| Verification run | 2026-06-24 — smoke TEST-20260624141152 |

## Evidence Matrix

| Area | Doc | Status | Key result | Blocker |
|------|-----|--------|------------|---------|
| Runtime readiness | `LOW_CODE_RUNTIME_PILOT_READINESS_V0.1.md` | Ready | API smoke 200; entity panels wired | No |
| Staging checklist | `LOW_CODE_RUNTIME_PILOT_STAGING_CHECKLIST_V0.1.md` | Ready | Env, risk register, rollback | No |
| Auth-on verification | `LOW_CODE_STAGING_AUTH_ON_VERIFICATION_V0.1.md` | Ready | 401/403/200; default-off restored | No* |
| Pilot Go/No-Go | `LOW_CODE_PILOT_GO_NO_GO_REVIEW_V0.1.md` | Ready | GO_WITH_CONDITIONS | No |
| Launch runbook | `LOW_CODE_PILOT_LAUNCH_RUNBOOK_V0.1.md` | Ready | Step-by-step launch | No |
| Launch rehearsal | `LOW_CODE_PILOT_LAUNCH_REHEARSAL_V0.1.md` | Ready | All safe checks PASS | No |
| Import/export hardening | `LOW_CODE_TEMPLATE_IMPORT_EXPORT_HARDENING_V0.1.md` | Ready | Limits, checksum, DRAFT-only | No |
| Permissions matrix | `LOW_CODE_PERMISSIONS_MATRIX_V0.1.md` | Ready | Roles × actions documented | No |
| Entity integration | `LOW_CODE_ENTITY_INTEGRATION_V0.2.md` | Ready | validation_context on TO/SH/BR | No |
| Audit/metrics | `LOW_CODE_AUDIT_LOG_V0.1.md`, batch hardening | Ready | Audit GET; bounded metrics | No |

\* Auth-on must be **repeated on real staging** deploy (local Docker verified only).

## Included Capabilities

### Runtime

- Active template read (`GET .../form-templates/active`)
- Custom field values GET/PUT (tenant-scoped)
- `validation_context` on entity detail save
- Entity panels: TRANSPORT_ORDER (Phase 1); SHIPMENT/BILLING_REGISTER (Phase 2)
- Public PUBLISHED templates only (no DRAFT in runtime list)
- Conditional required validation

### Admin (PLATFORM_ADMIN, auth-on in staging)

- Form template admin list/detail/builder
- Clone published → DRAFT
- Publish with review/diff
- Template export (portable JSON v1)
- Import preview + execute → **DRAFT only**
- Single-entity migration preview/execute
- Batch migration preview/execute (max 100, preview gate)

### Security & ops

- Permissions matrix + UI guardrails
- Admin guard when `LOW_CODE_ADMIN_AUTH_ENABLED=true`
- Audit events (value, migration, import/export)
- Health-check, metrics, staging checklist, launch runbook

## Excluded Capabilities

| Item | Notes |
|------|-------|
| Mobile driver app | Out of pilot scope |
| ЭТрН / ЭПД integration | Out of pilot scope |
| Production-grade frontend test automation (Vitest) | Manual UI verification recommended |
| Batch > 100 entities | API + UI enforced cap |
| Batch-level audit row | Entity-level + `batch_id` filter |
| Auto rollback UI for migrated values | Audit + DB backup |
| Automatic template migration on publish | Manual preview/execute |
| Automatic import publish | Explicit publish step required |
| Manual DB edits | Emergency DBA only |
| Production-wide rollout | Staging/pilot only |

## Required Staging Settings

On **staging** `low-code-service` only (not tracked dev compose):

```yaml
LOW_CODE_ADMIN_AUTH_ENABLED: "true"
IDENTITY_SERVICE_URL: http://identity-service:8081
```

**Dev/local** (unchanged):

```yaml
LOW_CODE_ADMIN_AUTH_ENABLED: "false"
```

Gateway/web-admin must forward `X-User-ID` for admin sessions.

## Pilot Tenant and Entity Scope

### Dev reference (do not use in production)

| Item | Value |
|------|-------|
| Tenant ID | `74519f22-ff9b-4a8b-8fff-a958c689682f` |
| Template ID (TO) | `b1111111-1111-4111-8111-111111111102` |
| Demo entity DEMO-TO-001 | `2db04b49-665c-469f-bcb1-ffeb1274fedb` |
| Platform admin | `admin@7rights.local` / `8541a3a3-bde7-4fed-9501-37b9953bf904` |

Replace with **staging pilot tenant** values before go-live.

## Roles and Access

| Role | Admin low-code | Runtime custom fields |
|------|----------------|------------------------|
| `PLATFORM_ADMIN` | Yes (auth-on) | Yes |
| `SHIPPER_LOGIST`, operator roles | No (403) | Yes (UI) |
| `DRIVER` | No | No runtime edit (UI v0.1) |
| Unauthenticated curl (default-off dev) | Open with tenant | Open with tenant |

See `LOW_CODE_PERMISSIONS_MATRIX_V0.1.md`.

## Launch Checklist

- [ ] Read `LOW_CODE_PILOT_RELEASE_NOTES_V0.1.md` (management summary)
- [ ] DB backup verified (staging)
- [ ] Pilot tenant documented and isolated
- [ ] `PLATFORM_ADMIN` assigned for pilot admin
- [ ] Auth-on enabled on staging `low-code-service`
- [ ] Auth curls: 401 / 403 / 200 verified on staging
- [ ] `make health-check` — all OK
- [ ] Export current template before any change
- [ ] Manual UI verification (`LOW_CODE_PILOT_OPERATOR_CHECKLIST_V0.1.md`)
- [ ] Rollback plan accepted by ops/DBA
- [ ] Demo seeds **not** run on production

Full steps: `LOW_CODE_PILOT_LAUNCH_RUNBOOK_V0.1.md`.

## API Smoke Checklist

```powershell
cd D:\Projects\freight-platform
$T = "{pilot_tenant_id}"
$GW = "http://{gateway}/api/v1"
$ADMIN = "{platform_admin_user_id}"
$TPL = "{published_template_id}"
$TO = "{pilot_transport_order_id}"

# Runtime (tenant only)
curl.exe -H "X-Tenant-ID: $T" "$GW/low-code/form-templates/active?entity_type=TRANSPORT_ORDER&template_code=transport_order_default"
curl.exe -H "X-Tenant-ID: $T" "$GW/low-code/custom-field-values?entity_type=TRANSPORT_ORDER&entity_id=$TO"
curl.exe -H "X-Tenant-ID: $T" "$GW/low-code/audit-events?entity_type=TRANSPORT_ORDER&limit=10"

# Admin (auth-on: add -H "X-User-ID: $ADMIN")
curl.exe -H "X-Tenant-ID: $T" -H "X-User-ID: $ADMIN" "$GW/low-code/admin/form-templates"
curl.exe -H "X-Tenant-ID: $T" -H "X-User-ID: $ADMIN" "$GW/low-code/admin/form-templates/$TPL/export"
```

Preview-only (safe):

```powershell
curl.exe -X POST -H "Content-Type: application/json" -H "X-Tenant-ID: $T" -H "X-User-ID: $ADMIN" `
  --data-binary "@scripts/dev/payloads/lowcode_migration_preview_transport_order.json" `
  "$GW/low-code/admin/custom-field-values/migration-preview"
```

## Manual UI Checklist

**Status:** Not completed in automated rehearsal — **required before pilot users**.

| Page | Check |
|------|-------|
| `/low-code` | Hub loads |
| `/low-code/custom-field-values` | Entity lookup |
| `/low-code/audit` | Events visible |
| `/low-code/admin/form-templates` | List, import wizard |
| `/transport-orders/{id}` | Custom fields panel save |
| Non-admin | Blocked from `/low-code/admin/*` |

Next pack: **Low-code Pilot Manual UI Verification Pack v0.1**.

Login (dev): `admin@7rights.local` / `Admin123456!`

## Security Checklist

- [ ] Auth-on: admin 401 without user, 403 non-admin, 200 PLATFORM_ADMIN
- [ ] Runtime endpoints not admin-guarded (by design — document accepted)
- [ ] Tenant isolation via `X-Tenant-ID`
- [ ] Import ignores `source.tenant_id` for writes
- [ ] Export: no custom values, no audit logs in JSON
- [ ] No auto-publish on import
- [ ] No `v-html` for low-code JSON in UI
- [ ] SQL fragments rejected in template rules

## Audit and Monitoring

| Signal | Action |
|--------|--------|
| `GET /audit-events` | Daily review of value/admin actions |
| `make health-check` | Daily + after deploys |
| low-code `/metrics` | Scrape; no tenant_id in labels |
| Service logs | Error scan; no raw `value_json` |
| 5xx rate | Stop condition if repeated |

## Known Limitations

- Runtime PUT **not admin-guarded** — tenant-scoped API; UI permissions for operators
- `validation_context` is **advisory** — not financial source of truth
- `REPLACE_EXISTING_DRAFT` requires existing DRAFT
- Import/export max **200 fields**, 50 sections, 512 KB
- No Vitest/component test coverage
- Staging auth-on must be verified on **real staging** (not only local Docker)
- No auto-rollback UI for migration writes
- Batch-level audit deferred — use entity events + `batch_id`
- Manual UI not browser-verified in rehearsal pack

## Stop Conditions

Stop pilot immediately if:

- Non-admin accesses admin endpoints (auth-on)
- Tenant isolation failure
- Custom values written to wrong tenant/entity
- Migration execute differs materially from preview
- Audit missing for confirmed writes
- Health-check failure or repeated low-code 5xx
- Import creates unexpected published template

See `LOW_CODE_PILOT_LAUNCH_RUNBOOK_V0.1.md` → Stop Conditions.

## Rollback Procedure

| Scenario | Action |
|----------|--------|
| Admin auth blocks ops (approved) | `LOW_CODE_ADMIN_AUTH_ENABLED=false`; restart service |
| Bad template published | Keep previous PUBLISHED; clone-to-draft from export |
| Bad import DRAFT | Do not publish |
| Bad migration | Audit inspect; DB restore if widespread |
| Emergency | DBA backup restore |

No manual SQL on `lowcode.*` except approved DBA process.

## Daily Pilot Operations

See `LOW_CODE_PILOT_OPERATOR_CHECKLIST_V0.1.md`:

- health-check
- audit review
- error logs
- active template check
- spot-check custom values
- note blockers

## Owner Action Items

| Owner | Action |
|-------|--------|
| **Platform ops** | Enable auth-on on staging; verify health; monitoring |
| **Pilot admin** | Export template before changes; preview before execute |
| **QA / pilot lead** | Complete Manual UI Verification Pack |
| **Security** | Re-run auth curls on staging tenant |
| **DBA** | Confirm backup/restore procedure |
| **Product** | Phase 1 scope: TRANSPORT_ORDER only; sign-off for Phase 2 |

## Go/No-Go Confirmation

| Criterion | Status |
|-----------|--------|
| Rehearsal passed | Yes |
| Hard blockers | None |
| Auth-on verified (local) | Yes |
| Staging auth-on pending | Conditional — ops must repeat |
| Manual UI pending | Conditional — next pack |
| **Overall** | **GO_WITH_CONDITIONS** |

## Next Action

**Low-code Pilot Manual UI Verification Pack v0.1** — browser walkthrough before pilot users.

Related docs:

- `LOW_CODE_PILOT_RELEASE_NOTES_V0.1.md` — short summary for leadership
- `LOW_CODE_PILOT_OPERATOR_CHECKLIST_V0.1.md` — daily operator tasks
- `LOW_CODE_PILOT_LAUNCH_RUNBOOK_V0.1.md` — full launch procedure
