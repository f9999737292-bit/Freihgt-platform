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

The frozen fail-closed codes, not implemented in this discovery, are `POOL_LIMIT_EXCEEDED`, `SEARCH_BUDGET_EXCEEDED`, and `ROUTING_BUDGET_EXCEEDED`. Thresholds are `UNSET`. Until they are accepted, implementation of a larger set size must not ship an unbounded search.

Routing amplification: four provider calls per fresh current-trip candidate, including hard rejects. A larger set under the one-sequence rule still multiplies calls by the chain length. The routing budget exists so a client cannot demand that work.

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
