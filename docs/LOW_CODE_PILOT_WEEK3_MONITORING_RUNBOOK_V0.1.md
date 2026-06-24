# Low-code Pilot Week-3 Monitoring Runbook v0.1

## Purpose

Operational runbook for **daily low-code runtime pilot monitoring** during Week-3 across **TRANSPORT_ORDER**, **SHIPMENT**, and **BILLING_REGISTER**.

**Constraints:** Read-only checks by default. Writes only per entity enablement docs and operator approval — never in evidence-only packs.

**Tenant:** `74519f22-ff9b-4a8b-8fff-a958c689682f`

**Reference entities:**

| Entity | Demo ID | entity_id | template_code |
|--------|---------|-----------|---------------|
| TRANSPORT_ORDER | DEMO-TO-001 | `2db04b49-665c-469f-bcb1-ffeb1274fedb` | `transport_order_default` |
| SHIPMENT | DEMO-SH-PLANNED | `14d405e2-0152-4030-b356-eec464a3cc66` | `shipment_default` |
| BILLING_REGISTER | DEMO-BR-001 | `cf7dbc77-395f-42a2-9717-476e4cd93796` | `billing_register_default` |

## Daily Morning Checks

1. `make health-check` — all services must be OK.
2. Active template GET for each entity — expect HTTP **200**, status **PUBLISHED**, no unexpected version change.
3. Custom values GET for demo entities — expect HTTP **200**, values visible.
4. Audit GET (`limit=50`) — baseline event count; note any overnight migration/import/publish.
5. Review open P0/P1 from prior day.

**Pass criteria:** All GETs 200; low-code-service healthy; no unexpected template changes.

## After-write Checks

Apply only after an **approved** pilot write (per entity enablement doc).

### All entities

1. Custom values GET — values match expected save.
2. Audit GET — `CUSTOM_FIELD_VALUES_UPDATED` for entity_id within 5 minutes.
3. `make health-check` — low-code-service still OK.
4. Fill entity daily report template (see Reporting Template).

### SHIPMENT additional

- Confirm entity is **DEMO-SH-PLANNED** (or other approved entity only).
- Confirm field_codes are within allowed 6-field list.
- Reference: `LOW_CODE_PILOT_WEEK2_SHIPMENT_WRITE_MONITORING_V0.1.md`

### BILLING_REGISTER additional

- Core billing register GET — no unexpected financial side effects.
- Confirm entity is **DEMO-BR-001** only.
- Confirm field_codes within allowed 3-field list.
- Reference: `LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_WRITE_MONITORING_V0.1.md`

## Midday Checks

1. Scan low-code-service logs for 5xx spikes.
2. Audit GET — compare to morning baseline; flag unexpected events.
3. Collect operator issues (if any).
4. If writes occurred: verify after-write checklist completed.

## Evening Checks

1. `make health-check`
2. Audit review — any gaps for today's writes?
3. Fill daily monitoring report (or "no writes today").
4. Review stop conditions — P0/P1 status.
5. Document next-day plan.

## Audit Review

```powershell
$T = "74519f22-ff9b-4a8b-8fff-a958c689682f"
curl.exe -s -H "X-Tenant-ID: $T" "http://localhost:8080/api/v1/low-code/audit-events?limit=50"
```

**Look for:**

- `CUSTOM_FIELD_VALUES_UPDATED` after each approved write
- Unexpected `TEMPLATE_PUBLISHED`, migration, or import execute events
- Missing audit after a known write → **P1** until explained

**Target:** Zero unexplained audit gaps for pilot writes.

## Financial/Core Safety Review

Required after any **BILLING_REGISTER** write and during daily BR monitoring.

| Check | Action |
|-------|--------|
| BR custom values GET | Values match expected; no corrupt JSON |
| Core billing register | No unexpected status/amount changes |
| Audit | BR write event present |
| Stop conditions | See P0 Stop Procedure |

Reference: `LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_WRITE_MONITORING_V0.1.md`

## Incident Severity

| Severity | Definition | Response |
|----------|------------|----------|
| **P0** | Data loss, financial corruption, repeated 5xx blocking writes, security bypass, wrong tenant data exposed | **STOP** writes immediately |
| **P1** | Single write failure with repro, audit gap, template unexpected change, operator blocked | Fix within 24h; hold expansion |
| **P2** | UX confusion, non-blocking errors, doc gaps | Backlog; document |
| **P3** | Cosmetic / nice-to-have | Backlog |

## P0 Stop Procedure

1. **STOP** all SH/BR pilot writes immediately.
2. Notify PM + pilot lead.
3. Preserve evidence: audit GET, logs, repro steps.
4. Run `make health-check`.
5. Open **Low-code Runtime Pilot Fix Pack v0.1** — no further writes until PM clears.
6. Document incident in daily report with **STOPPED** status.

## P1 Fix Procedure

1. Log issue with repro and entity/field scope.
2. Assign owner (Backend/Frontend per area).
3. Hold entity expansion until fixed or waived by PM.
4. Re-run read-only verification after fix.
5. Update daily report.

## Reporting Template

Use entity-specific templates:

| Entity | Template |
|--------|----------|
| TRANSPORT_ORDER | `LOW_CODE_PILOT_DAILY_REPORT_TEMPLATE_V0.1.md` |
| SHIPMENT | `LOW_CODE_PILOT_WEEK2_SHIPMENT_WRITE_MONITORING_REPORT_TEMPLATE_V0.1.md` |
| BILLING_REGISTER | `LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_WRITE_MONITORING_REPORT_TEMPLATE_V0.1.md` |

**Baseline report:** `LOW_CODE_PILOT_WEEK3_MONITORING_BASELINE_REPORT_V0.1.md`

Minimum daily fields:

- Date / pilot day
- Health pass/fail
- Active template status (TO/SH/BR)
- Writes today (count + entities)
- Audit gaps (yes/no)
- P0/P1 count
- Operator feedback (if any)
- Decision: GO / GO_WITH_CONDITIONS / STOPPED

## Escalation Rules

| Condition | Escalate to | Action |
|-----------|-------------|--------|
| P0 | PM + pilot lead immediately | STOP writes |
| P1 unresolved >24h | PM | Fix pack or hold |
| Repeated low-code 5xx | DevOps + Backend | Health investigation |
| BR financial anomaly | PM + finance owner | STOP BR writes |
| Auth/RBAC concern | Security | Auth-on staging pack |
| Missing operator feedback by mid-week | Operator lead | Schedule walkthrough |

## Commands

### Project root

```powershell
cd D:\Projects\freight-platform
```

### Health and regression

```powershell
make health-check
make seed-lowcode-demo
make integration-smoke-test
```

### Active templates GET

```powershell
$T = "74519f22-ff9b-4a8b-8fff-a958c689682f"

curl.exe -i -H "X-Tenant-ID: $T" `
  "http://localhost:8080/api/v1/low-code/form-templates/active?entity_type=TRANSPORT_ORDER&template_code=transport_order_default"

curl.exe -i -H "X-Tenant-ID: $T" `
  "http://localhost:8080/api/v1/low-code/form-templates/active?entity_type=SHIPMENT&template_code=shipment_default"

curl.exe -i -H "X-Tenant-ID: $T" `
  "http://localhost:8080/api/v1/low-code/form-templates/active?entity_type=BILLING_REGISTER&template_code=billing_register_default"
```

### Custom values GET

```powershell
$T = "74519f22-ff9b-4a8b-8fff-a958c689682f"

curl.exe -i -H "X-Tenant-ID: $T" `
  "http://localhost:8080/api/v1/low-code/custom-field-values?entity_type=TRANSPORT_ORDER&entity_id=2db04b49-665c-469f-bcb1-ffeb1274fedb&template_code=transport_order_default"

curl.exe -i -H "X-Tenant-ID: $T" `
  "http://localhost:8080/api/v1/low-code/custom-field-values?entity_type=SHIPMENT&entity_id=14d405e2-0152-4030-b356-eec464a3cc66&template_code=shipment_default"

curl.exe -i -H "X-Tenant-ID: $T" `
  "http://localhost:8080/api/v1/low-code/custom-field-values?entity_type=BILLING_REGISTER&entity_id=cf7dbc77-395f-42a2-9717-476e4cd93796&template_code=billing_register_default"
```

### Audit GET

```powershell
$T = "74519f22-ff9b-4a8b-8fff-a958c689682f"
curl.exe -i -H "X-Tenant-ID: $T" "http://localhost:8080/api/v1/low-code/audit-events?limit=50"
```

### Frontend build

```powershell
cd D:\Projects\freight-platform\apps\web-admin
npm run build
```

**Forbidden in monitoring/evidence packs:** PUT/save, migration execute, batch migration execute, import execute, template publish, manual DB edits, committing `LOW_CODE_ADMIN_AUTH_ENABLED=true`.
