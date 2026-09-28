# TMS-MSTOP-0.1A status

Implementation of the transport execution aggregate in `shipment-service`. This note records what 0.1A persists. It does not change the frozen architecture.

```text
EXECUTION_ROUTE_MODEL=TRANSPORT_EXECUTION
SHIPMENT_IS_EXECUTION_ROOT=NO
STOP_PARENT=TRANSPORT_EXECUTION
REVISION_REFERENCES_STOP_VIA_LINK=YES
SUCCESSOR_REPLAN_RUNTIME_IMPLEMENTED=NO
DRIVER_RUNTIME_CHANGED=NO
```

`CreateExecutionProjectionFromActivation` is `POST /internal/v1/transport-executions/projections`. The caller must present the internal service token and `X-Internal-Service-Name: network-optimizer-service`. The route is not mounted on `/v1`. Client `X-Tenant-ID` is ignored. `shipment_tenant_id` is accepted only as the owner tenant to verify with `shipments.id` and `shipments.tenant_id` together.

One activation writes one `TransportExecution`, one `ACTIVE` revision, the participant rows, the stable stops and actions, and the revision link rows. `PENDING_EXECUTION` is the only accepted activation status. The service does not update `network_optimizer.route_plan_activations` and does not emit `network.route_plan.execution_linked`.

A non-null `supersedes_route_plan_id` or `supersedes_activation_id` is refused with `409 EXECUTION_PLAN_CONFLICT`. Successor revision switching, completed-stop inheritance, and driver commands are not implemented.

`SHIPMENT_IN_AT_MOST_ONE_ACTIVE_EXECUTION` is enforced by `transport.transport_execution_active_shipments`, primary key `(shipment_tenant_id, shipment_id)`, inserted in the projection transaction. Historical `transport_execution_participants` rows are not globally unique. Removing the active slot is a later wave.

`MAX_ACTIVE_REVISIONS_PER_TRANSPORT_EXECUTION` is the partial unique index `transport_execution_revisions_one_active_idx`. Activation idempotency is `UNIQUE (operating_tenant_id, source_activation_id)` on `transport_execution_revisions`.

Migration: `000088_tms_transport_execution_foundation_v0_1a`. `000086` and `000087` are not used.
