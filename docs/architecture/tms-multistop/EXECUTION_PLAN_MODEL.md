# Execution plan model

```text
EXECUTION_PLAN_MODEL=FIRST_CLASS_CHILD_OF_SHIPMENT
EXECUTION_PROJECTION_SOURCE=ACTIVATED_ROUTE_PLAN
IDEMPOTENT_EXECUTION_PROJECTION=YES
```

The shipment row cannot be the plan. It has one version, one origin, and one destination, and successor replans must leave the previous plan auditable. A nullable `active_route_plan_id` on `shipments` would either rewrite history or hide the successor. The plan is a first-class child. The shipment points at the current plan id; older plans remain.

## Minimum fields

| Field | Rule |
| --- | --- |
| `id` | Shipment-service allocated |
| `tenant_id` | From the shipment, never from the caller body |
| `shipment_id` | Required |
| `shipment_version` | Version observed at projection time |
| `source_route_plan_id` | External reference |
| `source_route_plan_version` | External reference |
| `source_activation_id` | Idempotency identity |
| `source_activation_version` | Detects a changed activation row |
| `evaluation_fingerprint` | Copied from the activation contract; not recomputed |
| `planning_mode` | `DEPOT_START` or `CURRENT_TRIP` as supplied by the contract |
| `supersedes_execution_plan_id` | Previous shipment plan, null for the first |
| `status` | `ACTIVE` or `SUPERSEDED` |
| `version` | Optimistic concurrency of this plan |
| `created_at`, `updated_at` | Server clocks |

`status=ACTIVE` is the only plan whose remaining stops accept driver commands. At most one `ACTIVE` plan per shipment.

```text
MAX_ACTIVE_EXECUTION_PLANS_PER_SHIPMENT=1
```

## Command

```text
CreateExecutionProjectionFromActivation
```

The command runs inside `shipment-service`. The caller is a trusted service principal for `network-optimizer-service`, or an internal consumer of that service's activation event. A driver, shipper, or anonymous client cannot submit stops.

### Idempotency

Reuse the existing pattern: a unique key plus a stored result. Do not add a generic idempotency platform.

| Situation | Result |
| --- | --- |
| Same `source_activation_id` | Return the same `ShipmentExecutionPlan` id. No second stop set |
| Same id, different contract body or fingerprint | `409 ACTIVATION_BODY_CONFLICT` |
| New activation that names this plan's route plan as the one it supersedes, and shipment version still matches the successor contract | Create the successor. Mark the old plan `SUPERSEDED` in the same transaction |
| New activation that does not supersede the active plan | `409 EXECUTION_PLAN_CONFLICT` |
| Activation status other than `EXECUTION_LINKED` | Do not project |

Unique constraint: `(tenant_id, source_activation_id)`.

Duplicate delivery of the activation event is the same-id case.

## What is copied

Ordered cargo stops and their actions, as defined in the stop and action documents. Planning-only rows (legs, capacity snapshots, scores, foreign commercial terms) are not copied into shipment tables and are not placed on driver APIs.

`planned_arrival` and `planned_departure` are copied once. Live ETA never overwrites them.

## Shipment status

Creating or superseding a plan does not change shipment status and does not move it backward.
