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

Tests 1-20 were not run. No live privilege check was run. Historical rows are not rewritten.
