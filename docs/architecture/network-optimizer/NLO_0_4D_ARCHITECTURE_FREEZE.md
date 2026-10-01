# NLO-0.4D-R1 architecture freeze

Docs only. No product code, no migration, no production mutation.

```text
AGENT=D
TASK=NLO-0.4D-R1
MODE=DOCS_ONLY_ARCHITECTURE_FREEZE
BASE_SHA=470f0a6fc915093f8bb91a32e97fcf33da3f3deb
BRANCH=feat/nlo-execution-handoff-v0.4d
DISCOVERY_STATUS=ACCEPTED
IMPLEMENTATION_AUTHORIZED=NO
MERGE_AUTHORIZED=NO
PRODUCTION_MUTATION=NO
NLO_0_4D_IMPLEMENTATION_STARTED=NO
```

Discovery on this SHA is the evidence. ADR-NET-022 records the decisions. The companion documents are `SERVICE_DURATION_SOURCE.md`, `EXECUTION_HANDOFF_ACK.md`, and `ACTIVATION_STATE_MACHINE.md`.

## Decisions

```text
SERVICE_DURATION_ARCHITECTURE_DECIDED=YES
SERVICE_DURATION_OWNER=OPERATING_TENANT_LOGISTICS_POLICY
SERVICE_DURATION_VERSIONED=YES
UNKNOWN_DURATION_FAIL_CLOSED=YES
TMS_ACK_ARCHITECTURE_DECIDED=YES
TMS_ACK_MECHANISM=SYNCHRONOUS_INTERNAL_API_PLUS_DURABLE_RECONCILIATION
ACK_CORRELATION_FIELDS=operating_tenant_id,activation_id,route_plan_id,execution_id,execution_revision_id
ACK_IDEMPOTENCY_MODEL=activation_id scoped by operating_tenant_id
ARCHITECTURE_SAFE_TO_IMPLEMENT=YES
PRODUCTION_ACTIVATION_RELEASE=BLOCKED_UNTIL_VERSIONED_POLICY_ROWS_EXIST
```

`ARCHITECTURE_SAFE_TO_IMPLEMENT=YES` means a later wave can implement this freeze without a cross-database write and without treating the client or the execution-created event as authority. It does not open production activation. The duration gate stays closed until versioned policy rows exist. This wave does not implement either side.

## Ownership

```text
NLO_PLAN_OWNER=YES
NLO_EXECUTION_OWNER=NO
TMS_EXECUTION_OWNER=YES
NLO_WRITES_TMS_DB=NO
TMS_WRITES_NLO_DB=NO
```

### Agent D

- Read the operating-tenant service-duration policy during evaluation.
- Record policy id and version on the plan dependency set and in the evaluation fingerprint.
- Re-check that version on accept and activate.
- Keep unknown duration fail-closed.
- Persist `PENDING_EXECUTION`.
- Call `CreateExecutionProjectionFromActivation` as the network-optimizer service identity.
- Require the five correlation fields from the stored revision.
- Store `execution_id` and `execution_revision_id`.
- Move `PENDING_EXECUTION` to `EXECUTION_LINKED`.
- Emit `network.route_plan.execution_linked` once.
- Retry a lost response from `PENDING_EXECUTION`.
- Set `REJECTED` only on a permanent TMS refusal.

### Agent C

- Expose the existing projection command on the shipment-service internal authenticated API.
- Return `operating_tenant_id`, `activation_id`, `route_plan_id`, `execution_id`, and `execution_revision_id` from the committed revision.
- Replay the same activation to the same execution root.
- Leave `shipment.execution_plan.created` as a TMS fact. It is not the acknowledgement.
- Leave return, redirect, completed stop history, and successor revisions on shipment-service.

### Shared contract

- Projection request in `docs/architecture/tms-multistop/ROUTEPLAN_EXECUTION_CONTRACT.md`.
- Response extended with `operating_tenant_id` and `route_plan_id`.
- Business key: `activation_id` within `operating_tenant_id`.
- Permanent refusal versus transport loss, as in `EXECUTION_HANDOFF_ACK.md`.
- Operating tenant is the carrier execution scope. `shipment_tenant_id` remains the shipment owner. A client tenant id is not authority.

## Depot start

```text
DEPOT_START_SUPPORTED=NO
DEPOT_START_BLOCKER=SHIPMENT_STATUS_NOT_ELIGIBLE
```

A depot-start plan stores `capacity_id` and a null `shipment_id`. Freshness returns an empty shipment status, and the depot-start allow-list rejects that empty status. Cargo actions on that plan are load opportunities until shipment and cargo ids exist. TMS already rejects an unmaterialized subject and writes nothing. The service-duration policy does not create those subjects and does not change the allow-list.

```text
DEPOT_START_FUTURE_REQUIREMENTS=MATERIALIZED_SHIPMENT_AND_CARGO_PER_CARGO_ACTION,SERVER_OWNED_DEPOT_START_ELIGIBILITY,VERSIONED_SERVICE_DURATION_POLICY,NO_ALLOW_LIST_WEAKENING
```

## TMS return

```text
TMS_RETURN_OWNERSHIP_CHANGED=NO
COMPLETED_HISTORY_MUTATED=NO
ONE_EXECUTION_ROOT=YES
```

Return and redirect stay on the TMS disposition and successor commands. NLO activation does not delete future disposition stops, does not put rejected cargo back on the plan, and does not edit a completed revision. A later route change is a new plan and, after a new acknowledgement, a successor revision of the same execution root.

## Scope

```text
PRODUCT_CODE_CHANGED=NO
MIGRATION_CREATED=NO
SHIPMENT_FSM_CHANGED=NO
NLO_SEARCH_BOUNDS_CHANGED=NO
TMS_EXECUTION_OWNER_CHANGED=NO
CONTROL_TOWER_EXECUTION_WRITE=NO
TRACKING_EXECUTION_WRITE=NO
BILLING_RUNTIME_CHANGED=NO
EDO_RUNTIME_CHANGED=NO
PRODUCTION_MUTATION=NO
```

Next action after R1 was controller review. NLO-0.4D-I2 then implemented the freeze on this branch. Controller acceptance and merge remain unauthorized.

## NLO-0.4D-I2 implementation

```text
NLO_0_4D_I2=IMPLEMENTED_PENDING_CONTROLLER_REVIEW
SERVICE_DURATION_OWNER=NETWORK_OPTIMIZER_SERVICE
POLICY_SCOPE=OPERATING_TENANT
POLICY_VERSIONING=MONOTONIC
POLICY_STATE=DRAFT/ACTIVE/RETIRED
ACTIVE_POLICY_IMMUTABLE=YES
MIGRATION=000094_nlo_service_duration_policy_v0_4d
TMS_ENDPOINT=POST /internal/v1/transport-executions/from-route-plan-activation
NLO_WRITES_TMS_DB=NO
TMS_WRITES_NLO_DB=NO
MERGE_AUTHORIZED=NO
```

The operating-tenant logistics policy is stored by network-optimizer-service. Published ACTIVE rows are the only duration source. Pickup and delivery seconds come from that row. A missing active policy stays fail-closed. The public evaluate request cannot supply a duration.

Activation still inserts `PENDING_EXECUTION`. When the projection client is configured, network-optimizer-service calls the shipment-service internal endpoint with the internal service token and the verified tenant. A matching acknowledgement stores `execution_id` and `execution_revision_id` and moves the row to `EXECUTION_LINKED`. The plan is not rebuilt. `network.route_plan.execution_linked` is emitted once in that same transaction. A lost response leaves the row pending and the same projection is retried. A correlation mismatch does not link. Depot-start stays blocked by `SHIPMENT_STATUS_NOT_ELIGIBLE`.
