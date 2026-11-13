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

The block above is the R1 snapshot. R2 supersedes `EXECUTION_SCOPE_AUTHORIZATION`. The current execution-target rule is `BLOCKED_WHILE_OWNER_EPOCH_ABSENT`.

## Contract

The checker is shipment-owned. It starts from the gateway user, resolves the driver (`driver_repository.go:82-86`), and applies `CanDriverAccessShipment` (`driver_operations.go:271-278`). A claimed vehicle must match the current shipment and execution vehicle columns. A missing vehicle is driver-only and does not authorize a vehicle lookup. An execution target requires the active shipment link (`000088:76-81`), the current revision, the driver match, a claimed vehicle when one was claimed, the operating tenant from the execution row, a confirmed owner epoch, and a use-time recheck. `EXECUTION_TARGET_AUTHORIZATION=BLOCKED_WHILE_OWNER_EPOCH_ABSENT`. `authorizeCommand` remains the existing command check of operating tenant and driver (`transport_execution_command_repository.go:159-187`). It is not a vehicle check and it is not this contract's write path.

The binding write is specified as one database transaction and is not granted. The presenter would have to be the gateway user, and that value cannot be a function argument. `BINDING_WRITE=BLOCKED_UNTIL_TRUSTED_PRESENTER`. `ON CONFLICT DO NOTHING` is rejected. The table owner can still write. Historical rows are not promoted.

The shared token is not a named service. A dedicated pair credential can become a name only through the receiver's map, and tracking's allowlist is empty. Integration JWT stays an external principal. No secret is changed.

Target reads re-check every device binding as its own scope. `LIMIT 1` is not a tenant choice. An unreadable authority source yields an empty set. A revision change or a shipment-version change requires a new decision. The old decision is not a version-independent location proof.

## Limits

Tests 1-49 were not run. PR #378, #377, #376, #375, #374, and #357 were not edited. Issue #344 was not closed.

## R2 — J-TBH-001, J-TBH-002, J-TBH-003

`J_TBH_001=ADDRESSED`

`J_TBH_002=ADDRESSED`

`J_TBH_003=ADDRESSED`

The R1 token `EXECUTION_SCOPE_AUTHORIZATION=LINK_PLUS_CURRENT_REVISION_AND_DRIVER` is superseded. An execution target is denied while the owner epoch is unconfirmed. The location right is not kept from an old decision across a revision change or a shipment-version change.

`DECISION_TABLE_SPECIFIED=transport.binding_authority_decisions`

`DECISION_TABLE_OWNER=binding_authority_owner`

`DECISION_GRANTS=OWNER_INSERT_VIA_ISSUE_FUNCTION_FENCE_UPDATE_CONSUMED_AT_ONLY`

`PRESENTER_IDENTITY_SOURCE=GATEWAY_AUTH_CONTEXT_USER_ID`

`PRESENTER_BOUND_TO_DECISION=REQUIRED_AND_UNAVAILABLE`

`DEVICE_IDENTITY_SOURCE=FIXED_IN_DECISION_FROM_RESOLVED_DRIVER`

`DEVICE_BOUND_TO_DECISION=YES`

`ATOMIC_CONSUMPTION_MODEL=SINGLE_FUNCTION_TRANSACTION_SPECIFIED_WRITE_BLOCKED`

`DECISION_REPLAY_POLICY=IDENTICAL_REPLAY_RETURNS_PRIOR_RESULT`

`EXECUTION_TARGET_AUTHORIZATION=BLOCKED_WHILE_OWNER_EPOCH_ABSENT`

`DRIVER_ONLY_REVISION_POLICY=REAUTHORIZATION_REQUIRED`

`SHIPMENT_VERSION_POLICY=REAUTHORIZATION_REQUIRED`

`TEST_MATRIX_EXTENDED=T16_T25_NOT_RUN`

`BINDING_WRITE=BLOCKED_UNTIL_TRUSTED_PRESENTER`

`TABLE_OWNER_DECISION_BYPASS=OPEN`

`DIRECT_BINDING_INSERT_BYPASS=OPEN`

`J_DML_001=NOT_CLOSED`

The decision table, the roles, and the consume function are specified and not created. Tests 16-25 were not executed. Shipment-owned binding authority is not implemented and is not approved for implementation.

## R3 — J-TBH-R2-001, J-TBH-R2-002

`J_TBH_R2_001=ADDRESSED`

`J_TBH_R2_002=ADDRESSED`

A future migration transfers `transport.binding_authority_decisions` to `binding_authority_owner` with `ALTER TABLE ... OWNER TO`. `REVOKE` does not replace that statement. The membership `freight_migrations` needs for the non-superuser transfer is revoked in the same migration. `binding_authority_fence` is not the table owner.

The fence receives column `SELECT` on the proposed decision row and `UPDATE` only of `consumed_at`, `use_status`, and `binding_id`. Assignment rows are read through `transport.read_binding_assignment`, not through table `SELECT` on shipments or executions. The status transition is `ISSUED` to `CONSUMED`. A trigger rejects the reverse while it is installed. The table owner and a superuser can still disable that trigger.

`PRESENTER_IDENTITY_BOUND=NOT_IMPLEMENTED`

`DECISION_CONSUMPTION_GATE=CLOSED`

`PHYSICAL_DEVICE_AUTHENTICATION=NO`

`OWNERSHIP_TRANSFER_SPECIFIED=YES`

`MIGRATION_ROLE_AFTER_TRANSFER=NOT_OWNER_NOT_MEMBER`

`FUNCTION_OWNER_PRIVILEGES=DECISION_COLUMN_SELECT_AND_CONSUME_UPDATE_ONLY`

`DECISION_SELECT_PATH=COLUMN_SELECT_FOR_BINDING_AUTHORITY_FENCE`

`ASSIGNMENT_SELECT_PATH=SHIPMENT_OWNED_READ_FUNCTION`

`BOUND_IDENTIFIERS_IMMUTABLE=TRIGGER_AND_NO_UPDATE_GRANT`

`ONE_WAY_CONSUMPTION=ISSUED_TO_CONSUMED_TRIGGER_SPECIFIED`

`CONSUMED_AT_RESET_DENIED_BY_DESIGN=TRIGGER_NOT_ABSOLUTE_AGAINST_OWNER`

`CONCURRENT_CONSUMPTION=SINGLE_TRANSACTION_ROW_LOCKS_SPECIFIED`

`ASSIGNMENT_ROW_LOCK=SPECIFIED_NOT_IMPLEMENTED`

`TEST_MATRIX_EXTENDED=T26_T37_NOT_RUN`

Tests 26-37 were not executed. The consume function stays ungranted. The current binding insert stays open.

```text
AGENT=C
STAGE=TRACKING-BINDING-AUTHORITY-DECISION-0.1H-R3
PR_NUMBER=379
PREVIOUS_HEAD=647153cba7e0344c8c44e59e2422dc981930c877
NEW_HEAD=
REMOTE_HEAD=
J_TBH_R2_001=ADDRESSED
J_TBH_R2_002=ADDRESSED
DECISION_TABLE_OWNER=binding_authority_owner
OWNERSHIP_TRANSFER_SPECIFIED=YES
MIGRATION_ROLE_AFTER_TRANSFER=NOT_OWNER_NOT_MEMBER
FUNCTION_OWNER_PRIVILEGES=DECISION_COLUMN_SELECT_AND_CONSUME_UPDATE_ONLY
DECISION_SELECT_PATH=COLUMN_SELECT_FOR_BINDING_AUTHORITY_FENCE
ASSIGNMENT_SELECT_PATH=SHIPMENT_OWNED_READ_FUNCTION
BOUND_IDENTIFIERS_IMMUTABLE=TRIGGER_AND_NO_UPDATE_GRANT
ONE_WAY_CONSUMPTION=ISSUED_TO_CONSUMED_TRIGGER_SPECIFIED
CONSUMED_AT_RESET_DENIED_BY_DESIGN=TRIGGER_NOT_ABSOLUTE_AGAINST_OWNER
CONCURRENT_CONSUMPTION=SINGLE_TRANSACTION_ROW_LOCKS_SPECIFIED
DECISION_REPLAY_POLICY=IDENTICAL_REPLAY_RETURNS_PRIOR_RESULT
TEST_MATRIX_EXTENDED=T26_T37_NOT_RUN
PRESENTER_IDENTITY_BOUND=NOT_IMPLEMENTED
DECISION_CONSUMPTION_GATE=CLOSED
PHYSICAL_DEVICE_AUTHENTICATION=NO
DIRECT_BINDING_INSERT_BYPASS=OPEN
J_DML_001=NOT_CLOSED
TENANT_ISOLATION_IMPLEMENTED=NO
ALL_WRITERS_FENCED=NO
BROKER_SAFETY_PROVEN=NO
IMPLEMENTATION_GATE=CLOSED
DOCS_ONLY=YES
PRODUCT_CODE_CHANGED=NO
MIGRATIONS_CREATED=NO
DB_ROLES_CHANGED=NO
STAGING_CHANGED=NO
PRODUCTION_CHANGED=NO
MERGE_PERFORMED=NO
CI_RUN=
CI_RESULT=RECORDED_ON_THE_PULL_REQUEST_AFTER_THIS_COMMIT
FINAL_VERDICT=READY_FOR_J_R3_REVIEW
```

The block above is the R3 snapshot. R4 supersedes the shipment-before-execution lock order.

## R4 — J-TBH-R3-001, J-TBH-R3-002

`J_TBH_R3_001=ADDRESSED`

`J_TBH_R3_002=ADDRESSED`

A driver command locks `transport.transport_executions` before `loadShipment` locks `transport.shipments`. Consume for an execution-scoped decision uses that same order, then the decision, then the binding insert. A driver-only decision does not lock an execution after the shipment. `SQLSTATE 40P01` rolls the consume transaction back.

Assignment rows are locked with `FOR SHARE`. R4 specified column `SELECT` and no `UPDATE` grant for that mode. Isolated PostgreSQL 16.15 rejected that grant with `42501`. The 0.1I-R1 section withdraws it. The one-way trigger is created while `SET ROLE binding_authority_owner` is still possible, and the membership is revoked after that statement, in the same uncommitted migration.

`COMMAND_WRITER_LOCK_ORDER=EXECUTION_THEN_SHIPMENT`

`CONSUME_LOCK_ORDER=EXECUTION_THEN_SHIPMENT_THEN_DECISION_THEN_BINDING`

`DRIVER_ONLY_LOCK_ORDER=SHIPMENT_THEN_DECISION_THEN_BINDING`

`ALL_RELEVANT_WRITERS_CHECKED=YES`

`DEADLOCK_COUNTEREXAMPLE_RESOLVED=YES`

`ASSIGNMENT_LOCK_MODE=FOR_SHARE`

`ASSIGNMENT_FUNCTION_OWNER=shipment_assignment_reader`

`ASSIGNMENT_LOCK_PRIVILEGES=COLUMN_SELECT_FOR_SHARE_TEST_REQUIRED`

`MINIMUM_UPDATE_COLUMN=NONE_ON_ASSIGNMENT_TABLES`

`SECURITY_DEFINER_MODEL=FIXED_SEARCH_PATH_QUALIFIED_NAMES_NO_DYNAMIC_SQL`

`TRIGGER_INSTALL_ORDER=SET_ROLE_OWNER_THEN_FUNCTION_THEN_TRIGGER_THEN_REVOKE`

`FUNCTION_OWNER_TRANSFER=CREATE_WHILE_SET_ROLE_BEFORE_REVOKE`

`TABLE_OWNER_TRANSFER=ALTER_OWNER_BEFORE_TRIGGER`

`TEMPORARY_MEMBERSHIP_REVOKED=AFTER_TRIGGER_SAME_TRANSACTION`

`MIGRATION_PRIVILEGES_VALID=SPECIFIED_FOR_EACH_DDL_NOT_EXECUTED`

`ASSIGNMENT_LOCK_PRIVILEGE_TEST=REQUIRED_NOT_RUN`

`TEST_MATRIX_EXTENDED=T38_T49_NOT_RUN`

`PRESENTER_IDENTITY_BOUND=NOT_IMPLEMENTED`

`DECISION_CONSUMPTION_GATE=CLOSED`

`DIRECT_BINDING_INSERT_BYPASS=OPEN`

Tests 38-49 were not executed. The consume function stays ungranted. The current binding insert stays open.

```text
AGENT=C
STAGE=TRACKING-BINDING-AUTHORITY-DECISION-0.1H-R4
PR_NUMBER=379
PREVIOUS_HEAD=600d8734453ac248394034ab8530d90868596a44
NEW_HEAD=
REMOTE_HEAD=
J_TBH_R3_001=ADDRESSED
J_TBH_R3_002=ADDRESSED
COMMAND_WRITER_LOCK_ORDER=EXECUTION_THEN_SHIPMENT
CONSUME_LOCK_ORDER=EXECUTION_THEN_SHIPMENT_THEN_DECISION_THEN_BINDING
DRIVER_ONLY_LOCK_ORDER=SHIPMENT_THEN_DECISION_THEN_BINDING
ALL_RELEVANT_WRITERS_CHECKED=YES
DEADLOCK_COUNTEREXAMPLE_RESOLVED=YES
ASSIGNMENT_LOCK_MODE=FOR_SHARE
ASSIGNMENT_FUNCTION_OWNER=shipment_assignment_reader
ASSIGNMENT_LOCK_PRIVILEGES=COLUMN_SELECT_FOR_SHARE_TEST_REQUIRED
MINIMUM_UPDATE_COLUMN=NONE_ON_ASSIGNMENT_TABLES
SECURITY_DEFINER_MODEL=FIXED_SEARCH_PATH_QUALIFIED_NAMES_NO_DYNAMIC_SQL
TRIGGER_INSTALL_ORDER=SET_ROLE_OWNER_THEN_FUNCTION_THEN_TRIGGER_THEN_REVOKE
FUNCTION_OWNER_TRANSFER=CREATE_WHILE_SET_ROLE_BEFORE_REVOKE
TABLE_OWNER_TRANSFER=ALTER_OWNER_BEFORE_TRIGGER
TEMPORARY_MEMBERSHIP_REVOKED=AFTER_TRIGGER_SAME_TRANSACTION
MIGRATION_PRIVILEGES_VALID=SPECIFIED_FOR_EACH_DDL_NOT_EXECUTED
TEST_MATRIX_EXTENDED=T38_T49_NOT_RUN
PRESENTER_IDENTITY_BOUND=NOT_IMPLEMENTED
DECISION_CONSUMPTION_GATE=CLOSED
DIRECT_BINDING_INSERT_BYPASS=OPEN
J_DML_001=NOT_CLOSED
TENANT_ISOLATION_IMPLEMENTED=NO
ALL_WRITERS_FENCED=NO
BROKER_SAFETY_PROVEN=NO
IMPLEMENTATION_GATE=CLOSED
DOCS_ONLY=YES
PRODUCT_CODE_CHANGED=NO
SQL_CHANGED=NO
MIGRATIONS_CREATED=NO
DB_ROLES_CHANGED=NO
STAGING_CHANGED=NO
PRODUCTION_CHANGED=NO
MERGE_PERFORMED=NO
CI_RUN=
CI_RESULT=RECORDED_ON_THE_PULL_REQUEST_AFTER_THIS_COMMIT
FINAL_VERDICT=READY_FOR_J_R4_REVIEW
```

The R4 block above is the lock-order snapshot. Its tokens `ASSIGNMENT_FUNCTION_OWNER=shipment_assignment_reader`, `ASSIGNMENT_LOCK_PRIVILEGES=COLUMN_SELECT_FOR_SHARE_TEST_REQUIRED`, and `MINIMUM_UPDATE_COLUMN=NONE_ON_ASSIGNMENT_TABLES` are withdrawn. The 0.1I-R1 section is the privilege decision. The 0.1I-R2 section after it is the reproducible evidence for that decision. The 0.1I-R3 section records the manifest and assertion remediation. The snapshot after 0.1I-R3 is 0.1H-R2.

## 0.1I-R1 — PostgreSQL 16.15 privilege evidence

`FOR_SHARE_COLUMN_SELECT=FAIL_42501` on the R4 grant. Column `SELECT` without `UPDATE`, and table `SELECT` without `UPDATE`, both returned `42501` for `SELECT ... FOR SHARE`. `UPDATE (updated_at)` made `FOR SHARE` and `FOR UPDATE` succeed, and it let the grantee change `updated_at`. The lock mode stays `FOR SHARE`. The privilege set is column `SELECT` plus `UPDATE (updated_at)` on each locked table. Those are separate decisions.

The selected owner is `assignment_lock_owner`. It is `NOLOGIN`. The fence has `EXECUTE` only. Runtime, fence, `freight`, and `PUBLIC` have no membership. `NOLOGIN` is not the control. `INHERIT TRUE` and `SET TRUE` were each enough to change `updated_at`, and both grants were revoked. Option A would put that write on `shipment_assignment_reader`. Option C would use `shipment_runtime`, which already updates the shipment tables, and an HTTP call is not the consume transaction. Option B is the contract.

`ASSIGNMENT_LOCK_PRIVILEGE_PROVEN=ISOLATED_PASS_NOT_PRODUCTION`. The isolated function locked the shipment and returned `current_user=assignment_lock_owner`. The full design `RETURN QUERY` was not executed. The production schema was not used. `DIRECT_UPDATED_AT_RISK=OPEN_FOR_LOCK_OWNER`. Table-owner `DISABLE TRIGGER` was not run. `TABLE_OWNER_DECISION_BYPASS=OPEN`.

The 0.1I rollback recorder compared the whole `psql -tAc` stdout with `ISSUED|null`. That stdout was `BEGIN`, `UPDATE 1`, `ROLLBACK`, and `ISSUED|null`. The row in the database was `use_status=ISSUED` and `consumed_at IS NULL`. 0.1I-R1 reproduced the stdout mismatch and passed the separate field predicate. The original `P1`/`P1c` failure and the `P4` invalid identity call stay in the test-plan evidence table. `P6c` was OK only while the function owner was the superuser.

Remaining bypasses: the lock owner, the table owner, and a superuser can write `updated_at`; a future membership reopens that write; the owner can replace the function body; the decision-table owner can disable the one-way trigger; tenant isolation is not implemented; the current binding insert remains open; presenter identity is not bound; not every writer is fenced; broker safety is not proven.

A later isolated run, before implementation, executes the full `transport.read_binding_assignment` body, checks `42501` for `UPDATE` of `driver_id`, `version`, and `updated_at` from the fence and the runtime, checks `42501` for `SET ROLE assignment_lock_owner` from those roles, and checks that `FOR SHARE` still waits on an assignment `UPDATE`. `ASSIGNMENT_LOCK_PRIVILEGE_PROVEN` stays `ISOLATED_PASS_NOT_PRODUCTION` until that body has been run. It is not a production pass.

```text
AGENT=C
STAGE=TRACKING-BINDING-POSTGRES-PRIVILEGE-PROOF-0.1I-R1
REFERENCE_PR=379
PREVIOUS_HEAD=27ef787b909de9ddef0f9a583c430af926561c49
NEW_HEAD=
POSTGRES_VERSION=16.15
ORIGINAL_TESTS_PASSED=30
ORIGINAL_TESTS_FAILED=2
ORIGINAL_TESTS_NOT_RUN=1
FOR_SHARE_REQUIRES_UPDATE=YES
COLUMN_SELECT_ONLY_VALID=NO
MINIMUM_UPDATE_COLUMN=updated_at
RECOMMENDED_PRIVILEGE_MODEL=B_SEPARATE_LOCK_FUNCTION_OWNER
PRIVILEGE_MODEL_RETESTED=YES
DIRECT_UPDATED_AT_RISK=OPEN_FOR_LOCK_OWNER
ROLLBACK_DB_STATE=CORRECT
ROLLBACK_HARNESS_FALSE_NEGATIVE=YES
SECURITY_DEFINER_INITIAL_TEST_INVALID=YES
CONTRACT_UPDATED=YES
ASSIGNMENT_LOCK_MODE=FOR_SHARE
ASSIGNMENT_FUNCTION_OWNER=assignment_lock_owner
ASSIGNMENT_LOCK_PRIVILEGES=COLUMN_SELECT_PLUS_UPDATE_UPDATED_AT
ASSIGNMENT_LOCK_PRIVILEGE_PROVEN=ISOLATED_PASS_NOT_PRODUCTION
CI_RUN=
CI_RESULT=RECORDED_ON_THE_PULL_REQUEST_AFTER_THIS_COMMIT
PRESENTER_IDENTITY_BOUND=NOT_IMPLEMENTED
DECISION_CONSUMPTION_GATE=CLOSED
J_DML_001=NOT_CLOSED
TENANT_ISOLATION_IMPLEMENTED=NO
ALL_WRITERS_FENCED=NO
BROKER_SAFETY_PROVEN=NO
IMPLEMENTATION_GATE=CLOSED
STAGING_CHANGED=NO
PRODUCTION_CHANGED=NO
PLATFORM_MIGRATIONS_CHANGED=NO
MERGE_PERFORMED=NO
PRODUCT_CODE_CHANGED=NO
FINAL_VERDICT=READY_FOR_J_REVIEW
```

## 0.1I-R2 — J-PGP-001

`J_PGP_001=ADDRESSED`

The reproducible pack is `docs/architecture/tracking/evidence/0_1i_r2/`. `ORIGINAL_TRANSCRIPT=NOT_AVAILABLE`: the 0.1I and 0.1I-R1 containers were already removed, so their stdout was not copied. The script files that remained on disk are under `prior_scripts/` and were copied unchanged. They are not reconstructed transcripts.

`NEW_RUN_EVIDENCE=runs/attempt-3`. That run used a fresh local container, image `postgres:16`, network `none`, no published ports, database `binding_privilege_proof_0_1i_r2`. `SELECT version()` returned PostgreSQL 16.15. `server_version_num` was `160015`. The client was psql 16.15. `runs/attempt-1/` and `runs/attempt-2/` are the earlier executions in this stage and are kept, including their failed assertions.

Attempt 3 recorded 37 passing assertions and no failures. Column `SELECT` and table `SELECT` each returned `42501` for `FOR SHARE`. After `UPDATE (updated_at)`, `FOR SHARE` returned the shipment id, the lock owner changed `updated_at`, and `UPDATE` of `driver_id` returned `42501`. Session B waited with `wait_event_type=Lock` and `wait_event=transactionid`, then completed `UPDATE 1` after session A committed. Fence execution returned `assignment_lock_owner|fence_probe|pg_catalog|33333333-3333-3333-3333-333333333333`. After rollback, `use_status` was `ISSUED`, `consumed_at IS NULL` was printed as `t`, and the state predicate returned `PASS`. The full design `RETURN QUERY` was not executed. `FULL_FUNCTION_TEST=NOT_RUN`.

```text
AGENT=C
STAGE=TRACKING-BINDING-POSTGRES-PRIVILEGE-PROOF-0.1I-R2
PR_NUMBER=379
PREVIOUS_HEAD=ab877b355fa99db4faedda9b390887db8cddb4a1
NEW_HEAD=
REMOTE_HEAD=
J_PGP_001=ADDRESSED
POSTGRES_VERSION=16.15
ORIGINAL_TRANSCRIPT=NOT_AVAILABLE
NEW_RUN_EVIDENCE=runs/attempt-3
ISOLATED_ENVIRONMENT=LOCAL_DOCKER_NETWORK_NONE
EVIDENCE_DIRECTORY=docs/architecture/tracking/evidence/0_1i_r2/
SQL_FILES_PRESENT=YES
RAW_STDOUT_PRESENT=YES
RAW_STDERR_PRESENT=YES
SQLSTATE_CAPTURED=YES
SHA256_MANIFEST=docs/architecture/tracking/evidence/0_1i_r2/SHA256SUMS
E01_VERSION=16.15
E02_COLUMN_SELECT=42501
E03_TABLE_SELECT=42501
E04_UPDATE_GRANT=PASS
E05_LOCK_CONFLICT=PASS
E06_SECURITY_DEFINER=PASS
E07_ROLLBACK=PASS
E08_ROLE_MEMBERSHIP=PASS
E09_PRIVILEGE_BOUNDARIES=PASS
TESTS_PASSED=37
TESTS_FAILED=0
TESTS_NOT_RUN=1
HARNESS_FALSE_NEGATIVE_FIXED=YES
FULL_FUNCTION_TEST=NOT_RUN
DOCS_ONLY_SCOPE=YES
PRODUCT_CODE_CHANGED=NO
PLATFORM_MIGRATIONS_CHANGED=NO
DB_ROLES_CHANGED_IN_REAL_ENV=NO
STAGING_CHANGED=NO
PRODUCTION_CHANGED=NO
MERGE_PERFORMED=NO
PRESENTER_IDENTITY_BOUND=NOT_IMPLEMENTED
DECISION_CONSUMPTION_GATE=CLOSED
J_DML_001=NOT_CLOSED
TENANT_ISOLATION_IMPLEMENTED=NO
ALL_WRITERS_FENCED=NO
BROKER_SAFETY_PROVEN=NO
IMPLEMENTATION_GATE=CLOSED
CI_RUN=
CI_RESULT=RECORDED_ON_THE_PULL_REQUEST_AFTER_THIS_COMMIT
FINAL_VERDICT=READY_FOR_J_R2_REVIEW
```

## 0.1I-R3 — J-PGP-R2-001 and J-PGP-R2-002

`J_PGP_R2_001=ADDRESSED`

`J_PGP_R2_002=ADDRESSED`

`SHA256SUMS` lists 344 paths. The previous 335 evidence paths are all still listed, and the manifest is not one of them. The nine added paths are under `harness_self_tests/`. Each digest is the SHA-256 of the Git blob (`git cat-file blob`), not the bytes of a checkout that changed line endings. All 344 digests matched those blobs. Five blobs changed: `README.md`, `run_all.sh`, `privilege_tests.sql`, `security_definer_tests.sql`, and `rollback_tests.sql`. The blobs for `runs/attempt-1/`, `runs/attempt-2/`, `runs/attempt-3/`, and `prior_scripts/` are the same objects as `c69365b6d3e8f21b009a8eeddb7046a4f351bfd8`.

Eleven files in this Windows worktree still have CRLF from the earlier `core.autocrlf` checkout. Their worktree hashes differ from the Git blobs. The manifest uses the blob bytes. `docs/architecture/tracking/evidence/.gitattributes` sets `-text` for that evidence tree only. `git check-attr text` reports `unset` for `runs/attempt-3/E04_updated_at_changed.stdout` and for the new fixtures, and `unspecified` for this report. A second operating-system checkout was not run.

`run_all.sh` compares one normalized field with the whole `exact=` value. `exact=UPDATE 1` is the whole command tag. The self-test in `harness_self_tests/` returned `self_test_failures=0`: `CHANGED` passed; `UNCHANGED`, empty stdout, two data lines, SQLSTATE `42P01` where `42501` was expected, and a non-zero psql status where OK was expected all failed. `grep -F CHANGED` matched `UNCHANGED` and was recorded as `WOULD_MATCH`. It is not a passing assertion. PostgreSQL was not started. Attempt 3 remains 37 pass and 0 fail.

```text
AGENT=C
STAGE=TRACKING-BINDING-POSTGRES-PRIVILEGE-PROOF-0.1I-R3
PR_NUMBER=379
PREVIOUS_HEAD=c69365b6d3e8f21b009a8eeddb7046a4f351bfd8
NEW_HEAD=
REMOTE_HEAD=
J_PGP_R2_001=ADDRESSED
J_PGP_R2_002=ADDRESSED
MANIFEST_ENTRIES=344
GIT_BLOB_HASHES_VERIFIED=344_OF_344
CROSS_PLATFORM_VERIFICATION=BLOB_BYTES_MATCH;TEXT_UNSET;LIVE_LINUX_CHECKOUT=NOT_RUN
HISTORICAL_TRANSCRIPTS_UNCHANGED=YES
EXACT_ASSERTION_IMPLEMENTED=YES
SUBSTRING_FALSE_PASS_REJECTED=YES
HARNESS_SELF_TESTS=PASS
SQLSTATE_HANDLING=SEPARATE_FROM_STDOUT
ORIGINAL_ATTEMPT_3=37_PASS_0_FAIL
FULL_FUNCTION_TEST=NOT_RUN
PRODUCT_CODE_CHANGED=NO
PLATFORM_MIGRATIONS_CHANGED=NO
STAGING_CHANGED=NO
PRODUCTION_CHANGED=NO
MERGE_PERFORMED=NO
IMPLEMENTATION_GATE=CLOSED
DECISION_CONSUMPTION_GATE=CLOSED
BROKER_SAFETY_PROVEN=NO
CI_RUN=
CI_RESULT=RECORDED_ON_THE_PULL_REQUEST_AFTER_THIS_COMMIT
FINAL_VERDICT=READY_FOR_J_R3_REVIEW
```

The block below is the 0.1H-R2 snapshot.

```text
AGENT=C
STAGE=TRACKING-BINDING-AUTHORITY-DECISION-0.1H-R2
PR_NUMBER=379
PREVIOUS_HEAD=7596efbdf104594bb79c8b62b4e8d63e979cd3b6
NEW_HEAD=
REMOTE_HEAD=
J_TBH_001=ADDRESSED
J_TBH_002=ADDRESSED
J_TBH_003=ADDRESSED
DECISION_TABLE_SPECIFIED=transport.binding_authority_decisions
DECISION_TABLE_OWNER=binding_authority_owner
DECISION_GRANTS=OWNER_INSERT_VIA_ISSUE_FUNCTION_FENCE_UPDATE_CONSUMED_AT_ONLY
PRESENTER_IDENTITY_SOURCE=GATEWAY_AUTH_CONTEXT_USER_ID
PRESENTER_BOUND_TO_DECISION=REQUIRED_AND_UNAVAILABLE
DEVICE_IDENTITY_SOURCE=FIXED_IN_DECISION_FROM_RESOLVED_DRIVER
DEVICE_BOUND_TO_DECISION=YES
ATOMIC_CONSUMPTION_MODEL=SINGLE_FUNCTION_TRANSACTION_SPECIFIED_WRITE_BLOCKED
DECISION_REPLAY_POLICY=IDENTICAL_REPLAY_RETURNS_PRIOR_RESULT
EXECUTION_TARGET_AUTHORIZATION=BLOCKED_WHILE_OWNER_EPOCH_ABSENT
DRIVER_ONLY_REVISION_POLICY=REAUTHORIZATION_REQUIRED
SHIPMENT_VERSION_POLICY=REAUTHORIZATION_REQUIRED
TEST_MATRIX_EXTENDED=T16_T25_NOT_RUN
DIRECT_BINDING_INSERT_BYPASS=OPEN
J_DML_001=NOT_CLOSED
TENANT_ISOLATION_IMPLEMENTED=NO
ALL_WRITERS_FENCED=NO
BROKER_SAFETY_PROVEN=NO
IMPLEMENTATION_GATE=CLOSED
DOCS_ONLY=YES
PRODUCT_CODE_CHANGED=NO
MIGRATIONS_CREATED=NO
DB_ROLES_CHANGED=NO
STAGING_CHANGED=NO
PRODUCTION_CHANGED=NO
MERGE_PERFORMED=NO
CI_RUN=
CI_RESULT=RECORDED_ON_THE_PULL_REQUEST_AFTER_THIS_COMMIT
FINAL_VERDICT=READY_FOR_J_R2_REVIEW
```
