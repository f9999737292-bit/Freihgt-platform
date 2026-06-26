# Low-code Pilot Week-3 Live Operator Session Run Sheet (Confirmed) v0.1

## Session Date / Time

**26.06.2026 12:30**

## PM / Coordinator

**Феликс Асаев**

## Participants

| Operator | Entity type | Demo |
|----------|-------------|------|
| **Пейсахов Семен** | TRANSPORT_ORDER | DEMO-TO-001 |
| **Крылова Любовь** | SHIPMENT | DEMO-SH-PLANNED |
| **Курганова Наталья** | BILLING_REGISTER | DEMO-BR-001 |

**Pilot tenant:** `74519f22-ff9b-4a8b-8fff-a958c689682f`

## Session Agenda

1. Login / access check
2. TRANSPORT_ORDER scenario (Пейсахов Семен)
3. SHIPMENT scenario (Крылова Любовь)
4. BILLING_REGISTER scenario (Курганова Наталья)
5. Feedback form completion (`LOW_CODE_PILOT_WEEK3_OPERATOR_FEEDBACK_FORMS_V0.1.md`)
6. Decision capture (ready / needs changes / blocked per operator)

## Scenario Checklist

### All operators

- [ ] Login successful
- [ ] Correct tenant / role visible
- [ ] Low-code panel loads for assigned entity
- [ ] Active template fields visible
- [ ] Audit history locatable (if applicable to scenario)

### TRANSPORT_ORDER — Пейсахов Семен

- [ ] Open DEMO-TO-001 (or assigned entity)
- [ ] Review transport order custom fields
- [ ] Execute approved limited-write scenario (if permitted)
- [ ] Complete TO feedback form section

### SHIPMENT — Крылова Любовь

- [ ] Open DEMO-SH-PLANNED (or assigned entity)
- [ ] Review shipment custom fields
- [ ] Execute approved limited-write scenario (if permitted)
- [ ] Complete SH feedback form section

### BILLING_REGISTER — Курганова Наталья

- [ ] Open DEMO-BR-001 (or assigned entity)
- [ ] Review billing register custom fields
- [ ] Financial safety briefing acknowledged
- [ ] Execute approved limited-write scenario (if permitted)
- [ ] Complete BR feedback form section

## Feedback Collection Rules

1. Each operator completes **their own** form section — no proxy answers.
2. Use `LOW_CODE_PILOT_WEEK3_OPERATOR_FEEDBACK_FORMS_V0.1.md`.
3. PM collects signed or confirmed digital forms after session.
4. Log entries go to `LOW_CODE_PILOT_WEEK3_OPERATOR_FEEDBACK_LOG_V0.1.md`.
5. Empty fields stay **TBD** until operator provides input.

## No-Fake-Feedback Rule

- Do **not** invent operator comments, ratings, or decisions.
- Do **not** mark forms complete without operator input.
- If operator absent or session incomplete → document **SESSION_INCOMPLETE**, do not fabricate feedback.

## Escalation Rules

| Condition | Action |
|-----------|--------|
| P0 — data loss, wrong tenant, blocking error | Stop session → Runtime Pilot Fix Pack v0.1 |
| P1 — feature unusable for scenario | Document; continue other operators if safe |
| Operator cannot access system | PM reschedules; update feedback log |
| Operator refuses limited write | Read-only review OK; note in form |

## Final Session Output

Expected deliverables after session:

| Deliverable | Owner | Status |
|-------------|-------|--------|
| Completed TO form (Пейсахов Семен) | Operator + PM | **pending** |
| Completed SH form (Крылова Любовь) | Operator + PM | **pending** |
| Completed BR form (Курганова Наталья) | Operator + PM | **pending** |
| Session notes (issues, P0/P1) | Феликс Асаев | **pending** |
| Feedback log update | Pilot lead | After forms received |

**Next pack when forms arrive:** Low-code Pilot Week-3 Real Operator Feedback Intake Pack v0.1

Reference: `LOW_CODE_PILOT_WEEK3_FIRST_REAL_OPERATOR_FEEDBACK_CAPTURE_RETRY_V0.1.md`
