# TRACKING-BINDING-AUTHORITY-DECISION-0.1H — Final Report

The shipment-owned contract is written for a controller decision. It is not implemented, and it is not approved.

```text
AGENT=C
STAGE=TRACKING-BINDING-AUTHORITY-DECISION-0.1H
BASE_SHA=3f306fc407aeda4bdc652d825954292f4a54fad6
FINAL_HEAD=
PR_NUMBER=
CI_RUN=
CI_RESULT=RECORDED_ON_THE_PULL_REQUEST_AFTER_THIS_COMMIT
SHIPMENT_AUTHORITY_CONTRACT=DESIGNED_FOR_CONTROLLER_DECISION
DRIVER_ONLY_POLICY=ALLOWED_FOR_DRIVER_MATCH_ONLY
VEHICLE_AUTHORIZATION=CLAIMED_VEHICLE_MUST_MATCH_CURRENT_ROWS
EXECUTION_SCOPE_AUTHORIZATION=LINK_PLUS_CURRENT_REVISION_AND_DRIVER
BINDING_WRITE_ENFORCEMENT=REVOKE_INSERT_PLUS_DECISION_FUNCTION
DUPLICATE_BINDING_POLICY=CONFLICT_UNLESS_SAME_DECISION_SHIPMENT
REVOCATION_POLICY=RECHECK_AND_OMIT
NAMED_SERVICE_IDENTITY=UNRESOLVED_NO_EXISTING_MECHANISM
USE_TIME_RECHECK=CURRENT_ROWS_OR_EMPTY
CROSS_TENANT_TEST_PLAN=SPECIFIED_NOT_RUN
J_DML_001=NOT_CLOSED
TENANT_ISOLATION_IMPLEMENTED=NO
ALL_WRITERS_FENCED=NO
BROKER_SAFETY_PROVEN=NO
IMPLEMENTATION_GATE=CLOSED
PRODUCT_CODE_CHANGED=NO
MIGRATIONS_CREATED=NO
STAGING_CHANGED=NO
PRODUCTION_CHANGED=NO
MERGE_PERFORMED=NO
FINAL_VERDICT=READY_FOR_J_REVIEW
```

`FINAL_HEAD`, `PR_NUMBER`, and `CI_RUN` are recorded on the pull request after this commit. They are not invented here.

## Contract

The checker is shipment-owned. It starts from the gateway user, resolves the driver (`driver_repository.go:82-86`), and applies `CanDriverAccessShipment` (`driver_operations.go:271-278`). A claimed vehicle must match the current shipment and execution vehicle columns. A missing vehicle is driver-only and does not authorize a vehicle lookup. Execution scope requires the active shipment link (`000088:76-81`) and the current revision. `owner_epoch` stays `ABSENT` until the column exists, and execution-target rights stay denied while it is absent. `authorizeCommand` remains the existing command check of operating tenant and driver (`transport_execution_command_repository.go:159-187`). It is not a vehicle check and it is not this contract's write path.

The binding write accepts a `decision_id`, re-reads current rows, and is the only writer after `INSERT` is revoked from the runtime role. `ON CONFLICT DO NOTHING` is rejected. The table owner can still write. Historical rows are not promoted.

The shared token is not a named service. A dedicated pair credential can become a name only through the receiver's map, and tracking's allowlist is empty. Integration JWT stays an external principal. No secret is changed.

Target reads re-check every device binding as its own scope. `LIMIT 1` is not a tenant choice. An unreadable authority source yields an empty set.

## Limits

Tests 1-15 were not run. PR #378, #377, #376, #375, #374, and #357 were not edited. Issue #344 was not closed.
