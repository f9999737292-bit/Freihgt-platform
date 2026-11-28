# Emergency stop

Use a switch this repository already implements. Any one of those switches prevents a new Cloud Agent create. The ignored file `tools/bintrans_autopilot/EMERGENCY_STOP` is a local marker only.

## Implemented now

1. GitHub → Settings → Secrets and variables → Actions → Variables.
2. Set `AUTOPILOT_EMERGENCY_STOP` to `true`.
3. Set `AUTOPILOT_ENABLED` to `false` when the job should be skipped before a runner starts.

`.github/workflows/bintrans-autopilot-dispatcher.yml` starts `dispatch` and `daily-report` only when `vars.AUTOPILOT_ENABLED` is exactly `true` and the ref is `main`. The job condition does not read `AUTOPILOT_EMERGENCY_STOP`. Both jobs pass that repository variable into the process. `runtime_enabled` returns before the program builds a Cursor client. `Dispatcher.run` checks the same gate before it handles an issue. `request_reserve` checks the kill switch again before a budget reserve.

A committed `emergency_stop: true` in `tools/bintrans_autopilot/autopilot.config.json` is read by the same gate after checkout of `main`. That file is tracked. Changing it goes through review. Do not force-push. `autopilot_enabled: false` in that file keeps dispatch off even when the Actions variable is `true`.

On a machine that already has the process, a local file `tools/bintrans_autopilot/EMERGENCY_STOP` also makes the gate stop. An unreadable config or an unreadable marker is fail-closed. The file is listed in `.gitignore`.

## Not a remote control

Remote delivery of `tools/bintrans_autopilot/EMERGENCY_STOP` is NOT_IMPLEMENTED. Git does not stage an ignored file, so an add of the work tree does not publish the marker, and a merge does not carry it. A fresh checkout does not contain the marker. Absence of the ignored marker in a fresh checkout is not evidence that a launch is allowed. Both enable flags are still required, and either implemented switch above still stops a launch.

## Future remote execution

A later remote worker needs a server-owned emergency stop outside the checkout. The server reads that source before the first action and again before each action it would otherwise allow. If the source cannot be read, the result is fail-closed: no launch, no reserve, and no restart. This paragraph does not add that source.

## Runs that already started

The default stop does not cancel a Cursor run. Cancelling is a separate documented call, `POST /v1/agents/{id}/runs/{runId}/cancel`, and it is terminal. The program exposes `CursorClient.cancel_run` for a later controller tool. This phase does not invoke it.

After a cancel, the dispatcher moves the issue to `NEEDS_CONTROLLER` and does not retry.

## What not to do

- Do not delete `main`.
- Do not change branch protection in a hurry from this runbook.
- Do not rotate keys into the git history. Rotate them in the Cursor dashboard and update the GitHub secret.
- Do not mark blocked issues `READY` until the stop flag is back to off and the budget is understood.
- Do not treat a missing ignored marker as permission to start.

## Check

A disabled or stopped dispatch prints `PAID_AGENTS_LAUNCHED=NO`. The unit test `test_emergency_stop_makes_no_cursor_call` covers the env flag with a fake client.
