# Low-code Pilot Week-3 Selectel SSH Security Group Restriction Execution Evidence v0.1

## Summary

Execution evidence capture attempted for STG-LIM-003.

Selectel Security Group restriction has **not** been applied in this pack. Operator execution in Selectel panel is still required.

## Decision

```text
SELECTEL_SSH_SG_RESTRICTION_EXECUTION_BLOCKED_PENDING_OPERATOR_INPUT
```

## STG-LIM-003 Status

```text
OPEN — execution blocked pending operator approval and Selectel panel action
```

## Production-ready Status

```text
not claimed
```

## Controlled Pilot

```text
continues
```

## Staging Target

Provider:

```text
Selectel
```

Server IP:

```text
161.104.53.221
```

Staging URL:

```text
http://161.104.53.221
```

## Reference Documents

```text
docs/LOW_CODE_PILOT_WEEK3_SELECTEL_SSH_SECURITY_GROUP_RESTRICTION_RUNBOOK_V0.1.md
docs/LOW_CODE_PILOT_WEEK3_SELECTEL_SSH_SECURITY_GROUP_RESTRICTION_CHECKLIST_V0.1.md
```

## Execution Status

| Item | Status |
| ---- | ------ |
| Operator approval captured | no |
| Trusted operator IP provided | no — not stored in docs |
| Selectel SG rules changed | no |
| SSH from trusted IP verified | not executed |
| SSH from non-trusted IP rejected | not executed |
| Break-glass Console tested | not executed |

## Baseline Read-only GET (pre-SG-change)

Mode:

```text
read-only GET
```

| Test ID | Target | Expected | Actual | Result |
| ------- | ------ | -------- | ------ | ------ |
| PRE-SSH-SG-001 | http://161.104.53.221/health | 200 | 200 | PASS |

API gateway route via /health returned 200 at time of check.

## Why STG-LIM-003 Remains Open

SSH Security Group restriction requires:

1. Explicit operator approval
2. Trusted operator IP (provided separately, not stored in docs)
3. Manual Security Group change in Selectel control panel
4. Post-change SSH verification from trusted and non-trusted sources
5. Sanitized evidence capture without secrets or operator IP in repo

This pack does not substitute for Selectel panel execution.

## Required Operator Input

```text
Selectel SSH SG restriction approval: yes
Trusted operator IP: <provided separately, not stored in docs>
Selectel SG change completed: yes
```

## Safety Confirmation

Backend code changed:

```text
no
```

Frontend code changed:

```text
no
```

Remote SSH executed:

```text
no
```

Staging writes executed:

```text
no
```

Secrets captured:

```text
no
```

Operator IP stored in docs:

```text
no
```

Production-ready claimed:

```text
no
```

## Next Recommended Event

```text
operator applies Selectel SG restriction per runbook, then re-run execution evidence capture
```

## Next Pack

```text
Selectel SSH Security Group Restriction Execution Evidence Pack v0.1 (re-run after operator execution)
```
