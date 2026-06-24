# Low-code Pilot Week-2 BILLING_REGISTER Write Monitoring v0.1

## Summary

Monitoring package for the **limited BILLING_REGISTER custom-field write pilot**. Defines after-write checks (including financial safety), safe read-only API commands, daily schedule, write monitoring log template, audit/error review, stop conditions, and escalation rules.

**Prerequisite:** Limited write enablement completed (`LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_LIMITED_WRITE_ENABLEMENT_V0.1.md` — **ENABLE_LIMITED_BILLING_REGISTER_WRITE_WITH_CONDITIONS**).

**No real limited-write pilot events collected yet** after enablement — only the earlier controlled validation PUT exists. This pack is **preparatory**; operators must follow it from the first approved pilot write onward.

**This is a monitoring/docs pack only** — no backend, frontend, API contract, or migration changes.

## Current Commit

| Field | Value |
|-------|-------|
| HEAD | `198ee00` — `docs: enable limited billing register write pilot` |
| Pack date | 2026-06-24 |
| Branch | `main` |
| Backend / frontend changed in this pack | **no** |

## Scope

**In scope**

- Limited BILLING_REGISTER write pilot monitoring (approved entities + fields only)
- After-write mandatory checks (12 items including financial safety)
- Safe GET/health/regression commands
- Daily monitoring schedule (morning / after-write / midday / evening)
- Write monitoring log template with financial columns
- Audit, financial safety, and error review
- Stop conditions and escalation
- Daily report template reference

**Out of scope**

- Backend / frontend / API contract changes
- Production broad BILLING_REGISTER rollout
- BILLING_REGISTER PUT in this pack
- migration execute / batch migration execute / import execute
- template publish without review
- manual DB edits
- Committing `LOW_CODE_ADMIN_AUTH_ENABLED=true`

## Evidence Documents

| Document | Status | Notes |
|----------|--------|-------|
| `LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_LIMITED_WRITE_ENABLEMENT_V0.1.md` | **Found** | ENABLE_LIMITED_BILLING_REGISTER_WRITE_WITH_CONDITIONS |
| `LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_LIMITED_WRITE_APPROVAL_NOTE_V0.1.md` | **Found** | Written approval conditions |
| `LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_CONTROLLED_WRITE_VALIDATION_V0.1.md` | **Found** | Controlled PUT 200, audit OK, financial safety OK |
| `LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_OPERATOR_FLOW_REVIEW_V0.1.md` | **Found** | GO_WITH_CONDITIONS |
| `LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_OPERATOR_QUICK_GUIDE_V0.1.md` | **Found** | Operator plain guide |
| `LOW_CODE_PILOT_OPERATOR_CHECKLIST_V0.1.md` | **Found** | BILLING_REGISTER Limited Write Pilot section |

**Missing evidence docs:** none blocking monitoring.

### Evidence summary

| Item | Result |
|------|--------|
| Read-only BILLING_REGISTER validation | **PASS** — GO_WITH_CONDITIONS |
| Controlled write validation | **PASS** — PUT HTTP 200, `saved_count: 3` |
| Limited write enablement decision | **ENABLE_LIMITED_BILLING_REGISTER_WRITE_WITH_CONDITIONS** |
| Allowed BILLING_REGISTER entities | **DEMO-BR-001** only (initial) |
| Allowed field_codes | 3 fields (see below) |
| Financial safety guardrails | Documented in enablement pack |
| Operator checklist status | **Documented** — BILLING_REGISTER Limited Write Pilot section |
| Known restrictions | Pilot/test entities only; no broad production rollout |
| Real pilot writes after enablement | **None yet** |
| Current next action | Begin monitored pilot writes per this pack; fill daily report template |

## Monitoring Decision

**MONITORING_READY**

Enablement evidence exists, controlled validation succeeded, financial safety passed in execute pack, operator flow documented, and no P0 blockers found during pack verification.

If enablement doc were missing or P0 blockers appeared during setup → **NOT_READY_FOR_MONITORING** → **Low-code Runtime Pilot Fix Pack v0.1**.

## Limited Write Scope

| Dimension | Scope |
|-----------|-------|
| entity_type | **BILLING_REGISTER** only |
| template_code | **billing_register_default** |
| Tenants | One pilot tenant (dev/staging demo) |
| Entities | **Approved pilot/test list only** |
| Initial demo entity | **DEMO-BR-001** |
| entity_id | `cf7dbc77-395f-42a2-9717-476e4cd93796` |
| tenant_id | `74519f22-ff9b-4a8b-8fff-a958c689682f` |
| Detail URL (dev) | `http://localhost:3000/billing-registers/cf7dbc77-395f-42a2-9717-476e4cd93796` |
| Users | Platform admin + explicitly approved operators |
| Environment | Staging/pilot — **not** broad production |

Additional BILLING_REGISTER entities require **explicit written approval** (entity ID + tenant documented in daily report).

## Allowed Fields

| field_code | Type | Valid SELECT options (dev) |
|------------|------|----------------------------|
| `cost_allocation_code` | TEXT | Free text |
| `approval_group` | SELECT | LOGISTICS_FINANCE, FINANCE, OPS, MANAGEMENT |
| `payment_priority` | SELECT | LOW, NORMAL, HIGH |

**No other field_codes** may be written under this pilot.

## Forbidden Operations

- Broad production BILLING_REGISTER write rollout
- Billing register core status changes via low-code
- Payment status changes via low-code
- Shipment financial status changes via low-code
- Invoice/act/UPD operations triggered by custom fields
- migration execute
- batch migration execute
- import execute
- template publish without admin review
- manual DB edits
- Writing fields outside allowed list
- Writing to non-approved entity IDs
- Committing `LOW_CODE_ADMIN_AUTH_ENABLED=true`

## After-write Checks

Mandatory after **every** BILLING_REGISTER pilot write:

| # | Check | Pass criteria |
|---|-------|---------------|
| 1 | **GET values after write** | HTTP **200**; response matches entity/tenant/template |
| 2 | **Only intended field_codes changed** | Diff vs pre-write baseline — changed fields ⊆ intended set |
| 3 | **No values disappeared** | All 3 allowed field_codes still present |
| 4 | **Audit event exists** | `CUSTOM_FIELD_VALUES_UPDATED` for entity with matching `request_id` if available |
| 5 | **No migration/import/publish audit** | No unexpected admin execute events |
| 6 | **Active template unchanged** | `billing_register_default` **PUBLISHED** |
| 7 | **Core billing register status unchanged** | GET `/billing-registers/{id}` — status matches baseline |
| 8 | **Payment status unchanged** | No unexpected payment state change |
| 9 | **Shipment financial status unchanged** | Linked shipment item unchanged if applicable |
| 10 | **No invoice/act/UPD triggered** | UPD/invoices/acts unchanged |
| 11 | **No low-code-service 5xx** | No repeated 5xx during/after write window |
| 12 | **Record in daily pilot report** | Row in write monitoring log |

Reference: `LOW_CODE_PILOT_OPERATOR_CHECKLIST_V0.1.md` — BILLING_REGISTER Limited Write Pilot section.

## Safe API Commands

Gateway dev default: `http://localhost:8080/api/v1`.

### GET BILLING_REGISTER values

```powershell
curl.exe -i -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" `
  "http://localhost:8080/api/v1/low-code/custom-field-values?entity_type=BILLING_REGISTER&entity_id=cf7dbc77-395f-42a2-9717-476e4cd93796&template_code=billing_register_default"
```

### Core billing register GET (financial baseline)

```powershell
curl.exe -i -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" `
  "http://localhost:8080/api/v1/billing-registers/cf7dbc77-395f-42a2-9717-476e4cd93796"
```

Compare `status`, `total_with_vat`, `total_without_vat`, `version`, UPD status before and after writes.

### Audit GET

```powershell
curl.exe -i -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" `
  "http://localhost:8080/api/v1/low-code/audit-events?limit=20"
```

Entity-filtered:

```powershell
curl.exe -i -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" `
  "http://localhost:8080/api/v1/low-code/audit-events?entity_type=BILLING_REGISTER&entity_id=cf7dbc77-395f-42a2-9717-476e4cd93796&limit=20"
```

### Active template

```powershell
curl.exe -i -H "X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f" `
  "http://localhost:8080/api/v1/low-code/form-templates/active?entity_type=BILLING_REGISTER&template_code=billing_register_default"
```

### Health

```powershell
cd D:\Projects\freight-platform
make health-check
```

### Regression

```powershell
cd D:\Projects\freight-platform
make integration-smoke-test
```

### Frontend build (dev/local validation)

```powershell
cd D:\Projects\freight-platform\apps\web-admin
npm run build
```

**Do not run PUT** during routine monitoring unless explicitly approved and documented.

## Daily Monitoring Schedule

| Window | Actions | Owner | Output |
|--------|---------|-------|--------|
| **Morning** | health-check; active template GET; audit latest 20; open P0/P1 review; financial side-effect incident review | Operator + DevOps | Baseline in daily report |
| **After every write** | GET after write; audit check; core billing register GET; log operator/action/entity/field_codes | Operator | Row in write monitoring log |
| **Midday** | low-code-service errors; audit review; operator feedback; verify no billing/payment side effects reported | Operator | Issues logged with severity |
| **Evening** | health-check; issue summary; write count; audit gap check; financial safety gap check; next-day decision | Pilot lead | Daily monitoring report filled |

## Write Monitoring Log Template

Copy into daily report or tracker. One row per BILLING_REGISTER pilot write.

| time | operator | tenant_id | entity_type | entity_id | template_code | changed field_codes | GET after write ok | audit event visible | active template unchanged | billing status unchanged | payment status unchanged | shipment financial status unchanged | invoice/act/UPD not triggered | errors | severity | decision | owner | next action |
|------|----------|-----------|-------------|-----------|---------------|---------------------|--------------------|-----------------------|---------------------------|--------------------------|--------------------------|-------------------------------------|-------------------------------|--------|----------|----------|-------|-------------|
| | | | BILLING_REGISTER | | billing_register_default | | yes/no | yes/no | yes/no | yes/no | yes/no | yes/no | yes/no | | P0–P3 | continue/stop/fix | | |

### Severity guide

| Level | Meaning | Response |
|-------|---------|----------|
| **P0** | Stop pilot | Stop BILLING_REGISTER writes immediately; escalate |
| **P1** | Fix today | Assign owner; fix or workaround before EOD |
| **P2** | Backlog | Log; schedule in Week fix plan |
| **P3** | Note only | Cosmetic / documentation; no action required |

## Audit Review

### What to look for (BILLING_REGISTER pilot)

| Event category | Examples | Red flag |
|----------------|----------|----------|
| Custom values | `CUSTOM_FIELD_VALUES_UPDATED` | Wrong `entity_id`, tenant, or field_codes outside allowed list |
| Template admin | `FORM_TEMPLATE_*`, export/import | Unexpected publish; import execute without ticket |
| Migration | preview/execute events | Any execute during pilot without approval |
| Batch | batch preview/execute | Batch execute without sign-off |
| Actor | `actor_id`, `request_id` | Missing audit after visible write |

### Audit gap check (evening)

Compare **write count** (from log) vs **audit `CUSTOM_FIELD_VALUES_UPDATED` count** for BILLING_REGISTER pilot entities. Any write without audit → **P0**.

## Financial Safety Monitoring

| Rule | Monitoring action |
|------|-------------------|
| Custom fields = auxiliary metadata only | Never treat custom field save as payment/billing command |
| validation_context advisory only | Do not use context fields as financial source of truth |
| Source of truth | Core billing register + billing services API |
| Status/totals after write | Core GET before/after every write — must match baseline |
| Suspected financial side effect | Treat as **P0** until proven unrelated to low-code write |
| invoice/act/UPD | Must not be triggered by custom-field PUT |
| Any confirmed side effect | Stop writes → **Low-code Runtime Pilot Fix Pack v0.1** |

### Core baseline (dev demo — record at monitoring start)

| Field | Current value |
|-------|-------------|
| status | `CLOSING_DOCUMENTS_CREATED` |
| total_with_vat | 119400 |
| version | 4 |
| UPD status | DRAFT |

## Error Review

| Source | What to check | Action |
|--------|---------------|--------|
| low-code-service logs | Repeated 5xx, stack traces | P0 if 3+ 5xx in 15 min |
| API responses | 400 on invalid SELECT (e.g. invalid `approval_group`) | P1 — document; use valid options |
| Core billing register GET | Unexpected status/total change | **P0** — stop writes |
| integration-smoke-test | Failure after write | P0 — stop writes; investigate |
| Operator feedback | Save failures, financial confusion | P1/P2 per impact |

Reference: `LOW_CODE_PILOT_OPERATOR_FEEDBACK_FORM_V0.1.md`.

## Stop Conditions

**P0 — stop all BILLING_REGISTER writes immediately** if any occur:

| # | Condition |
|---|-----------|
| 1 | Write to wrong tenant |
| 2 | Write to wrong entity |
| 3 | Audit missing after write |
| 4 | Active template changed unexpectedly |
| 5 | Custom values disappeared |
| 6 | Core billing register status changed unexpectedly |
| 7 | Payment status changed unexpectedly |
| 8 | Shipment financial status changed unexpectedly |
| 9 | Invoice/act/UPD operation triggered unexpectedly |
| 10 | low-code-service repeated **5xx** (3+ in 15 min) |
| 11 | `make integration-smoke-test` fails after write |
| 12 | Operator cannot identify entity/template |
| 13 | Non-admin can access admin endpoints in auth-on staging |

**On any P0:**

1. **Stop** BILLING_REGISTER writes
2. **Document** incident in write monitoring log + daily report
3. **Inspect audit** — entity_id, changed_fields, timestamp, request_id
4. **Compare core billing register GET** before/after
5. **Escalate:** **Low-code Runtime Pilot Fix Pack v0.1**
6. **Recovery:** see rollback in `LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_LIMITED_WRITE_ENABLEMENT_V0.1.md`

## Escalation Rules

| Situation | Escalate to | Channel |
|-----------|-------------|---------|
| P0 stop condition | Pilot lead + platform ops + finance lead | Immediate |
| P0 financial side effect | Pilot lead + finance ops | Immediate |
| P1 audit gap / wrong entity | Pilot lead + backend owner (read-only triage) | Same day |
| P1 auth/tenant issue | Security reviewer + DevOps | Same day |
| Repeated 5xx | DevOps | Check logs; restart if approved |
| Operator confusion (P2/P3) | Operator → pilot lead | Daily report |

**Do not** deploy code fixes during monitoring without explicit approval and separate fix pack.

## Known Limitations

- **No real limited-write pilot events collected yet** after enablement
- Single demo entity enabled initially (DEMO-BR-001)
- No real operator feedback collected yet
- Browser UI sign-off pending
- Auth-on must be verified on staging separately
- Invalid SELECT values return HTTP 400 — operators must use valid options
- Demo entity has test values from controlled write validation (optional rollback)
- Broad production BILLING_REGISTER rollout **not approved**
- TRANSPORT_ORDER and SHIPMENT pilots continue in parallel — cross-entity review is next

## Verification Run (this pack)

| Check | Result | Run ID / notes |
|-------|--------|----------------|
| `make health-check` | **PASS** | All 9 services OK |
| `make seed-lowcode-demo` | **PASS** | BILLING_REGISTER `billing_register_default` PUBLISHED |
| `make integration-smoke-test` | **PASS** | `TEST-20260624185843` |
| `npm run build` | **PASS** | web-admin build complete |
| GET BILLING_REGISTER values | **PASS** | HTTP 200 |
| Audit GET | **PASS** | HTTP 200 |
| Active template GET | **PASS** | `billing_register_default` PUBLISHED |
| Core billing register GET | **PASS** | status + totals baseline OK |
| PUT in this pack | **no** |

## Daily Report Template

Fill end-of-day using:

`docs/LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_WRITE_MONITORING_REPORT_TEMPLATE_V0.1.md`

## Next Action

**Low-code Pilot Week-2 Cross-Entity Pilot Readiness Review Pack v0.1**

Continue BILLING_REGISTER limited write monitoring per this pack during Week-2.

Continue SHIPMENT limited write monitoring per `LOW_CODE_PILOT_WEEK2_SHIPMENT_WRITE_MONITORING_V0.1.md`.

If P0 stop condition during monitoring:

**Low-code Runtime Pilot Fix Pack v0.1**

Reference docs:

- `LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_LIMITED_WRITE_ENABLEMENT_V0.1.md`
- `LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_LIMITED_WRITE_APPROVAL_NOTE_V0.1.md`
- `LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_OPERATOR_QUICK_GUIDE_V0.1.md`
- `LOW_CODE_PILOT_OPERATOR_CHECKLIST_V0.1.md`
