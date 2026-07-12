# Low-code Pilot Week-3 Bintrans HTTPS / Certbot Checklist v0.1

## Summary

Operator and execution checklist for enabling HTTPS on `staging.bintrans.ru`.

## Phase 1 — DNS (operator, before HTTPS)

| Step | Action | Done |
| ---- | ------ | ---- |
| 1 | Create A-record `staging.bintrans.ru` → `161.104.53.221` | ☐ |
| 2 | Optional: A-record `pilot.bintrans.ru` → `161.104.53.221` | ☐ |
| 3 | Verify: `Resolve-DnsName staging.bintrans.ru` → `161.104.53.221` | ☐ |
| 4 | Verify: `http://staging.bintrans.ru/health` → 200 | ☐ |

## Phase 2 — Nginx prep (server, after DNS + SSH approval)

| Step | Action | Done |
| ---- | ------ | ---- |
| 5 | Confirm Nginx installed: `nginx -v` | ☐ |
| 6 | Add `server_name staging.bintrans.ru` HTTP block | ☐ |
| 7 | `nginx -t` passes | ☐ |
| 8 | `systemctl reload nginx` | ☐ |
| 9 | HTTP domain health returns 200 | ☐ |

## Phase 3 — Certbot (server, after Phase 2)

| Step | Action | Done |
| ---- | ------ | ---- |
| 10 | Confirm Certbot installed: `certbot --version` | ☐ |
| 11 | `certbot --nginx -d staging.bintrans.ru` | ☐ |
| 12 | `https://staging.bintrans.ru/health` → 200 | ☐ |
| 13 | Capture evidence pack | ☐ |

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
