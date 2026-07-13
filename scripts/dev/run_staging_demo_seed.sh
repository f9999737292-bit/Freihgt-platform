#!/usr/bin/env bash
# Run demo seed on Selectel staging server (SSH target: /opt/bintrans/freight-platform).
# Requires: platform up, migrations applied, DEMO_PASSWORD in env (not stored in repo).
set -euo pipefail

step() { echo "==> $1" >&2; }
pass() { echo "OK: $1" >&2; }

export API_GATEWAY_URL="${API_GATEWAY_URL:-http://161.104.53.221}"
export TENANT_ID="${TENANT_ID:-74519f22-ff9b-4a8b-8fff-a958c689682f}"
export IDENTITY_SERVICE_URL="${IDENTITY_SERVICE_URL:-http://127.0.0.1:8081}"
export COMPANY_SERVICE_URL="${COMPANY_SERVICE_URL:-http://127.0.0.1:8082}"
export TRANSPORT_ORDER_SERVICE_URL="${TRANSPORT_ORDER_SERVICE_URL:-http://127.0.0.1:8083}"
export RFX_SERVICE_URL="${RFX_SERVICE_URL:-http://127.0.0.1:8084}"
export SHIPMENT_SERVICE_URL="${SHIPMENT_SERVICE_URL:-http://127.0.0.1:8085}"
export DOCUMENT_SERVICE_URL="${DOCUMENT_SERVICE_URL:-http://127.0.0.1:8086}"
export BILLING_REGISTER_SERVICE_URL="${BILLING_REGISTER_SERVICE_URL:-http://127.0.0.1:8087}"
export ADMIN_EMAIL="${ADMIN_EMAIL:-admin@bintrans.local}"
export SHIPPER_EMAIL="${SHIPPER_EMAIL:-shipper@bintrans.local}"
export CARRIER_EMAIL="${CARRIER_EMAIL:-carrier@bintrans.local}"
export FORWARDER_EMAIL="${FORWARDER_EMAIL:-forwarder@bintrans.local}"
export CONSIGNEE_EMAIL="${CONSIGNEE_EMAIL:-consignee@bintrans.local}"

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

step "Check API Gateway health"
if ! curl -sf "${API_GATEWAY_URL}/health" >/dev/null; then
  echo "ERROR: API Gateway unavailable at ${API_GATEWAY_URL}/health" >&2
  exit 1
fi
pass "API Gateway healthy"

step "Run seed-demo-data (STG-LIM-005)"
make seed-demo-data

step "Run seed-lowcode-demo custom field values (STG-LIM-006)"
make seed-lowcode-demo

pass "Staging demo seed completed"
echo ""
echo "Next: run read-only verification"
echo "  scripts/dev/verify_staging_demo_seed_readonly.sh"
echo "Or from Windows:"
echo "  pwsh scripts/dev/Verify-StagingDemoSeed.ps1"
