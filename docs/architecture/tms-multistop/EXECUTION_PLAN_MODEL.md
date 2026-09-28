# Execution route and revision

```text
EXECUTION_PLAN_MODEL=TRANSPORT_EXECUTION_PLUS_REVISION
EXECUTION_ROUTE_MODEL=TRANSPORT_EXECUTION
EXECUTION_PROJECTION_SOURCE=ACTIVATED_ROUTE_PLAN
IDEMPOTENT_EXECUTION_PROJECTION=YES
MULTI_SHIPMENT_EXECUTION_SUPPORTED_BY_MODEL=YES
DEPOT_START_WITHOUT_PREEXISTING_SHIPMENT_SUPPORTED=YES
RAW_LOAD_OPPORTUNITY_EXECUTABLE=NO
EXECUTION_SUBJECT_MATERIALIZATION_REQUIRED=YES
```

The shipment row is not the plan. It has one version, one origin, and one destination. A depot-start RoutePlan may have a null `shipment_id`. A current trip may carry several cargo subjects. The execution root is `TransportExecution`. Each activation is a `TransportExecutionRevision` of that route.

## TransportExecution

| Field | Rule |
| --- | --- |
| `id` | Allocated by shipment-service |
| `tenant_id` | Gateway tenant of the execution. Every participant shipment must match it |
| `carrier_company_id` | Carrier operating the route |
| `vehicle_id` | Nullable until assigned |
| `driver_id` | Nullable until assigned. The driver sequence uses this driver |
| `current_revision_id` | The one `ACTIVE` revision, or null before the first commit |
| `created_at`, `updated_at` | Server clocks |

No `shipment_id` column is required. `DEPOT_START` omits it.

## TransportExecutionRevision

| Field | Rule |
| --- | --- |
| `id` | Allocated by shipment-service |
| `execution_id` | Parent route |
| `tenant_id` | Same as the route |
| `source_route_plan_id` | External reference |
| `source_route_plan_version` | External reference |
| `source_activation_id` | Idempotency identity |
| `source_activation_version` | Detects a changed activation row |
| `evaluation_fingerprint` | Copied. Not recomputed |
| `planning_mode` | `DEPOT_START` or `CURRENT_TRIP` |
| `supersedes_revision_id` | Previous revision of this route, or null |
| `status` | `ACTIVE` or `SUPERSEDED` |
| `version` | Optimistic concurrency of this revision |
| `created_at`, `updated_at` | Server clocks |

```text
MAX_ACTIVE_REVISIONS_PER_TRANSPORT_EXECUTION=1
SHIPMENT_IN_AT_MOST_ONE_ACTIVE_EXECUTION=YES
```

Only the `ACTIVE` revision accepts new driver commands on its introduced or in-service stops.

## Participants

`TransportExecutionParticipant` binds a materialized shipment and cargo to the route: `shipment_id`, `shipment_version`, `cargo_id`, `cargo_version`, plus the planning subject type and id as references. `CURRENT_TRIP` includes the existing trip shipment here. It is not stored as the route's parent.

## Command

```text
CreateExecutionProjectionFromActivation
```

The command runs inside `shipment-service`. The caller is a trusted service principal for `network-optimizer-service`, or an internal consumer of that service's future activation event. A driver, shipper, or anonymous client cannot submit stops.

The command refuses the whole projection when any cargo action lacks `execution_shipment_id` and `cargo_id`. Reason: `409 EXECUTION_SUBJECT_UNMATERIALIZED`. It does not create a shipment from a `LOAD_OPPORTUNITY`. Materialization is a prior shipment-service fact: the shipment and cargo already exist and the contract names them. See `ROUTEPLAN_EXECUTION_CONTRACT.md`.

### Idempotency

Reuse a unique key plus a stored result. Do not add a generic idempotency platform.

| Situation | Result |
| --- | --- |
| Same `source_activation_id` | Return the same revision id. No second stop set |
| Same id, different contract body or fingerprint | `409 ACTIVATION_BODY_CONFLICT` |
| Cargo action without materialized shipment and cargo | `409 EXECUTION_SUBJECT_UNMATERIALIZED`. No rows |
| New activation that supersedes the active revision | Create the successor revision on the same route. Mark the old revision `SUPERSEDED` in the same transaction |
| New activation that does not supersede the active revision | `409 EXECUTION_PLAN_CONFLICT` |
| Activation status other than `EXECUTION_LINKED` | Do not project |

Unique constraint: `(tenant_id, source_activation_id)`.

## What is copied

Ordered stops and materialized actions. Planning-only rows are not copied and are not placed on driver APIs. `planned_arrival` and `planned_departure` are copied once. Live ETA never overwrites them.

## Shipment status

Creating or superseding a revision does not change any participant shipment's status and does not move it backward. A route with no participants yet, blocked on materialization, changes no shipment.
