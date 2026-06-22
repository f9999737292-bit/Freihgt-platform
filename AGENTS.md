# 7Rights Freight Platform — AI Working Rules

## Project root

Always work from:

D:\Projects\freight-platform

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
