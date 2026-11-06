# AUTOPILOT-0.3C2-R6B-3 — parallel task map

This map does not authorize any agent to start. Agent H owns the mock orchestrator until an independent review accepts a later head. No other agent may edit the SQL lifecycle, budget functions, or `mock_orchestrator.py` at the same time.

| Agent | Proposed later work | Allowed paths after a separate task | Forbidden while H owns R6B-3 |
| --- | --- | --- | --- |
| A | Infrastructure notes for an isolated autopilot Postgres service. No staging apply. | `docs/engineering/autopilot/infra/` | `infrastructure/migrations/**`, `.github/workflows/**`, SQL under `tools/bintrans_autopilot/sql/**` |
| B | Extra orchestration fixtures after H's tests exist. | `tools/bintrans_autopilot/tests/fixtures/` | `mock_orchestrator.py`, `postgres_queue.py`, `budget_controller.py`, `0008`–`0015` |
| C | Provider research notes. No client, no credential, no live call. | `docs/engineering/autopilot/provider-research/` | `cursor_api.py`, `launch/mock_transport.py`, any HTTP library |
| D | Observability field list taken from the audit event names. | `docs/engineering/autopilot/observability/` | `infrastructure/monitoring/**`, budget ledger tables |
| E | Control-panel wireframe that cannot toggle production flags. | `docs/engineering/autopilot/control-panel/` | `apps/**`, `autopilot.config.json`, launch control writes |
| F | Runbook describing estop, `DISABLED`, and the isolated-only `0015` rollback warning. | `docs/engineering/autopilot/runbooks/` | SQL function bodies, deployment scripts |
| I | Read-only security review of H's head. | none | every write, including docs and tests |

Shared rules for every agent: `AUTOPILOT_ENABLED` stays unset, paid automation stays off, Cloud Agent launches stay at zero, and merge is not authorized by this map.
