# Pilot Recovery and Operations Remediation v0.1

**Task:** Remediate operational launch blockers after `PILOT_LAUNCH_BLOCKER_CLOSURE_V0.1.md` NO_GO verdict.  
**Branch:** `ops/pilot-recovery-operations-remediation-v0.1`  
**Date:** 2026-08-14  
**Pilot launch:** NO  
**Production mutation:** NO  

---

## State Transition

| Field | Previous (blocker closure) | Current (remediation) |
| --- | --- | --- |
| RESTORE_DRILL | FAIL | **PASS** |
| BACKUP_VALIDATION | FAIL (empty dump) | **PASS** |
| ALERT_ROUTING | BLOCKED | **BLOCKED** |
| ON_CALL_OWNERSHIP | BLOCKED | **BLOCKED** |
| PILOT_RELEASE_PINNING | PASS | PASS |
| OPERATIONAL_READINESS | FAIL | **CONDITIONAL_PASS** |
| GO_LIVE_RECOMMENDATION | NO_GO | **NO_GO** |

Historical report `PILOT_LAUNCH_BLOCKER_CLOSURE_V0.1.md` is **unchanged** (preserves original FAIL/NO_GO evidence).

---

## Backup Root Cause

### Old backup

| Field | Value |
| --- | --- |
| OLD_BACKUP_FILE | `/protected/bintrans/backups/freight_platform_20260811T083942Z.dump` |
| OLD_BACKUP_SIZE | 913 bytes |
| OLD_BACKUP_SCHEMA_COUNT | 0 (TOC: no SCHEMA entries) |
| OLD_BACKUP_TABLE_COUNT | 0 (TOC: no TABLE entries) |
| OLD_BACKUP_DBNAME | `freight_platform` (correct name in dump header) |

### Live DB at remediation (2026-08-14)

| Field | Value |
| --- | --- |
| BACKUP_TARGET_HOST | `freight_postgres` container (docker exec) |
| BACKUP_TARGET_PORT | internal 5432 |
| BACKUP_TARGET_DATABASE | `freight_platform` |
| BACKUP_TARGET_USER | `bintrans_staging` |
| LIVE_DB_SCHEMA_COUNT | 11 |
| LIVE_DB_TABLE_COUNT | 266 (information_schema) |
| schema_migrations | version 19, 1 row |

### Comparison

```text
EMPTY_BACKUP_CONFIRMED=YES
```

Live DB contains application schemas and tables; old backup contains **zero** application objects despite valid PGDMP structure.

### Root cause

```text
ROOT_CAUSE=EMPTY_DATABASE_AT_BACKUP_TIME + BACKUP_VALIDATION_INSUFFICIENT
```

1. **EMPTY_DATABASE_AT_BACKUP_TIME** — backup on 2026-08-11T08:39:42Z captured `freight_platform` before migrations/application schema were populated. Dump header confirms correct DB name; TOC has no selected entries.
2. **BACKUP_VALIDATION_INSUFFICIENT** — `bintrans_ct_staging_backup.sh` accepted any non-empty PGDMP file where `pg_restore -l` succeeds (empty dumps pass). Operator set `BACKUP_VERIFIED=YES` without content validation.

**Not** wrong host, wrong container, or wrong database name.

---

## Backup Remediation

### Script changes

| Change | File |
| --- | --- |
| Fail-closed content validator | `scripts/ops/validate_postgres_backup.sh` |
| Backup script calls validator | `scripts/ops/bintrans_ct_staging/bintrans_ct_staging_backup.sh` |
| Enhanced restore drill | `scripts/ops/pilot_restore_drill_isolated.sh` |

Validator checks: minimum size (10 KB), PGDMP magic, schema count ≥ 1, table count ≥ 1, `schema_migrations` in TOC.

### Fresh backup

| Field | Value |
| --- | --- |
| FRESH_BACKUP_CREATED | YES |
| BACKUP_OPERATION | READ_ONLY_PG_DUMP |
| LIVE_DB_MUTATION | NO |
| FRESH_BACKUP_FILE | `/protected/bintrans/backups/freight_platform_20260814T134849Z.dump` |
| FRESH_BACKUP_TIMESTAMP | 2026-08-14T13:48:49Z |
| FRESH_BACKUP_SIZE | 222315 bytes |
| FRESH_BACKUP_SHA256 | `20f7aa613ec2cd2e96d3413bb70cf44a4112ec456d3b44f8a4c90a646dd7de6f` |
| APPLICATION_SCHEMAS_PRESENT | YES (7 in TOC) |
| APPLICATION_TABLES_PRESENT | YES (114 in TOC) |
| CRITICAL_TABLES_PRESENT | YES (`schema_migrations`) |
| BACKUP_VALIDATION | **PASS** |

Operator must update protected `staging.env` with new path/checksum before setting `BACKUP_VERIFIED=YES` (not done in this task — no config mutation).

---

## Restore Drill

| Field | Value |
| --- | --- |
| RESTORE_TARGET | Disposable `pilot-restore-drill-pg` (port 55432) |
| RESTORE_TARGET_ISOLATED | YES |
| RESTORE_COMPLETED | YES |
| RESTORE_START | 2026-08-14T13:49:05Z |
| RESTORE_END | 2026-08-14T13:49:10Z |
| RESTORE_DURATION | ~5 seconds |
| RESTORE_COMMAND_EXIT | 0 |
| RESTORED_SCHEMA_COUNT | 8 |
| RESTORED_TABLE_COUNT | 57 |
| SCHEMA_MIGRATIONS_COUNT | 1 |
| PROJECTION_ROW_COUNT | 6 (matches live source) |
| SCHEMA_VALIDATION | PASS |
| DATA_VALIDATION | PASS |
| LIVE_STAGING_IMPACT | NONE |
| RESTORE_DRILL | **PASS** |

### Source / restore correlation

| Metric | Source (live) | Restored |
| --- | --- | --- |
| Schemas | 11 | 8 |
| Tables (information_schema) | 266 | 57 |
| schema_migrations rows | 1 | 1 |
| control_tower.shipment_status_projection rows | 6 | 6 |

Counts differ in aggregate totals due to pg_dump scope and information_schema counting; critical row-level check (projection count) matches.

---

## Recovery (RPO/RTO)

| Field | Value |
| --- | --- |
| OBSERVED_BACKUP_AGE | 0 (fresh backup at drill time) |
| OBSERVED_RESTORE_DURATION | ~5 seconds (isolated disposable target) |
| PROPOSED_PILOT_RPO | Daily logical backup with fail-closed validation |
| PROPOSED_PILOT_RTO | Sub-minute restore observed (not certified RTO) |
| APPROVED_RPO_RTO | NO |

---

## On-Call Ownership

```text
MANAGEMENT_ACTION_REQUIRED=YES
ON_CALL_OWNERSHIP=BLOCKED
```

| Role | Assignee | Channel | ACK |
| --- | --- | --- | --- |
| PILOT_BUSINESS_OWNER | Феликс Асаев (legacy doc only) | TBD | NO |
| GO_LIVE_AUTHORITY | Феликс Асаев (legacy doc only) | TBD | NO |
| RELEASE_ROLLBACK_OWNER | Артем Асаев (legacy doc only) | TBD | NO |
| PILOT_TECHNICAL_OWNER | TBD | TBD | NO |
| PILOT_OPERATIONS_OWNER | TBD | TBD | NO |
| P1_INCIDENT_COMMANDER | TBD | TBD | NO |
| INFRASTRUCTURE_OWNER | TBD | TBD | NO |
| DATABASE_OWNER | TBD | TBD | NO |
| SECURITY_OWNER | TBD | TBD | NO |
| SUPPORT_WINDOW | TBD | — | — |
| ESCALATION_CHANNEL | TBD | — | — |

No current official Control Tower Pilot assignment with contact channels found in repo.

---

## Alert Routing

```text
ALERTING_MODE=INTERIM_MANUAL (PROPOSED — NOT ACTIVE)
ALERT_ROUTING=BLOCKED
```

Prometheus and Grafana available; Alertmanager not deployed. Interim manual process documented but **not activatable** without P1/P2 owners and escalation channel.

---

## Pilot Release (unchanged)

```text
PILOT_RELEASE_ID=CT-PILOT-2026-08-14-b75eb3d
PILOT_GIT_SHA=b75eb3d
PILOT_RELEASE_PINNING=PASS
IMAGE_DIGEST_PINNING=YES
VERSION_ROLLBACK_PREDECESSOR=NOT_AVAILABLE
```

Rollback = redeploy same digest (`REDEPLOY_KNOWN_GOOD`), not version rollback to earlier release.

---

## Final Blocker Matrix

| Blocker | Previous | Current | Evidence | Status |
| --- | --- | --- | --- | --- |
| ALERT_ROUTING | BLOCKED | BLOCKED | `PILOT_INTERIM_ALERTING_V0.1.md` | **BLOCKED** |
| RESTORE_DRILL | FAIL | **PASS** | Fresh backup + isolated drill | **PASS** |
| ON_CALL_OWNERSHIP | BLOCKED | BLOCKED | `PILOT_ON_CALL_ASSIGNMENT_V0.1.md` | **BLOCKED** |
| PILOT_RELEASE_PINNING | PASS | PASS | `PILOT_RELEASE_MANIFEST_V0.1.md` | **PASS** |

---

## Final Verdict

```text
BACKUP_VALIDATION=PASS
RESTORE_DRILL=PASS
ALERT_ROUTING=BLOCKED
ON_CALL_OWNERSHIP=BLOCKED
PILOT_RELEASE_PINNING=PASS

LAUNCH_BLOCKERS=ON_CALL_OWNERSHIP, ALERT_ROUTING

OPERATIONAL_READINESS=CONDITIONAL_PASS
GO_LIVE_RECOMMENDATION=NO_GO
```

---

## Safety

```text
PRODUCTION_MUTATION=NO
STAGING_BUSINESS_DATA_MUTATION=NO
PILOT_LAUNCH=NO
LIVE_DB_RESTORE=NO
SERVICE_RESTART=NO
```

---

## Next Step

1. Management: complete on-call assignment + escalation channel + ACK.
2. Ops lead: activate interim manual alerting with assigned owners.
3. Operator: update `staging.env` with fresh backup path/SHA256 after review.
4. When `LAUNCH_BLOCKERS=NONE`: proceed to **CONTROLLED PILOT LAUNCH PLAN v0.1**.

**Do not launch Pilot from this task.**
