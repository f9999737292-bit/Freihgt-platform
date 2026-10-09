# Customer Portals 0.1B — forwarder model

Forwarder is in scope as a business model. The app directory is not created in this stage.

```text
FORWARDER_PRODUCT_SURFACE_REQUIRED=YES
SEPARATE_WEB_FORWARDER_APP=YES
FORWARDER_IS_TWO_SIDED_MARKET_ACTOR=YES
FORWARDER_DUAL_SIDE_BACKEND_READY=NO
ONE_EXECUTION_CAN_HAVE_MULTIPLE_COMMERCIAL_RELATIONSHIPS=YES
CUSTOMER_SETTLEMENT_SEPARATE_FROM_SUPPLY_SETTLEMENT=YES
CUSTOMER_EDO_CHAIN_SEPARATE_FROM_SUPPLY_EDO_CHAIN=YES
EDO_PARTY_CONTEXT_MANDATORY=YES
```

## Two sides

| Side | Parties | What the portal may later show | What it must not do |
| --- | --- | --- | --- |
| Customer | Shipper or customer, and forwarder | Orders the forwarder sells, receivables, customer EDO | Treat the shipper as a user of `web-forwarder` |
| Supply | Forwarder and carrier | Procurement, carrier orders, payables, supply EDO | Treat the forwarder as `CARRIER_ADMIN` |

One physical execution may be linked from both sides. The link is a fact Agent C owns. The portal may display it. The portal may not define it, and may not store a second copy as the source of truth.

## What the repository does today

`FORWARDER_MANAGER` is seeded in `infrastructure/migrations/000013_seed_forwarder_manager_role.up.sql`.

`DeriveActorKind` in `services/api-gateway/internal/companycontext/actor.go` maps that role, and company types `FORWARDER` and `LSP`, to `BUYER`. The only other actor kind is `CARRIER`. There is no forwarder actor and no pair of commercial sides.

Buyer-like gateway policies therefore admit `FORWARDER_MANAGER` next to shipper and procurement codes on transport orders, RFx manage, and billing mutate. That is a single-sided buyer grant. It is not evidence that customer-side AR and supply-side AP both exist.

No customer portal route renders a forwarder workspace. `apps/web-procurement` is a buyer workspace with some carrier pages. It is not `apps/web-forwarder`.

## Party context

Every document, signature, settlement, and payment the forwarder UI shows later must carry an explicit party context: customer chain or supply chain. A missing context is a client bug, not a default to "the forwarder's company."

The two chains may reference the same execution id. They must not reference the same settlement id or the same EDO package id.

## Who decides the business rules

| Question | Decides | Portal does |
| --- | --- | --- |
| When an execution is the same physical job for both contracts | Agent C | Shows the link the API returns |
| When a signature or package is valid, and which party it binds | Agent B | Shows party context and status |
| What is a receivable, a payable, or an accounting document | Agent G | Shows the two chains in separate navigation groups |
| Which role may open `web-forwarder` | This freeze: `FORWARDER_MANAGER` only, until identity adds finer forwarder codes | Hides the other portals from that session |

This document does not specify invoice calculation, signature law, or execution status transitions.
