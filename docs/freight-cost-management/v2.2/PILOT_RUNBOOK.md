# FREIGHT COST INTELLIGENCE v2.2 — Pilot Runbook

**Audience:** Platform ops, release engineer, pilot owner  
**Scope:** NON-PRODUCTION only — **PRODUCTION_MUTATION=NO**  
**Release SHA:** `37c2eb62ccf9377359eb5c2fdf6f71eb9d187140`  
**Related:** `REBUILD_RUNBOOK.md`, `ROLLBACK_RUNBOOK.md`, `CONTROLLED_ROLLOUT_REPORT.md`

---

## 1. Preconditions

| Check | Required |
|-------|----------|
| Target is **dedicated non-prod** (recommended: Selectel CT VM `161.104.57.152`) | YES |
| **Not** shared VPS `161.104.53.221` with prod adjacency | YES |
| Git checkout @ `37c2eb6` or later ops-only fix on main | YES |
| Migrations applied through **000064** | YES |
| Fresh DB backup verified (`BACKUP_VERIFIED=YES`) before migration jump | YES |
| `INTERNAL_SERVICE_TOKEN` in protected env (not in git) | YES |
| `JWT_SECRET` provisioned for gateway auth | YES |
| Operator credentials for registry (`cr.selcloud.ru/bintrans-staging`) if using CT images | YES |

**Canonical tables (must not be mutated by analytics):**

- `freight_cost.cost_entry`
- `freight_cost.cost_summary_projection`
- Billing/settlement authoritative amounts

---

## 2. Target environment options

### Option A — Dedicated CT staging VM (preferred)

| Field | Value |
|-------|-------|
| Host | `161.104.57.152` |
| Compose project | `bintrans-ct-staging` |
| Gateway | `127.0.0.1:18080` |
| Protected env | `/protected/bintrans/control-tower-observation/staging.env` |
| Observability | Prometheus `127.0.0.1:9090`, Grafana `127.0.0.1:3001` |

**Prerequisite gap (2026-08-24):** VM at schema **19** — must migrate to **64** and add freight-cost to runtime before pilot.

### Option B — Local dev lab

```bash
cd infrastructure/docker-compose
docker compose -f docker-compose.yml -f docker-compose.freight-cost-pilot.yml --profile observability up -d
docker compose --profile tools run --rm migrate up
```

Copy `scripts/ops/freight_cost_pilot/pilot.env.example` → protected local path; populate secrets.

---

## 3. Release pinning

1. Record `PILOT_GIT_SHA=$(git rev-parse HEAD)` — must equal approved main or ops fix on main.
2. Build images from that SHA (local) or publish to registry:
   - `freight-cost-service`
   - `api-gateway` (with `FREIGHT_COST_SERVICE_URL`)
   - `web-procurement` (if browser pilot required)
3. Pin digests in protected env when using CT staging (`BINTRANS_*_IMAGE=@sha256:…`).

---

## 4. Migration check

```bash
# Read-only — current version
docker compose --profile tools run --rm migrate version

# Expected: 64, dirty: false
```

Verify files exist:

- `000061_freight_cost_analytics_projection_v2.2B`
- `000062_freight_cost_lane_carrier_intelligence_v2.2C`
- `000063_freight_cost_accessorial_enrichment_v2.2D`
- `000064_freight_cost_benchmark_savings_v2.2E`

Apply (only after backup gate):

```bash
docker compose --profile tools run --rm migrate up
```

---

## 5. Phase 1 — Deploy with flags OFF

**Environment:**

```env
FREIGHT_COST_ANALYTICS_PROJECTION_ENABLED=false
NUXT_PUBLIC_FREIGHT_COST_WORKSPACE_ENABLED=false
```

**Start stack** (local lab example):

```bash
docker compose \
  -f docker-compose.yml \
  -f docker-compose.freight-cost-pilot.yml \
  --env-file /protected/.../pilot.env \
  up -d
```

**Health checks:**

```bash
curl -sf http://127.0.0.1:8092/health   # freight-cost-service
curl -sf http://127.0.0.1:8080/health   # api-gateway (or :18080 on CT VM)
```

**Version alignment:** record running image digest / build label where supported.

---

## 6. Phase 2 — Enable analytics projection ONLY

1. Set `FREIGHT_COST_ANALYTICS_PROJECTION_ENABLED=true` in protected env.
2. Restart **freight-cost-service only**:

```bash
docker compose -f docker-compose.yml -f docker-compose.freight-cost-pilot.yml up -d --no-deps freight-cost-service
```

3. Record `ANALYTICS_FLAG_ENABLED_AT` (UTC timestamp).

---

## 7. Initial projection build

For each pilot tenant with canonical cost data:

```http
POST /internal/v1/freight-cost/analytics/tenants/{tenantId}/rebuild
Authorization: Bearer <INTERNAL_SERVICE_TOKEN>
```

Or use service CLI if exposed in container.

**Verify state:**

```http
GET /internal/v1/freight-cost/analytics/tenants/{tenantId}/state
```

Expect: `status=IDLE`, fresh `calculated_at`, `data_through` ≥ latest summary.

**Compare canonical safety:** row counts on `cost_entry` / `cost_summary_projection` unchanged.

---

## 8. Phase 3 — API validation (workspace still OFF)

Through **real API gateway** with buyer JWT + `X-Company-ID`:

| Route | Expected |
|-------|----------|
| `GET /api/v1/freight-costs/analytics/overview` | 200 |
| `GET /api/v1/freight-costs/analytics/lanes` | 200 |
| `GET /api/v1/freight-costs/analytics/carriers` | 200 |
| `GET /api/v1/freight-costs/analytics/accessorials` | 200 |
| `GET /api/v1/freight-costs/opportunities` | 200 |

**Security (non-destructive):**

| Actor | Expected |
|-------|----------|
| Carrier on all 5 intelligence routes | 403 |
| Buyer wrong tenant | 403 |
| Buyer wrong company | 403 |
| Spoof `X-Platform-Admin` / header elevation | 403 |

---

## 9. Phase 4 — Enable workspace

Only after Phase 3 PASS.

```env
NUXT_PUBLIC_FREIGHT_COST_WORKSPACE_ENABLED=true
```

Restart web-procurement (deploy static/SSR app with env injected at build or runtime per your Nuxt deploy model).

Record `WORKSPACE_FLAG_ENABLED_AT`.

---

## 10. Phase 5 — Browser validation

Buyer flow on deployed web-procurement (no mocks):

1. Overview
2. Lanes
3. Carriers
4. Accessorials
5. Opportunities

Verify: loading/empty/data-quality states, RU/EN/ZH navigation labels, no UUID as primary business label.

**Feature-flag-off page:** with flag false, routes show unavailable copy (FC22G1-UI-008 pattern).

---

## 11. Observability during pilot

Inspect Prometheus/Grafana/logs for:

- `freight_cost_analytics_rebuild_total`
- `freight_cost_analytics_rebuild_duration_seconds`
- `freight_cost_analytics_incremental_total`
- Worker errors / crash loops
- Gateway 5xx on analytics routes

**Log safety:** no JWT, S2S tokens, DB passwords, raw commercial payloads in logs.

---

## 12. Bounded soak

Observe after enablement (operator-defined window; suggest ≥30 min minimum):

- Service restarts
- Rebuild failures
- Dirty queue growth
- Projection lag / `STALE` states
- Persistent 5xx

---

## 13. Rebuild operational drill

1. Capture projection checksum/counts for pilot tenant.
2. Run supported rebuild (see `REBUILD_RUNBOOK.md` §5).
3. Compare semantic equivalence.
4. Confirm canonical ledger unchanged.

---

## 14. Rollback reference

See `ROLLBACK_RUNBOOK.md` — execute at least workspace flag OFF drill before declaring pilot complete.

---

## 15. Shutdown / final state

If environment is **shared** or pilot incomplete: restore both flags **OFF**.

If **dedicated pilot VM** and validation PASS: flags may remain ON per operator approval.

**Production:** flags remain OFF — never modify production env files in this runbook.

---

## 16. Pass criteria summary

Pilot PASS requires all gates in `CONTROLLED_ROLLOUT_REPORT.md` § PASS rule, including live deployed API/browser/rollback on approved non-prod target.

**Current status (2026-08-24):** runbook ready; **execution blocked** until CT staging reaches migration 64 + freight-cost runtime (see F22R001–003).
