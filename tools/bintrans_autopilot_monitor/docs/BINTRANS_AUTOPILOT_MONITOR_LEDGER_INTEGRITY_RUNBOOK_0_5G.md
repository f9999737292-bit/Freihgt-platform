# Ledger integrity runbook 0.5G

This runbook is the local procedure for the SQLite budget ledger. It does not start a workflow, does not read a token, and does not change the candidate workflow. A host administrator who can replace files is outside this control unless an independent trusted record of runs and minutes exists.

## What the ledger file is

The campaign lives in one SQLite file under a dedicated directory. `initialize_campaign` is the only creator. `BudgetGate.open` does not create a file. The file mode is owner read/write where the operating system honors that mode, and the directory mode is owner-only. On Windows those bits are best effort. They do not stop an administrator of the host.

The journal mode is WAL. A `-wal` or `-shm` file next to the database is normal. Its presence is not evidence that the database is corrupt. Opening the ledger does not treat those sidecar files as corruption.

## Backup

Take a backup only with `backup_ledger`. That function checkpoints the WAL when the lock allows it, then copies a consistent snapshot with the SQLite backup API, then runs `integrity_check` on the copy. It does not copy the database file by itself and leave a live WAL behind as the only copy. A failed integrity check deletes the new copy and leaves the live ledger unchanged.

Keep the backup inside the same protected directory. A path that is a symlink, a path whose parent is a symlink, or a path that resolves outside that directory is rejected. A missing directory or a path whose parent is not a directory is rejected.

## Restore

1. Stop the local controller so it is not writing the file.
2. Restore only with `restore_ledger`, and pass the campaign id that the file is required to have. A different campaign id is rejected and the live file is left unchanged. A backup that fails `integrity_check` is rejected and the live file is left unchanged.
3. The restored campaign is `RECOVERY_REQUIRED`. Reserve and dispatch stay denied. Known reconciled rows and any unfinished rows that were inside that snapshot stay in the file. The restore does not create a fresh 60 minute campaign and does not delete confirmed minutes that were in the snapshot.
4. Compare the snapshot with an independent list of workflow runs and their consumption. Record every run that the snapshot is missing with `record_external_consumption`. That records the job minutes. It does not clear recovery.
5. If that independent list cannot be read, stop. `release_recovery` with `external_state_known=false` denies. Do not continue.
6. After the missing runs are recorded, and only after a separate controller GO, the operator may attest the reconciliation and call `release_recovery(controller_go=true, external_state_known=true)`. Release does not change confirmed minutes. An unfinished reserve still denies release. A campaign blocked for a reason other than recovery stays blocked.

## Old snapshot

A backup can be an old, internally consistent database. Restoring it can drop dispatches that happened after the backup and can make the remaining minutes look larger, including a file that still shows 60 minutes because it was taken before any spend. That is not a successful recovery. The campaign stays in `RECOVERY_REQUIRED` until the external runs are reconciled and a controller GO is recorded. There is no automatic return to a fresh 60 minute budget.

This procedure does not protect against a host administrator who can replace the file and also forge the attestation. Without an independent trusted source of runs and minutes, the local file cannot prove that the restored numbers are complete. `LEDGER_ROLLBACK_RESIDUAL_RISK=HOST_ADMIN_CAN_REPLACE_A_VALID_SNAPSHOT`.

## Do not

- Do not copy only the `.sqlite` file while a WAL exists and then open that copy as the campaign.
- Do not delete the WAL or SHM to "repair" a database that still opens and passes `integrity_check`.
- Do not raise `MAX_TOTAL_ACTIONS_MINUTES` above 60 to hide a restore.
- Do not resume dispatch because the restored file looks valid.
