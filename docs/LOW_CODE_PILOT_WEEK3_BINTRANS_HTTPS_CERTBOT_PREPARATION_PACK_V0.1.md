# Low-code Pilot Week-3 Bintrans HTTPS / Certbot Preparation Pack v0.1

## Summary

Docs-only preparation pack for HTTPS on `staging.бинтранс.рф` via Nginx + Certbot on Selectel staging server.

Technical commands use punycode: `staging.xn--80abvubqje.xn--p1ai`.

No DNS changes, SSH writes, Certbot execution, Nginx server changes, backend changes, frontend changes, API contract changes, or migrations were executed in this pack.

External non-trusted TCP 22 scan verification for STG-LIM-003 was **deferred per operator request** — STG-LIM-003 remains **open**.

## Decision

```text
CYRILLIC_RF_DOMAIN_SELECTED_DNS_PENDING
```

## Target

Provider:

```text
Selectel
```

Staging IP:

```text
161.104.53.221
```

Primary domain (display):

```text
staging.бинтранс.рф
```

Primary domain (technical / punycode):

```text
staging.xn--80abvubqje.xn--p1ai
```

Deprecated previous domain:

```text
staging.bintrans.ru
```

Current HTTP endpoint:

```text
http://161.104.53.221/health — 200
```

Target HTTPS endpoint (display):

```text
https://staging.бинтранс.рф/health
```

Target HTTPS endpoint (technical):

```text
https://staging.xn--80abvubqje.xn--p1ai/health
```

## Prerequisites Checklist

| # | Prerequisite | Status | Notes |
| - | ------------ | ------ | ----- |
| 1 | DNS A-record `staging.бинтранс.рф` → `161.104.53.221` | **pending** | operator action at registrar |
| 2 | HTTP `/health` via domain returns 200 | **pending** | blocked by DNS |
| 3 | SSH trusted path available | **pass** | retry #6/#7 trusted SSH OK |
| 4 | Nginx installed on server | **assumed** | verify on execution |
| 5 | Selectel SG allows TCP 80 and 443 | **pass** | inbound 80/443 open |
| 6 | STG-LIM-003 external port 22 scan | **deferred** | operator requested skip; remains open |
| 7 | production-ready claimed | **no** | |

## DNS Verification (local — not executed on server)

```powershell
nslookup staging.бинтранс.рф
nslookup staging.xn--80abvubqje.xn--p1ai
```

Result at pack update:

```text
no resolution — DNS_PENDING_OPERATOR_ACTION
```

## Planned Nginx Configuration (docs-only — do not apply without approval)

Server block target for `staging.xn--80abvubqje.xn--p1ai`:

```nginx
server {
    listen 80;
    server_name staging.xn--80abvubqje.xn--p1ai;

    location /health {
        proxy_pass http://127.0.0.1:8080/health;
        proxy_set_header Host $host;
    }

    location /api/ {
        proxy_pass http://127.0.0.1:8080/api/;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

Notes:

* Adjust upstream port if api-gateway listens on a different port on the VM.
* Certbot nginx plugin will add HTTPS server block after certificate issuance.
* Do not apply until DNS resolves and operator approves execution.

## Planned Certbot Steps (docs-only — do not execute)

After DNS resolves and HTTP domain check passes:

```bash
# verify nginx config
nginx -t

# issue certificate (execute only after operator approval)
certbot --nginx -d staging.xn--80abvubqje.xn--p1ai

# verify renewal timer
systemctl status certbot.timer
```

Post-Certbot verification:

```bash
curl -s -o /dev/null -w "%{http_code}" https://staging.xn--80abvubqje.xn--p1ai/health
```

Expected:

```text
200
```

## Future CORS Origin (technical)

```text
https://staging.xn--80abvubqje.xn--p1ai
```

## Staging Limitations Impact

| ID | Status after this pack |
| -- | ---------------------- |
| STG-LIM-001 | OPEN — DNS still pending |
| STG-LIM-002 | OPEN — HTTPS prep created; execution pending DNS |
| STG-LIM-003 | OPEN — external scan deferred; not closed |

## Production-ready

```text
not claimed
```

## Controlled pilot

```text
active
```

## Safety

Secrets in docs:

```text
no
```

Operator IP in docs:

```text
no
```

Remote writes executed:

```text
no
```

## Next Pack

```text
Bintrans HTTPS / Certbot Execution Pack v0.1 (after DNS resolves + operator approval)
```

## Related Docs

```text
docs/LOW_CODE_PILOT_WEEK3_CYRILLIC_RF_DOMAIN_MIGRATION_DECISION_V0.1.md
docs/LOW_CODE_PILOT_WEEK3_BINTRANS_DNS_CHECKLIST_V0.1.md
docs/LOW_CODE_PILOT_WEEK3_BINTRANS_HTTPS_CERTBOT_CHECKLIST_V0.1.md
docs/LOW_CODE_PILOT_WEEK3_STAGING_DEPLOY_RUNBOOK_V0.1.md
```
