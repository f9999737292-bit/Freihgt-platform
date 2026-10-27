# Branch protection proposal

This document does not change GitHub branch protection or rulesets. No ruleset API was called.

## Keep for phase 0.1A

Every merge to `main`, including a green autopilot PR, needs a separate controller approval. The dispatcher has no merge call. `READY_FOR_MERGE` means the static gate passed, Agent I did not return a blocking finding, and the PR base SHA equals current `origin/main`.

A PR whose base SHA is behind `origin/main` is `NEEDS_CONTROLLER` and `merge_allowed` is false.

## Proposed ruleset, after a stability period

Apply this on `main` through the GitHub ruleset UI or API in a later change:

1. Require a pull request before merging. Block direct push.
2. Block force-push and branch deletion.
3. Require the existing CI checks, including `repository-safety`, the Go matrix, and `bintrans-autopilot-unit` once that check has been green on `main`.
4. Require one approving review from a person who is not the PR author. Agent I is not that review.
5. Dismiss stale reviews when new commits are pushed.
6. Require the branch to be up to date before merge. A merge queue is the safer serializer if several autopilot PRs become eligible; do not enable auto-merge in the same step as the queue.
7. Restrict who can push to `main` to repository admins. The autopilot `GITHUB_TOKEN` must not be in that list.

## Critical paths

Do not grant the autopilot token a bypass. Suggested ruleset path rules, as a review requirement rather than a silent CODEOWNERS rewrite:

- `packages/openapi/**`
- `infrastructure/migrations/**`
- `packages/shared-go/**`
- `services/identity-service/**`
- `services/api-gateway/**`
- `.github/workflows/bintrans-autopilot-dispatcher.yml`
- `tools/bintrans_autopilot/autopilot.config.json`
- `.cursor/permissions.json`
- `.cursor/cli.json`

The dispatcher already refuses task contracts that list the kill switch, the permissions files, staging compose, and `scripts/ops/` as allowed paths.

## Migrations

A migration PR should require the shared-path label, risk `HIGH` or `CRITICAL`, and a human owner. The ruleset should require a status check that the migration number does not collide. Do not add an auto-merge exception for migrations.

## Later auto-merge

Only after a controller-recorded stable period, and only for issues that are `LOW` risk, do not touch the critical paths above, and are already `READY_FOR_MERGE` against the current `origin/main`. That policy is not implemented. Implementing it requires a separate approval and a ruleset change outside this phase.
