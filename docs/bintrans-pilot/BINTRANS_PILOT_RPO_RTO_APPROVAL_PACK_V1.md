# BINTRANS Pilot RPO/RTO Approval Pack v1

**Status:** Targets management approved — RPO not operationally satisfied
**Blocker:** OPS-BLK-004 — **OPEN_PENDING_DAILY_BACKUP_AUTOMATION**

## Observed mechanism (HISTORICAL)

| Field | Value |
|---|---|
| RESTORE_DRILL | PASS |
| OBSERVED_TECHNICAL_RESTORE_DURATION | ~6 seconds (isolated DB restore) |
| LATEST_BACKUP | `/protected/bintrans/backups/freight_platform_20260901T190602Z.dump` |
| BACKUP_VALIDATION | PASS |
| BACKUP_SCRIPT | `scripts/ops/bintrans_ct_staging/bintrans_ct_staging_backup.sh` |

## Backup mode

| Field | Value |
|---|---|
| BACKUP_CURRENT_MODE | MANUAL_OPERATOR_INVOCATION |
| BACKUP_AUTOMATED | NO |
| BACKUP_SCHEDULE | None automated |
| RPO_IMPLEMENTATION_GAP | **YES** |
| DAILY_VALIDATED_BACKUP_REQUIRED | **YES** |
| BACKUP_FOLLOWUP_REQUIRED | **YES** |
| DAILY_BACKUP_TIME_PROPOSED | 03:00 MSK (technical proposal — not implemented) |

## Approved targets

| Field | Value | MANAGEMENT_APPROVED |
|---|---|---|
| RPO_TARGET | 24 hours | **YES** |
| RTO_TARGET | 30 minutes (committed operational) | **YES** |

## Approval record

| Field | Value |
|---|---|
| APPROVED_BY | Феликс |
| APPROVED_AT | 2026-09-03 |
| APPROVAL_SOURCE | CONTROLLER_CHAT |
| RPO_TARGET_MANAGEMENT_APPROVED | YES |
| RTO_MANAGEMENT_APPROVED | YES |
| RTO_APPROVED | **YES** |
| RPO_OPERATIONALLY_SATISFIED | **NO** |
| RPO_GAP_ACCEPTED | **NO** |
| RPO_GAP_ACCEPTED_BY | — |
| RPO_GAP_ACCEPTED_DATE | — |

```text
RPO_TARGET=24h
RTO_TARGET=30 minutes
RTO_APPROVED=YES
RPO_OPERATIONALLY_SATISFIED=NO
OPS-BLK-004=OPEN_PENDING_DAILY_BACKUP_AUTOMATION
OPS-BLK-004_CLOSED=NO
```

**Distinction:** OBSERVED_TECHNICAL_RESTORE_DURATION ≈ 6 seconds ≠ COMMITTED_OPERATIONAL_RTO = 30 minutes.
