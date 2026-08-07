# Control Tower Staging — Operator Command Sheet

Placeholders:

- `<STAGING_PROJECT_PATH>` — e.g. `/protected/bintrans-staging`
- `<PROTECTED_ENV_FILE>` — e.g. `/protected/control-tower-observation/staging.env`
- `<IMAGE_TAG>` — `git-b75eb3d` (runtime image tag; reviewed feature source SHA)
- `<REPOSITORY_RELEASE_SHA>` — `64d218b6474d85075126cbf753fe73c1bbff94dd` (repository/runbook release after Feature + Observation merge)
- `<POSTGRES_CONTAINER>` — staging Postgres container name

`git-b75eb3d` identifies **runtime images**. `64d218b` identifies the **repository/runbook release** recorded in deployment evidence.

All commands assume **staging only**. Primary mode must remain disabled.

---

## 1. Preflight

```bash
# STAGING ONLY
# Confirm project and environment before executing.

cd <STAGING_PROJECT_PATH>
git rev-parse HEAD
git status --short
docker compose ps
docker compose config
docker images --digests | grep <IMAGE_TAG>
```

Database preflight (aggregated counts, no tenant IDs):

```bash
# STAGING ONLY
# Confirm project and environment before executing.

docker exec -i <POSTGRES_CONTAINER> psql -U <POSTGRES_USER> -d <POSTGRES_DB> <<'SQL'
SELECT version, dirty FROM schema_migrations;
SELECT COUNT(*) AS projection_rows FROM control_tower.shipment_status_projection;
SELECT COUNT(*) AS incomplete_rows FROM control_tower.shipment_status_projection WHERE is_complete = false;
SELECT COUNT(*) AS inbox_rows FROM control_tower.shipment_status_inbox;
SELECT COUNT(*) AS dead_letter_rows FROM control_tower.shipment_status_dead_letter;
SELECT state, COUNT(*) FROM control_tower.shipment_status_projection_rebuild_job GROUP BY state ORDER BY state;
SELECT COUNT(*) AS stage_rows FROM control_tower.shipment_status_projection_rebuild_stage;
SELECT COUNT(*) AS backup_rows FROM control_tower.shipment_status_projection_rebuild_backup;
SQL
```

Stop if jobs exist in `IMPORTING`, `ACTIVATING`, or `ROLLING_BACK`. Review `VALIDATED` and `ACTIVE` jobs.

---

## 2. Backup confirmation

Record before migration:

```text
backup timestamp: ___________
backup location: ___________ (redact in shared reports)
verification result: ___________
restore procedure available: yes/no
```

---

## 3. Pull/build images

```bash
# STAGING ONLY
# Confirm project and environment before executing.

docker pull <registry>/identity-service:<IMAGE_TAG>
docker pull <registry>/shipment-service:<IMAGE_TAG>
docker pull <registry>/control-tower-read-model-service:<IMAGE_TAG>
docker pull <registry>/api-gateway:<IMAGE_TAG>
docker images --digests | grep <IMAGE_TAG>
```

Record digests in deployment log.

---

## 4. Migrate

```bash
# STAGING ONLY
# Confirm project and environment before executing.

cd <STAGING_PROJECT_PATH>
make migrate-up
make migrate-version
```

Expected: `version=19`. Do **not** run down migrations on staging.

---

## 5. Compose config validation

```bash
# STAGING ONLY
# Confirm project and environment before executing.

docker compose config | grep -E 'READ_MODEL_MODE|CONSUMER_ENABLED|OUTBOX|primary'
```

Expected: `shadow`, consumer=true, outbox=true, **no primary**.

---

## 6. Deploy (service restart order)

```bash
# STAGING ONLY
# Confirm project and environment before executing.

cd <STAGING_PROJECT_PATH>
docker compose up -d identity-service
docker compose up -d shipment-service
docker compose up -d control-tower-read-model-service
docker compose up -d api-gateway
# Reload Prometheus/Grafana configs per staging setup
```

---

## 7. Health checks

```bash
# STAGING ONLY
# Confirm project and environment before executing.

curl -sf http://127.0.0.1:<IDENTITY_PORT>/health
curl -sf http://127.0.0.1:<SHIPMENT_PORT>/health
curl -sf http://127.0.0.1:<READ_MODEL_PORT>/health
curl -sf http://127.0.0.1:18080/health
```

Kafka consumer group (per-partition lag):

```bash
# STAGING ONLY
# Confirm project and environment before executing.

rpk group describe control-tower-shipment-status-v1
```

---

## 8. Cohort baseline (Day 0)

```bash
# STAGING ONLY
# Confirm project and environment before executing.

set -a
source <PROTECTED_ENV_FILE>
set +a

export COHORT_MANIFEST=/protected/control-tower-cohort.json
export OBSERVATION_OUTPUT_DIR=/protected/control-tower-observation

make control-tower-shadow-observation-preflight
make control-tower-shadow-observation-snapshot
```

---

## 9. Daily snapshot

```bash
# STAGING ONLY
# Confirm project and environment before executing.

set -a
source <PROTECTED_ENV_FILE>
set +a

make control-tower-shadow-observation-snapshot
```

Fill daily report: `docs/templates/CONTROL_TOWER_SHADOW_DAILY_REPORT_TEMPLATE.md`

---

## 10. Daily gate

```bash
# STAGING ONLY
# Confirm project and environment before executing.

set -a
source <PROTECTED_ENV_FILE>
set +a

make control-tower-shadow-observation-gate
```

Daily PASS requires: primary disabled, public source legacy, cohort MATCH, dead-letter delta=0, offset commit errors delta=0, lag recovered, no stuck jobs.

---

## 11. Rollback drill

```bash
# STAGING ONLY
# Confirm project and environment before executing.

make control-tower-shadow-observation-rollback-drill
```

Activation/rollback CLI commands require explicit confirmation variables — not included here. See `docs/CONTROL_TOWER_PROJECTION_REBUILD.md`.

---

## 12. Consumer restart drill

```bash
# STAGING ONLY
# Confirm project and environment before executing.

make control-tower-shadow-observation-consumer-restart-drill
```

---

## 13. Final gate (Day 7)

```bash
# STAGING ONLY
# Confirm project and environment before executing.

set -a
source <PROTECTED_ENV_FILE>
set +a

make control-tower-shadow-observation-gate
```

Complete sign-off templates before declaring observation PASS.

---

## Observation state file (optional, protected)

Collector may persist runtime state outside Git:

```text
/protected/control-tower-observation/state.json
```

Allowed fields: start timestamp, deployed SHA, cohort aliases, previous aggregate counters, observation day, last successful gate time.

**Never store:** JWT, tenant UUID, DB URL, Kafka payload, event IDs.
