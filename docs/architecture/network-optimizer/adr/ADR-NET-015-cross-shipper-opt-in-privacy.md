# ADR-NET-015: Cross-shipper consolidation opt-in and privacy

Status: Proposed. NLO-0.3A freeze. Implementation is not authorized.

## Decision

Cross-shipper consolidation reads only an explicitly published `LoadOpportunity`. It does not select raw shipments, orders, or cargoes across tenants.

Two owner-controlled booleans, both defaulting to false, are the canonical consent fields:

- `consolidation_allowed` — this load may be planned with other loads of the same owner tenant.
- `cross_shipper_consolidation_allowed` — this load may be planned with a load owned by another tenant.

Absence of either flag is refusal. `can_co_load` is a documentation name only. It is not a column on `transport.cargoes` or `network_optimizer.load_opportunities`.

`NETWORK_OPTIMIZATION_ONLY` stays an internal publication scope. `ListMarketplaceLoads` already excludes it. A future consolidation pool may include that scope inside the optimizer trust zone. The human marketplace query must not.

## Consequences

- Shipper A does not receive shipper B's rate, contract, source id, customer, tender, tenant id, or detailed commodity.
- The carrier pre-offer view is the safe member view. The accepted operational view is a later stage and still excludes unpublished commercial fields.
- Internal audit may retain owner tenant ids. Human-facing audit may not.
