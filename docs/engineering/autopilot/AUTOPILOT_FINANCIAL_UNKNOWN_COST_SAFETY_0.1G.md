# BINTRANS Autopilot — unknown cost safety 0.1G

AGENT=H
STAGE=AUTOPILOT-FINANCIAL-UNKNOWN-COST-SAFETY-0.1G
SOURCE=docs/engineering/autopilot/AUTOPILOT_FINANCIAL_UNKNOWN_COST_SAFETY_0.1F.md
BT_01B_BUSINESS_APPROVAL=NOT_VERIFIED
AUTOPILOT_ENABLED=NO
PAID_AUTOMATION_ENABLED=NO
REAL_CLOUD_AGENT_LAUNCHES=0

The 0.1F findings are implemented only where the specification gives a closed fix.

| Fix | Result |
| --- | --- |
| FIX-1 | `cloud_charged_cents` raises `spend_amount_absent` for a missing, null, or blank `chargedCents`. An explicit number, including 0, is summed with `Decimal`. |
| FIX-2 | Migration `0016` makes `_budget_charged_cents` raise `invalid_amount` for NULL and blank text. The text `0` still returns bigint 0. Migrations `0008`–`0015` are unchanged. |
| FIX-3 | `_budget_release_unused_reserve` raises `unknown_usage_open` when `usage_unknown` is set and does not change the reservation or the windows. |
| FIX-4 | A later payload with a different amount stays `idempotency_conflict`. No zero is written and the reserve is not released. |
| FIX-5 | The dispatcher does not open a PostgreSQL connection. Without a supplied `budget_global_safety_block_reason` result it returns `ledger_unreadable` and does not create an agent. A supplied `unknown_usage_open` also blocks. |
| FIX-6 | Not implemented. `ESTOP_CANCEL_SEMANTICS=NOT_APPROVED`. Emergency stop still does not call `cancel_run` and does not call `create_agent`. |
| FIX-7 | `CursorClient.spend_events_lagged` stays true. A successful spend read is not treated as a closed hour, so `assess_budget` returns `spend_observation_stale`. |

No budget limit is assigned. BT-01b is not approved. Rollback of `0016` restores the previous helper bodies and is limited to an isolated test database.
