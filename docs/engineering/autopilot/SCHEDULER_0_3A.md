# Scheduler 0.3A

Phase 0.3A adds a planning queue beside the existing GitHub Issue queue. The Issue states in `states.py` are unchanged. The dispatcher still refuses to start unless `autopilot_enabled` is true and the repository variable `AUTOPILOT_ENABLED` is the string `true`. This phase does not change either switch.

`tools/bintrans_autopilot/task_queue.py` stores tasks in a JSON file. Claim and recovery run under a single-host OS file lock so two local schedulers cannot take the same task. The lock file is not deleted to steal a live holder, and a timestamp in that file does not transfer ownership. When a holder process exits, the OS releases the lock. This is not an NFS lock and it is not a multi-host lease. Distributed 24/7 workers are a later phase.

Writes go to a temporary file in the same directory, then `fsync`, then an atomic replace. A backup copy is refreshed after that replace. A read does not rewrite the queue. If the primary file is corrupt and the backup is valid, readers use the backup and leave the corrupt bytes in place. A later successful write moves those bytes aside once and does not overwrite an existing preserved copy. If neither file is valid, the store fails closed. Directory `fsync` runs only where the OS provides it. Windows durability is the file `fsync` plus a same-directory replace.

States:

```text
CREATED → READY → CLAIMED → RUNNING → REVIEW_REQUIRED → COMPLETED
```

Side states: `BLOCKED`, `FAILED`, `CANCELLED`. Every move is appended to the task history and to the store journal.

`tools/bintrans_autopilot/scheduler.py` runs only in `DRY_RUN`. A live mode value is rejected. The scheduler does not import the Cursor client. It does not merge, deploy, or write staging or production. `python -m bintrans_autopilot queue-dry-run` prints the closed switches and returns before any Cursor or GitHub client is constructed.

The trusted workspace root is the checkout that contains `tools/bintrans_autopilot/autopilot.config.json` and a `.git` file or directory. Local runs take that path from the scheduler module. They do not use the process current directory. In GitHub Actions, `GITHUB_WORKSPACE` is accepted only when `GITHUB_ACTIONS=true` and its canonical path is that same checkout. A different value leaves the root unavailable. An unavailable root blocks the task. Pull-request workflows still stop earlier with `UNTRUSTED_WORKFLOW` and do not reach this scheduler.

A task is blocked, and not claimed, when any of these are true:

- the assigned agent is outside A–I
- the base SHA, acceptance criteria, or allowed paths are missing or outside the committed path catalog
- a path is absolute, a drive path, a UNC path, or contains `.`, `..`, or an empty component; prefixes are matched by path components, not by string prefix
- a path is on the hard-forbidden list, checked before the allowlist
- the workspace root is empty, relative, or not an existing directory
- a path, or any parent component, resolves through a symlink or junction outside the canonical workspace root
- a requested action is not on the explicit dry-run allowlist; that allowlist is empty, so merge, deploy, `cloud_agent_create`, `production_change`, and unknown actions are refused and not executed
- `max_budget_cents` is null or negative
- the task requires a Cloud Agent

A configured launch cap of `0` blocks a paid task with `paid_budget_zero`. A missing observed charge is `null`. It is not stored as `0`. When the cap is positive and no charge has been read, the reason is `unknown_spend`. The first pilot run is recorded in `tools/bintrans_autopilot/usage_journal.json`. That record is not a launch balance. A later run with no recorded charge stays unknown.

An expired task lease on `CLAIMED` or `RUNNING` returns the task to `READY`. Each claim stores a new `lease_generation`. Only the scheduler that observed that generation can finish the task. A previous owner, including one that reused the same scheduler id, cannot finish it after a new claim. A missing lease on an active task becomes `BLOCKED`.

`COMPLETED` and `CANCELLED` do not change status. A late CI failure after `SUCCESS` then `COMPLETED` keeps `COMPLETED`, records the failure in the journal and task history, and sets `ci_confirmation` to `LOST`. That does not reopen the task, restore a pass, or permit merge or launch. A failed or unknown CI result cannot move a task to `COMPLETED` and is not recorded as `PASS`.

The scheduler does not call Cursor. A missing charge stays null and is not written as zero. The usage journal is a historical record, not a launch authorization.
