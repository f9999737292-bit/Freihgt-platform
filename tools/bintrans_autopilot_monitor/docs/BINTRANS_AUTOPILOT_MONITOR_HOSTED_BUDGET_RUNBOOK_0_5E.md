# Hosted budget control runbook 0.5E

This runbook defines the only admission rule for a future isolated hosted proof. It does not dispatch a workflow. A written checklist is not a technical limiter. `J-MON-J-B001` stays open until an independent review accepts this design. The gate below is not implemented.

## Caps

| Cap | Value |
| --- | --- |
| `MAX_TOTAL_ACTIONS_MINUTES` | 60 |
| `MAX_SINGLE_RUN_MINUTES` | 10 |
| `MAX_CONCURRENT_RUNS` | 1 |
| `PARALLEL_DISPATCH_ALLOWED` | NO |
| `AUTOMATIC_RETRY_ALLOWED` | NO |
| `PAID_AI_BUDGET` | 0 |

`PAID_AI_BUDGET=0` is a deny. It is not a balance that a later run may spend. The read-only monitor does not call a model. A missing PostgreSQL budget check does not raise this cap. `POSTGRES_BUDGET=NOT_RUN`.

## What counts

Count every job in the isolated repository during the campaign, including fixture workflows, the monitor run, and a rerun. GitHub concurrency that queues a second run is not a deny. A queued run still needs a reservation before it is allowed to start.

Until a usage reading says otherwise, the ledger assumes one billed minute per job, rounded up, plus one overhead minute per workflow run. A job that finishes in seconds still reserves that floor before dispatch. After the run is terminal, the ledger replaces the reservation with the larger of the usage-API minutes and that floor. If the usage API cannot be read, the full reservation stays consumed. Unknown budget state denies the next dispatch.

Reservation for one monitor run:

`11 = 10 minute timeout * 1 job * 1 attempt + 1 overhead minute`

A second attempt, including a manual rerun, is a new reservation. Automatic retry is forbidden. Two dispatches at the same time are forbidden. The second request must fail while the first run is non-terminal or while its actual minutes are still unknown.

## Admission before each dispatch

1. Read the ledger. If the ledger or the usage API cannot be read, deny.
2. Deny when any monitor run is queued, in progress, or waiting for reconciliation.
3. Compute the reservation from the timeout, the job count, and one attempt, then add one overhead minute.
4. Deny when the reservation is greater than `MAX_SINGLE_RUN_MINUTES + 1`, or when the remaining minutes are less than the reservation.
5. Write the reservation and a new dispatch id before anyone starts the workflow. A second writer that sees the same unused id must deny. This compare-and-swap is the double-dispatch guard.
6. Only then may an operator dispatch one run.
7. When the run is terminal, replace the reservation with the reconciled minutes. Do not release minutes that the usage API did not report.

The candidate workflow cannot do this. It starts after dispatch, its token stays `actions: read` and `contents: read`, and this stage does not change that file. The current concurrency block can queue another run. It does not keep a minute ledger.

## Campaign that fits, and the check that does not

D1 through D6 in the 0.5D plan reserve 58 minutes if each run is reconciled before the next. That set keeps every HOSTED and STOP observation except a live second jobs page. D5 still shows that a short page ends the loop. It does not show that page 2 is fetched.

A live HOSTED-003 page-2 proof needs at least 101 jobs. Under the one-minute floor that is at least 101 minutes, which is above 60. This plan does not dispatch that fixture. The controller decides whether to raise the cap, or to accept the local page-order tests plus the short-page hosted observation as the page-2 deferral. Dropping the check silently is not allowed.

## Proposed gate, not built

A separate controller-side ledger, outside the monitor job, should hold the remaining minutes and the dispatch lock. It reads usage after each terminal run and performs the compare-and-swap above. It must not add a PAT or a write token to the monitor job. Building that ledger needs its own controller GO. `BUDGET_RUNTIME_ENFORCEMENT=NOT_IMPLEMENTED`. `BUDGET_IMPLEMENTATION_REQUIRED=YES`.
