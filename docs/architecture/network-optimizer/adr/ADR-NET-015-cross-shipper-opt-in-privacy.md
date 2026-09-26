# ADR-NET-015: Cross-shipper consolidation opt-in and privacy

Status: Proposed. Architecture-freeze candidate, pending controller acceptance. Implementation is not authorized.

## Decision

Cross-shipper consolidation reads only an explicitly published `LoadOpportunity`. It does not select raw shipments, orders, or cargoes across tenants.

Two owner-controlled booleans, both defaulting to false, are persisted publication facts on `LoadOpportunity`. A searching carrier cannot set them in the search request. Absence is false.

Canonical rule, stated the same way in the privacy document, the roadmap, and the API design:

- Same-owner pair: every participating load has `consolidation_allowed=true`. The cross-shipper flag does not grant or deny that pair.
- Cross-shipper pair: every participating load has `cross_shipper_consolidation_allowed=true`. `consolidation_allowed` is not required and does not substitute.

`can_co_load` is a documentation name only. It is not a column on `transport.cargoes` or `network_optimizer.load_opportunities`.

`NETWORK_OPTIMIZATION_ONLY` stays an internal publication scope. `ListMarketplaceLoads` already excludes it. A future consolidation pool may include that scope inside the optimizer trust zone. The human marketplace query must not.

## Consequences

- Shipper A does not receive shipper B's rate, contract, source id, customer, tender, tenant id, or detailed commodity.
- The carrier pre-offer view is the safe member view. The accepted operational view is a later stage and still excludes unpublished commercial fields.
- Internal audit may retain owner tenant ids. Human-facing audit may not.
