# STG-LIM-002 Certbot Retry After Egress PASS Evidence v0.1

## Summary

Certbot retry was executed after Selectel outbound egress was fixed on 2026-07-17.

HTTPS certificate issued for `staging.xn--80abvubqje.xn--p1ai`. HTTPS `/health` returns 200. HTTP redirects to HTTPS. Certbot renewal dry-run succeeded.

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

## Previous Failure

```text
STG_LIM_002_OUTBOUND_DNS_FIX_FAIL
```

Root cause was outbound DNS / ACME egress blocked at Selectel SG level.

## Preconditions

```text
STG-LIM-001: CLOSED — DNS verified
STG-LIM-003: CLOSED — SSH SG /32 verified
Selectel outbound egress: fixed by operator
```

## Server Pre-check

```text
ACME DNS: PASS — getent hosts acme-v02.api.letsencrypt.org resolves
ACME HTTPS directory: PASS — HTTP/2 200
HTTP by domain before Certbot: PASS — 200
Nginx config test before Certbot: PASS
Nginx backup path: /root/nginx-backup-before-certbot-retry-20260717_185713
```

Do not store backup contents in repo.

## Certbot Retry

```text
Certbot executed: yes
Certbot result: PASS
Certificate name: staging.xn--80abvubqje.xn--p1ai
Key type: ECDSA
Expiry date: 2026-10-15 (89 days at issuance)
Nginx config test after Certbot: PASS
Nginx reload after Certbot: PASS
Certificate private key captured in docs: no
```

## Verification Matrix

| Check                            | Result | Notes                                              |
| -------------------------------- | ------ | -------------------------------------------------- |
| DNS A record                     | PASS   | 161.104.53.221                                     |
| ACME DNS from server             | PASS   | acme-v02.api.letsencrypt.org                       |
| ACME HTTPS directory from server | PASS   | HTTP/2 200                                         |
| Certbot retry                    | PASS   | certificate issued and deployed to nginx           |
| HTTPS /health punycode           | PASS   | 200                                                |
| HTTP -> HTTPS redirect           | PASS   | 301 → https://staging.xn--80abvubqje.xn--p1ai/health |
| HTTPS /health display Cyrillic   | PASS   | 200                                                |
| Certbot renewal dry-run          | PASS   | all simulated renewals succeeded                     |

## Machine-captured Output (sanitized)

Certbot issuance:

```text
Successfully received certificate.
Certificate is saved at: /etc/letsencrypt/live/staging.xn--80abvubqje.xn--p1ai/fullchain.pem
Successfully deployed certificate for staging.xn--80abvubqje.xn--p1ai
Congratulations! You have successfully enabled HTTPS on https://staging.xn--80abvubqje.xn--p1ai
```

External verification:

```text
HTTPS https://staging.xn--80abvubqje.xn--p1ai/health: 200
HTTP redirect http://staging.xn--80abvubqje.xn--p1ai/health: 301 https://staging.xn--80abvubqje.xn--p1ai/health
HTTPS https://staging.бинтранс.рф/health: 200
```

Renewal dry-run:

```text
Congratulations, all simulated renewals succeeded
certbot.timer: active (next run scheduled)
```

## Decision

```text
STG_LIM_002_CERTBOT_RETRY_AFTER_EGRESS_PASS
```

## STG-LIM-002

```text
READY_FOR_CLOSURE_REVIEW
```

## Remaining Open Limitations

```text
STG-LIM-004: OPEN — web-admin deploy pending
```

## Operator Follow-up

```text
1. Prepare STG-LIM-002 closure review pack
2. Web-admin deploy requires separate explicit approval
3. Rollback available from /root/nginx-backup-before-certbot-retry-20260717_185713 if needed
```

## Safety

```text
Backend/frontend changed: no
Docker compose repo changed: no
UFW changed: no
CORS/.env changed: no
Web-admin deployed: no
POST/PUT/PATCH/DELETE executed: no
Secrets captured: no
Certificate private key captured: no
Production-ready claimed: no
```
