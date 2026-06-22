#!/usr/bin/env bash
# Dev-only: create a DRAFT low-code form template via API Gateway.
set -euo pipefail

TENANT_ID="${TENANT_ID:-74519f22-ff9b-4a8b-8fff-a958c689682f}"
GATEWAY_URL="${GATEWAY_URL:-http://localhost:8080}"
PAYLOAD_FILE="${PAYLOAD_FILE:-scripts/dev/payloads/lowcode_form_template_draft_transport_order.json}"
USER_ID="${USER_ID:-00000000-0000-4000-8000-000000000001}"

step() { echo "==> $1" >&2; }
pass() { echo "OK: $1" >&2; }
fail() { echo "ERROR: $1" >&2; exit 1; }

if [[ ! -f "$PAYLOAD_FILE" ]]; then
  fail "payload file not found: $PAYLOAD_FILE"
fi

step "Check API Gateway health"
if ! curl -sf "${GATEWAY_URL}/health" >/dev/null; then
  fail "API Gateway is not reachable at ${GATEWAY_URL}"
fi
pass "API Gateway healthy"

CODE="$(python - "$PAYLOAD_FILE" <<'PY'
import json, sys
with open(sys.argv[1], encoding="utf-8") as f:
    print(json.load(f)["code"])
PY
)"

step "Check if draft already exists: ${CODE}"
list_file="$(mktemp)"
curl -sf \
  -H "X-Tenant-ID: ${TENANT_ID}" \
  "${GATEWAY_URL}/api/v1/low-code/admin/form-templates?status=DRAFT&entity_type=TRANSPORT_ORDER" \
  > "$list_file"

existing_count="$(python - "$list_file" "$CODE" <<'PY'
import json, sys
with open(sys.argv[1], encoding="utf-8") as f:
    data = json.load(f)
code = sys.argv[2]
print(sum(1 for item in data.get("items", []) if item.get("code") == code))
PY
)"

rm -f "$list_file"

if [[ "${existing_count:-0}" != "0" ]]; then
  pass "draft template already exists: ${CODE} (skip create)"
  exit 0
fi

step "Create draft form template via admin API"
response_file="$(mktemp)"
http_code="$(curl -sS -o "$response_file" -w "%{http_code}" \
  -X POST \
  -H "Content-Type: application/json; charset=utf-8" \
  -H "X-Tenant-ID: ${TENANT_ID}" \
  -H "X-User-ID: ${USER_ID}" \
  --data-binary "@${PAYLOAD_FILE}" \
  "${GATEWAY_URL}/api/v1/low-code/admin/form-templates")"

if [[ "$http_code" != "201" ]]; then
  echo "Response:" >&2
  cat "$response_file" >&2 || true
  rm -f "$response_file"
  fail "create draft failed with HTTP ${http_code}"
fi

template_id="$(python - "$response_file" <<'PY'
import json, sys
with open(sys.argv[1], encoding="utf-8") as f:
    print(json.load(f)["id"])
PY
)"
rm -f "$response_file"

pass "draft template created: id=${template_id} code=${CODE}"
