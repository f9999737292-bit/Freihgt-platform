# Agent Prompt — CT-AA-006

## Assignment

You are the **integrator** agent for the 7Rights Freight Platform.

**Task ID:** CT-AA-006

**Repository / worktree:** D:\Projects\freight-platform-wt\ct-alert-ack-integration

**Branch:** int/control-tower-alert-ack-v0.1

**Base SHA:** latest `origin/main` at integration start

## Objective

Merge contract → backend → frontend into `int/control-tower-alert-ack-v0.1`, resolve conflicts, run Level 2 validation, open PR to main. Update task registry. No force push.

## Allowed paths

- All paths required for merge resolution
- `docs/engineering/parallel/tasks/ct-aa-*.md` (status updates)
- `docs/engineering/parallel/pilots/control-tower-alert-ack-v0.1/INTEGRATION.md`

## Forbidden paths

- Scope beyond v0.1 pilot

## Dependencies

CT-AA-001 through CT-AA-005 complete per acceptance criteria

## Acceptance criteria

1. Integration branch contains full pilot.
2. git diff --check clean.
3. PR opened to main.
4. Registry statuses updated on success.

## Required validation level

2

## Merge order

1. CT-AA-001 (contract)
2. CT-AA-002 (backend)
3. CT-AA-003 (frontend)

## Worktree creation

```powershell
git fetch --prune origin
git worktree add D:\Projects\freight-platform-wt\ct-alert-ack-integration -b int/control-tower-alert-ack-v0.1 origin/main
```

Then merge feature branches in order above.
