# Control Tower Alert Acknowledgement v0.1 — Post-Merge / Staging Verification

## Source

| Field | Value |
|-------|-------|
| MAIN_SHA | `601fdb908f4a5e8690fdf502aa20f92e09a06972` |
| Worktree | `D:\Projects\freight-platform-wt\ct-alert-ack-post-merge` |
| Branch | `verify/control-tower-alert-ack-post-merge-v0.1` |
| Environment | **LOCAL/STAGING-SHADOW** (Docker Compose + `docker-compose.staging-shadow.yml` on localhost) |
| Date | 2026-08-13 |

**Not verified on Selectel staging:** `docs/CONTROL_TOWER_SELECTEL_STAGING_DEPLOYMENT_HANDOFF.md` status remains `STAGING_EXECUTION_PENDING_OPS_ACCESS`; no protected staging credentials or SSH target were available in this session.

| Safety metadata | Value |
|-----------------|-------|
| ENVIRONMENT_TYPE | LOCAL/STAGING-SHADOW |
| API_HOST | `127.0.0.1:8080` |
| DB_TARGET | `local-docker-freight_platform` |
| PRODUCTION_TARGET | NO |
| SAFE_STAGING_TARGET_CONFIRMED | YES (local non-production only) |
| NETWORK_FETCH | PASS (`origin/main` matches MAIN_SHA) |

---

## Migration

### Static migration review

**STATIC_MIGRATION_REVIEW=PASS**

Reviewed:

- `infrastructure/migrations/000020_create_control_tower_critical_event_acknowledgement_v0.1.up.sql`
- `infrastructure/migrations/000020_create_control_tower_critical_event_acknowledgement_v0.1.down.sql`

| Check | Result |
|-------|--------|
| Table `control_tower.critical_event_acknowledgement` | Matches ARCHITECTURE.md |
| PK `(tenant_id, event_id)` | Present — idempotency key correct |
| `tenant_id`, `event_id`, `acknowledged_by_user_id`, `acknowledged_at` | Present |
| `event_id` format CHECK `^[0-9a-f]{32}$` | Present |
| `source = control-tower` CHECK | Present |
| Secondary index `(tenant_id, shipment_id)` | Present |
| Down migration drops table only | Symmetric |

### Runtime migration

| Field | Value |
|-------|-------|
| Migration version before | **19** |
| Migration 000020 applied | **PASS** (`make migrate-up`) |
| Migration version after | **20** |
| Ack table exists | **YES** (`control_tower.critical_event_acknowledgement`) |

Down migration was **not** executed against the shared local database (by design).

---

## Service Health

| Check | Result | Notes |
|-------|--------|-------|
| API Gateway `/health` | **PASS** | `127.0.0.1:8080` |
| Read model `/health` | **PASS** | `127.0.0.1:8089` |
| Base / post smoke | **PASS** | Summary GET 200 after mutations; gateway `/ready` healthy |

**Runtime note:** Initial acknowledge attempts returned HTTP 404 with **0 ms** duration against **pre-rebuild** gateway images (route absent). Services were rebuilt from MAIN_SHA (`api-gateway`, `control-tower-read-model-service`) and restarted with staging-shadow overlay before successful E2E.

---

## Test identities (non-secret)

| Alias | Tenant | User ID | Role / purpose |
|-------|--------|---------|----------------|
| AUTHORIZED_USER_A | `74519f22-ff9b-4a8b-8fff-a958c689682f` (dev-bintrans) | `8541a3a3-bde7-4fed-9501-37b9953bf904` | PLATFORM_ADMIN |
| AUTHORIZED_USER_B | same tenant | `008e1462-6f67-4246-b7dc-4aae1669c0c5` | SHIPPER_LOGIST |
| UNAUTHORIZED_USER | same tenant | forwarder demo user | PROCUREMENT_MANAGER (no Control Tower access) |
| TENANT_B | `91babc18-1fe0-4df3-8d2c-b350e6052b33` (test-tenant) | verification bootstrap user | PLATFORM_ADMIN for cross-tenant probe |

Credentials: repository dev defaults from `scripts/dev/seed_dev_admin.sh` / `scripts/dev/seed_demo_data.sh` only. **Not recorded in this report.**

---

## Acknowledge E2E (LOCAL/STAGING-SHADOW)

Primary event exercised: `c2083c916d4cc2af3e4ffda63ed36ecd`
Secondary fresh-unacknowledged event (first-run pass): `27e21d202cd5e72bd7a254351ba4350e`

| Check | Result |
|-------|--------|
| First acknowledge HTTP 200 | **PASS** |
| Response contains event id + acknowledgement + actor + timestamp | **PASS** |
| Summary enrichment after ack | **PASS** |
| Persistence visible on subsequent GET | **PASS** |
| Same-user repeat HTTP 200 | **PASS** |
| Original actor preserved (same user) | **PASS** |
| Original timestamp preserved (same user) | **PASS** |
| Second authorized user repeat HTTP 200 | **PASS** |
| Original actor preserved across users | **PASS** |
| Original timestamp preserved across users | **PASS** |

First ack actor: `8541a3a3-bde7-4fed-9501-37b9953bf904`
First ack time: `2026-08-13T20:47:28Z` (UTC)

Idempotency semantics: **first ack wins** — confirmed for same-user and cross-user repeats.

---

## Negative Authorization

| Check | Result |
|-------|--------|
| Unauthorized role (PROCUREMENT_MANAGER) → HTTP 403 | **PASS** |
| Acknowledgement unchanged after 403 | **PASS** |

Closes deferred item **QA-004 / CT-AA-004-002** at runtime (local environment).

---

## Unknown / Malformed Event

| Check | Result |
|-------|--------|
| Unknown valid-format event ID → HTTP 404 | **PASS** (`bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb`) |
| Malformed event ID → HTTP 400 | **PASS** (`not-valid`) |

---

## Tenant Isolation

| Check | Result |
|-------|--------|
| TENANT_B authorized user ack of TENANT_A event | HTTP **404** — **PASS** |
| Foreign event not disclosed | **PASS** |
| No TENANT_B persistence for foreign `event_id` | **PASS** (0 rows) |
| TENANT_A acknowledgement preserved | **PASS** (1 row) |

Cross-tenant probe required bootstrapping `test-tenant` user with PLATFORM_ADMIN (repository identity/company APIs). No production tenants touched.

---

## UI

| Check | Result |
|-------|--------|
| UI E2E (browser login, badge, reload) | **NOT_RUN** — no browser-capable staging UI session in agent environment |
| Demo mode mutation hidden | **NOT_RUN** — depends on UI E2E |

---

## Optional / Deferred

| Check | Result | Reason |
|-------|--------|--------|
| READ_MODEL_UNAVAILABLE 503 | **NOT_RUN** | No isolated fault injection; avoid disrupting shared local stack |
| Demo seed (`make seed-demo-data`) | **PARTIAL** | Existing demo entities present; bid creation warnings (HTTP 400) — non-blocking for ack verification |
| Actual Selectel staging E2E | **NOT_RUN** | Ops access / protected env not available |
| Manual QA-003 staging checklist | **NOT_RUN** | Same as above |

---

## Post-test regression

| Check | Result |
|-------|--------|
| POST_TEST_SMOKE | **PASS** |
| SUMMARY_STILL_HEALTHY | **PASS** |
| Control Tower summary loads after mutations | **PASS** |
| Acknowledgement persists on fresh GET | **PASS** |

---

## Security Findings Follow-up

From `SECURITY_REVIEW.md` (CT-AA-004):

| ID | Severity | Staging-shadow evidence | Severity change |
|----|----------|-------------------------|-----------------|
| CT-AA-004-001 | LOW | Read-model remains bound to `127.0.0.1:8089` in local compose; public ack only via gateway | **Unchanged** — deployment invariant still required for real staging/prod |
| CT-AA-004-002 | LOW | Runtime 403 negative test **PASS** (PROCUREMENT_MANAGER) | **Closed at verification level** — optional unit-test follow-up remains non-blocking |

---

## Final Verdict

**CONDITIONAL PASS**

All blocking acknowledge, idempotency, authorization, unknown-event, migration, and cross-tenant checks **passed** on **LOCAL/STAGING-SHADOW** at MAIN_SHA `601fdb9`.

Conditions for full **PASS**:

1. Repeat manual UI E2E on staging when ops access is available (QA-003).
2. Confirm migration 000020 + acknowledge flow on actual Selectel staging (`STAGING_EXECUTION_PENDING_OPS_ACCESS`).

**PRODUCT_CODE_MODIFIED=NO**
**PRODUCTION_MUTATION=NO**
**SECRETS_EXPOSED=NO**
