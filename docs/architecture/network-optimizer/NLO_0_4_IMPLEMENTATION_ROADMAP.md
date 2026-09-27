# NLO-0.4 implementation roadmap

Discovery baseline: `origin/main` `b108c6cc62a23aaa9f5f866cf53c8d281fc14783`. NLO-0.3 is complete. This roadmap is proposed. Nothing below is authorized to build.

```text
NLO_0_4_STARTED=YES
NLO_0_4_IMPLEMENTATION_STARTED=NO
NLO_0_4_IMPLEMENTATION_AUTHORIZED=NO
ARCHITECTURE_FROZEN=NO
MIGRATION_RESERVED=NO
```

## Waves

### NLO-0.4A — architecture discovery

This publication. Route plan, stop, action, leg, bounded insertion, and the accept/activate split. ADR-NET-018 through ADR-NET-021 are Proposed.

### NLO-0.4B — persistent plan and bounded planner

Proposed next implementation, only after controller acceptance of 0.4A. Persist the planning tables in a new migration numbered at implementation time. Do not reserve `000085` now. Implement incremental insertion with the caps in `NLO_0_4A_MULTI_STOP_ALGORITHM.md`. `execution_supported` stays false. `CURRENT_TRIP_FILL` stays at one additional load until a product decision moves that public pattern. The new route-plan evaluate API is the multi-stop entry.

### NLO-0.4C — accept and activate contract

Accept freezes the plan. Activate writes the activation row and enforces `PLAN_STALE`, routing expiry, and idempotency. It still does not mutate shipment stops.

### NLO-0.4D — shipment and driver execution

Shipment-service gains an execution projection for ordered stops, or an equivalent compatible model, which ADR-NET-005 already requires before any multi-stop plan becomes executable. Driver stop tasks are shipment-owned. Slot booking stays outside the optimizer.

## Not in NLO-0.4

Unrestricted VRP, CVRP, VRPTW, MILP, CP-SAT, LNS, genetic algorithms, ML routing, fleet-wide optimization, backhaul, round trip, 3D packing, and price optimization. Those stay NLO-0.5 or later.

## Preserved NLO-0.3 behavior

| Wave | Contract that stays |
| --- | --- |
| 0.3B | Pairwise same origin and destination, two members, `evaluated_pair_count` |
| 0.3C | Server-built current-trip context |
| 0.3D | Exactly one additional load, existing routing, `evaluated_pair_count` |
| 0.3E | Same-origin N-member sets, size at most 3, no route sequence, `evaluated_set_count` |
