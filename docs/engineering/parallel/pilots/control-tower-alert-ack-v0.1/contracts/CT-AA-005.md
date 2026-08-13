# Task Contract

## Task ID

CT-AA-005

## Title

Control Tower alert acknowledgement — QA verification

## Owner

orchestrator

## Role

qa-verification

## Repository

D:\Projects\freight-platform-wt\ct-alert-ack-qa

## Base branch

int/control-tower-alert-ack-v0.1

## Base SHA

`<integration branch SHA after CT-AA-002/003 merged>`

## Working branch

test/control-tower-alert-ack-qa-v0.1

## Worktree

D:\Projects\freight-platform-wt\ct-alert-ack-qa

---

## Objective

Produce honest validation evidence for the acknowledgement pilot: targeted backend tests, frontend build/lint, contract validation where available. Report NOT_RUN for unexecuted checks.

## In scope

- Run Level 1 backend tests for api-gateway/controltower and control-tower-read-model-service ack code
- Run Level 1 frontend lint/build for web-admin
- Run Level 2: `make openapi-check` or `make openapi-validate` if integration branch includes OpenAPI
- Manual test checklist execution (document results)
- QA report with PASS/FAIL/NOT_RUN per command

## Out of scope

- Full repository test suite (unless authorized)
- Level 3 runtime/staging (optional — report NOT_RUN unless inexpensive)
- Code fixes (file defects back to owning workstream)

## Allowed paths

- `docs/engineering/parallel/pilots/control-tower-alert-ack-v0.1/QA_REPORT.md` (create)
- Read-only execution against integration branch codebase

## Forbidden paths

- Product code changes unless fixing test-only gaps explicitly authorized

## Dependencies

- CT-AA-004 security review complete (PASS or CONDITIONAL PASS)
- CT-AA-002 and CT-AA-003 merged to int/control-tower-alert-ack-v0.1 (or QA runs against combined branch)

## Security invariants

- N/A (verification only)

## Acceptance criteria

1. QA_REPORT.md lists every command with PASS/FAIL/NOT_RUN.
2. Acceptance criteria from CT-AA-001/002/003 traced to evidence.
3. No false PASS claims.

## Required validation

Level: 2 (where integration branch permits)

Commands:

- `go test ./...` targeted paths in both backend services
- `pnpm --filter web-admin lint` / build
- `make openapi-validate` or `make openapi-check`
- Manual: acknowledge flow checklist

## Required deliverables

- QA_REPORT.md
- Handoff summary

## Integration target

int/control-tower-alert-ack-v0.1 → main

## Handoff requirements

- Integration SHA tested
- Full command output summaries
- Blockers for CT-AA-006 if any FAIL
