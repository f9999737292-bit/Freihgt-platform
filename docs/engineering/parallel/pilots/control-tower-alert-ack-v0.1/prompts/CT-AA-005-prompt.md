# Agent Prompt — CT-AA-005

## Assignment

You are the **qa-verification** agent for the Bintrans Freight Platform.

**Task ID:** CT-AA-005

**Repository / worktree:** D:\Projects\freight-platform-wt\ct-alert-ack-qa

**Branch:** test/control-tower-alert-ack-qa-v0.1

**Base SHA:** `<int/control-tower-alert-ack-v0.1 SHA>`

## Objective

Produce QA evidence for alert acknowledgement pilot: Level 1 backend tests, Level 1 frontend lint/build, Level 2 openapi-check where available. Write QA_REPORT.md with PASS/FAIL/NOT_RUN per command.

## Allowed paths

- `docs/engineering/parallel/pilots/control-tower-alert-ack-v0.1/QA_REPORT.md`

## Forbidden paths

- Product code (unless authorized test fix)

## Dependencies

CT-AA-004 security PASS or CONDITIONAL PASS; integration branch with backend+frontend

## Acceptance criteria

1. QA_REPORT.md traces acceptance criteria to evidence.
2. No false PASS.
3. Manual acknowledge checklist documented.

## Required validation level

2

Commands to attempt:

- Targeted `go test` in api-gateway/controltower and control-tower-read-model-service
- `pnpm --filter web-admin lint` and build
- `make openapi-validate` or `make openapi-check`

## Worktree creation

```powershell
git worktree add D:\Projects\freight-platform-wt\ct-alert-ack-qa -b test/control-tower-alert-ack-qa-v0.1 int/control-tower-alert-ack-v0.1
```
