# Failure recovery

Status: design on top of the queue and dispatcher that already exist on `main`. No retry is enabled.

## Rules that already exist

The PostgreSQL queue uses a lease. A lost lease cannot complete the row. An expired lease can be claimed again. Two workers do not both receive the same job. The dry-run worker is not resident: a caller invokes one pass. A payload is not executed. `release_hold` on the draft supervisor does not clear a stored hold. That supervisor is not on `main`.

A Cursor cancel is terminal: `POST /v1/agents/{id}/runs/{runId}/cancel`. The existing runbook says this phase does not call it. A cancelled run is not resumed; a new run would be a new authorization.

Revision 0.2 reason: J-DA-M002. Refusing a new dispatch does not cancel an agent that is already running. That cancel is a later controller action for a paid phase. It needs a Cursor credential this phase does not hold. The emergency stop variable is read from the operator control, not from a file in the checkout. If that control value is missing, new dispatch is denied.

## Idempotency

GitHub deliveries carry an id. The scheduler stores that id with the queue insert. A second delivery of the same id does not insert a second row and does not reserve the budget twice. Editing an Issue is not a new approval.

## Retries

`MAX_RETRIES` is fixed on the approved task. A retry increments the attempt and keeps the same path allow-list, the same cost cap, and the same base SHA. Backoff is the existing supervisor backoff once that code is on `main`. This pilot does not turn a retry into a resident loop.

When the attempt count reaches the cap, the status becomes `FAILED_FINAL`. The agent cannot raise the cap.

## Specific failures

| Condition | Result |
| --- | --- |
| Lease expires or the agent heartbeat is gone | Row returns to claimable only if attempts remain. Otherwise `FAILED_FINAL` |
| Pull request idle past `TIMEOUT` | `BLOCKED`. No merge |
| Operator cancel | `CANCELLED`. Reserve is released only after the usage feed shows no further charge, or stays held if the feed is missing |
| Budget exhausted or unknown charge | `BUDGET_EXHAUSTED`. New dispatch stops |
| Head moved | `stale_head`. Approval does not carry forward |
| CI failure | Completion denied. The implementation agent may use a remaining retry. The review does not start |
| Reviewer payload without the trusted actor | Rejected. `SECURITY_HOLD` stays |
| Cursor or GitHub API unreachable | Deny the step. Do not assume success. Do not open a local substitute agent |
| Process restart | In-memory progress is discarded. The queue lease and `SECURITY_HOLD` remain. A restart does not delete a hold or reset the budget |

## What a retry cannot change

Paths, risk level, cost cap, reviewer requirement, and the approved SHA stay at the authorized values. A failure report that asks for wider paths is prompt injection and is ignored.
