#!/usr/bin/env bash
# Setup APPROVED billing register and run UPD HTTP 400 repro (legacy curl -d vs fixed stdin).
set -euo pipefail

BILLING_REGISTER_SERVICE_URL="${BILLING_REGISTER_SERVICE_URL:-http://localhost:8087}"
TENANT_ID="${TENANT_ID:-91babc18-1fe0-4df3-8d2c-b350e6052b33}"
CARRIER_COMPANY_ID="${CARRIER_COMPANY_ID:-11365de1-4464-406f-b381-d3ac3ff872fd}"
SHIPPER_COMPANY_ID="${SHIPPER_COMPANY_ID:-0ee8c11d-b3dd-4502-a5ec-de0564abba4d}"
USER_ID="${USER_ID:-e82f7433-4994-486d-9ebd-167038ca233c}"
SHIPMENT_ID="${SHIPMENT_ID:-24235d34-6c8f-4beb-a0c8-8e0039cb7455}"
RUN_ID="repro-$(date +%s)"

REG="$(curl -sS -X POST "$BILLING_REGISTER_SERVICE_URL/v1/billing-registers" \
  -H 'Content-Type: application/json' \
  -d "{\"tenant_id\":\"$TENANT_ID\",\"register_number\":\"BR-$RUN_ID\",\"customer_company_id\":\"$SHIPPER_COMPANY_ID\",\"contractor_company_id\":\"$CARRIER_COMPANY_ID\",\"period_from\":\"2026-07-01\",\"period_to\":\"2026-07-15\",\"currency_code\":\"RUB\",\"vat_rate\":20}")"
REGISTER_ID="$(echo "$REG" | jq -r .id)"
echo "Prepared APPROVED register: $REGISTER_ID (status=$(echo "$REG" | jq -r .status))"

curl -sS -X POST "$BILLING_REGISTER_SERVICE_URL/v1/billing-registers/$REGISTER_ID/items" \
  -H 'Content-Type: application/json' \
  -d "{\"tenant_id\":\"$TENANT_ID\",\"shipment_id\":\"$SHIPMENT_ID\",\"route_description\":\"repro\",\"pickup_date\":\"2026-07-01\",\"delivery_date\":\"2026-07-03\",\"base_amount\":100000,\"extra_charges\":5000,\"penalties\":0,\"vat_rate\":20}" >/dev/null
curl -sS -X POST "$BILLING_REGISTER_SERVICE_URL/v1/billing-registers/$REGISTER_ID/calculate" \
  -H 'Content-Type: application/json' \
  -d "{\"tenant_id\":\"$TENANT_ID\"}" >/dev/null
APPROVED="$(curl -sS -X POST "$BILLING_REGISTER_SERVICE_URL/v1/billing-registers/$REGISTER_ID/approve" \
  -H 'Content-Type: application/json' \
  -d "{\"tenant_id\":\"$TENANT_ID\",\"approved_by\":\"$USER_ID\"}")"
echo "After approve: $(echo "$APPROVED" | jq -c '{status,total_with_vat}')"
echo

bash "$(dirname "$0")/upd-400-repro.sh" "$TENANT_ID" "$REGISTER_ID" "$CARRIER_COMPANY_ID" "$SHIPPER_COMPANY_ID"
