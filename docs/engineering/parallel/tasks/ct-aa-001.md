# Task Registry — CT-AA-001

| Field | Value |
|-------|-------|
| Task ID | CT-AA-001 |
| Title | Control Tower alert ack — contract / architecture |
| Owner | orchestrator |
| Agent role | architect |
| Branch | arch/control-tower-alert-ack-contract-v0.1 |
| Worktree | D:\Projects\freight-platform-wt\ct-alert-ack-contract |
| Status | INTEGRATED |
| Base branch | chore/control-tower-alert-ack-orchestration-v0.1 |
| Base SHA | ORCHESTRATION_BASE_SHA (not origin/main) |
| PRODUCT_BASE_SHA | 02208106e494afcaa46372e44b417761d6613daf |
| Integration order | 1 |

## Allowed paths

- `packages/openapi/openapi.yaml`
- `docs/engineering/parallel/pilots/control-tower-alert-ack-v0.1/ARCHITECTURE.md`

## Forbidden paths

- `services/**`
- `apps/**`
- `infrastructure/migrations/**`

## Dependencies

- Orchestration branch pushed; contract worktree from ORCHESTRATION_BASE_SHA

## Validation Level

1

## Notes

Must record CONTRACT_FREEZE_SHA in handoff. Worktree starts from ORCHESTRATION_BASE_SHA so pilot docs are available. Full contract: `pilots/control-tower-alert-ack-v0.1/contracts/CT-AA-001.md`

## CT-AA-001 mandatory architect decisions

- Per-event-type derived identity analysis (do not assume deterministicEventID is permanent)
- Acknowledgement existence validation boundary (gateway derive/match before persist)
- Mutation authorization (view vs acknowledge)
- Idempotency semantics (preserve original actor/time on repeat)
