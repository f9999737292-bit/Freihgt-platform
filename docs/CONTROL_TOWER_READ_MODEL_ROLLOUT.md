# Control Tower Read-Model Gateway Rollout v0.1

## Goal

Connect API Gateway Control Tower summary to the shipment status read-model in a safe, phased way without replacing transactional sources or exposing internal infrastructure to browsers.

## Source of truth

- **Transactional source of truth:** `shipment-service` remains authoritative for shipment state and lifecycle.
- **Read-model:** eventually consistent projection consumed from Kafka by `control-tower-read-model-service`.

> The Control Tower read model is an eventually consistent projection and is not the transactional source of truth.

## Status-only scope (v0.1)

Integrated endpoint:

```http
GET /internal/v1/control-tower/status-summary
```

Public BFF block:

```json
{
  "statusSummary": { "...": "..." },
  "statusSummaryFreshness": { "...": "..." }
}
```

**Not in v0.1:**

- `GET /internal/v1/control-tower/shipments/{shipmentId}/status`
- `GET /internal/v1/control-tower/shipments/statuses`

Active shipment rows, SLA KPIs, critical events, ETA, and financial KPIs continue to use existing legacy aggregation paths.

## Rollout modes

| Mode | Behavior |
|------|----------|
| `disabled` (default) | Legacy aggregation only; no read-model HTTP call |
| `shadow` | Legacy response + bounded read-model call + comparison metrics/logs |
| `primary` | Legacy builds non-status fields; read-model supplies `statusSummary`; mandatory legacy fallback on dependency failure |

> Shadow mode never changes the user-facing Control Tower response.

> Primary mode replaces only the status-summary portion owned by the projection. All other Control Tower fields continue to use their existing sources.

## Legacy fallback

When read-model is unavailable, malformed, timed out, or consumer-not-running (if required), gateway uses legacy status counts derived from the same tenant-scoped shipment-service list fetch used by Control Tower aggregation.

> If the read-model dependency is unavailable, API Gateway falls back to the existing status source without exposing internal dependency details.

Fallback cannot be disabled in v0.1.

## Full legacy aggregate baseline

API Gateway resolves legacy status input through a two-tier hierarchy documented in [CONTROL_TOWER_FULL_STATUS_BASELINE.md](./CONTROL_TOWER_FULL_STATUS_BASELINE.md):

1. **Primary comparison source:** `GET /internal/v1/shipments/status-summary` in shipment-service (full tenant aggregate, `complete=true`).
2. **Fallback comparison source:** page-limited counts from the tenant-scoped shipment list fetch used by Control Tower aggregation.

When the full aggregate succeeds, shadow comparison uses exact totals and per-status counts across the entire tenant population. Page-limited legacy is used only when the aggregate client is unavailable, times out, returns incomplete data, or fails contract validation.

Gateway exposes `statusSummaryFreshness.legacyAggregateLoaded=true` when the full aggregate supplied the legacy baseline for that request.

## Legacy fallback completeness

### Full aggregate path (preferred)

- `FullAggregateAvailable=true`, `LimitedDataset=false`
- Shadow exact comparison enabled (`MATCH`, `TOTAL_MISMATCH`, `STATUS_COUNT_MISMATCH`)
- No `CONTROL_TOWER_LEGACY_STATUS_SUMMARY_LIMITED` warning

### Page-limited path (secondary)

Legacy status summary is derived from the same tenant-scoped shipment-service list fetch used by Control Tower aggregation.

1. Legacy summary may be **page-limited** when `totalShipments > countedShipments`.
2. `totalShipments` reflects the shipment-service total for the tenant scope.
3. `byStatus` counts apply only to `countedShipments` (fetched rows), not the full population.
4. Limited legacy summaries are always marked `partial=true`, `limitedDataset=true`, with warning `CONTROL_TOWER_LEGACY_STATUS_SUMMARY_LIMITED`.
5. Shadow exact comparison is disabled when `limitedDataset=true`; comparison result is `LEGACY_LIMITED_DATASET`.
6. Primary fallback preserves the limited marker and warnings when legacy data is page-limited.
7. Read-model primary success does **not** inherit the legacy limited marker (`limitedDataset=false`, `countedShipments=totalShipments`).
8. Rollout acceptance must not treat shadow mismatches caused by limited legacy population as read-model defects when the full aggregate was unavailable.
9. When the full aggregate is unavailable entirely, comparison result is `LEGACY_FULL_AGGREGATE_UNAVAILABLE`.

## Partial projection semantics

If `incompleteProjections > 0`, the read-model response is usable but partial:

- **Primary:** use read-model counts, set `partial=true`, emit `CONTROL_TOWER_READ_MODEL_PARTIAL`
- **No fallback** solely because of version gaps (fallback would hide projection gaps)

## Consumer-not-running semantics

When `CONTROL_TOWER_READ_MODEL_REQUIRE_CONSUMER_RUNNING=true` and `freshness.consumerRunning=false`:

- **Shadow:** legacy response; comparison marked unavailable; metric incremented
- **Primary:** legacy fallback + `CONTROL_TOWER_READ_MODEL_CONSUMER_NOT_RUNNING`

## Tenant isolation

- Verified tenant comes only from gateway JWT `AuthContext`
- Read-model receives `X-Tenant-ID` and `X-Request-ID` only
- Spoofed browser `X-Tenant-ID` is stripped and never forwarded

## RBAC ordering

```text
JWT authentication
→ StripUntrustedIdentityHeaders
→ verified AuthContext
→ Control Tower RBAC
→ legacy/read-model downstream calls
```

Forbidden role → `403` (no legacy, no read-model). Missing JWT → `401`.

## Timeout behavior

- Configurable via `CONTROL_TOWER_READ_MODEL_TIMEOUT` (default `800ms`)
- Shadow read-model call uses bounded timeout and does not block legacy response indefinitely
- Read-model is **not** a startup/readiness dependency

## Response provenance

| Field | Source in primary success | Source in fallback |
|-------|---------------------------|--------------------|
| `statusSummary.totalShipments` | read-model | legacy shipment total |
| `statusSummary.byStatus` | read-model | legacy counts from fetched rows |
| `statusSummary.incompleteProjections` | read-model | `0` (legacy has no equivalent) |
| `statusSummary.source` | `READ_MODEL` | `LEGACY` |
| `kpi`, `shipments`, `criticalEvents`, `filters` | legacy | legacy |

## Warnings

Stable public warning codes:

- `CONTROL_TOWER_READ_MODEL_UNAVAILABLE`
- `CONTROL_TOWER_READ_MODEL_CONSUMER_NOT_RUNNING`
- `CONTROL_TOWER_READ_MODEL_PARTIAL`
- `CONTROL_TOWER_READ_MODEL_FALLBACK_USED`

Shadow mismatches are observability-only in v0.1 (metrics/logs), not user-facing mismatch details.

## Metrics

Gateway Prometheus metrics:

- `control_tower_read_model_requests_total{mode,result,reason}`
- `control_tower_read_model_request_duration_seconds{mode,result,reason}`
- `control_tower_read_model_fallback_total{mode,reason}`
- `control_tower_read_model_shadow_comparison_total{mode,comparison}`
- `control_tower_read_model_partial_response_total{mode}`

No tenant/user/shipment/request_id/base_url labels.

## Logging

Structured fields: `mode`, `result`, `reason`, `comparison`, `duration`, `fallback_used`, `partial`, `request_id`.

Never log tenant ID, JWT, response body, or internal URLs with credentials.

## Rollback procedure

Set:

```text
CONTROL_TOWER_READ_MODEL_MODE=disabled
```

Restart API Gateway. No database migration or Kafka consumer rollback required.

## Operational checklist

1. Confirm read-model consumer healthy (`consumerRunning=true`)
2. Enable `mode=shadow` in staging
3. Monitor request success rate, timeout rate, shadow comparison results
4. Investigate mismatches and incomplete projection counts
5. Confirm tenant isolation and Control Tower latency
6. Promote to `mode=primary` with fallback still enabled

## Shadow acceptance criteria

- User response unchanged vs disabled mode
- Comparison metrics emitted for match/mismatch/unavailable
- Read-model failures do not break Control Tower summary
- No bearer token forwarded to read-model

## Primary acceptance criteria

- `statusSummary.source=READ_MODEL` on success
- Legacy KPI/shipments/events unchanged
- Fallback on timeout/5xx/malformed/consumer-not-running
- Partial projection uses read-model with warning (no silent fallback)

## v0.1 limitations

- Status summary only (no detail/list endpoints)
- Legacy status counts may reflect limited fetch window when total > fetched rows
- No admin UI for read-model rollout
- No circuit breaker or infinite retry in gateway
- No Kafka access from gateway

## Detail/list rollout TODO

- Per-shipment status from projection
- Bulk status list with cursor semantics aligned to active shipments table
- Strategy for fields absent from projection

## Projection rebuild TODO

- Operational runbook for full tenant rebuild
- Backfill from shipment-service or event replay

## Dead-letter replay TODO

- DLQ inspection and safe replay tooling
- Metrics/alerts for DLQ growth

## Recommended rollout phases

### Phase 1 — Disabled

```text
CONTROL_TOWER_READ_MODEL_MODE=disabled
```

Verify no behavior change.

### Phase 2 — Shadow

```text
CONTROL_TOWER_READ_MODEL_MODE=shadow
```

Transition to primary when operational thresholds are met (success rate, timeout rate, mismatch investigation, consumer stability, tenant isolation, latency). Use team-defined SLO thresholds; no hard-coded product numbers in this document.

### Phase 3 — Primary

```text
CONTROL_TOWER_READ_MODEL_MODE=primary
```

Fallback remains mandatory.
