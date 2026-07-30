# Demo Credentials and Seed Data Staging Execution Evidence v0.1

## Summary

Evidence for staging-only demo credentials and seed data execution.

Execution date: 2026-07-30.

Base commit: `8f7eaeb`.

Explicit staging write approval: **given by user**.

## Result

```text
DEMO_CREDENTIALS_AND_SEED_DATA_STAGING_EXECUTION_BLOCKED
```

## Pre-flight

| Check                    | Result |
| ------------------------ | ------ |
| branch                   | main   |
| HEAD                     | `8f7eaeb` |
| HEAD = origin/main       | yes    |
| source diff              | empty  |
| staged files before pack | none   |

## Endpoint Baseline

| Endpoint     | Before | After |
| ------------ | ------ | ----- |
| prod /       | 200 text/html | 200 text/html |
| prod /login  | 200 text/html | 200 text/html |
| prod /health | 200 application/json | 200 application/json |
| stg /        | 200 text/html | 200 text/html |
| stg /login   | 200 text/html | 200 text/html |
| stg /health  | 200 application/json | 200 application/json |

Production `/api/v1/companies` before: 400 application/json (route exists).
Production `/api/health` before: 404 application/json (expected).

No endpoint regression observed. No writes were performed.

## Gates

| Gate                       | Result |
| -------------------------- | ------ |
| staging isolation gate     | **BLOCKED** |
| supported seed method gate | **not checked** (stopped at isolation gate) |
| secret handling gate       | **PASS** (no secrets generated or recorded) |

## Isolation Gate Findings

| Item | Finding |
|---|---|
| production API proxy | `http://127.0.0.1:8080` |
| staging API proxy | `http://127.0.0.1:8080` — shared |
| production health proxy | `http://127.0.0.1:8080/health` |
| staging health proxy | `http://127.0.0.1:8080/health` — shared |
| gateway container | `freight_api_gateway` (single instance) |
| postgres container | `freight_postgres` (single instance) |
| production UI root | `/var/www/bintrans-web-admin` |
| staging UI root | `/var/www/staging-bintrans-web-admin` |

## Created Or Verified Staging Demo Objects

Execution blocked — no objects created or verified.

| Object            | Alias/Name                       | Status       | Safe ID if available |
| ----------------- | -------------------------------- | ------------ | -------------------- |
| tenant            | DEMO_BINTRANS_TENANT             | not created  | — |
| user              | DEMO_PLATFORM_ADMIN              | not created  | — |
| user              | DEMO_SHIPPER_ADMIN               | not created  | — |
| user              | DEMO_CARRIER_ADMIN               | not created  | — |
| user              | DEMO_FINANCE_MANAGER             | not created  | — |
| company           | DEMO Shipper Company             | not created  | — |
| company           | DEMO Carrier Company             | not created  | — |
| RFx               | DEMO RFx 001                     | not created  | — |
| transport order   | DEMO Transport Order Draft       | not created  | — |
| transport order   | DEMO Transport Order In Progress | not created  | — |
| transport order   | DEMO Transport Order Completed   | not created  | — |
| shipment          | DEMO Shipment In Transit         | not created  | — |
| shipment          | DEMO Shipment Delivered          | not created  | — |
| billing register  | DEMO Billing Register 001        | not created  | — |
| document metadata | DEMO Document Metadata 001       | not created  | — |

## Blocker

```text
Staging and production share the same API gateway (127.0.0.1:8080 / freight_api_gateway) and the same Postgres data scope (freight_postgres). Staging API writes cannot be proven isolated from production data. Explicit staging write approval does not override this production-risk gate.
```

## Safety Result

```text
Production changed in this pack: no
Production writes executed: no
Production deploy executed: no
Production live-data demo approved: no
Staging writes executed: no
Credentials created: no
Passwords generated: no
Passwords recorded in repo/docs/chat: no
Seed data created: no
Credentials entered into login screen: no
Fake session created: no
Server changed: no
Nginx changed: no
Nginx reload executed: no
DNS changed: no
Certbot changed: no
Docker restarted: no
Backend changed: no
API contracts changed: no
Migrations changed: no
Source code changed: no
Ports opened: no
Secrets captured: no
```

## Next Recommended Pack

```text
STAGING_BACKEND_DB_ISOLATION_PLAN_PACK v0.1
```
