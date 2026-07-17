# STG-LIM-002 HTTPS Closure Note v0.1

## Summary

STG-LIM-002 is closed after successful HTTPS / Certbot verification for the Cyrillic .рф staging domain.

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

## Related Evidence

```text
docs/LOW_CODE_PILOT_WEEK3_STG_LIM_002_CERTBOT_RETRY_AFTER_EGRESS_EVIDENCE_V0.1.md
```

Latest decision:

```text
STG_LIM_002_CERTBOT_RETRY_AFTER_EGRESS_PASS
```

## Verification Summary

| Check                       | Result     |
| --------------------------- | ---------- |
| Server ACME DNS             | PASS       |
| Server ACME HTTPS directory | PASS       |
| Certbot retry               | PASS       |
| HTTPS /health punycode      | PASS — 200 |
| HTTPS /health Cyrillic      | PASS — 200 |
| HTTP -> HTTPS redirect      | PASS — 301 |
| Certbot renewal dry-run     | PASS       |
| certbot.timer               | active     |
| Nginx config test           | PASS       |

Closure re-check (2026-07-17):

```text
HTTPS punycode: 200
HTTPS Cyrillic: 200
HTTP redirect: 301 to https://staging.xn--80abvubqje.xn--p1ai/health
Certificate expiry: 2026-10-15 (89 days at closure re-check)
```

## Closure Decision

```text
STG-LIM-002_CLOSED_HTTPS_CERTBOT_VERIFIED
```

## Remaining Open Limitations

```text
STG-LIM-004: OPEN — web-admin deploy pending
```

## Safety

```text
Backend/frontend changed: no
Docker compose repo changed: no
UFW changed: no
CORS/.env changed: no
Nginx changed during closure pack: no
Certbot executed during closure pack: no
Web-admin deployed: no
POST/PUT/PATCH/DELETE executed: no
Secrets captured: no
Certificate private key captured: no
Production-ready claimed: no
```
