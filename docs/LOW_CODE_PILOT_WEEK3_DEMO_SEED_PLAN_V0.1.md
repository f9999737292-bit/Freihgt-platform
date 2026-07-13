# Low-code Pilot Week-3 Demo Seed Plan v0.1

## Summary

Docs-only plan for staging demo seed execution covering STG-LIM-005 (full demo UI seed-data) and STG-LIM-006 (low-code custom field values).

No seed scripts executed in this pack. No SSH commands run. No staging writes. No secrets stored. Production-ready not claimed.

## Decision

```text
DEMO_SEED_PLAN_CREATED_PENDING_EXECUTION
```

## Environment

Staging server:

```text
161.104.53.221
```

Deploy root:

```text
/opt/bintrans/freight-platform
```

API gateway (current):

```text
http://161.104.53.221/api/v1
```

Tenant ID (existing):

```text
74519f22-ff9b-4a8b-8fff-a958c689682f
```

Tenant code (staging):

```text
dev-bintrans
```

## Current Staging Seed State

| Item | Status |
| ---- | ------ |
| Migrations | applied (11/11) |
| `make seed-dev-admin` | executed — admin available |
| `make seed-lowcode-demo` templates | executed — 6 published templates |
| `make seed-demo-data` | **not executed** |
| Custom field values (STG-LIM-006) | **skipped** — demo entities absent |
| Admin user | available — `admin@bintrans.local` |
| Shipper user | available — `shipper@bintrans.local` |
| Carrier / forwarder / consignee users | **needed** |
| Demo companies (carrier, forwarder, consignee) | **needed** |
| Demo entities (DEMO-TO-*, DEMO-SH-*, etc.) | **needed** |

## Target Demo Users (after seed)

| Role | Email | Script default | Staging override |
| ---- | ----- | -------------- | ---------------- |
| PLATFORM_ADMIN | admin@bintrans.local | admin@7rights.local | already exists |
| SHIPPER_LOGIST | shipper@bintrans.local | shipper@7rights.local | already exists |
| CARRIER_DISPATCHER | carrier@bintrans.local | carrier@7rights.local | create via seed |
| PROCUREMENT_MANAGER | forwarder@bintrans.local | forwarder@7rights.local | create via seed |
| CONSIGNEE_OPERATOR | consignee@bintrans.local | consignee@7rights.local | create via seed |

Passwords: **credentials provided separately / not stored in docs**.

## Target Demo Companies (after seed)

| Company | Type | Script name |
| ------- | ---- | ----------- |
| ООО Bintrans Dev Tenant | PLATFORM_OPERATOR | align tenant name |
| ООО Грузовладелец Север | SHIPPER | exists / seed ensures |
| ООО Перевозчик Волга | CARRIER | seed-demo-data |
| ООО Экспедитор Логистик | FORWARDER | seed-demo-data |
| ООО Грузополучатель Центр | CONSIGNEE | seed-demo-data |

Controlled pilot test plan references slightly different names for carrier/forwarder/consignee — execution may align names via env vars; functional roles matter more than display labels.

## Seed Scripts

| Script | Make target | Purpose | STG-LIM |
| ------ | ----------- | ------- | ------- |
| `scripts/dev/seed_dev_admin.sh` | `make seed-dev-admin` | Tenant + PLATFORM_ADMIN | prerequisite — done |
| `scripts/dev/seed_demo_data.sh` | `make seed-demo-data` | Companies, users, DEMO-TO/FR/SH/DOC/BR/RFX | **STG-LIM-005** |
| `scripts/dev/seed_lowcode_demo.sh` | `make seed-lowcode-demo` | Published templates + custom field values | templates done; **STG-LIM-006** values pending |

## Prerequisites

| # | Prerequisite | Status |
| - | ------------ | ------ |
| 1 | Platform healthy (`/health` 200) | **pass** |
| 2 | Migrations applied | **pass** |
| 3 | Trusted SSH available | **pass** |
| 4 | `seed-dev-admin` executed | **pass** |
| 5 | Low-code templates published | **pass** (6 templates) |
| 6 | Operator approval for staging seed writes | **pending** |
| 7 | DNS / HTTPS | **not required** for seed execution |
| 8 | Web-admin deployed | **not required** for API seed |

## Phase 1 — Operator Approval (required)

Explicit approval text:

```text
разрешаю staging seed execution на demo data
```

Without this approval, do **not** run `seed-demo-data` or custom field value seed on staging.

## Phase 2 — Staging Env Overrides (docs-only reference)

On staging server via SSH — **execute only after approval**:

```bash
cd /opt/bintrans/freight-platform

export API_GATEWAY_URL=http://161.104.53.221
export TENANT_ID=74519f22-ff9b-4a8b-8fff-a958c689682f
export TENANT_CODE=dev-bintrans
export TENANT_NAME="ООО Bintrans Dev Tenant"
export ADMIN_EMAIL=admin@bintrans.local
export COMPANY_LEGAL_NAME="ООО Bintrans Dev Tenant"
# DEMO_PASSWORD and ADMIN_PASSWORD — set from secure channel, not in docs
```

## Phase 3 — Execute seed-demo-data (STG-LIM-005)

```bash
make seed-demo-data
```

Expected demo entities (idempotent):

| Entity | Example numbers |
| ------ | --------------- |
| Transport orders | DEMO-TO-001..005 |
| Freight requests | DEMO-FR-001..003 |
| Shipments | DEMO-SH-PLANNED, DEMO-SH-PROGRESS, DEMO-SH-BILLING |
| Documents | DEMO-DOC-001 |
| Billing registers | DEMO-BR-001 |
| RFX events | DEMO-RFX-001 |

## Phase 4 — Custom Field Values (STG-LIM-006)

After `seed-demo-data` succeeds, re-run custom field value section:

```bash
make seed-lowcode-demo
```

Script seeds custom field values for entities that exist:

* TRANSPORT_ORDER DEMO-TO-001
* SHIPMENT DEMO-SH-PLANNED
* BILLING_REGISTER DEMO-BR-001
* FREIGHT_REQUEST DEMO-FR-001 (if present)
* DOCUMENT DEMO-DOC-001 (if present)
* RFX DEMO-RFX-001 (if present)

## Phase 5 — Post-seed Verification (read-only)

| Check | Method | Expected |
| ----- | ------ | -------- |
| Gateway health | GET `/health` | 200 |
| Demo TO list | GET transport-orders with tenant header | DEMO-TO-* visible |
| Runtime template | GET active template | 200 |
| Custom field values | GET custom-field-values for DEMO-TO-001 | 200 with values |
| Auth-on still works | admin 200 / non-admin 403 | unchanged |

No additional writes during verification.

## STG-LIM Impact

| ID | Before | After this pack |
| -- | ------ | --------------- |
| STG-LIM-005 | OPEN | OPEN — plan created; execution pending |
| STG-LIM-006 | OPEN | OPEN — plan created; execution pending |

## Production-ready

```text
not claimed
```

## Safety

| Rule | Status |
| ---- | ------ |
| Production data | forbidden |
| Secrets in docs | no |
| Staging writes in this pack | no |
| Backend code changed | no |
| Migrations created | no |

## Next Pack

```text
Demo Seed Execution Pack v0.1 (after operator approval)
```

## Related Docs

```text
docs/LOW_CODE_PILOT_WEEK3_DEMO_SEED_CHECKLIST_V0.1.md
docs/LOW_CODE_PILOT_WEEK3_CONTROLLED_PILOT_TEST_PLAN_V0.1.md
docs/LOW_CODE_PILOT_WEEK3_STAGING_DEPLOY_RUNBOOK_V0.1.md
docs/LOW_CODE_PILOT_WEEK3_REMOTE_AUTH_ON_STAGING_REPEAT_EVIDENCE_V0.1.md
```
