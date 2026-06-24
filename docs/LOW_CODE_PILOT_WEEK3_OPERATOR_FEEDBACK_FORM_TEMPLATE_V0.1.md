# Low-code Pilot Week-3 Operator Feedback Form Template v0.1

Copy one form per feedback session or issue. Submit to pilot lead for logging in `LOW_CODE_PILOT_WEEK3_OPERATOR_FEEDBACK_LOG_V0.1.md`.

Reference: `LOW_CODE_PILOT_WEEK3_OPERATOR_FEEDBACK_COLLECTION_V0.1.md`, `LOW_CODE_PILOT_OPERATOR_CHECKLIST_V0.1.md`.

---

## Operator Name

_______________________________________________

## Date / Time

`YYYY-MM-DD HH:MM`

## Entity Type

- [ ] TRANSPORT_ORDER
- [ ] SHIPMENT
- [ ] BILLING_REGISTER

## Entity ID / Demo Name

| Entity | Demo name | entity_id |
|--------|-----------|-----------|
| TRANSPORT_ORDER | DEMO-TO-001 | `2db04b49-665c-469f-bcb1-ffeb1274fedb` |
| SHIPMENT | DEMO-SH-PLANNED | `14d405e2-0152-4030-b356-eec464a3cc66` |
| BILLING_REGISTER | DEMO-BR-001 | `cf7dbc77-395f-42a2-9717-476e4cd93796` |

Filled: _______________________________________________

## Scenario Tested

Example: read-only panel review / limited write on approved fields / audit lookup

_______________________________________________

_______________________________________________

## Was The Panel Visible?

- [ ] Yes — loaded without issue
- [ ] Partial — slow or missing sections
- [ ] No — panel not found or blank

Notes: _______________________________________________

## Were Values Correct?

- [ ] Yes — matched expectation
- [ ] Partial — some fields wrong or empty
- [ ] No — incorrect or confusing values

Notes: _______________________________________________

## Were Field Labels Clear?

- [ ] Yes
- [ ] Partial
- [ ] No

Unclear fields: _______________________________________________

## Did Validation Make Sense?

- [ ] Yes — errors were understandable
- [ ] Partial — some errors confusing
- [ ] No — validation blocked without clear reason
- [ ] N/A — read-only only

Error example: _______________________________________________

## Did Save Behavior Make Sense?

- [ ] Yes — success/error clear
- [ ] Partial — unclear if save succeeded
- [ ] No — confusing or missing feedback
- [ ] N/A — no save attempted

Notes: _______________________________________________

## Audit Visibility

- [ ] Found audit history easily
- [ ] Found with difficulty
- [ ] Could not find audit
- [ ] N/A — did not look

Notes: _______________________________________________

## Permission / Access Issues

- [ ] None
- [ ] Could not access expected page
- [ ] Admin vs runtime access confusing
- [ ] Role blocked unexpectedly

Details: _______________________________________________

## Financial/Core Safety Concerns

- [ ] None
- [ ] Concern about billing/payment impact (BILLING_REGISTER)
- [ ] Concern about shipment/status impact (SHIPMENT)
- [ ] Other safety concern

Details: _______________________________________________

## Error Message / Screenshot Reference

```
(paste toast, console error, API response snippet, or link to secure storage — no secrets)
```

## Severity

- [ ] **P0** — stop pilot
- [ ] **P1** — fix before next pilot day
- [ ] **P2** — backlog before expansion
- [ ] **P3** — note only

## Suggested Improvement

_______________________________________________

_______________________________________________

## Operator Decision

- [ ] **GO** — comfortable continuing pilot on this entity
- [ ] **GO_WITH_CONDITIONS** — continue with noted issues
- [ ] **STOP** — do not continue until issue resolved

Conditions / stop reason: _______________________________________________

## Notes

_______________________________________________

_______________________________________________

---

**Pilot tenant:** `74519f22-ff9b-4a8b-8fff-a958c689682f`

**Do not include passwords, tokens, or production data in this form.**
