# Service boundaries

No new process is created by this freeze. The bounded context is `network-optimizer`. A future `network-optimizer-service` requires a separate implementation approval. Until then these are ownership rules.

## Context

| Context | Owns | Must not own |
|---------|------|----------------|
| network-optimizer | Capacity, load opportunities, matches, consolidation candidates, route and chain **plans**, carrier offers, optimization jobs and decision snapshots | Shipments, transport-order masters, rate cards, cost ledger, tenders, slots, documents, payments |
| city-rules | Versioned jurisdiction rules and city profiles | Solver code, live traffic |
| transport-order-service | Orders, cargo master, locations, rate snapshots | Network publication, routes |
| shipment-service | Execution shipment, driver, vehicle | Marketplace projection, optimization |
| tracking-service | Positions, ETA, slot intelligence | Next-load ranking |
| contract-rate-service | Contracts and rate resolution | Expected contribution |
| freight-cost-service | Cost ledger, variance, accruals | Planning scores |
| rfx-service | RFx, freight requests, bids, awards | Greedy next-load ranking |
| billing-register-service | Freight settlement, registers | Match economics |
| payment-service | Payment obligations | Offers |
| document-service | Document registry; future EDO document | Required-document **checks** may be read; documents are not authored here |
| control-tower-read-model-service | Operator projections | Network optimization jobs |
| api-gateway | JWT, tenant header trust, RBAC | Domain decisions |
| identity-service / company-service | Users, memberships, companies | Capacity |

`city-rules` is a bounded context with a `CityRulesProvider` port. v0.1 keeps it logically separate so rules are data. Physical extraction to its own service is optional later (ADR-NET-009). That is not authorization to deploy a service now.

## Read vs write

The optimizer **reads** through tenant-scoped APIs or explicit projections:

- transport order, cargo, location
- shipment status and assigned resources
- tracking ETA, position freshness, slot windows
- rate resolution and the order's rate snapshot
- company id of the carrier the caller is allowed to represent

The optimizer **writes** only its own future schema: opportunities, capacities, plans, offers, decisions.

Creating an executable shipment remains `shipment-service`, and only for plans the execution model can represent (today: one O–D). Shared multi-order legs are not executed by silently inserting foreign cargo into someone else's shipment.

## Gateway

Public routes terminate at `api-gateway`. Clients do not call a future optimizer port directly. Trusted downstream identity is `X-Tenant-ID` and `X-User-ID` set from JWT. Client-supplied copies of those headers are stripped (`services/api-gateway/internal/http/middleware/auth.go`).

## Integration map

| Concern | Relationship |
|---------|----------------|
| Shipment | Consumed as execution fact and as input to predictive capacity. Not scanned cross-tenant. |
| RFx | Competitive procedures stay in RFx. Optimizer may request a freight request; it does not award. |
| Freight cost | Planning estimate is labeled and is not a `CostEntry`. |
| Driver / vehicle | Private fleet binds them on the plan. Marketplace may match equipment before either id exists. |
| Slot | Feasibility queries availability. Soft hold and hard book stay slot-owner operations. |
| Control Tower | Future network KPIs are projections of optimizer facts. CT stays the operator read model. |
| EDO / documents | A constraint may require a document type. The optimizer does not sign or send EDO. |
| Settlement | Happens after execution and billing. An accepted offer does not create a settlement. |

## Evolution of movement

Preferred later change, not in this freeze:

1. Add nullable references from a shipment to a `route_leg_id` when execution of a planned leg is authorized.
2. Leave existing single O–D shipments valid with a null leg.
3. Do not replace `transport.shipments` in place.
4. Do not add stops by overloading origin/destination.

Until that change is authorized, multi-stop and multi-shipper plans cannot become `ACTIVE` execution.
