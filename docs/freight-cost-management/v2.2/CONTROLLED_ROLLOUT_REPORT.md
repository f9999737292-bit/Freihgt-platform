# FREIGHT COST INTELLIGENCE v2.2 — Controlled Rollout Report

**Date:** 2026-08-24  
**Release engineer / pilot owner:** Cursor ops session  
**Approved technical baseline:** `37c2eb62ccf9377359eb5c2fdf6f71eb9d187140` (main, PR #60 merged)  
**Scope:** NON-PRODUCTION validation & release readiness — **no production mutation**

---

## Executive summary

Freight Cost Intelligence v2.2 is **technically complete** on `main` with green CI (run [32760837345](https://github.com/f9999737292-bit/Freihgt-platform/actions/runs/32760837345)). A controlled **remote** non-production pilot **could not be executed** in this session: no documented staging environment currently runs `freight-cost-service`, `web-procurement` intelligence workspace, or migrations through `000064`.

This report records infrastructure discovery, operational gaps, partial local validation, and the decision model for production readiness.

| Flag | Value |
|------|-------|
| `V2_2_TECHNICAL_COMPLETE` | **YES** |
| `PILOT_VALIDATION` | **FAIL** |
| `PILOT_VALIDATION_COMPLETE` | **NO** |
| `READY_FOR_PRODUCTION_ROLLOUT` | **NO** |
| `PRODUCTION_ROLLOUT` | **NO** |
| `PRODUCTION_MUTATION` | **NO** |
| `NEW_BUSINESS_FEATURES_ADDED` | **NO** |
| `VERDICT` | **CONDITIONAL_PASS** (technical stack proven in CI; operational pilot blocked) |

---

## 1. Git pre-flight

| Field | Value |
|-------|-------|
| `ORIGIN_MAIN_SHA_AT_START` | `37c2eb62ccf9377359eb5c2fdf6f71eb9d187140` |
| `V2_2_FINAL_MERGE_IN_MAIN` | **YES** (`37c2eb6` is ancestor of `origin/main`) |
| `START_WORKTREE_CLEAN` | **NO** (primary checkout `D:\Projects\freight-platform` had local doc/ops deltas; ops work performed in clean worktree) |
| Ops worktree | `D:\Projects\freight-platform-wt\freight-cost-intelligence-controlled-rollout-v2.2` |
| Ops branch | `ops/freight-cost-intelligence-controlled-rollout-v2.2` @ `37c2eb6` |

---

## 2. Available non-production environments (discovered)

| ENV_NAME | HOST | CLASS | DEPLOY_METHOD | FREIGHT_COST | WEB_PROCUREMENT | STATUS |
|----------|------|-------|---------------|--------------|-----------------|--------|
| **selectel-staging (CT dedicated)** | `161.104.57.152` | **NON-PROD** | Manual Docker Compose + registry | **NOT in runtime set** | **NOT deployed** | Foundation @ schema **19** only; `RUNTIME_DEPLOYED=NO` |
| **shared VPS staging / low-code pilot** | `161.104.53.221` | **NON-PROD*** | Manual compose + nginx | **NOT present** | **NOT deployed** | *Historical prod DB sharing — isolation plan documented, not verified here |
| **local dev lab** | `localhost` | **NON-PROD** | `docker compose` / `make platform-up` | Via new pilot overlay | `pnpm dev:procurement` :3005 | Available for operator execution |

\* Shared VPS classified non-prod by docs but carries **production co-tenancy risk** — not selected for this pilot.

**Selected target for mutation:** **NONE** (remote pilot blocked; no SSH/operator credentials in repo).

| Gate | Value |
|------|-------|
| `TARGET_ENVIRONMENT_CLASSIFIED_NON_PROD` | **YES** (CT dedicated VM) |
| `CONTROLLED_ROLLOUT_BLOCKED` | **YES** |
| `BLOCKER` | `NON_PRODUCTION_TARGET_NOT_OPERATIONALLY_READY` |

---

## 3. Release pinning

| Field | Value |
|-------|-------|
| `PILOT_GIT_SHA` | `37c2eb62ccf9377359eb5c2fdf6f71eb9d187140` |
| `FREIGHT_COST_IMAGE` | Not published to registry (build from `services/freight-cost-service/Dockerfile`) |
| `API_GATEWAY_IMAGE` | CT staging template: `cr.selcloud.ru/bintrans-staging/api-gateway@sha256:…` — **not built for v2.2 SHA** |
| `WEB_PROCUREMENT_IMAGE` | **Not defined** in staging packs |
| `IMAGE_DIGEST_PINNING` | **PARTIAL** — CT staging supports digest pins; **no v2.2 images published** |
| `RUNNING_SHA_MATCHES_PIN` | **NOT_APPLICABLE** (no remote deploy) |

---

## 4. Database & migrations

| Field | Value |
|-------|-------|
| `EXPECTED_LATEST_MIGRATION` | `000064_freight_cost_benchmark_savings_v2.2E` |
| Analytics migrations | `000061`, `000062`, `000063`, `000064` — **present in repo** |
| CT staging `CURRENT_MIGRATION` | **19** (operator-supplied; 45 migrations behind v2.2) |
| `MIGRATION_STATE_VALID` | **NOT_VERIFIED_REMOTE** — local integration tests blocked (Postgres auth on `:5432`) |
| CI migration gate | **PASS** — `TestFC22G_MigrationGateV22UpDown` on main CI |

---

## 5. Backup / recovery

| Field | Value |
|-------|-------|
| CT staging `BACKUP_AVAILABLE` | **YES** (operator: `/protected/bintrans/backups/freight_platform_20260811T083942Z.dump`) |
| `BACKUP_VALIDATED` | **YES** (operator-supplied SHA-256) |
| `RESTORE_PROCEDURE_AVAILABLE` | **YES** — `scripts/ops/bintrans_ct_staging/bintrans_ct_staging_backup.sh` |
| `CANONICAL_SOURCE_TABLES` | `freight_cost.cost_entry`, `freight_cost.cost_summary_projection`, billing/settlement authoritative tables (derived analytics **rebuildable**) |

---

## 6. Feature flag plan (staged — not executed remotely)

| Phase | `FREIGHT_COST_ANALYTICS_PROJECTION_ENABLED` | `NUXT_PUBLIC_FREIGHT_COST_WORKSPACE_ENABLED` |
|-------|---------------------------------------------|-----------------------------------------------|
| 1 Deploy code | `false` | `false` |
| 2 Analytics only | `true` | `false` |
| 3 Validate API | `true` | `false` |
| 4 Workspace pilot | `true` | `true` |
| 5 Full checks | `true` | `true` |

| Field | Value |
|-------|-------|
| `ROLLOUT_SCOPE` | **ENVIRONMENT-WIDE** (no tenant-level flag in codebase) |
| `TENANT_LEVEL_FLAG_AVAILABLE` | **NO** |
| `ANALYTICS_DEFAULT` | `false` |
| `WORKSPACE_DEFAULT` | off |
| `FINAL_ANALYTICS_FLAG_NONPROD` | **unchanged** (`false` — no remote mutation) |
| `FINAL_WORKSPACE_FLAG_NONPROD` | **unchanged** (off) |
| `PRODUCTION_CONFIG_CHANGED` | **NO** |

---

## 7. Phase execution status

### Phase 1 — Deploy flags OFF

| Gate | Status |
|------|--------|
| `DEPLOYMENT_SUCCESS` | **NOT_EXECUTED** (remote) |
| `HEALTH_GATE` | **NOT_EXECUTED** |
| `FLAGS_DEFAULT_OFF_CONFIRMED` | **YES** (committed defaults) |

### Phase 2 — Analytics enablement

| Gate | Status |
|------|--------|
| `ANALYTICS_PROJECTION_BUILD` | **CI PASS** — not re-run locally (DB auth) |
| `REBUILD_METHOD` | Internal S2S `POST /internal/v1/freight-cost/analytics/tenants/{id}/rebuild` |
| `CANONICAL_LEDGER_MUTATED` | **NO** (CI DR drill + architecture) |
| `CANONICAL_BILLING_MUTATED` | **NO** |
| Projection row counts | **NOT_CAPTURED** (no pilot tenant on live staging) |

### API / security / browser (deployed environment)

| Gate | Status | Evidence |
|------|--------|----------|
| `BUYER_API_GATE` | **CI PASS** | main CI `freight-cost-public-e2e`, `freight-cost-analytics-final-e2e` |
| `CARRIER_DENY` | **CI PASS** | gateway integration tests |
| `CROSS_TENANT_DENY` | **CI PASS** | gateway integration tests |
| `CROSS_COMPANY_DENY` | **CI PASS** | gateway integration tests |
| `SPOOFING_DENY` | **CI PASS** | gateway integration tests |
| `LIVE_PILOT_BROWSER` | **CI PASS** | `freight-cost-intelligence-browser-e2e` job on main |
| `POST_ENABLE_SECURITY_GATE` | **NOT_EXECUTED** (no live workspace deploy) |

**Local re-run this session:**

- `api-gateway/internal/integration/freightcostpublic` — **PASS**
- `freight-cost-service/internal/integration/analytics` (DR, migration, browser) — **FAIL** (local Postgres `rfx_test` auth on `:5432`)

### Rollback drills

| Gate | Status |
|------|--------|
| `WORKSPACE_FLAG_ROLLBACK` | **NOT_EXECUTED** (remote) |
| `PROJECTION_FLAG_ROLLBACK` | **NOT_EXECUTED** (remote) |
| `WORKER_OFF_FAIL_CLOSED` | **CI PASS** (contract tests) |
| `ROLLBACK_RELEASE_AVAILABLE` | **NO** for v2.2 on CT staging registry |

---

## 8. Observability review (code + CI)

| Signal | Present | High-cardinality labels |
|--------|---------|-------------------------|
| `freight_cost_analytics_rebuild_total{result}` | YES | **NO** tenant/company IDs |
| `freight_cost_analytics_rebuild_duration_seconds` | YES | **NO** |
| `freight_cost_analytics_incremental_total{result}` | YES | **NO** |
| `freight_cost_analytics_benchmark_cohorts_total{data_quality}` | YES | **NO** |
| `freight_cost_analytics_opportunities_generated_total{opportunity_type}` | YES | **NO** |
| `freight_cost_analytics_benchmark_rebuild_failures_total` | YES | **NO** |

| Gate | Value |
|------|-------|
| `HIGH_CARDINALITY_LABELS` | **NO** |
| `OBSERVABILITY` | **CODE_READY** — **NOT_VALIDATED_ON_STAGING** (Prometheus/Grafana not running for freight-cost on any remote target) |

---

## 9. Secret scan

| Gate | Value |
|------|-------|
| `SECRET_SCAN` | **PASS** — no tracked `.env` secrets (only `*.example` / template env files) |
| CI `repository-safety` on main | **PASS** |

---

## 10. Findings (F22R)

### F22R001 — HIGH — No operational non-prod stack for freight-cost

| Field | Value |
|-------|-------|
| **AREA** | Infrastructure |
| **DESCRIPTION** | Documented Selectel staging (CT VM) has schema 19 and no `freight-cost-service` in compose runtime set. Shared VPS lacks freight-cost entirely. |
| **EVIDENCE** | `docker-compose.bintrans-ct-staging.yml`, `staging.env.example` (`MIGRATION_TARGET=000019`); base `docker-compose.yml` had no freight-cost before pilot overlay |
| **IMPACT** | Cannot execute live pilot on staging data |
| **ACTION** | Extend CT staging pack to migration 64, add freight-cost-service + gateway wiring, publish images @ `git-37c2eb6` |
| **STATUS** | OPEN |

### F22R002 — HIGH — Registry images not published for v2.2 SHA

| Field | Value |
|-------|-------|
| **AREA** | Release |
| **DESCRIPTION** | CT staging pins `git-b75eb3d`; v2.2 merge `37c2eb6` has no registry tags/digests |
| **EVIDENCE** | `scripts/ops/bintrans_ct_staging/registry.images.template.env` |
| **IMPACT** | `ROLLBACK_RELEASE_AVAILABLE=NO` for v2.2 pilot |
| **ACTION** | Operator publish + digest pin per existing CT staging scripts |
| **STATUS** | OPEN |

### F22R003 — HIGH — web-procurement not deployed to staging

| Field | Value |
|-------|-------|
| **AREA** | Frontend ops |
| **DESCRIPTION** | Intelligence workspace exists in repo but no staging deploy path documented (only web-admin static deploy on shared VPS) |
| **IMPACT** | Live pilot browser checks require local dev or new deploy pipeline |
| **ACTION** | Add staging deploy for web-procurement with `NUXT_PUBLIC_FREIGHT_COST_WORKSPACE_ENABLED` gated |
| **STATUS** | OPEN |

### F22R004 — MEDIUM — Environment-wide flags only

| Field | Value |
|-------|-------|
| **AREA** | Rollout scope |
| **DESCRIPTION** | No tenant-scoped feature flags; enabling workspace affects entire environment |
| **IMPACT** | Requires dedicated non-prod environment (acceptable on CT VM, not on shared VPS) |
| **ACTION** | Document scope; use dedicated VM only |
| **STATUS** | ACCEPTED_RISK |

### F22R005 — MEDIUM — Shared VPS production adjacency

| Field | Value |
|-------|-------|
| **AREA** | Safety |
| **DESCRIPTION** | Historical shared DB/gateway on `161.104.53.221` |
| **IMPACT** | Disqualified as pilot target |
| **ACTION** | Use CT dedicated VM only |
| **STATUS** | MITIGATED_BY_TARGET_SELECTION |

### F22R006 — INFO — Pilot compose overlay added (ops branch)

| Field | Value |
|-------|-------|
| **AREA** | Ops config |
| **DESCRIPTION** | Added `docker-compose.freight-cost-pilot.yml` + `scripts/ops/freight_cost_pilot/pilot.env.example` |
| **IMPACT** | Enables local/CT lab deploy without production changes |
| **ACTION** | Operator dry-run on CT VM after migration 64 |
| **STATUS** | DELIVERED |

---

## 11. Finding counts

| Severity | Count |
|----------|-------|
| CRITICAL | 0 |
| HIGH | 3 |
| MEDIUM | 2 |
| LOW | 0 |
| INFO | 1 |

**Unresolved production blockers:** F22R001, F22R002, F22R003

---

## 12. CI evidence (authoritative for technical gates)

Main CI run **32760837345** @ `37c2eb6` — all freight-cost jobs **success**:

- `freight-cost-analytics-final-e2e` (DR, security, race, migration)
- `freight-cost-analytics-100k-gate`
- `freight-cost-intelligence-browser-e2e`
- `freight-cost-public-e2e`
- `freight-cost-analytics-race-gate`

---

## 13. Documentation delivered

| Document | Status |
|----------|--------|
| `CONTROLLED_ROLLOUT_REPORT.md` | **YES** (this file) |
| `PILOT_RUNBOOK.md` | **YES** |
| `ROLLBACK_RUNBOOK.md` | **YES** |

---

## 14. Git / PR

| Field | Value |
|-------|-------|
| `OPS_PR_REQUIRED` | **YES** (ops overlay + runbooks) |
| `OPS_BRANCH` | `ops/freight-cost-intelligence-controlled-rollout-v2.2` |
| `OPS_PR_NUMBER` | pending |
| `OPS_PR_STATE` | pending |
| `OPS_PR_MERGED` | NO |

---

## 15. Remaining before production

1. Deploy v2.2 stack to **dedicated CT staging VM** with migrations through `000064`.
2. Publish and digest-pin container images for `37c2eb6`.
3. Execute phased flag rollout + live API/browser/security validation on staging data.
4. Prove rollback drills on staging (workspace OFF → projection OFF → image rollback).
5. Operational sign-off from platform owner after bounded soak.

---

## 16. Next action

**STOP_AFTER_CONTROLLED_ROLLOUT=YES** — do not start v2.3. Operator executes CT staging pilot using `PILOT_RUNBOOK.md` after F22R001–003 resolved.
