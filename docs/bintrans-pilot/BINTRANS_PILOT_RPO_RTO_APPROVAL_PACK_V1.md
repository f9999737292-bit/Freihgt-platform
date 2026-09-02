# BINTRANS Pilot RPO/RTO Approval Pack v1

**Status:** OPEN — awaiting GO_LIVE_AUTHORITY  
**Blocker:** OPS-BLK-004  
**Full analysis:** `docs/bintrans-pilot/BINTRANS_PILOT_MANAGEMENT_APPROVAL_PACK_V1.md` §7–8

## Observed mechanism (HISTORICAL — not approval)

| Field | Value |
|---|---|
| RESTORE_DRILL | PASS |
| OBSERVED_TECHNICAL_RESTORE_DURATION | ~6 seconds (isolated DB restore) |
| LATEST_BACKUP | `/protected/bintrans/backups/freight_platform_20260901T190602Z.dump` |
| BACKUP_VALIDATION | PASS (`pg_restore -l`, SHA256) |
| BACKUP_SCRIPT | `scripts/ops/bintrans_ct_staging/bintrans_ct_staging_backup.sh` |

## Backup mode

| Field | Value |
|---|---|
| BACKUP_CURRENT_MODE | Manual operator invocation |
| BACKUP_AUTOMATED | NO |
| BACKUP_SCHEDULE | None automated |
| BACKUP_VALIDATION_AUTOMATED | Partial (freshness metric after manual run) |
| RPO_IMPLEMENTATION_GAP | **YES** |
| DAILY_VALIDATED_BACKUP_REQUIRED | YES (for proposed RPO=24h) |

## Proposed pilot targets (NOT APPROVED)

| Field | PROPOSED | APPROVED |
|---|---|---|
| RPO | 24 hours (daily validated logical backup) | NO |
| RTO | 30 minutes (committed operational — not seconds-level drill) | NO |

## Approval record (do not fill without evidence)

| Field | Value |
|---|---|
| APPROVED_BY | — |
| APPROVED_AT | — |
| RISK_ACCEPTED_BY | — |
| RISK_ACCEPTED_AT | — |

```text
RPO_PROPOSED=24h
RPO_APPROVED=NO
RTO_PROPOSED=30m
RTO_APPROVED=NO
RISK_ACCEPTANCE_RECORDED=NO
OPS-BLK-004=OPEN_WAITING_FOR_MANAGEMENT_APPROVAL
```
