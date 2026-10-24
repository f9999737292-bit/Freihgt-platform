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

`TestBintransE2ETransportFlow` starts the rfx, contract-rate, transport-order, shipment, document, and control-tower server binaries against one empty PostgreSQL database and one in-process Kafka cluster. CI supplies the database. A local run without `TEST_DATABASE_URL` starts an embedded PostgreSQL instance instead of skipping. Each business step is an HTTP call to those processes. Setup inserts only the tenant, companies, users, memberships, roles, locations, drivers, and vehicles. Driver rows carry the user identity used by later driver events.

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

## Delivery and POD

The same `shipment_id` continues after assignment. TMS relationship activation stays disabled. The carrier dispatcher books `PICKUP_SLOT_BOOKED` through `PATCH /v1/shipments/{id}/status`. The assigned driver then posts `ARRIVED_AT_PICKUP`, `PICKUP_COMPLETED`, `DEPARTED_PICKUP`, `ARRIVED_AT_DELIVERY`, `UNLOADING_STARTED`, and `DELIVERY_COMPLETED` to `POST /v1/driver/me/shipments/{id}/events`. Those calls use `X-Tenant-ID` and the driver user's `X-User-ID`. Another driver in the same tenant and a driver in another tenant receive `404` and write no status history.

Each accepted status change inserts `transport.shipment_status_history` and `transport.shipment_event_outbox` in the shipment transaction. The shipment process publishes that outbox to topic `shipment.status.v1` on the Kafka cluster. The control-tower process consumes the same topic and serves `GET /internal/v1/control-tower/shipments/{shipmentId}/status`. The projection row is one per tenant and shipment, and its `currentStatus` is `DELIVERED`. A foreign tenant read is `404`. Repeating `DELIVERY_COMPLETED` with the same idempotency key returns `replayed=true` and does not add a history row, an outbox row, or a projection row.

POD uses the assigned driver and the awarded carrier on that shipment: `POST /internal/v1/pod-uploads`, `PUT /internal/v1/pod-uploads/{uploadId}/content`, `POST /internal/v1/pod-uploads/{uploadId}/complete`, then `GET /v1/documents/{id}`. The document tenant matches, `related_entity_type` is `SHIPMENT`, and `related_entity_id` is the original shipment. A repeated complete returns the same document and file. An unassigned driver cannot create an intent or complete the upload. A foreign tenant cannot create an intent or read the document. A denied POD call leaves the shipment at `DELIVERED` and inserts no document. The test does not move the shipment to `DOCUMENTS_COMPLETED`.

The Kafka cluster is `kfake`, listening on TCP and speaking the Kafka protocol. It is started inside the test process. It is not a direct insert into the control-tower repository and it is not a Redpanda container.
