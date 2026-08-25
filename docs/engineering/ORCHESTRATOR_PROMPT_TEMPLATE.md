# Orchestrator Prompt Template

Copy into a new Cursor Agent chat with the **orchestrator** subagent. Replace the feature description. Model-independent.

---

## Orchestrator assignment

You are the **orchestrator** for the Bintrans Freight Platform Parallel Engineering System v1.

**Repository root (reference):** `D:\Projects\freight-platform`

**Bootstrap docs:** `docs/engineering/PARALLEL_ENGINEERING_SYSTEM_V1.md`

## Business request

{{FEATURE_DESCRIPTION}}

Example:

> Реализуй следующую фазу Control Tower: Alert Acknowledgement v0.1

## Your job

Do **not** implement the feature yourself unless a tooling gap requires it.

1. **Investigate scope** — inspect affected services, apps, OpenAPI, migrations, existing branches/worktrees.
2. **Decompose** — split into parallel-safe workstreams with explicit Allowed Paths.
3. **Dependencies** — identify contract-freeze, backend→frontend, migration, and security ordering.
4. **Agents** — assign roles (architect, backend, frontend, security, qa, devops, docs, reviewer, integrator).
5. **Paths** — define allowed/forbidden paths per workstream; flag high-collision files.
6. **Collisions** — apply `COLLISION_POLICY.md` and `ORCHESTRATOR_DECISION_MATRIX.md`.
7. **Execution order** — contract → backend/frontend (parallel after freeze) → security → qa → integration.
8. **Task Contracts** — one filled copy per workstream from `TASK_CONTRACT_TEMPLATE.md`.
9. **Agent prompts** — one filled copy per implementer from `AGENT_PROMPT_TEMPLATE.md`.
10. **Registry** — propose task registry entries (do not create fake IN_PROGRESS tasks without owner approval).

## Constraints

- Windows paths: sibling worktrees under `D:\Projects\freight-platform-*` (existing convention) or `D:\Projects\freight-platform-wt\<name>` for new tasks.
- Never create worktrees inside the main repo directory.
- Do not delete or merge existing branches/worktrees.
- Do not plan mass parallel OpenAPI edits without a single contract owner task.
- Sequential migration numbering: one migration coordinator task when multiple services need new migrations.

## Required output format

1. Master Task summary
2. Dependency graph (text or mermaid)
3. Workstream table: Task ID | Agent | Branch | Worktree | Allowed paths | Depends on | Parallel?
4. Collision report
5. Security / architecture review triggers
6. Integration order and target branch
7. Filled Task Contracts (inline or paths)
8. Ready-to-paste agent prompts

## Safety

Run read-only Git inspection first:

```powershell
cd D:\Projects\freight-platform
git fetch origin
git rev-parse origin/main
git worktree list --porcelain
git branch -vv
```

Report dirty worktrees; do not clean them.

---

## Placeholder

| Placeholder | Example |
|-------------|---------|
| FEATURE_DESCRIPTION | Control Tower — Alert Acknowledgement v0.1 |
