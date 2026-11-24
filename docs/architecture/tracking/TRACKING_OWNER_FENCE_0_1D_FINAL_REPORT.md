# Tracking owner fence 0.1D — final report

```text
AGENT=C
STAGE=TRACKING-OWNER-FENCE-DECISION-0.1D
BASE_SHA=3f306fc407aeda4bdc652d825954292f4a54fad6
REFERENCE_PR_374=89b509797ae61558582f4c198647ba13d21d9a47
REFERENCE_PR_357=0cf21055dd2a91c5ef071d6982c1fcc70cd11670
MIGRATION_HEAD=000105_platform_postgres_role_foundation_v0_2_i1
ALL_WRITERS_INVENTORIED=YES
NOT_VERIFIED=OPERATOR_SQL_AND_DATABASE_URL_OUTSIDE_THIS_REPOSITORY
SAME_DATABASE_FENCE_DESIGNED=YES
LOCK_ORDER_DEFINED=YES
OWNER_EPOCH_LIFECYCLE_DEFINED=YES
DB_PRIVILEGE_MODEL=SECURITY_DEFINER_LOCK_FUNCTION
SECURITY_DEFINER_EVALUATED=YES
ETA_WRITER_COVERED=YES
MARK_APPROACH_WRITER_COVERED=YES
OLD_CONSUMER_RISK_DOCUMENTED=YES
CROSS_TENANT_TESTS_DEFINED=YES
CRASH_RECOVERY_TESTS_DEFINED=YES
SHIPMENT_OWNER_RACE=OPEN_UNTIL_IMPLEMENTED
BROKER_SAFETY_PROVEN=NO
MIXED_CONSUMER_GATE=CLOSED
DOCS_ONLY=YES
PRODUCT_CODE_CHANGED=NO
MIGRATIONS_CREATED=NO
STAGING_CHANGED=NO
PRODUCTION_CHANGED=NO
PR_374_CHANGED=NO
PR_357_CHANGED=NO
MERGE_PERFORMED=NO
IMPLEMENTATION_GATE=CLOSED
RECOMMENDATION_APPROVED_FOR_IMPLEMENTATION=NO
CI_RESULT=RECORDED_ON_THE_PULL_REQUEST_AFTER_THIS_COMMIT
FINAL_VERDICT=READY_FOR_J_REVIEW
```

## Decision

The proposed fence is one tracking transaction that calls `transport.lock_execution_owner` before it locks or writes tracking state. The inbox insert precedes the state effect that stores `last_event_id`. The function is `SECURITY DEFINER`, uses a fixed statement, and holds the execution-row lock until the caller commits. `tracking_runtime` is specified to receive schema `USAGE` and `EXECUTE` on that function only. `tracking_service_login` inherits those two privileges and receives no direct table DML. A column-level `UPDATE` grant and `SHIPMENT_SINGLE_WRITER` were compared and not recommended. Neither recommendation is approved for implementation.

The linearization point is the commit of the transaction that holds the execution row and stores `owner_epoch`. An effect committed at epoch 4 stays after a later commit of epoch 5. An epoch-4 write does not commit after that later commit. The current HTTP read before `Begin` does not provide this order.

## Future PR #357 contract changes

PR #357 is not edited. A later revision of that contract, before any consumer implementation, needs all of the following:

- `owner_epoch` is mandatory for a new terminal effect.
- The owner check is the row returned by `transport.lock_execution_owner` in the same transaction as the state write.
- The durable proof stores `event_id`, SHA-256 of the raw record bytes, payload size, tenant, sequence, and epoch.
- `tracking.execution_tracking_state.last_event_id` equals that `event_id` before the effect is terminal.
- `TERMINAL` remains a committed state effect or a verified prior effect with that proof. Invalid JSON, an unknown type, a missing epoch, and a tenant or epoch mismatch stay `NON_TERMINAL`.
- A mixed group stays closed. The old consumer does not call the lock function.
- The committed offset moves only across a contiguous terminal prefix.
- An unknown `CommitRecords` result does not send a second commit. The consumer reads `OffsetFetch` and does not start the next offset.

## Documents

- `docs/architecture/tracking/TRACKING_OWNER_FENCE_0_1D_WRITER_INVENTORY.md`
- `docs/architecture/tracking/TRACKING_OWNER_FENCE_0_1D_TRANSACTION_CONTRACT.md`
- `docs/architecture/tracking/TRACKING_OWNER_FENCE_0_1D_SECURITY.md`
- `docs/architecture/tracking/TRACKING_OWNER_FENCE_0_1D_EPOCH_LIFECYCLE.md`
- `docs/architecture/tracking/TRACKING_OWNER_FENCE_0_1D_TEST_PLAN.md`

Implementation, migrations, broker tests, and deployment remain stopped.

## R2

`J_TOF_001=CLOSED`. The caller check uses `session_user` membership and inheritance. `current_user` inside `SECURITY DEFINER` is the function owner and is not the caller check. `EXECUTE` is granted only to `tracking_runtime`. `freight` does not receive it.
`J_TOF_002=CLOSED`. `tracking_execution_fence` is `NOLOGIN`, `NOSUPERUSER`, and `NOBYPASSRLS`. Its only `UPDATE` column is `updated_at`. It has no `UPDATE` on `operating_tenant_id` or `owner_epoch`. The privilege test was not executed.
`J_TOF_003=CLOSED`. `MarkApproachEmitted` has a separate epoch oracle and a separate tenant oracle. A pre-lock read is not the authoritative epoch.
`DIRECT_DML_ENFORCEMENT_CHOSEN=NO`. `ALL_WRITERS_FENCED=NO`.
`OPEN_RISKS=SHARED_LOGIN_FREIGHT,TABLE_OWNER_AND_SUPERUSER_BYPASS,DIRECT_STATE_UPDATE_WITHOUT_EPOCH_CHECK,COLUMN_UPDATE_SUFFICIENCY_NOT_EXECUTED,LIVE_TABLE_OWNER_NOT_QUERIED,RLS_ABSENT_AND_NO_POLICY_DESIGNED,SHIPMENT_RUNTIME_HAS_TABLE_UPDATE,OPERATOR_SQL_NOT_VERIFIED,TENANT_ISOLATION_NOT_IMPLEMENTED,EPOCH_MONOTONIC_TRIGGER_NOT_CREATED,OLD_CONSUMER_CAN_BE_STARTED_AGAIN`.
`SHIPMENT_OWNER_RACE=OPEN_UNTIL_IMPLEMENTED`. `BROKER_SAFETY_PROVEN=NO`. `MIXED_CONSUMER_GATE=CLOSED`.

## R3

The R2 sentence that treated `SET ROLE` as making `pg_has_role(..., 'USAGE')` true is withdrawn. `USAGE` means the privileges are available without `SET ROLE`. `MEMBER` is a different question. The acceptance predicate is `pg_has_role(session_user, 'tracking_runtime', 'USAGE')`. `current_user` is not read inside the function.
`J_TOF_R2_001=ADDRESSED`. `tracking_runtime` is specified to receive `USAGE ON SCHEMA transport` and `EXECUTE` only on `transport.lock_execution_owner(uuid)`. `tracking_service_login` inherits that pair and has no direct table DML. `PUBLIC_FUNCTION_EXECUTE_AUDIT=REQUIRED`.
`J_TOF_R2_002=ADDRESSED`. `SESSION_USER_SEMANTICS=CORRECT`. `MEMBER_USAGE_DIFFERENCE=DOCUMENTED`. `SET_ROLE_BYPASS=NOT_CLAIMED_SAFE_WITHOUT_TEST`.
`J_TOF_R2_003=ADDRESSED`. The order is `EXECUTION_LOCK → OWNER_EPOCH_CHECK → INBOX_INSERT → STATE_EFFECT_AND_PROOF → COMMIT`. `INBOX_BEFORE_STATE_PROOF=YES`. `SAME_TRANSACTION=REQUIRED`. `FAILED_STATE_UPDATE_ROLLBACKS_INBOX=YES`. `OFFSET_COMMIT_AFTER_DURABLE_EFFECT=YES`. Hash equality alone is not terminal.
`DIRECT_DML_BYPASS=OPEN`. `SHARED_FREIGHT_RISK=OPEN`. `OWNER_EPOCH_MONOTONICITY=SPECIFIED_NOT_ENFORCED`. `ALL_WRITERS_FENCED=NO`.
`SHIPMENT_OWNER_RACE=OPEN_UNTIL_IMPLEMENTED`. `BROKER_SAFETY_PROVEN=NO`. `MIXED_CONSUMER_GATE=CLOSED`.
`CI_RESULT=RECORDED_ON_THE_PULL_REQUEST_AFTER_THIS_COMMIT`.
