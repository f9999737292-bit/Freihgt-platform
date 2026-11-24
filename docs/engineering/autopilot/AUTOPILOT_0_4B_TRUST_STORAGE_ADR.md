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

| Role | Login | Database rights |
| --- | --- | --- |
| Worker | yes | `EXECUTE` on `append_worker_claim` only. No `EXECUTE` on `append_reviewer_verdict`. No table `INSERT`, `UPDATE`, `DELETE`, or `TRUNCATE`. |
| Reviewer recorder | yes, used only by the trusted workflow | `EXECUTE` on `append_reviewer_verdict` only. No `EXECUTE` on `append_worker_claim`. |
| Read-only auditor | yes | `SELECT` on both event tables and both tip tables. No `EXECUTE` on either write function. |

The split is a grant, not a workflow convention. The worker is not a member of the recorder role or the owner role, and `SET ROLE` to either is denied. `EXECUTE` on one append function does not grant the other.

The worker role matches the existing autopilot queue rule: `NOSUPERUSER`, `NOBYPASSRLS`, `NOCREATEROLE`, `NOCREATEDB`. It has no `CREATE` on the audit schema. It does not choose a tenant by a query parameter. Tenant scope comes from the authenticated database principal mapped from `session_user`, as `autopilot_queue.tenant_principals` already does for queue rows.

Inside either `SECURITY DEFINER` function, `current_user` is the function owner. That is not the caller. The function reads `session_user` and rejects the call unless that role is the single role granted `EXECUTE` on that function. A payload field named `author`, `tenant`, `kind`, or `reviewer` is not a privilege.

## 4. Database Owner NOLOGIN

The table owner is `NOLOGIN`. It is not a connection string stored for the worker. It is not a superuser and does not have `BYPASSRLS`. The worker, the recorder, and the auditor are not members of the owner. Grants are explicit and minimal: `EXECUTE` on one function, or `SELECT` for the auditor. The owner is used by a migration run under a controlled operator session, not by the dry-run process.

## 5. Tenant Isolation and RLS

Journal rows that are tenant-scoped carry the tenant of the authenticated principal. Row security is forced. The worker policy allows select only where the principal matches the row tenant. A claim insert goes through `append_worker_claim`, which uses that same principal. The worker has no table `INSERT`. A second tenant's verdict is not visible and cannot be appended under the first tenant's login. The restart budget for a tenant is part of the restart chain, keyed by that tenant, not by a caller-supplied worker id or file path.

A global audit verdict for a commit SHA is not a tenant business row. It is still written only by the reviewer recorder, and the worker cannot update it.

## 6. Append-Only Data Model

These are two independent chains, not one shared journal. Neither is created by this change.

Audit chain:

- `audit_event`: `audit_sequence bigint` primary key, `prev_hash`, `hash`, `kind` constrained to `claim` or `verdict`, body, tenant where the row is tenant-scoped, commit SHA where the row names a commit
- `audit_tip`: exactly one row, `audit_sequence` and `hash` of the current audit head

Restart chain:

- `restart_event`: `restart_sequence bigint` primary key, `prev_hash`, `hash`, `kind` constrained to `restart` or `hold`, tenant, count, reason
- `restart_tip`: exactly one row, `restart_sequence` and `hash` of the current restart head

A row in one chain does not advance the other tip. `UPDATE`, `DELETE`, and `TRUNCATE` are revoked from the worker, the reviewer recorder, and the auditor on both event tables and both tip tables. There is no function that deletes or rewrites a negative verdict, and there is no function that replaces either chain with a new genesis.

## 7. Security-Definer Contract

Two functions, both owned by the `NOLOGIN` owner. Names are provisional: `append_worker_claim` and `append_reviewer_verdict`.

`append_worker_claim` inserts `kind=claim` only. The kind is fixed by the function, not by a client argument. It reads `session_user` and continues only when that role is the worker role. It does not insert `kind=verdict`.

`append_reviewer_verdict` inserts `kind=verdict` only. It reads `session_user` and continues only when that role is the reviewer-recorder role. Before the insert, the trusted workflow must already have bound that call to one pull request, one exact head SHA, the successful checks for that SHA (`bintrans-autopilot-unit` and `bintrans-autopilot-queue`), and the reviewer principal the workflow read. The function stores that principal. It does not treat a payload field `reviewer=agent-j` as the author. `current_user` inside the definer remains the owner and is never recorded as the reviewer.

Neither function accepts a caller-selected previous hash. Each locks and reads its own tip. Direct `INSERT` on the tables is not granted. Neither function sets `merge_authorized`.

Hardening required of both functions, still not created here:

- `REVOKE ALL ON FUNCTION ... FROM PUBLIC`, then `GRANT EXECUTE` to exactly one role.
- `SET search_path` to a fixed list: `pg_catalog` and the audit schema. The function qualifies every table it writes.
- The worker and the recorder have no `CREATE` on `pg_catalog` or the audit schema, so they cannot plant an object earlier in the name search.
- The owner has no login, no superuser, and no `BYPASSRLS`. Callers are not members of the owner.

## 8. Atomic Sequence and Tip Hash

Each append locks its own tip row with `SELECT ... FOR UPDATE`, reads that chain's sequence and hash, rejects the write when the stored hash does not match the chain head, inserts the next row with that sequence plus one, and updates that same tip in the transaction. Audit callers serialize on `audit_tip`. Restart callers serialize on `restart_tip`. They do not share a lock or a sequence. A tail cannot be removed while its tip still names the removed row. Replacing either chain requires changing its tip, which neither granted role can do.

## 9. PostgreSQL Concurrency and Transaction Boundary

The append is one transaction. A crash before commit leaves the previous tip. A crash after commit leaves the new row and the new tip together. The function does not use a session-level advisory lock held by the worker as the integrity mechanism. Isolation level is at least `READ COMMITTED` with `SELECT ... FOR UPDATE` on the tip. The worker cannot set `row_security = off`.

## 10. Privileged DBA Threat

A database superuser or the table owner can still disable row security, drop the function, or rewrite rows. Role separation does not stop that person. The ADR therefore does not call the database alone a complete anchor against its own administrator.

## 11. External Anchor Outside Worker/DBA Control

Each chain tip is also published by the trusted workflow to an append-only log that the autopilot database administrator and the worker cannot rewrite. The service, the publication interval, and the SLA are `POLICY_DECISION_REQUIRED`. This ADR does not choose them.

A reviewer verdict is not eligible for the audit gate until the external log has an entry for that verdict's `audit_sequence` and hash. The gate reads the external tip before it accepts any database verdict. If the external entry is missing, the gate stays `blocked`. A database row alone is not enough.

The window between the database commit and the external publication is the residual risk. During that window the row exists in the database and must not be treated as a trusted verdict. A later publication closes the window for that sequence. A publication that skips a sequence, or an external hash that differs from `audit_tip`, refuses the gate. The same rule applies to `restart_tip` before a restart count from the database is treated as the trusted budget.

The worker does not hold the signing key. Absence of the external tip is fail-closed, not an empty journal.

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
| Worker sends `reviewer=agent-j` | gate stays blocked, `reviewer_authenticated` false; no verdict row is written |
| Worker `EXECUTE` on `append_reviewer_verdict` | permission denied |
| Worker `SET ROLE` to the recorder or the owner | permission denied |
| `PUBLIC` `EXECUTE` on either write function | permission denied |
| Auditor `EXECUTE` on either write function | permission denied |
| Recorder `EXECUTE` on `append_worker_claim` | permission denied |
| Search-path object placed by the worker | function still writes the audit schema; worker has no `CREATE` there |
| Cross-tenant insert under another tenant's principal | rejected; tenant comes from `session_user`, not the payload |
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
2. Choose the external anchor, its operator, its publication interval, and its SLA. Those remain `POLICY_DECISION_REQUIRED`.
3. Approve an autopilot-schema migration number after the head is read again.
4. Keep `AUTOPILOT_ENABLED`, worker execution, paid automation, and production 24/7 off.
5. Do not treat a stored verdict as merge or deploy authority.

Until those gates are accepted, Agent H does not add the tables, the roles, or a path that clears the finding gate.
