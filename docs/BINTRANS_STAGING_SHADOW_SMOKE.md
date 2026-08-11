# BINTRANS Control Tower Shadow Smoke Design

Static and operator checks confirming shadow wiring without enabling primary mode.

## Startup dependencies (from source)

| Requirement | Required for container boot? | Verified by |
|-------------|------------------------------|-------------|
| `CONTROL_TOWER_READ_MODEL_MODE=shadow` | Yes (via compose) | runtime preflight |
| `CONTROL_TOWER_CONSUMER_ENABLED=true` | Yes | runtime preflight env contract |
| `SHIPMENT_OUTBOX_ENABLED=true` | Yes | runtime preflight env contract |
| Approved cohort | **No** for process boot | Day 0 gate only |

## Shadow semantics (source-derived)

| Mode | Public response source |
|------|------------------------|
| **shadow** | Legacy aggregate remains public; read-model fetched for comparison |
| **primary** | Read-model becomes public source (forbidden on BINTRANS staging) |
| **disabled** | Read-model path off (rejected by runtime preflight) |

Source: `services/api-gateway/internal/controltowerreadmodel/mode.go`, shadow merge behavior in gateway read-model package.

## Static checks (repository / VM, no cohort)

```bash
./scripts/ops/bintrans_ct_staging/bintrans_ct_staging_runtime_preflight.sh
./scripts/ops/bintrans_ct_staging/bintrans_ct_staging_runtime_health.sh
```

Preflight extracts **effective** `api-gateway` `CONTROL_TOWER_READ_MODEL_MODE` from rendered compose (not text grep).

## Process smoke (after runtime_up)

1. Foundation healthy (postgres, redpanda).
2. All 10 runtime services running; no migrate/prometheus/grafana.
3. Gateway `/health` and control-tower-read-model `/health` return 2xx on localhost.

## Functional shadow observation (later — requires cohort)

Requires:

- `COHORT_APPROVED=YES`
- Non-empty approved cohort at `/protected/bintrans/control-tower-cohort.json`
- `JWT_TOKEN` or operator auth for observation tooling
- `scripts/ops/control_tower_shadow_observation/`

Empty cohort rejected: `cohort manifest is empty` (`cohort.go`).

## Primary mode

**Must remain disabled.** No repository script exposes a casual `--primary` flag. Runtime preflight fails on effective `primary`.
