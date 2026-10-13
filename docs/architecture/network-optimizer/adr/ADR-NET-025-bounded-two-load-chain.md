# ADR-NET-025: Bounded two-load chain

## Status

Accepted as an architecture freeze. The chain runtime is not implemented and is not authorized.

```text
ADR_NET_025=ACCEPTED_ARCHITECTURE_ONLY
NLO_0_9B_STATUS=ARCHITECTURE_FROZEN
RUNTIME_IMPLEMENTATION_AUTHORIZED=NO
CHAIN_RUNTIME_IMPLEMENTED=NO
RECOMMENDED_FIRST_CHAIN_RUNTIME_SHAPE=TWO_LOAD_CHAIN_TOTAL
GLOBAL_CHAIN_OPTIMUM_CLAIM_ALLOWED=NO
MIGRATION_CREATED_IN_0_9B=NO
OPENAPI_CHANGED_IN_0_9B=NO
```

The decision body is [NLO_0_9B_CHAIN_ARCHITECTURE_FREEZE.md](../NLO_0_9B_CHAIN_ARCHITECTURE_FREEZE.md). This ADR records the frozen values. It supersedes the unfrozen policy markers in the NLO-0.9A discovery. It does not change ADR-NET-015 opt-in rules or ADR-NET-024 one-load search budgets.

## Context

NLO-0.9A showed that next-load search returns one load, that the server does not compose a second load, and that route-plan evaluate scores only caller-selected ids. Seven blockers remained: horizon, chain budget, objective, identity and discovery persistence, cross-shipper disclosure, stacking, and load-order selection. The discovery pool of 1000 and the routed shortlist of 25 make an unbounded pair search too large to leave unfrozen.

## Decision

```text
CHAIN_HORIZON_POLICY=FIXED_CLOCK_48H
FIRST_LOAD_SHORTLIST_CAP=8
SECOND_LOADS_PER_FIRST_CAP=2
CHAIN_PAIR_CAP=4
ROUTEPLAN_EVALUATION_CAP=2
CHAIN_GLOBAL_PROVIDER_CALL_CAP=24
CHAIN_GLOBAL_WALLCLOCK_BUDGET=20s
CHAIN_OBJECTIVE_POLICY=LEXICOGRAPHIC_EMPTY_ROAD_THEN_DURATION
CHAIN_SEARCH_IDENTITY=SERVER_UUID_TENANT_SCOPED
CHAIN_CANDIDATE_IDENTITY=SERVER_UUID_BEFORE_EVALUATION
CHAIN_EVALUATION_IDENTITY=ROUTE_PLAN_ID_AFTER_EVALUATION_ONLY
CHAIN_DISCOVERY_PERSISTENCE=SEARCH_RUN_AND_SHORTLIST
MIGRATION_REQUIRED_IN_FUTURE_RUNTIME=YES
CHAIN_PRE_ACCEPT_PRIVACY_SURFACE=INDIVIDUAL_PUBLICATION_SCOPE_PLUS_OWN_ROUTE_METRICS
CHAIN_POST_ACCEPT_PRIVACY_SURFACE=EXISTING_ACCEPTED_ROUTE_PLAN_AND_TMS
STACKING_POLICY=OVERLAP_REQUIRES_KNOWN_STACKABLE_NO_EXTRA_SPACE
STACKING_UNKNOWN_BEHAVIOR=REJECT
ROUTEPLAN_SELECTION_POLICY=BOUNDED_TWO_ORDER_COMPARISON
FIRST_RUNTIME_SEED_SOURCE=MANUAL_AVAILABLE_AND_CURRENT_TRIP_RESIDUAL
PREDICTED_CAPACITY_IN_FIRST_RUNTIME=NO
CHAIN_API_RESOURCE_SHAPE=SEPARATE_CHAIN_SEARCH_RESOURCE
CHAIN_API_PATH=POST /api/v1/network/chain-searches
CHAIN_API_VISIBILITY=PUBLIC_AUTHENTICATED
EXECUTION_SUPPORT_BOUNDARY=DISABLED_THROUGH_0_9C_UNTIL_0_9D_PROOF
```

A budget-exhausted search is `CHAIN_BUDGET_EXHAUSTED` with `search_complete=false`. It is not a complete search and not a global optimum. Missing road distance and missing revenue are omitted or excluding facts. They are not zeroes. `match_candidates` is not a two-load row. The client cannot submit L1 and L2 to the chain-search resource.

## Consequences

- NLO-0.9C may be authorized later against these values. This ADR does not start it.
- A future migration is required for the search run and shortlist. No number is reserved here.
- No OpenAPI change is included.
- Stacking-unknown overlap is a reject. That closes the policy gap and does not claim a layout engine.
- Predicted capacity remains out of the first runtime.
- NLO-0.9B-R1 sets the chain wall clock to 20s. `PUBLIC_SYNC_CHAIN_REQUEST_MUST_COMPLETE_BEFORE_GATEWAY_30S_TIMEOUT=YES`. The gateway write timeout is 30s, so a 120s chain budget is not a public request. Changing only `PROXY_TIMEOUT_SECONDS` does not remove that write timeout.
