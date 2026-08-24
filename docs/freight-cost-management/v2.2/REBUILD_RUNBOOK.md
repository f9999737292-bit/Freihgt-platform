# FREIGHT COST INTELLIGENCE v2.2G — Rebuild Runbook

**Status:** Operational reference  
**Date:** 2026-08-23  
**Audience:** Platform ops, on-call engineers

---

## 1. Overview

Analytics projections (v2.2B–E) are **derived read models**. A full tenant rebuild deletes and recomputes all analytics tables for one tenant from canonical sources (`cost_summary_projection`, transport dimensions, settlement accessorials). Safe for disaster recovery, mapping-version drift, or post-migration reconciliation.

**Scope:** Tenant-level only. Does not mutate ledger or settlement authoritative data.

---

## 2. When to rebuild

| Trigger | Action |
|---------|--------|
| Post-migration verification | Single-tenant rebuild + equivalence check |
| Mapping version pin drift suspected | Rebuild affected tenant |
| Enrichment snapshot stale (company rename) | Rebuild or wait for scheduled 24h cycle |
| Dirty queue backlog / projection `STALE` | Investigate worker; rebuild if incremental cannot catch up |
| Disaster recovery (projection tables lost) | Rebuild all tenants after restoring Postgres |
| Concurrent operator mistake | Advisory lock serializes — safe to retry |

**Do not rebuild** during active ledger ingest incident until canonical source is stable.

---

## 3. Tenant scope

Rebuild affects **one tenant UUID** at a time:

- Deletes: order facts, period/lane/carrier/accessorial projections, benchmarks, opportunities for tenant
- Rebuilds from: all `cost_summary_projection` rows for tenant + dimension/settlement batch enrichment
- Does **not** cross tenant boundaries (verified: `TestFC22BRebuildTenantCurrencySeparated`, `TestFC22CSEC001TenantIsolation`)

---

## 4. Prechecks

1. **Feature gate:** `FREIGHT_COST_ANALYTICS_PROJECTION_ENABLED=true` on freight-cost-service (default OFF).
2. **Database connectivity:** Postgres reachable; migrations 000061–000064 applied (`TestFC22G_MigrationGateV22UpDown`).
3. **Dependencies:** transport-order-service (batch dimensions), company-service (batch display), billing settlement reads available.
4. **No concurrent rebuild:** Check `analytics_projection_state.status` — if `RUNNING`, wait or investigate stuck transaction.
5. **Canonical health:** Spot-check `cost_summary_projection` row count for tenant > 0 if analytics expected.

---

## 5. Procedure

### 5.1 Manual rebuild (internal S2S)

```http
POST /internal/v1/freight-cost/analytics/tenants/{tenantId}/rebuild
Authorization: Bearer <internal-service-token>
```

Handler: `AnalyticsProjectionHandler.RebuildTenant` → `AnalyticsProjectionWorker.RebuildTenantNow` → `AnalyticsProjectionService.RebuildTenant`.

**Response:** `202 Accepted` with `{"tenant_id":"...","status":"rebuild_completed"}` on success.

### 5.2 Scheduled rebuild

Worker (`analytics_projection_worker.go`) runs full tenant rebuild for all tenants in `analytics_projection_state` every `FREIGHT_COST_ANALYTICS_REBUILD_INTERVAL` (default 24h).

### 5.3 Incremental path (normal operation)

Dirty queue processed by `ProcessDirtyBatch` with `FREIGHT_COST_ANALYTICS_DIRTY_BATCH_SIZE` (default 50) on `FREIGHT_COST_ANALYTICS_DIRTY_POLL_INTERVAL` (default 5s). Prefer incremental unless full rebuild required.

---

## 6. Monitoring metrics

| Metric | Meaning |
|--------|---------|
| `freight_cost_analytics_rebuild_total{result="success\|error"}` | Rebuild completion count |
| `freight_cost_analytics_rebuild_duration_seconds` | Tenant rebuild latency histogram |
| `freight_cost_analytics_benchmark_rebuild_failures_total` | Benchmark/opportunity step failures |
| `analytics_projection_state.status` | `IDLE`, `RUNNING`, `ERROR` per tenant |
| `analytics_projection_state.last_error_code` | Last failure reason |

**Logs:** `scheduled analytics tenant rebuild completed` / `failed` with `tenant_id`.

---

## 7. Failure and retry

| Failure | Action |
|---------|--------|
| Dependency timeout (company/transport/billing) | Retry rebuild; check downstream S2S health |
| Transaction rollback | State remains `ERROR`; inspect `last_error_message` |
| Partial enrichment | Rebuild is all-or-nothing per tenant (single transaction with advisory lock) |
| Lock wait | Concurrent rebuild blocked — second caller waits for xact lock release |

**Retry policy:** Idempotent — safe to re-run (`TestFC22BRebuildIdempotent`). On repeated failure, escalate with tenant ID and `last_error_code`.

---

## 8. Concurrent rebuild advisory lock

**Implementation:** `repository.AcquireTenantAnalyticsExclusiveLock` uses `pg_advisory_xact_lock(tenantAnalyticsLockKey(tenantID))` with namespace `0x4643415050524F4A` (FCAPPROJ).

**Behavior:** Transaction-scoped exclusive lock per tenant. Two concurrent `RebuildTenant` calls for the same tenant serialize; both complete successfully when run sequentially within lock.

**Test:** `TestFC22G_ConcurrentRebuildSameTenantSerialized` — two goroutines, zero errors.

---

## 9. Verification steps

After rebuild:

1. **State endpoint:**
   ```http
   GET /internal/v1/freight-cost/analytics/tenants/{tenantId}/state
   ```
   Expect `status=IDLE`, fresh `calculated_at`, `data_through` ≥ latest summary update.

2. **Equivalence (if incremental was running):** Compare period totals before/after; full ≡ incremental proven in CI (`TestFC22BEqvRebuildMatchesIncremental`, `TestFC22G_FullStackRebuildIncrementalEquivalence`).

3. **Coverage counters:** Check `analytics_projection_coverage` for unexpected `missing_city` spikes.

4. **Public API smoke (staging only, buyer JWT):**
   - `GET /api/v1/freight-costs/analytics/overview` → 200 with `projection_version`
   - Carrier same routes → 403

5. **Benchmark sample:** Opportunities include `evidence.sample_size` and `currency_code`.

---

## 10. SLA guidance

| Tenant size | Expected rebuild (indicative) |
|-------------|------------------------------|
| Integration (~120 orders) | Sub-second |
| Production (10k orders) | Minutes — monitor `rebuild_duration_seconds` |
| Production (100k orders) | Run in controlled environment first; schedule maintenance window |

Full rebuild is **deterministic** — repeated runs produce identical projections (`TestFC22BRebuildIdempotent`).

---

## 11. Automated projection-loss drill (v2.2G.1)

| Type | Test | Pass criteria |
|------|------|---------------|
| **AUTOMATED_DR_TEST** | `TestFC22G1_FullProjectionLossAndRebuildRestoresBusinessState` | Pre/post `ComputeAnalyticsBusinessChecksum` equal; canonical ledger unchanged; opportunity IDs stable; pinned accessorial mapping restored |
| **FAILED_REBUILD** | `TestFC22G1_FailedRebuildDoesNotPublishPartialFreshState` | Injected failure; no partial publish; prior checksum preserved |
| **RETRY** | `TestFC22G1_RetryAfterFailureRestoresBusinessState` | Second rebuild succeeds |

Derived tables cleared in drill: order facts, period/lane/carrier/accessorial facts & periods, benchmark, opportunity, coverage, projection state.

**Not sufficient alone:** `TestFC22G_FullStackRebuildIncrementalEquivalence` (incremental equivalence only).

**References:** `PERFORMANCE_REPORT.md`, `TEST_INVENTORY.md`, `ARCHITECTURE.md` ADR-22-003.
