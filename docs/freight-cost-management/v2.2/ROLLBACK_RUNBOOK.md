# FREIGHT COST INTELLIGENCE v2.2 — Rollback Runbook

**Scope:** NON-PRODUCTION pilot rollback — **PRODUCTION_MUTATION=NO**  
**Release SHA:** `37c2eb62ccf9377359eb5c2fdf6f71eb9d187140`  
**Related:** `PILOT_RUNBOOK.md`, `REBUILD_RUNBOOK.md`

---

## Principles

1. **Fastest safe rollback first** — disable flags before redeploying images.
2. **Never roll back canonical ledger** as part of feature rollback — analytics are derived.
3. **Derived projections are rebuildable** — loss of analytics tables is recoverable via rebuild.
4. **No secrets in commands** — use protected env files on the host.

---

## Rollback levels

| Level | Action | When to use |
|-------|--------|-------------|
| **L1** | Workspace flag OFF | UI should hide immediately; buyer confusion / UI defect |
| **L2** | Analytics projection worker OFF | Worker errors, bad projections, runaway rebuild |
| **L3** | Redeploy previous application image | Service crash loop after upgrade |
| **L4** | Rebuild derived analytics | Projection corruption without canonical damage |
| **L5** | DB restore from backup | **Rare** — only if canonical data corrupted (not normal feature rollback) |

---

## Level 1 — Disable workspace

### Procedure

1. Set in protected env:

```env
NUXT_PUBLIC_FREIGHT_COST_WORKSPACE_ENABLED=false
```

2. Restart/redeploy **web-procurement** only.

### Expected results

- Buyer routes `/freight-costs/*` show unavailable page (English fallback + env var hint in body).
- Direct API calls still governed by gateway RBAC (browser hide ≠ authorization).
- Carrier still denied on intelligence API routes.

### Verification

```bash
# Browser smoke or curl frontend route
# API: carrier still 403 on /api/v1/freight-costs/analytics/overview
```

**Gate:** `WORKSPACE_FLAG_ROLLBACK=PASS`

---

## Level 2 — Disable analytics projection worker

### Procedure

1. Set in protected env:

```env
FREIGHT_COST_ANALYTICS_PROJECTION_ENABLED=false
```

2. Restart **freight-cost-service** only:

```bash
docker compose -f docker-compose.yml -f docker-compose.freight-cost-pilot.yml up -d --no-deps freight-cost-service
```

### Expected results

- Worker stops processing dirty queue and scheduled rebuilds.
- Public analytics API behavior follows contract:
  - Existing projections may remain readable with explicit freshness/`STALE` semantics, OR
  - Service returns explicit not-available/stale — **must not fabricate fresh data**.
- No new benchmark/opportunity generation.

### Verification

```http
GET /api/v1/freight-costs/analytics/overview
```

Check response `data_quality` / projection metadata — no false "fresh" timestamp after worker disabled.

**Gate:** `PROJECTION_FLAG_ROLLBACK=PASS`, `WORKER_OFF_FAIL_CLOSED=PASS`

---

## Level 3 — Application image rollback

### Preconditions

- Previous digest-pinned image references documented in protected env backup.
- CT staging: `BINTRANS_*_IMAGE` lines for prior SHA.

### Procedure

1. Execute L1 + L2 (flags OFF).
2. Restore previous image refs in protected env.
3. Run CT staging runtime scripts (example):

```bash
./scripts/ops/bintrans_ct_staging/bintrans_ct_staging_runtime_images_validate.sh
./scripts/ops/bintrans_ct_staging/bintrans_ct_staging_runtime_up.sh
./scripts/ops/bintrans_ct_staging/bintrans_ct_staging_runtime_health.sh
```

Local lab: redeploy prior compose build tag or `git checkout <prev-sha> && docker compose build`.

### Expected results

- Gateway + freight-cost health 200.
- Analytics routes behave per rolled-back version (likely 404/503 if freight-cost absent on old version).

### Current gap (2026-08-24)

**`ROLLBACK_RELEASE_AVAILABLE=NO`** for v2.2 on CT registry — prior pin is `git-b75eb3d` without freight-cost intelligence. Operator must publish v2.2 images before forward pilot; keep previous pin for L3 rollback target.

---

## Level 4 — Rebuild derived analytics

Use when projections are corrupt but canonical ledger is intact.

### Procedure

See `REBUILD_RUNBOOK.md` §5:

```http
POST /internal/v1/freight-cost/analytics/tenants/{tenantId}/rebuild
```

For disaster-style loss, run full DR drill pattern (CI: `TestFC22G1_FullProjectionLossAndRebuildRestoresBusinessState`).

### Expected results

- Derived tables repopulated from canonical sources.
- Business checksum equivalent to pre-loss state (for unchanged canonical data).
- `cost_entry` / `cost_summary_projection` row counts unchanged.

**Gate:** `PILOT_REBUILD=PASS`, `CANONICAL_SOURCE_PRESERVED=YES`

---

## Level 5 — Database restore (exceptional)

**Not a normal feature rollback.**

Use only if canonical financial data corrupted.

1. Stop write traffic to freight-cost and billing paths.
2. Restore from verified backup (`pg_restore` / operator procedure on CT VM).
3. Re-apply migrations if needed from known good state.
4. Rebuild analytics (L4).

CT staging backup script:

```bash
./scripts/ops/bintrans_ct_staging/bintrans_ct_staging_backup.sh
```

---

## Combined rollback drill (pilot validation)

Execute on dedicated non-prod during pilot sign-off:

1. Workspace **ON** → verify pages load.
2. Workspace **OFF** (L1) → verify hidden/unavailable.
3. Workspace **ON** → continue if pilot proceeds.
4. Analytics **OFF** (L2) → verify fail-closed semantics.
5. Analytics **ON** → restore pilot state.

Record timestamps and observer.

---

## Rollback decision matrix

| Symptom | First action | Escalate to |
|---------|--------------|-------------|
| UI shows wrong tenant/company data | **L1** immediately | L3 + incident |
| Carrier sees buyer intelligence | **L1 + L2** + security incident | L3 |
| Cross-currency totals in API | **L2** | data fix + L4 |
| Worker crash loop | **L2** | L3 |
| Projection corrupt, ledger OK | **L2** then **L4** | — |
| Canonical ledger mutated | **L2 + stop ingest** | **L5** + incident |

---

## Production guardrail

- **Do not** change production Helm/compose/env/secrets in this runbook.
- Production flags remain:
  - `FREIGHT_COST_ANALYTICS_PROJECTION_ENABLED=false`
  - `NUXT_PUBLIC_FREIGHT_COST_WORKSPACE_ENABLED=off`

---

## References

- Feature flags: `services/freight-cost-service/internal/config/config.go`, `apps/web-procurement/nuxt.config.ts`
- Pilot overlay: `infrastructure/docker-compose/docker-compose.freight-cost-pilot.yml`
- CT staging ops: `scripts/ops/bintrans_ct_staging/`
- CI DR proof: main run 32760837345, job `freight-cost-analytics-final-e2e`
