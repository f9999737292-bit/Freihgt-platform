# System Test Wave 3 — Report

**Wave:** Failure, Recovery, Resilience & Operational Readiness v1  
**Branch:** `test/system-wave3-failure-recovery-resilience-v1`  
**Date:** 2026-08-25

---

## BASELINE

| Field | Value |
|-------|-------|
| PR64_STATE | MERGED |
| PR64_HEAD | `24cbc471b39bde898071cccd4142a5acc2ac1232` |
| PR64_MERGEABLE | N/A (merged) |
| PR64_CI | PASS (Wave 2, pre-merge) |
| PR64_MERGED | YES |
| PR64_MERGE_SHA | `49938beb0d194ba3e0780ad7beb9f3db3d7f572b` |
| WAVE3_BASE_SHA | `49938beb0d194ba3e0780ad7beb9f3db3d7f572b` |
| WORKTREE | `D:\Projects\freight-platform-wt\system-wave3-failure-recovery-resilience-v1` |
| BRANCH | `test/system-wave3-failure-recovery-resilience-v1` |
| BASE_SHA | `49938beb0d194ba3e0780ad7beb9f3db3d7f572b` |
| ORIGIN_MAIN_SHA_AT_START | `49938beb0d194ba3e0780ad7beb9f3db3d7f572b` |
| PR64_IS_ANCESTOR_OF_MAIN | YES |

---

## DISCOVERY

Full component map: `docs/testing/system-wave3-discovery.md`

**Summary:** Shipment service implements canonical transactional outbox with atomic shipment+history+outbox writes. Control Tower read-model consumes Kafka with inbox+projection transactional apply, duplicate/stale classification, and staged rebuild with in-DB backup. Billing and freight-cost financial dedupe operate via HTTP idempotency keys and ledger ingest idempotency (not Kafka consumers for billing). Extensive integration tests existed but were not aggregated in CI until Wave 3.

---

## FAILURE_MATRIX

Canonical F001–F025 matrix: `docs/testing/system-wave3-failure-matrix.md`

---

## DATABASE_FAILURES

| Service | Mechanism | Test evidence |
|---------|-----------|---------------|
| Shipment | Single pgx tx for shipment + history + outbox | W3-01: `TestAtomic*`, `TestRollback*` |
| CT projection | Inbox + projection single tx; rollback on projection failure | W3-02: `TestProjectionFailureRollsBackInbox` |
| RFx award | Transactional award + conflict | W3-11: `ConcurrentDoubleAward` |

**PARTIAL_COMMIT=NO** — asserted in W3-01/W3-02  
**ORPHAN_RECORD=NO** — rollback tests cover companion row loss  
**CROSS_TENANT_MUTATION=NO** — W3-26/W3-27

---

## HTTP_FAILURES

| Scenario | Evidence |
|----------|----------|
| Downstream 503 | W3-07 `TestBlackBoxPrimaryFallback503` |
| Downstream timeout | W3-07 `TestBlackBoxPrimaryFallbackTimeout` |
| Fail-closed / no secret leak | W3-08 |

**HTTP_FAILURE_BEHAVIOUR:** Bounded timeout, meaningful status, request-id retained; auth not bypassed on dependency failure.

---

## IDEMPOTENCY

| Endpoint area | IDEMPOTENCY_SUPPORTED | DEDUP_MECHANISM | DUPLICATE_BUSINESS_EFFECT |
|---------------|----------------------|-----------------|---------------------------|
| Order execution | YES | Idempotency table | NO (W3-09) |
| Settlement create | YES | Idempotency key + unique | NO (W3-10) |
| RFx award | Conflict on double | Transactional lock | NO (W3-11) |
| Freight-cost ingest | YES | source_event_id uniqueness | NO (W3-24/W3-25) |

---

## OUTBOX

| Property | Result |
|----------|--------|
| Business commits before publish attempt | YES |
| Kafka down → PENDING/retryable | YES (W3-12) |
| Recovery on broker return | YES (W3-03, W3-12) |
| BUSINESS_DATA_LOST | NO |
| EVENT_LOST | NO |
| DUPLICATE_BUSINESS_EFFECT | NO |

---

## KAFKA

| Property | Result |
|----------|--------|
| Producer E2E (shipment → topic) | W3-12 |
| Broker unavailable recovery | W3-12 |
| CT consumer read + commit | W3-14, W3-15 |
| Catch-up after rebuild | W3-22 |

**Note:** CI provides Redpanda via Docker; `REQUIRE_TEST_KAFKA=1` prevents silent skip.

---

## DUPLICATE_EVENT_SAFETY

| Consumer domain | Evidence |
|-----------------|----------|
| Shipment publish (at-least-once) | W3-13 |
| CT projection inbox | W3-16, W3-17 |
| Billing settlement | W3-23 |
| Freight-cost ledger | W3-24, W3-25 |

Consumers recover correctly (idempotent upsert/classification), not merely constraint errors.

---

## CONSUMER_CRASH_RECOVERY

W3-14: `TestConsumerRestartOffsetCommitFailureE2E` — DB commit then offset failure → redelivery → no duplicate business effect.

---

## EVENT_ORDERING

| Domain | Guarantee | Evidence |
|--------|-----------|----------|
| CT projection | Monotonic version; stale rejected | W3-16, W3-18 |
| Freight-cost ledger | Revision ordering | W3-24 `OutOfOrder` |
| Shipment Kafka | Per-partition key ordering | W3-12 integration |

**Intentionally absent:** global cross-partition ordering; payment Kafka consumer (not in repo).

---

## CONTROL_TOWER_LIVE

| Field | Value |
|-------|-------|
| CT_LIVE_CONSUMPTION | YES (W3-14, W3-15, W3-22 with Kafka) |
| CT_TENANT_LINEAGE | YES (W3-21, W3-27) |
| CT_COMPANY_LINEAGE | Partial — company in event payload; tenant is primary isolation key |

Live path exercised via Kafka integration tests, not direct DB projection inserts.

---

## CONTROL_TOWER_REBUILD

| Field | Value |
|-------|-------|
| REBUILD_COMPLETED | YES (W3-19) |
| REBUILD_ROW_COUNT_MATCH | YES (acceptance tests) |
| REBUILD_SEMANTIC_MATCH | YES (historical + rollback acceptance) |
| CROSS_TENANT_LEAK | NO (W3-21) |

Failure injection: W3-20 preserves projection on activation/rollback failure.

---

## FINANCIAL_RECOVERY

| Chain stage | Coverage |
|-------------|----------|
| Settlement duplicate/retry | W3-23 |
| Freight-cost duplicate/replay/rebuild | W3-24 |
| Variance ingest dedupe | W3-25 |
| Award → billing E2E | Wave 2 (not re-run in Wave 3 orchestrator except security) |

**DUPLICATE_INVOICE=NO**, **DUPLICATE_LEDGER_EFFECT=NO**, **DUPLICATE_FREIGHT_COST=NO**, **CROSS_TENANT_FINANCIAL_LEAK=NO**

Payment settlement Kafka stage not connected in current product — documented NOT_APPLICABLE.

---

## TENANT_FAILURE_ISOLATION

W3-26 (freight-cost SEC), W3-27 (CT inbox), W3-21 (rebuild tenant), W3-30 (Wave 1 full regression).

**CROSS_TENANT_READ=NO**, **CROSS_TENANT_WRITE=NO**, **CROSS_TENANT_REBUILD_LEAK=NO**

---

## SERVICE_RESTART

W3-14 consumer restart; W3-04 stale lease reclaim; W3-06 worker shutdown unit tests.

---

## HEALTH_READINESS

| Field | Value |
|-------|-------|
| HEALTH_MODEL | `/health` liveness — process up |
| READINESS_MODEL | `/ready` — DB ping (`shared-go/observability/ready.go`); Kafka excluded by design |
| DEPENDENCY_FAILURE_BEHAVIOUR | DB down → not ready; gateway CT proxy degrades without auth bypass |

CT read-model not in gateway aggregate `/ready` — operational gap, not data safety defect.

---

## GRACEFUL_SHUTDOWN

W3-28: `TestWorkerShutdownCancellation`, `TestWorkerCancelledBeforeStart`. Consumer Close in kafka integration tests.

---

## BACKUP_RESTORE

| Field | Value |
|-------|-------|
| BACKUP_CREATED | BLOCKED (pg_dump staging script) |
| BACKUP_VALIDATED | BLOCKED |
| RESTORE_COMPLETED | BLOCKED |
| SCHEMA_MATCH | N/A in CI |
| REPRESENTATIVE_DATA_MATCH | N/A in CI |

**Evidence reused:** In-DB rebuild backup in W3-19/W3-20 (`shipment_status_projection_rebuild_backup`). Staging pg_dump: `scripts/ops/bintrans_ct_staging/bintrans_ct_staging_backup.sh` — not CI-safe.

---

## OBSERVABILITY

| Failure class | Classification |
|---------------|----------------|
| Outbox publish failure | DETECTABLE (metrics + status column) |
| Consumer processing failure | DETECTABLE (dead letter + metrics) |
| HTTP downstream failure | DIAGNOSABLE (gateway logs, request-id) |
| DB transaction rollback | DIAGNOSABLE (integration tests; app error logs) |
| Backup/restore ops | NOT_OBSERVABLE in CI |

Secrets not exposed in outbox log tests (`TestWorkerLogsDoNotContainPayload`, W3-08).

---

## SECURITY_REGRESSION

Wave 1 orchestrator re-run as W3-30 mandatory suite:

| Gate | Expected |
|------|----------|
| AUTH_REGRESSION | PASS |
| RBAC_REGRESSION | PASS |
| TENANT_ISOLATION_REGRESSION | PASS |
| COMPANY_ISOLATION_REGRESSION | PASS |

No production auth/RBAC/tenant weakening in Wave 3 changes.

---

## CI

New job: `system-wave3-resilience`

- Fresh checkout
- Postgres service (disposable)
- Redpanda via Docker (`localhost:9092`)
- `REQUIRE_TEST_DATABASE=1`, `REQUIRE_TEST_KAFKA=1`
- Entrypoint: `scripts/test/run-system-wave3-resilience.sh`
- Makefile: `make system-test-wave3-resilience`

---

## FINDINGS

| ID | Severity | Description | Status |
|----|----------|-------------|--------|
| W3-GAP-001 | LOW | Kafka tests previously skipped silently without brokers | FIXED — `REQUIRE_TEST_KAFKA` |
| W3-GAP-002 | LOW | Resilience suites not in CI | FIXED — Wave 3 job |
| W3-GAP-003 | LOW | pg_dump backup/restore not CI-safe | BLOCKED — documented |
| W3-GAP-004 | LOW | Gateway `/ready` excludes CT read-model | DOCUMENTED — by design |

No CRITICAL or HIGH data corruption / auth bypass / tenant leak defects found during discovery.

---

## Deliverables

| Artifact | Path |
|----------|------|
| Discovery | `docs/testing/system-wave3-discovery.md` |
| Failure matrix | `docs/testing/system-wave3-failure-matrix.md` |
| Orchestrator | `scripts/test/run-system-wave3-resilience.sh` |
| Manifest | `tests/system/wave3/WAVE3_MANIFEST.yaml` |
| Makefile target | `system-test-wave3-resilience` |
| CI job | `.github/workflows/ci.yml` → `system-wave3-resilience` |

---

## FINAL VERDICT

**CI confirmation:** Run on `31033c07bfd91e149ebbc58ca9bbee33d5fbbbcf` — `system-wave3-resilience` **PASS** (4m52s). PR [#65](https://github.com/f9999737292-bit/Freihgt-platform/pull/65).

```
PR64_MERGED=YES
PR64_MERGE_SHA=49938beb0d194ba3e0780ad7beb9f3db3d7f572b

WAVE3_BASE_SHA=49938beb0d194ba3e0780ad7beb9f3db3d7f572b
BRANCH=test/system-wave3-failure-recovery-resilience-v1
FINAL_HEAD=31033c07bfd91e149ebbc58ca9bbee33d5fbbbcf
REMOTE_HEAD=31033c07bfd91e149ebbc58ca9bbee33d5fbbbcf
WORKTREE_CLEAN=YES

DATABASE_FAILURE_SAFETY=PASS
PARTIAL_COMMIT_PROTECTION=PASS

HTTP_FAILURE_BEHAVIOUR=PASS
IDEMPOTENCY=PASS

OUTBOX_RECOVERY=PASS
KAFKA_RECOVERY=PASS
DUPLICATE_EVENT_SAFETY=PASS
CONSUMER_CRASH_RECOVERY=PASS
EVENT_ORDERING=PASS

CONTROL_TOWER_LIVE_CONSUMPTION=PASS
CONTROL_TOWER_REBUILD=PASS

FINANCIAL_RECOVERY=PASS
TENANT_FAILURE_ISOLATION=PASS

SERVICE_RESTART_RECOVERY=PASS
HEALTH_READINESS=PASS
GRACEFUL_SHUTDOWN=PASS

BACKUP_RESTORE=BLOCKED
OBSERVABILITY=PASS

AUTH_REGRESSION=PASS
RBAC_REGRESSION=PASS
TENANT_ISOLATION_REGRESSION=PASS
COMPANY_ISOLATION_REGRESSION=PASS

CRITICAL_OPEN=0
HIGH_OPEN=0
MEDIUM_OPEN=0
LOW_OPEN=2

SYSTEM_TEST_WAVE3_COMPLETE=YES
WAVE3_VERDICT=CONDITIONAL_PASS

MERGE_RECOMMENDATION=YES

REMAINING_GAPS=pg_dump backup/restore (F023) staging-only not CI-safe; gateway /ready excludes CT read-model by design; payment Kafka chain not connected

NEXT_RECOMMENDED_WAVE=Wave 4 — operational acceptance (staging pg_dump cert, full gateway golden HTTP path, procurement browser E2E)

STOP_AFTER_WAVE3=YES
```

**LOW findings:** W3-GAP-003 (backup BLOCKED), W3-GAP-004 (gateway ready scope).

**Fixes applied during Wave 3:** CI Redpanda listener, CT migration parity (000041), `REQUIRE_TEST_KAFKA`, Kafka `DriverTopic` in test helpers, outbox claim test unique transport order seeding.
