# Task Contract

## Task ID

CT-AA-001

## Title

Control Tower alert acknowledgement — API contract and architecture freeze

## Owner

orchestrator

## Role

architect

## Repository

D:\Projects\freight-platform-wt\ct-alert-ack-contract

## Base branch

origin/main

## Base SHA

02208106e494afcaa46372e44b417761d6613daf

## Working branch

arch/control-tower-alert-ack-contract-v0.1

## Worktree

D:\Projects\freight-platform-wt\ct-alert-ack-contract

---

## Objective

Freeze the v0.1 API contract and architecture for Control Tower critical event acknowledgement: public POST endpoint, summary enrichment schema, internal read-model endpoints, stable event identity, persistence ownership, and migration outline. Deliver **CONTRACT_FREEZE_SHA** for downstream parallel implementation.

## In scope

- OpenAPI: `POST /api/v1/control-tower/critical-events/{eventId}/acknowledge`
- OpenAPI: extend `ControlTowerEvent` with optional acknowledgement block
- OpenAPI: document error semantics (400/401/403/404)
- Architecture doc: `docs/engineering/parallel/pilots/control-tower-alert-ack-v0.1/ARCHITECTURE.md`
- Confirm stable event_id algorithm matches `deterministicEventID()` in gateway
- Define internal read-model endpoint shapes (document in ARCHITECTURE.md; OpenAPI optional for internal)
- Define migration 000020 table schema (columns, PK, indexes) — SQL file NOT created here unless contract task explicitly includes migration stub; prefer schema spec in ARCHITECTURE.md for backend to implement

## Out of scope

- Backend handler implementation
- Frontend UI implementation
- Migration file creation (deferred to CT-AA-002 unless architect includes draft in contract commit — prefer spec only)
- Security review execution
- QA test execution

## Allowed paths

- `packages/openapi/openapi.yaml`
- `docs/engineering/parallel/pilots/control-tower-alert-ack-v0.1/ARCHITECTURE.md`
- `docs/engineering/parallel/pilots/control-tower-alert-ack-v0.1/contracts/CT-AA-001-FROZEN.md` (optional freeze marker)

## Forbidden paths

- `services/**`
- `apps/**`
- `infrastructure/migrations/**`
- `infrastructure/docker-compose/**`
- `.github/**`
- `Makefile`, `go.work`, root `package.json`

## Dependencies

- none

## Security invariants

- No request body fields for `tenant_id`, `user_id`, or `acknowledged_by` on public API
- Tenant and actor from JWT/gateway context only
- Cross-tenant unknown event → 404, not 403 with leak
- RBAC: reuse existing `CanAccessControlTower` roles for v0.1 (document in ARCHITECTURE.md)
- Internal endpoints trust gateway-set `X-Tenant-ID` / `X-User-ID` only

## Acceptance criteria

1. OpenAPI defines acknowledge endpoint with full request/response/error schemas.
2. `ControlTowerEvent` includes optional acknowledgement representation for summary merge.
3. ARCHITECTURE.md documents: persistence table, event identity, idempotency, service boundaries, internal API paths.
4. Handoff records **CONTRACT_FREEZE_SHA** = final commit on working branch.
5. `make openapi-validate` PASS (or NOT_RUN with reason if tooling unavailable).

## Required validation

Level: 1

Commands:

- `git diff --check`
- `make openapi-validate` (if available)

## Required deliverables

- OpenAPI diff
- ARCHITECTURE.md
- Handoff with CONTRACT_FREEZE_SHA

## Integration target

int/control-tower-alert-ack-v0.1 → main

## Handoff requirements

- Base SHA, final SHA, branch, worktree
- **CONTRACT_FREEZE_SHA** prominently recorded
- Changed files list
- Contracts changed: packages/openapi/openapi.yaml
- Validation results
