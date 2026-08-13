# Task Contract

## Task ID

CT-AA-002

## Title

Control Tower alert acknowledgement — backend implementation

## Owner

orchestrator

## Role

backend-engineer

## Repository

D:\Projects\freight-platform-wt\ct-alert-ack-backend

## Base branch

CONTRACT_FREEZE_SHA (from CT-AA-001 handoff)

## Base SHA

`<CONTRACT_FREEZE_SHA>` — do not use floating origin/main

## Working branch

feat/control-tower-alert-ack-backend-v0.1

## Worktree

D:\Projects\freight-platform-wt\ct-alert-ack-backend

---

## Objective

Implement tenant-scoped, idempotent critical event acknowledgement: migration 000020, read-model persistence + internal API, gateway public POST + summary enrichment with batch ack lookup. Trusted identity from gateway auth context only.

## In scope

- Migration `000020_create_control_tower_critical_event_acknowledgement_v0.1.{up,down}.sql`
- Read-model: repository, handler, router registration for internal acknowledge + batch lookup
- Gateway: acknowledge handler, read-model client extension, summary enrichment in `GetSummary` / `BuildCriticalEvents` merge step
- Gateway: route `POST /api/v1/control-tower/critical-events/{eventId}/acknowledge` in router.go (single route addition)
- Targeted unit/integration tests in both services

## Out of scope

- OpenAPI edits (frozen in CT-AA-001 — if gap found, STOP and request contract amendment)
- Frontend changes
- New RBAC roles or permission seeds
- Unacknowledge, comments, bulk ack, notifications
- New microservice

## Allowed paths

- `infrastructure/migrations/000020_create_control_tower_critical_event_acknowledgement_v0.1.up.sql`
- `infrastructure/migrations/000020_create_control_tower_critical_event_acknowledgement_v0.1.down.sql`
- `services/control-tower-read-model-service/internal/repository/**` (ack-related)
- `services/control-tower-read-model-service/internal/http/handlers/**` (ack-related)
- `services/control-tower-read-model-service/internal/http/router.go`
- `services/control-tower-read-model-service/internal/domain/**` (ack-related only)
- `services/control-tower-read-model-service/internal/integration/**` (ack-related tests only)
- `services/api-gateway/internal/controltower/**` (ack handler, enrichment, models)
- `services/api-gateway/internal/controltowerreadmodel/**` (client methods for ack)
- `services/api-gateway/internal/http/router.go` (one POST route line only)
- `services/api-gateway/internal/integration/controltower/**` (ack-related tests only)

## Forbidden paths

- `packages/openapi/**`
- `apps/**`
- `infrastructure/migrations/0000{01..019}_*` (existing migrations)
- `services/api-gateway/internal/shipmentrbac/**`
- `Makefile`, `go.work`, `.github/**`

## Dependencies

- CT-AA-001 complete with recorded CONTRACT_FREEZE_SHA

## Security invariants

- All DB queries include `tenant_id` predicate
- `acknowledged_by_user_id` from gateway `AuthContext.UserID` only
- Reject client body identity fields if present
- Cross-tenant event_id → 404
- Preserve gateway middleware auth strip of spoofed identity headers

## Acceptance criteria

1. POST acknowledge persists ack idempotently per (tenant_id, event_id).
2. GET summary returns criticalEvents with acknowledgement block when ack exists.
3. Repeated POST returns 200 with original ack (first actor preserved).
4. Unauthorized/forbidden/foreign cases match frozen OpenAPI.
5. Migration 000020 applies cleanly; down migration reverses.
6. Targeted tests PASS for changed packages.

## Required validation

Level: 1 (Level 2 for gateway+read-model integration tests if added)

Commands:

- `git diff --check`
- `go test ./...` in `services/control-tower-read-model-service` (targeted paths)
- `go test ./...` in `services/api-gateway/internal/controltower/...` and ack-related tests

## Required deliverables

- Implementation diff within Allowed Paths
- Handoff with final SHA, test results

## Integration target

int/control-tower-alert-ack-v0.1 → main

## Handoff requirements

- CONTRACT_FREEZE_SHA used as base documented
- Migration number 000020 confirmed
- List of new routes (public + internal)
- Validation results with NOT_RUN explicit
