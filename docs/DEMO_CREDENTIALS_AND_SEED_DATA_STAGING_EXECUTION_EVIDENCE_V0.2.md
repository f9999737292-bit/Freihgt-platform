# Demo Credentials and Seed Data Staging Execution Evidence v0.2

## Summary

Evidence for staging-only demo credentials and seed data execution after staging backend/DB isolation.

## Result

```text
DEMO_CREDENTIALS_AND_SEED_DATA_STAGING_EXECUTION_COMPLETE
```

## Pre-flight

| Check                                                | Result     |
| ---------------------------------------------------- | ---------- |
| branch                                               | main       |
| HEAD                                                 | 35466c0    |
| HEAD = origin/main                                   | yes        |
| source diff                                          | empty      |
| staged files before pack                             | none       |
| explicit staging credentials/seed execution approval | given      |

## Endpoint Baseline

| Endpoint               | Before              | After               |
| ---------------------- | ------------------- | ------------------- |
| prod /                 | 200 text/html       | 200 text/html       |
| prod /login            | 200 text/html       | 200 text/html       |
| prod /health           | 200 application/json | 200 application/json |
| prod /api/v1/companies | 400 application/json | 400 application/json |
| stg /                  | 200 text/html       | 200 text/html       |
| stg /login             | 200 text/html       | 200 text/html       |
| stg /health            | 200 application/json | 200 application/json |
| stg /api/v1/companies  | 400 application/json | 400 application/json |

## Gates

| Gate                       | Result |
| -------------------------- | ------ |
| staging isolation gate     | PASS   |
| supported seed method gate | PASS   |
| secret handling gate       | PASS   |

## Seed Method

```text
Method: seed_dev_admin.sh + seed_demo_data.sh + post-seed API fixes
Target: http://127.0.0.1:18080 (staging gateway)
Service URLs: http://127.0.0.1:18080/api (gateway proxy)
Postgres container: bintrans-staging-postgres (tenant insert only)
Idempotent: yes
Passwords printed to chat: no (logs redirected to server secret dir)
```

## Created Or Verified Staging Demo Objects

Do not include passwords/tokens.

| Object            | Alias/Name                       | Status  | Safe ID if available |
| ----------------- | -------------------------------- | ------- | -------------------- |
| tenant            | DEMO_BINTRANS_TENANT             | created | da13ede3-e957-4618-965f-926807fc643e |
| user              | DEMO_PLATFORM_ADMIN              | created | 180b792b-4ff9-4dfb-95a6-cefce0952c0a |
| user              | DEMO_SHIPPER_ADMIN               | created | f48cf684-1b87-4931-b107-12dd76b110c2 |
| user              | DEMO_CARRIER_ADMIN               | created | 4e16d805-7fc6-4a18-924c-e3b33776db1b |
| user              | DEMO_FINANCE_MANAGER             | created | 9af37ff6-866d-46b4-9d89-1a50c3247eb3 |
| company           | DEMO Shipper Company             | created | 7803f029-47e1-4321-a41a-d89d0e86b413 |
| company           | DEMO Carrier Company             | created | 1addc7ec-2345-4012-8cf0-1342ec6de856 |
| RFx               | DEMO RFx 001                     | created | 68999503-0af6-4da8-83c3-f7b710faf064 |
| transport order   | DEMO-TO-001..005                 | created | 5 records |
| shipment          | DEMO-SH-PLANNED                  | created | planned |
| shipment          | DEMO-SH-IN-PROGRESS              | created | in transit |
| shipment          | DEMO-SH-BILLING                  | created | billing-ready |
| billing register  | DEMO-BR-001                      | created | 7bcbf89f-de18-42f4-a550-f99f5cda9717 |
| document metadata | DEMO-DOC-001                     | created | a8d08846-abcc-487b-bf7c-90b362362378 |

## Notes

```text
Additional legacy demo users from seed_demo_data defaults (shipper@7rights.local etc.) exist in staging tenant; approved alias users were created separately.
Shipment DEMO-SH-BILLING advanced to READY_FOR_BILLING (billing workflow demo).
Transport orders exceed minimum approved count (5 vs 3 required).
```

## Safety Result

```text
Production changed in this pack: no
Production writes executed: no
Production live-data demo approved: no
Staging writes executed: yes
Credentials created: yes
Passwords generated: yes
Passwords recorded in repo/docs/chat: no
Seed data created: yes
Credentials entered into browser login: no
Fake session created: no
Server changed: no (staging data only)
Nginx changed: no
Nginx reload executed: no
DNS changed: no
Certbot changed: no
Docker restarted: no
Backend changed: no
API contracts changed: no
Migrations changed: no
Source code changed: no
Ports opened publicly: no
Secrets captured: no
```
