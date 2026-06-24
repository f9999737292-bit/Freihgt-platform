# Low-code Pilot Week-3 PM Scheduling Action Plan v0.1

## Purpose

Action plan for PM to schedule and execute Week-3 low-code pilot operator feedback sessions after **PM_SCHEDULING_DECISION_REQUIRED** (Option B — scheduling blocked until PM assigns owner and calendar).

**Real operator submissions:** **0**

Reference: `LOW_CODE_PILOT_WEEK3_PM_SCHEDULING_DECISION_V0.1.md`

## Owner

| Role | Assigned |
|------|----------|
| PM / pilot owner | **TBD** |
| Pilot lead (facilitator) | **TBD** |
| Operator lead (participant nomination) | **TBD** |

**Owner action:** Assign named persons before next feedback capture pack.

## Required Participants

| Role | Purpose | Assigned |
|------|---------|----------|
| Logistics / shipment operator | TO baseline and/or SH session | **TBD** |
| Billing / finance operator | BR session — **mandatory** | **TBD** |
| Platform admin (observer) | Technical support only | **TBD** |
| PM | Scheduling owner, P0/P1 decisions | **TBD** |
| Pilot lead | Facilitation, log updates | **TBD** |

## Required Sessions

| # | Entity | Demo | entity_id | template_code | Timebox | Slot |
|---|--------|------|-----------|---------------|---------|------|
| 1 | TRANSPORT_ORDER | DEMO-TO-001 | `2db04b49-665c-469f-bcb1-ffeb1274fedb` | `transport_order_default` | 30 min | **TBD** |
| 2 | SHIPMENT | DEMO-SH-PLANNED | `14d405e2-0152-4030-b356-eec464a3cc66` | `shipment_default` | 45 min | **TBD** |
| 3 | BILLING_REGISTER | DEMO-BR-001 | `cf7dbc77-395f-42a2-9717-476e4cd93796` | `billing_register_default` | 45 min | **TBD** |
| — | PM wrap-up | — | — | — | 15 min | **TBD** |

**Total:** ~2h 15min (may split across 2 days).

**Environment:** Local dev `http://localhost:3000` or staging when available.

## Proposed Calendar Slots

| Window | Proposal |
|--------|----------|
| PM owner assignment | By **2026-06-27** |
| Operator nomination | By **2026-06-26** |
| Session 1 (TO) | **TBD** — propose 2026-06-30 AM |
| Session 2 (SH) | **TBD** — propose 2026-06-30 PM or 2026-07-01 AM |
| Session 3 (BR) | **TBD** — propose 2026-07-01 AM |
| PM wrap-up | **TBD** — same day as Session 3 |

All slots **TBD** until PM confirms calendar.

## Evidence Required

Per session:

- [ ] `LOW_CODE_PILOT_WEEK3_OPERATOR_FEEDBACK_FORM_TEMPLATE_V0.1.md` completed
- [ ] Feedback log row (`FB-W3-001+`)
- [ ] Session schedule template filled
- [ ] Optional screenshots (secure storage)
- [ ] P0/P1 escalated same day if found
- [ ] PM wrap-up decision documented

## Preparation Checklist

- [ ] Named PM owner assigned
- [ ] Operators nominated (logistics + billing/finance)
- [ ] Platform admin observer assigned
- [ ] Calendar invites sent (2h 15min or split)
- [ ] Demo entities seeded (`make seed-lowcode-demo`)
- [ ] Form template + session notes template distributed
- [ ] SH/BR quick guides shared (if available)
- [ ] BR financial safety briefing prepared for Session 3
- [ ] health-check passed before sessions

## During-session Checklist

- [ ] Confirm correct tenant and demo entity
- [ ] Walk through allowed fields only (SH/BR limited write scope)
- [ ] Capture panel visibility, labels, values, audit understanding
- [ ] BR: confirm operator understands no core billing/payment status change
- [ ] Complete feedback form during or immediately after session
- [ ] Note permission/access issues if any

## After-session Checklist

- [ ] Add `FB-W3-001+` to feedback log
- [ ] Run triage per runbook
- [ ] Update improvements backlog for P0/P1/P2/P3
- [ ] PM wrap-up: severity summary + next pack decision
- [ ] Execute **First Real Operator Feedback Capture Retry Pack v0.1**

## Escalation Rules

| Condition | Action |
|-----------|--------|
| P0 found in session | **STOP** — Runtime Pilot Fix Pack same day |
| P1 found | Fix before next write session; P1 Fix Design Pack |
| Operator no-show | Reschedule; log NEEDS_INFO; PM decision if repeated |
| Operator unavailable past 2026-06-27 | PM evaluates Option D (stop feedback track) |
| PM override requested | PM Override Decision Pack — not default |

## Next Decision

**Current:** PM_SCHEDULING_DECISION_REQUIRED — owner/date **TBD**

**After PM assigns owner + calendar:**

- Decision → **PM_SCHEDULED**
- Next pack → **First Real Operator Feedback Capture Retry Pack v0.1**

**If scheduling remains blocked past deadline:**

- Re-run **Operator Feedback Scheduling Follow-up Pack v0.1**
