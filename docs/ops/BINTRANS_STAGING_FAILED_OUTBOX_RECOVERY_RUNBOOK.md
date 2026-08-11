# BINTRANS Staging — FAILED Outbox Recovery Runbook

This runbook describes the **controlled recovery** of historical FAILED shipment outbox rows on the dedicated BINTRANS Control Tower staging VM.

**Do not execute live replay from this document alone.** Operator approval is required after reviewing dry-run output from `bintrans_shipment_outbox_replay`.

## Scope

Historical controlled run:

| Field | Value |
|-------|-------|
| RUN_ID | `BINTRANS-CT-STAGING-20260811T173628Z` |
| Shipments | 5 |
| Outbox total | 22 |
| Outbox FAILED | 22 |
| Outbox PUBLISHED | 0 |

Post-fix canary (must remain PASS):

| Field | Value |
|-------|-------|
| CANARY_RUN_ID | `BINTRANS-CT-CANARY-20260811T183315Z` |
| CANARY_END_TO_END_PASS | YES |

Tenant:

| Field | Value |
|-------|-------|
| TENANT_ID | `873b3fbc-3cb4-413f-81cd-6fa2c94e785e` |

Controlled shipment aggregate IDs:

```
df1cfb94-5def-48cf-a59e-b36341efe86a
3ca59040-7add-42ff-8e2e-de05c543c5c5
c746460c-0649-4f0c-9233-f11d5da29aa7
5b6b8d61-c25f-4c0c-bafe-16505ccf5be2
c20b86bf-26b2-45cd-b758-575b6c5b6c4f
```

## PRECHECK (read-only)

On `bintrans-ct-staging` (`bintrans-control-tower-staging`):

1. Topic `shipment.status.v1` exists (3 partitions, RF=1, healthy leaders)
2. Redpanda/broker healthy
3. Canary E2E PASS (`CANARY_END_TO_END_PASS=YES`)
4. `./scripts/ops/bintrans_ct_staging/bintrans_ct_staging_runtime_health.sh` → PASS
5. `./scripts/ops/bintrans_ct_staging/bintrans_ct_staging_observability_health.sh` → PASS
6. Schema version 19, `dirty=false`
7. Control Tower mode = `shadow`
8. `CONTROL_TOWER_CONSUMER_ENABLED=true`
9. `SHIPMENT_OUTBOX_ENABLED=true`
10. Primary disabled (`CONTROL_TOWER_READ_MODEL_MODE=shadow`)
11. `COHORT_APPROVED=NO`

Stop if any check fails.

## Build tooling (once per deployment)

From repository root on operator workstation or staging VM **after** the replay commit is deployed:

```bash
cd services/shipment-service
go build -o shipment-outbox-replay ./cmd/shipment-outbox-replay
```

## DRY RUN (required)

Default mode performs **no mutation**.

```bash
export DATABASE_URL='postgres://...'

./scripts/ops/bintrans_shipment_outbox_replay.sh \
  --tenant-id 873b3fbc-3cb4-413f-81cd-6fa2c94e785e \
  --aggregate-id df1cfb94-5def-48cf-a59e-b36341efe86a \
  --aggregate-id 3ca59040-7add-42ff-8e2e-de05c543c5c5 \
  --aggregate-id c746460c-0649-4f0c-9233-f11d5da29aa7 \
  --aggregate-id 5b6b8d61-c25f-4c0c-bafe-16505ccf5be2 \
  --aggregate-id c20b86bf-26b2-45cd-b758-575b6c5b6c4f
```

Expected dry-run properties:

- `MODE=DRY_RUN`
- `MATCHED_COUNT=22`
- Every row shows `CURRENT_STATUS=FAILED`
- No payload printed
- No database writes

Review output with a second operator before proceeding.

## EXECUTE (separate approval only)

```bash
./scripts/ops/bintrans_shipment_outbox_replay.sh \
  --tenant-id 873b3fbc-3cb4-413f-81cd-6fa2c94e785e \
  --aggregate-id df1cfb94-5def-48cf-a59e-b36341efe86a \
  --aggregate-id 3ca59040-7add-42ff-8e2e-de05c543c5c5 \
  --aggregate-id c746460c-0649-4f0c-9233-f11d5da29aa7 \
  --aggregate-id 5b6b8d61-c25f-4c0c-bafe-16505ccf5be2 \
  --aggregate-id c20b86bf-26b2-45cd-b758-575b6c5b6c4f \
  --execute
```

Expected execute properties:

- `MODE=EXECUTE`
- `AFFECTED_COUNT=22`
- `AFFECTED_COUNT` equals dry-run `MATCHED_COUNT`

The outbox worker will republish rows asynchronously. Do **not** restart containers or run arbitrary SQL.

## POSTCHECK (read-only)

Verify pipeline convergence:

| Check | Expected |
|-------|----------|
| Historical outbox PUBLISHED | 22 |
| Historical outbox FAILED | 0 |
| Kafka `shipment.status.v1` offsets | increased from pre-replay baseline |
| Control Tower inbox | 22 new APPLIED rows for historical events |
| Projections for 5 shipments | present |
| Projection status vs legacy shipment | missing=0, extra=0, mismatch=0 |
| Canary shipment/projection | unchanged and still PASS |
| Runtime health | PASS |
| Observability health | PASS |
| Shadow mode | still enabled |
| Primary | still disabled |
| Cohort | not created / not approved |

Poll read-only for up to 5 minutes if async processing is incomplete. Do not manually publish Kafka messages or repair projections.

## Rollback / stop conditions

Stop and escalate if:

- dry-run `MATCHED_COUNT` ≠ 22
- execute `AFFECTED_COUNT` ≠ dry-run count
- any non-FAILED row appears in preview
- partial aggregate replay error
- projection mismatch persists after bounded wait
- duplicate inbox errors (should be DUPLICATE/STALE, not hard failures)
- primary mode or cohort approval changes unexpectedly

There is no automatic rollback of replayed rows. Recovery from a bad execute requires operator incident review; do not use ad-hoc SQL.

## Why not arbitrary SQL?

Direct `UPDATE ... SET status='PENDING'` bypasses:

- tenant-scoped selection guards
- per-aggregate ordering validation
- exact affected-row enforcement
- transactional row locking
- immutable field verification
- dry-run audit trail

Use the repository-backed CLI only.
