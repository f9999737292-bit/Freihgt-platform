# Nginx Vhost Investigation Evidence v0.1

## Summary

Read-only Nginx vhost investigation was completed after production deployment execution retry failure.

No server changes were made.

## Previous Failure

```text
PRODUCTION_DEPLOYMENT_EXECUTION_RETRY_FAIL
```

Failure reason:

```text
PRODUCTION_DEPLOYMENT_EXECUTION_RETRY_SERVER_HTTPS_VERIFICATION_FAIL
```

## Read-only Scope

```text
Nginx config changed: no
Nginx reload executed: no
Certbot executed: no
DNS changed: no
Production deploy executed: no
Secrets captured: no
Certificate private key captured: no
```

## Baseline

| Check | Result |
| --- | --- |
| Production DNS | PASS — 161.104.53.221 (1.1.1.1, 8.8.8.8) |
| Production HTTP root | FAIL — 404 application/json |
| Production HTTPS root | PARTIAL — 200 text/html with `-k` only; no dedicated production vhost |
| Staging HTTPS root | PASS — 200 text/html |
| Staging health | PASS — 200 |
| Staging API read-only | PASS — 200 |

## Server Nginx Inventory

| Item | Result |
| --- | --- |
| nginx -t | PASS |
| sites-enabled inspected | yes — `freight-staging`, `staging-bintrans.conf` |
| sites-available inspected | yes — `default`, `freight-staging`, `staging-bintrans.conf`; no `bintrans-production.conf` |
| conf.d inspected | yes — empty |
| default_server explicit | no — implicit default is first port-80 block |
| production server_name currently enabled | **no** |
| staging server_name currently enabled | **yes** — `staging.xn--80abvubqje.xn--p1ai` |
| production cert exists | **yes** — `/etc/letsencrypt/live/xn--80abvubqje.xn--p1ai/` |
| staging cert exists | **yes** — `/etc/letsencrypt/live/staging.xn--80abvubqje.xn--p1ai/` |
| web root index exists | **yes** — `/var/www/bintrans-web-admin/index.html` |

## Enabled Site Map

| File | listen | server_name | Behavior |
| --- | --- | --- | --- |
| `freight-staging` | 80, [::]:80 | `161.104.53.221` | `proxy_pass http://127.0.0.1:8080` — API gateway |
| `staging-bintrans.conf` | 80 | `staging.xn--80abvubqje.xn--p1ai` | HTTP → HTTPS redirect |
| `staging-bintrans.conf` | 443 ssl | `staging.xn--80abvubqje.xn--p1ai` | SPA + `/health` + `/api/` proxy |

Production apex `xn--80abvubqje.xn--p1ai` has **no enabled server block** after rollback.

## Host Matching Findings

| Host | Protocol | Result | Likely matched vhost |
| --- | --- | --- | --- |
| xn--80abvubqje.xn--p1ai | HTTP | 404 application/json — `ROUTE_NOT_FOUND` JSON body | implicit default → `freight-staging` → API gateway |
| xn--80abvubqje.xn--p1ai | HTTPS | 200 text/html (with `-k` / SNI fallback) | no prod vhost; staging 443 block serves same web root as fallback |
| staging.xn--80abvubqje.xn--p1ai | HTTP | 301 → HTTPS | `staging-bintrans.conf` port 80 |
| staging.xn--80abvubqje.xn--p1ai | HTTPS | 200 text/html | `staging-bintrans.conf` port 443 ssl |

Machine-captured local host tests:

```text
prod_http_root=404 application/json — {"error":{"code":"ROUTE_NOT_FOUND",...}}
prod_http_health=200 application/json
prod_https_root=200 text/html — login redirect HTML
stg_http_health=301 text/html
```

## Likely Root Cause

```text
1. Production apex has no enabled server_name after rollback, so HTTP requests with Host xn--80abvubqje.xn--p1ai hit the implicit default server (freight-staging) and are proxied to API gateway, returning 404 application/json.
2. During retry v0.2, temporary production HTTP site was created but prod_http_root still returned 404 application/json — consistent with default-server / load-order conflict before dedicated vhost took effect.
3. Certbot succeeded and production cert exists, but after final HTTPS site creation server-side curl without -k failed (SSL error 60) because no production 443 vhost with matching cert was active; rollback then removed production site entirely.
4. External HTTPS with -k currently returns 200 text/html only because staging 443 block serves the shared web root as SSL fallback — this is not a valid production deployment state.
```

## Recommendation for Retry v0.3

```text
1. Re-enable dedicated production site /etc/nginx/sites-available/bintrans-production.conf with server_name xn--80abvubqje.xn--p1ai only.
2. Use existing production cert /etc/letsencrypt/live/xn--80abvubqje.xn--p1ai/ — certbot certonly not required unless renewal needed.
3. Port 80: ACME path + SPA + /health + /api/ proxy, or HTTP→HTTPS redirect after verify.
4. Port 443 ssl: SPA + /health + /api/ proxy with production fullchain/privkey paths.
5. Ensure freight-staging default does not catch production Host — production vhost must match server_name before implicit default.
6. Server-side verify: use curl --resolve with Host/SNI and without -k only after production ssl_certificate paths are active.
7. Keep staging-bintrans.conf unchanged; verify staging still PASS after production enable.
8. Use existing Nginx backup /root/prod-deploy-retry-backup-20260720_154539 or create fresh backup before v0.3.
```

## Decision

```text
NGINX_VHOST_INVESTIGATION_COMPLETE
```

## Safety

```text
Nginx config changed during investigation: no
Nginx reload executed during investigation: no
Certbot executed during investigation: no
DNS changed during investigation: no
Production deploy executed: no
POST/PUT/PATCH/DELETE executed: no
Secrets captured: no
Certificate private key captured: no
```
