# TRACKING-TENANT-READ-AUTHORITY-0.1F — Final Report

Discovery and design only. The product still reads tracking state without a trusted tenant scope.

```text
AGENT=C
STAGE=TRACKING-TENANT-READ-AUTHORITY-0.1F
BASE_SHA=3f306fc407aeda4bdc652d825954292f4a54fad6
FINAL_HEAD=
PR_NUMBER=
CI_RUN=
CI_RESULT=RECORDED_ON_THE_PULL_REQUEST_AFTER_THIS_COMMIT
IDENTITY_INVENTORY=DOCUMENTED
DEVICE_BINDING_SOURCE=tracking.shipment_tracking_binding
COMPANY_MEMBERSHIP_SOURCE=core.company_memberships_NOT_CONSULTED
TRUSTED_SERVICE_IDENTITY=SHARED_TOKEN_ONLY
GET_STATE_AUTHORIZATION=BLOCKED_NO_TRUSTED_CALLER
GET_EXECUTION_STOP_ETA_AUTHORIZATION=BLOCKED_TOKEN_IS_NOT_TENANT
LIST_ACTIVE_TARGETS_AUTHORIZATION=PER_BINDING_SCOPE_NOT_SUFFICIENT
SHARED_ACTOR_SCOPE=SEPARATE_VERIFIED_SCOPES
MULTI_TENANT_BINDING_POLICY=PER_BINDING_NO_ARBITRARY_PICK
OWNER_EPOCH_INTEGRATION=READ_IS_NOT_THE_FENCE
PROTECTED_READ_MODEL=HYBRID_WITH_BLOCKING_GATES
TENANT_ISOLATION_TEST_PLAN=SPECIFIED_NOT_RUN
J_DML_001_CLOSURE_PLAN=DESIGNED_NOT_IMPLEMENTED
OPEN_RISKS=BINDING_CREATION_TRUST,SHARED_TOKEN_IS_NOT_A_TENANT,SHARED_FREIGHT_RISK,FUNCTION_OWNER_BYPASS,PUBLIC_EXECUTE_AUDIT,OWNER_EPOCH_NOT_ENFORCED,RELATIONSHIP_CHECK_NOT_WIRED
BLOCKERS=NONE_FOR_DOCS_REVIEW
DOCS_ONLY=YES
PRODUCT_CODE_CHANGED=NO
MIGRATIONS_CREATED=NO
DB_ROLES_CHANGED=NO
STAGING_CHANGED=NO
PRODUCTION_CHANGED=NO
MERGE_PERFORMED=NO
TENANT_ISOLATION_IMPLEMENTED=NO
ALL_WRITERS_FENCED=NO
BROKER_SAFETY_PROVEN=NO
IMPLEMENTATION_GATE=CLOSED
FINAL_VERDICT=READY_FOR_J_REVIEW
```

`FINAL_HEAD`, `PR_NUMBER`, and `CI_RUN` are filled in the pull-request conversation after commit, push, and the exact-head workflow. They are not invented in this file.

## What this stage decided

- `J-DML-001` is not closed. PR #376 removed the idea of granting table `SELECT` to the runtime role and still left a caller-supplied tenant as the read predicate. Matching that predicate to `operating_tenant_id` is not authority.
- `ListActiveTargetsForActors` is called from `approach.go:69` and `eta_ingest_service.go:192`. The SQL at `execution_tracking_repository.go:211-215` has no tenant predicate. One device may be active in several tenants (`000026:28-30`). The any-tenant lookup keeps one row by `active_from` (`tracking_repository.go:51-53`). The selected shape is one checked scope per binding. The binding insert still trusts the caller (`tracking_repository.go:472-477`), so the shape is not sufficient.
- `GetState` has no production caller and no trusted tenant. It stays blocked.
- `GetExecutionStopETA` is an internal route. `X-Internal-Service-Token` proves the shared secret (`eta_handler.go:282-288`). `X-Tenant-ID` is the caller header. The route stays blocked.
- A shared driver or vehicle is a set of scopes. It is not one tenant.
- Company membership and carrier relationships exist and are not consulted by these reads.
- The read is not the owner-epoch fence from PR #375.

## Evidence limits

- No live database owner was queried.
- No privilege test was run.
- Tests 1-20 were not run.
- The binding-revoke writer outside this service was not found.
- Reference PRs were not edited. At document time they remained at the SHAs in the identity inventory.

## Follow-up after acceptance

A later task, not this one, would update the PR #376 protected-read text so the claimed-tenant argument is no longer the contract. Implementation remains behind `IMPLEMENTATION_GATE=CLOSED`.
