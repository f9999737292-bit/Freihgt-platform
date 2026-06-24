# Low-code Pilot Week-3 First Operator Feedback Session Notes Template v0.1

Copy one template per entity scenario during live operator session.

Reference: `LOW_CODE_PILOT_WEEK3_FIRST_OPERATOR_FEEDBACK_SESSION_V0.1.md`, `LOW_CODE_PILOT_WEEK3_OPERATOR_FEEDBACK_FORM_TEMPLATE_V0.1.md`

**Rules:** Read-only session default — **do not click Save** unless separate approved controlled write pack.

---

## Session Date

`YYYY-MM-DD HH:MM`

## Facilitator

Name / role: _______________________________________________

## Operator / Role

Name / role: _______________________________________________

Example roles: shipper logist, carrier dispatcher, finance manager, platform admin (observer only)

## Environment

| Item | Value |
|------|-------|
| web-admin URL | `http://localhost:3000` (dev) or staging URL |
| Tenant ID | `74519f22-ff9b-4a8b-8fff-a958c689682f` |
| Login used | _______________________________________________ |
| Auth-on mode | default-off / staging auth-on |

## Entity Tested

- [ ] TRANSPORT_ORDER — DEMO-TO-001
- [ ] SHIPMENT — DEMO-SH-PLANNED
- [ ] BILLING_REGISTER — DEMO-BR-001

**entity_id:** _______________________________________________

**Route used:** _______________________________________________

**Fallback used?** yes / no — `/low-code/custom-field-values`

## Scenario

- [ ] Baseline read-only review (TO)
- [ ] Limited write pilot review — read-first (SH)
- [ ] Limited write pilot review — read-first (BR)
- [ ] Other: _______________________________________________

## Observations

### Panel visibility

- [ ] Panel visible immediately
- [ ] Panel slow to load
- [ ] Panel missing / blank

Notes: _______________________________________________

### Values displayed

- [ ] Values match expectation
- [ ] Some empty / wrong
- [ ] Confusing formatting

Fields reviewed: _______________________________________________

### Labels / help text

- [ ] Clear
- [ ] Partially unclear
- [ ] Unclear

Unclear labels: _______________________________________________

### Browser console

- [ ] No critical errors
- [ ] Errors observed (paste below)

```
(console output)
```

## Operator Feedback

(Direct quotes or paraphrase — do not invent if operator silent)

_______________________________________________

_______________________________________________

## Confusing Points

_______________________________________________

_______________________________________________

## Safety Concerns

- [ ] None
- [ ] SHIPMENT context confusion
- [ ] BILLING_REGISTER financial/payment concern
- [ ] Permission/access confusion
- [ ] Other

Details: _______________________________________________

**BR reminder given:** low-code fields do not change core billing/payment status — yes / no

## Errors / Screenshots

Reference (secure storage — no secrets in repo):

_______________________________________________

## Severity

- [ ] P0 — stop pilot
- [ ] P1 — fix before next session
- [ ] P2 — backlog
- [ ] P3 — note only
- [ ] N/A — no issues reported

## Decision

Operator decision:

- [ ] **GO**
- [ ] **GO_WITH_CONDITIONS**
- [ ] **STOP**

Conditions: _______________________________________________

## Follow-up Actions

| Action | Owner | Due |
|--------|-------|-----|
| Add row to feedback log (`FB-W3-###`) | facilitator | Same day |
| Complete form template | operator | Same day |
| Escalate P0/P1 | PM | Immediate if P0/P1 |
| | | |

---

**After session:** transfer summary to `LOW_CODE_PILOT_WEEK3_OPERATOR_FEEDBACK_FORM_TEMPLATE_V0.1.md` and feedback log.
