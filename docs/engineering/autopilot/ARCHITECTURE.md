# BINTRANS autopilot 24/7 — phase 0.1A

The autopilot is a disabled execution layer on top of the existing Task Contract. GitHub Issues are the queue. The markdown registry under `docs/engineering/parallel/tasks/` stays the planning record. Agent O is the GitHub Actions dispatcher in `tools/bintrans_autopilot`. It does not run unless both of these are true:

- `tools/bintrans_autopilot/autopilot.config.json` has `autopilot_enabled` set to `true` by a later controller change. This phase leaves it `false`.
- The GitHub repository variable `AUTOPILOT_ENABLED` is the string `true`.

Either switch alone does nothing. `AUTOPILOT_EMERGENCY_STOP=true` stops dispatch even when both are true.

## What is live after this phase

Nothing calls the Cursor API from CI. The dispatcher workflow is present and its jobs are skipped until the variable is set. Unit tests use a fake HTTP client.

## Queue

An approved contract is copied into `.github/ISSUE_TEMPLATE/bintrans-autopilot-task.yml`. The dispatcher trusts only the first occurrence of each known heading. Text in `OBJECTIVE`, comments, and later duplicate headings cannot change allowed paths, status, or API options.

A task is launched only when all of the following hold:

- status `READY`
- label `autopilot-contract-approved`
- dependencies already `MERGED` or `DONE`
- `BASE_SHA` equal to current `origin/main`
- allowed paths inside the committed catalog and outside the hard-forbidden list
- shared OpenAPI, migration, security, and `packages/shared-go` paths also have label `autopilot-shared-path-approved` and risk `HIGH` or `CRITICAL`
- global active agents below 3, and the domain cap is free
- launch count for the UTC day is below the daily cap
- spend check is under the configured cent limit

States:

```text
BACKLOG → READY → CLAIMED → IMPLEMENTING → PR_OPEN → CI_CHECK → INDEPENDENT_QA → READY_FOR_MERGE → MERGED → DONE
```

Side states: `BLOCKED`, `NEEDS_CONTROLLER`, `FAILED`, `BUDGET_HOLD`.

This dispatcher does not set `MERGED` or `DONE` and does not call a merge API. Those states are reserved for a later controller action.

## Claim lock

The dispatcher creates `refs/autopilot/claims/<TASK_ID>` through the GitHub git refs API. Creating a ref is atomic: the second creator receives HTTP 422 and does not call Create Agent. The feature branch `autopilot/<task-id>` is created from the approved SHA before the agent starts.

The Cloud Agent is created with the documented body:

- `POST https://api.cursor.com/v1/agents`
- `repos[0].url` and `repos[0].startingRef` set to the feature branch
- `workOnCurrentBranch: true` so commits land on that branch
- `autoCreatePR: true`
- `skipReviewerRequest: true`
- client `agentId` of the form `bc-<uuid>` so a repeated POST returns `409 agent_id_conflict` instead of a second agent
- no `envVars`, because the documented API rejects that combination with `agentId`

`startingRef` is rejected unless it is under `autopilot/` and is not `main` or `master`. The Cloud Agents API does not document a draft-PR flag. After a PR URL appears, the dispatcher calls GitHub `convertPullRequestToDraft`.

Before any retry, the dispatcher reads `GET /v1/agents/{id}` and `GET /v1/agents/{id}/runs/{runId}`. Active run statuses `CREATING`, `RUNNING`, and `WAITING_FOR_BACKGROUND_WORK` are not launched again. A terminal `ERROR` or `EXPIRED` run may be retried once (`MAX_RETRIES` capped at 1) with `POST /v1/agents/{id}/runs`. The next failure moves the issue to `BLOCKED`.

A Cursor HTTP or network error after the claim is stored on the issue. The task stays `CLAIMED` with the same `agentId`. It is not marked done and a second id is not minted.

## Agent I

Static checks run in `qa_gate.py` before any QA agent: allowed paths, forbidden paths, secrets in added lines, skipped or exclusive tests, and critical-domain approval. Agent I is a second create with `mode: plan`, `autoCreatePR: false`, and no `repos` entry (the documented no-repo agent). The prompt forbids edits, pushes, and fixes. Findings are commented on the issue and, for at most two cycles, sent as a follow-up to the implementation agent only.

## Enterprise completion

No approved numeric completion model exists on this base SHA. The daily report prints `ENTERPRISE_COMPLETION_PERCENT=UNCHANGED` and does not derive a percent from merged PRs. See `ENTERPRISE_COMPLETION_METHODOLOGY.md`.

## Related

- Cursor connection: `CURSOR_CONNECTION.md`
- API field notes: `CURSOR_API_CONTRACT.md`
- Secrets: `GITHUB_SECRETS.md`
- Stop: `EMERGENCY_STOP.md`
- Money and limits: `COST_AND_LIMITS.md`
- Branch protection proposal, not applied: `BRANCH_PROTECTION_PROPOSAL.md`
- Dry-run planning queue, phase 0.3A: `SCHEDULER_0_3A.md`
