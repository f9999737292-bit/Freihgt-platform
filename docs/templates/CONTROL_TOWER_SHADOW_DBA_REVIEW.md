# Control Tower Shadow Observation — DBA Review

Review before staging migration and before Day 7 final gate.

**Result:** APPROVED / APPROVED WITH CONDITIONS / REJECTED

---

## Checklist

| # | Item | Pass | Notes |
|---|------|------|-------|
| 1 | Migrations `000016–000019` reviewed | | |
| 2 | Backup verified before migrate | | |
| 3 | Restore tested or reviewed | | |
| 4 | Stage table growth acceptable | | |
| 5 | Backup table growth acceptable | | |
| 6 | Activation transaction duration reviewed | | |
| 7 | Exclusive advisory lock duration reviewed | | |
| 8 | WAL growth monitored | | |
| 9 | Replication lag acceptable | | |
| 10 | Database connections within limits | | |
| 11 | Query plans reviewed for rebuild paths | | |
| 12 | Autovacuum adequate for stage/backup tables | | |
| 13 | Retention policy documented | | |
| 14 | Cleanup authorization process defined | | |
| 15 | Down migration limitations for `000018` and `000019` understood | | |

---

## Migration notes

| Version | Description |
|---------|-------------|
| 000016 | Rebuild core (job, stage, backup) |
| 000017 | Activation/rollback constraints |
| 000018 | Nullable `last_event_type` on stage — down fails with NULL values |
| 000019 | Nullable `last_event_type` on rebuild backup — down fails with NULL values |

**Staging policy:** no down migrations.

---

## Backup retention proposal

- **ACTIVE** backup: retain through observation window + 7 days after activation
- **FAILED / CANCELLED / ROLLED_BACK**: cleanup only after review and explicit confirmation

---

## Reviewer

| Field | Value |
|-------|-------|
| Reviewer | |
| Date | |
| Result | APPROVED / APPROVED WITH CONDITIONS / REJECTED |
| Conditions | |
