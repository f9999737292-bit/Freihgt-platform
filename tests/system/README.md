# System Tests — Freight Platform v1

Master catalog and golden-path automation skeleton for cross-domain acceptance.

## Layout

```
tests/system/
├── test-catalog.yaml          # machine-readable test index
├── fixtures/tenant-spec.yaml  # Tenant A/B data model
├── golden/fp_e2e_golden_001.sh
└── README.md
```

## Commands

```bash
make system-test-design-check
make system-test-golden-skeleton    # DRY_RUN=1 default
DRY_RUN=0 make system-test-golden-skeleton   # requires full stack
```

## Status rules

- **PASS** — only with execution evidence (CI run, staging log)
- **PLANNED** — designed, not automated
- **IMPLEMENTED_NOT_EXECUTED** — skeleton/code exists, not run
- **BLOCKED_STAGING** — requires live staging

## Documentation

See [`docs/testing/README.md`](../docs/testing/README.md).
