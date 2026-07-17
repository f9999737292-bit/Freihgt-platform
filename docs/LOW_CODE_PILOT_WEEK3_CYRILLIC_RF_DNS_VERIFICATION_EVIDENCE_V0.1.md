# Low-code Pilot Week-3 Cyrillic .рф DNS Verification Evidence v0.1

## Summary

DNS verification was executed for the active Cyrillic .рф staging domain on 2026-07-17.

Production-ready is not claimed.

Operator reported DNS A-record created; machine verification from this workstation did not resolve the domain on public resolvers. HTTP `/health` by domain returned 503 via VPN proxy intercept (DNS unresolved); direct IP health remains 200.

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

| Check                                         | Result | Notes                                                                 |
| --------------------------------------------- | ------ | --------------------------------------------------------------------- |
| http://staging.xn--80abvubqje.xn--p1ai/health | FAIL   | 503 via VPN proxy intercept; curl exit 6 could not resolve host       |
| http://staging.бинтранс.рф/health             | FAIL   | 503 via VPN proxy intercept; expected 200 if DNS + nginx vhost ready |

## Decision

```text
CYRILLIC_RF_DNS_VERIFICATION_FAIL
```

## STG-LIM-001

DNS and HTTP health did not pass — closure candidate not advanced:

```text
OPEN_DNS_PENDING_CYRILLIC_RF_DOMAIN — verification failed 2026-07-17
```

Do not close STG-LIM-002. HTTPS remains pending Certbot and SSH readiness.

## Open Limitations

```text
STG-LIM-001: OPEN — DNS A-record not verified; re-check propagation / registrar record
STG-LIM-002: OPEN — HTTPS pending DNS + SSH readiness
STG-LIM-003: OPEN — SSH SG /32
STG-LIM-004: OPEN — web-admin deploy pending
```

## Production-ready

```text
not claimed
```

## Safety

```text
Certbot executed: no
Nginx changed: no
Web-admin deployed: no
Writes executed: no
Secrets captured: no
```

## Next operator action

```text
1. Confirm A-record at registrar: staging.бинтранс.рф -> 161.104.53.221
2. Confirm punycode equivalent: staging.xn--80abvubqje.xn--p1ai -> 161.104.53.221
3. Wait for DNS propagation; re-run Cyrillic .рф DNS Verification Evidence Pack v0.1
4. Ensure nginx server_name includes staging domain before expecting domain /health 200
```
