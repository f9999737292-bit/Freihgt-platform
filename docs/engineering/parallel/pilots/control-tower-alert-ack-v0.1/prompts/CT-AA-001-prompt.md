# Agent Prompt — CT-AA-001

## Assignment

You are the **architect** agent for the 7Rights Freight Platform.

**Task ID:** CT-AA-001

**Repository / worktree:** D:\Projects\freight-platform-wt\ct-alert-ack-contract

**Branch:** arch/control-tower-alert-ack-contract-v0.1

**Base SHA:** 02208106e494afcaa46372e44b417761d6613daf

## Objective

Freeze the v0.1 API contract and architecture for Control Tower critical event acknowledgement. Critical events are **derived** in api-gateway (`BuildCriticalEvents`, `deterministicEventID`). Acknowledgement state will persist in `control_tower.critical_event_acknowledgement` owned by control-tower-read-model-service. Deliver CONTRACT_FREEZE_SHA.

## Allowed paths

- `packages/openapi/openapi.yaml`
- `docs/engineering/parallel/pilots/control-tower-alert-ack-v0.1/ARCHITECTURE.md`

## Forbidden paths

- `services/**`
- `apps/**`
- `infrastructure/migrations/**`

## Dependencies

none

## Acceptance criteria

1. OpenAPI: `POST /api/v1/control-tower/critical-events/{eventId}/acknowledge` — empty body, JWT auth, responses 200/400/401/403/404.
2. Extend `ControlTowerEvent` with optional `acknowledgement` object (`acknowledgedAt`, `acknowledgedBy.userId`).
3. ARCHITECTURE.md documents: event_id = sha256(`{shipmentId}:{eventType}:{occurredAtUnix}`)[:16 hex]; table schema; idempotency; internal read-model endpoints; RBAC = existing `CanAccessControlTower` roles.
4. Handoff records **CONTRACT_FREEZE_SHA**.
5. `make openapi-validate` PASS or NOT_RUN with reason.

## Required validation level

1 — see `docs/engineering/VALIDATION_LEVELS.md`

## Safety rules (mandatory)

1. Run `git rev-parse --show-toplevel`, `git branch --show-current`, `git rev-parse HEAD`, `git status --short` first.
2. Read full Task Contract: `docs/engineering/parallel/pilots/control-tower-alert-ack-v0.1/contracts/CT-AA-001.md`
3. Read `.cursor/rules/05-parallel-engineering.mdc` and `docs/engineering/COLLISION_POLICY.md`.
4. Do **not** use destructive Git commands or touch other worktrees.
5. Change only Allowed Paths.
6. Report **NOT_RUN** for checks not executed.

## Workflow

1. Investigate — read `services/api-gateway/internal/controltower/events.go`, existing OpenAPI Control Tower section, PILOT_PLAN.md.
2. Plan — confirm endpoint path and schemas align with derived-event model.
3. Implement — OpenAPI + ARCHITECTURE.md only.
4. Validate — openapi-validate, git diff --check.
5. Handoff — record CONTRACT_FREEZE_SHA prominently.

Do not commit or push unless Task Contract authorizes it.

## Worktree creation (human/orchestrator step before agent run)

```powershell
git fetch --prune origin
git worktree add D:\Projects\freight-platform-wt\ct-alert-ack-contract -b arch/control-tower-alert-ack-contract-v0.1 origin/main
```

Open `D:\Projects\freight-platform-wt\ct-alert-ack-contract` in a dedicated Cursor window.
