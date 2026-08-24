# Freight Platform — System Testing Documentation

Master System Test Plan & End-to-End Business Acceptance **v1**.

| Document | Purpose |
|----------|---------|
| [MASTER_SYSTEM_TEST_PLAN.md](./MASTER_SYSTEM_TEST_PLAN.md) | Architecture, levels L0–L14, waves, gates, CI strategy |
| [BUSINESS_E2E_CATALOG.md](./BUSINESS_E2E_CATALOG.md) | Golden paths, negative/security scenarios, step definitions |
| [ROLE_RBAC_TEST_MATRIX.md](./ROLE_RBAC_TEST_MATRIX.md) | Role × domain × action matrix |
| [TEST_DATA_MODEL.md](./TEST_DATA_MODEL.md) | Deterministic tenants, companies, reset strategy |
| [SYSTEM_TEST_TRACEABILITY_MATRIX.md](./SYSTEM_TEST_TRACEABILITY_MATRIX.md) | Business capability → tests → services → APIs |
| [UAT_PLAN.md](./UAT_PLAN.md) | Human acceptance by persona |
| [TEST_ENVIRONMENT_MATRIX.md](./TEST_ENVIRONMENT_MATRIX.md) | LOCAL / CI / STAGING / UAT / PILOT |
| [SYSTEM_TEST_READINESS_REPORT.md](./SYSTEM_TEST_READINESS_REPORT.md) | Current scorecard, gaps, executability |

Machine-readable catalog: [`tests/system/test-catalog.yaml`](../../tests/system/test-catalog.yaml)

Execution entrypoints:

```bash
make system-test-design-check    # validate catalog + docs
make system-test-preflight       # disposable stack readiness
make system-test-smoke           # existing integration smoke (alias)
make system-test-golden-skeleton # FP-E2E-GOLDEN-001 scaffold (dry-run)
make staging-acceptance-pack     # checklist for when staging is restored
```

**Baseline SHA:** `37c2eb62ccf9377359eb5c2fdf6f71eb9d187140` (origin/main)  
**Branch:** `test/master-system-test-plan-v1`
