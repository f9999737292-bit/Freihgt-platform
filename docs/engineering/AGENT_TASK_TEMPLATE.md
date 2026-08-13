# Agent Task Template

> **Canonical standard:** use `TASK_CONTRACT_TEMPLATE.md` for new parallel tasks.
> This file is retained for backward compatibility with Phase 1 references.

Assign to one developer subagent (backend, frontend, devops, or documentation) per task.

## Agent Role

<!-- backend-engineer | frontend-engineer | devops-engineer | documentation-engineer -->

## Task ID

<!-- e.g. WS-1 -->

## Branch

<!-- feat/<domain>-<task>-v0.1 on dedicated worktree -->

## Worktree

<!-- D:\Projects\freight-platform-wt\<name> or D:\Projects\freight-platform-<name> -->

## Base branch / Base SHA

<!-- origin/main @ git rev-parse origin/main -->

## Allowed Paths

<!-- explicit globs or directories -->

## Forbidden Paths

<!-- everything else by default -->

## Goal

<!-- one-sentence outcome -->

## Required Changes

<!-- bullet list -->

## Non-Goals

<!-- bullet list -->

## Dependencies

<!-- blocker Task IDs and SHAs -->

## API Contract

<!-- OpenAPI files affected; breaking vs compatible -->

## Security Requirements

<!-- tenant scoping, RBAC, review required? -->

## Verification Required

<!-- Level 0–3; commands; expected PASS/FAIL/NOT_RUN -->

## Definition of Done

- [ ] Changes limited to Allowed Paths
- [ ] Handoff completed (`HANDOFF_TEMPLATE.md`)
- [ ] Required validation level executed and reported honestly
- [ ] No unrequested commits/pushes

## Handoff Format

Use `HANDOFF_TEMPLATE.md`.

## See also

- `TASK_CONTRACT_TEMPLATE.md` — full contract
- `AGENT_PROMPT_TEMPLATE.md` — copy-paste agent chat
- `parallel/tasks/README.md` — registry
