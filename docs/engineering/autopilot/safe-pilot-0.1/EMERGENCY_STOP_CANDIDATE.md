# Emergency stop candidate

Status: proposed replacement for `docs/engineering/autopilot/EMERGENCY_STOP.md`. Not applied to `main`. Production settings are unchanged. Revision reason: J-DA-M002.

## Immediate stop

The immediate control is the repository variable `AUTOPILOT_EMERGENCY_STOP`, set by a trusted operator in GitHub → Settings → Secrets and variables → Actions → Variables.

- `true` blocks new dispatch.
- A missing, empty, or any other value also blocks new dispatch. The control is fail-closed when it cannot be read as the explicit operator value `false`.
- The process reads the variable from the Actions environment. It does not need the local checkout to learn the stop.
- Changing, adding, or deleting `tools/bintrans_autopilot/EMERGENCY_STOP` in Git is not an immediate stop. A merge of that file is not the procedure.

Setting `AUTOPILOT_ENABLED` to a value other than `true` also keeps the existing dispatcher job from starting. That is a separate switch. It does not replace the emergency variable.

The committed `emergency_stop` flag in `tools/bintrans_autopilot/autopilot.config.json` is a slower control. It takes effect only after the reviewed file is on the ref the job checks out. Use it when the change can wait for review. Do not force-push.

## What the stop does not do

Blocking the next dispatch does not cancel a Cursor run that has already started. For a future paid phase, a controller may call `POST /v1/agents/{id}/runs/{runId}/cancel`. That call is terminal. This candidate does not perform it, and the Phase A monitor has no Cursor credential with which to perform it.

## What not to do

- Do not treat a fresh checkout without the stop file as permission to start.
- Do not delete `main`.
- Do not change branch protection from this note.
- Do not put secrets into the stop file or the Git history.
