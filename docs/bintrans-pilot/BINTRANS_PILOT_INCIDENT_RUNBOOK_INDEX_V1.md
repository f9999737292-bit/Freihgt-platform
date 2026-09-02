# BINTRANS Pilot Incident Runbook Index v1

**Status:** READY (canonical navigation index)  
**Scope:** Controlled operator-assisted pilot on BINTRANS CT staging (`161.104.57.152`)  
**Escalation policy:** `docs/bintrans-pilot/BINTRANS_PILOT_MANAGEMENT_APPROVAL_PACK_V1.md` §5–6 (PROPOSED)

Use this index to reach the correct procedure. Detailed steps remain in linked runbooks.

---

## Quick reference

| Scenario | Detection | First checks | Owner | Procedure |
|---|---|---|---|---|
| [Service outage](#service-outage) | Alert `BintransPilotServiceDown`, health scripts, user reports | `bintrans_ct_staging_runtime_health.sh`, `docker ps`, Prometheus targets | PILOT_OPERATIONS_OWNER | See links below |
| [Auth / login failure](#auth-login) | 401/403 surge, login smoke fail | JWT classification in protected env, identity-service health | SECURITY_OWNER / PILOT_OPERATIONS_OWNER | See links |
| [RFx unavailable](#rfx) | RFx API errors, rfx-service down | `freight_rfx_service` health, gateway `/health` | PILOT_OPERATIONS_OWNER | See links |
| [Shipment execution failure](#shipment) | Shipment status stuck, assignment errors | shipment-service logs, event timeline API | PILOT_OPERATIONS_OWNER | See links |
| [Database unavailable](#database-unavailable) | `BintransPilotPostgresUnavailable`, app DB errors | postgres container, `pg_isready`, pool metrics | DATABASE_OWNER | See links |
| [Control Tower unavailable](#control-tower-unavailable) | CT summary 5xx, read-model errors | CT service health, shadow mode flag | PILOT_OPERATIONS_OWNER | See links |
| [Staging host unavailable](#host-unavailable) | SSH failure, all services down | Selectel panel, host uptime, disk | INFRASTRUCTURE_OWNER | See links |
| [Backup failure](#backup-failure) | Backup script fail, stale backup alert | `/protected/bintrans/backups/`, `BACKUP_*` env | DATABASE_OWNER / PILOT_OPERATIONS_OWNER | See links |
| [Security incident](#security-incident) | Tenant leak suspicion, auth bypass | Freeze pilot, capture sanitized evidence | SECURITY_OWNER + GO_LIVE_AUTHORITY | See links |
| [Telegram alert delivery](#telegram-alert) | Alertmanager notification failures | AM metrics, `extra_hosts`, Bot API reachability | PILOT_OPERATIONS_OWNER | OPS-BLK-001 closed |

---

## Service outage {#service-outage}

**Detection:** Prometheus alerts `BintransPilotServiceDown`, `BintransPilotTargetFlapping`, `BintransPilotHttp5xxSurge`; Grafana services-health dashboard.

**First checks:**
1. `docker ps --filter health=unhealthy`
2. `scripts/ops/bintrans_ct_staging/bintrans_ct_staging_runtime_health.sh`
3. Prometheus → Status → Targets

**Containment:** Do not destructive-reset DB. Recreate affected service from known digest/snapshot only.

**Rollback:** Use `/protected/bintrans/deploy-snapshots/` and wave2r7 digest manifest.

**Escalation:** P1 → P1_INCIDENT_COMMANDER → PILOT_TECHNICAL_OWNER (see management pack)

**Links:**
- `docs/BINTRANS_DEDICATED_CONTROL_TOWER_STAGING_RUNBOOK.md`
- `docs/CONTROL_TOWER_STAGING_OPERATOR_COMMANDS.md`

---

## Auth / login failure {#auth-login}

**Detection:** Login smoke failures; gateway 401/403 spike.

**First checks:**
1. `JWT_SECRET` classification in protected env (NONEMPTY_NONPLACEHOLDER)
2. `freight_identity_service` health
3. `docs/BINTRANS_STAGING_AUTH_SMOKE.md`

**Escalation:** SECURITY_OWNER if bypass suspected → recommend pilot freeze.

**Links:**
- `docs/BINTRANS_STAGING_AUTH_SMOKE.md`
- `docs/LOW_CODE_PILOT_WEEK3_AUTH_ON_STAGING_RUNBOOK_V0.1.md`

---

## RFx unavailable {#rfx}

**Detection:** RFx endpoints 5xx; `freight_rfx_service` unhealthy.

**First checks:**
1. Container health and OCI revision (`704ecbc…` intentional deploy)
2. Gateway proxy to rfx-service
3. Postgres connectivity

**Rollback snapshot:** `/protected/bintrans/deploy-snapshots/r3.1c1-rfx-*`

**Links:**
- `docs/BINTRANS_DEDICATED_CONTROL_TOWER_STAGING_RUNBOOK.md`

---

## Shipment execution failure {#shipment}

**Detection:** Status transition failures; assignment 400/409; timeline gaps.

**First checks:**
1. `freight_shipment_service` health (revision `48ef3e5…`)
2. Event timeline API for affected shipment ID
3. Outbox replay runbook if FAILED outbox events

**Links:**
- `docs/ops/BINTRANS_STAGING_FAILED_OUTBOX_RECOVERY_RUNBOOK.md`
- `scripts/ops/bintrans_shipment_outbox_replay.sh`

---

## Database unavailable {#database-unavailable}

**Detection:** Alert `BintransPilotPostgresUnavailable`; widespread 5xx; pool saturation alerts.

**First checks:**
1. `docker exec freight_postgres pg_isready`
2. Connection count / disk on host
3. **Do not rerun migrations** (schema at v64)

**Recovery:** Restore from `/protected/bintrans/backups/` into isolated target first if validation needed.

**Links:**
- `docs/BINTRANS_DEDICATED_CONTROL_TOWER_STAGING_RUNBOOK.md` (PHASE D)
- `scripts/ops/bintrans_ct_staging/bintrans_ct_staging_backup.sh`

---

## Control Tower unavailable {#control-tower-unavailable}

**Detection:** CT summary API errors; `BintransPilotControlTowerReadModelErrors`.

**First checks:**
1. `CONTROL_TOWER_READ_MODEL_MODE=shadow` (must not be primary)
2. `freight_control_tower_read_model_service` health
3. Redpanda consumer lag / dead letter metrics

**Links:**
- `docs/CONTROL_TOWER_STAGING_SHADOW_OBSERVATION.md`
- `docs/CONTROL_TOWER_SHADOW_ROLLOUT_RUNBOOK.md`

---

## Staging host unavailable {#host-unavailable}

**Detection:** SSH timeout; all endpoints down.

**First checks:**
1. Selectel VM panel / provider status
2. Disk full (`BintransPilotDiskPressure` alert)
3. Do not attempt destructive volume cleanup without authorization

**Links:**
- `docs/LOW_CODE_PILOT_WEEK3_SELECTEL_SSH_SECURITY_GROUP_RESTRICTION_RUNBOOK_V0.1.md`

---

## Backup failure {#backup-failure}

**Detection:** Backup script non-zero exit; `BintransPilotBackupStale` alert.

**First checks:**
1. Latest file in `/protected/bintrans/backups/`
2. `pg_restore -l` verification
3. Update protected env `BACKUP_PATH`, `BACKUP_SHA256`, `BACKUP_VERIFIED=YES`

**Links:**
- `scripts/ops/bintrans_ct_staging/bintrans_ct_staging_backup.sh`
- `scripts/ops/bintrans_ct_staging/bintrans_pilot_backup_metadata_update.sh`

---

## Security incident {#security-incident}

**Detection:** Suspected tenant leakage, auth bypass, credential exposure.

**Immediate actions:**
1. Recommend controlled pilot **freeze**
2. Capture sanitized evidence only (status codes, request IDs, paths — no JWT/passwords)
3. Escalate SECURITY_OWNER + GO_LIVE_AUTHORITY

**Links:**
- `docs/bintrans-pilot/BINTRANS_PILOT_MANAGEMENT_APPROVAL_PACK_V1.md`
- `docs/LOW_CODE_PILOT_WEEK3_SUPPORT_ESCALATION_MATRIX_V0.1.md`

---

## Telegram alert delivery {#telegram-alert}

**Status:** OPS-BLK-001 CLOSED (historical)

**Detection:** `contextDeadlineExceeded` in Alertmanager metrics; no human receipt.

**Remediation (staging):** Alertmanager `extra_hosts` for `api.telegram.org` — see operational decisions doc.

**Coordination channel:** BINTRANS Pilot Ops (ALERT_AND_INCIDENT_COORDINATION_CHANNEL)

---

## Observability / alerting

- Prometheus: `http://127.0.0.1:9090` (SSH tunnel)
- Alertmanager: `http://127.0.0.1:9093` (SSH tunnel)
- Grafana: `http://127.0.0.1:3001` (SSH tunnel)
- Health: `scripts/ops/bintrans_ct_staging/bintrans_ct_staging_observability_health.sh`

**INCIDENT_RUNBOOK_COMPLETE=YES** (operationally navigable via this index)
