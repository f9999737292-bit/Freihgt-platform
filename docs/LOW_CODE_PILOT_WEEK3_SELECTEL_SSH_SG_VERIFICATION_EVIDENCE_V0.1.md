# Low-code Pilot Week-3 Selectel SSH SG Verification Evidence v0.1

## Summary

Verification pack executed for STG-LIM-003 after operator SSH key use.

SSH from trusted operator workstation succeeded. Selectel Security Group /32 restriction is **not verified**. STG-LIM-003 remains open.

## Decision

```text
SELECTEL_SSH_SG_VERIFICATION_PARTIAL_SSH_TRUSTED_PASS_SG_PENDING
```

## STG-LIM-003 Status

```text
OPEN — SSH trusted access verified; Selectel SG /32 restriction not verified
```

## Production-ready Status

```text
not claimed
```

## Verification Matrix

| Test ID | Check | Expected | Actual | Result |
| ------- | ----- | -------- | ------ | ------ |
| VER-SSH-SG-001 | API health GET | 200 | 200 | PASS |
| VER-SSH-SG-002 | SSH from trusted operator workstation | success | success | PASS |
| VER-SSH-SG-003 | SSH with default agent (no operator key) | success or N/A | Permission denied (publickey) | NOTE |
| VER-SSH-SG-004 | Selectel SG SSH /32 rule applied | yes | not verified | FAIL |
| VER-SSH-SG-005 | SSH 22 not open to 0.0.0.0/0 in SG | yes | not verified | PENDING |
| VER-SSH-SG-006 | SSH from non-trusted IP rejected | rejected | not tested | PENDING |
| VER-SSH-SG-007 | UFW PostgreSQL 5432 deny | deny | deny | PASS |
| VER-SSH-SG-008 | UFW Redis 6379 deny | deny | deny | PASS |
| VER-SSH-SG-009 | Runtime containers healthy | healthy | 10 healthy | PASS |
| VER-SSH-SG-010 | STG-LIM-003 closed | yes | no | FAIL |

## Read-only GET Evidence

| Target | Result |
| ------ | ------ |
| http://161.104.53.221/health | 200 |

## SSH Evidence

Trusted operator SSH session:

```text
success — echo command returned OK
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

Runtime:

```text
10 containers healthy (api gateway, low-code, identity, document, rfx, company, transport-order, shipment, billing-register, postgres)
```

## Why STG-LIM-003 Remains Open

STG-LIM-003 requires **Selectel Security Group** restriction of SSH to trusted operator IP /32.

Current evidence:

* SSH works from trusted operator IP with configured private key
* Selectel SG rules were **not** verified via panel or non-trusted IP rejection test
* UFW alone does not close STG-LIM-003

## Safety Confirmation

Remote SSH executed:

```text
yes — read-only verification commands only
```

Remote writes executed:

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
operator applies Selectel SG SSH /32 rule in panel, then re-run verification with non-trusted IP test
```

## Next Pack

```text
Selectel SSH SG Panel Confirmation Pack v0.1
```
