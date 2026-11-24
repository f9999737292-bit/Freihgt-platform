# Tracking DML 0.1E — final report

```text
AGENT=C
STAGE=TRACKING-DML-ENFORCEMENT-DECISION-0.1E
BASE_SHA=3f306fc407aeda4bdc652d825954292f4a54fad6
REFERENCE_PR_375=9f31b6cfd96a06ece04eb4b946787f9813bfeeb4
REFERENCE_PR_374=89b509797ae61558582f4c198647ba13d21d9a47
REFERENCE_PR_357=0cf21055dd2a91c5ef071d6982c1fcc70cd11670
MIGRATION_HEAD=000105_platform_postgres_role_foundation_v0_2_i1
ALL_WRITERS_INVENTORIED=YES
OPTION_A_ASSESSED=YES
OPTION_B_ASSESSED=YES
OPTION_C_ASSESSED=YES
RECOMMENDED_OPTION=B
PROTECTED_WRITE_CONTRACT=DESIGNED_NOT_TESTED
DIRECT_DML_REVOCATION_PLAN=DESIGNED_NOT_APPLIED
SHARED_FREIGHT_RISK=OPEN
PUBLIC_EXECUTE_AUDIT=REQUIRED
OWNER_EPOCH_MONOTONICITY_DESIGNED=YES
OWNER_EPOCH_MONOTONICITY=SPECIFIED_NOT_ENFORCED
ETA_WRITER_MIGRATION=DESIGNED_NOT_APPLIED
MARK_APPROACH_MIGRATION=DESIGNED_NOT_APPLIED
OLD_CONSUMER_BLOCKING_PLAN=DESIGNED_NOT_ENFORCED
TEST_PLAN_COMPLETED=DESIGNED_NOT_EXECUTED
SHIPMENT_OWNER_RACE=OPEN_UNTIL_IMPLEMENTED
BROKER_SAFETY_PROVEN=NO
ALL_WRITERS_FENCED=NO
TENANT_ISOLATION_IMPLEMENTED=NO
MIXED_CONSUMER_GATE=CLOSED
DOCS_ONLY=YES
PRODUCT_CODE_CHANGED=NO
MIGRATIONS_CREATED=NO
DB_ROLES_CHANGED=NO
STAGING_CHANGED=NO
PRODUCTION_CHANGED=NO
MERGE_PERFORMED=NO
PR_375_CHANGED=NO
PR_374_CHANGED=NO
PR_357_CHANGED=NO
IMPLEMENTATION_GATE=CLOSED
DB_ROLE_SPLIT_APPROVED=NO
DIRECT_DML_ENFORCEMENT_APPROVED=NO
PUBLIC_EXECUTE_AUDITED=NO
ALL_WRITERS_MIGRATION_PLAN_APPROVED=NO
HISTORICAL_DATA_POLICY_APPROVED=NO
BROKER_TEST_ENVIRONMENT_APPROVED=NO
CI_RESULT=RECORDED_ON_THE_PULL_REQUEST_AFTER_THIS_COMMIT
FINAL_VERDICT=READY_FOR_J_REVIEW
```

## Decision

Option B is the recommended investigation shape. A non-owner runtime role loses direct DML on `tracking.execution_tracking_state`, `tracking.execution_event_inbox`, and `tracking.execution_stop_eta_state`. Fixed `SECURITY DEFINER` functions lock `transport.transport_executions`, compare the stored tenant and `owner_epoch`, insert the inbox proof, and write the state effect in the same caller transaction. The functions do not commit. `CommitRecords` stays after that commit.

Option A leaves direct DML, so a statement can omit the check. Option C does not reject a statement either. The current writers of the protected table are already in tracking-service.

B is not a mandatory fence while the shared login may own the tables. No `OWNER TO` for those tables exists through `000105`, and the live owner was not queried. The function owner can also bypass the body if a session can become that role. `ALL_WRITERS_FENCED=NO`. The ownership move is a controller decision. Without it, A, B, and C remain open.

## Gates

All of the following stay closed until a separate controller decision:

- `DB_ROLE_SPLIT_APPROVED=NO`
- `DIRECT_DML_ENFORCEMENT_APPROVED=NO`
- `PUBLIC_EXECUTE_AUDITED=NO`
- `OWNER_EPOCH_ENFORCEMENT_DESIGNED=YES` and not installed
- `ALL_WRITERS_MIGRATION_PLAN_APPROVED=NO`
- `HISTORICAL_DATA_POLICY_APPROVED=NO`
- `BROKER_TEST_ENVIRONMENT_APPROVED=NO`
- `MIXED_CONSUMER_DEPLOYMENT_GATE=CLOSED`
- `TENANT_SCOPED_READ_APPROVED=NO`

Issue #344 stays open. No consumer, migration, role, broker test, or deployment was changed.

## Documents

- `docs/architecture/tracking/TRACKING_DML_0_1E_WRITER_MATRIX.md`
- `docs/architecture/tracking/TRACKING_DML_0_1E_OPTIONS.md`
- `docs/architecture/tracking/TRACKING_DML_0_1E_PROTECTED_WRITE_CONTRACT.md`
- `docs/architecture/tracking/TRACKING_DML_0_1E_PRIVILEGE_PLAN.md`
- `docs/architecture/tracking/TRACKING_DML_0_1E_EPOCH_ENFORCEMENT.md`
- `docs/architecture/tracking/TRACKING_DML_0_1E_TEST_PLAN.md`

Current code, the design, and unproven claims are separated in those files. The protected functions were not executed. `PROTECTED_WRITE_SAFE_WITHOUT_TESTS=NO`.

## R2

`J_DML_001=ADDRESSED`. The three named reads are inventoried with their `WHERE` clauses. `GetState` and `ListActiveTargetsForActors` have no tenant predicate. `GetExecutionStopETA` uses the caller-supplied `X-Tenant-ID`. Table `SELECT` is not granted to `tracking_runtime`.
`RECOMMENDED_READ_MODEL=HYBRID_PROTECTED_READ`. Direct `INSERT`, `UPDATE`, and `DELETE` stay revoked. The reads call fixed functions. A function argument that names the stored tenant is not tenant authorization.
`READ_PATHS_INVENTORIED=YES`. `MINIMAL_SELECT_MODEL=NOT_SELECTED`. `PROTECTED_READ_FUNCTIONS_EVALUATED=YES`. `CROSS_TENANT_READ_RISK=OPEN`. `PRIVILEGE_MATRIX_UPDATED=YES`. `TENANT_ISOLATION_TESTS_DEFINED=YES`.
`LIST_ACTIVE_TARGETS_TENANT_SOURCE=MISSING`. `TENANT_SCOPED_READ_APPROVED=NO`. `TENANT_ISOLATION_IMPLEMENTED=NO`.
`DIRECT_DML_REVOCATION=DESIGNED_NOT_APPLIED`. `SHARED_FREIGHT_RISK=OPEN`. `FUNCTION_OWNER_BYPASS=OPEN`. `PUBLIC_EXECUTE_AUDIT=NOT_RUN`.
`ALL_WRITERS_FENCED=NO`. `SHIPMENT_OWNER_RACE=OPEN_UNTIL_IMPLEMENTED`. `BROKER_SAFETY_PROVEN=NO`. `MIXED_CONSUMER_GATE=CLOSED`.
`CI_RESULT=RECORDED_ON_THE_PULL_REQUEST_AFTER_THIS_COMMIT`.
