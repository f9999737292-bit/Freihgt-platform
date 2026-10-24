# BINTRANS E2E Transport Flow 0.1B — Identifiers and Idempotency

One verified tenant holds the shipper company and the awarded carrier company. Cross-tenant TMS activation stays off. Freight Request is not part of this chain.

## Chain

| Step | Interface | Auth context | Tenant rule |
| --- | --- | --- | --- |
| Create, publish, respond, award | `RfxService` methods on the RFx repository | Buyer or carrier `ActorContext` (`tenant_id`, `user_id`) with company membership and role | Actor tenant must own the event. A foreign tenant receives `NOT_FOUND` |
| Award conversion | `POST /internal/v1/transport-orders/from-award-scope` | `X-Internal-Service-Token`, `X-Tenant-ID`, `Idempotency-Key` | Header tenant is the event tenant. Body has no `tenant_id` |
| Rate resolution | `POST /internal/v1/rates/resolve` | Internal token plus `X-Tenant-ID`, `X-User-ID`, `X-Company-ID`, `X-Actor-Kind: BUYER` | Contract-rate reloads membership and accepts the call only when the user is a buyer member of the owner company |
| Award pricing | `GET /internal/v1/pricing/award-scope/{eventId}` | Internal token and `X-Tenant-ID` | Pricing row is read in that tenant |
| Execute | `POST /v1/order-execution/transport-orders/{id}/execute?carrier_company_id=` | `X-Tenant-ID`, `X-User-ID` | Link and order are loaded in the header tenant. Another carrier in the same tenant is `403`. A foreign tenant is `404` |
| Assign vehicle / driver | `POST /v1/shipments/{id}/assign-vehicle` and `assign-driver` | `X-Tenant-ID`, `X-User-ID` | Driver or vehicle must belong to the shipment tenant and the awarded carrier |
| Public order create | `POST /v1/transport-orders` | `X-Tenant-ID`, `X-User-ID`, `X-Company-ID`, `X-Actor-Kind`, `Idempotency-Key` | A body `tenant_id` that differs from `X-Tenant-ID` is `403` and writes no order |

`TestBintransE2ETransportFlow` starts the rfx, contract-rate, transport-order, and shipment server binaries against one empty PostgreSQL database. CI supplies that database. A local run without `TEST_DATABASE_URL` starts an embedded PostgreSQL instance instead of skipping. Each step above is an HTTP call to those processes. Setup inserts only the tenant, companies, users, memberships, roles, locations, drivers, and vehicles.

## Transactions

Award conversion calls transport-order over HTTP before the RFx link transaction. The order and rate snapshot commit in transport-order. The RFx link commits after that call returns. Execute then inserts the shipment and updates the order status in one shipment transaction. Assignment updates the shipment row and status history in the shipment transaction. There is no cross-service transaction.

## Identifiers

| Name | Where it is born | Later use |
| --- | --- | --- |
| `rfx_event_id` | RFx event create | Award, pricing scope, order external reference, award link |
| `rfx_lot_id` | Lot create | Offer line, pricing query, idempotency key, award link |
| `rfx_response_id` | Carrier response | Award and award link |
| `rfx_award_id` | Award | Award link |
| `transport_order_id` | Award-scope create | Award link, shipment `transport_order_id` |
| `shipment_id` | Execute | Assign driver and assign vehicle |
| `carrier_company_id` | Awarded participant | Order pricing context, award link, shipment |
| Idempotency key `award-conv:{tenant}:{event}:{lot}` | Derived by `AwardConversionIdempotencyKey` | `transport.transport_order_create_idempotency` |

`source_system` on the created order is `rfx_award`. The original shipper order id is not reused. This iteration creates one new execution order.

## Idempotency

A repeated conversion finds the existing award link and does not call create again. The transport-order key still returns the same order if the create is retried. A repeated execute finds the shipment for that tenant and transport order and returns it with `created=false`. Assignment has no separate idempotency key. A second award or execute does not insert a second order or shipment.

## Actor kind

Award-scope create forwards `X-Actor-Kind: BUYER`. Contract-rate rejects `USER` before resolving a rate. The buyer kind is checked again against the converting user's membership of the owner company. A carrier membership does not satisfy that check.
