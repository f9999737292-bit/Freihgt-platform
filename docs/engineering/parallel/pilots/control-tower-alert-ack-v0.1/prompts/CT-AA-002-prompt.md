# Agent Prompt — CT-AA-002

## Assignment

You are the **backend-engineer** agent for the 7Rights Freight Platform.

**Task ID:** CT-AA-002

**Repository / worktree:** D:\Projects\freight-platform-wt\ct-alert-ack-backend

**Branch:** feat/control-tower-alert-ack-backend-v0.1

**Base SHA:** `<CONTRACT_FREEZE_SHA from CT-AA-001 handoff — NOT origin/main>`

## Objective

Implement tenant-scoped idempotent critical event acknowledgement: migration 000020, read-model persistence + internal API, gateway public POST handler, summary enrichment with batch ack lookup. Identity from gateway AuthContext only.

## Allowed paths

- `infrastructure/migrations/000020_create_control_tower_critical_event_acknowledgement_v0.1.up.sql`
- `infrastructure/migrations/000020_create_control_tower_critical_event_acknowledgement_v0.1.down.sql`
- `services/control-tower-read-model-service/internal/repository/**` (ack)
- `services/control-tower-read-model-service/internal/http/handlers/**` (ack)
- `services/control-tower-read-model-service/internal/http/router.go`
- `services/control-tower-read-model-service/internal/domain/**` (ack)
- `services/api-gateway/internal/controltower/**`
- `services/api-gateway/internal/controltowerreadmodel/**`
- `services/api-gateway/internal/http/router.go` (one POST route)

## Forbidden paths

- `packages/openapi/**`
- `apps/**`
- Existing migrations 000001–000019

## Dependencies

CT-AA-001 complete; branch from CONTRACT_FREEZE_SHA

## Acceptance criteria

1. POST `/api/v1/control-tower/critical-events/{eventId}/acknowledge` works per frozen OpenAPI.
2. Idempotent repeat returns 200 with first ack preserved.
3. GET summary enriches criticalEvents with acknowledgement when present.
4. All queries tenant-scoped; foreign event → 404.
5. Migration 000020 up/down clean.
6. Targeted go tests PASS.

## Required validation level

1 (2 if cross-service integration tests added)

## Safety rules (mandatory)

1. Run git context commands first.
2. Read Task Contract: `contracts/CT-AA-002.md` and frozen ARCHITECTURE.md.
3. Read `.cursor/rules/20-security-tenancy.mdc`.
4. No destructive Git; no other worktrees touched.
5. Allowed paths only.
6. Honest NOT_RUN reporting.

## Worktree creation

```powershell
git fetch origin
git worktree add D:\Projects\freight-platform-wt\ct-alert-ack-backend -b feat/control-tower-alert-ack-backend-v0.1 <CONTRACT_FREEZE_SHA>
```

**Wait for CT-AA-001 CONTRACT_FREEZE_SHA before creating this worktree.**
