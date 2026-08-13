# Task Contract

## Task ID

CT-AA-004

## Title

Control Tower alert acknowledgement — security review

## Owner

orchestrator

## Role

security-auditor

## Repository

D:\Projects\freight-platform-wt\ct-alert-ack-security

## Base branch

int/control-tower-alert-ack-v0.1 (or backend+frontend SHAs under review)

## Base SHA

`<integration or review target SHA>` — set when CT-AA-002 and CT-AA-003 complete

## Working branch

review/control-tower-alert-ack-security-v0.1

## Worktree

D:\Projects\freight-platform-wt\ct-alert-ack-security

---

## Objective

Readonly security review of alert acknowledgement: tenant isolation, IDOR, identity spoofing, RBAC, cross-tenant behavior, actor attribution. Produce severity-rated findings or PASS.

## In scope

- Review diffs from CT-AA-002 and CT-AA-003 (checkout integration branch or compare SHAs)
- Review frozen OpenAPI and ARCHITECTURE.md
- Check gateway auth context propagation to read-model
- Check DB queries for tenant_id predicates
- Document findings in handoff

## Out of scope

- Feature implementation
- OpenAPI or code fixes (report only unless critical blocker assigned)
- Full penetration test

## Allowed paths

- `docs/engineering/parallel/pilots/control-tower-alert-ack-v0.1/**` (review report only)
- Read-only inspection of all other paths

## Forbidden paths

- Any product code modification without explicit orchestrator authorization

## Dependencies

- CT-AA-002 and CT-AA-003 complete (or merged to integration branch for review)

## Security invariants

Review explicitly for:

- IDOR on eventId
- Cross-tenant acknowledgement
- Tenant spoofing via headers or body
- Identity spoofing (client-supplied actor)
- RBAC bypass (roles without Control Tower access)
- Foreign-resource disclosure (403 vs 404)
- Actor attribution integrity

## Acceptance criteria

1. Written review covering all triggers in REVIEW_TRIGGERS.md for this feature.
2. Each finding has severity (CRITICAL/HIGH/MEDIUM/LOW) or PASS.
3. No unresolved CRITICAL findings without documented waiver.

## Required validation

Level: 0

Commands:

- `git diff --check` (on review report files only)

## Required deliverables

- Security review report in handoff or `SECURITY_REVIEW.md` under pilot folder
- PASS or findings list

## Integration target

int/control-tower-alert-ack-v0.1 → main

## Handoff requirements

- SHAs reviewed
- Finding summary table
- PASS / CONDITIONAL PASS / FAIL recommendation
