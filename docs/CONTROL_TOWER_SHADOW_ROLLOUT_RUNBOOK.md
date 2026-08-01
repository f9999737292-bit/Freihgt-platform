# Control Tower Read Model Shadow Rollout Runbook v0.1

## Scope

This runbook covers a **controlled shadow rollout** of the Control Tower read-model in staging or an isolated local environment that mirrors staging topology.

**Shadow mode never replaces the user-facing status summary. The authoritative full legacy aggregate remains the response source while comparison metrics are collected.**

In shadow mode:

- User response = full legacy status aggregate (`statusSummary.source = LEGACY`)
- Read-model = parallel comparison and observability only
- Mismatches are recorded in metrics/logs, never shown to end users
- `primary` mode is **not** enabled in v0.1

Target event chain:

```text
shipment mutation
→ PostgreSQL transactional outbox
→ Kafka shipment.status.v1
→ Control Tower read-model consumer
→ status projection

Control Tower request
→ full legacy status aggregate
→ read-model status aggregate
→ shadow comparison
→ metrics/logging
→ user receives legacy response
```

## Preconditions

1. Branch `feat/control-tower-shadow-rollout-v0.1` (or equivalent) based on Kafka outbox reliability fixes.
2. Migration `000015_create_control_tower_shipment_status_projection_v0.1` applied.
3. Kafka topic `shipment.status.v1` exists with known partition count.
4. Shipment-service outbox publisher operational.
5. API Gateway base compose default: `CONTROL_TOWER_READ_MODEL_MODE=disabled`.
6. Staging shadow override available: `infrastructure/docker-compose/docker-compose.staging-shadow.yml`.
7. Prometheus/Grafana observability profile available (optional but recommended).
8. JWT/RBAC credentials available via environment only (never committed).

## Required services

| Service | Purpose |
|---------|---------|
| PostgreSQL | Outbox, projection tables (`control_tower` schema) |
| Redpanda/Kafka | `shipment.status.v1` topic |
| shipment-service | Authoritative shipments + outbox publisher |
| api-gateway | Control Tower summary, shadow comparison |
| control-tower-read-model-service | Kafka consumer + internal status summary API |
| Prometheus | Metrics scrape + recording rules |
| Grafana | Shadow rollout dashboard (optional) |

## Migration check

Confirm migration `000015` is applied:

```sql
SELECT schema_name FROM information_schema.schemata WHERE schema_name = 'control_tower';
SELECT tablename FROM pg_tables WHERE schemaname = 'control_tower';
```

Expected objects:

- Schema `control_tower`
- Inbox table
- Projection table (`shipment_status_projection` or equivalent)
- Dead-letter table
- Constraints and indexes per migration file

**Do not** run destructive down migrations or manual backfill during shadow rollout.

## Kafka check

Verify:

- Topic `shipment.status.v1` exists
- Partition count documented
- Producer (shipment outbox) can publish
- Consumer group `control-tower-shipment-status-v1` has access
- TLS/SASL config matches staging secrets (via ENV only)
- Auto-commit disabled in consumer
- API Gateway has **no** Kafka credentials

Local init (if needed):

```bash
make messaging-up
make shipment-kafka-topic-create
```

## Consumer check

After read-model starts:

```bash
curl -fsS http://127.0.0.1:8089/health
curl -fsS http://127.0.0.1:8089/ready
```

Expected:

- `/health` → process alive
- `/ready` → PostgreSQL available
- Internal summary metadata: `consumerRunning=true` when consumer enabled
- Kafka outage does **not** fail PostgreSQL readiness
- Consumer failures visible in metrics/logs

Record (aggregated only):

- `lastRecordReceivedAt`
- `lastProjectionAppliedAt`
- `consumerRunning`

## Full baseline check

Before trusting shadow comparison, verify full legacy aggregate:

```text
complete=true
countedShipments=totalShipments
sum(byStatus)=countedShipments
all statuses recognized
```

If aggregate is unavailable or incomplete, comparison must be:

- `LEGACY_FULL_AGGREGATE_UNAVAILABLE`
- `LEGACY_FULL_AGGREGATE_INCOMPLETE`

Page-limited legacy fallback must **not** produce `MATCH` or ordinary mismatch comparisons.

## Shadow enable procedure

1. Ensure base compose defaults remain `disabled`:

   ```bash
   docker compose -f infrastructure/docker-compose/docker-compose.yml config | grep CONTROL_TOWER_READ_MODEL_MODE
   ```

2. Apply staging shadow override:

   ```bash
   docker compose \
     -f infrastructure/docker-compose/docker-compose.yml \
     -f infrastructure/docker-compose/docker-compose.staging-shadow.yml \
     --profile messaging \
     --profile read-model \
     --profile observability \
     up -d
   ```

3. Validate merged config:

   ```bash
   make control-tower-shadow-rollout-config-check
   ```

4. Confirm gateway env:

   ```text
   CONTROL_TOWER_READ_MODEL_MODE=shadow
   CONTROL_TOWER_READ_MODEL_BASE_URL=http://control-tower-read-model-service:8089
   CONTROL_TOWER_READ_MODEL_TIMEOUT=800ms
   CONTROL_TOWER_READ_MODEL_REQUIRE_CONSUMER_RUNNING=true
   ```

5. Confirm read-model consumer env:

   ```text
   CONTROL_TOWER_CONSUMER_ENABLED=true
   CONTROL_TOWER_KAFKA_TOPIC=shipment.status.v1
   CONTROL_TOWER_KAFKA_GROUP_ID=control-tower-shipment-status-v1
   ```

6. Restart gateway if already running (to pick up shadow mode).

## Smoke test

```bash
export JWT_TOKEN="<staging-jwt>"
export TENANT_ID="<test-tenant-uuid>"
make control-tower-shadow-rollout-smoke-test
make control-tower-shadow-rollout-metrics-check
```

Smoke verifies:

1. Gateway `/health`, `/ready`
2. Read-model `/health`, `/ready`
3. Consumer running metadata
4. Full legacy aggregate endpoint (when `TENANT_ID` set)
5. Control Tower summary returns `source=LEGACY`
6. Shadow comparison and read-model request metrics increase
7. No internal URLs in public response

## Metrics

### Shadow comparison

```promql
sum by (comparison) (
  rate(control_tower_read_model_shadow_comparison_total[5m])
)
```

Bounded comparison values: `MATCH`, `TOTAL_MISMATCH`, `STATUS_COUNT_MISMATCH`, `LEGACY_FULL_AGGREGATE_UNAVAILABLE`, `LEGACY_FULL_AGGREGATE_INCOMPLETE`, `LEGACY_LIMITED_DATASET`, `READ_MODEL_UNAVAILABLE`.

### Read-model requests

```promql
sum by (result, reason) (
  rate(control_tower_read_model_requests_total[5m])
)
```

### Legacy aggregate

```promql
sum by (result, reason) (
  rate(control_tower_legacy_status_aggregate_requests_total[5m])
)
```

### Read-model latency (p95)

```promql
histogram_quantile(
  0.95,
  sum by (le) (
    rate(control_tower_read_model_request_duration_seconds_bucket[5m])
  )
)
```

### Consumer errors

```promql
sum by (error_code) (
  rate(control_tower_shipment_consumer_errors_total[5m])
)
```

Recording rules: `infrastructure/monitoring/prometheus/control_tower_shadow_recording_rules.yml`

## Dashboard

Grafana dashboard: **Control Tower Read Model Shadow Rollout**

Path: `infrastructure/monitoring/grafana/provisioning/dashboards/control-tower-shadow-rollout.json`

Panels cover comparison results, match ratio, latency, consumer health, projection freshness, and gateway Control Tower latency.

## Expected MATCH

`MATCH` is expected only when:

- Full legacy aggregate is complete and authoritative
- Read-model projection reflects the same status counts for the tenant
- No version gaps causing incomplete projections for compared shipments

**Without historical backfill**, early shadow rollout may show mismatches or incomplete projections for tenants with pre-rollout shipment history. This is expected and **not** a gateway defect.

## Mismatch investigation

1. Confirm full legacy aggregate is complete (`complete=true`).
2. Check `comparison` label: `TOTAL_MISMATCH` vs `STATUS_COUNT_MISMATCH`.
3. Inspect read-model projection row counts and `incompleteProjections`.
4. Verify consumer is running and processing recent Kafka records.
5. Check dead-letter table for poison messages.
6. Do **not** expose mismatch details to users.
7. Do **not** enable `primary` to "fix" mismatches.

## Partial projection investigation

Version gaps (e.g. v1 and v3 without v2) produce:

- `incompleteProjections > 0`
- Possible `STATUS_COUNT_MISMATCH` until events converge
- Public response remains legacy

Investigate inbox/projection gap metrics; plan approved rebuild/replay separately.

## Dead-letter investigation

Query dead-letter count (aggregated). Inspect error codes in consumer metrics. Resolve root cause before considering primary rollout.

## Consumer-not-running investigation

When `consumerRunning=false`:

- Shadow comparison should reflect unavailable/not-running semantics
- Public response remains legacy
- Gateway readiness must **not** depend on read-model consumer
- Restart consumer or fix Kafka connectivity; do not reset offsets without approval

## Timeout investigation

When read-model exceeds `CONTROL_TOWER_READ_MODEL_TIMEOUT`:

- Legacy response still returned
- Metric reason = `TIMEOUT`
- No raw `context deadline exceeded` in user response
- Verify connection cleanup and no goroutine leak

## Tenant isolation verification

- Allowed role: summary returned; verified tenant passed to internal clients
- Forbidden role: HTTP 403; no legacy/read-model calls; no shadow metric increase
- Missing JWT: HTTP 401
- Spoofed `X-Tenant-ID`, `X-User-ID`, `X-User-Roles` must not affect trusted downstream tenant

## Rollback

Immediate gateway rollback:

```text
CONTROL_TOWER_READ_MODEL_MODE=disabled
```

Redeploy/restart gateway only. Rollback:

- Does **not** require stopping consumer
- Does **not** require Kafka offset reset
- Does **not** require database rollback
- Does **not** alter projection data
- Immediately stops shadow HTTP calls to read-model

Optional consumer disable (not required for gateway rollback):

```text
CONTROL_TOWER_CONSUMER_ENABLED=false
```

Verify rollback:

```bash
make control-tower-shadow-rollout-smoke-test  # with JWT; comparison metrics should not increase on repeated calls if mode=disabled
```

## Evidence checklist

- [ ] Environment type documented (staging vs local controlled)
- [ ] Commit SHA recorded
- [ ] Gateway mode = shadow
- [ ] Consumer enabled + group ID
- [ ] Migration 000015 confirmed
- [ ] Health/readiness OK
- [ ] Initial projection/inbox/dead-letter counts (aggregated)
- [ ] Shadow comparison results captured
- [ ] Controlled mismatch + convergence documented
- [ ] Latency measurements (disabled vs shadow)
- [ ] Rollback tested
- [ ] No credentials or tenant IDs in evidence artifacts

## Primary-mode blockers

Do **not** enable `primary` until all blockers resolved:

- [ ] Consumer stable and `consumerRunning=true`
- [ ] Full legacy aggregate consistently complete
- [ ] Read-model requests consistently successful
- [ ] Mismatch causes investigated and explained
- [ ] Projection gaps explained with rebuild strategy agreed
- [ ] Dead-letter backlog absent or remediated
- [ ] Offset commit errors investigated
- [ ] Tenant isolation confirmed
- [ ] Latency impact acceptable
- [ ] Projection rebuild/replay strategy approved
- [ ] Rollback tested

No fictitious numeric SLO thresholds in v0.1.

## Known risks

1. **No backfill**: projection may be incomplete for historical shipments; mismatches expected initially.
2. **Eventual consistency**: transient `STATUS_COUNT_MISMATCH` until Kafka events applied.
3. **Read-model timeout**: adds latency up to configured timeout; legacy path unaffected.
4. **Consumer outage**: shadow observability degraded; user experience unchanged.
5. **Staging secrets**: misconfigured Kafka/DB URLs cause consumer failure without affecting gateway readiness.

## Related Makefile targets

```bash
make control-tower-shadow-rollout-config-check
make control-tower-shadow-rollout-smoke-test
make control-tower-shadow-rollout-metrics-check
make control-tower-shadow-rollout-regression
```

## Dev JWT for live smoke

Never store JWT in Git. Use one of:

1. `export JWT_TOKEN="<temporary token>"`
2. `export DEV_ADMIN_EMAIL="<dev admin email>"` and `export DEV_ADMIN_PASSWORD="<dev password>"` (same values used by `make seed-dev-admin`, supplied via shell ENV only)
3. When `AUTH_ENABLED=false`, gateway may accept Control Tower summary using configured dev tenant without JWT

Smoke script never prints tokens and removes temporary login files.

## Metric delta evidence

Pre-initialized Prometheus series at value `0` only prove metric registration, **not** that shadow executed.

Prove rollout with deltas around one authenticated (or auth-disabled dev-tenant) summary request:

```bash
make control-tower-shadow-rollout-metrics-check
```

Expected increases:

- `control_tower_legacy_status_aggregate_requests_total`
- `control_tower_read_model_requests_total`
- `control_tower_read_model_shadow_comparison_total`

## Poll error classification

Normal idle poll outcomes **do not** increment `control_tower_shipment_consumer_errors_total`:

- poll returned 0 records
- poll context timeout (`PollTimeout`)
- parent shutdown cancellation

Real errors use bounded codes such as `BROKER_UNAVAILABLE`, `FETCH_NETWORK_ERROR`, `AUTHORIZATION_ERROR`, `FETCH_PROTOCOL_ERROR`, `UNKNOWN_POLL_ERROR`.

Historical `POLL_ERROR` spikes were caused by treating poll timeout as an error. After classification fix, idle consumer should not accumulate poll errors.

## Prometheus rules validation

```bash
make control-tower-shadow-rollout-observability-check
```

Uses `promtool check rules` for recording rules and example alerts, and `promtool check config` for `prometheus.yml`.

Active Prometheus loads only `control_tower_shadow_recording_rules.yml`. Example alerts remain inactive.

## Grafana dashboard validation

Dashboard JSON: `infrastructure/monitoring/grafana/provisioning/dashboards/control-tower-shadow-rollout.json`

Validated with `jq empty` in observability-check target. Reload Grafana provisioning profile to pick up panels.

## Docker image rebuild

Use Go 1.25 base images locally (`golang:1.25-alpine`) without `GOTOOLCHAIN=auto` network downloads:

```bash
docker compose \
  -f infrastructure/docker-compose/docker-compose.yml \
  -f infrastructure/docker-compose/docker-compose.staging-shadow.yml \
  --profile messaging --profile read-model --profile observability \
  build api-gateway control-tower-read-model-service
```

Confirm new image IDs with `docker inspect <container> --format '{{.Image}}'`.

Set `DEV_TENANT_ID` to the seeded dev tenant before applying the shadow override (`make seed-dev-admin` prints the tenant id). Without it, Control Tower summary returns `401` in dev auth-off mode.

Gateway downstream list fetches default to `CONTROL_TOWER_MAX_DOWNSTREAM_FETCH_LIMIT=100` because shipment-service rejects `limit>100`.

## Rollback metric check

Redeploy gateway with base compose (`mode=disabled`) and repeat Control Tower summary. Read-model request and shadow comparison counters must **not** increase.

## Local controlled rollout disclaimer

Docker Compose on a developer machine is **local controlled rollout**, not production staging. Use the same checks before enabling shadow on real staging infrastructure.

## TODO (out of v0.1 scope)

- [ ] Approved projection rebuild/replay for historical shipments
- [ ] Retention policies for inbox/projection/dead-letter
- [ ] Production SLO thresholds and paging policy
- [ ] Primary mode rollout (separate runbook)
