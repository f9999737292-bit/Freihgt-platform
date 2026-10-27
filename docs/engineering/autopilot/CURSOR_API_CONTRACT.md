# Cursor API contract used by the dispatcher

Checked on 2026-10-27 against:

- https://cursor.com/docs/cloud-agent/api/endpoints
- https://cursor.com/docs/cloud-agent/api/overview
- https://cursor.com/docs/account/teams/admin-api

The Cloud Agents API v1 is in public beta. Authentication for these endpoints is HTTP Basic with the API key as the username and an empty password. Bearer is also accepted by the overview. This client sends Basic.

## Calls the dispatcher may make

| Purpose | Method and path | Body fields used |
| --- | --- | --- |
| Create implementation or QA agent | `POST /v1/agents` | `prompt.text`, `name`, `agentId`, `repos[].url`, `repos[].startingRef`, `workOnCurrentBranch`, `autoCreatePR`, `skipReviewerRequest`, `mode` |
| Read agent | `GET /v1/agents/{id}` | — |
| Read run | `GET /v1/agents/{id}/runs/{runId}` | — |
| Retry or QA fix | `POST /v1/agents/{id}/runs` | `prompt.text` only |
| Emergency cancel | `POST /v1/agents/{id}/runs/{runId}/cancel` | empty |
| Token report | `GET /v1/agents/{id}/usage` | optional `runId` query |
| Dollar stop | `POST /teams/filtered-usage-events` | `startDate`, `endDate`, `cloudAgentId` = `*`, `hostingType` = `CLOUD`, `page`, `pageSize` |

Spend uses the Admin API key, not the Cloud Agents key. The documented reconciliation field is `chargedCents`. `GET /v1/agents/{id}/usage` returns token counts, not cents, so it is not the budget stop.

## Fields that are not sent

The create request omits `model` so Cursor applies the documented default. It omits `env`, `envVars`, `mcpServers`, `customSubagents`, and `prUrl`. `envVars` cannot be combined with a client `agentId`. There is no documented create field that forces a draft pull request.

`workOnCurrentBranch: true` is documented to push to `startingRef`. The dispatcher therefore creates the feature branch first and refuses `main` and `master`.

Agent I omits `repos` and `env`. The create documentation says omitting both starts a no-repo agent. `mode: plan` is the documented non-implementation mode. The prompt also forbids edits. The API has no separate hard read-only flag.

## Idempotency and run status

Re-posting the same `agentId` returns `409 agent_id_conflict`. A follow-up while a run is active returns `409 agent_busy`; the dispatcher does not send `delivery: steer`. Run status is `CREATING`, `RUNNING`, `WAITING_FOR_BACKGROUND_WORK`, `FINISHED`, `ERROR`, `CANCELLED`, or `EXPIRED`. Agent status `ACTIVE` / `IDLE` / `ARCHIVED` is not used as a substitute for the run status.

Pushed branches and PR URLs are read from `run.git.branches[]` (`repoUrl`, `branch`, `prUrl`). `repoUrl` in responses has no scheme.

## Not called

`/v0/*`, `/v1/repositories` (strict rate limit, not required), environment build APIs, secret write APIs, worker pool APIs, and `POST /teams/spend`. Spend events are enough to sum `chargedCents`. No endpoint was invented for this client.
