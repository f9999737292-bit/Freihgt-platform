# System Test Wave 3 — Failure, Recovery, Resilience

Maps Wave 3 gate groups to executable suites. Entrypoint: `make system-test-wave3-resilience`.

## Prerequisites

| Variable | Required | Purpose |
|----------|----------|---------|
| `TEST_DATABASE_URL` | Yes (CI) | Disposable PostgreSQL |
| `REQUIRE_TEST_DATABASE=1` | CI | Fail if DB unavailable |
| `TEST_KAFKA_BROKERS` | Yes (CI) | Kafka/Redpanda for live messaging tests |
| `REQUIRE_TEST_KAFKA=1` | CI | Fail if Kafka brokers unset |

Local (with Docker compose messaging profile):

```bash
export TEST_DATABASE_URL='postgres://freight:freight_password@localhost:5432/freight_platform?sslmode=disable'
export TEST_KAFKA_BROKERS='localhost:19092'
make system-test-wave3-resilience
```

## Gate Groups

| Gate | Scenarios | Suite prefix |
|------|-----------|--------------|
| DB transaction safety | F001–F003 | W3-01, W3-02 |
| Outbox recovery | F010–F011 | W3-03–W3-06 |
| HTTP downstream | F004–F006 | W3-07, W3-08 |
| Idempotency | F007–F008 | W3-09–W3-11 |
| Kafka publish | F009–F012 | W3-12, W3-13 |
| Consumer crash | F013–F014 | W3-14, W3-15 |
| Event ordering | F015–F016 | W3-16–W3-18 |
| CT rebuild | F017–F018 | W3-19–W3-21 |
| CT live catch-up | F021 | W3-22 |
| Financial | F019–F020 | W3-23–W3-25 |
| Tenant isolation | F022 | W3-26, W3-27 |
| Graceful shutdown | F025 | W3-28 |
| Backup (informational) | F023 | W3-29 NON_BLOCKING |
| Security regression | — | W3-30 |

## Failure Matrix

See `docs/testing/system-wave3-failure-matrix.md`.

## Discovery

See `docs/testing/system-wave3-discovery.md`.
