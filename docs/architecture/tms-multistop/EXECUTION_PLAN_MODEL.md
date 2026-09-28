# Execution route and revision

```text
EXECUTION_PLAN_MODEL=TRANSPORT_EXECUTION_PLUS_REVISION
EXECUTION_ROUTE_MODEL=TRANSPORT_EXECUTION
EXECUTION_PROJECTION_SOURCE=PENDING_EXECUTION_CONTRACT
EXECUTION_PROJECTION_TRIGGER_IS_EXECUTION_LINKED=NO
EXECUTION_LINKED_IS_POST_PROJECTION_FACT=YES
IDEMPOTENT_EXECUTION_PROJECTION=YES
MULTI_SHIPMENT_EXECUTION_SUPPORTED_BY_MODEL=YES
CROSS_SHIPPER_EXECUTION_SUPPORTED_BY_MODEL=YES
PARTICIPANT_SHIPMENT_OWNERSHIP_PRESERVED=YES
SHIPMENT_REHOMED_TO_CARRIER_TENANT=NO
DEPOT_START_WITHOUT_PREEXISTING_SHIPMENT_SUPPORTED=YES
RAW_LOAD_OPPORTUNITY_EXECUTABLE=NO
EXECUTION_SUBJECT_MATERIALIZATION_REQUIRED=YES
```

The shipment row is not the plan. It has one version, one origin, and one destination. A depot-start RoutePlan may have a null `shipment_id`. A current trip may carry several cargo subjects. The execution root is `TransportExecution`. Each activation is a `TransportExecutionRevision` of that route.

## TransportExecution

| Field | Rule |
| --- | --- |
| `id` | Allocated by shipment-service |
| `operating_tenant_id` | Carrier execution scope. Not a shipment owner tenant |
| `carrier_company_id` | Carrier operating the route |
| `vehicle_id` | Nullable until assigned |
| `driver_id` | Nullable until assigned. Driver authorization uses this assignment |
| `current_revision_id` | The one `ACTIVE` revision, or null before the first commit |
| `created_at`, `updated_at` | Server clocks |

No `shipment_id` column is required. `DEPOT_START` omits it. Participant shipments are not required to share `operating_tenant_id`.

## TransportExecutionRevision

| Field | Rule |
| --- | --- |
| `id` | Allocated by shipment-service |
| `execution_id` | Parent route |
| `operating_tenant_id` | Same as the route |
| `source_route_plan_id` | External reference for this revision only |
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

`TransportExecutionParticipant` binds one materialized shipment without moving it: `shipment_id`, `shipment_tenant_id`, `shipment_version`, `cargo_id`, `cargo_version`, `route_subject_type`, `route_subject_id`, and the provenance that this binding came from the trusted projection contract. `shipment_tenant_id` stays the shipment's original owner tenant. `CURRENT_TRIP` includes the existing trip shipment here. It is not the route's parent.

## Command

```text
CreateExecutionProjectionFromActivation
```

The command runs inside `shipment-service`. The first implementation uses one trusted synchronous service command from `network-optimizer-service`. There is no second transport. A driver, shipper, or browser caller cannot submit stops or a foreign `shipment_tenant_id`.

The trigger status is `PENDING_EXECUTION`. `EXECUTION_LINKED` is not the input. Shipment-service returns `execution_id`, `revision_id`, and `activation_id` after the revision commits. Agent D then sets `EXECUTION_LINKED` in its own store and emits `network.route_plan.execution_linked`. That event is a post-link fact. `IMPLEMENTED_TODAY=NO`. `AGENT_D_CONTRACT_REQUIRED=YES`.

The command refuses the whole projection when any cargo action lacks `execution_shipment_id`, `shipment_tenant_id`, and `cargo_id`. Reason: `409 EXECUTION_SUBJECT_UNMATERIALIZED`. It does not create a shipment and it does not copy a foreign shipment into `operating_tenant_id`. The named shipment must already exist in `shipment_tenant_id`. See `ROUTEPLAN_EXECUTION_CONTRACT.md`.

### Idempotency

Reuse a unique key plus a stored result. Do not add a generic idempotency platform.

| Situation | Result |
| --- | --- |
| Same `source_activation_id` | Return the same `execution_id` and revision id. No second stop set |
| Same id, different contract body or fingerprint | `409 ACTIVATION_BODY_CONFLICT` |
| Cargo action without materialized shipment, owner tenant, and cargo | `409 EXECUTION_SUBJECT_UNMATERIALIZED`. No rows |
| New `PENDING_EXECUTION` activation that supersedes the active revision | Create the successor revision on the same route. Mark the old revision `SUPERSEDED` in the same transaction |
| New activation that does not supersede the active revision | `409 EXECUTION_PLAN_CONFLICT`. The old revision stays `ACTIVE` |
| `activation_status=EXECUTION_LINKED` as the projection trigger | Do not project. That status is written by Agent D only after this command has already returned ids |
| `activation_status` other than `PENDING_EXECUTION` | Do not project |

Unique constraint: `(operating_tenant_id, source_activation_id)`.

```text
SAME_ACTIVATION_RETURNS_SAME_EXECUTION_REVISION=YES
NO_STATE_WITH_EXECUTION_LINKED_BUT_NO_EXECUTION_PROJECTION=YES
```

Agent D must not persist `EXECUTION_LINKED` unless it has stored the returned `execution_id` and `revision_id` for that `activation_id`. A lost response is a retry of the same command, which returns the same ids.

## What is copied

Ordered stops and materialized actions. Planning-only rows are not copied and are not placed on driver APIs. `planned_arrival` and `planned_departure` are copied once. Live ETA never overwrites them.

## Shipment status

Creating or superseding a revision does not change any participant shipment's status and does not move it backward. A route with no participants yet, blocked on materialization, changes no shipment.
