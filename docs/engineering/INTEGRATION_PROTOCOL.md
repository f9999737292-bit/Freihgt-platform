# Integration Protocol

Merge reviewed workstreams into an integration branch or target release branch.

## Preconditions

- Reviewer verdict PASS or PASS_WITH_NOTES for every included workstream
- Exact approved final SHAs recorded
- Dependency order satisfied

## Steps

1. **Verify SHAs** — confirm commits exist and match handoff.
2. **Order merges** — migrations and shared contracts first, then dependents.
3. **Merge** — no force push; report conflicts immediately.
4. **Migration order** — apply `infrastructure/migrations/` in numeric order; no reordering of applied history.
5. **OpenAPI consistency** — run `make openapi-validate` / `make openapi-check`.
6. **Frontend/backend compatibility** — smoke affected apps against gateway when authorized.
7. **Broader tests** — integration scripts under `tests/integration/` when authorized.
8. **Integration report** — document merges, SHAs, verification, remaining risks.

## Merge conflict policy

- Stop on conflict; document both sides.
- Do not guess business-logic resolution.
- Escalate to architect or human owner.

## Release gate (when authorized)

- `make health-check` or `make platform-health`
- `make integration-smoke-test` or targeted integration scripts
- Observability checks only if release includes ops changes

## Integrator constraints

- Integrate only reviewed work.
- Do not silently redesign failed implementations.
- Do not merge unreviewed branches.

## Integration report template

| Workstream | SHA | Review verdict | Merge status |
|------------|-----|----------------|--------------|
| | | | |

Verification summary: PASS / FAIL / NOT_RUN / BLOCKED per check.
