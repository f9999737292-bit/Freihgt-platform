# Low-code Pilot Week-3 Selectel SSH SG Post-Panel Re-Verification Evidence v0.1

## Summary

Post-panel re-verification executed after operator reported Selectel Security Group change completed.

Trusted-path checks passed. External non-trusted TCP 22 scan **failed closure criteria** — port 22 remains publicly reachable from five international nodes. STG-LIM-003 remains **open**.

## Operator Input

```text
operator reported SG panel change completed
```

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
| RP-SSH-SG-001 | Trusted SSH path | pass | SSH_TRUSTED_PATH_OK | PASS |
| RP-SSH-SG-002 | API health GET | 200 | 200 | PASS |
| RP-SSH-SG-003 | Runtime containers healthy | 10 | 10 | PASS |
| RP-SSH-SG-004 | UFW 5432/6379 denied | deny | deny | PASS |
| RP-SSH-SG-005 | Non-trusted TCP 22 rejection | filtered/timeout | connect success × 5 | **FAIL** |
| RP-SSH-SG-006 | Selectel SG /32 effective | yes | no | **FAIL** |
| RP-SSH-SG-007 | STG-LIM-003 closed | yes | no | FAIL |

## Trusted Path

SSH from trusted operator workstation:

```text
pass
```

API health:

```text
200
```

Runtime:

```text
10 containers healthy
```

UFW:

```text
5432 DENY, 6379 DENY, 22 ALLOW Anywhere (VM level)
```

## Non-Trusted Rejection Re-Test

Method:

```text
check-host.net TCP connect scan — 5 international non-trusted nodes (new request)
```

Non-trusted source available:

```text
yes
```

Regions tested (countries only):

| Region | Result |
| ------ | ------ |
| Switzerland, Zurich | TCP 22 connect success (~62 ms) |
| Spain, Madrid | TCP 22 connect success (~68 ms) |
| Spain, Barcelona | TCP 22 connect success (~68 ms) |
| Serbia, Belgrade | TCP 22 connect success (~68 ms) |
| Vietnam, Ho Chi Minh City | TCP 22 connect success (~239 ms) |

Non-trusted rejection result:

```text
fail — 5/5 nodes still connect to TCP 22
```

Comparison to prior scan:

```text
prior scan (5 nodes): 5/5 connect — current scan (5 different nodes): 5/5 connect — no improvement
```

## Selectel SG Assessment

Operator reported panel change:

```text
yes
```

External evidence SG /32 effective:

```text
no
```

## Decision

```text
SELECTEL_SSH_SG_POST_PANEL_REVERIFICATION_FAILED_PORT_22_STILL_PUBLIC
```

Blocker:

```text
BLOCKED_WAITING_FOR_SELECTEL_SG_PANEL_CHANGE
```

## STG-LIM-003 Status

```text
OPEN — operator reported SG change; external scan shows port 22 still publicly reachable
```

## Production-ready Status

```text
not claimed
```

## Operator Follow-up Required

Verify in Selectel panel:

1. Security Group bound to server `161.104.53.221` is the correct group
2. Inbound TCP 22 allows **only** trusted operator IP /32
3. No inbound TCP 22 rule for `0.0.0.0/0` or `::/0` remains
4. Changes saved and applied to the server port/security group attachment
5. Wait 1–2 minutes and re-run external scan

Trusted operator IP must be entered in Selectel panel only — **not stored in this repository**.

## Safety Confirmation

Remote SSH:

```text
read-only verification only
```

Writes:

```text
no
```

Secrets captured:

```text
no
```

Operator IP in docs:

```text
no
```

## Next Pack

```text
Selectel SSH SG Post-Panel Re-Verification Pack v0.1 (retry after SG fix confirmed in panel)
```
