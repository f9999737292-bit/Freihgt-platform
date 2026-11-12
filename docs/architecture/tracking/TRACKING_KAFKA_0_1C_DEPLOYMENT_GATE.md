# Tracking Kafka 0.1C — deployment gate

Status: specified and closed.
`MIXED_CONSUMER_DEPLOYMENT_GATE=CLOSED`.
`DEPLOYMENT_RUNBOOK=SPECIFIED_NOT_AUTHORIZED`.

This stage does not stop a consumer, join a group, read a staging or production broker, or rewrite an offset. The current tracking client does not take the advisory lock or the single-flight commit rule from PR #357. A new binary does not repair an offset the current binary has already committed. Two versions in one group are not compatible.

`Client.GroupMetadata` in franz-go v1.21.5 returns a member id and a generation. It does not return the group roster or a contract version. The default client id is `tracking-service` (`services/tracking-service/internal/config/config.go`). That string is not a contract version.

## 1. Version identity

A later implementation needs a consumer-contract version that every member presents in coordinator-visible membership. The version value for the contract in PR #357 plus this 0.1C foundation is `tracking-execution-stop-contract-0.1c`. Advertising it is future work. Until a membership sample shows that field, the gate treats the member as unidentified.

Unidentified, missing, and any other version are incompatible with `0.1c`.

## 2. Ordered gate

The new mode stays off until every step below has a recorded result. A failed step stops the start. The process does not join the group in the new mode while a step is incomplete.

1. Stop the old consumers. The old process does not use the new advisory lock and can still commit a nil `Apply`, invalid JSON, or an unknown type. Stopping it is a runtime action.
2. Read group membership for `tracking-execution-stop`, including every member, not only the local `GroupMetadata` pair. The sample must show no old member and no unidentified member before the new mode is allowed.
3. Read the tracking inbox for historical conflicts. The unique key means a committed pair `(execution_id, event_sequence)` has one `event_id`. The scan looks for null proof columns, sequence values on state rows that no `last_event_id` explains, and offsets that moved without a terminal proof. Issue #344's rejected event is absent from the inbox. The scan cannot clear that conflict. An unresolved row keeps the gate closed.
4. Read committed offsets with `OffsetFetch` for the group and topic. Compare them with the inbox and the state proof. An offset ahead of the last proven terminal record keeps the gate closed. The new consumer does not move that offset backward.
5. Confirm every active member advertises `tracking-execution-stop-contract-0.1c` and none advertises an older contract.
6. Start the new consumer only after steps 1–5 are true and the controller has authorized the start. The first fetch begins at the stored offset. The consumer commits only a terminal result defined by the persistence document.
7. If a later membership sample contains an old, unidentified, or different version, the new member stops, leaves the group, and alerts. It does not keep consuming beside that member.
8. Recovery is a separate plan. It does not run automatically. It does not rewrite offsets, delete inbox rows, or backfill hashes. Historical conflicts stay `NON_TERMINAL` until a controller authorizes a specific repair.

## 3. Rollback

Rollback of a start is: stop the new members, confirm the membership sample no longer contains them, and leave the committed offsets where `OffsetFetch` shows them. Starting the old consumer again is its own authorization. The old consumer can commit records the new contract classifies as `NON_TERMINAL`. Returning to it is not a silent rollback.

A database down migration follows `TRACKING_KAFKA_0_1C_PERSISTENCE.md`. It is blocked once any proof column is non-null.

## 4. Actions that need a separate controller authorization

- stopping or starting any consumer process;
- `DescribeGroups`, `OffsetFetch`, or any other broker read against staging or production;
- starting the isolated broker plan in `TRACKING_KAFKA_0_1C_BROKER_TEST_PLAN.md`;
- enabling the new mode in any deployed environment;
- rewriting a committed offset, deleting or updating inbox rows, or backfilling proof columns;
- opening the mixed-version gate;
- deploying, or changing staging and production.

Local documentation of this gate does not perform those actions.

## 5. Acceptance

- `OLD_CONSUMER_USES_NEW_LOCK=NO`.
- `NEW_CONSUMER_REPAIRS_OLD_OFFSET=NO`.
- `MIXED_GROUP_COMPATIBLE=NO`.
- `GATE=CLOSED`.
- `ROLLOUT_PERFORMED=NO`.
- `CONTROLLER_AUTHORIZATION_REQUIRED_FOR_RUNTIME=YES`.
