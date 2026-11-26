# BINTRANS autopilot monitor design 0.4

Stage `DEV-AUTOPILOT-24X7-PHASE-A-MONITOR-PREFLIGHT-0.4`. Local prototype only. This directory is not a git commit and is not a pull request.

## Baseline on origin/main `c194da0e4943668e66953407eacd87f438e05c0b`

| Component | Class | Evidence |
| --- | --- | --- |
| Dispatcher workflow `.github/workflows/bintrans-autopilot-dispatcher.yml` | `IMPLEMENTED_MAIN` | Present. Triggers include schedule, workflow_dispatch, and issues. Dispatch job permissions are `contents: write`, `issues: write`, `pull-requests: write`. Jobs run only when `vars.AUTOPILOT_ENABLED == 'true'`. |
| `autopilot.config.json` | `IMPLEMENTED_MAIN` | `autopilot_enabled` false. `emergency_stop` false. All budget caps 0. `pricing.source` unverified. `require_distinct_provider` false. `cursor_plan` empty. |
| `workflow_guard.py` | `IMPLEMENTED_MAIN` | Rejects `pull_request` and `pull_request_target`. Trusts only `bintrans-autopilot-dispatcher.yml` on `main` of `f9999737292-bit/Freihgt-platform`. Does not read Cursor or GitHub credentials. |
| PostgreSQL queue SQL `0001` through `0015` | `IMPLEMENTED_MAIN` | Schema files are on main. Applying them to a database was not run. |
| `budget_controller.py` | `IMPLEMENTED_MAIN` | Kill switch denies when caps are 0 (`zero_budget`), when `AUTOPILOT_EMERGENCY_STOP` is `true`, or when the stop file or config flag is set. A live PostgreSQL proof was not run. |
| `models.unpaid_model_reason` | `IMPLEMENTED_MAIN` | Returns `unknown_cost` unless pricing source is `verified_pricing_api` and cost status is `VERIFIED`. |
| Distinct provider gate | `IMPLEMENTED_MAIN` code, flag off | `require_distinct_provider: true` raises `provider_unverified`. The committed flag is false, so the gate is not armed. |
| Product emergency stop | `IMPLEMENTED_MAIN` | `runtime_enabled` stops when the env value lowercases to `true`, or when the config flag or `EMERGENCY_STOP` file exists. Missing env does not by itself stop the product dispatcher. |
| Monitor candidate | `IMPLEMENTED_LOCAL` | This directory. Stricter stop: only the exact string `false` allows a run. |
| Monitor workflow YAML | `IMPLEMENTED_LOCAL` | Unpublished candidate. `workflow_dispatch` only. |
| `audit_gateway.py`, `supervisor.py` | `DRAFT_ONLY` | Present at PR #403 head `a068ab6a`. Absent from `origin/main`. Not imported here. |
| PR #411 documentation pack | `DRAFT_ONLY` | Head `0755e2f29c92a96fdf14c4444aa8e3d815c17619`. Not modified. |
| PR #403 readiness branch | `DRAFT_ONLY` | Head `a068ab6a`, base `892361db`. Not rebased. |
| Issue #313 | `IMPLEMENTED_MAIN` state is open | Title remains the 0.2B cost and provider gate. `updatedAt` `2026-10-27T20:15:09Z`. |
| Offline dry-run simulator from Phase A 0.2 | `SIMULATED_ONLY` | Not copied into this directory and not treated as product proof. |
| Live monitor GitHub client | `NOT_IMPLEMENTED` | `LiveGitHubTransport.load` raises `live_github_not_authorized` before any socket. |
| Schedule, Cursor calls, paid spend, deployment | `NOT_IMPLEMENTED` | No authorization in this stage. |
| NLO or embedded-postgres repair | `NOT_IMPLEMENTED` | Out of scope. |

## Shape

Two local pieces stay apart from the write-capable dispatcher:

1. `candidate/.github/workflows/bintrans-autopilot-monitor.yml` is the future read-only shell. It is not on `main` and not in PR #411.
2. `src/bintrans_monitor/` classifies fixture payloads. It does not import `tools/bintrans_autopilot`.

The workflow does not check out a pull request and does not start the local package. The package is not on `main`. A later publication GO would have to add both, still without using the dispatcher workflow.

## Report

`run_monitor` returns a report with `exit_code`, `ci_result`, job observations, grouped signatures, draft pull-request metadata, and counters that stay at zero for actions, Cursor calls, GitHub mutations, and retries.

| Exit | Meaning |
| --- | --- |
| 0 | Every observed job concluded `success`. |
| 1 | At least one job is `failure`, `cancelled`, or `timed_out`. The report does not relabel that job as success. |
| 2 | Emergency stop denied the run. No fixture was read. |
| 3 | Repository, SHA, network, or live-client refusal. `ci_result` is `UNKNOWN` or `ERROR`, never `SUCCESS`. |
| 4 | Reserved for `SecurityDeny`. Mutation verbs raise before a report is treated as success. |

A successful monitor process is not a successful CI run. Exit 0 is used only when every observed conclusion is `success`.

## Emergency stop

The monitor reads `AUTOPILOT_EMERGENCY_STOP` on every call. `StopSource` does not cache an allow.

| Value | Result |
| --- | --- |
| exact `false` | Monitoring may continue. |
| `true` | Stop. |
| missing `None` | Stop. |
| any other string, including `False` | Stop as `invalid`. |
| unreadable | Stop, even if a previous value was `false`. |

This is stricter than `runtime_enabled` on main, which treats only a lowercased `true` as the env stop and does not treat a missing variable as a stop. The product function was not changed.

Stopping this monitor does not cancel a Cursor agent that is already running. No cancel call exists in this package.

The Git file `tools/bintrans_autopilot/EMERGENCY_STOP` is not the control for this monitor. The real GitHub Actions variable was not changed.

## Cost contract for a later paid stage

These are future deny rules. This prototype does not call a model and does not authorize spend.

| Rule | Value |
| --- | --- |
| `MONTHLY_TARGET_USD` | 500 |
| `CURRENT_PAID_SPEND_AUTHORIZATION` | 0 |
| `UNKNOWN_COST` | `DENY` |
| `UNVERIFIED_PROVIDER` | `DENY` |
| `BUDGET_EXHAUSTED` | `DENY` |

On main today the monthly cap is 0 cents, so `budget_controller` returns `zero_budget` before any paid launch. `unpaid_model_reason` already returns `unknown_cost` for unverified pricing. A Cursor Ultra subscription is not a Cloud Agent API authorization. `cursor_plan` in the committed config is empty.

## Draft pull requests

The NLO fixture records PR #411 and PR #403 as `draft: true`, matching the read-only `gh pr view` baseline of this stage. The monitor copies that field into the report and has no verb that edits it.

## R2 release candidate

`J-DA-R2-M001` is `CLOSED_INVALID_REQUIREMENT`. `metadata: read` is not added.

`J-MON-M001` is closed in this candidate by deleting `issues: read`. The workflow GETs `/actions/runs` and `/pulls` only, so the granted keys are `actions: read` and `pull-requests: read`. `contents: read` is not granted because the file does not call a contents endpoint and does not check out code.

`run_monitor` reads the stop signal before `transport.load`. A denied stop leaves the transport load count at 0. The workflow has one job, no `continue-on-error`, no `always()`, and no `uses:` step before or after the stop.

## R3 draft preparation

Current `origin/main` is `34a328da00a646806caeb5f2294e9de065dacb02`, the merge of PR #412. That merge changes `services/rfx-service` only. It does not change `tools/bintrans_autopilot` or `.github/workflows/bintrans-autopilot-dispatcher.yml`.

PROTOTYPE ONLY. The workflow fetches GitHub data but does not execute the Python classifier. Successful workflow execution does not prove successful project CI.

`J-MON-R2-M001` stays OPEN. `WORKFLOW_CLASSIFIER_INTEGRATION=NOT_IMPLEMENTED`.

`J-MON-R2-M002` is corrected in the test report: Agent J's five product checks used an archive of `origin/main` `c194da0e4943668e66953407eacd87f438e05c0b`. The PR #411 worktree is not described as byte-identical to that commit.

`J-MON-R2-M003` stays OPEN. `HOSTED_PERMISSION_ENFORCEMENT=NOT_RUN`.

This prototype is not production-ready. The candidate YAML remains under `candidate/.github/workflows/` and is not installed as a repository workflow.

## 0.5a classifier integration

The inactive candidate now classifies one `workflow_dispatch` run id. Emergency stop still runs first. The shell then checks the trusted repository and `refs/heads/main`, checks out `github.sha` with a pinned checkout action and `persist-credentials: false`, GETs that run and its job pages into a temporary directory, and runs `python3 -m bintrans_monitor.cli`. The CLI uses `github_adapter.assemble` and the existing `run_monitor` engine. Printed rows keep only the sanitized fields. Raw API JSON is not written to the Actions log. The candidate is still not a repository workflow, and a local test is not hosted permission enforcement.

## 0.5B false-success remediation

Exit 0 requires a completed run whose `conclusion` is `success`, a complete job set, and a success conclusion on every job of the latest confirmed `run_attempt`. Job identity is `job.id` plus `run_id` plus `run_attempt`. The same `name` on two ids does not collapse the rows. A missing id, a conflicting id, a missing attempt, or an incomplete page is `UNKNOWN` or `ERROR`, never `SUCCESS`.

The adapter does not treat a Jobs API body as a log. Known signatures remain local-fixture behavior. `J-0.5A-M002` is not fully closed.
