# Production Deployment Runbook Draft v0.1

## Purpose

This is a draft runbook for a future production deployment execution pack.

It is not an execution approval.

It does not execute production deploy.

## Required Before Execution

```text
OWNER_APPROVES_PRODUCTION_DEPLOYMENT_EXECUTION
```

## Phase 0 — Stop Conditions

Stop immediately if any of these occur:

```text
Unexpected staged files
Unapproved source code changes
Unapproved migrations
Secrets detected in repo/docs
Missing owner execution approval
Missing backup/snapshot confirmation
Target production environment undefined
Nginx/config test failure
Health check failure
```

## Phase 1 — Pre-deploy Evidence

Read-only checks to capture before execution:

```text
git log latest approved commit
git status
staging HTTPS root
staging login
staging health
API proxy read-only
current deployment target inventory
backup/snapshot confirmation
```

## Phase 2 — Backup Requirement

Before any server change, execution pack must require:

```text
VM snapshot or provider backup confirmation
Server config backup
Application release backup
Database backup/snapshot confirmation if database changes are involved
```

## Phase 3 — Deployment Execution Placeholder

Actual commands are intentionally not included in this draft.

They must be generated in a separate execution pack after:

```text
OWNER_APPROVES_PRODUCTION_DEPLOYMENT_EXECUTION
```

## Phase 4 — Post-deploy Verification

Future execution pack must verify:

```text
HTTPS root
login route
health endpoint
API read-only endpoint
container health
Nginx/config test
logs for critical errors
```

## Phase 5 — Rollback Trigger Criteria

Rollback must be considered if:

```text
HTTPS root unavailable
login route unavailable
health endpoint fails
API read-only endpoint fails
Nginx/config test fails
critical runtime errors appear
operator cannot verify controlled pilot functionality
```

## Boundary

```text
This draft runbook is preparation only.
Production deploy is not authorized.
Production deploy is not executed.
```

## Decision

```text
PRODUCTION_DEPLOYMENT_RUNBOOK_DRAFT_CREATED
```

## References

```text
docs/LOW_CODE_PILOT_WEEK3_PRODUCTION_DEPLOYMENT_PREPARATION_PLAN_V0.1.md
docs/LOW_CODE_PILOT_WEEK3_PRODUCTION_DEPLOYMENT_PREPARATION_CHECKLIST_V0.1.md
docs/LOW_CODE_PILOT_WEEK3_PRODUCTION_ROLLBACK_PLAN_V0.1.md
```
