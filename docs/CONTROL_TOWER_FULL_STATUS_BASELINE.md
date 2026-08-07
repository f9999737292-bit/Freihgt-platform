# Control Tower Full Legacy Status Baseline v0.1

## Purpose

Establish a tenant-scoped **full legacy status aggregate** in `shipment-service` and wire API Gateway to use it as the primary shadow-comparison and primary-fallback baseline, replacing page-limited shipment-list counts when the aggregate is available.

This document is the v0.1 baseline for rollout acceptance, observability, and integration testing. It complements [CONTROL_TOWER_READ_MODEL_ROLLOUT.md](./CONTROL_TOWER_READ_MODEL_ROLLOUT.md).

## Mandatory assertions

The following three assertions are non-negotiable invariants for Control Tower legacy baseline v0.1:

> The full legacy shipment status aggregate is calculated directly from the tenant-scoped shipment source-of-truth table and does not depend on a paginated shipment list.

> Shadow comparisons are considered authoritative only when the full legacy aggregate is available.

> A page-limited legacy summary remains a last-resort fallback and is always marked as limited and partial.

Additional rollout invariants (read-model):

> The Control Tower read model is an eventually consistent projection and is not the transactional source of truth.

> Shadow mode never changes the user-facing Control Tower response.

> If the read-model dependency is unavailable, API Gateway falls back to the existing status source without exposing internal dependency details.

## 1. Transactional source of truth

- **Authoritative:** `shipment-service` and `transport.shipments` remain the transactional source of truth for shipment state.
- **Read-model:** `control-tower-read-model-service` is an eventually consistent Kafka projection consumed for Control Tower status summary only during rollout.
- **Full legacy aggregate:** derived from the same PostgreSQL table as mutations; it is the correct legacy baseline for shadow comparison but is not a replacement for the read-model in primary mode.

## 2. Full legacy aggregate endpoint

Internal endpoint in `shipment-service`:

```http
GET /internal/v1/shipments/status-summary
```

Headers:

- `X-Tenant-ID` (required, trusted gateway context)
- `X-Request-ID` (optional, propagated for correlation)

Response fields:

| Field | Type | Description |
|-------|------|-------------|
| `totalShipments` | int64 | Tenant total non-deleted shipments |
| `countedShipments` | int64 | Sum of `byStatus` counts included in this response |
| `byStatus` | map[string]int64 | Count per shipment status |
| `complete` | bool | `true` when aggregate is safe for exact comparison |
| `calculatedAt` | RFC3339 | Aggregate calculation timestamp (UTC) |
| `warnings` | string[] | Optional; e.g. `UNKNOWN_SHIPMENT_STATUS` when invalid statuses are skipped |

Not exposed via public `/api/v1/...`. Called only by API Gateway over the internal service network.

## 3. SQL aggregate semantics

Repository query (`ShipmentStatusSummaryRepository`):

```sql
SELECT status, COUNT(*)::BIGINT, SUM(COUNT(*)) OVER ()::BIGINT
FROM transport.shipments
WHERE tenant_id = $1 AND deleted_at IS NULL
GROUP BY status
ORDER BY status
```

- Tenant isolation enforced in SQL (`tenant_id = $1`).
- Soft-deleted rows excluded (`deleted_at IS NULL`).
- Unknown/invalid status values are skipped in service layer; response may set `complete=false` with warning `UNKNOWN_SHIPMENT_STATUS`.

## 4. Complete aggregate contract

Gateway legacy aggregate client accepts the response only when:

1. HTTP 2xx
2. JSON parses and passes contract validation
3. `complete=true`
4. `totalShipments == countedShipments`
5. Sum of `byStatus` values equals `countedShipments`
6. No negative counts

Incomplete or malformed aggregates are treated as dependency failures; gateway falls back to page-limited counts.

## 5. Page-limited legacy tier (secondary)

When the full aggregate is unavailable, gateway derives status counts from the same tenant-scoped `GET /v1/shipments` list fetch already used for Control Tower KPI rows:

- `totalShipments` = shipment-service list total
- `countedShipments` = number of fetched rows
- `byStatus` = counts from fetched rows only
- `limitedDataset=true` when `totalShipments > countedShipments`

Page-limited summaries emit warning `CONTROL_TOWER_LEGACY_STATUS_SUMMARY_LIMITED`.

## 6. Fallback hierarchy

API Gateway resolves legacy status input in order:

```text
1. FULL_AGGREGATE  — GET /internal/v1/shipments/status-summary (complete=true)
2. PAGE_LIMITED    — counts from fetched shipment list page
```

| Tier | Trigger | `FullAggregateAvailable` | `LimitedDataset` | Metric `fallback_level` |
|------|---------|--------------------------|------------------|-------------------------|
| `FULL_AGGREGATE` | Aggregate HTTP success + `complete=true` | `true` | `false` | `FULL_AGGREGATE` |
| `PAGE_LIMITED` | Aggregate client nil/unavailable/error/incomplete | `false` | per page math | `PAGE_LIMITED` |

Primary read-model fallback preserves the tier that supplied legacy counts. Read-model primary success never inherits the page-limited marker.

## 7. Gateway legacy aggregate client

Package: `services/api-gateway/internal/controltower/legacyaggregate/`

- Base URL: `SHIPMENT_SERVICE_URL`
- Path: `/internal/v1/shipments/status-summary`
- Timeout: `CONTROL_TOWER_LEGACY_STATUS_TIMEOUT` (default `800ms`)
- Max response body: 256 KiB
- Forwards `X-Tenant-ID` and `X-Request-ID` only (no bearer token)

Wiring: `resolveLegacyStatusInput` in `legacy_status.go` calls the aggregate client after the shipment list fetch; page-limited values remain the fallback input.

## 8. Shadow comparison baseline

Shadow mode compares read-model status summary against **full legacy aggregate** when available:

- Exact comparison enabled when `FullAggregateAvailable=true` and `LimitedDataset=false`
- Comparison disabled with `LEGACY_LIMITED_DATASET` when only page-limited legacy is available
- Comparison disabled with `LEGACY_FULL_AGGREGATE_UNAVAILABLE` when neither tier yields usable legacy input
- User-facing response remains legacy-derived in shadow mode (unchanged vs disabled)

Comparison results:

| Result | Meaning |
|--------|---------|
| `MATCH` | Totals and all status counts equal |
| `TOTAL_MISMATCH` | `totalShipments` differs |
| `STATUS_COUNT_MISMATCH` | Per-status count differs |
| `LEGACY_LIMITED_DATASET` | Page-limited legacy; comparison skipped |
| `LEGACY_FULL_AGGREGATE_UNAVAILABLE` | No usable legacy baseline |
| `LEGACY_UNAVAILABLE` | Legacy input invalid |
| `READ_MODEL_UNAVAILABLE` | Read-model call failed |
| `READ_MODEL_NOT_RUNNING` | Consumer not running when required |

## 9. Primary mode behavior

- Success: `statusSummary.source=READ_MODEL`, `limitedDataset=false`, `countedShipments=totalShipments`
- Fallback: legacy tier per hierarchy above; `statusSummaryFreshness.fallbackUsed=true`
- Partial projection (`incompleteProjections > 0`): use read-model with `CONTROL_TOWER_READ_MODEL_PARTIAL`; no silent fallback solely for version gaps
- `statusSummaryFreshness.legacyAggregateLoaded` reflects whether full aggregate was loaded for the request (even when primary serves read-model)

## 10. Disabled mode behavior

When `CONTROL_TOWER_READ_MODEL_MODE=disabled`:

- Status summary sourced from full aggregate when available, else page-limited legacy
- No read-model HTTP call
- Same limited-dataset warnings and freshness markers as shadow/primary legacy paths

## 11. Tenant isolation

- Verified tenant from gateway JWT `AuthContext` only
- Aggregate endpoint receives gateway-set `X-Tenant-ID`; spoofed browser headers stripped
- Missing tenant → `401` on shipment-service handler before repository access

## 12. Timeout and non-blocking behavior

| Dependency | Config | Default | Blocks summary? |
|------------|--------|---------|-----------------|
| Legacy aggregate | `CONTROL_TOWER_LEGACY_STATUS_TIMEOUT` | `800ms` | No; falls back to page-limited |
| Read-model | `CONTROL_TOWER_READ_MODEL_TIMEOUT` | `800ms` | No in shadow; primary falls back to legacy |

Neither dependency blocks gateway startup or `/ready`.

## 13. Response provenance (`statusSummaryFreshness`)

| Field | Full aggregate path | Page-limited path | Primary read-model success |
|-------|---------------------|-------------------|----------------------------|
| `source` | `LEGACY` | `LEGACY` | `READ_MODEL` |
| `legacyAggregateLoaded` | `true` | `false` | omitted / prior resolve |
| `partial` | `false` | `true` | true when incomplete projections |
| `fallbackUsed` | `false` | `false` | `true` on read-model failure |

## 14. Gateway metrics

Read-model (existing):

- `control_tower_read_model_requests_total{mode,result,reason}`
- `control_tower_read_model_request_duration_seconds{mode,result,reason}`
- `control_tower_read_model_fallback_total{mode,reason}`
- `control_tower_read_model_shadow_comparison_total{mode,comparison}`
- `control_tower_read_model_partial_response_total{mode}`

Legacy full aggregate (new):

- `control_tower_legacy_status_aggregate_requests_total{result,reason,mode}`
- `control_tower_legacy_status_aggregate_duration_seconds{result,reason,mode}`
- `control_tower_legacy_status_aggregate_errors_total{result,error_code}`
- `control_tower_legacy_status_fallback_total{mode,fallback_level,reason}`

No tenant/user/shipment/request_id labels on any Control Tower metric.

## 15. Shipment-service metrics

- `shipment_status_summary_requests_total{result,error_code}`
- `shipment_status_summary_query_duration_seconds{result,error_code}`
- `shipment_status_summary_errors_total{result,error_code}`

## Incomplete full aggregate

A full aggregate is authoritative only when `complete=true`, `countedShipments` equals `totalShipments`, and all statuses are recognized.

When shipment-service returns `complete=false` (for example due to `UNKNOWN_SHIPMENT_STATUS`):

- `totalShipments` reflects all non-deleted tenant shipments from the SQL window total
- `countedShipments` includes only recognized statuses in `byStatus`
- Gateway rejects the response via `ValidateCompleteLegacyAggregate` and falls back to page-limited legacy
- Shadow exact comparison is blocked (`LEGACY_FULL_AGGREGATE_INCOMPLETE`)
- Metrics record bounded reason `FULL_LEGACY_AGGREGATE_INCOMPLETE`

## Unknown status

An aggregate containing unrecognized shipment statuses is marked incomplete and is not used for authoritative shadow comparison. Unknown raw status values are never exposed in public `byStatus`, warnings beyond the bounded code, or logs.

## Black-box isolation

Integration tests use:

- **Unique binaries:** cached build per service in OS temp, copied into `t.TempDir()` per test
- **Unique ports:** `127.0.0.1:0` reservation per child process
- **Temporary databases:** `freight_platform_full_baseline_test_<random>` with force-drop cleanup
- **Readiness:** bounded HTTP polling on `/health` (or service-specific ready path), early fail if child exits
- **Cleanup:** `t.Cleanup` kills child process, waits bounded, then drops database

Makefile target `control-tower-full-status-baseline-integration-test` runs with `-p=1` because parallel child-process suites previously flaked when multiple tests concurrently invoked `go build` and startup exceeded readiness timeout. A separate `control-tower-full-status-baseline-integration-test-parallel` target validates `-p=2` after binary cache isolation.

## Query-plan evidence

Live integration test `TestStatusSummaryQueryPlanExplain` seeds a synthetic dataset (default 20,000 rows, configurable via `STATUS_SUMMARY_EXPLAIN_ROW_COUNT`) in a temporary database and runs `EXPLAIN (ANALYZE, BUFFERS)`. Plan output is logged to test output only (not committed). Composite partial index `(tenant_id, status) WHERE deleted_at IS NULL` is deferred until production-like volume review.

## 16. Index decision placeholder

**Decision deferred to post-v0.1 performance review.**

Current indexes on `transport.shipments` include single-column indexes on `tenant_id` and `status` (migration `000003`). The aggregate query filters `tenant_id` + `deleted_at IS NULL` and groups by `status`.

Candidate indexes for evaluation (not applied in v0.1):

| Option | Definition | Rationale |
|--------|------------|-----------|
| A | `(tenant_id) WHERE deleted_at IS NULL` | Partial index matching aggregate filter |
| B | `(tenant_id, status) WHERE deleted_at IS NULL` | Covering index for GROUP BY status |
| C | No change | Rely on existing `idx_shipments_tenant_id` if EXPLAIN shows acceptable plans at tenant scale |

Acceptance gate: run `EXPLAIN (ANALYZE, BUFFERS)` on staging tenant sizes before promoting composite/partial index migration.

## 17. Failure matrix

### Shipment-service aggregate

| Condition | HTTP | Gateway effect |
|-----------|------|----------------|
| Missing `X-Tenant-ID` | 401 | Fallback to page-limited |
| DB unavailable | 500 | Fallback to page-limited |
| Unknown status in rows | 200, `complete=false` | Fallback to page-limited |
| `countedShipments != totalShipments` | 200, `complete=false` | Fallback to page-limited |
| Timeout | — | Fallback to page-limited |
| Malformed JSON / oversize body | — | Fallback to page-limited |

### Gateway legacy aggregate client failure reasons

`TIMEOUT`, `NETWORK_ERROR`, `NON_2XX`, `MALFORMED_RESPONSE`, `INVALID_CONTRACT`, `FULL_LEGACY_AGGREGATE_INCOMPLETE`, `CANCELLED`, `UNKNOWN`

### Read-model (unchanged from rollout doc)

Primary falls back to legacy tier; shadow keeps legacy response and records comparison unavailable/mismatch metrics.

## 18. Integration test

Makefile target:

```bash
make control-tower-full-status-baseline-integration-test
```

Runs:

```bash
cd services/api-gateway && go test -tags=integration ./internal/integration/controltowerreadmodel/... \
  -run "FullBaseline|BlackBox.*Aggregate|ShadowMatch" -count=1 -p=1 -v
```

Covers full aggregate HTTP contract, gateway client wiring, shadow exact match when aggregate available, and page-limited comparison skip.

## 19. Operational acceptance

Before promoting shadow → primary with full baseline:

1. `control_tower_legacy_status_aggregate_requests_total{result="success"}` stable in staging
2. `control_tower_legacy_status_fallback_total{fallback_level="PAGE_LIMITED"}` near zero when shipment-service healthy
3. Shadow `comparison=MATCH` rate meets team SLO after consumer catch-up
4. `LEGACY_LIMITED_DATASET` only during aggregate outages or incomplete responses
5. Tenant isolation verified on aggregate endpoint
6. Control Tower p95 latency within budget with aggregate + read-model calls

## 20. v0.1 limitations

- Status summary only; no per-shipment or bulk list from read-model in gateway
- Aggregate query scans tenant shipment population (index optimization deferred — see §16)
- No circuit breaker or retry storm protection on aggregate client beyond single timeout
- Shadow mismatches are metrics/logs only; not exposed in public API
- Page-limited tier remains mandatory fallback; cannot be disabled
- No admin UI for baseline health
- `UNKNOWN_SHIPMENT_STATUS` rows excluded from counts until data remediated

## Related documents

- [CONTROL_TOWER_READ_MODEL_ROLLOUT.md](./CONTROL_TOWER_READ_MODEL_ROLLOUT.md)
- [CONTROL_TOWER_SHIPMENT_STATUS_READ_MODEL.md](./CONTROL_TOWER_SHIPMENT_STATUS_READ_MODEL.md)
- [CONTROL_TOWER.md](./CONTROL_TOWER.md)
