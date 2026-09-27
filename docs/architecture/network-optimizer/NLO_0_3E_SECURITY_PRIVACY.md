# NLO-0.3E security and privacy

Privacy takes precedence over optimization coverage.

## Cross-shipper

NLO-0.3B fails closed when a multi-party reference context cannot be proven. `assessCrossShipper` returns `INDETERMINATE` / `MULTI_PARTY_REFERENCE_CONTEXT_UNAVAILABLE` and never `FEASIBLE`. Same-owner evaluation loads the capacity-owner catalog and the load-owner catalog separately. It does not apply one tenant's catalog overlay to the other tenant's cargo.

NLO-0.3E does not remove that rule for two shippers or for three or more shippers.

```text
MULTI_PARTY_REFERENCE_CONTEXT_MODEL=FAIL_CLOSED_UNPROVEN
CROSS_SHIPPER_N_WAY_FEASIBLE_ALLOWED=NO
PRIVATE_TENANT_OVERLAY_LEAK=NO
```

A set that mixes load owners stays `INDETERMINATE` until a future design proves compatibility without reading one tenant's private catalog onto another tenant's cargo. That design is not this freeze.

## Resource exhaustion

`ListPublicConsolidationPool` has no SQL limit. Pair generation is `N*(N-1)/2` inside one origin-destination group. Every assessed pair is written. `candidate_limit=0` still evaluates and persists the full set. A large `candidate_limit` does not reduce that work.

NLO-0.3E fail-closed codes are `POOL_LIMIT_EXCEEDED` and `SEARCH_BUDGET_EXCEEDED`. Same-origin expansion makes no routing call, so `ROUTING_BUDGET_EXCEEDED` is not an NLO-0.3E outcome. NLO-0.3D still performs its existing four `roadLeg` calls for one additional load. That protection stays on NLO-0.3D. A future NLO-0.4 multi-stop planner owns any later routing budget.

```text
NLO_0_3E_SCOPE=SAME_ORIGIN_SAME_DESTINATION_N_MEMBER_SETS_ONLY
ROUTE_SEQUENCES_EXPLORED_BY_0_3E=0
NLO03E_ROUTING_CALLS_PER_SET=0
NLO03E_MAX_ROUTING_CALLS=0
MULTI_STOP_PLANNING_IN_0_3E=NO
```

Audit payload: the pairwise 500-load body was about 277 MB in memory. A production search must not persist unbounded JSON because a client asked for it.

Repeated identical searches insert a new run each time. No cache is added.

## Execution boundary

```text
EXECUTION_SUPPORTED=NO
SHIPMENT_MUTATION=NO
ORDER_MUTATION=NO
ASSIGNMENT=NO
RESERVATION=NO
CARRIER_OFFER=NO
SLOT_BOOKING=NO
DRIVER_TASK=NO
ROUTE_PLAN_ACTIVATION=NO
MULTI_STOP_PLANNING_IN_0_3E=NO
```

`PLAN_ONLY` same-origin sets are not a stop list and are not executable multi-stop transport. One explored stop order, in any later wave, is not proof that the set is globally infeasible.

## Unknown

Unknown weight, volume, windows, position freshness, catalog scope, or routing proof stays `INDETERMINATE`. It is not coerced to zero or to feasible.

## Metrics

Existing labels are bounded: outcome status is three values, duration buckets are fixed. A later budget counter may use only the three reason codes above. Load ids, tenant ids, and raw set sizes must not be labels.
