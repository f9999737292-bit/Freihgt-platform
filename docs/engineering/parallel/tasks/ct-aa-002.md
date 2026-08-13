# Task Registry — CT-AA-002

| Field | Value |
|-------|-------|
| Task ID | CT-AA-002 |
| Title | Control Tower alert ack — backend |
| Owner | orchestrator |
| Agent role | backend-engineer |
| Branch | feat/control-tower-alert-ack-backend-v0.1 |
| Worktree | D:\Projects\freight-platform-wt\ct-alert-ack-backend |
| Status | INTEGRATED |
| Base branch | CONTRACT_FREEZE_SHA |
| Base SHA | (from CT-AA-001 handoff) |
| Integration order | 2 |

## Allowed paths

- `infrastructure/migrations/000020_*`
- `services/control-tower-read-model-service/**` (ack scope)
- `services/api-gateway/internal/controltower/**`
- `services/api-gateway/internal/controltowerreadmodel/**`
- `services/api-gateway/internal/http/router.go` (one route)

## Forbidden paths

- `packages/openapi/**`
- `apps/**`

## Dependencies

- CT-AA-001

## Validation Level

1 (2 if integration tests added)

## Parallel

Yes — with CT-AA-003 after contract freeze

## Notes

Full contract: `pilots/control-tower-alert-ack-v0.1/contracts/CT-AA-002.md`
