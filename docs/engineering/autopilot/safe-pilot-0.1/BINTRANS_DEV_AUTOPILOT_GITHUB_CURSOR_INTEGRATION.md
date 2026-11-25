# GitHub and Cursor integration

Status: research for the safe pilot. No call was made to Cursor, GitHub mutations, or a paid model.

Official pages read on 2026-11-25:

- https://cursor.com/docs/cloud-agent/api/endpoints
- https://cursor.com/docs/api
- https://cursor.com/docs/account/teams/admin-api

The Cloud Agents API v1 is documented as a public beta. Endpoints below are the ones those pages list. Anything not listed there is `NOT_SUPPORTED_OR_NOT_VERIFIED`.

## Verified Cloud Agent operations

Base host documented by Cursor: `https://api.cursor.com`.

| Operation | Method and path | Use |
| --- | --- | --- |
| Create agent and enqueue the first run | `POST /v1/agents` | Launch. Returns an agent and a run |
| List and get agent | `GET /v1/agents`, `GET /v1/agents/{id}` | Status identity |
| Follow-up run | `POST /v1/agents/{id}/runs` | Another run on the same agent |
| Run status | `GET /v1/agents/{id}/runs/{runId}` | Poll status |
| Cancel | `POST /v1/agents/{id}/runs/{runId}/cancel` | Terminal. The run cannot be resumed |
| Token usage | `GET /v1/agents/{id}/usage` | Token counts, not dollars |
| Model catalog | `GET /v1/models` | Ids the account can select |
| Dollar usage | `POST /teams/filtered-usage-events` | Sum `chargedCents`. Hourly aggregate. Documented guidance: poll at most once an hour |

Create accepts `repos[].url`, `repos[].startingRef` (branch or 40-character SHA), `repos[].prUrl`, `workOnCurrentBranch` (default false, which pushes a generated `cursor/...` branch), and `autoCreatePR`. `autoCreatePR` opens a pull request when the run completes. The page does not say that pull request is forced to draft. Draft state for BINTRANS must be enforced by our own GitHub call or verified later. Until that is verified, `autoCreatePR` is not the BINTRANS draft-PR control.

`envVars` is documented as beta and may be silently ignored. Do not put a secret in that field. A silently ignored secret is also a reason not to depend on it.

## Automations

Documented on the same endpoints page:

- `GET /v1/automations` and `GET /v1/automations/{id}` read an automation.
- `PATCH /v1/automations/{id}` changes the model and nothing else.
- Create, edit, and delete stay in the Automations dashboard.
- A run can be started by calling the automation webhook.
- The page says automations run on a schedule or when something happens in a connected tool. The sample workflow shows a cron trigger and a `gitPr` action. A GitHub Issue label or comment trigger is `NOT_SUPPORTED_OR_NOT_VERIFIED`.
- Automations always run in Max Mode.
- There is no documented API field that applies the BINTRANS budget gate, the controller event, or the emergency-stop variable before an automation run.

Programmatic create or disable of an automation is `NOT_SUPPORTED_OR_NOT_VERIFIED`.

## GitHub side that exists today

`.github/workflows/bintrans-autopilot-dispatcher.yml` on `main`:

- Schedule and `workflow_dispatch` and issue opened/edited/labeled.
- `pull_request` and `pull_request_target` are absent.
- The job condition requires `vars.AUTOPILOT_ENABLED == 'true'` and `github.ref == 'refs/heads/main'`.
- The process receives `AUTOPILOT_EMERGENCY_STOP` from the repository variable.
- Checkout is pinned to `main` before secrets are used. The workflow guard runs before the dispatch step.

Issue #313 remains OPEN. It is not an execution approval.

## Option A — Cursor-native Automations

An automation would watch a schedule or a connected tool and open a pull request.

- Cost: Max Mode is mandatory. Our monthly cap cannot be applied inside the documented automation object. Dollar cost appears later in `chargedCents`, after the spend.
- Security: controller approval, path allow-lists, and the forged-reviewer gate are not documented automation preconditions. Dashboard edit is outside the repository review path.
- Operations: create and delete are dashboard actions. The API cannot turn the automation off; it can only change the model.
- API limits: no verified Issue-label trigger. Webhook start is a second entry point that bypasses GitHub Actions.
- Observability: token usage and later `chargedCents`. No BINTRANS lease id unless the prompt happens to include one.
- Errors: a failed run is a Cursor run. Retry policy would be the automation's, not our `MAX_RETRIES`.
- Emergency stop: `NOT_SUPPORTED_OR_NOT_VERIFIED` through the API. An operator would use the dashboard. That is not the checked Actions variable.

## Option B — existing queue plus a Cursor adapter

The current dispatcher remains the caller of `POST /v1/agents`.

- Cost: no call is made until the budget gate reserves `MAX_COST_USD` from verified prices. Unknown price denies. `chargedCents` reconciles the reserve. Unknown usage blocks new reserves, which the budget controller on `main` already models.
- Security: controller event, path rules, emergency stop, and tenant lease run in our process before the HTTP call. `workOnCurrentBranch` stays false so the agent does not push to `main`.
- Operations: one entry point. The workflow and the queue already exist. Phase A does not add the HTTP call.
- API limits: we use only the verified create, status, cancel, models, and usage endpoints. We do not use Automations.
- Observability: queue lease, issue state, run id, and the usage event share one task id.
- Errors: retry stays inside `MAX_RETRIES` and does not raise the reserve. Cancel uses the documented terminal cancel.
- Emergency stop: the Actions variable is already passed into the process. A missing trusted stop source fails closed. The ignored checkout file is not the remote stop.

## Phase A monitor

Revision 0.2 reason: J-DA-C002. The existing dispatcher workflow, once `AUTOPILOT_ENABLED` is true, grants the job `contents: write`, `issues: write`, and `pull-requests: write`. That workflow is not the Phase A monitor.

The candidate `.github/workflows/bintrans-autopilot-monitor.yml` is read-only and manual. It is not in the repository. It does not receive `CURSOR_API_KEY` or `CURSOR_ADMIN_API_KEY`. It does not call `api.cursor.com`.

## Recommendation

Use option B. It is the smaller change that keeps the budget gate and the stop in a place we already control. Option A is retained as a later comparison only if Cursor documents a pre-run deny hook and an API disable. Neither was on the page read for this pilot.

Claude API and DeepSeek API are not connected. A direct paid integration is out of scope. No endpoint for those providers is selected here.
