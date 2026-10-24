# BINTRANS E2E-001 — Order / RFx Contract 0.1A

STAGE=BINTRANS-E2E-TRANSPORT-FLOW-0.1A-FOUNDATION
BASE=f42964d65ec1004604bdfe3f7ee56b46c9f8fe6c

This stage does not merge the freight-request model and the RFx award model. It records which object is canonical for the first executable transport, and which identifier is created by the conversion that already exists.

## Two chains on current main

Chain A, order then freight request:

1. A buyer creates a priced transport order on `POST /v1/transport-orders`.
2. `FreightRequestService.CreateFromTransportOrder` requires that order in the same tenant and in `READY_FOR_SOURCING` or `SOURCING_IN_PROGRESS`.
3. A bid references `freight_request_id`.

`FreightRequest` stores `transport_order_id`. It has no `rfx_event_id`. `RfxEvent` has no `freight_request_id`. A bid is not an RFx response.

Chain B, RFx then a new order:

1. A buyer creates an `RfxEvent` owned by `owner_company_id` in one tenant.
2. A carrier response is awarded.
3. `ConvertAwardToTransportOrders` calls internal `POST /internal/v1/transport-orders/from-award-scope`.
4. That call creates a new priced transport order. The request body has no predecessor `transport_order_id`.
5. `rfx.rfx_award_transport_orders` stores the new order, the event, the award, the response, and the optional lot.

Shipment execution reads the transport order that exists after this conversion. It does not read a freight-request bid as that order.

## Decision

The first E2E uses chain B.

`RfxEvent` is the procurement root. The transport order inserted by award conversion is the execution root. After conversion, that new `transport_order_id` is the canonical execution identifier. The product contract does not carry an original order id through award, so this stage does not invent one.

Chain A stays available for clients that already create a freight request from an order. It is not the award-conversion contract and is not joined to chain B here.

## Identity

| Fact | Rule |
| --- | --- |
| Buyer company | `rfx_events.owner_company_id`, copied to `rfx_award_transport_orders.buyer_company_id` and to the new order shipper and consignee |
| Carrier company | `rfx_awards.carrier_company_id`, copied to the link and sent as `carrier_company_id` |
| Tenant | The event is loaded with `actor.TenantID`. The new order and the link use `event.TenantID` |
| Cross-tenant link | A foreign actor does not load the event. The link join requires `transport_orders.tenant_id = rfx_award_transport_orders.tenant_id` |
| Award retry | An existing link for the same event lot, or for the event when there is no lot, is skipped. The transport-order idempotency key is `award-conv:{tenant}:{event}:{lot-or-event}` |
| Duplicate link | Unique indexes `uq_rfx_award_transport_order_scope_lot` and `uq_rfx_award_transport_order_scope_event` |

## Compatibility

Public order create, freight-request create, and bid create keep their current request shapes. Award conversion keeps its internal route and its internal service token. This stage does not add a migration, does not add a column that would force an original order id, and does not change the award-conversion write path.

Order-execution runtime continues to address a transport order and a shipment. The order it should address for this E2E is the order id returned by conversion.

## Acceptance specification

The selected scenario is one tenant, one awarded RFx event, and one execution order created by conversion.

1. Conversion of an awarded event creates a transport order and one `rfx_award_transport_orders` row in that event tenant.
2. The conversion request does not send a predecessor `transport_order_id`.
3. Repeating the conversion uses the same idempotency key and does not insert a second link for the same event scope.
4. An actor whose tenant does not own the event cannot convert it.
5. The shipment, if started, uses the converted `transport_order_id`.
6. A freight request and its bid are not required, and are not written, by this scenario.

Assertions 2 and 3 are locked by `TestAwardConversionCreatesOrderWithoutPredecessorID`. Assertions 1, 4, and 5 already have tenant-scoped queries and integration coverage behind the `integration` build tag. They are not reimplemented here.

No failing end-to-end test is added. A test that required chain A and chain B to share one order would assert a join this contract rejects. A test of the full financial close would need fixtures this stage is not authorized to create.

## Out of scope

TMS activation, the activation enforcement gate, PostgreSQL grants, RFx schema changes, and a rewrite of `ConvertAwardToTransportOrders`.
