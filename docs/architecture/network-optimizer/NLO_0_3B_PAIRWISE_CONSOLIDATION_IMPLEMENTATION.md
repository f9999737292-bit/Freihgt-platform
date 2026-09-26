# NLO-0.3B pairwise consolidation implementation

Status: IMPLEMENTED_IN_BRANCH / UNDER_REVIEW. This wave is not closed and NLO-0.3 is not complete.

The question this slice answers is whether two published loads can travel together on one owned available capacity when they share a canonical origin and a canonical destination. The result is planning feasibility. It does not execute a shipment, assign a vehicle, reserve capacity, score a pair, or insert a route stop.

## Opt-in

`consolidation_allowed` and `cross_shipper_consolidation_allowed` are owner-controlled columns on `network_optimizer.load_opportunities`. Both are `boolean NOT NULL DEFAULT false`. Create omits them as false. Patch uses nullable booleans, so false replaces true, and the load version increments under the existing optimistic lock, audit, and outbox path. There is no separate opt-in endpoint and no search-request override.

A same-owner pair is eligible only when both loads have `consolidation_allowed`. A cross-shipper pair is eligible only when both loads have `cross_shipper_consolidation_allowed`. The flags are independent. Cross-shipper eligibility does not also require `consolidation_allowed`.

## Canonical identity and windows

A pair is `SAME_ORIGIN_SAME_DESTINATION` only when both pickup `location_id` values are present and equal and both delivery `location_id` values are present and equal. Label, city, region, country, coarse geography, rounded coordinates, and Haversine are not equality. A missing origin increments `ORIGIN_IDENTITY_UNPROVEN`. A missing destination increments `DESTINATION_IDENTITY_UNPROVEN`. Those loads are excluded from pair generation. Their ids are not returned as rejected candidate bodies.

Known pickup windows must have a positive overlap (`end > start`). The same rule applies to delivery windows. A boundary touch is `PICKUP_WINDOWS_DISJOINT` or `DELIVERY_WINDOWS_DISJOINT` and is `HARD_REJECT`. An unknown required window is `INDETERMINATE` (`PICKUP_WINDOW_UNKNOWN` or `DELIVERY_WINDOW_UNKNOWN`). The service does not invent timestamps or reuse the capacity availability window as the load window.

## Pair generation

Eligible published loads are grouped by pickup location id and delivery location id. Members of a group are sorted by load UUID ascending. The generator emits unordered pairs `i < j` inside that group only. Set size is exactly 2. Reverse duplicates, triples, and subset enumeration are not produced. `candidate_limit` is applied after evaluation, ordering feasible candidates before indeterminate ones. Zero returns no candidate rows and leaves counts and persisted rows unchanged. Hard rejects are persisted and omitted from the default response.

## Compatibility

`compat.EvaluateGroupageItems` is the canonical B2 engine. Each cargo is checked with its own `AccessNeed`. Allowed access lists are not unioned. Payload, volume, pallet positions, linear metres, height, temperature, ADR, food grade, odor, contamination, and cargo-cargo rules stay in that engine. `EvaluateGroupage` remains backward compatible and still shares one access need.

Known exceedance maps to `HARD_REJECT`. Unknown required dimensions stay `INDETERMINATE`. Unknown pallet count and unknown linear metres are not coerced to zero. Pallet conversion uses only explicit positive equivalences. Height is a scalar comparison. Temperature uses the existing common-interval rule. More than one independently controllable zone stays `MULTI_ZONE_ALLOCATION_REQUIRED` unless the B2 engine already proves a result. ADR uses sourced B2 rules only. `REQUIRE_SEPARATION` and `REQUIRE_CONDITION` stay unsatisfied and therefore `INDETERMINATE`.

## Multi-party reference context

Same-owner and cross-shipper evaluation calls `Catalog.Evaluation` once for the capacity owner and once for each distinct load owner. A hard deny from any of those contexts is `HARD_REJECT`. Any indeterminate context blocks `FEASIBLE`. One tenant's catalog is not applied as another tenant's overlay.

When no catalog is configured, a cross-shipper pair that would otherwise be feasible is `INDETERMINATE` with `MULTI_PARTY_REFERENCE_CONTEXT_UNAVAILABLE`. A physical hard reject is kept. That pair is never marked `FEASIBLE` from a single empty context.

## Persistence, privacy, and execution

One transaction stores the search run, every evaluated candidate, and both member pins. Member ordinals are 1 and 2. `load_owner_tenant_id` is an internal audit column and is not a public response field. Capacity id and version, load versions, and the compatibility fingerprint are pinned. A later load change does not rewrite the old row.

The public pool is `ListMarketplaceLoads` for the carrier actor. `PRIVATE` and `NETWORK_OPTIMIZATION_ONLY` stay out of marketplace list and out of this search. Anonymized members use the existing coarse marketplace view, so exact location ids, coordinates, owner tenant ids, and source ids are not copied into the response. The public compatibility trace omits rule-set tenant ids, catalog tenant ids, and source references. Rates are not summed.

Every candidate returns `execution_supported=false` and `placement_check=NOT_EVALUATED`, including `FEASIBLE`. No shipment, assignment, reservation, or route insertion is created.

## API

`POST /v1/network/consolidation/search` on network-optimizer-service. The gateway route is `POST /api/v1/network/consolidation/search` under `PolicySearchConsolidation` for `CARRIER_ADMIN` and `CARRIER_DISPATCHER`. Shipper roles are denied. `CURRENT_TRIP_FILL` is recognized and returns `PATTERN_NOT_IMPLEMENTED`.

## Non-goals

Current-trip fill, onboard evidence, residual capacity, multi-pick, multi-drop, route insertion, scoring, ranking, offers, solvers, and 3D placement are out of this wave. Event publication for consolidation search is deferred. Persistence is the audit record.
