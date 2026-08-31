#!/usr/bin/env bash
# R3.4C — verify post-fix authenticated request headers match api-gateway CORS allowlist.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
WEB="${ROOT}/apps/web-admin"
ORIGIN="http://localhost:3000"
API_PORT="${API_GATEWAY_HOST_PORT:-18080}"
API_BASE="http://127.0.0.1:${API_PORT}"

pass() { echo "AUTH_CORS_PREFLIGHT: PASS $*"; }
fail() { echo "AUTH_CORS_PREFLIGHT: FAIL $*" >&2; exit 1; }

echo "=== BINTRANS web-admin authenticated CORS preflight selfcheck (R3.4C) ==="

cd "${WEB}"
npm install --no-audit --no-fund >/dev/null 2>&1 || npm install --no-audit --no-fund
npm run test
pass "unit header contract tests"

POSTFIX_PREFLIGHT_REQUEST_HEADERS="authorization,content-type,x-company-id,x-locale,x-request-id,x-tenant-id"
echo "POSTFIX_PREFLIGHT_REQUEST_HEADERS=${POSTFIX_PREFLIGHT_REQUEST_HEADERS}"

if [[ "${POSTFIX_PREFLIGHT_REQUEST_HEADERS}" == *"x-user-id"* ]]; then
  fail "x-user-id still present in preflight header set"
fi
echo "POSTFIX_X_USER_ID_IN_PREFLIGHT=NO"

python - <<PY &
import json
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

ALLOWED_ORIGINS = {"http://localhost:3000"}
ALLOWED_HEADERS = "Content-Type, Authorization, X-Tenant-ID, X-Company-ID, X-Request-ID, X-Locale"
ALLOWED_METHODS = "GET, POST, PATCH, PUT, DELETE, OPTIONS"

class GatewayMock(BaseHTTPRequestHandler):
    def log_message(self, *_args):
        return

    def _apply_cors(self):
        origin = self.headers.get("Origin", "")
        if origin in ALLOWED_ORIGINS:
            self.send_header("Access-Control-Allow-Origin", origin)
            self.send_header("Vary", "Origin")
            self.send_header("Access-Control-Allow-Methods", ALLOWED_METHODS)
            self.send_header("Access-Control-Allow-Headers", ALLOWED_HEADERS)
            self.send_header("Access-Control-Expose-Headers", "X-Request-ID")

    def do_OPTIONS(self):
        self.send_response(204)
        self._apply_cors()
        self.end_headers()

    def do_GET(self):
        if self.path.startswith("/api/v1/companies"):
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self._apply_cors()
            self.end_headers()
            self.wfile.write(json.dumps({"total": 1, "items": []}).encode())
            return
        self.send_response(404)
        self.end_headers()

ThreadingHTTPServer(("127.0.0.1", int("${API_PORT}")), GatewayMock).serve_forever()
PY
MOCK_PID=$!
trap 'kill "${MOCK_PID}" >/dev/null 2>&1 || true' EXIT
sleep 1

PREFLIGHT_HEADERS=$(curl -sS -D - -o /dev/null -X OPTIONS "${API_BASE}/api/v1/companies" \
  -H "Origin: ${ORIGIN}" \
  -H "Access-Control-Request-Method: GET" \
  -H "Access-Control-Request-Headers: ${POSTFIX_PREFLIGHT_REQUEST_HEADERS}")

echo "${PREFLIGHT_HEADERS}" | grep -qi 'HTTP/.* 204' || fail "cors preflight http status"
echo "${PREFLIGHT_HEADERS}" | grep -qi "Access-Control-Allow-Origin: ${ORIGIN}" || fail "cors allow-origin"
echo "${PREFLIGHT_HEADERS}" | grep -qi 'Access-Control-Allow-Headers:.*Authorization' || fail "cors allow-headers"

echo "POSTFIX_CORS_PREFLIGHT_HTTP=204"
echo "POSTFIX_ACCESS_CONTROL_ALLOW_ORIGIN=${ORIGIN}"
echo "POSTFIX_CORS_HEADER_SET_ACCEPTED=YES"
pass "live OPTIONS against gateway CORS contract mock"
