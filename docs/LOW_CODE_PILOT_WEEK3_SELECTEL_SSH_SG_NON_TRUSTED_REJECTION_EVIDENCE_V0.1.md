# Low-code Pilot Week-3 Selectel SSH SG Non-Trusted Rejection Evidence v0.1

## Summary

Non-trusted SSH rejection test executed via external check-host.net nodes from five international regions.

All five non-trusted nodes successfully established TCP connections to port 22 on `161.104.53.221`. This indicates Selectel Security Group SSH /32 restriction is **not applied** or **not effective**. STG-LIM-003 remains **open**.

## Target

Provider:

```text
Selectel
```

Server IP:

```text
161.104.53.221
```

Controlled pilot:

```text
active
```

Production-ready claimed:

```text
no
```

## Verification Matrix

| Test ID | Check | Expected | Actual | Result |
| ------- | ----- | -------- | ------ | ------ |
| NT-SSH-SG-001 | Trusted SSH path baseline | pass | SSH_TRUSTED_OK | PASS |
| NT-SSH-SG-002 | API health GET | 200 | 200 | PASS |
| NT-SSH-SG-003 | Runtime containers healthy | 10 | 10 | PASS |
| NT-SSH-SG-004 | Non-trusted external source available | yes | yes — check-host.net | PASS |
| NT-SSH-SG-005 | Non-trusted TCP 22 rejection | filtered/timeout | connect success × 5 | **FAIL** |
| NT-SSH-SG-006 | Selectel SG /32 restriction effective | yes | no | **FAIL** |
| NT-SSH-SG-007 | TCP 22 not globally reachable | yes | no — publicly open | **FAIL** |
| NT-SSH-SG-008 | STG-LIM-003 closed | yes | no | FAIL |

## Trusted Path Baseline

SSH from trusted operator workstation:

```text
pass — SSH_TRUSTED_OK
```

API health:

```text
200
```

Runtime:

```text
10 containers healthy
```

Operator IP stored in docs:

```text
no
```

## Non-Trusted Rejection Test

Method:

```text
check-host.net TCP connect scan — 5 international non-trusted nodes
```

Scan request:

```text
check-host.net check-tcp port 22 — permanent link captured in operator session only, not stored in repo
```

Non-trusted source available:

```text
yes
```

Regions tested (sanitized — node countries only):

| Region | Result |
| ------ | ------ |
| Bulgaria, Sofia | TCP 22 connect success (~80 ms) |
| Indonesia, Jakarta | TCP 22 connect success (~198 ms) |
| India, Bengaluru | TCP 22 connect success (~178 ms) |
| Iran, Shiraz | TCP 22 connect success (~58 ms) |
| Ukraine, Khmelnytskyi | TCP 22 connect success (~59 ms) |

Non-trusted SSH rejection result:

```text
fail — port 22 accepts TCP connections from all tested non-trusted external nodes
```

Interpretation:

```text
Selectel Security Group SSH /32 restriction is not in effect. Port 22 remains publicly reachable at TCP level from non-trusted international sources.
```

Note:

```text
TCP connect success does not imply SSH authentication success — only that port 22 is reachable externally. This is sufficient to fail STG-LIM-003 closure criteria.
```

## Selectel Security Group Assessment

Selectel SG panel changed manually:

```text
no — external scan evidence indicates SG /32 not applied
```

TCP 22 allowed only from trusted operator IP /32:

```text
no
```

TCP 22 open to 0.0.0.0/0 removed:

```text
no — port 22 publicly reachable from non-trusted nodes
```

Independent panel evidence captured:

```text
no
```

## Decision

```text
SELECTEL_SSH_SG_NON_TRUSTED_REJECTION_FAILED_PORT_22_PUBLICLY_OPEN
```

Equivalent operator-facing blocker:

```text
BLOCKED_WAITING_FOR_SELECTEL_SG_PANEL_CHANGE
```

## STG-LIM-003 Status

```text
OPEN — external non-trusted scan confirms port 22 publicly reachable; Selectel SG /32 not applied
```

Closure candidate:

```text
no — STG_LIM_003_REMAINS_OPEN
```

## Production-ready Status

```text
not claimed
```

## Safety Confirmation

Remote SSH executed:

```text
yes — read-only baseline checks only
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

## Operator Action Required

1. Log in to Selectel control panel for server `161.104.53.221`
2. Security Group inbound: TCP 22 → trusted operator IP /32 only
3. Remove SSH rules allowing `0.0.0.0/0` or `::/0`
4. Re-run non-trusted rejection test after panel change

Trusted operator IP must be entered in Selectel panel only — **not stored in this repository**.

Reference runbook:

```text
docs/LOW_CODE_PILOT_WEEK3_SELECTEL_SSH_SECURITY_GROUP_RESTRICTION_RUNBOOK_V0.1.md
```

## Next Recommended Event

```text
operator applies Selectel SG SSH /32 restriction in control panel, then re-run non-trusted rejection test
```

## Next Pack

```text
Selectel SSH SG Post-Panel Re-Verification Pack v0.1
```
