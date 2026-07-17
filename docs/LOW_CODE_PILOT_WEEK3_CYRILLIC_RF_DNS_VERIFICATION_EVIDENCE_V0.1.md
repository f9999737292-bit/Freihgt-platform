# Low-code Pilot Week-3 Cyrillic .рф DNS Verification Evidence v0.1

## Summary

DNS verification was executed for the active Cyrillic .рф staging domain on 2026-07-17.

**Initial attempt (v0.1):** FAIL — NXDOMAIN on public and authoritative resolvers; domain `/health` 503 via VPN proxy intercept.

**Retry (v0.2):** PASS — DNS delegation and A-record propagated; domain `/health` 200.

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

## DNS Results

Initial attempt (2026-07-17, before propagation):

| Check                                           | Result | Notes                                                                 |
| ----------------------------------------------- | ------ | --------------------------------------------------------------------- |
| Resolve-DnsName staging.бинтранс.рф             | FAIL   | DNS name does not exist; PowerShell IDN encoding may affect query     |
| Resolve-DnsName staging.xn--80abvubqje.xn--p1ai | FAIL   | DNS name does not exist; tested default, 8.8.8.8, 9.9.9.9, 1.1.1.1   |
| nslookup staging.бинтранс.рф                    | FAIL   | Non-existent domain (1.1.1.1, 8.8.8.8)                                |
| nslookup staging.xn--80abvubqje.xn--p1ai        | FAIL   | Non-existent domain (1.1.1.1, 8.8.8.8, 9.9.9.9)                       |

Expected IP:

```text
161.104.53.221
```

Observed IP:

```text
none — no A record returned by any tested resolver
```

Reference (not domain pass criteria):

```text
http://161.104.53.221/health — 200 OK
```

## HTTP Results

Initial attempt (2026-07-17, before propagation):

| Check                                         | Result | Notes                                                                 |
| --------------------------------------------- | ------ | --------------------------------------------------------------------- |
| http://staging.xn--80abvubqje.xn--p1ai/health | FAIL   | 503 via VPN proxy intercept; curl exit 6 could not resolve host       |
| http://staging.бинтранс.рф/health             | FAIL   | 503 via VPN proxy intercept; expected 200 if DNS + nginx vhost ready |

## Initial Decision

```text
CYRILLIC_RF_DNS_VERIFICATION_FAIL
```

## STG-LIM-001 (initial)

DNS and HTTP health did not pass — closure candidate not advanced:

```text
OPEN_DNS_PENDING_CYRILLIC_RF_DOMAIN — verification failed 2026-07-17 (initial attempt)
```

Do not close STG-LIM-002. HTTPS remains pending Certbot and SSH readiness.

## Retry Result — DNS Propagation Completed

Decision:

```text
CYRILLIC_RF_DNS_VERIFICATION_PASS
```

NS apex from public resolver:

```text
xn--80abvubqje.xn--p1ai NS ns3-l2.nic.ru
xn--80abvubqje.xn--p1ai NS ns4-l2.nic.ru
xn--80abvubqje.xn--p1ai NS ns4-cloud.nic.ru
xn--80abvubqje.xn--p1ai NS ns8-l2.nic.ru
xn--80abvubqje.xn--p1ai NS ns8-cloud.nic.ru
```

A-record:

```text
staging.xn--80abvubqje.xn--p1ai A 161.104.53.221
```

Verification matrix:

| Check            | Result                |
| ---------------- | --------------------- |
| ns3-l2.nic.ru    | PASS — 161.104.53.221 |
| ns4-l2.nic.ru    | PASS — 161.104.53.221 |
| ns8-l2.nic.ru    | PASS — 161.104.53.221 |
| ns4-cloud.nic.ru | PASS — 161.104.53.221 |
| ns8-cloud.nic.ru | PASS — 161.104.53.221 |
| 1.1.1.1          | PASS — 161.104.53.221 |
| 8.8.8.8          | PASS — 161.104.53.221 |
| default resolver | PASS — 161.104.53.221 |
| HTTP /health     | PASS — 200            |

STG-LIM-001:

```text
READY_FOR_CLOSURE_REVIEW
```

Production-ready:

```text
not claimed
```

Safety:

```text
Certbot executed: no
Nginx changed: no
Web-admin deployed: no
Writes executed: no
Secrets captured: no
```

## Open Limitations (after retry)

```text
STG-LIM-001: READY_FOR_CLOSURE_REVIEW — DNS verified 2026-07-17 (retry)
STG-LIM-002: OPEN — HTTPS pending DNS + SSH readiness
STG-LIM-003: OPEN — SSH SG /32
STG-LIM-004: OPEN — web-admin deploy pending
```

## Next operator action

```text
1. Review STG-LIM-001 closure
2. Do not run Certbot until explicit approval
3. HTTPS / Certbot pack only after separate approval
4. Fix/verify SSH SG /32 (STG-LIM-003)
5. Web-admin deploy only after separate approval (STG-LIM-004)
```
