# Emergency stop runbook 0.5E

This runbook is the operator sequence for a future isolated proof. It does not start a workflow, and it does not give the monitor token any new permission. Changing `vars.AUTOPILOT_EMERGENCY_STOP` has not been shown to stop a step that has already started. The in-flight control is cancellation by a trusted operator.

## Three different controls

| Control | What it covers | What it does not cover |
| --- | --- | --- |
| Exact `false` check | The next run exits 2 when the variable is missing, `true`, or any value other than `false`, before checkout and again before GET. | A step whose environment was already captured. |
| Operator cancellation | The Actions UI cancel of each queued or in-progress monitor run. | A new run that someone dispatches after the cancel. |
| Dispatch lock in the budget ledger | A second dispatch is denied while one run is non-terminal. | This lock is not implemented. The workflow concurrency group can queue a run instead of rejecting it. |

`STOP_NEW_RUNS_GUARANTEE=NEXT_RUN_EXITS_2_UNLESS_VALUE_IS_EXACT_FALSE`. That is not a lock against a person pressing dispatch. `STOP_IN_FLIGHT_GUARANTEE=NOT_PROVEN_OPERATOR_CANCEL_ONLY`.

## Sequence

1. Deny new dispatches in the budget ledger. If the ledger is not deployed, the operator writes no further dispatch and treats any extra dispatch as an incident.
2. Set `AUTOPILOT_EMERGENCY_STOP` to `true`. This is for the next run only.
3. List every queued and in-progress run of the monitor workflow in the isolated repository. Do not use the monitor job token to list or cancel production runs.
4. A trusted operator cancels each of those runs in the Actions UI. The monitor token is not given cancel permission.
5. Refresh each run until its status is completed and its conclusion is cancelled, failure, or success. A success after a stop order is an incident, not a pass.
6. List runs again and confirm that no new monitor run was created after step 1.
7. Record the run ids, the cancel times, and the terminal conclusions. Do not copy tokens or raw API bodies into the incident note.
8. Leave the variable at `true` and leave dispatch denied until a separate controller GO says to continue.

## Future TOCTOU control, not implemented

The candidate reads the variable into the step environment when the step starts. The classify step does not read it again between jobs pages. A later change could, between pages, GET a repository file with the `contents: read` permission the candidate already has, and exit 2 unless that file says exactly `false`. That shrinks the window to one page. It does not stop a `gh api` process that has already started, and it does not replace operator cancellation. This stage does not edit the workflow. `STOP_TOCTOU_REMEDIATION_PROPOSAL=CONTENTS_READ_STOP_FILE_BETWEEN_PAGES_PLUS_OPERATOR_CANCEL`.
