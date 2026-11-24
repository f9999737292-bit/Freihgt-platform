# Autopilot 0.4B trust storage ADR

Status: proposal only. This document does not authorize an implementation, a migration, a role, a credential, or a transition that sets `merge_authorized`. The current finding gate stays `blocked`.

The local files `tools/bintrans_autopilot/state/audit-journal.jsonl` and `restart-ledger.jsonl` remain the dry-run record. They stay inside the checkout because moving them would change the directory a deployment must persist. They are gitignored. They are not deleted or truncated by this stage. A valid shorter prefix of either file still verifies. That residual is open.

## 1. Threat Model

The worker process can read and write its own files. A hash chain in such a file detects a torn line and a changed byte. It does not detect:

- deletion of the file followed by a new empty chain, if some caller is allowed to create that chain
- truncation to a shorter prefix that still hashes, including loss of a negative verdict or of restart-budget rows
- replacement of the file by a newly computed chain

An HMAC key stored where the worker can read it does not help. The worker could recompute the tag. Python code running in the worker process is not an untrusted isolated subject. Isolation requires a database role, and for the database administrator an anchor outside that database.

The packet fields `reviewer`, `verdict`, and `test_result` are caller text. They are not identity, not a CI result, and not a merge grant.

## 2. Trusted Reviewer Identity

A future reviewer identity is a GitHub login on an allowlist, read by the trusted workflow on `main`, for a pull-request review of the exact head SHA. The login must not be the pull-request author. The string `agent-j` inside a packet does not establish that identity. The server records the login it read. The client does not send it as the source of truth.

## 3. Worker / Reviewer / Read-Only Auditor Roles

Three database roles, none of which is the table owner:

| Role | Login | Intended rights |
| --- | --- | --- |
| Worker | yes | `INSERT` into the append function only. No `UPDATE`, `DELETE`, `TRUNCATE`, or direct table write. |
| Reviewer recorder | yes, used only by the trusted workflow | `INSERT` of a verdict row through the same append function, after the workflow has verified identity and CI. |
| Read-only auditor | yes | `SELECT` on the journal and the tip. No write. |

The worker role matches the existing autopilot queue rule: `NOSUPERUSER`, `NOBYPASSRLS`, `NOCREATEROLE`, `NOCREATEDB`. It does not receive the owner role and it does not choose a tenant by a query parameter. Tenant scope comes from the authenticated database principal, as `autopilot_queue.tenant_principals` already does.

## 4. Database Owner NOLOGIN

The table owner is `NOLOGIN`. It is not a connection string stored for the worker. Grants to the worker, the reviewer recorder, and the auditor are explicit. The owner is used by a migration run under a controlled operator session, not by the dry-run process.

## 5. Tenant Isolation and RLS

Journal rows that are tenant-scoped carry the tenant of the authenticated principal. Row security is forced. The worker policy allows insert and select only where the principal matches the row tenant. A second tenant's verdict is not visible and cannot be appended under the first tenant's login. The restart budget for a tenant is part of the same store, keyed by that tenant, not by a caller-supplied worker id or file path.

A global audit verdict for a commit SHA is not a tenant business row. It is still written only by the reviewer recorder, and the worker cannot update it.

## 6. Append-Only Data Model

Proposed tables, not created by this change:

- `audit_event`: monotonic sequence, previous hash, hash, kind, body, tenant where applicable, commit SHA where applicable
- `audit_tip`: one row, current sequence and current hash
- `restart_event`: the same chain shape for restart and hold rows, included in this store rather than left only on disk

`UPDATE`, `DELETE`, and `TRUNCATE` are revoked from the worker, the reviewer recorder, and the auditor. There is no application function that deletes a negative verdict.

## 7. Security-Definer Contract

A single definer function, owned by the `NOLOGIN` owner, performs the insert. It runs as the owner only for that function. It does not accept a caller-selected previous hash as authority: it reads the tip itself. It rejects a direct `INSERT` by the worker if table privileges do not include `INSERT` on the table. The function is the only write path granted to the worker and the reviewer recorder.

The definer does not set `merge_authorized`. It stores claims and verdicts. Authorization remains a separate decision of the trusted workflow.

## 8. Atomic Sequence and Tip Hash

The function locks the tip row, reads its sequence and hash, rejects the write when the stored hash does not match the hash of the existing chain head, inserts the next row with `sequence + 1`, and updates the tip in the same transaction. Two concurrent callers serialize on that row. A tail cannot be removed while the tip still names the removed row. Replacing the whole chain requires changing the tip, which the worker cannot do.

## 9. PostgreSQL Concurrency and Transaction Boundary

The append is one transaction. A crash before commit leaves the previous tip. A crash after commit leaves the new row and the new tip together. The function does not use a session-level advisory lock held by the worker as the integrity mechanism. Isolation level is at least `READ COMMITTED` with `SELECT ... FOR UPDATE` on the tip. The worker cannot set `row_security = off`.

## 10. Privileged DBA Threat

A database superuser or the table owner can still disable row security, drop the function, or rewrite rows. Role separation does not stop that person. The ADR therefore does not call the database alone a complete anchor against its own administrator.

## 11. External Anchor Outside Worker/DBA Control

The tip hash is also published by the trusted workflow to an append-only log that the autopilot database administrator and the worker cannot rewrite. Examples of a later choice, none selected or implemented here: a signed note on a repository the workflow account can append to and the database role cannot, or a transparency log under a different operator. The worker does not hold the signing key. Verification reads the external tip and refuses the database chain when the two hashes differ. Absence of the external tip is fail-closed, not an empty journal.

## 12. Crash Recovery and Reconciliation

Do not delete rows to recover. Do not truncate a local file to the last valid line and continue. After a crash:

1. The database transaction either contains the new row and the new tip, or neither.
2. A torn local file, if one is still used as a cache, stays refused.
3. Reconciliation re-reads the database tip and the external tip. A mismatch refuses further appends until an operator following this ADR restores from the external log. Restoration is not an automated delete.

The restart ledger uses the same rule. A damaged ledger holds the tenant. It is not replaced with a new zero count.

## 13. Fail-Closed on Database Outage

If the store cannot be read or the tip transaction cannot commit, submit and restart accounting return a refusal. They do not fall back to a new local file. `readiness-check` stays `REFUSED` when emergency stop is set, and a store outage does not enable autopilot, a worker, or a merge.

## 14. Reviewer Verdict Authentication

A verdict row is written only by the reviewer recorder after the trusted workflow has all of:

1. The allowlisted reviewer login from the GitHub review API, not from the packet.
2. The review targeted at the exact head SHA.
3. Successful check runs for that same SHA, including `bintrans-autopilot-unit` and `bintrans-autopilot-queue`.

The stored author is the login the workflow read. A later packet that repeats `reviewer=agent-j` does not authenticate anyone and does not remove a stored negative verdict.

## 15. Audit-Gate State Machine

Current code has one effective state: `blocked`. `merge_authorized`, `reviewer_authenticated`, and `ci_verified` stay false. This ADR does not add a code path out of that state.

A later implementation, after controller approval, would distinguish:

| State | Meaning |
| --- | --- |
| `blocked` | Default. Missing store, torn chain, tip mismatch, outage, or no authenticated verdict. |
| `negative_recorded` | An authenticated reviewer verdict is `CHANGES_REQUIRED` or `BLOCKED`, or a `CRITICAL` or `HIGH` finding was recorded. Later text does not remove it. |
| `reviewed` | An authenticated pass exists for the exact SHA, CI for that SHA succeeded, and no negative row remains for that SHA. |

`reviewed` is not `merge_authorized`. Merge, deploy, and production 24/7 remain separate controller actions. They are not implied by a verdict row.

## 16. Migration Isolation

No migration is added in the change that introduces this document. The autopilot schema head stays `0015`. A later migration, if approved, is an autopilot-schema migration only. It must not reuse a number, must not be a platform migration, and must not run against staging or production as part of writing the ADR. The database name check that rejects `prod`, `stag`, `freight`, `platform`, and `postgres` remains in force for tests.

## 17. Negative Security Test Matrix

A later implementation must show, and this document does not claim these tests exist yet:

| Case | Required result |
| --- | --- |
| Worker `UPDATE`, `DELETE`, or `TRUNCATE` | permission denied, row remains |
| Worker replaces the tip | permission denied |
| Worker sends `reviewer=agent-j` | gate stays blocked, `reviewer_authenticated` false |
| Worker sets `merge_authorized` | no such write succeeds |
| Second tenant reads the first tenant's row | zero rows |
| Worker connects with `BYPASSRLS` | role does not have the attribute |
| Truncated chain whose tip no longer matches | append refused |
| Database unavailable | refusal, no new local chain |
| External tip missing or different | refusal |
| Torn local file | refusal, file not deleted |
| Restart ledger prefix that still hashes | not accepted once the ledger lives in this store |
| `CRITICAL` or `HIGH` finding | negative row, later pass does not remove it |

## 18. Controller Approval Gates

Implementation waits for a separate controller decision on all of the following:

1. Accept this role split and the `NOLOGIN` owner.
2. Choose the external anchor and the operator who holds it.
3. Approve an autopilot-schema migration number after the head is read again.
4. Keep `AUTOPILOT_ENABLED`, worker execution, paid automation, and production 24/7 off.
5. Do not treat a stored verdict as merge or deploy authority.

Until those gates are accepted, Agent H does not add the tables, the roles, or a path that clears the finding gate.
