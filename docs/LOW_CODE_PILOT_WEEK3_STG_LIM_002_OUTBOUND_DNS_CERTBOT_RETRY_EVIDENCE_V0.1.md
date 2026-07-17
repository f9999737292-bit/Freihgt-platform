# STG-LIM-002 Outbound DNS Fix + Certbot Retry Evidence v0.1

## Summary

Outbound DNS fix and Certbot retry were attempted for staging HTTPS on 2026-07-17.

systemd-resolved was reconfigured with public resolvers and `/etc/resolv.conf` was switched to the systemd-resolved stub. Server outbound DNS and ACME reachability remained FAIL — queries to `1.1.1.1`, `8.8.8.8`, and prior Selectel resolvers timed out. Certbot was not executed.

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
STG_LIM_002_HTTPS_CERTBOT_FAIL
```

Root cause:

```text
Server outbound DNS failed to resolve acme-v02.api.letsencrypt.org.
Resolvers 188.93.16.19 / 188.93.17.19 timed out.
```

## Preconditions

```text
STG-LIM-001: CLOSED — DNS verified
STG-LIM-003: CLOSED — SSH SG /32 verified
```

## Server DNS Fix

```text
Resolver config backup created: yes
Backup path: /root/dns-backup-before-certbot-retry-20260717_182208
systemd-resolved drop-in created/updated: yes — /etc/systemd/resolved.conf.d/99-bintrans-outbound-dns.conf
/etc/resolv.conf changed to systemd-resolved stub: yes — /run/systemd/resolve/stub-resolv.conf
Global DNS servers after fix: 1.1.1.1 8.8.8.8 9.9.9.9
ACME DNS after fix: FAIL — getent hosts acme-v02.api.letsencrypt.org timeout
ACME HTTPS directory after fix: FAIL — curl resolving timed out after 10000 ms
dig @1.1.1.1: FAIL — communications error timed out
dig @8.8.8.8: FAIL — communications error timed out
```

Do not store full resolver backup contents in repo.

Likely cause: outbound UDP/TCP 53 (and general egress DNS/HTTPS) blocked at Selectel Security Group or network layer — changing resolver config on the server is insufficient until outbound egress is permitted.

## Certbot Retry

```text
Certbot executed: no — blocked by DNS fix failure
Certbot result: not attempted
Certificate private key captured in docs: no
Nginx config test after DNS fix attempt: PASS
Nginx reload after DNS fix attempt: not required
```

## Verification Matrix

| Check                          | Result | Notes                                                        |
| ------------------------------ | ------ | ------------------------------------------------------------ |
| DNS A record                   | PASS   | 161.104.53.221 — verified from operator workstation          |
| HTTP before Certbot            | PASS   | 200 — external and nginx local Host header                   |
| Server ACME DNS after fix      | FAIL   | acme-v02.api.letsencrypt.org — timeout                       |
| Server ACME HTTPS directory    | FAIL   | curl resolving timed out                                     |
| Certbot retry                  | FAIL   | not executed — DNS fix prerequisite failed                   |
| HTTPS /health punycode         | FAIL   | certificate not issued                                       |
| HTTP -> HTTPS redirect         | FAIL   | not configured                                               |
| HTTPS /health display Cyrillic | FAIL   | certificate not issued                                       |
| Certbot renewal dry-run        | FAIL   | not run — certificate not issued                             |

## Machine-captured Output (sanitized)

Pre-fix server state:

```text
/etc/resolv.conf -> /run/systemd/resolve/resolv.conf
nameserver 188.93.16.19
nameserver 188.93.17.19
getent hosts acme-v02.api.letsencrypt.org: FAIL (timeout)
curl ACME directory: Resolving timed out after 10000 ms
```

Post-fix server state:

```text
/etc/resolv.conf -> /run/systemd/resolve/stub-resolv.conf
nameserver 127.0.0.53
Global DNS Servers: 1.1.1.1 8.8.8.8 9.9.9.9
getent hosts acme-v02.api.letsencrypt.org: FAIL (timeout)
dig @1.1.1.1: no servers could be reached
dig @8.8.8.8: no servers could be reached
curl ACME directory: Resolving timed out after 10000 ms
nginx -t: successful
nginx80 local Host header: 200
```

External verification:

```text
HTTP http://staging.xn--80abvubqje.xn--p1ai/health: 200
HTTPS https://staging.xn--80abvubqje.xn--p1ai/health: FAIL (000)
```

## Decision

```text
STG_LIM_002_OUTBOUND_DNS_FIX_FAIL
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
1. Allow outbound egress on Selectel Security Group for 161.104.53.221:
   - UDP/TCP 53 to public DNS resolvers (or Selectel resolvers 188.93.16.19 / 188.93.17.19)
   - TCP 443 to Let's Encrypt ACME endpoints
2. Verify from server: getent hosts acme-v02.api.letsencrypt.org
3. Re-run STG-LIM-002 Outbound DNS Fix + Certbot Retry Pack
4. DNS resolver rollback available from /root/dns-backup-before-certbot-retry-20260717_182208
5. Nginx rollback available from /root/nginx-backup-before-certbot-20260717_165628
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
