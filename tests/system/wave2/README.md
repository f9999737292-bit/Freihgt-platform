# System Test Wave 2

Core business flow and cross-service integration verification.

## Entry point

```bash
make system-test-wave2-core-business-flow
```

Requires disposable PostgreSQL (`TEST_DATABASE_URL`, `REQUIRE_TEST_DATABASE=1` in CI).

## Golden flow tests

| Test | Package | Scope |
|------|---------|-------|
| `TestSYSTEM_WAVE2_GOLDEN_RFX_TO_AWARD` | `services/rfx-service/internal/integration/systemwave2` | Tenant A: freight request → bids → award |
| `TestSYSTEM_WAVE2_GOLDEN_FLOW` | `services/shipment-service/internal/integration/systemwave2` | Bid → shipment → FSM → outbox lineage |

## Manifest

See `WAVE2_MANIFEST.yaml` for W2-01 … W2-20 gate mapping.

## Relationship to Wave 1

Wave 1 covers identity/RBAC/tenant isolation in isolation. Wave 2 re-validates security **in business-process context** (procurement → execution → settlement) without duplicating the full Wave 1 matrix.
