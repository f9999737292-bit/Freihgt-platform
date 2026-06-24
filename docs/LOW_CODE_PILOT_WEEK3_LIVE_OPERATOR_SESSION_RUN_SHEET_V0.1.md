# Low-code Pilot Week-3 Live Operator Session Run Sheet v0.1

## Purpose

Facilitator run sheet for live Week-3 low-code pilot operator feedback sessions. Use during TO / SH / BR walkthroughs. **Not a substitute for completed feedback forms.**

Reference: `LOW_CODE_PILOT_WEEK3_LIVE_OPERATOR_SESSION_SCHEDULING_V0.1.md`, `LOW_CODE_PILOT_WEEK3_LIVE_OPERATOR_SESSION_CONFIRMATION_V0.1.md`

## Confirmation Status

| Session | Confirmed | Operator | Date/time | Capture retry |
|---------|-----------|----------|-----------|---------------|
| 1 — TRANSPORT_ORDER | **pending** | **TBD** | **TBD** (proposed 2026-06-30 09:00) | **blocked** |
| 2 — SHIPMENT | **pending** | **TBD** | **TBD** (proposed 2026-06-30 14:00) | **blocked** |
| 3 — BILLING_REGISTER | **pending** | **TBD** | **TBD** (proposed 2026-07-01 09:00) | **blocked** |

**Decision:** **LIVE_SESSION_CONFIRMATION_PENDING**

Do **not** run **First Real Operator Feedback Capture Retry Pack v0.1** until sessions are **confirmed** and **completed** with real operator feedback forms.

## Facilitator

| Field | Value |
|-------|-------|
| Pilot lead | **TBD** |
| Support | Platform admin observer (**TBD**) |

## PM Owner

**Virtual PM / Pilot Coordinator** (virtual — scheduling coordinator)

## Participants

| Session | Required participant | Assigned |
|---------|---------------------|----------|
| 1 — TO | Logistics / transport order user | **TBD** |
| 2 — SH | Shipment / logistics operator | **TBD** |
| 3 — BR | Billing / finance operator | **TBD** |

## Environment

| Item | Value |
|------|-------|
| Web UI | `http://localhost:3000` (local dev) |
| API gateway | `http://localhost:8080` |
| Tenant ID | `74519f22-ff9b-4a8b-8fff-a958c689682f` |
| Pre-session | `make platform-up-no-build`, `make health-check`, `make seed-lowcode-demo` |

## Login / Access Notes

- Use existing dev admin credentials from **secure internal notes** — **do not record passwords in this doc**.
- Platform admin observer provides technical support only; does **not** substitute for operator feedback.
- Confirm operator sees correct tenant and demo entity before walkthrough.

## Session 1 TRANSPORT_ORDER

| Field | Value |
|-------|-------|
| Demo | DEMO-TO-001 |
| entity_id | `2db04b49-665c-469f-bcb1-ffeb1274fedb` |
| template_code | `transport_order_default` |
| Timebox | 30 min |
| Proposed slot | 2026-06-30 09:00–09:30 (**NEEDS_CONFIRMATION**) |

**Fields to review:** `cargo_class`, `internal_cost_center`, `loading_window_note`

**Walkthrough steps:**

1. Open demo transport order in web-admin.
2. Confirm low-code panel visible.
3. Review each field label and current value.
4. Ask: Are labels clear? Are values correct/understandable?
5. Explain audit expectation (where operator would find change history).
6. Complete live feedback checklist + form.

## Session 2 SHIPMENT

| Field | Value |
|-------|-------|
| Demo | DEMO-SH-PLANNED |
| entity_id | `14d405e2-0152-4030-b356-eec464a3cc66` |
| template_code | `shipment_default` |
| Timebox | 45 min |
| Proposed slot | 2026-06-30 14:00–14:45 (**NEEDS_CONFIRMATION**) |

**Fields to review:** `temperature_mode`, `loading_contact_phone`, `driver_comment`, `planned_pickup_date`, `declared_value`, `handling_flags`

**Walkthrough steps:**

1. Open demo shipment; confirm limited-write field scope.
2. Review rich editors (phone, comment, flags, date, value).
3. Ask: Is save behavior understood? Where is audit visible?
4. Note any role/safety concerns.
5. Complete live feedback checklist + form.

## Session 3 BILLING_REGISTER

| Field | Value |
|-------|-------|
| Demo | DEMO-BR-001 |
| entity_id | `cf7dbc77-395f-42a2-9717-476e4cd93796` |
| template_code | `billing_register_default` |
| Timebox | 45 min |
| Proposed slot | 2026-07-01 09:00–09:45 (**NEEDS_CONFIRMATION**) |

**Fields to review:** `cost_allocation_code`, `approval_group`, `payment_priority`

**Walkthrough steps:**

1. Open demo billing register.
2. **Financial safety briefing:** low-code custom fields do **not** change billing/payment/core status.
3. Confirm operator understands wording and field purpose.
4. Review audit expectation.
5. Complete live feedback checklist + form.

## Questions To Ask

- Is the low-code panel visible where you expect it?
- Are field labels and help text clear?
- Do displayed values match your expectations?
- If you saved (when allowed in pilot), would you know where to find audit history?
- For BR: Do you understand these fields do not change payment or core billing status?
- Any permission or access concerns?
- Severity if blocked: P0 / P1 / P2 / P3?

## Evidence To Capture

- Completed `LOW_CODE_PILOT_WEEK3_LIVE_OPERATOR_FEEDBACK_CHECKLIST_V0.1.md`
- Completed `LOW_CODE_PILOT_WEEK3_OPERATOR_FEEDBACK_FORM_TEMPLATE_V0.1.md`
- Feedback log entry `FB-W3-001+`
- Optional screenshots (secure storage — no secrets)
- Session notes in schedule template

## Stop Rules

| Condition | Action |
|-----------|--------|
| P0 (wrong tenant, audit missing, financial side effect, unsafe rendering) | Stop session; escalate same day; Runtime Fix Pack |
| P1 (blocks next write session) | Log; fix before next pilot day |
| Operator cannot proceed | Reschedule; do not invent feedback |

## Follow-up Actions

1. Virtual PM confirms all three sessions completed or rescheduled.
2. Pilot lead adds `FB-W3-001+` to feedback log.
3. Run triage.
4. Execute **First Real Operator Feedback Capture Retry Pack v0.1**.
