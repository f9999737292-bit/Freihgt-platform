# Low-code Pilot Week-3 Selectel SSH SG Retry #7 Evidence v0.1

## Summary

Selectel SSH Security Group /32 verification retry #7 was executed on 2026-07-17.

Trusted-path checks passed. External non-trusted TCP 22 scan from five international nodes returned timeout/denied on all nodes (0/5 connect). STG-LIM-003 advanced to **READY_FOR_CLOSURE_REVIEW**.

Production-ready is not claimed.

## Context

```text
Server IP: 161.104.53.221
Domain: staging.бинтранс.рф
Punycode: staging.xn--80abvubqje.xn--p1ai
STG-LIM-001: CLOSED — DNS verified
STG-LIM-002: OPEN — HTTPS / Certbot pending
STG-LIM-003: READY_FOR_CLOSURE_REVIEW
STG-LIM-004: OPEN — web-admin deploy pending
```

## Trusted Operator IP

```text
trusted_operator_ip: 193.xxx.xxx.xxx/32
```

Full IP not stored in repository.

## Verification Matrix

| Check                                  | Result | Notes                                      |
| -------------------------------------- | ------ | ------------------------------------------ |
| Operator confirmed Selectel SG changed | PASS   | inferred — external scan reversed vs retry #6 |
| Trusted TCP 22 from operator IP        | PASS   | TcpTestSucceeded: True                     |
| Trusted SSH read-only                  | PASS   | root@gpt-docker; UFW/docker read-only      |
| Non-trusted external TCP 22 check #1   | PASS   | Bulgaria (bg1) — timeout/denied            |
| Non-trusted external TCP 22 check #2   | PASS   | Germany (de1) — connect timeout            |
| Non-trusted external TCP 22 check #3   | PASS   | Netherlands (nl2) — timeout/denied         |
| Non-trusted external TCP 22 check #4   | PASS   | Serbia (rs1) — timeout/denied            |
| Non-trusted external TCP 22 check #5   | PASS   | Turkey (tr2) — timeout/denied              |
| HTTP /health by domain                 | PASS   | 200                                        |

## Trusted Path Detail

Trusted TCP 22:

```text
TcpTestSucceeded: True — 161.104.53.221:22
```

Trusted SSH read-only:

```text
hostname: gpt-docker
whoami: root
UFW: 5432/6379/8080/8088/3000/5173 DENY; 22/80/443 ALLOW (VM level)
docker: 10 containers healthy
```

## Non-Trusted External TCP 22 (Retry #7)

Method:

```text
check-host.net TCP connect scan — 5 international non-trusted nodes
request_id: 44a92817k8a7
```

| # | Location / provider        | Result              |
| - | -------------------------- | ------------------- |
| 1 | Bulgaria (bg1.node)        | DENIED — timeout    |
| 2 | Germany (de1.node)         | DENIED — timeout    |
| 3 | Netherlands (nl2.node)     | DENIED — timeout    |
| 4 | Serbia (rs1.node)          | DENIED — timeout    |
| 5 | Turkey (tr2.node)          | DENIED — timeout    |

Non-trusted rejection result:

```text
pass — 0/5 nodes connect; 5/5 timeout/denied
```

Comparison to retry #6:

```text
retry #6: 4/5 connect — retry #7: 0/5 connect (SG /32 effective per external evidence)
```

Confirmatory second scan (same day):

```text
request_id: 44a92d09k5f7 — Brazil, India ×3, Iran — 5/5 timeout/denied
```

## Decision

```text
SELECTEL_SSH_SG_RETRY_007_PASS
```

## STG-LIM-003

```text
READY_FOR_CLOSURE_REVIEW
```

## Remaining Limitations

```text
STG-LIM-002: OPEN — HTTPS / Certbot pending
STG-LIM-004: OPEN — web-admin deploy pending
```

## Safety

```text
UFW changed: no
Nginx changed: no
Certbot executed: no
Web-admin deployed: no
Backend/frontend changed: no
Writes executed: no
Secrets captured: no
Production-ready claimed: no
```

## Next Pack

```text
STG-LIM-003 Closure Review Pack v0.1
```
