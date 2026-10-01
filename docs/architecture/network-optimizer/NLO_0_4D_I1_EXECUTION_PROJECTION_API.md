# NLO-0.4D-I1 execution projection API

TMS_EXECUTION_OWNER=shipment-service
NLO_EXECUTION_OWNER=NO
NLO_IMPLEMENTATION_CHANGED=NO
MIGRATION_REQUIRED=NO

`network-optimizer-service` calls:

`POST /internal/v1/transport-executions/from-route-plan-activation`

The route requires `X-Internal-Service-Token` and `X-Internal-Service-Name: network-optimizer-service`.
`X-Tenant-ID` is the verified operating tenant. It must equal `operating_tenant_id` in the body.
The body is the existing `ProjectionCommand`. TMS does not write `network_optimizer.route_plan_activations`.

The response is returned only after the projection transaction commits and a following read sees the stored row.
Fields:

- `operating_tenant_id`
- `activation_id`
- `route_plan_id`
- `execution_id`
- `execution_revision_id`

The same operating tenant and activation with the same body returns the same execution and revision.
A changed body returns `ACTIVATION_BODY_CONFLICT` and does not create a second execution root.
`shipment.execution_plan.created` remains a TMS fact. It is not the handshake acknowledgement.
