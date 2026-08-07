# Control Tower — Selectel Staging Deployment Handoff

> **This procedure changes staging only.**
> **It must not be executed against production.**
> **Primary mode must remain disabled.**

Handoff for an operator with SSH access to the Selectel VPS running `bintrans-staging`.

**Current status:** `STAGING_EXECUTION_PENDING_OPS_ACCESS`

---

## 1. Deployment prerequisites

Confirm access and artifacts before starting:

| Prerequisite | Notes |
|--------------|-------|
| SSH/VPS access | Selectel staging host |
| Repository access | Clone/pull `Freihgt-platform` |
| Container registry access | Pull images tagged `git-b75eb3d` |
| Protected staging `.env` | Outside Git; never commit |
| Database backup procedure | Verified backup + restore path |
| Migration binary | Via `make migrate-up` (Docker migrate image) |
| Docker Compose | Staging stack orchestration |
| Kafka/Redpanda admin | `rpk group describe` for consumer group |
| Prometheus access | Scrape and query endpoints |
| Grafana access | Dashboard reload after deploy |
| Staging admin credentials | Protected secrets store |
| Approved cohort manifest | `/protected/control-tower-cohort.json` |

Do **not** record secret values in this document or in Git.

---

## 2. Release reference

| Field | Value |
|-------|-------|
| Feature review SHA | `b75eb3d` |
| Migration target | `000019` |
| Expected migration version | **19** |
| Mode | `shadow` |
| Primary | **disabled** |
| Manifest | `docs/releases/CONTROL_TOWER_SHADOW_OBSERVATION_V0.6_RELEASE_MANIFEST.md` |
| Env template | `scripts/ops/control_tower_shadow_observation/staging.env.example` |

Image policy: tag with Git SHA; record digest; **no unverified `latest`**.

### Release identity

| Field | Value |
|-------|-------|
| Repository release SHA | `64d218b6474d85075126cbf753fe73c1bbff94dd` |
| Runtime image source SHA | `b75eb3de751002da94a3c271fda30d09be1db450` |
| Observation tooling source SHA | `9da601044f8bd5248a5fd04fb8dc7e2652e6415e` |

**Repository release SHA** — final `main` after controlled merge of Feature PR #1 and Observation PR #2.

**Runtime image SHA** — reviewed feature code from which staging runtime images must be built as `git-b75eb3d`. Do **not** retag runtime images to `git-64d218b` unless images with that tag were built and verified separately.

**Observation tooling** is included in the repository release on `main` and does **not** change the runtime image source SHA.

---

## 3. Preflight commands

```bash
# STAGING ONLY
# Confirm project and environment before executing.

cd <STAGING_PROJECT_PATH>   # e.g. /protected/bintrans-staging

git rev-parse HEAD
git status --short

docker compose ps
docker compose config
docker images --digests | grep -E 'identity|shipment|read-model|gateway|b75eb3d'
```

Verify runtime intent in compose config:

```text
CONTROL_TOWER_READ_MODEL_MODE=shadow
CONTROL_TOWER_CONSUMER_ENABLED=true
SHIPMENT_OUTBOX_ENABLED=true
```

If `CONTROL_TOWER_READ_MODEL_MODE=primary` appears anywhere, **stop deployment**.

---

## 4. Database preflight (aggregated)

Run against staging PostgreSQL. Output counts only — **do not log tenant IDs**.

```bash
# STAGING ONLY
# Confirm project and environment before executing.

docker exec -i <POSTGRES_CONTAINER> psql -U <POSTGRES_USER> -d <POSTGRES_DB> <<'SQL'
SELECT version, dirty FROM schema_migrations;

SELECT COUNT(*) AS projection_rows
FROM control_tower.shipment_status_projection;

SELECT COUNT(*) AS incomplete_rows
FROM control_tower.shipment_status_projection
WHERE is_complete = false;

SELECT COUNT(*) AS inbox_rows
FROM control_tower.shipment_status_inbox;

SELECT COUNT(*) AS dead_letter_rows
FROM control_tower.shipment_status_dead_letter;

SELECT state, COUNT(*) AS job_count
FROM control_tower.shipment_status_projection_rebuild_job
GROUP BY state
ORDER BY state;

SELECT COUNT(*) AS stage_rows
FROM control_tower.shipment_status_projection_rebuild_stage;

SELECT COUNT(*) AS backup_rows
FROM control_tower.shipment_status_projection_rebuild_backup;
SQL
```

Record:

- current migration version
- projection / incomplete / inbox / dead-letter row counts
- jobs grouped by state
- stage and backup row counts

---

## 5. Unfinished rebuild jobs gate

**Stop deployment** if any jobs exist in:

```text
IMPORTING
ACTIVATING
ROLLING_BACK
```

Also review `VALIDATED` and `ACTIVE` jobs before proceeding.

**Operator rule:** Do not automatically change a stuck job state. Stop deployment and perform manual investigation.

---

## 6. Database backup gate

Before `migrate up`, confirm and record:

| Field | Recorded |
|-------|----------|
| Backup timestamp | |
| Backup location | (redact sensitive path in reports) |
| Backup verification result | |
| Restore procedure available | yes/no |

Deployment is **forbidden** without a verified backup/restore process.

---

## 7. Migration procedure

Target version: **000019**

| Order | Migration | Purpose |
|-------|-----------|---------|
| 1 | `000016` | Rebuild core |
| 2 | `000017` | Activation |
| 3 | `000018` | Nullable `last_event_type` on projection/stage |
| 4 | `000019` | Nullable `last_event_type` on rebuild backup |

```bash
# STAGING ONLY
# Confirm project and environment before executing.

cd <STAGING_PROJECT_PATH>
make migrate-up
make migrate-version   # expect version=19
```

Post-migration verification:

```text
migration version = 19
projection row count unchanged
inbox row count unchanged
dead-letter row count unchanged
existing job count unchanged (except expected migration metadata)
```

**Do not run down migrations on staging.**

---

## 8. Image deployment gate

Before restart:

```bash
# STAGING ONLY
# Confirm project and environment before executing.

docker compose config
```

Verify:

```text
image Git SHA = b75eb3d
image digest recorded
no image uses unverified latest
mode = shadow
consumer = true
outbox = true
primary absent
```

If `primary` mode is detected, **stop deployment**.

---

## 9. Service restart order

Safe restart sequence (adjust service names to match staging Compose):

1. PostgreSQL migration (already applied)
2. Identity service
3. Shipment service
4. Read-model service
5. API Gateway
6. Prometheus configuration reload
7. Grafana / dashboard reload

Do **not** perform destructive Kafka/Redpanda restart unless required.

---

## 10. Smoke checklist

After deployment:

| Check | Expected |
|-------|----------|
| Identity health | PASS |
| Shipment health | PASS |
| Read-model health | PASS |
| Gateway health | PASS |
| Kafka connectivity | PASS |
| Consumer running | PASS |
| Outbox publisher running | PASS |
| Prometheus scrape | PASS |
| Primary absent | PASS |

Review logs for:

```text
migration error
unknown status
consumer poll error
offset commit error
dead-letter
projection DB error
lock timeout
panic
```

---

## 11. Cohort baseline

1. Deploy approved cohort manifest to `/protected/control-tower-cohort.json`.
2. Minimum recommended: **12 tenants** across categories (see runbook).
3. Resolve tenant IDs from protected env secret refs — **not** from Git.
4. Run baseline snapshot (Day 0).

See `scripts/ops/cohort.manifest.example.json` and `docs/CONTROL_TOWER_STAGING_SHADOW_OBSERVATION.md`.

---

## 12. Observation window

Starts only after:

```text
deployment healthy
migration version = 19
cohort manifest approved
baseline snapshot completed
```

- **Day 0** — deployment baseline
- **Days 1–6** — daily snapshot + gate
- **Day 7** — final gate

Minimum 5 business days within the 7-day window.

Daily report: `docs/templates/CONTROL_TOWER_SHADOW_DAILY_REPORT_TEMPLATE.md`

---

## 13. Backup retention

- **ACTIVE** backup: retain through observation window + 7 days after activation
- **FAILED / CANCELLED / ROLLED_BACK**: cleanup only after review and explicit confirmation
- No automatic cleanup scheduler

---

## 14. Sign-off requirements

Before declaring observation complete:

| Sign-off | Template |
|----------|----------|
| Security | `docs/templates/CONTROL_TOWER_SHADOW_SECURITY_REVIEW.md` |
| DBA | `docs/templates/CONTROL_TOWER_SHADOW_DBA_REVIEW.md` |
| Ops | `docs/templates/CONTROL_TOWER_SHADOW_OPS_APPROVAL.md` |
| SLO | `docs/templates/CONTROL_TOWER_SHADOW_SLO_APPROVAL.md` |

---

## 15. Operator command reference

Full command sheet: `docs/CONTROL_TOWER_STAGING_OPERATOR_COMMANDS.md`

PR handoff: `docs/CONTROL_TOWER_PROJECTION_REBUILD_PR_HANDOFF.md`

---

## 16. Blocked actions

This handoff does **not** authorize:

- Production deployment
- Enabling `primary` mode
- Kafka offset reset
- Automatic activation or rollback
- Merging to `main` without PR review

**Classification:** `STAGING_EXECUTION_PENDING_OPS_ACCESS` until operator completes deployment on Selectel VPS.
