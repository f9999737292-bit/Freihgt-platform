# Service duration source

NLO-0.4D-R1 architecture freeze. Docs only. Baseline `470f0a6fc915093f8bb91a32e97fcf33da3f3deb`.

```text
SERVICE_DURATION_ARCHITECTURE_DECIDED=YES
SERVICE_DURATION_OWNER=OPERATING_TENANT_LOGISTICS_POLICY
SERVICE_DURATION_STORAGE=VERSIONED_OPERATING_TENANT_POLICY
SERVICE_DURATION_VERSIONING=MONOTONIC_POLICY_VERSION
SERVICE_DURATION_PRECEDENCE=TENANT_ACTION_POLICY_ONLY
SERVICE_DURATION_FALLBACK_POLICY=NONE
UNKNOWN_DURATION_POLICY=FAIL_CLOSED
CLIENT_SERVICE_DURATION_TRUSTED=NO
UNKNOWN_SERVICE_DURATION_FAIL_CLOSED=YES
SERVICE_DURATION_FALLBACK_APPROVED=NO
DEFAULT_ZERO=NO
NUMERIC_PICKUP_SECONDS_PUBLISHED=NO
NUMERIC_DELIVERY_SECONDS_PUBLISHED=NO
PRODUCT_CODE_CHANGED=NO
MIGRATION_CREATED=NO
```

ADR-NET-019 already requires an authoritative source or an unknown result. NLO-0.4A allows an indeterminate plan as advice and refuses activation until a policy names an owner and a version. This document is that policy. It does not invent pickup or delivery minutes. `OPEN_QUESTIONS.md` Q2 stays open: facility history is not selected.

## Owner

| Candidate | Result |
| --- | --- |
| Shipment / TMS stop or action | Rejected as authority. `transport.transport_execution_stops.service_duration_seconds` is a copy of the plan. The plan must already hold the duration before projection. Return and redirect stops are TMS disposition facts. |
| Platform reference / catalog | Rejected. Groupage catalog and rule sets score equipment compatibility. City rules are ADR-NET-009 and are not cargo dwell. No duration row exists there. |
| Optimizer policy constant | Rejected. ADR-NET-019 sets `DEFAULT_ZERO=NO`. A constant minute value would be an invented duration. The optimizer consumes a versioned fact. |
| Operating-tenant logistics policy | Selected. Dwell is a server-owned policy of the tenant that operates the route. |

```text
SERVICE_DURATION_OWNER=OPERATING_TENANT_LOGISTICS_POLICY
SERVICE_DURATION_READER=network-optimizer-service
SERVICE_DURATION_AUTHOR=OPERATING_TENANT_SERVER_POLICY
TMS_DURATION_AUTHORITY=NO
OPTIMIZER_DURATION_AUTHORITY=NO
```

The public evaluate body stays limited to planning mode, shipment or capacity identity, and candidate load ids. A client duration field is ignored if a later request adds one. `CLIENT_SERVICE_DURATION_TRUSTED=NO`.

## Storage and version

One published policy belongs to one operating tenant. Each published generation has a monotonic version. The policy maps action type to whole seconds:

```text
PICKUP   -> seconds at that version, or ABSENT
DELIVERY -> seconds at that version, or ABSENT
```

`ABSENT` is not zero. The plan stop stores the resolved seconds as a snapshot. The policy row remains the authority. TMS copies the snapshot at projection and does not resolve the policy again.

NLO-0.4D-I2 persists that policy in network-optimizer-service. Migration `000094_nlo_service_duration_policy_v0_4d` adds `service_duration_policies` and `service_duration_policy_entries`. The row owner is the network optimizer. The scope is the operating tenant. Versions are monotonic. Status is `DRAFT`, `ACTIVE`, or `RETIRED`. One `ACTIVE` row is allowed per tenant, and an `ACTIVE` row cannot be edited. `route_plan_dependencies.dependency_kind` includes `SERVICE_DURATION_POLICY`.

```text
SERVICE_DURATION_OWNER=NETWORK_OPTIMIZER_SERVICE
POLICY_SCOPE=OPERATING_TENANT
SERVICE_DURATION_STORAGE=network_optimizer.service_duration_policies
```

```text
POLICY_KEY=operating_tenant_id + policy_version
ACTION_KEYS=PICKUP,DELIVERY
SNAPSHOT_COLUMN=route_plan_stops.service_duration_seconds
TMS_COPY_COLUMN=transport.transport_execution_stops.service_duration_seconds
```

## Precedence

1. The operating tenant's published policy version for that action type.
2. No facility or historical dwell override. Q2 is not decided here.
3. No platform default.
4. No optimizer constant.
5. No client value.
6. No TMS execution-stop value read back into planning.

If step 1 is absent, the duration is unknown.

## Unknown policy

```text
UNKNOWN_SERVICE_DURATION_FAIL_CLOSED=YES
TIME_FEASIBILITY=INDETERMINATE
PLAN_RESULT=INDETERMINATE
ACTIVATION_ALLOWED=NO
PRODUCTION_ACTIVATION_RELEASE=BLOCKED_UNTIL_VERSIONED_POLICY_ROWS_EXIST
```

An indeterminate plan may still be stored as advice, which is the NLO-0.4B rule. Accept may freeze that advice. Activate refuses it while `ProductionActivationRequiresServiceDurationSource` stays true. Zero is not substituted.

## Applicability

| Case | Rule |
| --- | --- |
| `PICKUP` | Resolve seconds from the published policy. Absent seconds make the plan indeterminate and block activation. |
| `DELIVERY` | Same rule, separate policy entry. Onboard delivery on `END` is a delivery. |
| `START` | No cargo service duration. A null stop duration is valid when the stop has no actions. |
| `END` without actions | Null duration is valid. |
| `END` or `CARGO` with actions | Each action resolves on its type. The stop snapshot is the sum of those seconds, in action order. Any absent entry makes the stop unknown. |
| `DEPOT_START` | Planning mode, not a duration type. Cargo actions on that plan still use the pickup and delivery entries. The mode stays ineligible for activation. See `NLO_0_4D_ARCHITECTURE_FREEZE.md`. |
| `RETURN`, `REDIRECT` | TMS disposition. NLO does not assign a planning duration and does not overwrite those stops. |
| Other stop roles (`HUB`, `BREAK`, and any role outside `START`, `CARGO`, `END`) | Out of the v0.4 planner. No duration authority is assigned. |

## Where the version enters

| Stage | Use |
| --- | --- |
| Evaluation | Load the operating tenant's current published policy. Resolve each cargo action. Missing data yields `SERVICE_DURATION_UNKNOWN`. |
| Dependency set | Record kind `SERVICE_DURATION_POLICY`, the policy id, and the policy version. |
| Fingerprint | The evaluation fingerprint includes the policy id, the policy version, and each stop's resolved seconds. A single plan-wide integer is not enough once pickup and delivery differ. Today's `SERVICE_KNOWN` marker is replaced by that per-stop record in the later implementation. |
| Accept | Re-read the policy version. A mismatch is `PLAN_STALE`. |
| Activate | The same freshness check. Unknown duration still refuses activation before any TMS call. |
| Projection | Copy the stop snapshot. TMS does not look up the policy. A null snapshot on a cargo stop with actions must not be linked. |

Search bounds stay unchanged. This policy does not raise load, stop, sequence, or routing caps.
