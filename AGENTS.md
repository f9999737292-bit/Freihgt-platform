# BINTRANS — AI Repository Working Rules

## Product identity

The product and project name is BINTRANS.

Do not introduce `7rights`, `7rights.ru`, or `7RL` as current product names.

Historical references may remain only where required for auditability or repository history.

The GitHub repository name may remain `Freihgt-platform`; do not rename repository resources unless explicitly instructed.

## Repository scope

These rules apply repository-wide unless a deeper `AGENTS.md` provides more specific instructions.

Never assume a developer machine path, operating system path, worktree location, feature branch, staging host, or deployment environment from this file.

Always determine the active repository, branch, HEAD, and current `origin/main` from the actual environment.

## General workflow

1. Inspect before changing.
2. Confirm current branch, HEAD, origin/main, worktree status, and relevant existing implementation.
3. Make the smallest scoped change required by the task.
4. Preserve existing public contracts unless the task explicitly changes them.
5. Run the relevant safe tests and static checks.
6. Report exact evidence: branch, base SHA, final HEAD, changed files, tests, CI status, and unresolved gaps.
7. Never claim PASS for a check that was not actually executed.

## Git safety

Do not use force push.

Do not rewrite shared history.

Do not merge a pull request unless the task explicitly authorizes merge.

Do not push unless the task explicitly authorizes push or the assigned implementation workflow clearly requires it.

Before final PR approval or merge recommendation, compare the PR against the latest `origin/main`.

If the PR is stale relative to `origin/main`, report `STALE_BASE=YES` and require synchronization before final approval.

Do not silently resolve merge conflicts by discarding another agent's work.

## Environment safety

Repository inspection, local builds, safe static checks, unit tests, and read-only review are allowed when relevant to the task.

Do not perform any of the following unless the task explicitly authorizes that exact operation:

- SSH to staging or production
- staging or production deployment
- registry mutation
- database mutation
- live migration execution
- infrastructure mutation
- production/staging secret changes
- destructive Docker cleanup
- destructive cloud operations

Never expose, print, commit, or reproduce secret values.

Never commit populated `.env` files, credentials, private keys, access tokens, or production/staging secrets.

## Security invariants

Authentication and authorization must fail closed.

Do not trust client-supplied identity, tenant, company, driver, role, or authorization headers when server-derived authenticated context exists.

Tenant and company isolation must be preserved.

Server-owned identity and authorization decisions must not be replaced by caller-controlled values.

Cross-tenant and cross-company access must be explicitly tested when the touched scope can affect isolation.

## Database migrations

Never assume a migration number from stale documentation.

Determine the current migration head before assigning a new migration.

Do not reuse or collide with an existing migration number.

For new migrations, verify the applicable up/down/up or repository-standard migration checks.

Do not run live staging or production migrations without explicit authorization.

## API and distributed-service changes

Preserve ownership boundaries between services.

Do not create a second source of truth when an existing service owns the data.

Explicitly verify idempotency where commands may be retried.

Reject incompatible or spoofed identifiers rather than silently accepting them.

Document intentional contract changes.

## Tests and evidence

Prefer tests that prove behavior rather than only compilation.

Do not remove, weaken, skip, or bypass a failing test solely to obtain a green result.

If a test cannot run because of an environment dependency, report:

`NOT_RUN`

and state the exact blocker.

Distinguish:

`PASS`

`FAIL`

`NOT_RUN`

`BLOCKED`

`NOT_APPLICABLE`

Do not convert `NOT_RUN` into `PASS`.

## Autopilot

The 24/7 dispatcher in `tools/bintrans_autopilot` is off by default. Do not set `AUTOPILOT_ENABLED`, do not call the Cursor Cloud Agents API, and do not merge to `main` unless the current task explicitly allows that exact action.

GitHub Issues that use the autopilot task template are the execution queue for an existing Task Contract. They are not a second task system. The planning registry remains `docs/engineering/parallel/tasks/`.

A cloud task uses the feature branch named in its contract. Do not require `D:\Projects\freight-platform-staging-pack` unless that checkout is the one `git rev-parse --show-toplevel` actually reports.

## Agent roles

Follow the role declared by the current task.

Implementation agents may edit only within their assigned scope.

Do not modify another agent's active worktree or branch unless explicitly instructed.

### Agent I — Independent QA / Review Mode

When the task identifies the agent as `AGENT=I`, `Agent I`, `QA`, `independent reviewer`, or equivalent, operate in read-only review mode.

Agent I may:

- inspect repository state
- fetch current refs
- inspect PR metadata and diffs
- run safe tests
- run static analysis
- compare contracts and migrations
- verify security, tenancy, regressions, and compatibility
- produce findings and a final verification report

Agent I must NOT:

- modify product code
- modify documentation
- modify tests
- create remediation commits
- push
- merge
- deploy
- execute live migrations
- mutate staging or production
- silently fix findings it discovers

Agent I reports findings to the owning implementation agent.

For Agent I, a review is not final if the PR is stale against current `origin/main`.

Agent I findings should include:

- severity
- affected file/component
- evidence
- expected behavior
- actual behavior
- recommended remediation
- whether the finding blocks merge

Agent I final verdict must be one of:

`PASS`

`PASS_WITH_NITS`

`CHANGES_REQUIRED`

`BLOCKED`

## Final-report integrity

Final reports must use observed data.

Never invent:

- commit SHAs
- PR numbers
- CI run IDs
- migration numbers
- test results
- environment status
- deployment status

If exact evidence is unavailable, explicitly state that it is unknown or not verified.
