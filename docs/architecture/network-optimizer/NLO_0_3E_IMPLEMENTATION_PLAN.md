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
2. `NLO03E_I001`: accepted numeric values for `MAX_CANDIDATE_POOL`, `MAX_SET_SIZE`, `MAX_SETS_EVALUATED`, `MAX_GROUPAGE_CALLS`, `MAX_ROUTING_CALLS`, and `TIME_BUDGET`. This discovery leaves them `UNSET` because the evidence is an in-memory store and a fake routing provider.
3. `NLO03E_I002`: a migration, not written here, that can store more than two members, plus generated OpenAPI for that member list. `000081` checks `ordinal IN (1, 2)`. OpenAPI `members.maxItems` is 2 and `ordinal` is enum `[1, 2]`. `NEW_MIGRATION_REQUIRED=YES`. `PROPOSED_MIGRATION=NONE`. Do not reserve `000084` in discovery.
4. `NLO03E_I003`: a server-owned evaluation budget that is not `candidate_limit`. `candidate_limit` stays the returned-result cap.

## Implementation order, after those blockers

1. Server-owned policy with the accepted numbers. Fail closed on pool, search, and routing budgets before enumeration.
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

Optional later metrics, low cardinality only: a counter of budget exhaustions labelled with `POOL_LIMIT_EXCEEDED`, `SEARCH_BUDGET_EXCEEDED`, or `ROUTING_BUDGET_EXCEEDED`. No ids. No raw set size as a label. The series named in older notes (`bno_consolidation_hard_reject_total`, `bno_consolidation_indeterminate_total`, `bno_consolidation_set_size_bucket`) are not emitted today and are not added here.
