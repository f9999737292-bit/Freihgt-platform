# Low-code Pilot Week-2 SHIPMENT Write Monitoring Daily Report Template v0.1

Copy this template for each SHIPMENT limited-write pilot day. Save as `docs/pilot-reports/YYYY-MM-DD-shipment-write-monitoring.md` or paste into your tracker.

Reference: `LOW_CODE_PILOT_WEEK2_SHIPMENT_WRITE_MONITORING_V0.1.md`, `LOW_CODE_PILOT_OPERATOR_CHECKLIST_V0.1.md`.

---

## Date

`YYYY-MM-DD`

## Pilot Day

Week-2 SHIPMENT limited write — Day **N** (Day 1 = first day with monitored pilot writes after enablement)

## Overall Status

Select one:

- [ ] **GO** — no issues; SHIPMENT write pilot operating normally
- [ ] **GO_WITH_CONDITIONS** — minor issues logged; writes continue with watch items
- [ ] **STOPPED** — P0 stop condition triggered; SHIPMENT writes paused

## Write Summary

| Metric | Value |
|--------|-------|
| Total SHIPMENT pilot writes today | |
| Successful writes (GET + audit OK) | |
| Failed / partial writes | |
| Operators who performed writes | |
| Writes with audit gap | should be **0** |

**Note:** If no writes today, state: *No SHIPMENT pilot writes performed.*

## Entity Scope Used

| entity_id | demo name | tenant_id | approved? | write count | notes |
|-----------|-----------|-----------|-----------|-------------|-------|
| `14d405e2-0152-4030-b356-eec464a3cc66` | DEMO-SH-PLANNED | `74519f22-ff9b-4a8b-8fff-a958c689682f` | initial approved | | |
| | | | | | |

Additional entities require explicit written approval before first write.

## Field Codes Changed

| field_code | times changed | anomalies |
|------------|---------------|-----------|
| `temperature_mode` | | |
| `loading_contact_phone` | | |
| `driver_comment` | | |
| `planned_pickup_date` | | |
| `declared_value` | | |
| `handling_flags` | | |

Any field outside this list → **P0 incident**.

## Audit Summary

| Category | Event count (approx.) | Anomalies |
|----------|----------------------|-----------|
| `CUSTOM_FIELD_VALUES_UPDATED` (SHIPMENT pilot) | | |
| Template admin (export/import/publish) | should be **0** unless approved | |
| Migration (preview/execute) | should be **0** unless approved | |
| Batch migration | should be **0** unless approved | |
| Import/export execute | should be **0** unless approved | |

Baseline audit count at day start: ______  
Audit count at day end: ______  
Audit gap (writes without events): ______ (must be **0**)

## Health Summary

| Check | Result | Notes |
|-------|--------|-------|
| Morning `make health-check` | PASS / FAIL | |
| Evening `make health-check` | PASS / FAIL | |
| low-code-service | OK / DEGRADED / DOWN | |
| Active template (`shipment_default`) | PUBLISHED / CHANGED | |
| GET SHIPMENT values (demo entity) | 200 / other | |
| `make integration-smoke-test` (if run) | PASS / FAIL / skipped | |

## Errors

| time | source | HTTP / symptom | entity | severity | status |
|------|--------|----------------|--------|----------|--------|
| | low-code-service / API / UI | | | P0–P3 | open / resolved |

## Incidents

| time | area | severity | symptom | affected entity/tenant | audit evidence | decision | owner | status | next action |
|------|------|----------|---------|------------------------|----------------|----------|-------|--------|-------------|
| | runtime / audit / security | P0–P3 | | | | stop / fix / defer | | open / resolved | |

(Full write log columns: see Write Monitoring Log Template in `LOW_CODE_PILOT_WEEK2_SHIPMENT_WRITE_MONITORING_V0.1.md`)

## Stop Conditions Review

Confirm each — **none triggered** unless STOPPED status above:

- [ ] No write to wrong tenant
- [ ] No write to wrong entity
- [ ] Audit present for every write today
- [ ] Active template unchanged (`shipment_default` PUBLISHED)
- [ ] No custom values disappeared
- [ ] No repeated low-code-service 5xx
- [ ] integration-smoke-test OK (if run after writes)
- [ ] Operators could identify entity/template
- [ ] No non-admin admin access (auth-on staging)
- [ ] No core shipment status changed unexpectedly
- [ ] No migration/import/publish execute without approval

## Operator Feedback

| operator | feedback summary | severity | action taken |
|----------|------------------|----------|--------------|
| | | P2/P3 / none | |

Form reference: `LOW_CODE_PILOT_OPERATOR_FEEDBACK_FORM_V0.1.md`.

## Decision For Next Day

Select one:

- [ ] **Continue SHIPMENT limited write pilot** — same scope (DEMO-SH-PLANNED + approved list)
- [ ] **Continue with conditions** — list watch items:
- [ ] **Pause writes** — reason:
- [ ] **Expand entity list** — requires written approval (document new entity_id):
- [ ] **Escalate to Fix Pack** — P0/P1 unresolved:

## Owner Actions

| action | owner | due | status |
|--------|-------|-----|--------|
| | pilot lead / operator / DevOps | | open / done |

---

**Report filed by:** _______________  
**Reviewed by (pilot lead):** _______________
