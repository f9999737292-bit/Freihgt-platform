# Low-code Pilot Week-3 First Operator Feedback Action Plan v0.1

## Purpose

Action plan to obtain **first real operator feedback** for Week-3 low-code pilot on **TRANSPORT_ORDER** (baseline), **SHIPMENT** (limited write), and **BILLING_REGISTER** (limited write).

**Prerequisite:** Feedback collection process ready — `FEEDBACK_READY_WITH_CONDITIONS`. **No submissions yet** — this plan defines how to collect them.

**Reference:** `LOW_CODE_PILOT_WEEK3_OPERATOR_FEEDBACK_EVIDENCE_V0.1.md`, `LOW_CODE_PILOT_WEEK3_OPERATOR_FEEDBACK_FORM_TEMPLATE_V0.1.md`

## Required First Feedback Sessions

| # | Entity | Type | Minimum |
|---|--------|------|---------|
| 1 | TRANSPORT_ORDER | Baseline read-only review | **1** session |
| 2 | SHIPMENT | Limited-write pilot review | **1** session |
| 3 | BILLING_REGISTER | Limited-write pilot review | **1** session |

**Duration:** ~15 minutes each. **Format:** guided walkthrough + form completion.

## Operators / Roles Needed

| Role | Suggested participant | Purpose |
|------|----------------------|---------|
| Operator / shipper logist | `shipper@7rights.local` or designated pilot operator | Primary feedback |
| Pilot lead | Facilitates walkthrough | Records in feedback log |
| PM / triage owner | Optional observer | P0/P1 escalation path |
| Platform admin | Optional for TO baseline | Compare admin vs operator view |

**Do not** require production operators until pilot scope explicitly expanded.

## Scenario 1: TRANSPORT_ORDER Baseline Review

**Entity:** DEMO-TO-001 — `2db04b49-665c-469f-bcb1-ffeb1274fedb`  
**Template:** `transport_order_default`  
**Tenant:** `74519f22-ff9b-4a8b-8fff-a958c689682f`

### Steps

1. Open transport order detail for DEMO-TO-001 in web-admin.
2. Locate low-code custom fields panel.
3. Verify panel visibility and load time.
4. Review fields (read-only focus):
   - `cargo_class`
   - `internal_cost_center`
   - `loading_window_note`
5. Check field labels, required indicators, help text.
6. Optional: locate audit history for prior TO writes.
7. **Do not save** unless separate controlled write approval exists.

### Operator questions (from collection doc)

- Entity easy to find?
- Panel loaded correctly?
- Labels clear?
- Audit findable?

### Evidence to capture

Completed form template — entity_type **TRANSPORT_ORDER**, scenario **baseline read-only review**.

## Scenario 2: SHIPMENT Limited Write Review

**Entity:** DEMO-SH-PLANNED — `14d405e2-0152-4030-b356-eec464a3cc66`  
**Template:** `shipment_default`  
**Quick guide:** `LOW_CODE_PILOT_WEEK2_SHIPMENT_OPERATOR_QUICK_GUIDE_V0.1.md`

### Steps

1. Open shipment detail for DEMO-SH-PLANNED.
2. Verify low-code panel visibility.
3. Review allowed fields (6 field_codes):
   - `temperature_mode`
   - `loading_contact_phone`
   - `driver_comment`
   - `planned_pickup_date`
   - `declared_value`
   - `handling_flags`
4. Discuss Save button clarity and success/error feedback (read-first; optional limited write only with pilot lead approval).
5. Discuss audit visibility after save (if write performed under separate approval).
6. **Production writes forbidden** in this evidence pack.

### Operator questions

- SHIPMENT fields clear?
- Date/money/multi-select behavior understood?
- Route/status/driver context confusion?

### Evidence to capture

Form template — entity_type **SHIPMENT**, note whether read-only or limited write attempted.

## Scenario 3: BILLING_REGISTER Limited Write Review

**Entity:** DEMO-BR-001 — `cf7dbc77-395f-42a2-9717-476e4cd93796`  
**Template:** `billing_register_default`  
**Quick guide:** `LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_OPERATOR_QUICK_GUIDE_V0.1.md`

### Steps

1. Open billing register detail for DEMO-BR-001.
2. Verify low-code panel visibility.
3. Review allowed fields (3 field_codes):
   - `cost_allocation_code`
   - `approval_group`
   - `payment_priority`
4. **Financial safety briefing:** low-code custom fields do **not** change core billing/payment/status.
5. Review financial safety wording in UI and quick guide.
6. Discuss operator confidence — any fear of accidental payment impact?
7. Optional limited write only with pilot lead + financial guardrails approval (not in this docs pack).

### Operator questions

- Billing fields clear?
- Payment/billing status confusion?
- Financial safety wording clear?
- Cost allocation / approval / payment priority concerns?

### Evidence to capture

Form template — entity_type **BILLING_REGISTER**, **Financial/Core Safety Concerns** section mandatory.

## Evidence To Capture

Per session, minimum:

| Field | Required |
|-------|----------|
| Operator name / role | Yes |
| Date/time | Yes |
| Entity type + demo ID | Yes |
| Scenario tested | Yes |
| Panel visible? | Yes |
| Field labels clear? | Yes |
| Validation / save / audit sections | As applicable |
| Financial safety (BR) | Yes for BR |
| Severity P0–P3 | Yes |
| Operator decision GO/GO_WITH_CONDITIONS/STOP | Yes |

## How To Record Feedback

1. Operator fills `LOW_CODE_PILOT_WEEK3_OPERATOR_FEEDBACK_FORM_TEMPLATE_V0.1.md`.
2. Pilot lead adds row to `LOW_CODE_PILOT_WEEK3_OPERATOR_FEEDBACK_LOG_V0.1.md`:
   - id: `FB-W3-001`, `FB-W3-002`, … (sequential)
   - status: **NEW**
3. PM triage within same day for P0/P1.
4. Update `LOW_CODE_PILOT_WEEK3_IMPROVEMENTS_BACKLOG_V0.1.md` if P2/P3 themes emerge.
5. Fill daily triage report template if triage day.

## How To Escalate P0/P1

| Severity | Action |
|----------|--------|
| **P0** | STOP affected writes; notify PM immediately; open Fix Pack |
| **P1** | Assign owner; Fix Pack or hotfix ≤24h; pilot continues only with PM condition |

Reference: `LOW_CODE_PILOT_WEEK3_OPERATOR_FEEDBACK_TRIAGE_RUNBOOK_V0.1.md`

## Conditions

- Remote staging auth-on repeat pending ops (does not block local feedback sessions).
- Production rollout not approved.
- Broad write scope not approved.
- No backend/frontend code changes from feedback alone unless P0/P1 with repro.
- No PUT/save in evidence/docs packs without separate approval.

## Target Outcome

| Outcome | Gate |
|---------|------|
| ≥1 real form per entity type (TO, SH, BR) | Week-3 execution plan exit |
| Zero unowned P0 | Required |
| P1 owned within 24h | Required when P1 occurs |
| Feedback log updated | After each session |
| Next pack | **First Operator Feedback Session Pack v0.1** executes this plan |

**Success:** Evidence status moves from **NO_SUBMISSIONS_YET** → **SUBMISSIONS_COLLECTED**.
