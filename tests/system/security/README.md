# System Test Wave 1 — Security

Executable Wave 1 gate for authentication, RBAC, tenant and company isolation.

## Run

```bash
make system-test-wave1-security
```

Requires `TEST_DATABASE_URL` for DB-backed suites (CI provides Postgres).

## Structure

| Path | Purpose |
|------|---------|
| `WAVE1_MANIFEST.yaml` | Catalog ID → test mapping |
| `../../scripts/test/run-system-security-wave1.sh` | Orchestrator |
| `../../services/api-gateway/internal/integration/securitywave1/` | Wave 1 gateway security tests |

## Invariants

- `AUTH_MIDDLEWARE_REAL=YES` — tests use actual JWT verifier
- `WAVE1_STAGING_DEPENDENCY=NO`
- No mockAuth acceptance evidence

## Catalog IDs

See `WAVE1_MANIFEST.yaml` for FP-AUTH-*, FP-SEC-*, FP-E2E-SEC-* mapping.
