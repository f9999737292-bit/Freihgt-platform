# 7Rights Engineering Instructions

Operating manual for AI agents working in the Freight Platform monorepo.

## 1. Mission

Deliver minimal, safe, verifiable changes to the 7Rights Freight Platform while preserving architecture, tenant isolation, and public contracts. Use parallel worktrees and specialized subagents for multi-stream work.

## 2. Repository source-of-truth rule

- The repository layout, Makefile, OpenAPI specs, and migrations define how the system works.
- Inspect before editing; do not assume generic patterns override this repo.
- Active worktree root: run `git rev-parse --show-toplevel` and work from that path.
- Primary checkout example: `D:\Projects\freight-platform`. Parallel worktrees use separate paths (e.g. `D:\Projects\freight-platform-*`).

## 3. Architecture discipline

- Monorepo: Go services in `services/`, Nuxt apps in `apps/`, shared packages in `packages/`.
- API Gateway is the HTTP entry point; preserve service boundaries.
- Do not move domain logic between services without explicit architecture authorization.
- See `.cursor/rules/10-architecture.mdc` and `docs/engineering/PARALLEL_ENGINEERING_SYSTEM_V1.md`.

## 4. Security / tenant isolation

- Authenticate at `api-gateway`; trusted tenant/user context comes from JWT via gateway middleware.
- Do not trust client-supplied `X-Tenant-ID` / `X-User-ID` without gateway validation.
- Use tenant-scoped repository access (`tenant_id` predicates) for tenant-owned data.
- See `.cursor/rules/20-security-tenancy.mdc`.

## 5. Scope discipline

- One task = one responsibility; smallest safe diff.
- Agents must not independently change stacks, service boundaries, auth/tenant architecture, or deployment topology.
- See `.cursor/rules/90-scope-control.mdc`.

## 6. Git discipline

- One implementation task = one branch/worktree when possible.
- Never modify another agent's branch; no force push, no `reset --hard`, no unrequested merge/rebase.
- Always report base SHA, final SHA, changed files, and `git status --short`.
- Commit and push only when explicitly requested.
- See `.cursor/rules/70-git-workflow.mdc`.

## 7. Verification discipline

1. Inspect diff → targeted static checks → unit tests → service tests → build → integration (when authorized).
2. Do not start full Docker stacks for ordinary isolated tasks unless required.
3. Report **PASS**, **FAIL**, **NOT_RUN**, or **BLOCKED** — never claim PASS for unchecked work.
4. See `.cursor/rules/80-verification.mdc`.

## 8. Handoff requirements

Before review, complete `docs/engineering/HANDOFF_TEMPLATE.md` with SHAs, files changed, API/DB/security impact, and verification results.

Parallel workflow (start here for owners):

- `docs/engineering/QUICK_START.md`
- `docs/engineering/PARALLEL_ENGINEERING_SYSTEM_V1.md`
- `docs/engineering/TASK_CONTRACT_TEMPLATE.md`
- `docs/engineering/AGENT_PROMPT_TEMPLATE.md`
- `docs/engineering/ORCHESTRATOR_PROMPT_TEMPLATE.md`
- `docs/engineering/MASTER_TASK_TEMPLATE.md`
- `docs/engineering/REVIEW_PROTOCOL.md`
- `docs/engineering/INTEGRATION_PROTOCOL.md`
- `docs/engineering/WORKTREE_PROCEDURE.md`

Subagents: `.cursor/agents/` — orchestrator, architect, backend-engineer, frontend-engineer, security-auditor, qa-verification, devops-engineer, documentation-engineer, reviewer, integrator.

Parallel rules: `.cursor/rules/05-parallel-engineering.mdc`.

## 9. Forbidden autonomous actions

Without explicit approval, do not:

- Docker volume prune or destructive cleanup
- Database wipe
- API contract rewrite
- Backend business logic rewrite
- Mass formatting of the whole repository
- Change generated files without need
- Push, merge, or rebase shared branches unrequested
- Alter another worktree's uncommitted work

## 10. Definition of Done

- Task requirements met within Allowed Paths
- Relevant verification executed and honestly reported
- Handoff and review gates satisfied when part of a parallel workflow
- `git status --short` clean or explicitly explained
- No secrets committed

---

# 7Rights Freight Platform — AI Working Rules

Preserved project-specific operational rules (do not discard).

## Project root

Always work from the git toplevel of your active worktree:

```bash
git rev-parse --show-toplevel
```

Primary checkout: `D:\Projects\freight-platform`

## Safety rules

- Do not commit .env or secrets.
- Do not run docker volume prune.
- Do not change backend business logic unless explicitly requested.
- Do not change API contracts without a diagnostic report first.
- Do not rewrite working services without approval.
- Prefer small changes and small commits.
- Before changes run: git status --short.
- After changes run relevant checks.

## Runtime commands

Prefer Makefile targets:

- make health-check
- make seed-dev-admin
- make seed-demo-data
- make integration-smoke-test
- make platform-up-no-build
- make platform-up-safe

## Windows compatibility

- Prefer Git Bash for .sh scripts.
- Use curl with stdin / --data-binary @- for UTF-8 JSON.
- Avoid curl -d with Cyrillic on Windows Git Bash.
- Do not assume WSL bash works the same as Git Bash.

## Workflow

For every task:

1. Diagnose first.
2. Explain the root cause.
3. Change the minimum number of files.
4. Run checks.
5. Report changed files.
6. Commit only when requested.
7. Push only when requested.

## Forbidden without explicit approval

- Docker volume prune
- destructive cleanup
- database wipe
- API contract rewrite
- backend business logic rewrite
- mass formatting of whole repository
- changing generated files without need

## Parallel engineering references

- System guide: `docs/engineering/PARALLEL_ENGINEERING_SYSTEM_V1.md`
- Cursor rules: `.cursor/rules/`
- Worktree setup (diagnostics only): `.cursor/worktrees.json`, `.cursor/setup-worktree-windows.ps1`
