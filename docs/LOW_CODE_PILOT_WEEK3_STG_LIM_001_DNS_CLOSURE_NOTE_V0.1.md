# STG-LIM-001 DNS Closure Note v0.1

## Summary

STG-LIM-001 is closed after successful Cyrillic .рф staging DNS verification.

Production-ready is not claimed.

## Domain

Display domain:

```text
staging.бинтранс.рф
```

Technical / punycode domain:

```text
staging.xn--80abvubqje.xn--p1ai
```

Target IP:

```text
161.104.53.221
```

## Evidence

Related evidence:

```text
docs/LOW_CODE_PILOT_WEEK3_CYRILLIC_RF_DNS_VERIFICATION_EVIDENCE_V0.1.md
```

Latest verification decision:

```text
CYRILLIC_RF_DNS_VERIFICATION_PASS
```

## Verification Matrix

| Check                       | Result                |
| --------------------------- | --------------------- |
| DNS delegation              | PASS                  |
| Authoritative DNS-master NS | PASS                  |
| Public resolver 1.1.1.1     | PASS — 161.104.53.221 |
| Public resolver 8.8.8.8     | PASS — 161.104.53.221 |
| Default resolver            | PASS — 161.104.53.221 |
| HTTP /health by domain      | PASS — 200            |

## Closure Decision

```text
STG-LIM-001_CLOSED_DNS_VERIFIED
```

## Remaining Open Limitations

```text
STG-LIM-002: OPEN — HTTPS / Certbot pending
STG-LIM-003: OPEN — SSH SG /32 pending
STG-LIM-004: OPEN — web-admin deploy pending
```

## Safety

```text
Certbot executed: no
Nginx changed: no
Web-admin deployed: no
Writes executed: no
Secrets captured: no
Production-ready claimed: no
```
