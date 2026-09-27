# NLO-0.3E implementation plan

```text
NLO_0_3E_IMPLEMENTATION_STARTED=NO
NLO_0_3E_IMPLEMENTATION_AUTHORIZED=NO
PRODUCT_CODE_CHANGED=NO
RUNTIME_BEHAVIOR_CHANGED=NO
MIGRATION_ADDED=NO
```

This plan is the sequence a later task would follow after controller acceptance. It is not authorization to start.

## Blockers before any runtime change

1. Controller acceptance of ADR-NET-017. Review R1 is `CHANGES_REQUIRED`. `ARCHITECTURE_FROZEN=NO`.
2. `NLO03E_I001` values from the disposable Postgres measurement: `MAX_CANDIDATE_POOL=10` for a complete size-2 and size-3 search, `MAX_SET_SIZE=3`, `MAX_SETS_EVALUATED=1225`, `MAX_GROUPAGE_CALLS=2450`, `TIME_BUDGET=5s`. `NLO03E_MAX_ROUTING_CALLS=0`. `REAL_ROUTING_PROVIDER_BUDGET_REQUIRED_FOR_0_3E=NO`. A 2GIS load test is not required for this same-origin expansion. Controller acceptance of these defaults is still required before runtime work.
3. `NLO03E_I002`: a migration, not written here, that can store more than two members, plus generated OpenAPI for that member list. `000081` checks `ordinal IN (1, 2)`. OpenAPI `members.maxItems` is 2 and `ordinal` is enum `[1, 2]`. `NEW_MIGRATION_REQUIRED=YES`. `PROPOSED_MIGRATION=NONE`. Do not reserve `000084` in discovery.
4. `NLO03E_I003`: a server-owned evaluation budget that is not `candidate_limit`. `candidate_limit` stays the returned-result cap.

## Implementation order, after those blockers

1. Server-owned policy with the accepted numbers. Fail closed on pool and search budgets before enumeration. Do not add a routing budget to NLO-0.3E.
2. Lexicographic set generation for same-origin groups, size 2 through `MAX_SET_SIZE`, stop at `MAX_SETS_EVALUATED`. No stop list.
3. Groupage and residual snapshot reuse. No second engine. Sequence-independent failure may be `HARD_REJECT`. Do not treat one stop order as global infeasibility, because NLO-0.3E explores no stop orders.
4. Persist N members only after the migration. Fingerprint includes every member version and compatibility provenance.
5. Keep cross-shipper sets `INDETERMINATE`.
6. Do not add `ConsolidationPlanningScore`, MatchScore, or client weights.
7. Do not raise `MAX_ADDITIONAL_LOADS` above 1.
8. Tests in `NLO_0_3E_TEST_STRATEGY.md` after that strategy is accepted. `TEST_STRATEGY_FROZEN=NO`.

## Explicitly out

Shipment activation, multi-stop planning, multi-stop execution, `RoutePlan` activation, `RouteStop` execution, driver multi-stop tasks, assignment, reservation, carrier offer, slot booking, VRP, MILP, CP-SAT, LNS, ML, genetic algorithms, 3D bin packing, unbounded subset enumeration, unbounded route permutation, a single canonical sequence used as global infeasibility, backhaul, round trip, Moscow city rules, Saint Petersburg city rules, and network-wide optimization.

`CURRENT_TRIP_FILL` above one additional load, `MULTI_PICK_ONE_DROP`, `ONE_PICK_MULTI_DROP`, and `MULTI_PICK_MULTI_DROP` stay in NLO-0.4.

## API and metrics notes for that later task

Extend pattern semantics on the existing search. Do not add an endpoint in discovery, and do not add one unless the accepted ADR is revised.

Optional later metrics, low cardinality only: a counter of budget exhaustions labelled with `POOL_LIMIT_EXCEEDED` or `SEARCH_BUDGET_EXCEEDED`. No ids. No raw set size as a label. `ROUTING_BUDGET_EXCEEDED` is not an NLO-0.3E label. The series named in older notes (`bno_consolidation_hard_reject_total`, `bno_consolidation_indeterminate_total`, `bno_consolidation_set_size_bucket`) are not emitted today and are not added here.
