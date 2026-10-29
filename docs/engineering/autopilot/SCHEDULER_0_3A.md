# Scheduler 0.3A

Phase 0.3A adds a planning queue beside the existing GitHub Issue queue. The Issue states in `states.py` are unchanged. The dispatcher still refuses to start unless `autopilot_enabled` is true and the repository variable `AUTOPILOT_ENABLED` is the string `true`. This phase does not change either switch.

`tools/bintrans_autopilot/task_queue.py` stores tasks in a JSON file. Claim and recovery run under an exclusive lock so two schedulers cannot take the same task. The file is local operator state. It is not a second product database and it is not read by the paid dispatcher.

States:

```text
CREATED → READY → CLAIMED → RUNNING → REVIEW_REQUIRED → COMPLETED
```

Side states: `BLOCKED`, `FAILED`, `CANCELLED`. Every move is appended to the task history and to the store journal.

`tools/bintrans_autopilot/scheduler.py` runs only in `DRY_RUN`. A live mode value is rejected. The scheduler does not import the Cursor client. It does not merge, deploy, or write staging or production. `python -m bintrans_autopilot queue-dry-run` prints the closed switches and returns before any Cursor or GitHub client is constructed.

A task is blocked, and not claimed, when any of these are true:

- the assigned agent is outside A–I
- the base SHA, acceptance criteria, or allowed paths are missing or outside the committed path catalog
- a path is on the hard-forbidden list
- a requested action is merge, deploy, staging, production, or production secrets
- `max_budget_cents` is null or negative
- the task requires a Cloud Agent

A configured launch cap of `0` blocks a paid task with `paid_budget_zero`. A missing observed charge is `null`. It is not stored as `0`. When the cap is positive and no charge has been read, the reason is `unknown_spend`. The 0.2E pilot charge remains unknown and is not used as a zero balance.

An expired lease on `CLAIMED` or `RUNNING` returns the task to `READY`. The previous owner cannot finish it. A missing lease on an active task becomes `BLOCKED`. A failed or unknown CI result cannot move a task to `COMPLETED` and is not recorded as `PASS`.

The first Cloud Pilot usage record is still unavailable: no `CURSOR_API_KEY` was present for a read-only usage call. That absence is not a zero-cost result.
