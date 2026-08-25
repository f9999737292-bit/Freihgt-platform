#!/usr/bin/env bash
# System Test Wave 2 — Core Business Flow & Cross-Service Integration
# WAVE2_STAGING_DEPENDENCY=NO — runs in CI/disposable Postgres only
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

export CI="${CI:-true}"
export REQUIRE_TEST_DATABASE=1

PASS=0
FAIL=0
EXECUTED=0

run_suite() {
  local name="$1"
  shift
  EXECUTED=$((EXECUTED + 1))
  echo ""
  echo "=== WAVE2 SUITE: ${name} ==="
  if "$@"; then
    echo "PASS: ${name}"
    PASS=$((PASS + 1))
  else
    echo "FAIL: ${name}" >&2
    FAIL=$((FAIL + 1))
    return 1
  fi
}

set +e

run_suite "W2-01 Golden RFx → Award" \
  bash -lc 'cd services/rfx-service && go test -tags=integration ./internal/integration/systemwave2/... -run "TestSYSTEM_WAVE2_GOLDEN_RFX_TO_AWARD|TestW2_Money" -count=1'

run_suite "W2-02 Award lineage + duplicate protection" \
  bash -lc 'cd services/rfx-service && go test -tags=integration ./internal/integration/systemwave2/... -run "TestW2_DuplicateAccept|TestW2_IDOR" -count=1'

run_suite "W2-03 Order → Shipment golden flow" \
  bash -lc 'cd services/shipment-service && go test -tags=integration ./internal/integration/systemwave2/... -run "TestSYSTEM_WAVE2_GOLDEN_FLOW" -count=1'

run_suite "W2-04 Driver/Vehicle assignment isolation" \
  bash -lc 'cd services/shipment-service && go test -tags=integration ./internal/integration/systemwave2/... -run "ForeignDriver|ForeignVehicle" -count=1'

run_suite "W2-05 Shipment FSM invalid transitions" \
  bash -lc 'cd services/shipment-service && go test -tags=integration ./internal/integration/systemwave2/... -run "InvalidFSM" -count=1'

run_suite "W2-06 Cross-tenant procurement" \
  bash -lc 'cd services/rfx-service && go test -tags=integration ./internal/integration/systemwave2/... -run "CrossTenant|CrossCompany" -count=1'

run_suite "W2-07 Outbox on FSM progression" \
  bash -lc 'cd services/shipment-service && go test -tags=integration ./internal/integration/outbox/... -run "Transaction|State" -count=1'

run_suite "W2-08 Control Tower tenant isolation (business context)" \
  bash -lc 'cd services/api-gateway && go test -tags=integration ./internal/integration/controltowerreadmodel/... -run "TestBlackBoxTenantIsolation$" -count=1'

run_suite "W2-09 Settlement readiness from shipment" \
  bash -lc 'cd services/shipment-service && go test -tags=integration ./internal/integration/systemwave2/... -run "ReadyForBilling|Golden_FLOW" -count=1'

run_suite "W2-10 Settlement cross-service e2e" \
  bash -lc 'cd services/billing-register-service && go test -tags=integration ./internal/integration/freightsettlement/... -run "TestCE2E002" -count=1'

run_suite "W2-11 Billing closing integration" \
  bash -lc 'cd services/billing-register-service && go test -tags=integration ./internal/integration/freightbillingclosing/... -run "Test06CrossTenant|TestLegacyMarkPaidHTTPCrossTenant" -count=1'

run_suite "W2-12 Freight cost security (ledger scope)" \
  bash -lc 'cd services/freight-cost-service && go test -tags=integration ./internal/integration/ledger/... -run "Ingest|Ledger" -count=1'

run_suite "W2-13 Cross-tenant settlement" \
  bash -lc 'cd services/billing-register-service && go test -tags=integration ./internal/integration/freightsettlement/... -run "CrossTenant" -count=1'

run_suite "W2-14 Cross-company bid isolation" \
  bash -lc 'cd services/rfx-service && go test -tags=integration ./internal/integration/enterprise/... -run "CrossCompany|ConcurrentDoubleAward" -count=1'

run_suite "W2-15 IDOR matrix (procurement)" \
  bash -lc 'cd services/rfx-service && go test -tags=integration ./internal/integration/systemwave2/... -run "IDOR" -count=1'

run_suite "W2-16 Idempotency (order execution + settlement)" \
  bash -lc 'cd services/shipment-service && go test -tags=integration ./internal/integration/orderexecution/... -run "Idempotent" -count=1 && cd ../billing-register-service && go test -tags=integration ./internal/integration/freightsettlement/... -run "Test03DuplicateIdempotent|Test04Concurrent" -count=1'

run_suite "W2-17 Concurrency (double award)" \
  bash -lc 'cd services/rfx-service && go test -tags=integration ./internal/integration/enterprise/... -run "ConcurrentDoubleAward" -count=1'

run_suite "W2-18 Data integrity + tenant lineage" \
  bash -lc 'cd services/shipment-service && go test -tags=integration ./internal/integration/systemwave2/... -run "DataIntegrity|CrossServiceTenant" -count=1'

run_suite "W2-19 Money integrity" \
  bash -lc 'cd services/rfx-service && go test -tags=integration ./internal/integration/systemwave2/... -run "MoneyIntegrity" -count=1'

run_suite "W2-20 Header spoof business regression" \
  bash -lc 'cd services/api-gateway && go test -tags=integration ./internal/integration/systemwave2/... -count=1'

set -e

echo ""
echo "=== WAVE 2 SUMMARY ==="
echo "SUITES_EXECUTED=${EXECUTED}"
echo "SUITES_PASS=${PASS}"
echo "SUITES_FAIL=${FAIL}"

if [[ "$FAIL" -ne 0 ]]; then
  echo "SYSTEM_WAVE2_CORE_BUSINESS_FLOW=FAIL"
  exit 1
fi
echo "SYSTEM_WAVE2_CORE_BUSINESS_FLOW=PASS"
exit 0
