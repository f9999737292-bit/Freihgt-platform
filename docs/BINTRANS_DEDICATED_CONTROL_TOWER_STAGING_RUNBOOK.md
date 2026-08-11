# BINTRANS Dedicated Control Tower Staging Runbook

> **Staging-only.** Do not execute against production or the old shared VPS (`gpt-docker` / `161.104.53.221`).

Operator repository checkout: `a1c246d0629e1cc8be3c0681064a31626f396273`  
Runtime image source: `b75eb3d` / tag `git-b75eb3d`  
Migration target: `000019` (schema version **19**)  
Control Tower mode: **shadow** — **PRIMARY MUST REMAIN DISABLED**

---

## Target environment

| Field | Value |
|-------|-------|
| VM | `bintrans-control-tower-staging` |
| Public IP | `161.104.57.152` |
| Private IP | `10.70.0.2` |
| Repository path | `/opt/bintrans/control-tower-staging` |
| Compose project | `bintrans-ct-staging` |
| Protected env | `/protected/bintrans/control-tower-observation/staging.env` |
| Cohort manifest | `/protected/bintrans/control-tower-cohort.json` |
| Registry | `cr.selcloud.ru/bintrans-staging` |

---

## Current expected state (not claimed executed by this document)

| Gate | Expected |
|------|----------|
| `DOCKER_BOOTSTRAP` | PASS |
| `REPOSITORY_CHECKOUT` | PASS (`a1c246d`) |
| `PROTECTED_ENV_CREATED` | PASS |
| `NETWORK_ISOLATION_OPERATOR_VERIFIED` | **YES** (operator-supplied; Cursor did not verify Selectel) |
| `COHORT_APPROVED` | **NO** |
| `FOUNDATION_STARTED` | **NO** |
| `BACKUP_VERIFIED` | **NO** |
| `MIGRATION_19_EXECUTED` | **NO** |
| `RUNTIME_DEPLOYED` | **NO** |
| `DAY_0_STARTED` | **NO** |
| `OBSERVATION_WINDOW_STARTED` | **NO** |
| `PRIMARY_ENABLED` | **NO** |

### Network isolation (operator-supplied evidence)

Operator reports for dedicated VM `161.104.57.152`:

- Security group `bintrans-ct-staging-sg` allows inbound **TCP/22 only** from trusted admin source
- Direct-path tests showed internal service ports closed externally
- Earlier apparent wide-open port scans were caused by local HAPP TUN/VPN path, not the VM SG
- SSH works through the trusted path

**Cursor did NOT independently access Selectel.** Static compose isolation (`127.0.0.1` / no publish) is separate from this operator network evidence.

---

## Compose pack

Foundation + isolation (always):

```text
-f infrastructure/docker-compose/docker-compose.yml
-f infrastructure/docker-compose/docker-compose.bintrans-ct-staging.yml
--env-file /protected/bintrans/control-tower-observation/staging.env
-p bintrans-ct-staging
```

Shadow runtime (later, after migration approval):

```text
-f infrastructure/docker-compose/docker-compose.staging-shadow.yml
-f infrastructure/docker-compose/docker-compose.bintrans-ct-staging-images.yml
--profile read-model
```

Observability (optional, later):

```text
--profile observability
```

### Host port policy (after `docker-compose.bintrans-ct-staging.yml`)

| Service | Host publish | Reason |
|---------|--------------|--------|
| postgres | **none** | DB access via `docker exec` only |
| redpanda | **none** | Kafka access via internal network / `docker exec` |
| identity … low-code | **none** | Internal only; external via gateway |
| control-tower-read-model | `127.0.0.1:8089` | Operator diagnostics on VM localhost |
| api-gateway | `127.0.0.1:18080` | Staging API (`GATEWAY_URL`) on VM localhost |
| prometheus | `127.0.0.1:9090` | Operator metrics on VM localhost |
| grafana | `127.0.0.1:3001` | Operator dashboards on VM localhost |

Do **not** rely on UFW alone — Docker publish rules must match this table.

---

## PHASE A — Static preflight (NOT executed automatically)

From repository root on VM:

```bash
chmod +x scripts/ops/bintrans_ct_staging/*.sh
./scripts/ops/bintrans_ct_staging/bintrans_ct_staging_preflight.sh
```

Validates:

- migration `000019` files exist
- protected env shadow / consumer / outbox / auth flags
- primary mode absent
- compose config renders
- dangerous wide host binds flagged
- empty / unapproved cohort blocked for Day 0

Does **not** start containers or run migrations.

---

## PHASE B — Foundation only (NOT executed yet)

```bash
./scripts/ops/bintrans_ct_staging/bintrans_ct_staging_foundation_up.sh
```

Starts **only**:

- `postgres`
- `redpanda`

Does **not** start: api-gateway, application services, control-tower, prometheus, grafana, migrate.

Equivalent explicit command (includes `--no-deps` so dependent services are never pulled up):

```bash
docker compose \
  --env-file /protected/bintrans/control-tower-observation/staging.env \
  -p bintrans-ct-staging \
  -f infrastructure/docker-compose/docker-compose.yml \
  -f infrastructure/docker-compose/docker-compose.bintrans-ct-staging.yml \
  --profile messaging \
  up -d --no-deps postgres redpanda
```

Static contract check (repository-only):

```bash
./scripts/ops/bintrans_ct_staging/bintrans_ct_staging_foundation_up_selfcheck.sh
```

---

## PHASE C — Foundation health (NOT executed yet)

```bash
./scripts/ops/bintrans_ct_staging/bintrans_ct_staging_foundation_health.sh
```

Checks:

- `docker compose ps`
- `pg_isready` (uses `POSTGRES_USER` / `POSTGRES_DB` from protected env)
- `rpk cluster info --brokers localhost:9092` inside redpanda container

---

## PHASE D — Backup (NOT executed yet)

Hard gate: **`BACKUP_VERIFIED=YES`** required before migration.

```bash
./scripts/ops/bintrans_ct_staging/bintrans_ct_staging_backup.sh
```

- Uses `pg_dump -Fc` via `docker exec`
- Never prints `POSTGRES_PASSWORD`
- Writes timestamped file under `/protected/bintrans/backups/` (override with `BINTRANS_BACKUP_DIR`)
- Verifies non-empty custom format + `pg_restore -l`
- Records SHA-256 checksum

Operator must manually set in protected env **in this order**:

1. `BACKUP_PATH=<path from script output>`
2. `BACKUP_SHA256=<checksum from script output>`
3. After manual verification: `BACKUP_VERIFIED=YES`

Never set `BACKUP_VERIFIED=YES` before recording path and checksum.

---

## PHASE E — Migration approval gate (NOT executed yet)

Prerequisites:

1. Foundation healthy
2. `BACKUP_VERIFIED=YES`
3. `MIGRATION_TARGET=000019` in protected env
4. Explicit operator approval

Gate-only (shows version, does **not** migrate):

```bash
./scripts/ops/bintrans_ct_staging/bintrans_ct_staging_migrate_gate.sh
```

Gate checks before execution:

1. `MIGRATION_TARGET=000019`
2. migration `000019` files exist in repository checkout
3. PostgreSQL container running + `pg_isready`
4. `BACKUP_VERIFIED=YES`
5. `migrate version` parsed (rejects unknown/dirty)
6. rejects current version > 19
7. if current version == 19 → `ALREADY_AT_TARGET=YES`, no migration
8. requires `CONFIRM_MIGRATION_000019=true` to apply

`golang-migrate` CLI behavior used by this repository:

- `version` prints `N` on success, or `N (dirty)` when dirty (rejected)
- no migrations yet → error containing `no migration` (treated as version 0)
- apply uses explicit `goto 19`, **not** unbounded `up`

---

## PHASE F — Migration 000019 (NOT executed yet)

**Do not use unbounded `make migrate-up`** for this staging path. Use explicit target:

```bash
CONFIRM_MIGRATION_000019=true \
  ./scripts/ops/bintrans_ct_staging/bintrans_ct_staging_migrate_gate.sh
```

Underlying tooling: `golang-migrate` **`goto 19`** (not `up` to latest).

Post-migration verification:

```bash
docker compose ... --profile tools run --rm migrate \
  -path=/migrations -database '<DATABASE_URL>' version
# expect: 19
```

Migration `000019` alters `control_tower.shipment_status_projection_rebuild_backup.last_event_type` to nullable.

**Do not run down migrations on staging.**

---

## PHASE G — Registry digest verification (NOT executed yet)

1. Login: `docker login cr.selcloud.ru` (operator supplies pull token out-of-band)
2. Pull tag baseline: `git-b75eb3d` per service
3. Inspect digest: `docker inspect --format='{{index .RepoDigests 0}}' <image>`
4. Copy digest-pinned lines from `scripts/ops/bintrans_ct_staging/registry.images.template.env` into protected env

Service → registry path (tag baseline):

| Service | Image reference |
|---------|-----------------|
| identity-service | `cr.selcloud.ru/bintrans-staging/identity-service:git-b75eb3d` |
| company-service | `cr.selcloud.ru/bintrans-staging/company-service:git-b75eb3d` |
| transport-order-service | `cr.selcloud.ru/bintrans-staging/transport-order-service:git-b75eb3d` |
| rfx-service | `cr.selcloud.ru/bintrans-staging/rfx-service:git-b75eb3d` |
| shipment-service | `cr.selcloud.ru/bintrans-staging/shipment-service:git-b75eb3d` |
| document-service | `cr.selcloud.ru/bintrans-staging/document-service:git-b75eb3d` |
| billing-register-service | `cr.selcloud.ru/bintrans-staging/billing-register-service:git-b75eb3d` |
| low-code-service | `cr.selcloud.ru/bintrans-staging/low-code-service:git-b75eb3d` |
| control-tower-read-model-service | `cr.selcloud.ru/bintrans-staging/control-tower-read-model-service:git-b75eb3d` |
| api-gateway | `cr.selcloud.ru/bintrans-staging/api-gateway:git-b75eb3d` |

`localization-service` is **not** in the runtime set.

---

## PHASE H — Runtime shadow deployment (NOT executed yet)

After migration version 19 + digest-pinned images:

```bash
docker compose \
  --env-file /protected/bintrans/control-tower-observation/staging.env \
  -p bintrans-ct-staging \
  -f infrastructure/docker-compose/docker-compose.yml \
  -f infrastructure/docker-compose/docker-compose.bintrans-ct-staging.yml \
  -f infrastructure/docker-compose/docker-compose.staging-shadow.yml \
  -f infrastructure/docker-compose/docker-compose.bintrans-ct-staging-images.yml \
  --profile messaging --profile read-model \
  up -d \
  identity-service company-service transport-order-service rfx-service \
  shipment-service document-service billing-register-service low-code-service \
  control-tower-read-model-service api-gateway
```

Restart order recommendation: identity → company → transport-order → rfx → shipment → document → billing → low-code → read-model → gateway.

### Shadow safety semantics (source proof)

| Mode | Public `StatusSummary.Source` | Behavior |
|------|------------------------------|----------|
| `shadow` | **LEGACY** | Read-model fetched for comparison only (`merge.go` `ModeShadow`) |
| `primary` | **READ_MODEL** (with legacy fallback) | Read-model becomes public source (`mergePrimary`) |

Preflight validates **effective** `api-gateway` `CONTROL_TOWER_READ_MODEL_MODE: shadow` in rendered compose — not a generic text search for the word `primary`.

Optional observability (requires api-gateway healthy first — do not use during foundation-only):

```bash
docker compose ... --profile observability up -d prometheus grafana
```

---

## PHASE I — Cohort approval (NOT ready)

**`COHORT_APPROVED=NO`**

- Do **not** create fake tenant IDs
- Empty file or `"tenants": []` → loader error `cohort manifest is empty` (`cohort.go` `validateCohortEntries`)
- No approved tenants after filtering → `cohort manifest has no approved tenants`

Until real approved tenants are supplied, Day 0 is blocked.

---

## PHASE J — Day 0 observation (NOT started)

Requires: runtime healthy, migration 19, approved cohort, observation tooling from operator checkout.

See also (on `a1c246d`):

- `docs/CONTROL_TOWER_STAGING_SHADOW_OBSERVATION.md`
- `docs/CONTROL_TOWER_STAGING_OPERATOR_COMMANDS.md`
- `scripts/ops/control_tower_shadow_observation/staging.env.example`

---

## Related scripts

| Script | Purpose |
|--------|---------|
| `bintrans_ct_staging_preflight.sh` | Static validation |
| `bintrans_ct_staging_foundation_up.sh` | Start postgres + redpanda |
| `bintrans_ct_staging_foundation_health.sh` | Foundation health |
| `bintrans_ct_staging_backup.sh` | pg_dump backup |
| `bintrans_ct_staging_foundation_up_selfcheck.sh` | Foundation script static contract |
| `staging.env.example` | Protected env template |
| `bintrans_ct_staging_migrate_gate.sh` | Migration 000019 gate |
| `registry.images.template.env` | Digest pinning template |

## Explicitly forbidden in this runbook path

- Production / old shared VPS changes
- `make migrate-up` without gate (unbounded up)
- Enabling `CONTROL_TOWER_READ_MODEL_MODE=primary`
- Kafka offset reset / topic delete / group delete
- Fake cohort tenants
- Committing secrets to Git
