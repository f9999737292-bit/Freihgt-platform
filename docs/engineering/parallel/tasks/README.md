# Parallel Task Registry

Declarative registry for active and planned parallel tasks. Used by the orchestrator to prevent path collisions.

## Location

```text
docs/engineering/parallel/tasks/
```

One file per task: `<task-id>.md` (lowercase, hyphenated).

## Required fields

| Field | Description |
|-------|-------------|
| Task ID | Unique identifier |
| Title | Short name |
| Owner | Human or role |
| Agent role | Implementing subagent |
| Branch | Working branch |
| Worktree | Absolute Windows path |
| Status | See statuses below |
| Allowed paths | Globs |
| Forbidden paths | Globs |
| Dependencies | Task IDs / SHAs |
| Integration order | Integer or phase |
| Base SHA | Starting commit |

## Status values

```text
PLANNED
READY
IN_PROGRESS
BLOCKED
READY_FOR_REVIEW
APPROVED
INTEGRATED
CANCELLED
```

## Rules

1. Orchestrator creates or updates entries when planning a batch.
2. Do **not** add fake `IN_PROGRESS` tasks — use `_EXAMPLE.md` as template only.
3. Before assigning overlapping paths, check existing registry files.
4. On integration, set status to `INTEGRATED` and record final SHA.

## Collision check (manual v1)

Before starting task B, confirm no other `IN_PROGRESS` or `READY` task lists overlapping Allowed paths.

Future automation may grep this directory; keep format consistent.

## Related

- Task Contract: `../TASK_CONTRACT_TEMPLATE.md`
- Example entry: `_EXAMPLE.md`
