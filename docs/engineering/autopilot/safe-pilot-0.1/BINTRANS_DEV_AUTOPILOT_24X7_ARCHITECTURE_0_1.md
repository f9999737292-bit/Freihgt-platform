# BINTRANS development autopilot — architecture 0.1

Status: proposal, revision 0.2. This document does not enable autopilot, a worker, a paid call, a merge, or a deploy.

Revision 0.2 reason: J-DA-C002. Phase A must not reuse the dispatcher workflow. That workflow's enabled job requests contents, issues, and pull-requests write. The read-only monitor is a separate candidate file and is not on `main`.

Checked sources, read on 2026-11-25:

- `origin/main` = `5432e1444ea4ed607cc8992e967023b241315759` (`Merge pull request #409`).
- Draft PR #403 head = `a068ab6af64f14699304752956a192746dc23bf7`, base = `892361db87985f35132aef86550a025909af648a`. The PR is OPEN and draft. Its base is not current `main`.
- Issue #313 is OPEN.
- Cursor Cloud Agents API, public beta: https://cursor.com/docs/cloud-agent/api/endpoints
- Cursor Admin API: https://cursor.com/docs/account/teams/admin-api

## Decision

Extend the existing BINTRANS Autopilot. Do not create a second task system.

The GitHub Issue remains the task source. The PostgreSQL queue in schema `autopilot_queue` remains the execution lease. The dispatcher remains the only component allowed to call Cursor, and only after the gates in this document. Draft PR #403 adds a dry-run supervisor and a blocked audit gate. Those files are not on `main`. This pilot does not merge that PR.

Recommended shape: `BINTRANS_QUEUE_WITH_CURSOR_ADAPTER`. Cursor-native Automations are the comparison case, not the first implementation. The comparison is in `BINTRANS_DEV_AUTOPILOT_GITHUB_CURSOR_INTEGRATION.md`.

## What already exists

On `main`:

- Dispatcher, GitHub client, and Cursor client. `autopilot_enabled` is `false`. Budget caps in `autopilot.config.json` are `0`. `pricing.source` is `unverified`.
- Workflow `.github/workflows/bintrans-autopilot-dispatcher.yml` runs a job only when repository variable `AUTOPILOT_ENABLED` is exactly `true` and the ref is `main`. Pull-request events are excluded.
- PostgreSQL queue through migration `0015`, with tenant principals, leases, and a budget protocol.
- Dry-run worker. It is not a resident process. `mode` must be `dry_run`. A job payload is not executed.
- Model policy. `unpaid_model_reason` returns `unknown_cost` unless pricing comes from a verified pricing API and the choice is `VERIFIED`. `require_distinct_provider: true` raises `provider_unverified`. The committed config leaves that flag `false`, so the provider check is not armed.
- Finance classification uses exact service prefixes (`services/billing-register-service/` and the sibling finance services), not a search for the substring `billing` inside an arbitrary path.
- Issue states in `states.py`: `BACKLOG`, `READY`, `CLAIMED`, `IMPLEMENTING`, `PR_OPEN`, `CI_CHECK`, `INDEPENDENT_QA`, `READY_FOR_MERGE`, `MERGED`, `DONE`, `BLOCKED`, `NEEDS_CONTROLLER`, `FAILED`, `BUDGET_HOLD`. `READY_FOR_MERGE` can move only to `NEEDS_CONTROLLER` or `BLOCKED`. The program does not merge.
- Emergency stop on `main` is the Actions variable, the committed config flag, and a local file. The `main` runbook still says to merge the file `tools/bintrans_autopilot/EMERGENCY_STOP`. That delivery path is unsafe because a later draft ignores the file. Remote delivery of the file is not implemented. The variable path is the one the workflow actually passes into the process.

Only on draft PR #403, inspected at `a068ab6a`, not re-tested in this task:

- `audit_gateway.py` treats packet `reviewer` as untrusted and returns `trust_boundary_absent`. `reviewer_authenticated` stays false.
- Supervisor, local hash-chain journal, and the trust-storage ADR. The ADR does not authorize tables, roles, or a clear path.
- No trusted GitHub App identity for Agent J is implemented. `INDEPENDENT_REVIEW_GATE` stays `BLOCKED`.

`AP04A-J-001` asked that a forged `reviewer=agent-j` cannot grant a pass. On current `main` that gate does not exist. On the draft head the forged string does not clear the gate, and a real independent identity still does not exist. Neither statement was re-proven by running the suite in this task. Source inspection is the evidence.

## Target flow

```text
GitHub Issue
→ controller authorization event
→ existing PostgreSQL queue
→ budget gate
→ security gate
→ agent selection
→ Cursor Cloud Agent, only when a later phase allows the call
→ dedicated branch
→ unit and integration tests
→ draft pull request
→ GitHub CI
→ Agent J, trusted actor only
→ controller merge decision
→ completion
→ next eligible issue
```

Nothing in this flow writes to `main` except a later human controller action outside the agent.

## Status model

The names below are the pilot vocabulary. They map onto the states already in `states.py` where that state exists. New names are not a second store.

| Pilot status | Existing state | Meaning |
| --- | --- | --- |
| `BACKLOG` | `BACKLOG` | Recorded, not eligible |
| `READY_FOR_CONTROLLER` | `NEEDS_CONTROLLER` | Waiting for a controller event |
| `APPROVED` | no automatic `READY` | Controller event accepted for one base SHA |
| `QUEUED` | queue `PENDING` | Lease not held |
| `RUNNING` | `CLAIMED` / `IMPLEMENTING` | One lease holder |
| `PR_OPEN` | `PR_OPEN` | Draft pull request only |
| `CI_PENDING` | `CI_CHECK` | Required checks not yet green for that SHA |
| `SECURITY_REVIEW` | `INDEPENDENT_QA` | Review requested; gate stays blocked until trusted identity |
| `CONTROLLER_REVIEW` | `READY_FOR_MERGE` | Controller may merge. The agent may not |
| `COMPLETED` | `DONE` | After the controller merge, not after a green CI run |

Additional states stay fail-closed: `BLOCKED`, `FAILED_RETRYABLE`, `FAILED_FINAL`, `BUDGET_EXHAUSTED` (today `BUDGET_HOLD`), `CANCELLED`, `SECURITY_HOLD`.

`READY` on `main` is launchable once the enable flags are on. This pilot must not treat `READY`, a label named approved, or a comment as `APPROVED`.

## Task fields

The Issue carries the contract. The execution authority is the controller event plus the queue row.

Required fields: `TASK_ID`, `ISSUE_NUMBER`, `PRIORITY`, `OWNER_AGENT`, `SCOPE`, `ALLOWED_PATHS`, `FORBIDDEN_PATHS`, `EXPECTED_RESULT`, `ACCEPTANCE_CRITERIA`, `DEPENDENCIES`, `RISK_LEVEL`, `APPROVAL_ID`, `APPROVED_HEAD_OR_BASE`, `MAX_COST_USD`, `MAX_RETRIES`, `TIMEOUT`, `REVIEWER_REQUIRED`.

Text inside the Issue can supply those fields for display. It cannot authorize them. A prompt in the body cannot widen `ALLOWED_PATHS` or set `AUTOPILOT_ENABLED`.

## Controller event

An accepted authorization is an event whose actor is the trusted controller principal, whose `approval_id` is unique, and whose base SHA equals the SHA the queue row will use. The event has an expiry. A label edit, an issue comment, or a body line is not that event.

If the head moves after approval, the approval is stale and dispatch is denied.

## After completion

The scheduler selects the next issue that is approved for the current head, whose dependencies are `DONE`, whose paths do not overlap an active lease, and whose budget and security gates allow it. The selection rules are in the scheduler document. Two development agents and one review agent are the proposed starting caps. They are not enabled here.

## Phase A boundary

Phase A is a read-only report of Issues, pull requests, and Actions run conclusions. Its candidate workflow is `bintrans-autopilot-monitor.yml`. Triggers: `workflow_dispatch` only. Permissions: contents read, actions read, issues read, pull-requests read. No schedule, no Cursor secret, no `pull_request_target`, no self-hosted runner. The file is not published by this revision. Adding it to the default branch would make `workflow_dispatch` available, so that publish waits for a separate controller GO.

The monitor denies unless `AUTOPILOT_EMERGENCY_STOP` is the explicit operator value `false`. It does not grant `APPROVED`, does not write to GitHub, and does not call Cursor.

## What this pilot refuses

- A second queue.
- A path from CI success to merge.
- A path from the text `reviewer=agent-j` to a clear gate.
- Paid Cursor, Claude, or DeepSeek calls.
- Production, live ЭДО, payments, and customer secrets.
