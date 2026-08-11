# BINTRANS Control Tower Staging Pack — AI Working Rules

## Project root

Always work from:

```
D:\Projects\freight-platform-staging-pack
```

Branch: `ops/bintrans-ct-staging-pack`

Do **not** modify `D:\Projects\freight-platform` (read-only comparison only if explicitly needed).

## Autonomous local scope

Agent may proceed without operator prompts for local inspection, static validation, tests, docs, and local commits.

Stop and request approval before: SSH, live staging deploy, database/registry mutations, protected env changes, `git push`, primary Control Tower mode, or cohort creation.

See `.cursor/rules/` and `.cursor/permissions.json` for full policy.

## Safety rules

- Do not commit secrets, populated `staging.env`, or digest-pinned production image refs.
- Do not run `docker compose up`, migrations, or registry push/pull/login autonomously.
- Schema version 19 is verified on staging — **do not rerun migration**.
- `CONTROL_TOWER_READ_MODEL_MODE=primary` is forbidden.
- Never print secret values; report classifications only.

## Windows compatibility

- Prefer Git Bash for `.sh` scripts.
- Do not assume WSL bash matches Git Bash.

## Workflow

1. Diagnose first.
2. Minimum scoped change.
3. Run safe static checks / selftests.
4. Commit when requested; push only when explicitly authorized.
