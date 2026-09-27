# NLO-0.3E implementation plan

```text
NLO_0_3E_ARCHITECTURE=FROZEN_ACCEPTED
NLO_0_3E_TEST_STRATEGY=FROZEN_ACCEPTED
NLO_0_3E_IMPLEMENTATION_STARTED=NO
NLO_0_3E_IMPLEMENTATION_AUTHORIZED=NO
PRODUCT_CODE_CHANGED=NO
RUNTIME_BEHAVIOR_CHANGED=NO
MIGRATION_ADDED=NO
```

ADR-NET-017 is accepted. This plan is the sequence a later task would follow on a new branch from merged main. It is not authorization to start, and this publication does not start it.

## Gates before any runtime change

1. ADR-NET-017 is accepted. `ADR_STATUS=ACCEPTED`. `ARCHITECTURE_FROZEN=YES`. `TEST_STRATEGY_FROZEN=YES`. `NLO_0_3E_IMPLEMENTATION_AUTHORIZED=NO`.
2. `NLO03E_I001` is resolved for `SAME_ORIGIN_SAME_DESTINATION_N_MEMBER` only. `MAX_CANDIDATE_POOL=10` is the global visible eligible pool. `MAX_SET_SIZE=3`. `MAX_SETS_EVALUATED=165`. `MAX_GROUPAGE_CALLS=330`. `TIME_BUDGET=5s` is a watchdog, not an SLO. `NLO03E_MAX_ROUTING_CALLS=0`. Legacy `SAME_ORIGIN_SAME_DESTINATION` stays pairwise. Implementation authorization is still required before runtime work.
3. `NLO03E_I002` design is frozen, and no migration file is created. The later migration admits exactly three patterns, requires capacity for both same-origin patterns, keeps current-trip capacity null, adds `evaluated_set_count`, and keeps `evaluated_pair_count` for pairwise and for current-trip only. An N-member run stores `evaluated_set_count` and leaves `evaluated_pair_count` null. Ordinal becomes `BETWEEN 1 AND 3`. Primary key, load uniqueness, candidate foreign key, and the composite run foreign key stay. The down migration aborts if an N-member pattern row or an ordinal-3 member exists. It does not delete or rewrite those rows. `000084` is not reserved.
4. `NLO03E_I003` is resolved at design level for the N-member pattern. `candidate_limit` stays the returned-result cap. Pool, set, groupage, and time budgets are checked before the work they bound. A budget failure rolls back and does not return a partial candidate list. Those codes are not added to legacy pairwise search.

## Future migration shape, not a file

`MIGRATION_CREATED=NO`. `MIGRATION_RESERVED=NO`. The checks below are the design a later migration would apply. Exact text is also in `NLO_0_3E_BOUNDED_EXPANSION_MODEL.md`.

- Pattern check admits `SAME_ORIGIN_SAME_DESTINATION`, `SAME_ORIGIN_SAME_DESTINATION_N_MEMBER`, and `CURRENT_TRIP_FILL` only.
- Context check: both same-origin patterns require `capacity_id` and `capacity_version`. `CURRENT_TRIP_FILL` requires both null.
- Add `evaluated_set_count integer NULL`. Pairwise and current-trip rows require `evaluated_pair_count` and a null set count. N-member rows require `evaluated_set_count` and a null pair count.
- Ordinal check becomes `ordinal BETWEEN 1 AND 3`. Primary key, load unique key, and candidate foreign key stay.
- Down migration aborts if an N-member run or an ordinal-3 member exists. No delete, no pattern rewrite, no copy of the set count into the pair count.

## Implementation order, after those blockers

1. Server-owned policy for `SAME_ORIGIN_SAME_DESTINATION_N_MEMBER` with the accepted numbers. Fail closed on pool and search budgets before enumeration. Do not add a routing budget to NLO-0.3E. Do not apply that pool bound to legacy pairwise search.
2. Lexicographic set generation for the N-member pattern, size 2 through `MAX_SET_SIZE`, stop at `MAX_SETS_EVALUATED`. No stop list. Leave `SAME_ORIGIN_SAME_DESTINATION` pairwise.
3. Groupage and residual snapshot reuse. No second engine. Sequence-independent failure may be `HARD_REJECT`. Do not treat one stop order as global infeasibility, because NLO-0.3E explores no stop orders.
4. Persist N members only after that migration. The stored `pattern` is the request pattern. The N-member run writes `evaluated_set_count` and leaves `evaluated_pair_count` null. The N-member fingerprint includes `SAME_ORIGIN_SAME_DESTINATION_N_MEMBER` and the budget policy version. Do not infer the mode from member count.
5. Keep cross-shipper sets `INDETERMINATE`.
6. Do not add `ConsolidationPlanningScore`, MatchScore, or client weights.
7. Do not raise `MAX_ADDITIONAL_LOADS` above 1.
8. Tests in `NLO_0_3E_TEST_STRATEGY.md`. The strategy is frozen. Those tests are not implemented in this publication.

## Explicitly out

Shipment activation, multi-stop planning, multi-stop execution, `RoutePlan` activation, `RouteStop` execution, driver multi-stop tasks, assignment, reservation, carrier offer, slot booking, VRP, MILP, CP-SAT, LNS, ML, genetic algorithms, 3D bin packing, unbounded subset enumeration, unbounded route permutation, a single canonical sequence used as global infeasibility, backhaul, round trip, Moscow city rules, Saint Petersburg city rules, and network-wide optimization.

`CURRENT_TRIP_FILL` above one additional load, `MULTI_PICK_ONE_DROP`, `ONE_PICK_MULTI_DROP`, and `MULTI_PICK_MULTI_DROP` stay in NLO-0.4.

## API and metrics notes for that later task

Add the pattern `SAME_ORIGIN_SAME_DESTINATION_N_MEMBER` on the existing search. Do not change the response of `SAME_ORIGIN_SAME_DESTINATION` or `CURRENT_TRIP_FILL`. Do not add an endpoint. Do not edit generated OpenAPI in discovery.

Optional later metrics, low cardinality only: a counter of budget exhaustions labelled with `POOL_LIMIT_EXCEEDED` or `SEARCH_BUDGET_EXCEEDED`. No ids. No raw set size as a label. `ROUTING_BUDGET_EXCEEDED` is not an NLO-0.3E label. The series named in older notes (`bno_consolidation_hard_reject_total`, `bno_consolidation_indeterminate_total`, `bno_consolidation_set_size_bucket`) are not emitted today and are not added here.
