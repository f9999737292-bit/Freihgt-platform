# Low-code Pilot Week-3 Selectel SSH SG Restriction Re-run Evidence v0.1

## Summary

Operator re-run evidence capture for STG-LIM-003 after operator approval.

Selectel Security Group restriction could not be verified as applied. SSH verification blocked by missing operator SSH key. API baseline remains healthy.

## Decision

```text
SELECTEL_SSH_SG_RE_RUN_PARTIAL_VERIFICATION_STG_LIM_003_OPEN
```

## Operator Approval

```text
SELECTEL_SSH_SG_OPERATOR_APPROVAL_CAPTURED
```

Reference:

```text
docs/LOW_CODE_PILOT_WEEK3_SELECTEL_SSH_SG_OPERATOR_APPROVAL_CAPTURE_V0.1.md
```

## STG-LIM-003 Status

```text
OPEN — SG panel change not verified; SSH key not configured on operator workstation
```

## Production-ready Status

```text
not claimed
```

## Verification Matrix

| Test ID | Check | Expected | Actual | Result |
| ------- | ----- | -------- | ------ | ------ |
| OP-SSH-SG-001 | Operator approval captured | yes | yes | PASS |
| OP-SSH-SG-002 | Trusted operator IP identified | yes | yes — not stored in docs | PASS |
| OP-SSH-SG-003 | Selectel SG SSH /32 rule applied | yes | not verified | FAIL |
| OP-SSH-SG-004 | SSH 22 not open to 0.0.0.0/0 in SG | yes | not verified | PENDING |
| OP-SSH-SG-005 | SSH from trusted operator workstation | success | Permission denied (publickey) | FAIL |
| OP-SSH-SG-006 | SSH from non-trusted IP rejected | rejected | not tested | PENDING |
| OP-SSH-SG-007 | API health GET | 200 | 200 | PASS |
| OP-SSH-SG-008 | STG-LIM-003 closed | yes | no | FAIL |

## Read-only GET Evidence

| Target | Result |
| ------ | ------ |
| http://161.104.53.221/health | 200 |

## SSH Observation

SSH connection reached authentication stage on port 22.

Result:

```text
Permission denied (publickey)
```

Interpretation:

* Port 22 is reachable from operator workstation
* Selectel SG restriction to trusted IP /32 is **not evidenced**
* Operator SSH private key is **not configured** on workstation

## Blockers

| Blocker | Status |
| ------- | ------ |
| Selectel SG change in control panel | not verified |
| Operator SSH key for staging server | not configured |
| Non-trusted IP SSH rejection test | not executed |

## Operator Panel Action Still Required

1. Open Selectel control panel for server `161.104.53.221`
2. Edit Security Group inbound rules
3. Restrict TCP 22 to trusted operator IP /32 only
4. Remove broad SSH allow rules if present
5. Confirm PostgreSQL 5432 and Redis 6379 remain closed in SG
6. Configure operator SSH key on workstation
7. Re-run verification pack

Operator IP must be entered in Selectel panel only — **not stored in this repository**.

## Safety Confirmation

Remote SSH commands executed:

```text
yes — connection test only, no remote writes
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
operator applies Selectel SG restriction in panel and configures SSH key, then re-run verification
```

## Next Pack

```text
Selectel SSH SG Restriction Verification Pack v0.1 (after panel change and SSH key configured)
```
