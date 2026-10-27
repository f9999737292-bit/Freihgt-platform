# Emergency stop

Use the fastest switch that is available. Any one of them prevents a new Cloud Agent create.

## Immediate

1. GitHub → Settings → Secrets and variables → Actions → Variables.
2. Set `AUTOPILOT_EMERGENCY_STOP` to `true`.
3. Set `AUTOPILOT_ENABLED` to `false`.

The next workflow evaluation skips the job when the enable variable is not `true`. If a run is already inside the program, `AUTOPILOT_EMERGENCY_STOP=true` returns before create.

## In git

Set `emergency_stop` to `true` in `tools/bintrans_autopilot/autopilot.config.json`, or add an empty file at `tools/bintrans_autopilot/EMERGENCY_STOP`, and merge that through the normal review path. Do not force-push.

## Runs that already started

The default stop does not cancel a Cursor run. Cancelling is a separate documented call, `POST /v1/agents/{id}/runs/{runId}/cancel`, and it is terminal. The program exposes `CursorClient.cancel_run` for a later controller tool. This phase does not invoke it.

After a cancel, the dispatcher moves the issue to `NEEDS_CONTROLLER` and does not retry.

## What not to do

- Do not delete `main`.
- Do not change branch protection in a hurry from this runbook.
- Do not rotate keys into the git history. Rotate them in the Cursor dashboard and update the GitHub secret.
- Do not mark blocked issues `READY` until the stop flag is back to off and the budget is understood.

## Check

A disabled or stopped dispatch prints `PAID_AGENTS_LAUNCHED=NO`. The unit test `test_emergency_stop_makes_no_cursor_call` covers the env flag with a fake client.
