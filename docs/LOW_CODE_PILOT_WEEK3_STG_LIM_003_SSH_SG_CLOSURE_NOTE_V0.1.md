# STG-LIM-003 SSH Security Group Closure Note v0.1

## Summary

STG-LIM-003 is closed after successful Selectel SSH Security Group /32 verification retry #7.

Production-ready is not claimed.

## Context

```text
Server IP: 161.104.53.221
Domain: staging.бинтранс.рф
Punycode: staging.xn--80abvubqje.xn--p1ai
```

## Related Evidence

```text
docs/LOW_CODE_PILOT_WEEK3_SELECTEL_SSH_SG_RETRY_007_EVIDENCE_V0.1.md
```

Retry #7 decision:

```text
SELECTEL_SSH_SG_RETRY_007_PASS
```

## Verification Summary

| Check                              | Result                                                 |
| ---------------------------------- | ------------------------------------------------------ |
| Trusted TCP 22 from operator IP    | PASS — TcpTestSucceeded: True (closure re-check)       |
| Trusted SSH read-only              | PASS — retry #7 evidence                               |
| Non-trusted external TCP 22 checks | PASS — denied/timeout from required external locations |
| Confirmatory external scan         | PASS — denied/timeout                                  |
| HTTP /health by domain             | PASS — 200 (closure re-check)                          |

Trusted operator IP:

```text
masked /32
```

Full trusted IP is not stored in docs.

## Closure Decision

```text
STG-LIM-003_CLOSED_SSH_SG_VERIFIED
```

## Remaining Open Limitations

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
