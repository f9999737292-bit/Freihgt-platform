# System Test Wave 3 — Discovery

**Baseline:** `49938beb0d194ba3e0780ad7beb9f3db3d7f572b` (main after PR #64)  
**Scope:** Failure, recovery, resilience, operational readiness  
**Method:** Repository source + existing integration test inventory (no assumptions)

---

## Component Map

| COMPONENT | FAILURE_MODE | CURRENT_PROTECTION | TEST_COVERAGE | GAP | WAVE3_ACTION |
|-----------|--------------|-------------------|---------------|-----|--------------|
| **A. DB transactions (shipment+outbox)** | Partial commit (shipment without outbox) | Single pgx tx in `shipment_repository.go`; rollback on outbox/history failure | `outbox_transaction_integration_test.go` | None for atomic create/transition | Orchestrate W3-F002 |
| **A. DB transactions (rfx award)** | Double award | Transactional award + conflict | `evaluation_award_integration_test.go` ConcurrentDoubleAward | — | Orchestrate W3-F008 |
| **A. DB transactions (CT projection)** | Inbox committed, projection fails | Single tx rollback in `projection_repository.go` | `TestProjectionFailureRollsBackInbox` | — | Orchestrate W3-F002 |
| **A. DB transactions (CT rebuild)** | Partial activation | Advisory lock + staged tx hooks | `failure_injection_integration_test.go` | — | Orchestrate W3-F018 |
| **B. Shipment outbox** | Publish failure after commit | PENDING/FAILED states, retry backoff, max attempts | `outbox_worker_integration_test.go`, `outbox_state_integration_test.go` | — | Orchestrate W3-F010/F011 |
| **B. Payment outbox** | N/A | Not implemented in repo | None | No payment outbox | Document NOT_APPLICABLE |
| **B. Freight-cost ingest** | Duplicate event | Idempotent ingest outcomes (`NoOpEvent`, `NoOpFact`) | `variance/outbox_integration_test.go`, `ledger_led_integration_test.go` | — | Orchestrate W3-F020 |
| **C. Kafka producer (shipment)** | Broker unavailable | Classified errors, retry, FAILED terminal | `kafka_e2e_integration_test.go` BrokerUnavailable | Requires `TEST_KAFKA_BROKERS` | CI redpanda + W3-F009 |
| **D. Kafka consumer (CT read-model)** | Crash after DB commit | Offset commit after tx; duplicate idempotent | `consumer_restart_integration_test.go` | Requires Kafka | W3-F013/F014 |
| **E. Retries** | Transient publish errors | Exponential backoff `outbox/backoff.go`; `ReleaseWithRetry` | `outbox_worker_integration_test.go` | — | W3-F011 |
| **F. Idempotency keys** | Duplicate HTTP/command | Transport order idempotency table; settlement idempotency; award conflict | `orderexecution`, `freightsettlement/exit_gates` | Not all endpoints | W3-F007 |
| **G. Deduplication** | Duplicate Kafka delivery | Outbox `UNIQUE(source_event_id)`; CT inbox by event_id/kafka position | `DuplicateDelivery`, `TestDuplicateSameEventID` | — | W3-F012 |
| **H. HTTP timeouts** | Downstream hang | Gateway proxy timeout (30s default); CT client 800ms | `blackbox_integration_test.go` Timeout/503 | — | W3-F004/F005 |
| **I. Graceful shutdown** | SIGTERM during publish | Context cancel in worker; consumer Close | `worker_test.go` ShutdownCancellation | Unit-level | W3-F025 |
| **J. Health/readiness** | DB down | `/health` always OK; `/ready` pings DB only (Kafka excluded by design) | `low-code-service/readiness_test.go`; docs | CT read-model not in gateway `/ready` | W3-F024 |
| **K. CT live projection** | Consumer lag/loss | Inbox + projection tx; offset after commit | Kafka integration + live_consumer | Needs Kafka in CI | W3-F011/F017 |
| **L. CT rebuild** | Import/activation failure | Staged import, activation hooks, rollback | `rebuild/*_integration_test.go`, Makefile acceptance | Live acceptance needs Docker stack | W3-F018 (integration subset in CI) |
| **M. Billing events** | Duplicate settlement | Idempotency key + unique constraints | `Test03DuplicateIdempotent`, `Test04ConcurrentCreateNoDuplicate` | HTTP-only (no Kafka consumer) | W3-F019 |
| **N. Backup/restore (pg_dump)** | Restore to wrong env | `bintrans_ct_staging_backup.sh` (staging-only) | Script + migrate gate | Not safe for generic CI | BLOCKED — document only |
| **N. Rebuild in-DB backup** | Bad activation | `shipment_status_projection_rebuild_backup` table | `failure_injection`, `acceptance_integration_test.go` | — | W3-F018 evidence |
| **O. Observability** | Silent failure | Prometheus metrics (outbox, consumer); structured logs | `metrics-check`, shadow observability scripts | Alerts example-only | W3-F019 classify DETECTABLE |
| **P. Existing chaos tests** | — | Deterministic failure injection (not network chaos) | See Wave 3 orchestrator manifest | No chaos mesh | Aggregate existing suites |

---

## Key File References

| Area | Path |
|------|------|
| Shipment outbox worker | `services/shipment-service/internal/outbox/worker.go` |
| Shipment outbox tx | `services/shipment-service/internal/repository/shipment_repository.go` |
| CT consumer | `services/control-tower-read-model-service/internal/consumer/consumer.go` |
| CT projection inbox | `services/control-tower-read-model-service/internal/repository/projection_repository.go` |
| CT rebuild | `services/control-tower-read-model-service/internal/rebuild/` |
| Gateway CT fallback | `services/api-gateway/internal/integration/controltowerreadmodel/` |
| Freight-cost ledger dedupe | `services/freight-cost-service/internal/integration/ledger/` |
| Shared readiness | `packages/shared-go/observability/ready.go` |
| Rebuild ops scripts | `scripts/dev/control_tower_projection_rebuild_*.sh` |
| Staging backup | `scripts/ops/bintrans_ct_staging/bintrans_ct_staging_backup.sh` |

---

## Environment Variables (test)

| Variable | Purpose |
|----------|---------|
| `TEST_DATABASE_URL` | Disposable Postgres (required CI) |
| `REQUIRE_TEST_DATABASE=1` | Fail if DB missing |
| `TEST_KAFKA_BROKERS` | Kafka/Redpanda for live messaging tests |
| `TEST_KAFKA_TOPIC_PREFIX` | Optional topic isolation |

---

## CI Gap (pre-Wave3)

Integration/resilience tests were Makefile-local only (`-tags=integration`). Wave 3 adds `system-wave3-resilience` CI job with Postgres + Kafka services.
