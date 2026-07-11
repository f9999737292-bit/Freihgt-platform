# Low-code Pilot Week-3 Selectel SSH Security Group Restriction Verification Note v0.1

## Summary

Verification note for STG-LIM-003 execution attempt.

Execution is blocked pending operator input. Baseline API health check passed.

## Decision

```text
SELECTEL_SSH_SG_RESTRICTION_VERIFICATION_BLOCKED_PENDING_OPERATOR_EXECUTION
```

## Verification Matrix

| Check | Required | Executed | Result |
| ----- | -------- | -------- | ------ |
| Operator approval | yes | no | BLOCKED |
| Selectel SG SSH /32 rule | yes | no | PENDING |
| SSH trusted IP success | yes | no | PENDING |
| SSH non-trusted rejected | yes | no | PENDING |
| API health GET 200 | yes | yes | PASS |
| STG-LIM-003 closed | no | no | OPEN |

## STG-LIM-003

```text
OPEN
```

## Production-ready Status

```text
not claimed
```

## Operator Action Required

Follow `docs/LOW_CODE_PILOT_WEEK3_SELECTEL_SSH_SECURITY_GROUP_RESTRICTION_RUNBOOK_V0.1.md` in Selectel panel, then provide:

```text
Selectel SSH SG restriction approval: yes
Trusted operator IP: <separately>
Selectel SG change completed: yes
```

After operator execution, re-run execution evidence pack to close STG-LIM-003.
