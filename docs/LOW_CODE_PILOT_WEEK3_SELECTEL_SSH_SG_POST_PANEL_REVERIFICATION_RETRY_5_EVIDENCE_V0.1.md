# Low-code Pilot Week-3 Selectel SSH SG Post-Panel Re-Verification Retry 5 Evidence v0.1

## Summary

Post-panel re-verification retry #5 executed on 2026-07-12 after operator reported SG fix and provided panel screenshot showing /32-only SSH rule.

Trusted SSH path **failed** (banner exchange timeout). API health passed. External non-trusted TCP 22 scan **failed closure criteria** — port 22 remains publicly reachable from five international nodes. STG-LIM-003 remains **open**.

## Operator Input

```text
operator reported SG fix completed (retry #5)
operator provided Security Group panel screenshot — TCP 22 /32 only, 80/443 open
user requested re-verification: проверь снова
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
| RP5-SSH-SG-001 | Trusted SSH path | pass | banner exchange timeout | **FAIL** |
| RP5-SSH-SG-002 | API health GET | 200 | 200 | PASS |
| RP5-SSH-SG-003 | Runtime containers healthy | 10 | not verified — SSH unavailable | SKIP |
| RP5-SSH-SG-004 | UFW 5432/6379 denied | deny | not verified — SSH unavailable | SKIP |
| RP5-SSH-SG-005 | Non-trusted TCP 22 rejection | filtered/timeout | connect success × 5 | **FAIL** |
| RP5-SSH-SG-006 | Selectel SG /32 effective | yes | no | **FAIL** |
| RP5-SSH-SG-007 | STG-LIM-003 closed | yes | no | FAIL |

## Trusted Path

SSH from trusted operator workstation:

```text
fail — TCP connection established; banner exchange timeout
```

API health:

```text
200
```

Runtime:

```text
not verified — SSH unavailable on retry #5
```

## Non-Trusted Rejection Re-Test (Retry #5)

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
| Israel, Netanya | TCP 22 connect success (~106 ms) |
| Moldova, Chisinau | TCP 22 connect success (~59 ms) |
| Portugal, Viana | TCP 22 connect success (~75 ms) |
| USA, Dallas | TCP 22 connect success (~152 ms) |
| USA, Atlanta | TCP 22 connect success (~137 ms) |

Non-trusted rejection result:

```text
fail — 5/5 nodes connect to TCP 22
```

Prior retry #4 same-day scan:

```text
5/5 connect — trusted SSH also failed
```

## Selectel SG Assessment

Operator reported panel change:

```text
yes (retry #5) — screenshot shows /32-only SSH rule
```

External evidence SG /32 effective:

```text
no — 5/5 non-trusted nodes still connect
```

Trusted SSH regression:

```text
yes — banner exchange timeout despite panel screenshot appearing correct
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
OPEN — operator reported SG fix with panel screenshot; external scan 5/5 connect; trusted SSH failed
```

## Production-ready Status

```text
not claimed
```

## Operator Follow-up Required

Verify in Selectel panel:

1. Security Group bound to server `161.104.53.221` is the correct group and **applied/saved**
2. Inbound TCP 22 allows **only** trusted operator IP /32 (current workstation IP — enter in panel only)
3. No inbound TCP 22 rule for `0.0.0.0/0` or `::/0` remains
4. If trusted SSH banner timeout persists while external scan succeeds — check sshd via Selectel console
5. Wait 1–2 minutes after save and re-run external scan

Trusted operator IP must be entered in Selectel panel only — **not stored in this repository**.

## Safety Confirmation

Remote SSH:

```text
read-only verification attempt only
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
Selectel SSH SG Post-Panel Re-Verification Pack v0.1 (retry #6 after SG fix confirmed effective)
```
