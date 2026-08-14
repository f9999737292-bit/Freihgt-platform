# Pilot On-Call Assignment v0.1

**Status:** INCOMPLETE — launch blocker **BLOCKER-003** not closed.

Do not treat this sheet as operational assignment until all required fields are `ASSIGNED` and `ACKNOWLEDGED=YES`.

---

## Documented Names (Repository Evidence Only)

These names appear in existing low-code Pilot rollback/approval docs. **Not independently verified for Control Tower Pilot scope.** Contact channels were **not provided** in source documents.

| Name | Documented role (legacy low-code Pilot) | Contact |
| --- | --- | --- |
| Артем Асаев | Rollback owner, Release owner | **not provided** |
| Феликс Асаев | Business/PM, Go/no-go, production data owner | **not provided** |

Source: `LOW_CODE_PILOT_WEEK3_ROLLBACK_OWNER_*`, `LOW_CODE_PILOT_WEEK3_OPERATOR_FEEDBACK_LOG_V0.1.md`

---

## Required Pilot Roles

| Role | Assignee | Coverage | Channel | ACK | Status |
| --- | --- | --- | --- | --- | --- |
| PILOT_BUSINESS_OWNER | Феликс Асаев (documented) | TBD | TBD | NO | PARTIAL |
| PILOT_TECHNICAL_OWNER | TBD | TBD | TBD | NO | **UNASSIGNED** |
| PILOT_OPERATIONS_OWNER | TBD | TBD | TBD | NO | **UNASSIGNED** |
| P1_INCIDENT_COMMANDER | TBD | TBD | TBD | NO | **UNASSIGNED** |
| BACKEND_OWNER | TBD | TBD | TBD | NO | **UNASSIGNED** |
| FRONTEND_OWNER | TBD | TBD | TBD | NO | **UNASSIGNED** |
| INFRASTRUCTURE_OWNER | TBD | TBD | TBD | NO | **UNASSIGNED** |
| DATABASE_OWNER | TBD | TBD | TBD | NO | **UNASSIGNED** |
| SECURITY_OWNER | TBD | TBD | TBD | NO | **UNASSIGNED** |
| GO_LIVE_AUTHORITY | Феликс Асаев (documented) | TBD | TBD | NO | PARTIAL |
| ROLLBACK_AUTHORITY | Артем Асаев (documented) | TBD | TBD | NO | PARTIAL |

---

## Coverage Requirements (Unmet)

- [ ] P1 coverage assigned with contact channel
- [ ] P2 coverage assigned with contact channel
- [ ] Database escalation assigned
- [ ] Infrastructure escalation assigned
- [ ] Security escalation assigned
- [ ] Business launch authority acknowledged for **Control Tower Pilot**
- [ ] `PILOT_SUPPORT_WINDOW` defined

---

## Proposed ACK Targets (Not Approved)

| Severity | Proposed ACK | Status |
| --- | --- | --- |
| P1 | 15m | PROPOSED |
| P2 | 30m | PROPOSED |
| P3 | 4h | PROPOSED |

---

## Verdict

```text
ON_CALL_OWNERSHIP=BLOCKED
BLOCKER=CONTACT_CHANNEL_AND_ROLE_ASSIGNMENT_INCOMPLETE
MANAGEMENT_ACTION_REQUIRED=YES
```

**Unclosed fields (management):** P1_INCIDENT_COMMANDER, PILOT_OPERATIONS_OWNER, PILOT_TECHNICAL_OWNER, INFRASTRUCTURE_OWNER, DATABASE_OWNER, SECURITY_OWNER, ESCALATION_CHANNEL, PILOT_SUPPORT_WINDOW, ACK for all roles.

**Remaining action:** Authorized owners must confirm roles, contact channels, and ACK before Pilot launch.
