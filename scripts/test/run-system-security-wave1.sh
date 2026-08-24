#!/usr/bin/env bash
# System Test Wave 1 — Auth, RBAC, Tenant & Company Isolation
# WAVE1_STAGING_DEPENDENCY=NO — runs in CI/disposable Postgres only
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
  echo "=== WAVE1 SUITE: ${name} ==="
  if "$@"; then
    echo "PASS: ${name}"
    PASS=$((PASS + 1))
  else
    echo "FAIL: ${name}" >&2
    FAIL=$((FAIL + 1))
    return 1
  fi
}

# Continue collecting failures but exit non-zero at end
set +e

run_suite "FP-AUTH gateway middleware" \
  bash -lc 'cd services/api-gateway && go test ./internal/http/middleware/... -run "Test(Auth|FP_AUTH)" -count=1'

run_suite "FP-SEC gateway RBAC guards" \
  bash -lc 'cd services/api-gateway && go test ./internal/shipmentrbac/... ./internal/fleetrbac/... ./internal/rfxrbac/... ./internal/billingrbac/... ./internal/settlementrbac/... ./internal/paymentrbac/... ./internal/driverrbac/... -count=1'

run_suite "FP-SEC control tower RBAC" \
  bash -lc 'cd services/api-gateway && go test ./internal/controltower/... -count=1'

run_suite "FP-SEC wave1 integration gate" \
  bash -lc 'cd services/api-gateway && go test -tags=integration ./internal/integration/securitywave1/... -count=1'

run_suite "FP-SEC freight cost public" \
  bash -lc 'cd services/api-gateway && go test -tags=integration ./internal/integration/freightcostpublic/... -run "Sec|Auth|CrossTenant|Spoof|Public" -count=1'

run_suite "FP-SEC contract rate public" \
  bash -lc 'cd services/api-gateway && go test -tags=integration ./internal/integration/contractratepublic/... -run "Sec|Cross|Auth|Spoof" -count=1'

run_suite "FP-E2E-SEC RFx tenant/company" \
  bash -lc 'cd services/rfx-service && go test -tags=integration ./internal/integration/laneownership/... ./internal/integration/enterprise/... -run "Cross|CompanyIsolation|Foreign|CrossTenant|RealDBCompany" -count=1'

run_suite "FP-E2E-SEC billing settlement" \
  bash -lc 'cd services/billing-register-service && go test -tags=integration ./internal/integration/freightsettlement/... -run "CrossTenant|Competitor|ForeignBuyer|Foreign" -count=1'

run_suite "FP-E2E-SEC transport order snapshot" \
  bash -lc 'cd services/transport-order-service && go test -tags=integration ./internal/integration/pricingsnapshot/... -run "CrossTenant" -count=1'

run_suite "FP-E2E-SEC contract rate DB" \
  bash -lc 'cd services/contract-rate-service && go test -tags=integration ./internal/integration/contractrate/... -run "CrossTenant" -count=1'

run_suite "FP-E2E-SEC document POD" \
  bash -lc 'cd services/document-service && go test -tags=integration ./internal/integration/podupload/... -run "CrossTenant" -count=1'

run_suite "FP-E2E-SEC shipment tenant handlers" \
  bash -lc 'cd services/shipment-service && go test ./internal/http/handlers/... -run "Tenant|Actor|BodyTenant" -count=1'

run_suite "FP-E2E-SEC CT read model blackbox" \
  bash -lc 'cd services/api-gateway && go test -tags=integration ./internal/integration/controltowerreadmodel/... -run "TestBlackBoxTenantIsolation$" -count=1'

run_suite "FP-E2E-SEC freight cost service security" \
  bash -lc 'cd services/freight-cost-service && go test -tags=integration ./internal/integration/variance/... -run "SEC|CrossTenant|TenantA|FakePlatform" -count=1'

set -e

echo ""
echo "=== WAVE 1 SUMMARY ==="
echo "SUITES_EXECUTED=${EXECUTED}"
echo "SUITES_PASS=${PASS}"
echo "SUITES_FAIL=${FAIL}"

if [[ "$FAIL" -ne 0 ]]; then
  echo "SYSTEM_SECURITY_WAVE1=FAIL"
  exit 1
fi
echo "SYSTEM_SECURITY_WAVE1=PASS"
exit 0
