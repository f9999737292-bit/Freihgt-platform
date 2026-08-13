# Task Registry — CT-AA-003

| Field | Value |
|-------|-------|
| Task ID | CT-AA-003 |
| Title | Control Tower alert ack — frontend |
| Owner | orchestrator |
| Agent role | frontend-engineer |
| Branch | feat/control-tower-alert-ack-frontend-v0.1 |
| Worktree | D:\Projects\freight-platform-wt\ct-alert-ack-frontend |
| Status | INTEGRATED |
| Base branch | CONTRACT_FREEZE_SHA |
| Base SHA | (from CT-AA-001 handoff) |
| Integration order | 2 |

## Allowed paths

- `apps/web-admin/types/controlTower.ts`
- `apps/web-admin/composables/useControlTower.ts`
- `apps/web-admin/components/control-tower/CriticalEventsPanel.vue`
- `apps/web-admin/locales/**` (controlTower ack keys)

## Forbidden paths

- `packages/openapi/**`
- `services/**`

## Dependencies

- CT-AA-001

## Validation Level

1

## Parallel

Yes — with CT-AA-002 after contract freeze

## Notes

Full contract: `pilots/control-tower-alert-ack-v0.1/contracts/CT-AA-003.md`
