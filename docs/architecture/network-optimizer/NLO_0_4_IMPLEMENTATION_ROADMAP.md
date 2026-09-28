# NLO-0.4 implementation roadmap

Discovery baseline: `origin/main` `b108c6cc62a23aaa9f5f866cf53c8d281fc14783`. NLO-0.3 is complete. NLO-0.4A architecture is accepted. NLO-0.4B is IMPLEMENTED_CLOSED on main in PR #184 at `0fc6a7979ca5ea0cbbbd50ff5770bbbf22304a5c`. Feature head `b2d3b16bdcb30d1918b6bcc5074ee1d651c2739b`. CI run `36403539278` succeeded on that feature head. NLO-0.4C accept and activate is implemented and not yet accepted. NLO-0.4D is not started.

```text
NLO_0_4_STARTED=YES
NLO_0_4A=ARCHITECTURE_FROZEN_ACCEPTED
NLO_0_4_IMPLEMENTATION_STARTED=YES
NLO_0_4B_IMPLEMENTATION_AUTHORIZED_AFTER_PR182_MERGE=YES
NLO_0_4B_IMPLEMENTATION_STARTED=YES
NLO_0_4B_STATUS=IMPLEMENTED_CLOSED
NLO_0_4B_MERGED=YES
NLO_0_4B_PR=184
NLO_0_4B_FEATURE_HEAD=b2d3b16bdcb30d1918b6bcc5074ee1d651c2739b
NLO_0_4B_MERGE_SHA=0fc6a7979ca5ea0cbbbd50ff5770bbbf22304a5c
NLO_0_4B_CI=36403539278
NLO_0_4C_STARTED=YES
NLO_0_4C_STATUS=IMPLEMENTED_REMEDIATED_PENDING_CONTROLLER_REVIEW
NLO_0_4C_ACCEPTED=NO
NLO_0_4D_STARTED=NO
ARCHITECTURE_FROZEN=YES
TEST_STRATEGY_FROZEN=YES
BLOCKING_FINDINGS=0
MIGRATION_RESERVED=NO
NLO_0_4C_ACTIVATION_RELEASE_BLOCKED_UNTIL_SERVICE_DURATION_SOURCE=YES
NLO04A_ERRATUM_E1=ACCEPTED
NLO04A_POST_ACCEPT_F001=CLOSED
ERRATUM_CONTROLLER_ACCEPTANCE=PASS
NLO_0_4A_CLOSED=YES
NLO_0_4B_IMPLEMENTATION_AUTHORIZED=YES
NLO_0_4B_IMPLEMENTED=YES
NLO_0_4B_ACCEPTED=YES
NLO_0_4B_CLOSED=YES
CONTROLLER_REVIEW_PENDING=NO
EXISTING_FUTURE_ROUTE_LOAD_SOURCE_IN_0_4B=NONE
```

Post-freeze erratum E1 is accepted in ADR-NET-019. It corrects start-anchor identity. It does not start NLO-0.4B.

## Waves

### NLO-0.4A — architecture discovery

This publication. Route plan, stop, action, leg, bounded insertion, and the accept/activate split. ADR-NET-018 through ADR-NET-021 are Accepted. `NLO_0_4A=ARCHITECTURE_FROZEN_ACCEPTED`.

### NLO-0.4B — persistent plan and bounded planner

Status: IMPLEMENTED_CLOSED. Merged to main in PR #184. Feature head `b2d3b16bdcb30d1918b6bcc5074ee1d651c2739b`. Merge SHA `0fc6a7979ca5ea0cbbbd50ff5770bbbf22304a5c`. CI run `36403539278` succeeded on that feature head, including `network-optimizer-nlo04b-integration`. Controller acceptance passed after privacy remediation R2. `NLO_0_4B_IMPLEMENTED=YES` and `NLO_0_4B_ACCEPTED=YES`. `CONTROLLER_REVIEW_PENDING=NO`. `NLO_0_4C_STARTED=YES` and `NLO_0_4D_STARTED=NO`. The route-plan public load snapshot omits foreign owner tenant ids and commercial terms. Migration `000085_nlo_route_plan_bounded_planner_v0_4b` persists planning tables only and was corrected in place. `route_plan_activations` is not created. `execution_supported` stays false. An unknown routing profile or unknown service duration keeps the plan advisory and indeterminate. A known late first stop is still a hard reject. `CURRENT_TRIP_FILL` stays at one additional load. `EXISTING_FUTURE_ROUTE_LOAD_SOURCE_IN_0_4B=NONE`. The evaluate API is the multi-stop entry. Accept and activate remain NLO-0.4C.

```text
NLO04B_IMPL_F001=CLOSED
NLO04B_IMPL_F002=CLOSED
NLO04B_IMPL_F003=CLOSED
NLO04B_IMPL_F004=CLOSED
NLO04B_IMPL_F005=CLOSED
NLO04B_IMPL_F006=CLOSED
NLO04B_IMPL_F007=CLOSED
NLO04B_IMPL_F008=CLOSED
RUNTIME_BLOCKING_FINDINGS=0
IMPLEMENTATION_ACCEPTANCE_HEAD=777e47765ce534d2e12a04695458a107fbb603aa
IMPLEMENTATION_ACCEPTANCE_CI=36400921559
CONTROLLER_ACCEPTANCE=PASS
```

### NLO-0.4C — accept and activate contract

Status: IMPLEMENTED_REMEDIATED_PENDING_CONTROLLER_REVIEW. Controller acceptance is not claimed. `NLO_0_4C_STARTED=YES`. `NLO_0_4C_ACCEPTED=NO`. `NLO_0_4D_STARTED=NO`.

Accept freezes an evaluated plan at `ACCEPTED` when dependency versions and the current-trip context fingerprint still match. Public activate writes one `route_plan_activations` row with status `PENDING_EXECUTION`. The plan status stays `ACCEPTED`. `execution_id` and `execution_revision_id` stay null. A pending successor does not supersede a previous execution-linked plan. `EXECUTION_LINKED` is reached only after a future execution projection stores those ids. `TMS_PROJECTION_HANDSHAKE_IMPLEMENTED=NO`. `NETWORK_ROUTE_PLAN_EXECUTION_LINKED_IMPLEMENTED=NO`. `NLO_ACTIVATION_SCHEMA_READY_FOR_FUTURE_HANDSHAKE=YES`. It does not mutate shipment stops or set `execution_supported`. Unknown service duration refuses activation. `NLO_0_4C_ACTIVATION_RELEASE_BLOCKED_UNTIL_SERVICE_DURATION_SOURCE=YES`. Migration `000086_nlo_route_plan_accept_activate_v0_4c`.

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
