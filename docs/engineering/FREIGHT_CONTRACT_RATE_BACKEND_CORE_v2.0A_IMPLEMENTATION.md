# FREIGHT CONTRACT & RATE BACKEND CORE v2.0A — Implementation

## GIT

| Field | Value |
|-------|-------|
| Architecture base | `cb4fff93419c7274147aa8abb49e885ce03c277b` (main after PR #34) |
| Review PR #35 merged | YES → `a42ccbb9ab3614d5025fba9e9c33dbf763d2d8ae` on arch branch |
| PR #34 merged to main | YES → `cb4fff9` |
| Migration | `000048_contract_rate_backend_core_v2.0A` |

## DISCOVERY

| Item | Value |
|------|-------|
| Latest migration before v2.0A | `000047` |
| v2.0A migration | `000048` |
| S2S pattern | `packages/shared-go/internalauth` — `X-Internal-Service-Token`, constant-time compare |
| OpenAPI | Not modified (internal-only v2.0A; public exposure deferred v2.0E) |
| Module | `github.com/freight-platform/contract-rate-service` |
| Port | `8091` |

## SERVICE STRUCTURE

`services/contract-rate-service/` — cmd/server, domain, repository, service, http (internal `/internal/v1`), config, platform.

## DATABASE

Schema `contract_rate`:

- `transport_contract`
- `rate_card`
- `rate_card_version` — partial unique index `uq_rate_card_version_one_active`
- `audit_event` — append-only

**NOT created:** `rate_line`, `rate_component`, `transport_order_rate_snapshots`

## DOMAIN

Contract lifecycle: DRAFT→ACTIVE→SUSPENDED↔ACTIVE→TERMINATED/EXPIRED; DRAFT→CANCELLED. Lazy EXPIRED on read/mutation when `valid_to` passed.

Draft version CRUD only; version activation deferred to v2.0B.

## S2S SECURITY

All routes under `/internal/v1/*` wrapped with `internalauth.Middleware`. No api-gateway routes added.

## TESTS

- Unit: domain lifecycle, currency, internal auth matrix
- Integration (`-tags=integration`, `TEST_DATABASE_URL`): lifecycle, one-ACTIVE DB constraint, concurrent version numbers, audit emission

## OUT OF SCOPE

No rate resolution, RFx adapter, TO snapshot, settlement, payment, frontend, public gateway.

## FINAL GATES

See completion report in PR body.
