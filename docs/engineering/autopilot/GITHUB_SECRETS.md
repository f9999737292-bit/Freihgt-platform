# GitHub secrets and variables

The dispatcher reads credentials from the Actions environment. It does not start while `autopilot.config.json` has `autopilot_enabled: false`.

## Secrets

In the GitHub repository: Settings → Secrets and variables → Actions → Secrets.

| Name | Value | Used for |
| --- | --- | --- |
| `CURSOR_API_KEY` | Cloud Agents user or service-account key | `POST /v1/agents` and run reads |
| `CURSOR_ADMIN_API_KEY` | Team Admin API key | `POST /teams/filtered-usage-events` |

`GITHUB_TOKEN` is provided by Actions. The gated job asks for `contents: write`, `issues: write`, and `pull-requests: write` so it can create claim refs, update the issue, and mark a PR draft. It does not ask for administration, so it cannot change branch protection.

Do not echo these values. Do not put them in `autopilot.config.json`.

## Variables

Settings → Secrets and variables → Actions → Variables.

| Name | Initial value | Effect |
| --- | --- | --- |
| `AUTOPILOT_ENABLED` | leave unset, or set `false` | Jobs run only when this is exactly `true`. |
| `AUTOPILOT_EMERGENCY_STOP` | unset | Set to `true` to stop dispatch without a code change. |

Unset is not `true`. A skipped job does not start a runner and does not call Cursor.

## Labels to create before the first real issue

Create these repository labels before using the issue form. This phase does not create them.

- `autopilot`
- `autopilot-contract-approved`
- `autopilot-shared-path-approved`
- one `autopilot:<STATUS>` label for each status in the architecture doc

Add `autopilot` and `autopilot:BACKLOG` when the issue is opened. Add `autopilot-contract-approved` only after a human checks the contract. Add `autopilot-shared-path-approved` only for OpenAPI, migrations, identity/gateway, or `packages/shared-go`.

## Enabling later

1. Finish independent QA of this foundation.
2. Set the two secrets.
3. Confirm `AUTOPILOT_EMERGENCY_STOP` is not `true`.
4. In a separate approved change, set `autopilot_enabled` to `true` and set a positive `budget_max_charged_cents`.
5. Set the variable `AUTOPILOT_ENABLED` to `true`.
6. Approve one issue.

Until step 4 and step 5 are both done, `python -m bintrans_autopilot dispatch` prints `AUTOPILOT_ENABLED=NO` and exits before building an HTTP client.
