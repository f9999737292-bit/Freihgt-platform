# Agent Prompt — CT-AA-001

## Assignment

You are the **architect** agent for the Bintrans Freight Platform.

**Task ID:** CT-AA-001

**Repository / worktree:** D:\Projects\freight-platform-wt\ct-alert-ack-contract

**Branch:** arch/control-tower-alert-ack-contract-v0.1

**ORCHESTRATION_BASE_SHA:** `<HEAD of chore/control-tower-alert-ack-orchestration-v0.1 at worktree creation>`

**PRODUCT_BASE_SHA:** `02208106e494afcaa46372e44b417761d6613daf` (origin/main product reference — not the worktree start point)

## Objective

Freeze the v0.1 API contract and architecture for Control Tower critical event acknowledgement. Critical events are **derived** in api-gateway (`BuildCriticalEvents`, `deterministicEventID`). Acknowledgement persists in `control_tower.critical_event_acknowledgement` (read-model). Deliver **CONTRACT_FREEZE_SHA**.

**Critical:** Do not assume `deterministicEventID` is automatically a permanent acknowledgement identity — analyze each event type first.

## Allowed paths

- `packages/openapi/openapi.yaml`
- `docs/engineering/parallel/pilots/control-tower-alert-ack-v0.1/ARCHITECTURE.md`

## Forbidden paths

- `services/**`
- `apps/**`
- `infrastructure/migrations/**`

## Dependencies

Worktree already created from ORCHESTRATION_BASE_SHA (orchestration artifacts present)

## Acceptance criteria

1. OpenAPI: `POST /api/v1/control-tower/critical-events/{eventId}/acknowledge` — empty body, JWT auth, 200/400/401/403/404.
2. Extend `ControlTowerEvent` with optional `acknowledgement` object.
3. ARCHITECTURE.md: persistence, internal endpoints, **per-event-type identity matrix** (SHIPMENT_CANCELLED, PICKUP_DELAY, DELIVERY_DELAY, STALE_UPDATES, MISSING_DOCUMENTS, TECHNICAL_PROBLEM) covering same occurrence / resolved / re-triggered.
4. ARCHITECTURE.md: **existence validation boundary** — gateway derives/matches events before read-model persist; no blind opaque eventId storage.
5. ARCHITECTURE.md: **mutation authorization decision** — view vs acknowledge (`CanAccessControlTower` or narrower); rationale required.
6. ARCHITECTURE.md: **idempotency table** — first ack, repeat same/different user, resolved/non-current event; preserve original actor/time unless explicitly chosen otherwise.
7. Handoff records **CONTRACT_FREEZE_SHA**.
8. `make openapi-validate` PASS or NOT_RUN with reason.

## Required validation level

1 — see `docs/engineering/VALIDATION_LEVELS.md`

## Safety rules (mandatory)

1. Run `git rev-parse --show-toplevel`, `git branch --show-current`, `git rev-parse HEAD`, `git status --short` first.
2. Read full Task Contract: `docs/engineering/parallel/pilots/control-tower-alert-ack-v0.1/contracts/CT-AA-001.md`
3. Read `services/api-gateway/internal/controltower/events.go` and `rbac.go`.
4. Read `.cursor/rules/05-parallel-engineering.mdc` and `docs/engineering/COLLISION_POLICY.md`.
5. Do **not** use destructive Git commands or touch other worktrees.
6. Change only Allowed Paths.
7. Report **NOT_RUN** for checks not executed.

## Workflow

1. Investigate — events.go identity inputs per type; existing OpenAPI; PILOT_PLAN.md.
2. Plan — prove or revise event identity; validation boundary; RBAC; idempotency.
3. Implement — OpenAPI + ARCHITECTURE.md only.
4. Validate — openapi-validate, git diff --check.
5. Handoff — record CONTRACT_FREEZE_SHA prominently.

Do not commit or push unless Task Contract authorizes it.

## Worktree (already created by orchestrator)

```powershell
# Created from ORCHESTRATION_BASE_SHA — NOT origin/main
git worktree add D:\Projects\freight-platform-wt\ct-alert-ack-contract -b arch/control-tower-alert-ack-contract-v0.1 <ORCHESTRATION_BASE_SHA>
```

Open `D:\Projects\freight-platform-wt\ct-alert-ack-contract` in a dedicated Cursor window.
