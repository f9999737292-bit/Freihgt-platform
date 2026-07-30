# Demo Credentials and Seed Data Staging Isolation Check v0.1

## Summary

Read-only staging isolation check before any staging writes.

Check date: 2026-07-30.

Server: `161.104.53.221` (Selectel).

Method: read-only SSH inspection of Nginx sites and Docker container names. No env/secrets captured.

## Result

```text
STAGING_ISOLATION_GATE_BLOCKED
```

## Evidence

| Check                                 | Result |
| ------------------------------------- | ------ |
| production UI root                    | `/var/www/bintrans-web-admin` (Nginx `00-bintrans-production.conf`) |
| staging UI root                       | `/var/www/staging-bintrans-web-admin` (Nginx `staging-bintrans.conf` enabled) |
| production API proxy target           | `http://127.0.0.1:8080` (`/api/` location) |
| staging API proxy target              | `http://127.0.0.1:8080` (`/api/` location) — **same as production** |
| production health proxy target        | `http://127.0.0.1:8080/health` |
| staging health proxy target           | `http://127.0.0.1:8080/health` — **same as production** |
| separate staging backend proven       | **no** — single container `freight_api_gateway` on `:8080` |
| separate staging DB/data scope proven | **no** — single container `freight_postgres` on `:5432` |
| production write risk                 | **yes** — staging API calls would hit shared gateway/DB |

## Nginx Isolation Detail

Production and staging have **separate static UI roots** but **identical API/health proxy targets**:

```text
production: root /var/www/bintrans-web-admin; proxy_pass http://127.0.0.1:8080
staging:    root /var/www/staging-bintrans-web-admin; proxy_pass http://127.0.0.1:8080
```

## Docker Isolation Detail

```text
freight_api_gateway                0.0.0.0:8080->8080/tcp
freight_postgres                   0.0.0.0:5432->5432/tcp
(+ identity, company, transport-order, rfx, shipment, document, billing-register, low-code services)
```

No second gateway or Postgres instance observed for staging.

## Decision

```text
STAGING_ISOLATION_GATE_BLOCKED
```

## Blocker Classification

```text
staging /api and production /api proxy to the same backend service (127.0.0.1:8080)
staging /health and production /health proxy to the same gateway
staging and production share the same DB/data scope (freight_postgres)
```

## Notes

```text
No secrets captured.
No production writes executed.
No staging writes executed.
Execution stopped before credentials/seed data creation.
UI-only isolation is insufficient for approved staging-first demo data writes.
```

## Next Recommended Pack

```text
STAGING_BACKEND_DB_ISOLATION_PLAN_PACK v0.1
```
