# Low-code Pilot Week-3 Selectel SSH SG Panel Confirmation Evidence v0.1

## Summary

Operator executed the Selectel SSH SG Panel Confirmation Pack v0.1 from the trusted workstation.

Re-verification passed for API health, trusted SSH, runtime, and UFW database port denial. Selectel Security Group panel change could **not** be applied from this automation environment — no Selectel control panel or OpenStack CLI access in scope.

STG-LIM-003 remains **open**.

## Decision

```text
SELECTEL_SSH_SG_PANEL_CONFIRMATION_REVERIFIED_SG_PANEL_CHANGE_PENDING
```

## STG-LIM-003 Status

```text
OPEN — trusted SSH PASS; Selectel SG /32 panel change not applied; non-trusted IP test pending
```

## Production-ready Status

```text
not claimed
```

## Operator Directive

```text
сделай следующий шаг от лица оператора
```

Interpretation:

```text
Execute Selectel SSH SG Panel Confirmation Pack v0.1 — panel SG change + post-change verification
```

## Verification Matrix

| Test ID | Check | Expected | Actual | Result |
| ------- | ----- | -------- | ------ | ------ |
| PC-SSH-SG-001 | Operator directive captured | yes | yes | PASS |
| PC-SSH-SG-002 | Trusted operator IP identified at runtime | yes | yes — not stored in docs | PASS |
| PC-SSH-SG-003 | API health GET | 200 | 200 | PASS |
| PC-SSH-SG-004 | SSH from trusted operator workstation | success | success | PASS |
| PC-SSH-SG-005 | SSH without operator private key | denied | Permission denied (publickey) | PASS |
| PC-SSH-SG-006 | Selectel SG SSH /32 rule applied in panel | yes | not applied — no panel access | FAIL |
| PC-SSH-SG-007 | SSH 22 not open to 0.0.0.0/0 in Selectel SG | yes | not verified | PENDING |
| PC-SSH-SG-008 | SSH from non-trusted IP rejected | rejected | not tested | PENDING |
| PC-SSH-SG-009 | UFW PostgreSQL 5432 deny on VM | deny | deny | PASS |
| PC-SSH-SG-010 | UFW Redis 6379 deny on VM | deny | deny | PASS |
| PC-SSH-SG-011 | Runtime containers healthy | healthy | 10 healthy | PASS |
| PC-SSH-SG-012 | STG-LIM-003 closed | yes | no | FAIL |

## Read-only GET Evidence

| Target | Result |
| ------ | ------ |
| http://161.104.53.221/health | 200 |

## SSH Evidence

Trusted operator SSH session:

```text
success — echo OK; read-only remote checks executed
```

Default SSH without operator private key:

```text
Permission denied (publickey)
```

Operator private key path:

```text
not stored in docs
```

## Remote Read-only Observations (sanitized)

UFW on VM:

* SSH 22: ALLOW from Anywhere (UFW level — does not satisfy STG-LIM-003 alone)
* PostgreSQL 5432: DENY
* Redis 6379: DENY
* HTTP 80 / HTTPS 443: ALLOW
* Internal service ports 8080, 8088, 3000, 5173: DENY

Runtime:

```text
10 containers healthy (api gateway, low-code, identity, document, rfx, company, transport-order, shipment, billing-register, postgres)
```

OpenStack instance metadata (SG rules):

```text
not available — metadata endpoint returned no usable security group detail
```

## Why Selectel Panel Change Was Not Applied

| Blocker | Detail |
| ------- | ------ |
| Selectel control panel | not accessible from agent automation environment |
| OpenStack CLI | not installed on operator workstation |
| Provider API credentials | not in scope for this pack — must not be stored in repo |

## Operator Panel Action Still Required (manual)

1. Log in to Selectel control panel.
2. Open cloud server attached to `161.104.53.221`.
3. Open bound Security Group inbound rules.
4. Add TCP 22 ingress limited to trusted operator IP /32 only.
5. Remove any SSH rule allowing `0.0.0.0/0` or `::/0`.
6. Confirm PostgreSQL 5432 and Redis 6379 are not open in Security Group.
7. Confirm HTTP 80 and HTTPS 443 remain open.
8. Save changes.
9. Re-run post-panel verification pack.

Trusted operator IP must be entered in Selectel panel only — **not stored in this repository**.

Reference runbook:

```text
docs/LOW_CODE_PILOT_WEEK3_SELECTEL_SSH_SECURITY_GROUP_RESTRICTION_RUNBOOK_V0.1.md
```

## Safety Confirmation

Remote SSH executed:

```text
yes — read-only verification commands only
```

Remote writes executed:

```text
no
```

Selectel panel changes executed:

```text
no — blocked by missing panel access
```

Secrets captured:

```text
no
```

Operator IP stored in docs:

```text
no
```

SSH private key path/name stored in docs:

```text
no
```

Production-ready claimed:

```text
no
```

## Next Recommended Event

```text
operator completes Selectel SG SSH /32 change in control panel, then re-run post-panel verification with non-trusted IP rejection test
```

## Next Pack

```text
Selectel SSH SG Post-Panel Verification Pack v0.1
```
