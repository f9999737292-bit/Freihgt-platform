# NLO-0.3 privacy and tenancy

Baseline: `origin/main` `6d47cc92`. Reconciles [MARKETPLACE_DATA_VISIBILITY_MATRIX.md](MARKETPLACE_DATA_VISIBILITY_MATRIX.md) and ADR-NET-002.

## Trust boundary

External callers authenticate at the API gateway. Downstream services trust `X-Tenant-ID` and `X-User-ID` only when the gateway set them. Browser-supplied identity headers are not an authorization source.

The optimizer does not `SELECT` shipments, orders, or cargoes across tenants. The allowed path is:

owner tenant → explicit `LoadOpportunity` publication → optimizer projection inside the trust zone → `EvaluateGroupage`.

Current-trip facts are read with the capacity owner's tenant through internal service token endpoints, the same pattern as `GET /internal/v1/shipments/{id}/prediction-input` and `POST /internal/v1/tracking/eta/lookup`.

## Opt-in

| Field | Default | Effect |
| --- | --- | --- |
| `consolidation_allowed` | false | Same-owner loads may be planned together |
| `cross_shipper_consolidation_allowed` | false | A load may be planned with another tenant's published load |

No flag means no consolidation of that kind. `NETWORK_OPTIMIZATION_ONLY` is not consent. It is a visibility scope that humans do not browse. `share_commodity_with_co_load` is not required for the first waves because the other shipper view omits commodity detail anyway.

## Views

| Viewer | May see | May not see |
| --- | --- | --- |
| BNO internal | owner tenant ids, source ids, versions, full compatibility trace | nothing withheld inside the trust zone; logs still omit secrets |
| capacity-owning carrier, pre-offer | safe member summaries, residual facts, pattern, conditions | unpublished rate, contract, source id, customer, other tenant id |
| carrier, accepted plan | later stage | still no unpublished commercial fields |
| Shipper A | own load and the fact that another load is compatible at a coarse level | B's rate, contract, `transport_order_id`, customer, tender, tenant id, detailed commodity |
| Shipper B | symmetric | A's private fields |
| driver | not a consolidation audience in NLO-0.3 | other shipper commercial data |
| platform admin | support view still scoped by an explicit admin path | not a raw cross-tenant scan |

Anonymized geography rules from BNO-0.1C0 remain. Exact coordinates and exact road kilometres stay off anonymized human views.

## Internal pool

A consolidation candidate query may include `NETWORK_OPTIMIZATION_ONLY` and same-owner `PRIVATE` loads when the owner opted in. That query is not `ListMarketplaceLoads` and is not `GET` marketplace load opportunities. Human marketplace predicates stay as they are in `repository/postgres.go`.

## Audit and metrics

Internal audit keeps versions, provenance, rule fingerprints, provider name, and distances. Human audit drops other-tenant identifiers and commercial amounts.

Metrics use bounded labels only: search count, sets evaluated, hard reject reason code from a fixed set, indeterminate count, set-size bucket, duration. Never `tenant_id`, `shipment_id`, `load_id`, `capacity_id`, `user_id`, or `search_id`.
