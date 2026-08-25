#!/usr/bin/env bash
# System Test Wave 3 — Failure, Recovery, Resilience & Operational Readiness
# WAVE3_STAGING_DEPENDENCY=NO — disposable Postgres + Kafka in CI only
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

export CI="${CI:-true}"
export REQUIRE_TEST_DATABASE=1
export REQUIRE_TEST_KAFKA="${REQUIRE_TEST_KAFKA:-1}"

PASS=0
FAIL=0
SKIP=0
EXECUTED=0

run_suite() {
  local name="$1"
  shift
  EXECUTED=$((EXECUTED + 1))
  echo ""
  echo "=== WAVE3 SUITE: ${name} ==="
  if "$@"; then
    echo "PASS: ${name}"
    PASS=$((PASS + 1))
  else
    echo "FAIL: ${name}" >&2
    FAIL=$((FAIL + 1))
    return 1
  fi
}

run_suite_optional() {
  local name="$1"
  shift
  EXECUTED=$((EXECUTED + 1))
  echo ""
  echo "=== WAVE3 SUITE (NON_BLOCKING): ${name} ==="
  if "$@"; then
    echo "PASS (informational): ${name}"
    SKIP=$((SKIP + 1))
  else
    echo "SKIP/BLOCKED (informational): ${name}" >&2
    SKIP=$((SKIP + 1))
  fi
  return 0
}

set +e

# --- F002 Database transaction safety (shipment + outbox) ---
run_suite "W3-01 DB transaction atomicity (shipment/outbox)" \
  bash -lc 'cd services/shipment-service && go test -tags=integration ./internal/integration/outbox/... -run "TestAtomic|TestRollback|TestOptimisticLock" -count=1'

run_suite "W3-02 DB partial commit protection (CT projection inbox)" \
  bash -lc 'cd services/control-tower-read-model-service && go test -tags=integration ./internal/integration/postgres/... -run "TestProjectionFailureRollsBackInbox" -count=1'

# --- F003/F011 Outbox worker, state, claim, replay ---
run_suite "W3-03 Outbox worker retry and terminal states" \
  bash -lc 'cd services/shipment-service && go test -tags=integration ./internal/integration/outbox/... -run "TestWorker|TestReleaseWithRetry|TestMarkFailed|TestMarkPublished" -count=1'

run_suite "W3-04 Outbox claim concurrency and stale lease recovery" \
  bash -lc 'cd services/shipment-service && go test -tags=integration ./internal/integration/outbox/... -run "TestConcurrent|TestStaleLease|TestPublishedAndFailed" -count=1'

run_suite "W3-05 Outbox replay idempotency" \
  bash -lc 'cd services/shipment-service && go test -tags=integration ./internal/integration/outbox/... -run "TestOutboxReplay" -count=1'

run_suite "W3-06 Outbox unit shutdown and retry classification" \
  bash -lc 'cd services/shipment-service && go test ./internal/outbox/... -run "TestWorker(Shutdown|Transient|Permanent|MaxAttempts|Cancelled)" -count=1'

# --- F004/F005 HTTP downstream failure (gateway CT proxy) ---
run_suite "W3-07 HTTP downstream 503/timeout (CT read-model proxy)" \
  bash -lc 'cd services/api-gateway && go test -tags=integration ./internal/integration/controltowerreadmodel/... -run "TestBlackBoxPrimaryFallback" -count=1'

run_suite "W3-08 HTTP downstream fail-closed (no secret leak)" \
  bash -lc 'cd services/api-gateway && go test -tags=integration ./internal/integration/controltowerreadmodel/... -run "TestBlackBoxClientHeadersAndNoSecretsInError" -count=1'

# --- F007/F008 Idempotency and concurrent mutation ---
run_suite "W3-09 HTTP idempotency (order execution)" \
  bash -lc 'cd services/shipment-service && go test -tags=integration ./internal/integration/orderexecution/... -run "Idempotent" -count=1'

run_suite "W3-10 HTTP idempotency + concurrency (settlement)" \
  bash -lc 'cd services/billing-register-service && go test -tags=integration ./internal/integration/freightsettlement/... -run "Test03DuplicateIdempotent|Test04Concurrent" -count=1'

run_suite "W3-11 Concurrent award protection (RFx)" \
  bash -lc 'cd services/rfx-service && go test -tags=integration ./internal/integration/enterprise/... -run "ConcurrentDoubleAward" -count=1'

# --- F009/F010/F011/F012 Kafka + outbox publish path ---
run_suite "W3-12 Kafka outbox E2E and broker unavailable recovery" \
  bash -lc 'cd services/shipment-service && go test -tags=integration ./internal/integration/kafka/... -run "TestWorkerPostgreSQLRedpanda|TestWorkerBrokerUnavailable|TestKafkaBrokerUnavailable" -count=1'

run_suite "W3-13 Kafka duplicate delivery (at-least-once)" \
  bash -lc 'cd services/shipment-service && go test -tags=integration ./internal/integration/kafka/... -run "TestDuplicateDelivery" -count=1'

# --- F013/F014 Consumer crash and restart ---
run_suite "W3-14 CT consumer restart and offset commit failure" \
  bash -lc 'cd services/control-tower-read-model-service && go test -tags=integration ./internal/integration/kafka/... -run "TestConsumerRestart|TestDeadLetterRestart" -count=1'

run_suite "W3-15 CT consumer valid event processing" \
  bash -lc 'cd services/control-tower-read-model-service && go test -tags=integration ./internal/integration/kafka/... -run "TestConsumerReadsValidEvent" -count=1'

# --- F012/F015/F016 Duplicate delivery, stale, ordering (CT projection) ---
run_suite "W3-16 CT projection duplicate and stale event safety" \
  bash -lc 'cd services/control-tower-read-model-service && go test -tags=integration ./internal/integration/postgres/... -run "TestProcessEventDuplicate|TestProcessEventStale|TestConcurrentProcessing" -count=1'

run_suite "W3-17 CT duplicate classification (not stale)" \
  bash -lc 'cd services/control-tower-read-model-service && go test -tags=integration ./internal/integration/postgres/... -run "TestDuplicateSameEventID" -count=1'

run_suite "W3-18 CT live projection stale after activation" \
  bash -lc 'cd services/control-tower-read-model-service && go test -tags=integration ./internal/integration/rebuild/... -run "TestStaleDuplicate|TestLiveEvent|TestLiveUpdate" -count=1'

# --- F017/F018 Control Tower rebuild ---
run_suite "W3-19 CT rebuild acceptance (historical + rollback)" \
  bash -lc 'cd services/control-tower-read-model-service && go test -tags=integration ./internal/integration/rebuild/... -run "TestHistoricalAcceptance|TestRollbackAcceptance" -count=1'

run_suite "W3-20 CT rebuild failure injection (activation/rollback)" \
  bash -lc 'cd services/control-tower-read-model-service && go test -tags=integration ./internal/integration/rebuild/... -run "TestActivationFailure|TestRollbackFailure" -count=1'

run_suite "W3-21 CT rebuild tenant isolation" \
  bash -lc 'cd services/control-tower-read-model-service && go test -tags=integration ./internal/integration/rebuild/... -run "TestActivationTenant|TestRollbackTenant|TestImportProtocolTenant" -count=1'

# --- F017/F021 CT Kafka catch-up (live path) ---
run_suite "W3-22 CT Kafka catch-up and gap recovery" \
  bash -lc 'cd services/control-tower-read-model-service && go test -tags=integration ./internal/integration/rebuild/... -run "TestKafkaCatchUp|TestGapAfterActivation|TestEventsDuringPause" -count=1'

# --- F019/F020 Financial resilience ---
run_suite "W3-23 Billing duplicate settlement safety" \
  bash -lc 'cd services/billing-register-service && go test -tags=integration ./internal/integration/freightsettlement/... -run "Test03Duplicate|Test04Concurrent|CrossTenant" -count=1'

run_suite "W3-24 Freight-cost ledger duplicate delivery and ordering" \
  bash -lc 'cd services/freight-cost-service && go test -tags=integration ./internal/integration/ledger/... -run "DuplicateDelivery|OutOfOrder|Duplicate|Replay|RebuildIdempotent" -count=1'

run_suite "W3-25 Freight-cost variance ingest dedupe" \
  bash -lc 'cd services/freight-cost-service && go test -tags=integration ./internal/integration/variance/... -run "TestFC_C_OUT_00[234]" -count=1'

# --- F022 Cross-tenant failure isolation ---
run_suite "W3-26 Cross-tenant financial isolation" \
  bash -lc 'cd services/freight-cost-service && go test -tags=integration ./internal/integration/ledger/... -run "SEC_002|SEC_004" -count=1'

run_suite "W3-27 Cross-tenant CT projection inbox" \
  bash -lc 'cd services/control-tower-read-model-service && go test -tags=integration ./internal/integration/postgres/... -run "TestProcessEventTenantIsolation|TestGetStatusSummaryIsTenantScoped" -count=1'

# --- F025 Graceful shutdown (unit-level worker) ---
run_suite "W3-28 Outbox worker graceful shutdown" \
  bash -lc 'cd services/shipment-service && go test ./internal/outbox/... -run "TestWorkerShutdownCancellation|TestWorkerCancelledBeforeStart" -count=1'

# --- F023 Backup/restore — staging pg_dump tooling not CI-safe (informational only) ---
run_suite_optional "W3-29 Backup/restore (staging pg_dump — BLOCKED in CI)" \
  bash -lc 'test -f scripts/ops/bintrans_ct_staging/bintrans_ct_staging_backup.sh && echo "Evidence: rebuild in-DB backup covered by W3-19/W3-20"'

# --- Section 22: Security regression (Wave 1) ---
run_suite "W3-30 Security regression (Wave 1 auth/RBAC/tenant/company)" \
  bash scripts/test/run-system-security-wave1.sh

set -e

echo ""
echo "=== WAVE 3 SUMMARY ==="
echo "SUITES_EXECUTED=${EXECUTED}"
echo "SUITES_PASS=${PASS}"
echo "SUITES_FAIL=${FAIL}"
echo "SUITES_NON_BLOCKING=${SKIP}"

if [[ "$FAIL" -ne 0 ]]; then
  echo "SYSTEM_TEST_WAVE3_RESILIENCE=FAIL"
  exit 1
fi
echo "SYSTEM_TEST_WAVE3_RESILIENCE=PASS"
exit 0
