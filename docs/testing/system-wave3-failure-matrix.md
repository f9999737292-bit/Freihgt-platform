# System Test Wave 3 — Failure Matrix (F001–F025)

Canonical scenarios for failure/recovery/resilience validation.

---

## F001 — DATABASE_UNAVAILABLE

| Field | Value |
|-------|-------|
| **PRECONDITION** | Service running; Postgres stopped or unreachable |
| **ACTION** | HTTP mutation or integration test with bad DSN |
| **EXPECTED_HTTP** | 503/500 on dependent routes; health may still 200 |
| **EXPECTED_DB_STATE** | No partial writes |
| **EXPECTED_EVENT_STATE** | Outbox unchanged |
| **EXPECTED_RETRY_BEHAVIOUR** | Client/connection retry until timeout |
| **EXPECTED_RECOVERY** | Service resumes when DB returns |
| **TENANT_ISOLATION_EXPECTATION** | No cross-tenant reads on recovery |

**Evidence:** Readiness fails DB ping; integration tests use disposable DB only.

---

## F002 — DATABASE_TRANSACTION_ROLLBACK

| Field | Value |
|-------|-------|
| **PRECONDITION** | Valid tenant context |
| **ACTION** | Force failure mid-tx (outbox insert, projection update) |
| **EXPECTED_HTTP** | 4xx/5xx per API contract |
| **EXPECTED_DB_STATE** | Full rollback — no shipment without outbox, no inbox without projection |
| **EXPECTED_EVENT_STATE** | No PUBLISHED outbox from failed tx |
| **EXPECTED_RETRY_BEHAVIOUR** | Safe to retry mutation if idempotent |
| **EXPECTED_RECOVERY** | Consistent state after retry |
| **TENANT_ISOLATION_EXPECTATION** | Rollback scoped to initiating tenant |

**Tests:** `outbox_transaction_integration_test.go`, `TestProjectionFailureRollsBackInbox`

---

## F003 — DATABASE_RESTART

| Field | Value |
|-------|-------|
| **PRECONDITION** | Pending outbox / in-flight consumer |
| **ACTION** | Restart Postgres container |
| **EXPECTED_HTTP** | Temporary errors then recovery |
| **EXPECTED_DB_STATE** | Committed data persisted |
| **EXPECTED_EVENT_STATE** | Pending outbox survives restart |
| **EXPECTED_RETRY_BEHAVIOUR** | Worker resumes publish |
| **EXPECTED_RECOVERY** | Lease reclaim for stale claims |
| **TENANT_ISOLATION_EXPECTATION** | Unchanged |

**Tests:** `TestStaleLeaseRecovery`, worker restart patterns

---

## F004 — HTTP_DOWNSTREAM_500

| Field | Value |
|-------|-------|
| **PRECONDITION** | Gateway proxying to CT read-model |
| **ACTION** | Downstream returns 500 |
| **EXPECTED_HTTP** | Gateway returns degraded response (503/502 per config) |
| **EXPECTED_DB_STATE** | Gateway stateless |
| **EXPECTED_EVENT_STATE** | N/A |
| **EXPECTED_RETRY_BEHAVIOUR** | Bounded; no auth bypass |
| **EXPECTED_RECOVERY** | Normal when downstream healthy |
| **TENANT_ISOLATION_EXPECTATION** | Tenant headers preserved |

**Tests:** `controltowerreadmodel` blackbox 503

---

## F005 — HTTP_DOWNSTREAM_TIMEOUT

| Field | Value |
|-------|-------|
| **PRECONDITION** | Slow/stalled downstream |
| **ACTION** | Exceed client timeout |
| **EXPECTED_HTTP** | 504/503 within bounded timeout |
| **EXPECTED_DB_STATE** | N/A |
| **EXPECTED_EVENT_STATE** | N/A |
| **EXPECTED_RETRY_BEHAVIOUR** | Context cancellation |
| **EXPECTED_RECOVERY** | No goroutine leak |
| **TENANT_ISOLATION_EXPECTATION** | Fail-closed auth |

**Tests:** `TestProxyTimeoutReturnsServiceUnavailable`

---

## F006 — HTTP_CONNECTION_DROP

| Field | Value |
|-------|-------|
| **PRECONDITION** | In-flight proxy request |
| **ACTION** | Connection reset |
| **EXPECTED_HTTP** | Client sees error; server cleans up |
| **EXPECTED_DB_STATE** | Downstream may have completed — idempotency required |
| **EXPECTED_EVENT_STATE** | N/A |
| **EXPECTED_RETRY_BEHAVIOUR** | Idempotent retry safe where supported |
| **EXPECTED_RECOVERY** | Service healthy |
| **TENANT_ISOLATION_EXPECTATION** | Maintained |

**Tests:** Covered partially via timeout tests; idempotency suites for mutations

---

## F007 — DUPLICATE_HTTP_MUTATION

| Field | Value |
|-------|-------|
| **PRECONDITION** | Successful server-side mutation; ambiguous client |
| **ACTION** | Repeat same request (same idempotency key where required) |
| **EXPECTED_HTTP** | 200/409 consistent with first |
| **EXPECTED_DB_STATE** | Single business effect |
| **EXPECTED_EVENT_STATE** | Single outbox row per source event |
| **EXPECTED_RETRY_BEHAVIOUR** | Dedupe |
| **EXPECTED_RECOVERY** | N/A |
| **TENANT_ISOLATION_EXPECTATION** | Key scoped per tenant |

**Tests:** `orderexecution` idempotent execute; `freightsettlement` Test03DuplicateIdempotent

---

## F008 — CONCURRENT_MUTATION

| Field | Value |
|-------|-------|
| **PRECONDITION** | Two concurrent award/settlement attempts |
| **ACTION** | Parallel requests |
| **EXPECTED_HTTP** | One success, one conflict |
| **EXPECTED_DB_STATE** | Single winner |
| **EXPECTED_EVENT_STATE** | Single canonical event |
| **EXPECTED_RETRY_BEHAVIOUR** | Loser may retry with new key |
| **EXPECTED_RECOVERY** | Consistent |
| **TENANT_ISOLATION_EXPECTATION** | No cross-tenant lock bleed |

**Tests:** `TestConcurrentDoubleAwardProtection`, `Test04ConcurrentCreateNoDuplicate`

---

## F009 — KAFKA_UNAVAILABLE

| Field | Value |
|-------|-------|
| **PRECONDITION** | Business committed; outbox PENDING |
| **ACTION** | Broker down during publish |
| **EXPECTED_HTTP** | Mutation already succeeded |
| **EXPECTED_DB_STATE** | Business row committed |
| **EXPECTED_EVENT_STATE** | Outbox FAILED/PENDING with retry |
| **EXPECTED_RETRY_BEHAVIOUR** | Worker backoff |
| **EXPECTED_RECOVERY** | Publish when broker up |
| **TENANT_ISOLATION_EXPECTATION** | Outbox queries tenant-scoped |

**Tests:** `TestBrokerUnavailableMarksRetryableFailure`

---

## F010 — OUTBOX_PUBLISH_FAILURE

| Field | Value |
|-------|-------|
| **PRECONDITION** | Committed outbox row |
| **ACTION** | Publish error (non-transient) |
| **EXPECTED_HTTP** | N/A |
| **EXPECTED_DB_STATE** | Business intact |
| **EXPECTED_EVENT_STATE** | FAILED after max attempts or retryable |
| **EXPECTED_RETRY_BEHAVIOUR** | Bounded |
| **EXPECTED_RECOVERY** | Manual/ops or retry |
| **TENANT_ISOLATION_EXPECTATION** | Per-row tenant_id |

**Tests:** `outbox_state_integration_test.go`

---

## F011 — OUTBOX_RETRY

| Field | Value |
|-------|-------|
| **PRECONDITION** | FAILED/PENDING outbox |
| **ACTION** | Worker cycle after transient error |
| **EXPECTED_HTTP** | N/A |
| **EXPECTED_DB_STATE** | Unchanged |
| **EXPECTED_EVENT_STATE** | PUBLISHED terminal |
| **EXPECTED_RETRY_BEHAVIOUR** | Exponential backoff |
| **EXPECTED_RECOVERY** | Full pipeline |
| **TENANT_ISOLATION_EXPECTATION** | Claim excludes other tenants |

**Tests:** `outbox_worker_integration_test.go`

---

## F012 — DUPLICATE_EVENT_DELIVERY

| Field | Value |
|-------|-------|
| **PRECONDITION** | Event already processed |
| **ACTION** | Redeliver same event_id/kafka offset |
| **EXPECTED_HTTP** | N/A |
| **EXPECTED_DB_STATE** | No duplicate projection/fact |
| **EXPECTED_EVENT_STATE** | Classified DUPLICATE |
| **EXPECTED_RETRY_BEHAVIOUR** | Consumer continues |
| **EXPECTED_RECOVERY** | Offset advanced |
| **TENANT_ISOLATION_EXPECTATION** | Dedupe per tenant inbox |

**Tests:** `DuplicateDelivery`, `TestDuplicateSameEventIDClassifiedAsDuplicateNotStale`

---

## F013 — CONSUMER_CRASH_AFTER_RECEIVE

| Field | Value |
|-------|-------|
| **PRECONDITION** | Message received, processing started |
| **ACTION** | Kill consumer before offset commit |
| **EXPECTED_HTTP** | N/A |
| **EXPECTED_DB_STATE** | If committed: idempotent on redelivery |
| **EXPECTED_EVENT_STATE** | At-least-once |
| **EXPECTED_RETRY_BEHAVIOUR** | Redelivery |
| **EXPECTED_RECOVERY** | No duplicate effect |
| **TENANT_ISOLATION_EXPECTATION** | Inbox tenant match |

**Tests:** `consumer_restart_integration_test.go`

---

## F014 — SERVICE_PROCESS_RESTART

| Field | Value |
|-------|-------|
| **PRECONDITION** | Pending work (outbox/consumer) |
| **ACTION** | SIGTERM + restart |
| **EXPECTED_HTTP** | /health OK after start |
| **EXPECTED_DB_STATE** | Consistent |
| **EXPECTED_EVENT_STATE** | Work resumes |
| **EXPECTED_RETRY_BEHAVIOUR** | Stale lease reclaim |
| **EXPECTED_RECOVERY** | Full |
| **TENANT_ISOLATION_EXPECTATION** | Maintained |

**Tests:** CT restart E2E, outbox worker restart

---

## F015 — STALE_EVENT

| Field | Value |
|-------|-------|
| **PRECONDITION** | Newer projection version applied |
| **ACTION** | Deliver older event |
| **EXPECTED_HTTP** | N/A |
| **EXPECTED_DB_STATE** | Projection unchanged |
| **EXPECTED_EVENT_STATE** | OutcomeStale |
| **EXPECTED_RETRY_BEHAVIOUR** | Ack and skip |
| **EXPECTED_RECOVERY** | Monotonic state |
| **TENANT_ISOLATION_EXPECTATION** | Version per aggregate/tenant |

**Tests:** `TestStaleDuplicateNewerAfterActivation`, `TestProcessEventStaleVersionDoesNotUpdateProjection`

---

## F016 — OUT_OF_ORDER_EVENT

| Field | Value |
|-------|-------|
| **PRECONDITION** | Events with version ordering |
| **ACTION** | Newer then older delivery |
| **EXPECTED_HTTP** | N/A |
| **EXPECTED_DB_STATE** | Canonical = newest applied |
| **EXPECTED_EVENT_STATE** | Older rejected as stale |
| **EXPECTED_RETRY_BEHAVIOUR** | Consumer stable |
| **EXPECTED_RECOVERY** | Catch-up on missing newer |
| **TENANT_ISOLATION_EXPECTATION** | Per-aggregate |

**Tests:** Same as F015; shipment status monotonic in domain

---

## F017 — CONTROL_TOWER_PROJECTION_LOSS

| Field | Value |
|-------|-------|
| **PRECONDITION** | Live consumer + projection |
| **ACTION** | Truncate projection table (disposable) |
| **EXPECTED_HTTP** | CT APIs show stale/missing until rebuild |
| **EXPECTED_DB_STATE** | Canonical shipment intact |
| **EXPECTED_EVENT_STATE** | Kafka retains or outbox replay |
| **EXPECTED_RETRY_BEHAVIOUR** | Rebuild/catch-up |
| **EXPECTED_RECOVERY** | Rebuild restores |
| **TENANT_ISOLATION_EXPECTATION** | No cross-tenant rows in rebuild |

**Tests:** `kafka_catchup_integration_test.go`, live consumer tests

---

## F018 — CONTROL_TOWER_REBUILD

| Field | Value |
|-------|-------|
| **PRECONDITION** | Canonical + correct projection |
| **ACTION** | Reset read model; official rebuild |
| **EXPECTED_HTTP** | CT matches post-rebuild |
| **EXPECTED_DB_STATE** | Rebuild backup table |
| **EXPECTED_EVENT_STATE** | Catch-up if configured |
| **EXPECTED_RETRY_BEHAVIOUR** | Activation rollback on failure |
| **EXPECTED_RECOVERY** | Semantic match |
| **TENANT_ISOLATION_EXPECTATION** | REBUILD_ROW tenant-scoped |

**Tests:** `acceptance_integration_test.go`, failure_injection rebuild hooks

---

## F019 — BILLING_DUPLICATE_EVENT

| Field | Value |
|-------|-------|
| **PRECONDITION** | Settlement created |
| **ACTION** | Duplicate idempotency key / concurrent create |
| **EXPECTED_HTTP** | Idempotent response |
| **EXPECTED_DB_STATE** | Single settlement row |
| **EXPECTED_EVENT_STATE** | N/A (HTTP path) |
| **EXPECTED_RETRY_BEHAVIOUR** | Safe retry |
| **EXPECTED_RECOVERY** | N/A |
| **TENANT_ISOLATION_EXPECTATION** | Keys tenant-scoped |

**Tests:** `freightsettlement` exit gates

---

## F020 — FREIGHT_COST_DUPLICATE_EVENT

| Field | Value |
|-------|-------|
| **PRECONDITION** | Cost fact ingested |
| **ACTION** | Replay duplicate delivery |
| **EXPECTED_HTTP** | NoOp / 200 |
| **EXPECTED_DB_STATE** | Single ledger effect |
| **EXPECTED_EVENT_STATE** | NoOpFact |
| **EXPECTED_RETRY_BEHAVIOUR** | Idempotent ingest |
| **EXPECTED_RECOVERY** | Rebuild idempotent |
| **TENANT_ISOLATION_EXPECTATION** | Tenant in dedupe key |

**Tests:** `ledger_led_integration_test.go`, variance outbox duplicate

---

## F021 — CROSS_SERVICE_RECOVERY

| Field | Value |
|-------|-------|
| **PRECONDITION** | Award → shipment → outbox → CT path |
| **ACTION** | Failure at each layer (subset) |
| **EXPECTED_HTTP** | Per-service |
| **EXPECTED_DB_STATE** | No orphan cross-service refs |
| **EXPECTED_EVENT_STATE** | Eventually consistent via outbox/Kafka |
| **EXPECTED_RETRY_BEHAVIOUR** | Per component |
| **EXPECTED_RECOVERY** | End-to-end on disposable stack |
| **TENANT_ISOLATION_EXPECTATION** | Lineage preserved |

**Tests:** Kafka E2E + live consumer (when Kafka available)

---

## F022 — CROSS_TENANT_FAILURE_ISOLATION

| Field | Value |
|-------|-------|
| **PRECONDITION** | TENANT_A failure path; TENANT_B parallel |
| **ACTION** | Retry/outbox/consumer for A only |
| **EXPECTED_HTTP** | B unaffected |
| **EXPECTED_DB_STATE** | No B mutation from A failure |
| **EXPECTED_EVENT_STATE** | Outbox claim tenant-filtered |
| **EXPECTED_RETRY_BEHAVIOUR** | Scoped queries |
| **EXPECTED_RECOVERY** | Independent |
| **TENANT_ISOLATION_EXPECTATION** | CROSS_TENANT_*=NO |

**Tests:** Wave 1 security + freightsettlement cross-tenant + concurrent outbox claim

---

## F023 — BACKUP_RESTORE

| Field | Value |
|-------|-------|
| **PRECONDITION** | Disposable Postgres |
| **ACTION** | pg_dump backup/restore OR rebuild backup table |
| **EXPECTED_HTTP** | N/A |
| **EXPECTED_DB_STATE** | Schema + data match |
| **EXPECTED_EVENT_STATE** | N/A |
| **EXPECTED_RETRY_BEHAVIOUR** | N/A |
| **EXPECTED_RECOVERY** | Service starts |
| **TENANT_ISOLATION_EXPECTATION** | Data boundaries preserved |

**Status:** pg_dump tooling = staging-only (`bintrans_ct_staging_backup.sh`) — **BLOCKED in CI**. In-DB rebuild backup covered by F018 tests.

---

## F024 — READINESS_DEPENDENCY_FAILURE

| Field | Value |
|-------|-------|
| **PRECONDITION** | DB down; Kafka optional |
| **ACTION** | GET /ready |
| **EXPECTED_HTTP** | 503 when DB unreachable |
| **EXPECTED_DB_STATE** | N/A |
| **EXPECTED_EVENT_STATE** | N/A |
| **EXPECTED_RETRY_BEHAVIOUR** | K8s restart |
| **EXPECTED_RECOVERY** | Ready when DB up |
| **TENANT_ISOLATION_EXPECTATION** | N/A |

**Evidence:** `shared-go/observability/ready.go` — DB only; Kafka intentionally excluded

---

## F025 — GRACEFUL_SHUTDOWN

| Field | Value |
|-------|-------|
| **PRECONDITION** | In-flight HTTP, outbox worker, consumer |
| **ACTION** | SIGTERM |
| **EXPECTED_HTTP** | Drain or cancel with deadline |
| **EXPECTED_DB_STATE** | In-flight tx completes or rolls back |
| **EXPECTED_EVENT_STATE** | No duplicate publish |
| **EXPECTED_RETRY_BEHAVIOUR** | Pending work reclaim |
| **EXPECTED_RECOVERY** | Clean restart |
| **TENANT_ISOLATION_EXPECTATION** | N/A |

**Tests:** `worker_test.go` ShutdownCancellation; consumer Close

---

## Intentionally Absent Ordering Guarantees

- Global cross-aggregate event ordering across Kafka partitions
- Payment service (no event consumer in repo)
- Gateway `/ready` does not include CT read-model (documented operational gap)
