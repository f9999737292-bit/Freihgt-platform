# BINTRANS Pilot RPO/RTO Approval Pack v1

**Status:** CLOSED — first scheduled backup proven (2026-09-03)

## Observed mechanism (P0 + P0.1 + first timer run)

| Field | Value |
|---|---|
| RESTORE_DRILL | PASS |
| OBSERVED_DB_RESTORE_DURATION | 6 seconds |
| LATEST_VALIDATED_BACKUP | `/protected/bintrans/backups/freight_platform_20260903T000016Z.dump` |
| FIRST_SCHEDULED_TIMER_RUN_UTC | 2026-09-03T00:00:16Z (03:00 MSK) |
| BACKUP_VALIDATION | PASS |
| DAILY_BACKUP_AUTOMATION | ACTIVE (`bintrans-pilot-backup.timer`) |

## Approved pilot targets

| Field | PROPOSED | APPROVED |
|---|---|---|
| RPO_TARGET | 24h (daily validated backup) | YES |
| RTO_TARGET | ≤30 minutes (restore + targeted service recreate) | YES |

## Approval record

| Field | Value |
|---|---|
| APPROVED_BY | GO_LIVE_AUTHORITY (Феликс) |
| APPROVED_AT | 2026-09-03 |
| FIRST_SCHEDULED_BACKUP_PROVEN | YES |
| RPO_OPERATIONALLY_SATISFIED | YES |

```text
RPO_TARGET=24h
RPO_TARGET_APPROVED=YES
RPO_AUTOMATION_ACTIVE=YES
RPO_FIRST_SCHEDULED_RUN_PROVEN=YES
RPO_OPERATIONALLY_SATISFIED=YES
RTO_TARGET=30m
RTO_APPROVED=YES
OPS_BLK_004_STATUS=CLOSED
OPS_BLK_004_CLOSED=YES
OPS_BLK_004_FINAL_VERDICT=PASS
```
