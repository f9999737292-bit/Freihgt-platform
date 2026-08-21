# Freight Cost Foundation v2.1A — Implementation

**Status:** RUNTIME IMPLEMENTED (Draft PR)  
**Authorization base:** `33bf88346680a82fe442300c96af52717d658543` (main after planning PR #43)  
**Contract:** `docs/engineering/FREIGHT_COST_FOUNDATION_v2.1A_IMPLEMENTATION_PLAN.md`

---

## Deliverables

| # | Item | Status |
|---|------|--------|
| 1 | `services/freight-cost-service` — stateless Go service on port **8092** | DONE |
| 2 | Domain types + pure functions (Money, finality, accrual, reconciliation, view scope) | DONE |
| 3 | Internal API `GET /internal/v1/freight-cost/transport-orders/{transportOrderId}` | DONE |
| 4 | Transport-order internal `GET /internal/v1/transport-orders/{transportOrderId}/rate-snapshot` | DONE |
| 5 | Provider interfaces (transport live; settlement/billing/payment stubbed) | DONE |
| 6 | Tests: FC-A-DOM/SEC/SRC/API/E2E (32) + TO-CR (10) | DONE |
| 7 | CI matrix + `go.work` entry | DONE |
| 8 | No migrations, no ledger, no gateway/frontend | VERIFIED |

---

## Service structure

```
services/freight-cost-service/
  cmd/server/main.go
  internal/config/
  internal/domain/          # Money, finality, accrual, reconciliation, view scope
  internal/security/        # TrustedActor, company access
  internal/provider/        # Canonical read interfaces + stubs
  internal/client/transport_order/
  internal/service/
  internal/http/            # router, handlers, dto
  internal/platform/        # errors, respond, metrics, logger
  Dockerfile                # EXPOSE 8092
  README.md
```

Transport-order additions:

- `internal/service/rate_snapshot_read_service.go`
- `internal/http/handlers/rate_snapshot_internal_handler.go`
- `internal/repository/priced_order_repository.go` — `GetOrderByID`, `GetSnapshotByTransportOrder`

---

## Frozen runtime semantics (v2.1A)

| Rule | Value |
|------|-------|
| `data_stage` | `PLANNED_ONLY` |
| `financial_finality` | `NOT_EVALUATED` |
| Cross-tenant resource | `404 NOT_FOUND` |
| Same-tenant wrong company | `403 FORBIDDEN` |
| Unpriced / missing snapshot (TO) | `409 CONFLICT` |
| Downstream unavailable | `503 SERVICE_UNAVAILABLE` |
| Invalid downstream decimal/DTO | `502 BAD_GATEWAY` |
| Downstream 200 with tenant/order identity mismatch | `502 BAD_GATEWAY` (not 403/404) |
| Downstream zero UUID canonical facts | `502 BAD_GATEWAY` |
| Negative planned `total_amount` | `502 BAD_GATEWAY` |
| Zero planned `total_amount` | **ALLOW** (`"0.00"`) |
| Money wire format | decimal string, scale 2 |
| Unknown money | JSON `null` (`*Money == nil`); not `omitempty` |
| Platform admin HTTP | denied (`400` on actor parse) |

### Downstream canonical snapshot validation

Transport HTTP client (`internal/client/transport_order`) fail-closes on HTTP 200 payloads until all checks pass:

| Gate | Value |
|------|-------|
| `DOWNSTREAM_IDENTITY_BINDING` | YES — `tenant_id` and `transport_order_id` must match request |
| `DOWNSTREAM_INVALID_CANONICAL_FACT` | `502 BAD_GATEWAY` |
| `NEGATIVE_PLANNED_AMOUNT` | DENY |
| `ZERO_PLANNED_AMOUNT` | ALLOW |
| `pricing_model_version` | exactly `SNAPSHOT_V1` |
| `pricing_source` | non-empty |
| Source success metric | recorded only after full validation |

Cross-tenant lookup semantics:

- Transport-order returns `404` for absent resource in tenant scope → propagate `404`
- Transport-order returns `200` but body `tenant_id` ≠ requested tenant → `502` (contract violation, not auth)

---

## Authentication

Inbound freight-cost and transport internal routes use `packages/shared-go/internalauth`:

- `X-Internal-Service-Token` validates S2S caller class
- Actor headers (`X-Tenant-ID`, `X-User-ID`, `X-Company-ID`, `X-Actor-Kind`) are trusted forwarded context after token gate
- S2S token does **not** bind actor identity

---

## Tests

### freight-cost-service (32)

| Family | Count | Package |
|--------|-------|---------|
| FC-A-DOM-* | 10 | `internal/domain` |
| FC-A-SEC-* | 5 | `internal/security`, `internal/integration/planned_cost` |
| FC-A-SRC-* | 11 | `internal/client/transport_order` |
| FC-A-API-* | 3 | `internal/http/dto` |
| FC-A-E2E-* | 3 | `internal/integration/planned_cost` |

### transport-order-service (10)

| ID | Description |
|----|-------------|
| TO-CR-001 | Missing internal token → 401 |
| TO-CR-002 | Missing tenant → 400 |
| TO-CR-003 | Priced snapshot → 200 |
| TO-CR-004 | Not found → 404 |
| TO-CR-005 | Invalid transport order id → 400 |
| TO-CR-006 | Unpriced order → 409 |
| TO-CR-007 | Missing snapshot → 409 |
| TO-CR-008 | Zero total → `"0.00"` |
| TO-CR-009 | Invalid tenant id → 400 |
| TO-CR-010 | Wrong internal token → 401 |

Run:

```bash
cd services/freight-cost-service && go test ./...
cd services/transport-order-service && go test ./internal/service/... ./internal/http/handlers/...
```

---

## Out of scope (deferred v2.1B+)

Ledger, event ingestion, migrations, settlement/billing/payment live providers, public API, gateway routes, frontend, docker-compose production wiring.

---

## CI

`.github/workflows/ci.yml` backend-go-check matrix includes `services/freight-cost-service`.
