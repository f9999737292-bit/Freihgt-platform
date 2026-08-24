# Test Data Model — Freight Platform v1

## Tenants

- **TENANT_A:** `tenant-a-fp-test` — SHIPPER_A, CARRIER_A1, CARRIER_A2, FORWARDER_A, FINANCE_A, CONSIGNEE_A
- **TENANT_B:** `tenant-b-fp-test` — independent mirror

## Conventions

- Numbers: `TEST-{DOMAIN}-{runId}` (see `tests/integration/README.md`)
- Emails: `{role}-{runId}@tenant-a.test.local`
- Currencies: RUB (primary), EUR (secondary)
- Lanes: Moscow→SPb, Moscow→Kazan

## Fixture spec

`tests/system/fixtures/tenant-spec.yaml`

## Reset

| Env | Strategy |
|-----|----------|
| CI | ephemeral Postgres per job |
| Local | unique `SMOKE_RUN_ID` or volume reset |
| Staging | seed + cleanup checklist |

`TEST_REPEATABILITY=YES` — acceptance must not depend on prior run leftovers.

## Gap

FTST002: unified `make test-data-reset` not yet implemented — use migrate-up + fixture builder.
