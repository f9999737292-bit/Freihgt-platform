# BINTRANS Dedicated Control Tower Staging Runbook

> **Staging-only.** Do not execute against production or the old shared VPS (`gpt-docker` / `161.104.53.221`).

Operator repository checkout: `a1c246d0629e1cc8be3c0681064a31626f396273`  
Runtime image source: `b75eb3d` / tag `git-b75eb3d`  
Migration target: `000019` (schema version **19**)  
**Fresh DB note:** target version **19** is not “one migration only”. On a database with no `schema_migrations` table (version 0), `goto 19` applies the **full UP chain** `000001` … `000019` (up to 19 migrations).  
Control Tower mode: **shadow** — **PRIMARY MUST REMAIN DISABLED**

---

## Operator-supplied verified state (staging VM)

> Cursor did **not** independently re-verify the live database in repository-only hardening tasks.  
> The following reflects operator-supplied evidence from the dedicated staging VM.

| Gate | State |
|------|-------|
| FOUNDATION_STARTED | YES |
| FOUNDATION_HEALTHY | YES |
| BACKUP_VERIFIED | YES |
| MIGRATION_19_EXECUTED | YES (chain 000001–000019 on fresh DB) |
| SCHEMA_VERSION | 19 |
| SCHEMA_DIRTY | false |
| RUNTIME_DEPLOYED | NO |
| CONTROL_TOWER_STARTED | NO |
| COHORT_APPROVED | NO |
| DAY_0_STARTED | NO |
| PRIMARY_ENABLED | NO |

**Migration tooling note:** Migration chain **000001–000019** applied successfully (schema version **19**, dirty **false**). An earlier migrate gate exit code 1 was caused by **parser verification logic** parsing Docker Compose lifecycle noise — **not** a failed DB migration. **Do not rerun migration.**

**Next gates before runtime:** JWT_SECRET in protected env, digest-pinned `BINTRANS_*_IMAGE` refs, `./scripts/ops/bintrans_ct_staging/bintrans_ct_staging_runtime_preflight.sh`.

---

## Future live operator sequence (after Git push + operator review)

1. Operator reviews and pushes `ops/bintrans-ct-staging-pack` branch
2. VM checkout updated to reviewed commit
3. Provision real `JWT_SECRET` in protected env
4. Re-run runtime preflight (digest gate may skip until step 8)
5. `docker login cr.selcloud.ru` (operator credentials)
6. Publish 10 runtime images with tag `git-b75eb3d` (`bintrans_ct_staging_registry_publish.sh` prepare-only)
7. Retrieve canonical registry digests
8. Populate protected `BINTRANS_*_IMAGE=@sha256:...` entries
9. `bintrans_ct_staging_runtime_images_validate.sh` PASS
10. Final `bintrans_ct_staging_runtime_preflight.sh` PASS
11. `bintrans_ct_staging_runtime_up.sh`
12. `bintrans_ct_staging_runtime_health.sh` PASS
13. Authentication smoke (see `docs/BINTRANS_STAGING_AUTH_SMOKE.md`) — **OPERATOR_DATA_REQUIRED**
14. Control Tower shadow smoke (see `docs/BINTRANS_STAGING_SHADOW_SMOKE.md`)
15. `bintrans_ct_staging_observability_up.sh`
16. `bintrans_ct_staging_observability_health.sh` PASS
17. Real approved cohort manifest; `COHORT_APPROVED=YES`
18. Day 0 pre-checks (schema 19, shadow, consumer, outbox, primary disabled)
19. Observation window (`scripts/ops/control_tower_shadow_observation/`)

**Never enable primary mode in this sequence.**

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

## Current expected state (operator-supplied; autonomous task did NOT SSH)

| Gate | State |
|------|-------|
| `DOCKER_BOOTSTRAP` | PASS |
| `NETWORK_ISOLATION_OPERATOR_VERIFIED` | YES |
| `FOUNDATION_STARTED` | YES |
| `FOUNDATION_HEALTHY` | YES |
| `BACKUP_VERIFIED` | YES |
| `MIGRATION_19_EXECUTED` | YES |
| `SCHEMA_VERSION` | 19 |
| `SCHEMA_DIRTY` | false |
| `RUNTIME_DEPLOYED` | NO |
| `CONTROL_TOWER_STARTED` | NO |
| `OBSERVABILITY_STARTED` | NO |
| `COHORT_APPROVED` | NO |
| `DAY_0_STARTED` | NO |
| `PRIMARY_ENABLED` | NO |

Verified backup (operator): `/protected/bintrans/backups/freight_platform_20260811T083942Z.dump`  
SHA-256: `c04d993fedc70b9627b773a367f0a62872fd6feed6ccce7990793bd7e66c6c9b`

**Do not rerun migration.** Next live gates: JWT_SECRET provisioning, registry push, digest pinning, runtime preflight, runtime start.

---

## Operator sequence (post-migration → Day 0)

| Phase | Action |
|-------|--------|
| **K** | Update staging-pack commit on VM (`git fetch` + checkout latest `ops/bintrans-ct-staging-pack`) |
| **L** | Provision strong `JWT_SECRET` in protected env (see `docs/BINTRANS_STAGING_JWT_AUDIT.md`) |
| **M** | Registry login (`docker login cr.selcloud.ru`) — operator credentials only |
| **N** | Push runtime images — `bintrans_ct_staging_registry_publish.sh` (prepare) + manual push |
| **O** | Capture digests; validate with `bintrans_ct_staging_runtime_images_validate.sh` |
| **P** | Write digest-pinned `BINTRANS_*_IMAGE=` lines to protected env (template: `runtime.images.digest.env.example`) |
| **Q** | `./scripts/ops/bintrans_ct_staging/bintrans_ct_staging_runtime_preflight.sh` |
| **R** | `./scripts/ops/bintrans_ct_staging/bintrans_ct_staging_runtime_up.sh` |
| **S** | `./scripts/ops/bintrans_ct_staging/bintrans_ct_staging_runtime_health.sh` |
| **T** | `./scripts/ops/bintrans_ct_staging/bintrans_ct_staging_observability_up.sh` |
| **T2** | `./scripts/ops/bintrans_ct_staging/bintrans_ct_staging_observability_health.sh` |
| **U** | Operator-approved cohort at `/protected/bintrans/control-tower-cohort.json`; set `COHORT_APPROVED=YES` |
| **V** | Day 0 shadow observation (`scripts/ops/control_tower_shadow_observation/`) |

---

## Runtime smoke gate design (first deploy)

Read-only / minimally mutating sequence for operator after Phase R:

| Category | Check | Tool / method |
|----------|-------|---------------|
| **A. Infrastructure** | Exact runtime service set; no migrate/prometheus/grafana | `bintrans_ct_staging_runtime_health.sh` |
| **B. Authentication** | identity-service + gateway up; JWT issuance requires valid credentials | Manual login once cohort exists; not required for container health |
| **C. Service connectivity** | Gateway `/health`; internal routing | `curl` localhost gateway |
| **D. Database** | postgres `pg_isready` | `bintrans_ct_staging_runtime_health.sh` |
| **E. Kafka** | redpanda cluster info | `bintrans_ct_staging_runtime_health.sh` |
| **F. Control Tower shadow** | Effective mode=shadow; consumer enabled | runtime preflight + gateway env |

**Blocked until cohort:** functional tenant-scoped API requests, Day 0 observation, comparison metrics requiring approved tenants.

**Empty cohort rejected:** `cohort manifest is empty` / `cohort manifest has no approved tenants` (`cohort.go`).

---

## PHASE G — Registry digest verification

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
- on version 0, `goto 19` runs every applicable UP migration through version 19 and creates `schema_migrations` automatically
- **target migration version = 19** is distinct from **number of UP files applied from fresh DB = up to 19**

---

## PHASE F — Migration to version 19 (NOT executed yet)

**Do not use unbounded `make migrate-up`** for this staging path. Use explicit target:

```bash
CONFIRM_MIGRATION_000019=true \
  ./scripts/ops/bintrans_ct_staging/bintrans_ct_staging_migrate_gate.sh
```

Underlying tooling: `golang-migrate` **`goto 19`** (not `up` to latest).

**Fresh staging database (current state: version 0 / no `schema_migrations`):**  
`goto 19` applies migrations **000001 through 000019** in order. The final schema version is **19**. Migration **000019** alone is only the last step in that chain.

**Already-migrated database:** if current version is already 19, gate exits with `ALREADY_AT_TARGET=YES` and applies nothing.

Post-migration verification:

```bash
docker compose ... --profile tools run --rm migrate \
  -path=/migrations -database '<DATABASE_URL>' version
# expect: 19
```

Migration **000019** (final step) alters `control_tower.shipment_status_projection_rebuild_backup.last_event_type` to nullable.

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

Use wrapper (runs preflight first):

```bash
./scripts/ops/bintrans_ct_staging/bintrans_ct_staging_runtime_up.sh
```

Static contract:

```bash
./scripts/ops/bintrans_ct_staging/bintrans_ct_staging_runtime_up_selfcheck.sh
```

Health after start:

```bash
./scripts/ops/bintrans_ct_staging/bintrans_ct_staging_runtime_health.sh
```

Prerequisites beyond foundation/migration:

1. `JWT_SECRET` set in protected env (non-placeholder; externalized via `docker-compose.bintrans-ct-staging.yml`)
2. Digest-pinned `BINTRANS_*_IMAGE` for all 10 runtime services
3. `./scripts/ops/bintrans_ct_staging/bintrans_ct_staging_runtime_preflight.sh` PASS

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

Optional observability (separate script — requires api-gateway healthy first):

```bash
./scripts/ops/bintrans_ct_staging/bintrans_ct_staging_observability_up.sh
./scripts/ops/bintrans_ct_staging/bintrans_ct_staging_observability_up_selfcheck.sh
```

Manual equivalent:

```bash
docker compose ... --profile observability up -d --no-build prometheus grafana
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
| `bintrans_ct_staging_preflight.sh` | Static validation (foundation + compose; no JWT required) |
| `bintrans_ct_staging_runtime_preflight.sh` | Runtime deploy gate (JWT + digest images + shadow mode) |
| `bintrans_ct_staging_runtime_up.sh` | Start 10 runtime services (`--no-build`, excludes migrate/observability) |
| `bintrans_ct_staging_runtime_up_selfcheck.sh` | Runtime wrapper static contract |
| `bintrans_ct_staging_runtime_health.sh` | Post-start health (read-only) |
| `bintrans_ct_staging_observability_up.sh` | Start prometheus + grafana only |
| `bintrans_ct_staging_observability_up_selfcheck.sh` | Observability wrapper static contract |
| `bintrans_ct_staging_migrate_version_parser_selfcheck.sh` | Parser regression (no DB) |
| `bintrans_ct_staging_migration_parser_selfcheck.sh` | Alias for parser selfcheck |
| `bintrans_ct_staging_runtime_preflight_selfcheck.sh` | Runtime preflight regression (no DB) |
| `bintrans_ct_staging_runtime_images_validate.sh` | Canonical digest validator (10 services + repo name match) |
| `bintrans_ct_staging_runtime_images_validate_selfcheck.sh` | Digest validator regression |
| `bintrans_ct_staging_registry_digest_validate.sh` | Alias for runtime_images_validate |
| `bintrans_ct_staging_registry_publish.sh` | Registry publish prepare (no login/push) |
| `bintrans_ct_staging_registry_publish_guide.sh` | Operator registry push steps (text) |
| `bintrans_ct_staging_image_provenance_check.sh` | Local publish-tag presence check |
| `bintrans_ct_staging_observability_health.sh` | Observability health (read-only) |
| `bintrans_ct_staging_foundation_up.sh` | Start postgres + redpanda |
| `bintrans_ct_staging_foundation_health.sh` | Foundation health |
| `bintrans_ct_staging_backup.sh` | pg_dump backup |
| `bintrans_ct_staging_foundation_up_selfcheck.sh` | Foundation script static contract |
| `staging.env.example` | Protected env template |
| `runtime.images.digest.env.example` | Digest-pinned image template |
| `bintrans_ct_staging_migrate_gate.sh` | Migration to version 19 gate |
| `registry.images.template.env` | Legacy digest pinning template |
| `docs/BINTRANS_STAGING_JWT_AUDIT.md` | JWT trace documentation |
| `docs/BINTRANS_STAGING_AUTH_SMOKE.md` | Authentication smoke design |
| `docs/BINTRANS_STAGING_SHADOW_SMOKE.md` | Control Tower shadow smoke design |
| `docs/BINTRANS_STAGING_IMAGE_PROVENANCE.md` | Image provenance limitations |

## Explicitly forbidden in this runbook path

- Production / old shared VPS changes
- `make migrate-up` without gate (unbounded up)
- Enabling `CONTROL_TOWER_READ_MODEL_MODE=primary`
- Kafka offset reset / topic delete / group delete
- Fake cohort tenants
- Committing secrets to Git
