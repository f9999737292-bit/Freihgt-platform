# Low-code Pilot Week-3 Operator Feedback Session Schedule Template v0.1

Copy one template per scheduled session (TO, SH, BR).

Reference: `LOW_CODE_PILOT_WEEK3_OPERATOR_FEEDBACK_PM_ESCALATION_V0.1.md`, `LOW_CODE_PILOT_WEEK3_FIRST_OPERATOR_FEEDBACK_ACTION_PLAN_V0.1.md`

**Rules:** Read-only by default — **do not click Save** unless separate approved controlled write pack.

---

## Session Date

`YYYY-MM-DD HH:MM – HH:MM`

## Facilitator

Name / role: _______________________________________________

## PM Owner

Name / role: _______________________________________________

## Operator / Role

Name / role: _______________________________________________

Example: shipper logist, carrier dispatcher, finance manager

## Environment

| Item | Value |
|------|-------|
| URL | `http://localhost:3000` / staging: __________ |
| Tenant ID | `74519f22-ff9b-4a8b-8fff-a958c689682f` |
| Login account | _______________________________________________ |

## Entity Scope

- [ ] **Session 1 — TRANSPORT_ORDER** — DEMO-TO-001 (`2db04b49-665c-469f-bcb1-ffeb1274fedb`)
- [ ] **Session 2 — SHIPMENT** — DEMO-SH-PLANNED (`14d405e2-0152-4030-b356-eec464a3cc66`)
- [ ] **Session 3 — BILLING_REGISTER** — DEMO-BR-001 (`cf7dbc77-395f-42a2-9717-476e4cd93796`)

## Scenario

- [ ] Baseline read-only review (TO)
- [ ] Limited-write pilot review — read-first (SH)
- [ ] Limited-write pilot review — read-first + financial safety (BR)

## Timebox

| Block | Planned duration |
|-------|------------------|
| Brief + scope | ___ min |
| Entity walkthrough | TO: 30 / SH: 45 / BR: 45 min |
| Form completion | ___ min |
| PM wrap-up (if last session of day) | 15 min |

## Required Screens

| Screen | Route | Fallback |
|--------|-------|----------|
| Entity detail | `/transport-orders/{id}` / `/shipments/{id}` / `/billing-registers/{id}` | `/low-code/custom-field-values` |
| Audit (optional) | Low-code audit UI or API | `GET .../audit-events` |
| Login | `/login` | — |

Screens visited: _______________________________________________

## Required Evidence

- [ ] Feedback form completed (`LOW_CODE_PILOT_WEEK3_OPERATOR_FEEDBACK_FORM_TEMPLATE_V0.1.md`)
- [ ] Feedback log updated (`FB-W3-###`)
- [ ] Session notes (`LOW_CODE_PILOT_WEEK3_FIRST_OPERATOR_FEEDBACK_SESSION_NOTES_TEMPLATE_V0.1.md`)
- [ ] Screenshots (if errors) — secure storage reference: __________
- [ ] P0/P1 escalated to PM (if any)

## Feedback Form Link / Location

File path / shared folder: _______________________________________________

## Stop Conditions

**STOP session and escalate P0** if:

- Wrong tenant data visible
- Operator reports data corruption concern
- Billing operator believes payment/status could change via low-code panel
- Critical console errors block panel use

**Do not Save** — record Save test request as feedback for separate approved pack.

## Follow-up Owner Actions

| Action | Owner | Due |
|--------|-------|-----|
| Triage feedback (P0/P1 same day) | PM | Session day |
| Update improvements backlog | Pilot lead | Session day |
| Schedule next entity session (if split) | PM | __________ |
| Run Capture Pack after all 3 sessions | Pilot lead | After Session 3 |

---

**Session completed:** yes / no / cancelled — reason: _______________________________________________
