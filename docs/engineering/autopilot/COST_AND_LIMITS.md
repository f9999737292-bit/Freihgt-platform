# Cost and operating limits

The figures below are caps, not a price list. Cursor's published Cloud Agents docs do not fix a per-model dollar rate in the endpoints this client uses. Do not invent one.

## Caps in `autopilot.config.json`

| Cap | Default | Effect when reached |
| --- | --- | --- |
| `autopilot_enabled` | `false` | No Cursor call |
| `budget_max_charged_cents` | `0` | No spend is authorized. Ready issues move to `BUDGET_HOLD` only after a real over-limit response. A zero cap holds before launch once the program is otherwise enabled. |
| `max_parallel_agents` | `3` | Further ready issues wait |
| `max_launches_per_day` | `8` | Further launches wait until the next UTC day |
| `max_retries_cap` | `1` | A second failed run becomes `BLOCKED` |
| `max_qa_fix_cycles` | `2` | A later QA failure becomes `BLOCKED` |
| Domain caps | `1` for OpenAPI, migrations, identity/gateway, and `packages/shared-go` | A second task in that domain waits |

`budget_max_charged_cents` must be raised in a reviewed change before a trial. The check sums `chargedCents` from `POST /teams/filtered-usage-events` with `cloudAgentId=*` and `hostingType=CLOUD`. If the Admin API errors or the admin key is missing, the issue state is left unchanged and nothing is launched.

The Admin API text recommends polling filtered usage at most once per hour. The schedule is hourly (`17 * * * *`) for that reason. Issue events can add extra polls; the documented rate limit on that endpoint is 60 requests per minute per team. The client stops after 20 pages and fail-closes if the result is truncated.

## GitHub Actions

While `AUTOPILOT_ENABLED` is not `true`, the job-level `if` skips the job before a runner starts. Skipped jobs do not call Cursor. GitHub may delay or drop a cron, and a repository with no Actions activity for 60 days stops scheduled workflows until a repository event wakes them. Event triggers cover that gap once the controller has enabled the variable.

## Historical usage journal

`tools/bintrans_autopilot/usage_journal.json` is the only stored Cloud Agent usage record. `raw_cost_cents` is the undiscounted model cost. `charged_cents` is the amount billed for that run. A JSON `null` means the amount was not read and is not zero. A recorded `0` is the server value for that run. Recording the same agent id and run id again does not add a second row. This file does not change the caps above. The live stop remains `POST /teams/filtered-usage-events`. The scheduler does not launch from the journal and does not treat a missing charge as zero.

## What the report shows

The daily report lists launches recorded on issues, blocked work, and open PRs. It prints `CURSOR_USAGE=NOT_RUN` when autopilot is off. It does not convert token totals from `GET /v1/agents/{id}/usage` into money. Token totals are not cents.

## Controller knobs

- Lower `max_parallel_agents` for the first trial. One ready issue is enough.
- Keep shared-path domains at 1.
- Treat dashboard on-demand spend as the human-visible number. `POST /teams/spend` returns per-member `spendCents` and `overallSpendCents` for the billing cycle; this client uses filtered cloud-agent events so local IDE spend is not mixed into the stop.
- Rotate the API key if a log might have printed it. This program does not log the key.
