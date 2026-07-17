# STG-LIM-002 HTTPS / Certbot Evidence v0.1

## Summary

HTTPS / Certbot execution was attempted for the Cyrillic .рф staging domain on 2026-07-17.

Nginx domain server block was created and HTTP by domain remained healthy. Certbot failed because the server could not resolve Let's Encrypt ACME endpoints via configured DNS resolvers.

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

## Preconditions

```text
STG-LIM-001: CLOSED — DNS verified
STG-LIM-003: CLOSED — SSH SG /32 verified
```

## Server Changes

```text
Nginx domain server block created/updated: yes — /etc/nginx/sites-available/staging-bintrans.conf
Certbot executed: attempted — FAIL
Certificate private key captured in docs: no
Nginx config test before Certbot: PASS
Nginx config test after failed Certbot: PASS
Nginx reload: PASS
```

Backup:

```text
Nginx backup created on server: yes
Backup path: /root/nginx-backup-before-certbot-20260717_165628
```

Do not store backup contents in repo.

## Failure Detail

Certbot error summary (no secrets):

```text
ConnectionError resolving acme-v02.api.letsencrypt.org — server DNS resolver timeout
Server resolvers: 188.93.16.19, 188.93.17.19 (systemd-resolved)
nslookup acme-v02.api.letsencrypt.org: no servers could be reached
```

HTTP by domain remained available after Nginx site creation.

## Verification Matrix

| Check                            | Result | Notes                                      |
| -------------------------------- | ------ | ------------------------------------------ |
| DNS A record                     | PASS   | 161.104.53.221                             |
| HTTP before Certbot              | PASS   | 200                                        |
| Nginx config test before Certbot | PASS   | nginx -t successful                        |
| Nginx domain site created        | PASS   | staging-bintrans.conf enabled              |
| HTTP after Nginx site            | PASS   | 200                                        |
| Certbot execution                | FAIL   | ACME DNS resolution failure on server      |
| Nginx config test after Certbot  | PASS   | nginx -t successful (no cert installed)    |
| HTTPS /health punycode           | FAIL   | certificate not issued                     |
| HTTP -> HTTPS redirect           | FAIL   | not configured — Certbot did not complete  |
| HTTPS /health display Cyrillic   | FAIL   | certificate not issued                     |
| Certbot renewal dry-run          | FAIL   | not run — certificate not issued           |

## Decision

```text
STG_LIM_002_HTTPS_CERTBOT_FAIL
```

## STG-LIM-002

```text
OPEN
```

## Remaining Open Limitations

```text
STG-LIM-004: OPEN — web-admin deploy pending
```

## Operator Follow-up

```text
1. Fix server outbound DNS on 161.104.53.221 (resolver reachability for acme-v02.api.letsencrypt.org)
2. Re-run Certbot after DNS fix: certbot --nginx -d staging.xn--80abvubqje.xn--p1ai --redirect
3. Rollback available from /root/nginx-backup-before-certbot-20260717_165628 if needed
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
Production-ready claimed: no
```
