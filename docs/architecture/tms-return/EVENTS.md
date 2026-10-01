# TMS-RETURN-0.1 events

Disposition events use the existing shipment outbox and are published with the execution event classifier.
They have their own `disposition_sequence`. They do not reset `transport_executions.event_seq`.
Route changes still emit `shipment.execution_plan.superseded` and `shipment.execution_plan.created` from the 0.1F writer in the same transaction.

| Event | When |
| --- | --- |
| shipment.delivery.partially_rejected | accepted and rejected quantities are both positive |
| shipment.delivery.rejected | accepted quantity is zero |
| shipment.cargo.disposition_pending | case is waiting, including hold |
| shipment.cargo.return_authorized | return successor committed |
| shipment.cargo.redirect_authorized | redirect successor committed |
| shipment.cargo.return_completed | return action evidence resolved the case |
| shipment.cargo.redirect_completed | redirect action evidence resolved the case |

Payload fields are operating tenant, execution, source stop, shipment, cargo, case, reason, quantities, UOM, status, disposition sequence, and target location id when authorized.
Payloads omit shipment tenant, price, rate, optimizer fingerprint, and coordinates.

Billing and EDO are not consumers in this wave. The events are the integration contract.
OPENAPI_CHANGE=NO because the write API is internal service auth only.
