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
CROSS_SHIPPER_EXECUTION_SUPPORTED_BY_MODEL=YES
PARTICIPANT_SHIPMENT_OWNERSHIP_PRESERVED=YES
SHIPMENT_REHOMED_TO_CARRIER_TENANT=NO
CROSS_TENANT_RAW_SHIPMENT_SCAN=NO
```

Two scopes stay separate.

`operating_tenant_id` is the carrier execution scope on `TransportExecution`. `shipment_tenant_id` is the owner scope on each `TransportExecutionParticipant` and stays the shipment's original tenant. Projection does not clone or recreate a foreign shipment into the carrier tenant.

`tenant_id` on a browser or driver request comes from the gateway-established identity. A client-supplied tenant, including a foreign `shipment_tenant_id`, is not an authorization source. Execution access does not come from guessing another tenant's shipment list. Shipment-service loads a participant shipment by its primary key inside `shipment_tenant_id` only when a trusted projection already bound that pair, or when the caller's own tenant owns that shipment.

Cargo evidence writes for a participant run inside shipment-service under that binding. The writer is the execution command, not a browser tenant override.

A `LOAD_OPPORTUNITY` is still not an execution identity. Materialization creates or names a shipment in the shipper's own tenant. Raw load-opportunity ids are not returned on driver or shipper APIs.

## Driver authorization

```text
DRIVER_AUTHORIZATION=ASSIGNED_DRIVER_ONLY
```

The driver identity is the gateway driver principal already used by driver operations. Route commands apply only when `TransportExecution.driver_id` is that driver. Authorization is the execution assignment in `operating_tenant_id`. It does not require every participant shipment to belong to the carrier tenant. The sequence is not authorized by picking one participant shipment. Operator override uses actor `OPERATOR` inside `operating_tenant_id`, not a client-supplied foreign tenant.

## Carrier and shipper

```text
CARRIER_VISIBILITY=CARRIER_COMPANY_ON_THE_SHIPMENT
SHIPPER_VISIBILITY=OWN_CARGO_ACTIONS_ONLY
```

The carrier on `TransportExecution` can read the route, stop statuses, and the action fields needed to run the assigned route. That read omits other shippers' tenant ids, raw marketplace owner identities, prices, and planning fingerprints.

A shipper read returns only that shipper's participant and action facts. It omits other shipper tenant ids, other cargo ids, and commercial or private planning fields.

The driver payload lists location, action type, and the cargo id required for the assigned stop. It does not include raw `LOAD_OPPORTUNITY` records, other owners' tenant ids, evaluation scores, or capacity snapshots.

## Control Tower

```text
CONTROL_TOWER_VISIBILITY=TENANT_OPERATOR_READ_MODEL
```

Operators in `operating_tenant_id` see route progress without other shippers' tenant ids on the operational projection. A shipper-tenant operator sees only that shipper's participant facts. Control Tower does not write stop completion. It may create the existing notice tasks. Sequence override is a shipment-service command authorized for an operator in `operating_tenant_id`, audited on the stop.

## Event payload privacy

```text
EVENT_PAYLOAD_PRIVACY=EXECUTION_FACTS_ONLY
```

Execution events include operating tenant, route, revision, stop, action, status, ordinal, and timestamps. They may include `shipment_id` and `cargo_id` for a materialized action. External payloads omit `shipment_tenant_id`, raw load-opportunity payloads, optimizer fingerprints beyond the revision's `evaluation_fingerprint` reference, leg geometry, prices, and capacity snapshots. `shipment_tenant_id` stays on the participant row for the trusted evidence write.
