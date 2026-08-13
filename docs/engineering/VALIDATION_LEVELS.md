# Validation Levels

Task Contracts specify one required level. Agents report honest results; **NOT_RUN** is acceptable, false **PASS** is not.

## Level 0 — Static sanity

Use for docs-only, planning, or read-only review tasks.

- `git status --short`
- `git diff` / `git diff --check`
- Syntax / format check on touched files where relevant
- OpenAPI lint only when YAML touched and authorized: `make openapi-validate`

## Level 1 — Targeted

Default for isolated service or app changes.

- Level 0 items
- Targeted unit tests, e.g. `go test ./internal/...` in changed service
- Frontend lint/build for touched app only
- `make openapi-validate` when contract files changed

## Level 2 — Integration

Use when Task Contract spans services or contracts.

- Level 1 items
- Cross-service or contract tests
- Gateway + service handler tests together
- `make openapi-check`
- Selected scripts under `tests/integration/` when authorized

## Level 3 — Runtime / full flow

Use for integration, staging, or release gates only.

- Level 2 items
- Compose smoke / health: `make health-check`, `make platform-up-safe` (when authorized)
- `make integration-smoke-test`
- Staging or E2E flows explicitly authorized in Task Contract

## Reporting

| Result | Meaning |
|--------|---------|
| **PASS** | Command executed successfully |
| **FAIL** | Command executed and failed |
| **NOT_RUN** | Not executed — must not be reported as PASS |
| **BLOCKED** | Could not run (missing env, dependency, approval) |

Full test suite is **not** required for every agent task. Task Contract defines the minimum level.

See also `.cursor/rules/80-verification.mdc`.
