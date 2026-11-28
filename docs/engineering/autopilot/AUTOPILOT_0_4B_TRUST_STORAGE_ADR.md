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

## 3. Worker / Reviewer / Restart Recorder / Auditor Roles

Four database roles, none of which is the table owner:

| Role | Login | Database rights |
| --- | --- | --- |
| Worker | yes, one role per tenant | `EXECUTE` on `append_worker_claim` only. No `EXECUTE` on `append_reviewer_verdict` or `append_restart_event`. No table `INSERT`, `UPDATE`, `DELETE`, or `TRUNCATE`. |
| Reviewer recorder | yes, one role, trusted workflow only | `EXECUTE` on `append_reviewer_verdict` only. No `EXECUTE` on the other two write functions. |
| Restart recorder | yes, one role, trusted restart component only | `EXECUTE` on `append_restart_event` only. No `EXECUTE` on the other two write functions. |
| Read-only auditor | yes, one role per tenant | `SELECT` only where the row tenant is the tenant mapped to that role. No `EXECUTE` on any write function. No `SELECT` of another tenant. |

The split is a grant, not a workflow convention. No caller is a member of another caller role or of the owner, and `SET ROLE` to any of them is denied. `EXECUTE` on one append function does not grant the others.

The worker role matches the existing autopilot queue rule: `NOSUPERUSER`, `NOBYPASSRLS`, `NOCREATEROLE`, `NOCREATEDB`. It has no `CREATE` on the audit schema. It does not choose a tenant by a query parameter.

`session_user` is the login PostgreSQL recorded at authentication. It does not follow `SET ROLE`. A future write function resolves the caller by the same rule as `autopilot_queue.bound_tenant`: one principal row whose `role_name` equals `session_user`. That lookup is not sufficient by itself when several services share a password or a role. A shared login, the queue runtime role, a pooler login, the owner, or a role with zero or more than one principal row is rejected. The call writes nothing.

Inside a `SECURITY DEFINER` function, `current_user` is the function owner. That value is never the caller and is never stored as a reviewer, a tenant, or a restart author. A payload field named `author`, `tenant`, `kind`, `reviewer`, `count`, `sequence`, or `tip` is not a privilege.

Connection reuse is part of the contract. A pool authenticates as the subject role for that request. It does not log in as a common user and then `SET ROLE`. When a connection is returned, the pool runs `DISCARD ALL`, so a prior `SET ROLE`, `search_path`, or session setting does not survive. If `session_user` after checkout is not the expected subject, the call fails closed.

## 4. Database Owner NOLOGIN

The table owner is `NOLOGIN`. It is not a connection string stored for the worker, the reviewer recorder, or the restart recorder. It is not a superuser and does not have `BYPASSRLS`. Those three roles and the auditor are not members of the owner. Grants are explicit and minimal: `EXECUTE` on one function for each writer, or tenant-scoped `SELECT` for the auditor. The owner is used by a migration run under a controlled operator session, not by the dry-run process.

## 5. Tenant Isolation and RLS

Journal rows that are tenant-scoped carry the tenant of the principal mapped from `session_user`. Row security is forced on direct table access. A `SECURITY DEFINER` function runs as the owner and would otherwise bypass that policy, so the function applies the same predicate itself: the tenant column is the mapped tenant, and every read inside the function repeats that predicate. An argument named `tenant` is ignored. Zero rows or more than one tenant for `session_user` raises before any write. The transaction rolls back.

The worker has no table `INSERT`. A claim goes through `append_worker_claim` under the worker's own principal. Worker `SELECT` returns that tenant's claim rows and that tenant's restart rows only. It does not return `kind=verdict` and it does not return another tenant. The worker learns that its own claim remains blocked from those rows. It does not learn another tenant's verdict by a missing-row error or by a global scan.

The reviewer recorder does not take Agent J from `session_user`. `session_user` only proves that the caller is the recorder role. The stored reviewer is the GitHub login the trusted workflow read from the review API for that pull request and exact SHA. If that read fails, the workflow does not call `append_reviewer_verdict`. A worker packet is never copied into the author column.

The auditor has `SELECT` in the tenant mapped to the auditor role and no write. `PUBLIC` has no `SELECT` and no `EXECUTE`. A global verdict row for a commit SHA is written only by the reviewer recorder. The worker cannot select or update it.

The restart budget for a tenant is the count the restart recorder computes from that tenant's restart chain. It is not a worker id, a file path, or a caller argument.

## 6. Append-Only Data Model

These are two independent chains, not one shared journal. Neither is created by this change.

Audit chain:

- `audit_event`: `audit_sequence bigint` primary key, `prev_hash`, `hash`, `kind` constrained to `claim` or `verdict`, body, tenant where the row is tenant-scoped, commit SHA where the row names a commit
- `audit_tip`: exactly one row, `audit_sequence` and `hash` of the current audit head

Restart chain:

- `restart_event`: `restart_sequence bigint` primary key, `prev_hash`, `hash`, `kind` constrained to `restart` or `hold`, tenant, server-computed count, server reason. The count, the sequence, the tenant, and the tip are not columns the caller may set.
- `restart_tip`: exactly one row, `restart_sequence` and `hash` of the current restart head

A row in one chain does not advance the other tip. `UPDATE`, `DELETE`, and `TRUNCATE` are revoked from the worker, the reviewer recorder, and the auditor on both event tables and both tip tables. There is no function that deletes or rewrites a negative verdict, and there is no function that replaces either chain with a new genesis.

## 7. Security-Definer Contract

Three functions, all owned by the `NOLOGIN` owner. Names are provisional: `append_worker_claim`, `append_reviewer_verdict`, and `append_restart_event`.

`append_worker_claim` inserts `kind=claim` only. The kind is fixed by the function, not by a client argument. It continues only when `session_user` is a worker role with exactly one principal row, and only when that role is the grantee of this function. It does not insert `kind=verdict` or a restart row. The tenant written is the mapped tenant.

`append_reviewer_verdict` inserts `kind=verdict` only. It continues only when `session_user` is the reviewer-recorder role. Before the insert, the trusted workflow must already have bound that call to one pull request, one exact head SHA, the successful checks for that SHA (`bintrans-autopilot-unit` and `bintrans-autopilot-queue`), and the GitHub login the workflow read. The function stores that login. It does not treat a payload field `reviewer=agent-j` as the author, and it does not treat `session_user` as Agent J. `current_user` inside the definer remains the owner and is never recorded as the reviewer.

`append_restart_event` is the only writer of the restart chain. `EXECUTE` belongs only to `restart_recorder`. The worker cannot grant that role to itself. The function takes no count, sequence, tenant, previous hash, or tip. It locks `restart_tip`, validates the stored hash against the chain head, rejects a replay, appends one event, advances that same tip, and commits. The count on the new row is computed from the trusted events for the mapped tenant. If that count is past the server budget, the appended kind is `hold` and the result is HOLD. A HOLD row is not deleted and is not replaced by a new genesis.

None of the three functions accepts a caller-selected previous hash. Direct `INSERT` on the tables is not granted. None of them sets `merge_authorized`. An undefined `session_user`, an undefined tenant, or a shared credential fails closed: the function raises and the gate stays `blocked`.

Hardening required of each write function, still not created here:

- `REVOKE ALL ON FUNCTION ... FROM PUBLIC`, then `GRANT EXECUTE` to exactly one role.
- `SET search_path` to a fixed list: `pg_catalog` and the audit schema. The function qualifies every table it writes.
- The worker, the reviewer recorder, and the restart recorder have no `CREATE` on `pg_catalog` or the audit schema, so they cannot plant an object earlier in the name search.
- The owner has no login, no superuser, and no `BYPASSRLS`. Callers are not members of the owner.

## 8. Atomic Sequence and Tip Hash

Each append locks its own tip row with `SELECT ... FOR UPDATE`, reads that chain's sequence and hash, rejects the write when the stored hash does not match the chain head, inserts the next row with that sequence plus one, and updates that same tip in the transaction. Audit callers serialize on `audit_tip`. Restart callers serialize on `restart_tip`. They do not share a lock or a sequence. A tail cannot be removed while its tip still names the removed row. Replacing either chain requires changing its tip, which no granted role can do.

For `append_restart_event` the order inside one transaction is fixed:

1. Lock `restart_tip`.
2. Validate the tip hash against the chain head. A shorter prefix that still hashes, a missing tip, or a tip that names a deleted row refuses the transaction.
3. Reject a replay. The same event for the current tip returns the existing row and does not insert a second one. A call that names an older tip is a stale replay and is rejected.
4. Append one `restart_event`. The count is the server value for the mapped tenant, not a caller argument.
5. Advance `restart_tip` to that new sequence and hash.
6. Commit.

Two supervisors serialize on the tip lock. Only one of them advances a given sequence. A crash before commit leaves the previous tip and does not consume a second count.

## 9. PostgreSQL Concurrency and Transaction Boundary

The append is one transaction. A crash before commit leaves the previous tip. A crash after commit leaves the new row and the new tip together. The function does not use a session-level advisory lock held by the worker as the integrity mechanism. Isolation level is at least `READ COMMITTED` with `SELECT ... FOR UPDATE` on the tip. The worker cannot set `row_security = off`.

## 10. Privileged DBA Threat

A database superuser or the table owner can still disable row security, drop the function, or rewrite rows. Role separation does not stop that person. The ADR therefore does not call the database alone a complete anchor against its own administrator.

## 11. External Anchor Outside Worker/DBA Control

Each chain tip is also published by the trusted workflow to an append-only log that the autopilot database administrator and the worker cannot rewrite. The service, the publication interval, and the SLA are `POLICY_DECISION_REQUIRED`. This ADR does not choose them.

A reviewer verdict is not eligible for the audit gate until the external log has an entry for that verdict's `audit_sequence` and hash. The gate reads the external tip before it accepts any database verdict. If the external entry is missing, the gate stays `blocked`. A database row alone is not enough.

The window between the database commit and the external publication is the residual risk. During that window the row exists in the database and must not be treated as a trusted verdict. A later publication closes the window for that sequence. A publication that skips a sequence, or an external hash that differs from `audit_tip`, refuses the gate.

The restart chain has its own external anchor. An audit-log entry does not stand in for `restart_tip`. A restart count from the database is not the trusted budget until the external restart anchor has that sequence and hash. A missing or different restart anchor refuses the restart. The service, the interval, and the SLA for this second anchor are the same `POLICY_DECISION_REQUIRED`. This ADR does not choose them.

The worker does not hold the signing key. Absence of the external tip is fail-closed, not an empty journal.

## 12. Crash Recovery and Reconciliation

Do not delete rows to recover. Do not truncate a local file to the last valid line and continue. After a crash:

1. The database transaction either contains the new row and the new tip, or neither.
2. A torn local file, if one is still used as a cache, stays refused.
3. Reconciliation re-reads the database tip and the external tip. A mismatch refuses further appends until an operator following this ADR restores from the external log. Restoration is not an automated delete.

The restart chain uses the same rule, in the order in section 8. A damaged tip or a prefix that no longer matches the tip holds the tenant. The hold is not replaced with a new zero count, and the chain is not replaced with a new genesis. After a crash the recorder reads `restart_tip` and the external restart anchor again. A mismatch refuses another restart.

## 13. Fail-Closed on Database Outage

If the store cannot be read, the tip transaction cannot commit, or the external restart anchor cannot be read, submit and restart accounting return a refusal. They do not fall back to a new local file and they do not reset the restart count to zero. An exhausted server-computed budget appends HOLD and does not allow another restart. `readiness-check` stays `REFUSED` when emergency stop is set, and a store outage does not enable autopilot, a worker, or a merge.

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
| Worker `EXECUTE` on `append_restart_event` | permission denied |
| Worker supplies a restart count, sequence, tenant, or tip | ignored; the function has no such argument; no row uses the supplied value |
| Worker `SET ROLE` to either recorder or the owner | permission denied |
| Two supervisors restart at the same tip | one commit; the other waits, then sees the new tip |
| Replay of an older restart event | rejected; the current tip does not move backward |
| Restart chain cut back to a valid prefix | refused; `restart_tip` no longer matches |
| Restart budget already exhausted | HOLD; the hold row remains |
| Replace a HOLD with a new restart genesis | permission denied; no such function |
| PostgreSQL unavailable during restart | refusal; count is not reset; no new local chain |
| External restart anchor missing or different | refusal; the database count is not the trusted budget |
| Shared database role used by two services | rejected; `session_user` alone is not a subject |
| Pool connection whose `session_user` is the pooler login | rejected; no write |
| `SET ROLE` left on a reused connection | `DISCARD ALL` on return; a surviving role change is still denied |
| `session_user` with no principal row, or with two tenants | fail closed; no row written |
| `PUBLIC` `EXECUTE` on any write function | permission denied |
| `PUBLIC` `SELECT` | permission denied |
| Auditor `EXECUTE` on any write function | permission denied |
| Auditor `SELECT` of another tenant | zero rows |
| Recorder `EXECUTE` on `append_worker_claim` or `append_restart_event` | permission denied |
| Search-path object placed by the worker | function still writes the audit schema; worker has no `CREATE` there |
| Cross-tenant insert under another tenant's principal | rejected; tenant comes from the principal map, not the payload |
| Worker `SELECT` of a verdict or of another tenant | zero rows |
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
2. Choose both external anchors, their operator, their publication interval, and their SLA. The audit anchor and the restart anchor stay `POLICY_DECISION_REQUIRED`.
3. Approve an autopilot-schema migration number after the head is read again.
4. Keep `AUTOPILOT_ENABLED`, worker execution, paid automation, and production 24/7 off.
5. Do not treat a stored verdict as merge or deploy authority.

Until those gates are accepted, Agent H does not add the tables, the roles, or a path that clears the finding gate.
