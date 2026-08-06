# Control Tower Staging Shadow Observation

Operational guide for prolonged shadow-mode observation on **real staging** before any primary-canary review. **Primary remains disabled** throughout this phase.

## 1. Scope

- Shadow-mode read-model vs full legacy aggregate comparison
- Historical projection rebuild for approved staging tenant cohort
- Same-group Kafka catch-up without offset reset
- Operational rollback and consumer-restart drills
- Provisional SLO collection (requires Ops/Product approval)

Out of scope: enabling `primary`, public rebuild HTTP routes, frontend activation UI, automatic rollback.

## 2. Environment prerequisites

| Requirement | Value |
|-------------|-------|
| Deployment | Docker Compose (`bintrans-staging` on Selectel VPS) or layered local shadow stack |
| Commit | Feature branch through `21cd301` (reviewed HEAD) or later observation branch |
| Migrations | Through **000019** (`make migrate-up`) |
| Mode | `CONTROL_TOWER_READ_MODEL_MODE=shadow` |
| Consumer | `CONTROL_TOWER_CONSUMER_ENABLED=true` |
| Outbox | `SHIPMENT_OUTBOX_ENABLED=true` |
| Auth | `AUTH_ENABLED=true` |
| Kafka topic | `shipment.status.v1` |
| Consumer group | `control-tower-shipment-status-v1` |

**Forbidden:** `CONTROL_TOWER_READ_MODEL_MODE=primary`

Remote staging reference: `docs/LOW_CODE_PILOT_WEEK3_STAGING_DEPLOY_RUNBOOK_V0.1.md` (manual VPS deploy, project `bintrans-staging`, API `127.0.0.1:18080`).

## 3. Cohort selection

Minimum recommended **12 tenants** across categories (use all approved staging tenants if fewer):

| Category | Min |
|----------|-----|
| Projection absent | 2 |
| Incomplete/gap projection | 2 |
| Multiple shipment statuses | 2 |
| High aggregate versions | 2 |
| Active event flow | 2 |
| Small/empty dataset | 1 |
| Large staging dataset | 1 |

Use aliases `STG-T01`… in reports. Store alias→tenant mapping in **protected** cohort manifest (`COHORT_MANIFEST` env). See `scripts/ops/cohort.manifest.example.json`.

## 4. Baseline

For each alias (authenticated, full legacy aggregate):

- legacy total / byStatus / complete
- read-model totals (internal comparison via Prometheus aggregate)
- comparison category
- per-partition consumer lag (via `rpk group describe`)
- dead-letter and offset-commit-error counters

Do **not** use page-limited legacy endpoints.

## 5. Historical rebuild procedure

Per tenant (one at a time until stable):

1. Gracefully pause consumer (`CONTROL_TOWER_CONSUMER_ENABLED=false`).
2. Record per-partition offsets (`committed`, `logEnd`, `lag`; `UNCOMMITTED` when absent).
3. Export → pipe → import (`CONFIRM_PROJECTION_REBUILD_IMPORT=true`) → **VALIDATED**.
4. Operational review (checksum, stage count, primary disabled).
5. Activate (`CONFIRM_PROJECTION_REBUILD_ACTIVATION=true`).
6. Verify committed offsets **unchanged** through activation.
7. Resume **same** consumer group (no offset reset).
8. Wait lag→0; verify shadow **MATCH** via gateway metrics.
9. Optional live event; confirm rollback window closes after live write.

## 6. Observation duration

**Provisional engineering gate: 7 consecutive calendar days** (minimum 5 business days), including:

- ≥1 consumer restart drill
- ≥1 planned backlog period
- ≥1 historical rebuild
- ≥1 rollback drill
- Active shipment traffic periods

Final duration requires Ops/Product sign-off.

## 7. Provisional SLO

**PROVISIONAL — REQUIRES OPS/PRODUCT APPROVAL**

| Metric | Threshold |
|--------|-----------|
| Cohort MATCH after convergence | 100% |
| Sustained mismatch | 0 longer than 5 min |
| Dead-letter delta | 0 |
| Offset commit errors | 0 |
| Steady-state consumer lag | 0 |
| Lag recovery after restart | ≤ 5 min |
| Read-model unavailable | 0 |
| Gateway 5xx regression | 0 |
| Incomplete projection after accepted rebuild | 0 |
| Read-model p95 vs legacy p95 | ≤ legacy p95 + 20% |

## 8. Metrics (actual names)

| Purpose | Metric | Labels |
|---------|--------|--------|
| MATCH | `control_tower_read_model_shadow_comparison_total` | `comparison="MATCH"`, `mode="shadow"` |
| Mismatch | `control_tower_read_model_shadow_comparison_total` | `comparison=~"TOTAL_MISMATCH\|STATUS_COUNT_MISMATCH"` |
| Legacy requests | `control_tower_legacy_status_aggregate_requests_total` | `mode="shadow"` |
| Read-model requests | `control_tower_read_model_requests_total` | `mode="shadow"` |
| Dead-letter | `control_tower_shipment_dead_letter_total` | — |
| Offset commit errors | `control_tower_shipment_consumer_offset_commit_errors_total` | — |
| Consumer lag | **No Prometheus metric** — use `rpk group describe` per partition |
| Incomplete projection | `control_tower_read_model_partial_response_total` | `mode="shadow"` |
| Legacy p95 | `control_tower_legacy_status_aggregate_duration_seconds` | histogram |
| Read-model p95 | `control_tower_read_model_request_duration_seconds` | histogram |

## 9. Alerts

Provisional rules: `infrastructure/monitoring/prometheus/control_tower_shadow_observation_alerts.provisional.yml`

Copy into Prometheus only after review. Alerts **must not** auto-enable primary or execute rollback.

Stuck rebuild jobs (`IMPORTING` > 30m, `ACTIVATING`/`ROLLING_BACK` > 5m): monitor via SQL preflight until dedicated metrics exist.

## 10. Rollback drill

Use `make control-tower-shadow-observation-rollback-drill` (wraps live tenant-B acceptance against staging/local stack).

Verify: old projection restored field-by-field; inbox/dead-letter/offsets unchanged; repeat rollback idempotent.

## 11. Consumer restart drill

Use `make control-tower-shadow-observation-consumer-restart-drill`.

Verify: no offset reset; lag grows while paused; lag→0 after resume; MATCH preserved.

## 12. Security review

Checklist: CLI not exposed via HTTP; confirmations required; secrets via env/vault; JWT ephemeral; cohort manifest protected; logs without UUIDs/payloads; no primary; staging RBAC limited.

## 13. DBA review

Review migrations 000016–000018; stage/backup growth; activation transaction duration; advisory lock duration; WAL/replication; backup retention; down-migration limitations (000018 fails with NULL `last_event_type`).

## 14. Failure policy

Immediate FAIL: data loss, offset reset, inbox/dead-letter deletion, projection regression from stale events, rollback after live write, public source≠LEGACY, primary enabled, persistent mismatch, unknown status silently excluded.

On FAIL: stop activations; keep public source LEGACY; preserve evidence; no automatic rollback.

## 15. Exit criteria

Observation PASS only when: approved duration complete; cohort MATCH; no sustained mismatches; no dead-letter/offset errors; lag recovery within threshold; rollback drill PASS; live-write rollback refusal PASS; security/DBA/ops runbook approved; **primary still disabled**.

## 16. Primary remains disabled

This phase ends with **PRIMARY_READINESS_BLOCKED** or **PRIMARY_CANARY_REVIEW_ELIGIBLE** — never **PRIMARY_ENABLED**. Canary requires separate plan (tenant allowlist, kill switch, automatic legacy fallback, multi-team approval).

## Automation

```bash
export COHORT_MANIFEST=/protected/path/cohort.json
export GATEWAY_URL=...
export READ_MODEL_URL=...
export PROMETHEUS_URL=...
export JWT_TOKEN=...   # or TENANT_ID+DEV_ADMIN_EMAIL+DEV_ADMIN_PASSWORD

make control-tower-shadow-observation-preflight
make control-tower-shadow-observation-snapshot
make control-tower-shadow-observation-gate
```

Reports are written to `OBSERVATION_OUTPUT` (default stdout). **Do not commit runtime reports to Git.**
