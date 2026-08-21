# FREIGHT CONTRACT & RATE PUBLIC HARDENING v2.0E — IMPLEMENTATION

## GIT

| Field | Value |
|-------|-------|
| Branch | `feat/freight-contract-rate-public-hardening-v2.0E` |
| Base SHA | `b6c2c7fe08c43d7a6bc9a2e76466c6cf46575358` |

## DISCOVERY

| Item | Before | After |
|------|--------|-------|
| Gateway role source | Tenant-global `/v1/auth/me` via `routeauth` | Company membership roles via identity `/v1/users/{id}/companies` |
| Company membership | Query `company_id` (billing/payment) | Header `X-Company-ID` captured pre-strip in JWT middleware |
| PLATFORM_ADMIN | Proven tenant-global via `/v1/users/{id}/roles` where `company_id` absent (`ur.company_id IS NULL`) | Same proven path in `ratesrbac` |
| Internal auth | N/A at gateway for contract-rate | `INTERNAL_SERVICE_TOKEN` injected server-side only |

## PUBLIC ARCHITECTURE

```
Browser → API Gateway (JWT)
       → ratesrbac (membership + company-scoped roles)
       → contractrates adapter (/api/v1 → /internal/v1)
       → contract-rate-service (S2S + actor headers)
       → PostgreSQL
```

## TRUST BOUNDARY

- JWT is sole authority for tenant/user.
- `X-Company-ID` is a selector validated against active membership.
- Client `X-Tenant-ID`, `X-User-ID`, `X-Actor-Kind`, `X-Internal-Service-Token` stripped/replaced.
- Downstream headers rebuilt in `contractrates.Client.Forward`.

## COMPANY-SCOPED RBAC

Package: `services/api-gateway/internal/ratesrbac/`

- Read: membership roles in selected company (+ tenant-global PLATFORM_ADMIN).
- Mutate: buyer actor + PROCUREMENT_MANAGER | SHIPPER_ADMIN | FORWARDER_MANAGER (+ PLATFORM_ADMIN).
- **CROSS_COMPANY_ROLE_BLEED=DENY** — mandatory test `TestRatesRBACCrossCompanyRoleBleedDenied`.

## GATEWAY ADAPTER

Package: `services/api-gateway/internal/contractrates/`

- Strict path allowlist
- Public DTO validation (`DisallowUnknownFields` / field allowlists)
- Public simulation strips manual spot / RFx controls
- S2S token never forwarded from browser

## NULLABLE PATCH (E_GAP_002)

Tri-state `domain.NullableDatePatch` for contract and rate version `valid_to`:

- omitted → preserve
- `null` → clear
- date → set

## RATE VERSION METADATA (E_GAP_001)

`mapRateVersion` exposes truthful `activated_at` (null for DRAFT). Frontend version table shows `created_at`, `activated_at`, supersedes relation.

## FRONTEND

- Types updated with `activated_at`
- Version history UI displays audit metadata (RU/EN/ZH)
- Feature flag remains default **OFF** (`NUXT_PUBLIC_CONTRACT_RATE_WORKSPACE_ENABLED`)

## PUBLIC E2E

Cross-module integration harness (build tag `integration`):

```
services/contract-rate-service/testkit/     ← real router/service/repos (same module, may import internal/*)
services/api-gateway/internal/integration/contractratepublic/  ← imports testkit ONLY (never internal/*)
```

| Gate | Test |
|------|------|
| E-E2E-001 | Buyer contract lifecycle (create → activate → metadata → suspend → reactivate) |
| E-E2E-002 | Full rate flow (card, version, lane, 4 components, activate, DB persistence) |
| E-E2E-003 | Public simulation MATCHED + forbidden manual_spot/RFx fields → 400 at gateway |
| E-E2E-004 | CARRIER_ADMIN read-only + mutation 403 |
| E-E2E-005 | Terminated contract historical reads preserved, no repricing |
| E-E2E-006 | Cross-company role bleed (SHIPPER_ADMIN@A + SHIPPER_LOGIST@B, PATCH real B contract → 403) |
| Security | Header spoof deny (tenant/user/actor/internal-token), unknown company → 403 |

Stack: PostgreSQL 16, real API gateway (JWT + ratesrbac + contractrates), real contract-rate HTTP via testkit, identity membership HTTP mock.

`PUBLIC_E2E=PASS` after CI job `contract-rate-public-e2e` green on merge head (replaces prior `NOT_EXECUTED_DUE_TO_GO_INTERNAL_IMPORT_FAILURE`).

## CI

Added job: `contract-rate-public-e2e` with `REQUIRE_TEST_DATABASE=1`.

## DEPLOYMENT

Gateway env:

- `CONTRACT_RATE_SERVICE_URL` (default `http://localhost:8091`)
- `INTERNAL_SERVICE_TOKEN` (must match contract-rate-service)

## OUT OF_SCOPE

No payment/billing/settlement/RFx product changes. No historical repricing. No migrations.

## FINAL GATES

See PR checklist. Draft PR — **do not merge** until all gates green.
