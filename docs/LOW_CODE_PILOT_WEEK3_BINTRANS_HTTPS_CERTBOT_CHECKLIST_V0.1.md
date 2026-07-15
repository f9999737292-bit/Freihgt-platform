# Low-code Pilot Week-3 Bintrans HTTPS / Certbot Checklist v0.1

## Summary

Operator and execution checklist for enabling HTTPS on `staging.бинтранс.рф`.

Technical commands use punycode: `staging.xn--80abvubqje.xn--p1ai`.

## Phase 1 — DNS (operator, before HTTPS)

| Step | Action | Done |
| ---- | ------ | ---- |
| 1 | Create A-record `staging.бинтранс.рф` → `161.104.53.221` | ☐ |
| 2 | Punycode equivalent: A-record `staging.xn--80abvubqje.xn--p1ai` → `161.104.53.221` | ☐ |
| 3 | Verify: `nslookup staging.бинтранс.рф` → `161.104.53.221` | ☐ |
| 4 | Verify: `nslookup staging.xn--80abvubqje.xn--p1ai` → `161.104.53.221` | ☐ |
| 5 | Verify: `http://staging.xn--80abvubqje.xn--p1ai/health` → 200 | ☐ |

## Phase 2 — Nginx prep (server, after DNS + SSH approval)

| Step | Action | Done |
| ---- | ------ | ---- |
| 6 | Confirm Nginx installed: `nginx -v` | ☐ |
| 7 | Add `server_name staging.xn--80abvubqje.xn--p1ai` HTTP block | ☐ |
| 8 | `nginx -t` passes | ☐ |
| 9 | `systemctl reload nginx` | ☐ |
| 10 | HTTP domain health returns 200 | ☐ |

## Phase 3 — Certbot (server, after Phase 2)

| Step | Action | Done |
| ---- | ------ | ---- |
| 11 | Confirm Certbot installed: `certbot --version` | ☐ |
| 12 | `certbot --nginx -d staging.xn--80abvubqje.xn--p1ai` | ☐ |
| 13 | `https://staging.xn--80abvubqje.xn--p1ai/health` → 200 | ☐ |
| 14 | Capture evidence pack | ☐ |

## Blockers

| ID | Status |
| -- | ------ |
| STG-LIM-001 | OPEN — DNS pending |
| STG-LIM-002 | OPEN — execution pending |
| STG-LIM-003 | OPEN — external port 22 scan deferred per operator |

## Do Not Execute Until

* DNS resolves to `161.104.53.221`
* Operator approves server-side execution
* production-ready remains not claimed

## Status

```text
HTTPS_PREP_DOCS_ONLY_DNS_PENDING
```

## Production-ready

```text
not claimed
```
