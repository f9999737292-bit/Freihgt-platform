# NLO-0.3B pairwise consolidation implementation

Status: IMPLEMENTED / CLOSED. Merged to main in PR #174. Feature head `8a99e71a8c06042e4bc46816dc41ce8c3142a1a4`. Merge SHA `8208c662f1c80475338a97dbc710268fa0fc5a07`. CI run `36267207012` succeeded. Migration `000081_nlo_pairwise_consolidation_v0_3b`. NLO-0.3C is IMPLEMENTED_CLOSED on main `cad3c67b93fd72329050db624ae048495ef0236f` (PR #176, migration `000082`). NLO-0.3D is IMPLEMENTED_CLOSED on main `ef3722db92fd1c115f6539cc491df3eabd57affa` (PR #178, migration `000083`). NLO-0.3E implementation is accepted, so NLO-0.3 is complete. `CURRENT_TRIP_FILL` is planning only and does not change this pairwise contract. Pairwise runs still require capacity id and version.

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

`CROSS_SHIPPER_OPT_IN_IMPLEMENTED=YES`. `CROSS_SHIPPER_FULL_COMPATIBILITY_PROOF=NO`. `CROSS_SHIPPER_FAIL_CLOSED=YES`. `CROSS_SHIPPER_FEASIBLE_ALLOWED=NO`.

A cross-shipper pair is not evaluated by loading one tenant's `Catalog.Evaluation` and applying it to another tenant's cargo. NLO-0.3B has no ownership-aware multi-party reference context. The pair is checked only with an empty physical context. A physical hard reject, such as payload, volume, equipment capability, or a temperature-range conflict, is kept. Every other cross-shipper result is `INDETERMINATE` with `MULTI_PARTY_REFERENCE_CONTEXT_UNAVAILABLE`. It is never `FEASIBLE`. Capacity usage and the compatibility fingerprint come from that physical result plus the stable marker `MULTI_PARTY_REFERENCE_CONTEXT_UNAVAILABLE`. This is the accepted safe outcome for 0.3B, not a failed implementation. Full cross-shipper proof waits for a later compatibility-context evolution.

A same-owner pair has two policy owners: the capacity owner and the load owner. A `FEASIBLE` result requires the system reference facts plus both parties' applicable tenant restrictions. Tenant hard denies from either party remain rejects, and neither party can loosen a regulatory or platform hard deny. The current B2 context does not label every alias or equivalence with an owner. Cargo aliases, classes, and equivalences are applied only when they belong to the load owner or are shared system facts. Equipment aliases and classes are applied only when they belong to the capacity owner or are shared system facts. One tenant's cargo catalog is not used to reinterpret the other tenant's cargo, and the load owner's equipment catalog is not used to reinterpret the carrier asset. When those ownership boundaries cannot be proven, the pair is `INDETERMINATE` with `MULTI_PARTY_REFERENCE_CONTEXT_UNAVAILABLE` and is not `FEASIBLE`. Load-owner context alone is not a complete party-policy evaluation.

## Persistence, privacy, and execution

One transaction stores the search run, every evaluated candidate, and both member pins. Member ordinals are 1 and 2. `load_owner_tenant_id` is an internal audit column and is not a public response field. Capacity id and version, load versions, and the compatibility fingerprint are pinned. A later load change does not rewrite the old row.

Search reads `ListPublicConsolidationPool`, not `ListMarketplaceLoads`. The marketplace list is unchanged. The pool keeps the carrier visibility contract: `PUBLISHED`, another tenant, and `MARKETPLACE`, `ANONYMIZED_MARKETPLACE`, or `INVITED_CARRIERS` for the actor company. `PRIVATE` and `NETWORK_OPTIMIZATION_ONLY` stay out. A load participates only when `consolidation_allowed` or `cross_shipper_consolidation_allowed` is true. Missing canonical location ids stay in the pool so the service can report `ORIGIN_IDENTITY_UNPROVEN` or `DESTINATION_IDENTITY_UNPROVEN`. Postgres orders the pool by pickup location id, delivery location id, then id. Pair permission is still applied later: same-owner needs both general flags, and cross-shipper needs both cross-shipper flags. Anonymized members use the existing coarse marketplace view, so exact location ids, coordinates, owner tenant ids, and source ids are not copied into the response. The public compatibility trace omits rule-set tenant ids, catalog tenant ids, and source references. Rates are not summed. `POST /v1/network/consolidation/search` rejects unknown JSON fields and a second JSON document. Candidate rows reference `(search_run_id, tenant_id, capacity_id, pattern)` on the search run.

Every candidate returns `execution_supported=false` and `placement_check=NOT_EVALUATED`, including `FEASIBLE`. No shipment, assignment, reservation, or route insertion is created.

## API

`POST /v1/network/consolidation/search` on network-optimizer-service. The gateway route is `POST /api/v1/network/consolidation/search` under `PolicySearchConsolidation` for `CARRIER_ADMIN` and `CARRIER_DISPATCHER`. Shipper roles are denied. `CURRENT_TRIP_FILL` is the NLO-0.3D planning pattern. This pairwise document does not implement it. Unknown patterns still return `PATTERN_NOT_IMPLEMENTED`.

## Non-goals

Current-trip fill, onboard evidence, residual capacity, multi-pick, multi-drop, route insertion, scoring, ranking, offers, solvers, and 3D placement are out of this wave. Event publication for consolidation search is deferred. Persistence is the audit record.
