# Tracking Kafka 0.1C — final report

```text
AGENT=C
STAGE=TRACKING-KAFKA-SAFETY-FOUNDATION-0.1C
BASE_SHA=3f306fc407aeda4bdc652d825954292f4a54fad6
PR357_HEAD=0cf21055dd2a91c5ef071d6982c1fcc70cd11670
PR346_HEAD=9b480f17118985c8b4791c66039b2a0525afdb98
PR372_HEAD=c5410bd107e396bff8893e8a14f484598aaafd31
ISSUE_344=OPEN
OWNER_FENCE_DESIGNED=YES
OWNER_FENCE_SELECTED=OWNER_EPOCH_FENCE
SHIPMENT_OWNER_RACE_RESOLVED_BY_DESIGN=NO
SHIPMENT_OWNER_RACE=OPEN
PERSISTENCE_MODEL_DESIGNED=YES
LAST_EVENT_ID_SPECIFIED=YES
OWNER_EPOCH_SPECIFIED=YES
HASH_AND_SIZE_SPECIFIED=YES
HISTORICAL_REPLAY_POLICY=NON_TERMINAL_NO_BACKFILL
BROKER_TEST_PLAN=SPECIFIED_NOT_RUN
BROKER_SAFETY_PROVEN=NO
NON_STOP_ALLOWLIST=EMPTY
MIXED_CONSUMER_GATE=CLOSED
DEPLOYMENT_RUNBOOK=SPECIFIED_NOT_AUTHORIZED
OPEN_RISKS=SHIPMENT_OWNER_READ_COMMIT_WINDOW,BROKER_APPLY_TIME,EMPTY_ALLOWLIST,HISTORICAL_ROWS_WITHOUT_PROOF,MIXED_GROUP
BLOCKERS=NO_SHARED_TRANSACTION,NO_EPOCH_COLUMN,NO_LAST_EVENT_ID_COLUMN,BROKER_TESTS_NOT_RUN,DEPLOYMENT_NOT_AUTHORIZED
DOCS_ONLY=YES
PRODUCT_CODE_CHANGED=NO
MIGRATIONS_CREATED=NO
STAGING_CHANGED=NO
PRODUCTION_CHANGED=NO
PR_357_CHANGED=NO
PR_346_CHANGED=NO
PR_372_CHANGED=NO
MERGE_PERFORMED=NO
IMPLEMENTATION_GATE=CLOSED
PRODUCTION_CONSUMER_READINESS=BLOCKED
CI_RESULT=RECORDED_ON_THE_PULL_REQUEST_AFTER_THIS_COMMIT
FINAL_VERDICT=READY_FOR_J_REVIEW
```

## Decisions

The owner contract is `OWNER_EPOCH_FENCE`. A second shipment lookup and a tracking-only advisory lock are not atomic with shipment-service. The read-versus-commit window stays open, so the race is not resolved. Serialized handoff with a receiver acknowledgement is not selected: that protocol does not exist, and a lost acknowledgement is a split owner.

Persistence adds nullable `payload_sha256`, `payload_size`, `last_event_id`, and `owner_epoch` in a later migration. Existing unique keys stay. Historical rows are not backfilled and stay `NON_TERMINAL`. Migration head `000105` is unchanged. The next migration number is not reserved.

The broker plan separates delivery time, response time, a later `OffsetFetch`, and a point-in-time membership sample. Coordinator apply time is not claimed. The broker was not started.

The non-stop allowlist is empty. Publish-gate membership is not a skip rule. Unknown types and invalid JSON are `NON_TERMINAL`.

The deployment gate is closed. Old and new members are not compatible. Runtime steps require a later controller authorization.

## Documents

- `docs/architecture/tracking/TRACKING_KAFKA_0_1C_OWNER_FENCE.md`
- `docs/architecture/tracking/TRACKING_KAFKA_0_1C_PERSISTENCE.md`
- `docs/architecture/tracking/TRACKING_KAFKA_0_1C_BROKER_TEST_PLAN.md`
- `docs/architecture/tracking/TRACKING_KAFKA_0_1C_EVENT_CLASSIFICATION.md`
- `docs/architecture/tracking/TRACKING_KAFKA_0_1C_DEPLOYMENT_GATE.md`

Implementation of the consumer, creation of migrations, broker tests, and deployment remain stopped.
