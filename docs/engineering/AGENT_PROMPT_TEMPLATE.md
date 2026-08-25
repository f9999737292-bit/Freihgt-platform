# Agent Prompt Template

Copy into a new Cursor Agent chat. Replace placeholders. Model-independent.

---

## Assignment

You are the **{{ROLE}}** agent for the Bintrans Freight Platform.

**Task ID:** {{TASK_ID}}

**Repository / worktree:** {{REPO}}

**Branch:** {{BRANCH}}

**Base SHA:** {{BASE_SHA}}

## Objective

{{OBJECTIVE}}

## Allowed paths

{{ALLOWED_PATHS}}

## Forbidden paths

{{FORBIDDEN_PATHS}}

## Dependencies

{{DEPENDENCIES}}

## Acceptance criteria

{{ACCEPTANCE_CRITERIA}}

## Required validation level

{{VALIDATION_LEVEL}} (see `docs/engineering/VALIDATION_LEVELS.md`)

## Safety rules (mandatory)

1. Run `git rev-parse --show-toplevel`, `git branch --show-current`, `git rev-parse HEAD`, `git status --short` first.
2. Read the full Task Contract at `docs/engineering/TASK_CONTRACT_TEMPLATE.md` (filled copy for this task).
3. Read `.cursor/rules/05-parallel-engineering.mdc` and role-specific rules.
4. Do **not** use destructive Git commands or touch other worktrees.
5. Change only Allowed Paths.
6. Report **NOT_RUN** for checks not executed; never fake **PASS**.

## Workflow

1. **Investigate** — inspect code and docs relevant to Allowed Paths.
2. **Plan** — confirm scope fits Task Contract; stop if collision or missing dependency.
3. **Implement** — minimal safe diff only.
4. **Validate** — run required validation level; record honest results.
5. **Handoff** — complete `docs/engineering/HANDOFF_TEMPLATE.md` and paste `git status --short`.

Do not commit or push unless the Task Contract explicitly authorizes it.

---

## Placeholder reference

| Placeholder | Example |
|-------------|---------|
| TASK_ID | CT-ALERT-ACK-WS-2 |
| ROLE | backend-engineer |
| OBJECTIVE | Add POST /control-tower/alerts/{id}/ack endpoint |
| REPO | D:\Projects\freight-platform-wt\ct-alert-ack-backend |
| BRANCH | feat/control-tower-alert-ack-backend-v0.1 |
| BASE_SHA | f88b2ec93f897cdafbb2a79ce7b74af10e53ca9a |
| ALLOWED_PATHS | services/control-tower-read-model-service/** |
| FORBIDDEN_PATHS | packages/openapi/**, apps/** |
| DEPENDENCIES | WS-1 contract freeze SHA abc1234 |
| ACCEPTANCE_CRITERIA | 1. Tenant-scoped ack 2. Tests pass 3. Handoff complete |
| VALIDATION_LEVEL | 1 — targeted unit tests |
