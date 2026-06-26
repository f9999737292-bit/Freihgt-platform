# Low-code Pilot Week-3 Production Data Owner Approval Form v0.1

## Purpose

Capture **named owners** and **explicit approvals** for the Week-3 low-code production data policy (PR-GAP-002). This form does **not** approve production-ready status or production data use unless explicitly marked.

Reference: `LOW_CODE_PILOT_WEEK3_PRODUCTION_DATA_POLICY_V0.1.md`

## How To Fill

1. PM or governance lead distributes form to required owners
2. Each owner completes their section with name, role, contact, and approval status
3. Confirmations checked **yes/no** — do not leave ambiguous
4. Completed form attached to **Production Data Owner Final Approval Pack v0.1**
5. Do **not** include secrets, JWT, tokens, or production data in this form

## Approval Form

```text
Product / Data Owner:
ФИО:
Роль:
Контакт:
Approval status: approved / not approved / pending

Legal / Compliance Owner:
ФИО:
Роль:
Контакт:
Approval status: approved / not approved / pending

Finance Owner, if needed:
ФИО:
Роль:
Контакт:
Approval status: approved / not approved / not applicable

Confirmations:
- Production data policy reviewed: yes/no
- Real production data use approved: yes/no
- Secrets/JWT/tokens forbidden in repo: yes/no
- Signed legal documents excluded: yes/no
- Payment data excluded: yes/no
- Low-code fields are not legal/financial source of truth without approval: yes/no
- Production-ready claimed: no
```

## Required Confirmations

| Confirmation | Expected for approval |
|--------------|----------------------|
| Production data policy reviewed | **yes** |
| Real production data use approved | **yes** or explicitly **no** (forbidden) |
| Secrets/JWT/tokens forbidden in repo | **yes** |
| Signed legal documents excluded | **yes** |
| Payment data excluded | **yes** |
| Low-code fields not SoT without approval | **yes** |
| Production-ready claimed | **no** (always) |

## Forbidden Items

Do **not** include in completed form or committed docs:

- Passwords, JWT, tokens, API keys, private keys
- Real production personal data
- Real production financial data
- Payment card or bank account details
- Raw production database dumps
- Signed legal document contents

## Decision Values

| Value | Meaning |
|-------|---------|
| **DATA_OWNER_APPROVAL_PROVIDED** | All required owners approved; confirmations complete |
| **DATA_OWNER_APPROVAL_PARTIAL** | Some owners approved; gap remains open |
| **DATA_OWNER_APPROVAL_REJECTED** | Owner rejected policy or production data use |
| **DATA_OWNER_ASSIGNMENT_PENDING** | Names or approvals still missing |

## Next Pack

**Low-code Pilot Week-3 Production Data Owner Final Approval Pack v0.1**

**Trigger:** Production data owner final approval provided

Reference: `LOW_CODE_PILOT_WEEK3_PRODUCTION_DATA_OWNER_ASSIGNMENT_V0.1.md`
