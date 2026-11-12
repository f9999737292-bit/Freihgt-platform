# TRACKING-BINDING-AUTHORITY-0.1G — Final Report

Discovery and design only. The production binding insert still stores caller claims. Those rows are not a tenant grant.

```text
AGENT=C
STAGE=TRACKING-BINDING-AUTHORITY-0.1G
BASE_SHA=3f306fc407aeda4bdc652d825954292f4a54fad6
FINAL_HEAD=
PR_NUMBER=
CI_RUN=
CI_RESULT=RECORDED_ON_THE_PULL_REQUEST_AFTER_THIS_COMMIT
BINDING_CREATION_CHAIN_VERIFIED=YES
CURRENT_BINDING_TRUST=CALLER_CLAIMS
AUTHORITATIVE_TENANT_SOURCE=NONE_ON_THE_BINDING_WRITE
OPTION_A_ASSESSED=YES
OPTION_B_ASSESSED=YES
OPTION_C_ASSESSED=YES
RECOMMENDED_AUTHORITY_MODEL=B_SHIPMENT_OWNED_NOT_APPROVED
BINDING_LIFECYCLE_DESIGNED=YES
REVOCATION_CONTRACT=RECHECK_CURRENT_ASSIGNMENT
REASSIGNMENT_CONTRACT=OLD_SCOPE_FAILS_RECHECK
SHARED_ACTOR_SCOPE=SEPARATE_VERIFIED_SCOPES
MULTI_TENANT_SCOPE=PER_SCOPE_RECHECK_NO_LIMIT_1
PROTECTED_READ_INTEGRATION=SPECIFIED_NOT_IMPLEMENTED
NAMED_SERVICE_IDENTITY_DEPENDENCY=UNRESOLVED
TEST_PLAN_COMPLETED=SPECIFIED_NOT_RUN
J_DML_001=NOT_CLOSED
TENANT_ISOLATION_IMPLEMENTED=NO
ALL_WRITERS_FENCED=NO
BROKER_SAFETY_PROVEN=NO
IMPLEMENTATION_GATE=CLOSED
OPEN_RISKS=DIRECT_INTERNAL_INSERT,ON_CONFLICT_DO_NOTHING_RACE,HISTORICAL_ROWS_ARE_NOT_GRANTS,SHARED_TOKEN,NAMED_SERVICE_IDENTITY_UNRESOLVED,RELATIONSHIP_PIN_NOT_WIRED,SHARED_FREIGHT_RISK,OWNER_EPOCH_NOT_ENFORCED
BLOCKERS=NONE_FOR_DOCS_REVIEW
DOCS_ONLY=YES
PRODUCT_CODE_CHANGED=NO
MIGRATIONS_CREATED=NO
DB_ROLES_CHANGED=NO
STAGING_CHANGED=NO
PRODUCTION_CHANGED=NO
MERGE_PERFORMED=NO
FINAL_VERDICT=READY_FOR_J_REVIEW
```

`FINAL_HEAD`, `PR_NUMBER`, and `CI_RUN` are filled in the pull-request conversation after commit, push, and the exact-head workflow.

## Decision

The binding chain is `router.go:218` to `driver/handler.go:229-264` to `tracking_handler.go:96-135` to `ingest_service.go:64` to `tracking_repository.go:456-478`. Tracking authenticates the shared token and stores `X-Tenant-ID`, `X-Driver-ID`, the path shipment, and an optional vehicle. Shipment-service already resolves the driver from the user and checks `CanDriverAccessShipment`, and the gateway calls that check only to read an HTTP status.

Option A is the user-entry boundary and does not own the assignment rows. Option B keeps shipment-service as the source of the driver, the shipment assignment, and the execution assignment, and it re-checks those rows at use time. Option C would add an issuer the repository does not have. The recommendation is B. The controller has not approved it. `IMPLEMENTATION_GATE` stays closed.

`ListActiveTargetsForActors` can return a target only for a scope that is ACTIVE under that re-check. A shared actor stays a set of scopes. `LIMIT 1` is not a tenant choice. `GetState` stays blocked. Stop ETA stays blocked until a named service identity exists. `J-DML-001` is not closed.

## Compatibility

PR #377, #376, #375, #374, and #357 were not edited. A later accepted decision would update the PR #377 binding-trust wording and the PR #376 read-owner column list. The owner-epoch write order and the Kafka contracts are unchanged.

## Limits

Tests 1-25 were not run. No live privilege check was run. Historical rows are not rewritten.

## R2 — J-TBA-001

`J_TBA_001=ADDRESSED`

`authorizeCommand` (`transport_execution_command_repository.go:159-187`) compares the operating tenant and, for a driver actor, the locked driver. The lock select is `operating_tenant_id, driver_id` (`:70-75`). `CanDriverAccessShipment` (`driver_operations.go:271-278`) compares the shipment tenant and `driver_id`. Neither function checks a vehicle. `GetByUserIDAndTenant` (`driver_repository.go:82-86`) resolves the driver row.

`EXISTING_DRIVER_CHECK=VERIFIED_BY_CODE`

`EXISTING_VEHICLE_CHECK=NOT_IMPLEMENTED_IN_THIS_PATH`

`VEHICLE_CHECK=PROPOSED`

`VEHICLE_CHECK_SOURCE=CURRENT_SHIPMENT_AND_EXECUTION_ROWS_WHEN_CLAIMED`

`DRIVER_ONLY_POLICY=ALLOWED_FOR_DRIVER_MATCH_ONLY`

`VEHICLE_REASSIGNMENT_POLICY=CLAIMED_VEHICLE_MUST_MATCH_CURRENT_ROW`

`EXECUTION_REVISION_POLICY=STALE_REVISION_IS_NOT_ACTIVE`

`SHIPMENT_EXECUTION_LINK=PROPOSED`

`BINDING_IS_AUTHORIZATION_PROOF=NO`

`DIRECT_BINDING_INSERT_BYPASS=OPEN`

`ON_CONFLICT_DO_NOTHING_RACE=OPEN`

`SHIPMENT_AUTHORITY_SOURCE=DESIGNED_NOT_IMPLEMENTED`

`J_DML_001=NOT_CLOSED`

`TEST_MATRIX_EXTENDED=T21_T25_NOT_RUN`

The binding insert still does not consume those checks. A stored row is not promoted to a decision. Tests 21-25 were not executed.

```text
AGENT=C
STAGE=TRACKING-BINDING-AUTHORITY-0.1G-R2
PR_NUMBER=378
PREVIOUS_HEAD=b23a56c2687eb3fb35f576d744b6a9a20e06811a
NEW_HEAD=
REMOTE_HEAD=
J_TBA_001=ADDRESSED
EXISTING_DRIVER_CHECK=VERIFIED_BY_CODE
EXISTING_VEHICLE_CHECK=NOT_IMPLEMENTED_IN_THIS_PATH
VEHICLE_CHECK_SOURCE=CURRENT_SHIPMENT_AND_EXECUTION_ROWS_WHEN_CLAIMED
DRIVER_ONLY_POLICY=ALLOWED_FOR_DRIVER_MATCH_ONLY
VEHICLE_REASSIGNMENT_POLICY=CLAIMED_VEHICLE_MUST_MATCH_CURRENT_ROW
EXECUTION_REVISION_POLICY=STALE_REVISION_IS_NOT_ACTIVE
TEST_MATRIX_EXTENDED=T21_T25_NOT_RUN
BINDING_IS_AUTHORIZATION_PROOF=NO
DIRECT_BINDING_INSERT_BYPASS=OPEN
J_DML_001=NOT_CLOSED
TENANT_ISOLATION_IMPLEMENTED=NO
ALL_WRITERS_FENCED=NO
BROKER_SAFETY_PROVEN=NO
DOCS_ONLY=YES
PRODUCT_CODE_CHANGED=NO
MIGRATIONS_CREATED=NO
DB_ROLES_CHANGED=NO
STAGING_CHANGED=NO
PRODUCTION_CHANGED=NO
MERGE_PERFORMED=NO
CI_RUN=
CI_RESULT=RECORDED_ON_THE_PULL_REQUEST_AFTER_THIS_COMMIT
IMPLEMENTATION_GATE=CLOSED
FINAL_VERDICT=READY_FOR_J_R2_REVIEW
```
