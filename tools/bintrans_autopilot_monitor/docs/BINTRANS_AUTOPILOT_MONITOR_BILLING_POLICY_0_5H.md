# Billing policy 0.5H

`BILLING_MODE=FULL_RESERVATION_RETENTION`

There is no authoritative GitHub billing source in this stage. Job duration is not billing. `apply_trusted_usage` still rejects a duration estimate and a caller-supplied minutes total. No billing credential is configured.

## What is held

A dispatch that was sent, or that may have been sent, keeps its whole reserve. `retain_full_reservation` records a terminal workflow identity and moves the row to `RESERVATION_HELD`. It writes no confirmed billing minutes. The same minutes are not counted again as reconciled consumption. A second retain of the same run does not add the minutes again.

`AVAILABLE = 60 - reconciled confirmed minutes - active reserves - held reserves`.

Unknown dispatch, unknown usage, an active run, a blocked campaign, and a recovery state all deny the next reserve. A terminal workflow is not a finished financial reconciliation.

## D1-D6

| Dispatch | Reserve held |
| --- | --- |
| D1, D2, D3, D5, D6 | 11 each |
| D4 | 3 |

Sum held: 58. Headroom: 2. The next 11 minute reserve does not fit. This is feasible only while every previous reserve stays held. It is not proof of a GitHub invoice. `GLOBAL_BILLING_CAP=NOT_PROVEN`.

The older `reconcile` path still stores local job minutes for the existing ledger tests. The hosted campaign mode does not use that path to release a reserve.
