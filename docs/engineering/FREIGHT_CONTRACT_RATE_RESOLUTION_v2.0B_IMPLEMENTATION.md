# FREIGHT CONTRACT RATE RESOLUTION v2.0B — Implementation

## GIT

| Field | Value |
|-------|-------|
| Base slice | v2.0A (`000048`) |
| Migration | `000049_contract_rate_lines_resolution_v2.0B` |
| Branch | `feat/freight-contract-rate-resolution-v2.0B` |

## SCOPE DELIVERED

- `rate_line` and `rate_component` tables with lane/component constraints
- Draft-only CRUD for rate lines and components (internal S2S)
- Rate card version activation with SERIALIZABLE cross-card lane conflict detection
- Deterministic contract rate resolution (`POST /internal/v1/rates/resolve`)
- Manual spot fallback when zero contract candidates (server-side role check via `ListUserGlobalRoleCodes`)
- RFQ_AWARD / SPOT_BID / `bid_id` / `award_link_id` fail closed with `PRICING_SOURCE_NOT_AVAILABLE`
- Decimal money via `shopspring/decimal`; JSON amounts as decimal strings
- WAITING / DETENTION returned as accessorial rules, excluded from pre-execution total
- Observability counters: `rate_resolution_*`, `rate_version_activation_total`

## SECURITY

All v2.0B routes remain under `/internal/v1/*` with `internalauth.Middleware`. No api-gateway routes added.

Manual spot authorization uses global role codes (`PLATFORM_ADMIN`, `SHIPPER_ADMIN`, `PROCUREMENT_MANAGER`, `FORWARDER_MANAGER`). Carrier actors are always denied manual spot.

## API (internal S2S)

| Method | Path | Purpose |
|--------|------|---------|
| POST | `/rate-card-versions/{versionId}/rate-lines` | Create draft rate line |
| GET | `/rate-card-versions/{versionId}/rate-lines` | List lines |
| GET/PATCH/DELETE | `/rate-lines/{id}` | Line CRUD |
| POST/GET | `/rate-lines/{lineId}/components` | Component CRUD |
| PATCH/DELETE | `/rate-components/{id}` | Component update/delete |
| POST | `/rate-card-versions/{versionId}/activate` | Activate draft version |
| POST | `/rates/resolve` | Deterministic rate resolution |

## RESOLUTION SEMANTICS

1. Load ACTIVE contract rate candidates for buyer/carrier/lane/date
2. Filter by contract + version validity and optional currency
3. `0` eligible → `NO_MATCH` (manual spot if authorized + amount supplied)
4. `1` eligible → `MATCHED` with `CONTRACT_RATE` breakdown
5. `>1` eligible → `AMBIGUOUS` (fail closed)

Pre-execution total = rounded BASE_FREIGHT + rounded FUEL_SURCHARGE (if present). Accessorial unit rules are not summed into `total_amount`.

## TESTS

- Unit: `resolver_test.go`, `money_test.go`
- Integration (`-tags=integration`, `TEST_DATABASE_URL`): `postgres_gates_v2_0B_integration_test.go` gates **CR-B-001 … CR-B-054**

Run:

```bash
cd services/contract-rate-service
go test ./...
go test -tags=integration ./internal/integration/contractrate/...
```

## OUT OF SCOPE (v2.0B)

- RFx pricing adapter (v2.0C)
- Transport order snapshots (v2.0C)
- Public gateway / OpenAPI (v2.0E)
- api-gateway route registration

## INVARIANTS PRESERVED

- `uq_rate_card_version_one_active` partial unique index
- Append-only audit events including `MANUAL_SPOT_RESOLVED`
- Tenant-scoped reads/writes
