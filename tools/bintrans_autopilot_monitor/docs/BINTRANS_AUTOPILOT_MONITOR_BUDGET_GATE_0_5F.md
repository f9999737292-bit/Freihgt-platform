# Budget gate 0.5F

Local SQLite admission gate for a future isolated Actions proof. It does not call GitHub, does not read a token, and does not change the candidate workflow. The offline dispatch adapter is a separate module and its real transport is disabled. The product PostgreSQL budget controller is not reused: it belongs to the autopilot package, stores cents, and is not an Actions-minute ledger.

Run the local tests from `tools/bintrans_autopilot_monitor`:

```text
python -m unittest tests.test_budget_gate -v
```

The tests use a temporary database and `FakeDispatchAdapter`. They do not open a socket.

## Components

`src/bintrans_monitor/budget_gate.py` holds the ledger, the reservation transaction, the dispatch state machine, the adapter interface, the fake adapter, reconciliation, the emergency-stop checks, and the audit table. `src/bintrans_monitor/dispatch_adapter.py` is the offline dispatch boundary added in 0.5G. Its real transport raises before any connection. `LIVE_CREDENTIALS_CONFIGURED=NO`. The monitor job token stays read-only. Dispatch and cancel principals are separate classifications and are not configured with a secret.

A new campaign file is created only by `initialize_campaign`. `BudgetGate.open` never creates a file. A missing file, a file that is not a database, or a database that fails `integrity_check` denies. A short SQLite lock denies rather than reserving without the lock.

## Reservation

`AVAILABLE = 60 - confirmed reconciled minutes - active reserved minutes`.

One monitor job reserved at its 10 minute cap is `10 + 1` overhead = 11. The check and the insert are one `BEGIN IMMEDIATE` transaction. Stop must be exactly `off`. Unknown stop denies. A second active dispatch denies. An unknown dispatch or unknown usage denies. Duplicate `dispatch_id` or `idempotency_key` denies. `paid_ai_calls` must be 0. `run_attempt` on a new reserve must be 1. The gate does not retry the adapter.

`DISPATCH_INTENT_RECORDED` is written before the adapter is called. The database transaction and a future GitHub call are not one transaction. There is no exactly-once claim. If the process stops after the intent, or the adapter times out, crashes, or loses the response, the row becomes `DISPATCH_UNKNOWN`, the full reserve stays, and no second dispatch is allowed until a real reconciliation. A timeout does not release the reserve. A definite adapter rejection before a run exists is `REJECTED` and releases the reserve. Cancellation before the adapter is called is `CANCELLED_BEFORE_DISPATCH`.

## Reconciliation

Each billable job is stored once. Cancelled, failed, and timed-out jobs still count. The same fingerprint does not add the minutes again. A different fingerprint after reconciliation blocks the campaign and does not change the stored total. If the stored run id does not match, the campaign blocks. Usage that has not arrived leaves `TERMINAL_PENDING_RECONCILIATION` and keeps the reserve. If confirmed minutes exceed the reserve, the actual total is stored, an overrun event is written, and further reserves deny. The actual total is not clipped to the reserve, and no reserve is released to hide a negative remainder.

## Bypass

The gate can notice a run id that has no row, a second attempt, a different SHA, or a different repository or workflow. It then writes a security incident and blocks new reserves. That is detection after the run exists. It does not stop GitHub from billing a dispatch made outside this process. `BYPASS_PREVENTION_HOSTED=NOT_PROVEN`. A future hosted proof would need a separate principal for dispatch and cancel, with the monitor token left at `actions: read` and `contents: read`. This stage does not grant those rights.

## Emergency stop

Stop is read before reserve, again before the intent row, a third time immediately before the adapter call, and again on open after a process restart. Unknown stop denies. If stop is on before the call, and the call has not been made, the local row is `CANCELLED_BEFORE_DISPATCH`. If the call may already have been made, the reserve stays, the row is `DISPATCH_UNKNOWN`, and further reserves deny. That third read does not atomically stop the GitHub API. The emergency-stop runbook remains the in-flight procedure. No stop file was added to the candidate workflow. Ledger backup and restore rules are in `BINTRANS_AUTOPILOT_MONITOR_LEDGER_INTEGRITY_RUNBOOK_0_5G.md`.

## D1-D6 reserves

`J-MON-J-E001`: D4 is its own workflow run, not a line inside the monitor dispatch.

| Dispatch | Workflow | Jobs | Cap per job | Overhead | Reserve |
| --- | --- | --- | --- | --- | --- |
| D1 | monitor | 1 | 10 | 1 | 11 |
| D2 | monitor | 1 | 10 | 1 | 11 |
| D3 | monitor | 1 | 10 | 1 | 11 |
| D4 | fixture, separate from the monitor | 1 | 2 | 1 | 3 |
| D5 | monitor | 1 | 10 | 1 | 11 |
| D6 | write-probe workflow, separate from the candidate | 1 | 10 | 1 | 11 |

Sum of reservations: 58. Headroom: 2. A rerun is a new reservation and is denied when the remainder is smaller than that reservation. D7, the live 101-job page, stays deferred. `LOCAL_FULL_PAGINATION_TEST=PASS`. `HOSTED_SINGLE_PAGE_TEST=PLANNED`. `HOSTED_LIVE_PAGE2=DEFERRED_D7`. `HOSTED_003_FULL_ACCEPTANCE=NO`.

`J-MON-J-B001` runtime enforcement is this local gate. Hosted enforcement is not run. `J-MON-R2-M003` stays open.
