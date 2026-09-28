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

`tenant_id` comes from the gateway-established identity, as shipment commands already require. Client-supplied `X-Tenant-ID`, body tenant, or query tenant is not an authorization source. Every plan, stop, action, and driver-task read and write includes `tenant_id` and the shipment predicate. Cross-tenant ids return not found, not a foreign object.

## Driver authorization

```text
DRIVER_AUTHORIZATION=ASSIGNED_DRIVER_ONLY
```

The driver identity is the gateway driver principal already used by driver operations. Commands apply only when `shipment.driver_id` is that driver and the stop's task names that driver. A driver cannot choose another shipment by id guess beyond the existing not-found behavior. Operator override uses actor `OPERATOR` inside the same tenant, not the driver flag.

## Carrier and shipper

```text
CARRIER_VISIBILITY=CARRIER_COMPANY_ON_THE_SHIPMENT
SHIPPER_VISIBILITY=OWN_CARGO_ACTIONS_ONLY
```

The carrier assigned on the shipment can read the execution plan, stop statuses, and actions for that shipment. A shipper can read stop progress for actions whose cargo belongs to that shipper's shipment. On a vehicle that is also carrying another shipper's cargo, the shipper API does not return the other cargo id, the other shipper identity, or the other commercial terms.

The driver receives action summaries for cargo they must pick up or deliver on the assigned shipment's active plan. That is execution-authorized data. It is not the optimizer marketplace projection and it does not include evaluation scores, capacity snapshots, or foreign owner tenant ids.

## Control Tower

```text
CONTROL_TOWER_VISIBILITY=TENANT_OPERATOR_READ_MODEL
```

Operators see execution progress for shipments in their tenant through the read model. Control Tower does not write stop completion. It may create the existing notice tasks. Sequence override is a shipment-service command authorized for the operator role, audited on the stop.

## Event payload privacy

```text
EVENT_PAYLOAD_PRIVACY=EXECUTION_FACTS_ONLY
```

Execution events include tenant, shipment, plan, stop, action, status, ordinal, and timestamps. They may include `cargo_id` for the action. They omit optimizer fingerprints beyond the plan's `evaluation_fingerprint` reference, leg geometry, prices, other tenants, and capacity snapshots. Driver APIs omit the same planning-only fields.

Projection rejects a contract body that contains a tenant id other than the shipment tenant.
