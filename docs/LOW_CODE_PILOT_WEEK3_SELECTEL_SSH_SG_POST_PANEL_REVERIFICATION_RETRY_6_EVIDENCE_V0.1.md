# Low-code Pilot Week-3 Selectel SSH SG Post-Panel Re-Verification Retry 6 Evidence v0.1

## Summary

Post-panel re-verification retry #6 executed on 2026-07-12 after operator reported SG fix.

Trusted-path checks passed. External non-trusted TCP 22 scan **failed closure criteria** — port 22 remains publicly reachable from four of five international nodes. STG-LIM-003 remains **open**.

## Operator Input

```text
operator reported SG fix completed (retry #6)
user requested re-verification: оператор исправил — проверь снова
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
| RP6-SSH-SG-001 | Trusted SSH path | pass | SSH_TRUSTED_PATH_OK | PASS |
| RP6-SSH-SG-002 | API health GET | 200 | 200 | PASS |
| RP6-SSH-SG-003 | Runtime containers healthy | 10 | 10 | PASS |
| RP6-SSH-SG-004 | UFW 5432/6379 denied | deny | deny | PASS |
| RP6-SSH-SG-005 | Non-trusted TCP 22 rejection | filtered/timeout | connect success × 4; timeout × 1 | **FAIL** |
| RP6-SSH-SG-006 | Selectel SG /32 effective | yes | no | **FAIL** |
| RP6-SSH-SG-007 | STG-LIM-003 closed | yes | no | FAIL |

## Trusted Path

SSH from trusted operator workstation:

```text
pass — SSH_TRUSTED_PATH_OK
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

## Non-Trusted Rejection Re-Test (Retry #6)

Method:

```text
check-host.net TCP connect scan — 5 international non-trusted nodes
```

Non-trusted source available:

```text
yes
```

Regions tested (countries only):

| Region | Result |
| ------ | ------ |
| Cyprus, Larnaca | TCP 22 connect success (~74 ms) |
| India, Bengaluru | TCP 22 connect success (~174 ms) |
| Romania, Bucharest | connect timeout |
| Serbia, Belgrade | TCP 22 connect success (~66 ms) |
| Turkey, Istanbul | TCP 22 connect success (~80 ms) |

Non-trusted rejection result:

```text
fail — 4/5 nodes connect to TCP 22; 1/5 timeout
```

Comparison to prior retry #5:

```text
retry #5: 5/5 connect, trusted SSH fail — retry #6: 4/5 connect, trusted SSH pass (partial improvement, closure criteria not met)
```

## Selectel SG Assessment

Operator reported panel change:

```text
yes (retry #6)
```

External evidence SG /32 effective:

```text
no — 4/5 non-trusted nodes still connect
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
OPEN — operator reported SG fix; external scan 4/5 connect; trusted SSH pass
```

## Production-ready Status

```text
not claimed
```

## Operator Follow-up Required

Verify in Selectel panel:

1. Security Group bound to server `161.104.53.221` is the correct group and **applied/saved**
2. Inbound TCP 22 allows **only** trusted operator IP /32
3. No inbound TCP 22 rule for `0.0.0.0/0` or `::/0` remains
4. Wait 1–2 minutes and re-run external scan

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
Selectel SSH SG Post-Panel Re-Verification Pack v0.1 (retry #7 after SG fix confirmed effective)
```
