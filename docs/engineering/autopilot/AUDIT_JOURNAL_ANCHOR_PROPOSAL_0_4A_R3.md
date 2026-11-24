# Audit journal anchor proposal 0.4A R3

Status: proposal only. This note does not add a store, a role, a migration, or a way to clear the finding gate.

## What the local file can prove

`AuditJournal` links each line to the hash of the previous line. While the file is held under an exclusive lock, two processes cannot append at the same time, so a normal crash after `fsync` leaves one chain.

The verifier rejects a torn line, a missing trailing newline, a changed byte, and a missing file. A missing file is not recreated. `readiness-check` and the gateway stay fail-closed.

## What the local file cannot prove

The worker process can rewrite the path. These edits still verify, because the new bytes form a chain from the same genesis:

- dropping complete trailing lines, including a block that recorded a negative decision
- replacing the file with a new chain that omits that block

An HMAC key stored where the worker can read it would not close this gap. The worker could recompute the tag. This stage does not add such a key.

## Store that would close the gap

The anchor has to live where the worker role cannot update, delete, or replace history. One fit for the existing autopilot database, and not a product migration, is:

1. An append-only table owned by a role the worker does not use.
2. `INSERT` only for the worker role. `UPDATE`, `DELETE`, and `TRUNCATE` stay revoked.
3. A tip row, or a hash stored by a definer the worker cannot replace, that rejects an insert whose previous hash does not match the tip.
4. No repair path that deletes rows. A missing or torn local copy stays refused until that store is read by the trusted workflow.

That role split is a new trust boundary. It is not created here. Until an operator accepts it and a later change adds it, a negative decision can still be removed by replacing the local file with a valid shorter chain. The finding gate remains `blocked` either way.

## Recovery chosen for the local file

A torn file is left in place. The next open refuses it. The file is not deleted and the tail is not cut off to make verification pass. Two processes wait on the lock instead of writing interleaved lines.
