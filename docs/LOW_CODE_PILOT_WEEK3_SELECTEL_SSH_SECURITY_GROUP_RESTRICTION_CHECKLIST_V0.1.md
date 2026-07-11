# Low-code Pilot Week-3 Selectel SSH Security Group Restriction Checklist v0.1

## Summary

Checklist for STG-LIM-003 — SSH 22 restriction via Selectel Security Group.

Preparation pack only. Execution not performed in this step.

## Staging Limitation

```text
STG-LIM-003
```

## Current Status

```text
OPEN — panel confirmation re-verified; Selectel SG /32 panel change pending manual operator action
```

## Preparation Checklist

| # | Item | Status |
| - | ---- | ------ |
| 1 | Runbook created | PASS |
| 2 | Trusted operator IP identified | PASS — not stored in docs |
| 3 | Selectel panel access confirmed | PENDING — manual panel action required |
| 4 | Break-glass Console access confirmed | PENDING |
| 5 | Security Group change approved | PASS |

## Execution Checklist (operator — not done in this pack)

| # | Item | Required | Current | Status |
| - | ---- | -------- | ------- | ------ |
| 1 | SSH 22 restricted to trusted IP /32 in Selectel SG | yes | no | PENDING |
| 2 | SSH 22 not open to 0.0.0.0/0 in Selectel SG | yes | unknown | PENDING |
| 3 | PostgreSQL 5432 not open in Selectel SG | yes | unknown | PENDING |
| 4 | Redis 6379 not open in Selectel SG | yes | unknown | PENDING |
| 5 | HTTP 80 open in Selectel SG | yes | yes | PASS |
| 6 | HTTPS 443 open in Selectel SG | yes | yes | PASS |
| 7 | SSH from trusted IP verified | yes | success | PASS |
| 8 | SSH from non-trusted IP rejected | yes | not executed | PENDING |
| 9 | API health GET still 200 | yes | yes | PASS |
| 10 | Sanitized evidence captured | yes | yes | PASS |

## Safety

* No SSH executed in preparation pack: **yes**
* No secrets captured: **yes**
* No production-ready claimed: **yes**
* UFW alone does not close STG-LIM-003: **acknowledged**

## Decision

```text
SELECTEL_SSH_SG_PANEL_CONFIRMATION_REVERIFIED_SG_PANEL_CHANGE_PENDING
```

## STG-LIM-003 After This Pack

```text
OPEN — trusted SSH PASS; Selectel SG /32 panel change pending
```

## Production-ready Status

```text
not claimed
```

## Required Operator Input Before Execution

```text
Selectel SSH SG restriction approval: yes
Trusted operator IP: <provided separately, not stored in docs>
```

## Next Pack After Execution

```text
Selectel SSH SG Post-Panel Verification Pack v0.1
```
