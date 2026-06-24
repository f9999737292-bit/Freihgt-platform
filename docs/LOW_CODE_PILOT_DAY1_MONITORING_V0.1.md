# Low-code Pilot Day-1 Monitoring v0.1

## Summary

Day-1 monitoring package for the first operational day of the low-code staging pilot. Defines morning/midday/evening checklists, safe commands, audit focus areas, stop conditions, incident logging, and escalation rules.

**Prerequisite:** Final smoke handoff completed (`LOW_CODE_PILOT_FINAL_SMOKE_HANDOFF_V0.1.md` — **GO_WITH_CONDITIONS**).

**Day-1 pilot can proceed** only if final smoke decision remains **GO_WITH_CONDITIONS** and **no P0 stop condition** appears during monitoring.

**This is a monitoring/docs pack only** — no code changes.

## Current Commit

| Field | Value |
|-------|-------|
| HEAD | `466d593` — `docs: add low-code pilot final smoke handoff` |
| Pack date | 2026-06-24 |
| Branch | `main` |
| Backend / frontend changed in this pack | **no** |

## Scope

**In scope**

- Day-1 monitoring schedule (morning / midday / evening)
- Safe read-only and health commands
- Audit, security, and stop-condition review
- Incident log template and escalation rules
- Daily report template reference

**Out of scope**

- Code changes
- Production import/migration execute
- Batch migration execute
- Template publish
- Manual DB edits
- Auth-on env commit

## Evidence Documents

| Document | Status |
|----------|--------|
| `LOW_CODE_PILOT_FINAL_SMOKE_HANDOFF_V0.1.md` | **Found** (`466d593`) |
| `LOW_CODE_PILOT_HANDOFF_NOTE_V0.1.md` | **Found** (`466d593`) |
| `LOW_CODE_PILOT_RELEASE_PACKAGE_V0.1.md` | **Found** (`168e2f9`) |
| `LOW_CODE_PILOT_OPERATOR_CHECKLIST_V0.1.md` | **Found** (`168e2f9`) |
| `LOW_CODE_PILOT_RELEASE_NOTES_V0.1.md` | **Found** (`168e2f9`) |
| `LOW_CODE_STAGING_AUTH_ON_VERIFICATION_V0.1.md` | **Found** (`9afb85c`) |
| `LOW_CODE_RUNTIME_PILOT_STAGING_CHECKLIST_V0.1.md` | **Found** (`da5af8e`) |

**Missing evidence docs:** none.

**Current pilot decision:** **GO_WITH_CONDITIONS** (from final smoke handoff).

## Day-1 Monitoring Schedule

| Window | Time (suggested) | Owner | Output |
|--------|------------------|-------|--------|
| **Morning** | T+0h (pilot open) | Operator + DevOps | Health OK; active template confirmed; audit baseline |
| **Midday** | T+4h | Operator | Usage/audit review; user issues logged |
| **Evening** | T+8h | Pilot lead | Daily report filled; stop-condition review; tomorrow plan |

Replace staging tenant/credentials with pilot values before Day-1.

## Morning Checklist

- [ ] **Platform health** — `make health-check` all services green (including `low-code-service`)
- [ ] **Low-code-service health** — direct `/health` or via gateway OK
- [ ] **Active template** — `transport_order_default` PUBLISHED for TRANSPORT_ORDER (curl below → 200)
- [ ] **Custom values GET** — demo/pilot entity returns 200 with expected fields
- [ ] **Audit events GET** — `/audit-events?limit=20` returns 200; note baseline count
- [ ] **Admin access check** (staging auth-on):
  - No `X-User-ID` on admin route → **401**
  - Non-admin user (`SHIPPER_LOGIST`) → **403**
  - Platform admin → **200**
- [ ] **Known limitations reminder** — share with pilot users:
  - TO only (Phase 1)
  - One tenant
  - No batch execute without preview sign-off
  - Import creates DRAFT only
- [ ] Record morning status in daily report template

## Midday Checklist

- [ ] **Recent audit events** — scan last 4h for unexpected writes/admin actions
- [ ] **Low-code-service logs/errors** — no repeated 5xx; no raw `value_json` in logs
- [ ] **User-reported issues** — log in incident table (severity P0–P3)
- [ ] **Custom values writes** — confirm PUT events appear in audit with correct `entity_id` / tenant
- [ ] **Template changes** — any DRAFT created? Any publish attempted? (should be none without approval)
- [ ] **Import/export usage** — if used: export before import; preview only unless approved execute
- [ ] **Migration preview usage** — if used: preview result documented; no execute unless SAFE + approved
- [ ] Escalate any P0/P1 immediately

## Evening Checklist

- [ ] **Health-check** — re-run `make health-check`
- [ ] **Audit summary** — count events by category (values, template, migration, import/export)
- [ ] **Issue summary** — open incidents; resolved vs open
- [ ] **Stop condition review** — confirm none triggered (see below)
- [ ] **Next-day action list** — fill daily report; assign owners
- [ ] Share daily report with pilot lead and staging owner

## Safe Commands

### Platform (from project root)

```powershell
cd D:\Projects\freight-platform
make health-check
make seed-lowcode-demo          # dev/local only — not on staging during pilot
make integration-smoke-test     # dev/local validation — optional on staging if approved
```

### Frontend build (dev/local validation)

```powershell
cd D:\Projects\freight-platform\apps\web-admin
npm run build
```

### Low-code API monitoring (read-only)

Replace `$T` with **pilot tenant ID**. Gateway: `http://localhost:8080/api/v1` (dev) or staging gateway URL.

```powershell
$T = "74519f22-ff9b-4a8b-8fff-a958c689682f"
$GW = "http://localhost:8080/api/v1"

# Active template
curl.exe -i -H "X-Tenant-ID: $T" `
  "$GW/low-code/form-templates/active?entity_type=TRANSPORT_ORDER&template_code=transport_order_default"

# Custom values GET
curl.exe -i -H "X-Tenant-ID: $T" `
  "$GW/low-code/custom-field-values?entity_type=TRANSPORT_ORDER&entity_id=2db04b49-665c-469f-bcb1-ffeb1274fedb&template_code=transport_order_default"

# Audit GET
curl.exe -i -H "X-Tenant-ID: $T" `
  "$GW/low-code/audit-events?limit=20"

# Admin templates list (requires auth-on + platform admin on staging)
curl.exe -i -H "X-Tenant-ID: $T" `
  "$GW/low-code/admin/form-templates"
```

**Do not run** import execute, migration execute, batch execute, or publish during routine Day-1 monitoring unless explicitly approved and documented.

## API Monitoring Checks

| Check | Expected | Action if fail |
|-------|----------|----------------|
| Active template HTTP | **200** | P0 if wrong template/code; escalate |
| Custom values GET | **200** | P1 if 5xx; P0 if wrong tenant data |
| Audit GET | **200** | P1 if down; P0 if writes occurred but audit empty |
| Admin list (auth-on) | **401/403/200** per role | P0 if non-admin gets 200 |

## UI Monitoring Checks

Spot-check in browser (platform admin + one shipper user):

| Page | What to verify |
|------|----------------|
| `/transport-orders/{id}` | Custom fields panel loads; save works; errors readable |
| `/low-code/admin/form-templates` | Admin only; list loads; no crash |
| `/low-code/audit` | Events visible; filters work |
| `/low-code/custom-field-values` | Values load for TO entities |

Non-admin user must **not** reach `/low-code/admin/*` (redirect or 403).

## Audit Review

### What to look for on Day-1

| Event category | Examples | Red flag |
|----------------|----------|----------|
| Custom values | `CUSTOM_FIELD_VALUES_UPDATED` | Wrong `entity_id` or tenant |
| Template admin | `FORM_TEMPLATE_*`, export/import | Unexpected publish; import execute without ticket |
| Migration | preview/execute events | Execute without prior preview record |
| Batch | batch preview/execute | Batch execute on Day-1 without sign-off |
| Actor | `actor_id`, `request_id` if present | Missing actor on admin write |

### Audit query examples

```powershell
$T = "{pilot_tenant_id}"
$GW = "http://{gateway}/api/v1"

# Last 20 events
curl.exe -H "X-Tenant-ID: $T" "$GW/low-code/audit-events?limit=20"

# Entity-specific
curl.exe -H "X-Tenant-ID: $T" `
  "$GW/low-code/audit-events?entity_type=TRANSPORT_ORDER&entity_id={id}&limit=50"

# Migrations category (UI)
# http://{web-admin}/low-code/audit?category=migrations
```

## Security Review

| Area | Day-1 check |
|------|-------------|
| Tenant isolation | All API calls scoped to pilot tenant; no cross-tenant data in UI |
| Admin RBAC | Only `PLATFORM_ADMIN` on admin routes (staging auth-on) |
| Env hygiene | `LOW_CODE_ADMIN_AUTH_ENABLED=true` **not committed** to tracked compose |
| Manual DB | No ad-hoc SQL during pilot |
| Logs | No raw sensitive `value_json` in low-code-service logs |

Reference: `LOW_CODE_STAGING_AUTH_ON_VERIFICATION_V0.1.md`, `LOW_CODE_PERMISSIONS_MATRIX_V0.1.md`.

## Stop Conditions

**Stop pilot immediately** (P0) if any occur:

| # | Condition |
|---|-----------|
| 1 | Admin endpoint accessible by non-admin (auth-on staging) |
| 2 | Tenant isolation issue — cross-tenant data visible |
| 3 | Wrong entity/tenant writes |
| 4 | Audit missing for write/admin operations |
| 5 | Repeated low-code-service **5xx** (3+ in 15 min) |
| 6 | `make health-check` failure on low-code-service |
| 7 | Active template wrong version/code unexpectedly changed |
| 8 | Import/export unexpected status/version without approval |
| 9 | Migration execute outcome differs from preview expectation |

**On stop:** follow rollback in `LOW_CODE_PILOT_LAUNCH_RUNBOOK_V0.1.md`; log P0 incident; notify pilot lead and platform ops.

## Incident Log Template

Use this table for Day-1 incidents (copy into daily report or tracker):

| time | reporter | area | severity | symptom | affected entity/tenant | command/page | logs/audit evidence | decision | owner | status | next action |
|------|----------|------|----------|---------|------------------------|--------------|---------------------|----------|-------|--------|-------------|
| | | runtime / admin / security / audit | P0–P3 | | | | | stop / fix / defer | | open / resolved | |

### Severity guide

| Level | Meaning | Response |
|-------|---------|----------|
| **P0** | Stop pilot | Immediate stop + escalate pilot lead + platform ops |
| **P1** | Fix today | Assign owner; fix or workaround before EOD |
| **P2** | Defer | Log; schedule for Week-1 fix plan |
| **P3** | Note only | Cosmetic / documentation; no action required |

## Escalation Rules

| Situation | Escalate to | Channel |
|-----------|-------------|---------|
| P0 stop condition | Pilot lead + platform ops | Immediate (call/chat) |
| P1 auth/tenant issue | Security reviewer + DevOps | Same day |
| P1 functional bug | Frontend/backend owner (read-only triage) | Same day — **no hotfix without approval** |
| Repeated 5xx | DevOps | Check logs, restart service if approved |
| User confusion (P2/P3) | Operator → pilot lead | Daily report |

**Do not** deploy code fixes on Day-1 without explicit approval and separate fix pack.

## Daily Report Template

Fill end-of-day using:

`docs/LOW_CODE_PILOT_DAILY_REPORT_TEMPLATE_V0.1.md`

## Known Limitations

- Auth-on verified locally — repeat on staging before Day-1 open
- Phase 1 scope: one tenant, TRANSPORT_ORDER, `transport_order_default`
- No automatic rollback UI
- Batch execute deferred unless explicitly signed off
- SHIPMENT/BILLING_REGISTER panels exist but Phase 2 scope
- Agent/automated browser monitoring not included — operator manual checks required

## Verification Run (this pack)

| Check | Result | Run ID / notes |
|-------|--------|----------------|
| `make health-check` | **PASS** | All 9 services OK |
| `make seed-lowcode-demo` | **PASS** | 6 PUBLISHED templates |
| `make integration-smoke-test` | **PASS** | `TEST-20260624150656` |
| `npm run build` | **PASS** | web-admin build complete |

## Next Action

**Low-code Pilot Week-1 Feedback & Fix Plan Pack v0.1**

If Day-1 monitoring finds **P0 stop condition**:

**Low-code Runtime Pilot Fix Pack v0.1**
