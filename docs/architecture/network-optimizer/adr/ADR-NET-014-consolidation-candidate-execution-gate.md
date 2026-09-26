# ADR-NET-014: ConsolidationCandidate and the execution gate

Status: Accepted. NLO-0.3A is FROZEN_ACCEPTED. Implementation is not authorized.

## Decision

`ConsolidationCandidate` is a planning record. Its statuses are `HARD_REJECT`, `INDETERMINATE`, `FEASIBLE`, `PROPOSED`, and `INVALIDATED`. `ASSIGNED` and `ACTIVE` are not statuses of this record.

`FEASIBLE` means the proposed hard checks that could be evaluated did not fail. A required unknown fact is `INDETERMINATE`, not `FEASIBLE`. `EXECUTABLE` is a separate claim and is false for any plan that needs multiple pickups, multiple drops, or more than one shipper cargo on one shared movement, until NLO-0.4 changes `transport.shipments`.

The public search references an owned `capacity_id` or `shipment_id`. It does not accept a caller-authored trip context as authority.

The first waves store route assumptions on the candidate: ordered planning stops, road kilometres, road seconds, provider name, and window results. They do not create `RoutePlan` / `RouteLeg` / `RouteStop` tables. ADR-NET-005 remains the execution gate. NLO-0.4 owns those execution tables.

## Consequences

- A same-origin/same-destination plan is still planning-only. `transport.shipments.transport_order_id` is one nullable foreign key, not a set of orders.
- Accepting a plan does not create a shipment, book a slot, or assign a driver.
- A changed input version invalidates the candidate. The record is not edited in place into a new decision.
