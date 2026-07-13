# Low-code Pilot Week-3 Demo Seed Checklist v0.1

## Summary

Checklist for executing demo seed on Selectel staging (STG-LIM-005 / STG-LIM-006).

## Phase 1 — Prerequisites

| Step | Action | Done |
| ---- | ------ | ---- |
| 1 | `/health` returns 200 | ☐ |
| 2 | Migrations applied (11/11) | ☐ |
| 3 | Trusted SSH available | ☐ |
| 4 | `seed-dev-admin` already run | ☐ |
| 5 | Low-code templates published (6) | ☐ |
| 6 | Operator approval: `разрешаю staging seed execution на demo data` | ☐ |

## Phase 2 — Environment

| Step | Action | Done |
| ---- | ------ | ---- |
| 7 | Set `API_GATEWAY_URL=http://161.104.53.221` | ☐ |
| 8 | Set `TENANT_ID=74519f22-ff9b-4a8b-8fff-a958c689682f` | ☐ |
| 9 | Set Bintrans email overrides (admin/shipper/carrier/forwarder/consignee) | ☐ |
| 10 | Set passwords from secure channel (not in docs) | ☐ |

## Phase 3 — STG-LIM-005 seed-demo-data

| Step | Action | Done |
| ---- | ------ | ---- |
| 11 | `make seed-demo-data` succeeds | ☐ |
| 12 | Demo companies created (shipper, carrier, forwarder, consignee) | ☐ |
| 13 | Demo users created and role-assigned | ☐ |
| 14 | DEMO-TO-001..005 exist | ☐ |
| 15 | DEMO-SH-PLANNED / DEMO-SH-PROGRESS / DEMO-SH-BILLING exist | ☐ |
| 16 | DEMO-DOC-001 exists | ☐ |
| 17 | DEMO-BR-001 exists | ☐ |
| 18 | DEMO-RFX-001 exists | ☐ |

## Phase 4 — STG-LIM-006 custom field values

| Step | Action | Done |
| ---- | ------ | ---- |
| 19 | Re-run `make seed-lowcode-demo` after demo entities exist | ☐ |
| 20 | Custom field values for DEMO-TO-001 seeded | ☐ |
| 21 | Custom field values for DEMO-SH-PLANNED seeded | ☐ |
| 22 | Custom field values for DEMO-BR-001 seeded | ☐ |

## Phase 5 — Verify (read-only)

| Step | Action | Done |
| ---- | ------ | ---- |
| 23 | GET health 200 | ☐ |
| 24 | GET custom-field-values for DEMO-TO-001 returns 200 | ☐ |
| 25 | Auth-on matrix still pass | ☐ |
| 26 | Capture evidence pack | ☐ |

## Blockers

| ID | Status |
| -- | ------ |
| STG-LIM-001 | OPEN — DNS pending |
| STG-LIM-002 | OPEN — HTTPS pending |
| STG-LIM-003 | OPEN — deferred |
| STG-LIM-004 | OPEN — web-admin execution pending |
| STG-LIM-005 | OPEN — plan created, execution pending |
| STG-LIM-006 | OPEN — plan created, execution pending |

## Status

```text
DEMO_SEED_PLAN_CREATED_PENDING_EXECUTION
```

## Production-ready

```text
not claimed
```
