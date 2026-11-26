# Phase A publication plan

Nothing in this plan is executed here. PR #411 is not edited. The monitor workflow is not added to that documentation pull request.

## Future workflow pull request, after a separate GO

Artifacts that would have to be reviewed again before publication:

| Path in this directory | Role |
| --- | --- |
| `candidate/.github/workflows/bintrans-autopilot-monitor.yml` | Unpublished workflow candidate. |
| `src/bintrans_monitor/` | Read-only classifier. Not on `main`. |
| `fixtures/` | Local inputs, including the synthetic second NLO run. |
| `tests/test_monitor.py` | MON-001 through MON-015, plus the R2 permission and stop-gate tests. |
| `docs/BINTRANS_AUTOPILOT_MONITOR_SECURITY_0_4.md` | Permission and stop contract. |
| `docs/BINTRANS_AUTOPILOT_MONITOR_CI_CLASSIFICATION.md` | Failure labels. |
| `MANIFEST.sha256` | Hashes of this local tree. |

Publication still requires:

- a controller GO that names the workflow file;
- `workflow_dispatch` only, unless that GO explicitly adds another trigger;
- the emergency-stop step before any `gh api` call;
- no checkout of a pull-request head;
- no use of `.github/workflows/bintrans-autopilot-dispatcher.yml`;
- `J-DA-R2-M001` is `CLOSED_INVALID_REQUIREMENT`. Do not add `metadata: read`.
- `J-MON-M001` is addressed in this candidate by removing `issues: read`. `contents: read` is also absent because the GET calls do not use it.
- no change to the live `AUTOPILOT_EMERGENCY_STOP` variable inside the documentation PR.

Merging the workflow onto the default branch would make `workflow_dispatch` runnable. That is a later GO, not this stage.

## R3 draft pull request preparation

The prepared tree is `tools/bintrans_autopilot_monitor/` on `feat/bintrans-autopilot-monitor-draft-v0.4`, base `34a328da00a646806caeb5f2294e9de065dacb02`. The YAML stays at `candidate/.github/workflows/bintrans-autopilot-monitor.yml` inside that tree. It is not copied to the repository `.github/workflows/` directory, so this draft does not register or publish a workflow.

PROTOTYPE ONLY. The workflow fetches GitHub data but does not execute the Python classifier. Successful workflow execution does not prove successful project CI.

Carried limits:

- `J-MON-R2-M001=OPEN`. `WORKFLOW_CLASSIFIER_INTEGRATION=NOT_IMPLEMENTED`.
- `J-MON-R2-M003=OPEN`. `HOSTED_PERMISSION_ENFORCEMENT=NOT_RUN`.
- `J-MON-R2-M002` is a documentation correction in the test report, not a claim that the monitor is production-ready.

PR #411 and PR #403 are not edited. PR #403 remains a stale draft. PR #412 is already merged and is the current main. Issue #313 stays open.

## Before any paid mode

Paid calls stay denied until all of the following are true and separately authorized:

| Dependency | Current evidence |
| --- | --- |
| Issue #313 | `OPEN`. Cost and provider gates are not closed. |
| `pricing.source` | `unverified` on main. `UNKNOWN_COST=DENY`. |
| `require_distinct_provider` | false on main. Code can deny when the flag is true. The flag is not armed. |
| Budget caps | 0 on main. `BUDGET_EXHAUSTED` and `zero_budget` deny. |
| `autopilot_enabled` and `AUTOPILOT_ENABLED` | Config false. Dispatcher jobs require the variable `true`. This stage sets neither. |
| Cursor Ultra or any local subscription | Not spend authorization. `CURRENT_PAID_SPEND_AUTHORIZATION=0`. |
| `audit_gateway.py` and `supervisor.py` | Draft PR #403 only. Need their own audit before they are considered for main. |
| PR #403 base | `892361db`, stale relative to `c194da0e`. No rebase and no merge in this stage. |
| Trust storage and webhook receiver | Documented in the Phase A pack. Not implemented on main. |
| Independent review gate | Still blocked on main. |
| Emergency stop variable | Not modified. Monitor allows a run only for exact `false`. |

`MONTHLY_TARGET_USD=500` is the future ceiling to design against. It is not a cap that this prototype turns on.
