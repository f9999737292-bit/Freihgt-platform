# Security and privacy

```text
TENANT_ISOLATION_MODEL_FROZEN=YES
DRIVER_AUTH_MODEL_FROZEN=YES
CROSS_TENANT_PRIVACY_MODEL_FROZEN=YES
EVENT_PAYLOAD_PRIVACY_FROZEN=YES
```

RoutePlan marketplace visibility does not grant shipment execution rights. A caller who can see an anonymized plan cannot arrive, complete, or read another shipper's cargo.

## Tenant isolation

```text
SHIPMENT_TENANT_ISOLATION=GATEWAY_JWT_TENANT_ON_EVERY_EXECUTION_QUERY
```

`tenant_id` comes from the gateway-established identity, as shipment commands already require. Client-supplied `X-Tenant-ID`, body tenant, or query tenant is not an authorization source. Every route, revision, stop, action, and driver-task read and write includes `tenant_id`. Cross-tenant ids return not found.

A `TransportExecution` has one `tenant_id`. Every participant shipment must already belong to that tenant. A `LOAD_OPPORTUNITY` from another tenant is not an execution identity. It becomes executable only after materialization creates a shipment and cargo in this tenant. Projection rejects a contract body that names another tenant. Raw load-opportunity ids are not returned on driver or shipper APIs.

## Driver authorization

```text
DRIVER_AUTHORIZATION=ASSIGNED_DRIVER_ONLY
```

The driver identity is the gateway driver principal already used by driver operations. Route commands apply only when `TransportExecution.driver_id` is that driver. The sequence is not authorized by picking one participant shipment. A driver cannot read another tenant's route. Operator override uses actor `OPERATOR` inside the same tenant, not the driver flag.

## Carrier and shipper

```text
CARRIER_VISIBILITY=CARRIER_COMPANY_ON_THE_SHIPMENT
SHIPPER_VISIBILITY=OWN_CARGO_ACTIONS_ONLY
```

The carrier on `TransportExecution` can read the route, stop statuses, and actions. A shipper can read stop progress only for actions whose `execution_shipment_id` is that shipper's shipment. On a vehicle that is also carrying another shipper's cargo, the shipper API does not return the other cargo id, the other shipper identity, or the other commercial terms.

The driver receives action summaries for materialized cargo on the assigned route. That includes cargo ids they must pick up or deliver. It does not include raw `LOAD_OPPORTUNITY` records, evaluation scores, capacity snapshots, or foreign owner tenant ids.

## Control Tower

```text
CONTROL_TOWER_VISIBILITY=TENANT_OPERATOR_READ_MODEL
```

Operators see execution progress for shipments in their tenant through the read model. Control Tower does not write stop completion. It may create the existing notice tasks. Sequence override is a shipment-service command authorized for the operator role, audited on the stop.

## Event payload privacy

```text
EVENT_PAYLOAD_PRIVACY=EXECUTION_FACTS_ONLY
```

Execution events include tenant, route, revision, stop, action, status, ordinal, and timestamps. They may include `execution_shipment_id` and `cargo_id` for a materialized action. They omit raw load-opportunity payloads, optimizer fingerprints beyond the revision's `evaluation_fingerprint` reference, leg geometry, prices, other tenants, and capacity snapshots. Driver APIs omit the same planning-only fields.
