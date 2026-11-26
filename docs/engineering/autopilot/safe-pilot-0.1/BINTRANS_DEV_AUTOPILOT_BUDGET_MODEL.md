# Budget model

Status: planning target, not a spending authorization.

`TOTAL_MONTHLY_TARGET_USD=500` splits on paper into `CURSOR_ULTRA_PLANNING_USD=200` and `VARIABLE_AI_AND_AUTOMATION_PLANNING_USD=300`. Neither number enables a launch. Committed caps on `main` are `0`. `pricing.source` is `unverified`.

## Gates

Every dispatch checks, in order:

1. Emergency stop. A stop denies before a reserve.
2. `UNKNOWN_MODEL_PRICE=DENY` when the price is not from the verified pricing source. Cursor default is not a price. The class `default` and the class `unknown` are not free. `unpaid_model_reason` on `main` already returns `unknown_cost` unless `pricing.source` is `verified_pricing_api` and the choice is `VERIFIED`.
3. `UNVERIFIED_PROVIDER=DENY` when the provider is not attested by the catalog response from `GET /v1/models` or by the usage event. The `provider` string in the local JSON is not attestation. `require_distinct_provider` on `main` fail-closes when it is true, and the committed file leaves it false. This pilot requires it true before any paid phase.
4. Monthly limit, daily limit, per-task `MAX_COST_USD`, and per-agent limit. The reserve is the task cap, not a guess of the final bill.
5. Active reserves are subtracted before a new task is accepted.
6. After the run, sum `chargedCents` from `POST /teams/filtered-usage-events` for that cloud agent. `GET /v1/agents/{id}/usage` reports tokens only. Tokens are not converted to dollars by a local formula.
7. If the usage event is missing, late, or not attributable, the task stays `BUDGET_EXHAUSTED` for new work: unknown cost blocks. The agent cannot clear that block or raise its own cap.
8. A retry uses the original reserve. It does not increase `MAX_COST_USD`.

`chargedCents` is documented to include model cost and the Cursor Token Rate when that rate applies. Reconciliation sums that field. It does not sum a second local estimate on top.

The Admin API page says the usage feed is hourly and should be polled at most once an hour. A launch decision that cannot wait for that feed denies when the price was not known in advance.

## Issue 313 on current main

| Item | Status on `5432e144` | Evidence |
| --- | --- | --- |
| M-312-1 unknown default cost | Deny path present. Not armed as a paid configuration | `unpaid_model_reason`; committed `pricing.source=unverified`; caps `0`. Tests were not re-run in this task |
| L-312-1 finance role boundary | Exact service prefixes | `FINANCE_PREFIXES` in `models.py`. Not a substring scan of arbitrary paths |
| L-312-2 provider string | Fail-closed only when the flag is true. Flag is false | `require_distinct_provider: false` in `autopilot.config.json` |
| RELEASE-GATE | Open controller decision | Issue #313 is OPEN |

## What an agent cannot do

The worker does not set the monthly number, the daily number, or the task cap. A prompt that names a higher cap is ignored. Overrun is a hold, not an automatic top-up.

## Phase limit

Phase A and this pilot: max spend `0`. No Admin API call is made, because that call needs a live admin key and this task forbids paid and secret use. The dollar endpoint is cited from the public doc only.
