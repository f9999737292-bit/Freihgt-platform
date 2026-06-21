#!/usr/bin/env bash
# One-off repro for UPD HTTP 400 diagnostic (old curl -d on Windows). Not part of CI.
set -euo pipefail

BILLING_REGISTER_SERVICE_URL="${BILLING_REGISTER_SERVICE_URL:-http://localhost:8087}"
TENANT_ID="${1:?tenant_id required}"
REGISTER_ID="${2:?register_id required}"
CARRIER="${3:?carrier required}"
SHIPPER="${4:?shipper required}"

DATA="$(cat <<EOF
{
  "tenant_id": "$TENANT_ID",
  "upd_number": "UPD-REPRO-OLD-CURL",
  "upd_date": "2026-07-16",
  "seller_company_id": "$CARRIER",
  "buyer_company_id": "$SHIPPER",
  "function_code": "СЧФДОП"
}
EOF
)"

echo "=== Repro A: curl -d \"\$DATA\" (Windows cmdline / legacy smoke-test) ==="
curl -sS -w "\nHTTP:%{http_code}\n" -X POST \
  "$BILLING_REGISTER_SERVICE_URL/v1/billing-registers/$REGISTER_ID/upd" \
  -H "Content-Type: application/json" \
  -d "$DATA" || true

echo
echo "=== Repro B: printf | curl --data-binary @- (fixed smoke-test) ==="
printf '%s' "$DATA" | curl -sS -w "\nHTTP:%{http_code}\n" -X POST \
  "$BILLING_REGISTER_SERVICE_URL/v1/billing-registers/$REGISTER_ID/upd" \
  -H "Content-Type: application/json; charset=utf-8" \
  --data-binary @- || true
