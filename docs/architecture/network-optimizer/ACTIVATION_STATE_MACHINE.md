# Activation state machine

NLO-0.4D-R1 architecture freeze. Docs only. The plan lifecycle and the activation lifecycle are different rows.

```text
PLAN_STATUSES=EVALUATED,ACCEPTED,SUPERSEDED,CANCELLED
ACTIVATION_STATUSES=PENDING_EXECUTION,EXECUTION_LINKED,REJECTED
ROUTE_PLAN_REOPTIMIZED_ON_LINK=NO
EXECUTION_LINK_REQUIRES_TMS_ACK=YES
FAKE_EXECUTION_ID_ALLOWED=NO
```

## Plan

```text
EVALUATED -> ACCEPTED
ACCEPTED -> SUPERSEDED
EVALUATED -> CANCELLED
ACCEPTED -> CANCELLED
```

Accept checks the plan version and the dependency fingerprints. It does not create an activation and it does not call shipment-service. Structure stays immutable after `EVALUATED`.

## Activation

```text
ACCEPTED plan -> PENDING_EXECUTION
PENDING_EXECUTION -> EXECUTION_LINKED
PENDING_EXECUTION -> REJECTED
```

Forbidden:

```text
EVALUATED -> EXECUTION_LINKED
ACCEPTED -> EXECUTION_LINKED without a durable activation row
PENDING_EXECUTION -> EXECUTION_LINKED without the TMS acknowledgement
EXECUTION_LINKED -> PENDING_EXECUTION
EXECUTION_LINKED -> REJECTED
REJECTED -> EXECUTION_LINKED
PENDING_EXECUTION -> EVALUATED
```

`EXECUTION_LINKED` requires both `execution_id` and `execution_revision_id`, which is already the `000086` check. `PENDING_EXECUTION` and `REJECTED` require both ids null. `effective_shipment_id` is set only on the effective linked row.

The link time is the `occurred_at` of `network.route_plan.execution_linked`. This freeze does not add `linked_at`.

## Gates before `PENDING_EXECUTION`

Activate refuses when any of these fail:

- plan status is not `ACCEPTED`
- plan version or a dependency fingerprint is stale (`PLAN_STALE`)
- result is not feasible, or a compatibility snapshot is `INDETERMINATE`
- service duration is unknown (`SERVICE_DURATION_UNKNOWN`)
- a route leg is expired
- shipment status is outside the mode allow-list (`SHIPMENT_STATUS_NOT_ELIGIBLE`)

Depot-start plans fail the status allow-list. That refusal stays.

## Link transaction

NLO writes the two execution ids and `EXECUTION_LINKED` in one transaction, and inserts `network.route_plan.execution_linked` in that same transaction. A crash before commit leaves `PENDING_EXECUTION` and no event. A crash after commit leaves `EXECUTION_LINKED` and one event.

The plan row stays `ACCEPTED`. Linking does not re-run the planner.

```text
ROUTE_PLAN_REOPTIMIZED_ON_LINK=NO
PLAN_STATUS_ON_LINK=ACCEPTED
```

## Idempotent edges

| Current row | Input | Next row |
| --- | --- | --- |
| none | activate | `PENDING_EXECUTION` |
| `PENDING_EXECUTION` | activate again | same row |
| `PENDING_EXECUTION` | same TMS acknowledgement | `EXECUTION_LINKED` |
| `EXECUTION_LINKED` | same acknowledgement | same row, no second event |
| `PENDING_EXECUTION` | permanent TMS refusal | `REJECTED` |
| `REJECTED` | same refusal | same row |
| `PENDING_EXECUTION` | lost response or timeout | `PENDING_EXECUTION` |

A successor plan uses a new plan id and a new activation. It does not rewrite the linked activation. Supersession of an execution revision stays a TMS successor command.
