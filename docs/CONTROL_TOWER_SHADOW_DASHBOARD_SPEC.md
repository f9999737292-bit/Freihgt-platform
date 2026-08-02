# Control Tower Shadow Observation — Dashboard Specification

Provisional Grafana dashboard for Selectel staging observation. Create or update dashboard JSON from this specification after Ops approval.

**Runbook:** `docs/CONTROL_TOWER_STAGING_SHADOW_OBSERVATION.md`

---

## Panel 1 — Public source

- **Query:** Gateway metric or log-derived indicator of public response source
- **Expected:** `LEGACY` throughout observation
- **Alert linkage:** PrimaryModeActivityDetected

## Panel 2 — MATCH / mismatch ratio

- **Metric:** `control_tower_read_model_shadow_comparison_total{mode="shadow",comparison=~".*"}`
- **Views:** rate by `comparison`; ratio MATCH vs mismatch types
- **Alert linkage:** ShadowMismatchSustained

## Panel 3 — Incomplete projection

- **Metric:** `control_tower_read_model_partial_response_total{mode="shadow"}`
- **Alert linkage:** ProjectionIncompleteAfterRebuild

## Panel 4 — Legacy requests

- **Metric:** `control_tower_legacy_status_aggregate_requests_total`

## Panel 5 — Read-model shadow requests

- **Metric:** `control_tower_read_model_requests_total{mode="shadow"}`

## Panel 6 — Consumer lag by partition

- **Source:** `rpk group describe control-tower-shipment-status-v1` (manual or scripted export)
- **Note:** No native Prometheus consumer lag metric; use external text panel or scripted datasource

## Panel 7 — Offset commit errors

- **Metric:** `control_tower_shipment_consumer_offset_commit_errors_total`
- **Alert linkage:** OffsetCommitErrors

## Panel 8 — Dead-letter growth

- **Metric:** `control_tower_shipment_dead_letter_total`
- **Alert linkage:** DeadLetterGrowth

## Panel 9 — Gateway errors

- **Metric:** `http_requests_total{service="api-gateway",status=~"5.."}` 
- **Alert linkage:** GatewayErrorRegression

## Panel 10 — Legacy / read-model latency

- **Metrics:** histogram `_bucket` series for legacy and read-model shadow paths
- **SLO reference:** read-model p95 ≤ legacy + 20%

## Panel 11 — Jobs by state

- **Source:** SQL query or exported count from `control_tower.shipment_status_projection_rebuild_job`
- **Highlight:** IMPORTING, ACTIVATING, ROLLING_BACK

## Panel 12 — Activation / rollback outcomes

- **Source:** job state transitions over time (SQL or ops log)
- **No automatic actions** from dashboard

## Panel 13 — Lock timeouts

- **Source:** application logs filtered for advisory lock timeout
- **No tenant/snapshot/shipment IDs in panel titles or labels**

## Panel 14 — Backup / stage row growth

- **Source:** SQL counts on `shipment_status_projection_rebuild_stage` and `_backup`
- **Trend over observation window**

---

## Dashboard constraints

- Do not include tenant IDs, snapshot IDs, shipment IDs, or Kafka payloads in labels
- Dashboard must not trigger primary enablement, shadow disable, activation, rollback, or job cleanup
- Provisional alert rules: `infrastructure/monitoring/prometheus/control_tower_shadow_observation_alerts.provisional.yml`

---

## Status

Dashboard JSON: specification only (create in Grafana after staging deploy).

Observation window: **NOT STARTED**
