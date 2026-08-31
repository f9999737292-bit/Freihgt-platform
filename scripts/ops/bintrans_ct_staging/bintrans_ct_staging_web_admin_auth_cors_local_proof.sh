#!/usr/bin/env bash
# R3.4C — production Docker web-admin proof: authenticated dashboard fetch without CORS errors.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
WEB="${ROOT}/apps/web-admin"
CONTAINER_PORT="${WEB_ADMIN_CONTAINER_PORT:-13000}"
LOCAL_BROWSER_PORT="${WEB_ADMIN_LOCAL_BROWSER_PORT:-3000}"
API_PORT="${API_GATEWAY_HOST_PORT:-18080}"
BROWSER_ORIGIN="http://localhost:${LOCAL_BROWSER_PORT}"
API_BASE="http://localhost:${API_PORT}"
CONTAINER_NAME="bintrans_web_admin_auth_cors_proof"
IMAGE_TAG="bintrans-web-admin-auth-cors-proof:r3.4c"
RELEASE_SHA="$(git -C "${ROOT}" rev-parse HEAD)"
PILOT="${BINTRANS_PILOT_DIR:-/d/Projects/freight-platform-staging-pack/scripts/ops/bintrans_pilot}"
PLAYWRIGHT_BROWSERS_PATH="${PILOT}/.playwright-browsers"
TENANT_W2="${TENANT_W2:-285f9447-faf7-423e-96dd-e4c5e2b3fc6c}"
BUYER_EMAIL="${BUYER_EMAIL:-buyer@example.com}"
MOCK_API_PID=""
NUXT_PID=""

cleanup() {
  if [[ -n "${NUXT_PID:-}" ]] && kill -0 "${NUXT_PID}" 2>/dev/null; then
    kill "${NUXT_PID}" >/dev/null 2>&1 || true
  fi
  docker rm -f "${CONTAINER_NAME}" >/dev/null 2>&1 || true
  if [[ -n "${MOCK_API_PID}" ]] && kill -0 "${MOCK_API_PID}" 2>/dev/null; then
    kill "${MOCK_API_PID}" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

pass() { echo "AUTH_CORS_LOCAL: PASS $*"; }
fail() { echo "AUTH_CORS_LOCAL: FAIL $*" >&2; exit 1; }

if ! docker info >/dev/null 2>&1; then
  echo "LOCAL_AUTHENTICATED_API_REQUEST=BLOCKED"
  echo "BLOCKER=DOCKER_DAEMON_UNAVAILABLE"
  exit 1
fi

start_mock_api_with_gateway_cors() {
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
        auth = self.headers.get("Authorization", "")
        if self.path == "/health":
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self._apply_cors()
            self.end_headers()
            self.wfile.write(json.dumps({"status": "ok"}).encode())
            return
        if not auth.startswith("Bearer "):
            self.send_response(401)
            self.send_header("Content-Type", "application/json")
            self._apply_cors()
            self.end_headers()
            self.wfile.write(json.dumps({"error": {"code": "UNAUTHORIZED"}}).encode())
            return
        if self.path.startswith("/api/v1/"):
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self._apply_cors()
            self.end_headers()
            self.wfile.write(json.dumps({"total": 1, "items": []}).encode())
            return
        self.send_response(404)
        self.end_headers()

    def do_POST(self):
        if self.path == "/api/v1/auth/login":
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self._apply_cors()
            self.end_headers()
            self.wfile.write(json.dumps({
                "access_token": "proof-token",
                "user": {
                    "id": "83cb2447-75e9-41f2-8e0d-93c70f8506be",
                    "tenant_id": "${TENANT_W2}",
                    "email": "${BUYER_EMAIL}",
                    "full_name": "Proof Buyer",
                    "preferred_locale": "ru-RU",
                    "status": "ACTIVE",
                    "roles": ["PLATFORM_ADMIN"],
                },
            }).encode())
            return
        self.send_response(404)
        self.end_headers()

ThreadingHTTPServer(("127.0.0.1", int("${API_PORT}")), GatewayMock).serve_forever()
PY
  MOCK_API_PID=$!
  sleep 1
}

echo "=== BINTRANS web-admin authenticated CORS local proof (R3.4C) ==="
echo "RELEASE_SHA=${RELEASE_SHA}"
echo "BROWSER_ORIGIN=${BROWSER_ORIGIN}"
echo "API_BASE=${API_BASE}"

start_mock_api_with_gateway_cors

cd "${WEB}"
NUXT_PUBLIC_API_BASE_URL="${API_BASE}" NUXT_PUBLIC_MOCK_AUTH=false npm run build
pass "production nuxt build"

NUXT_PUBLIC_API_BASE_URL="${API_BASE}" PORT="${LOCAL_BROWSER_PORT}" HOST=127.0.0.1 node .output/server/index.mjs >/tmp/web-admin-auth-cors-preview.log 2>&1 &
NUXT_PID=$!

for _ in $(seq 1 30); do
  code="$(curl -sS -o /dev/null -w '%{http_code}' "${BROWSER_ORIGIN}/login" || true)"
  [[ "${code}" == "200" ]] && break
  sleep 2
done
[[ "$(curl -sS -o /dev/null -w '%{http_code}' "${BROWSER_ORIGIN}/login")" == "200" ]] || fail "login page"

PROOF_MJS="${PILOT}/.tmp-auth-cors-local-proof.mjs"
cat > "${PROOF_MJS}" <<'EOF'
import { chromium } from 'playwright';

const BASE = process.env.PILOT_UI_BASE_URL || 'http://localhost:3000';
const tenant = process.env.TENANT_W2 || '';
const buyerEmail = process.env.BUYER_EMAIL || 'buyer@example.com';
const buyerPassword = process.env.BUYER_PASSWORD || 'proof-password';

let corsErrors = 0;
let authRequest = false;
let companiesRequest = false;

const browser = await chromium.launch({ channel: 'chrome', headless: true });
const page = await browser.newPage();
page.on('console', (msg) => {
  if (msg.type() === 'error' && /CORS|blocked by/i.test(msg.text())) corsErrors++;
});
page.on('requestfinished', async (req) => {
  const url = req.url();
  if (req.method() === 'POST' && url.includes('/auth/login')) authRequest = true;
  if (req.method() === 'GET' && url.includes('/api/v1/companies')) companiesRequest = true;
});

await page.goto(`${BASE}/login`);
for (let i = 0; i < 15; i++) {
  if (/Backend доступен|Backend online/i.test(await page.locator('body').innerText())) break;
  await page.waitForTimeout(1000);
}
await page.locator('#login-tenant-id').fill(tenant);
await page.locator('#login-email').fill(buyerEmail);
await page.locator('#login-password').fill(buyerPassword);
await page.locator('form.login-form button[type="submit"]').click();
await page.waitForURL((u) => !u.pathname.includes('/login'), { timeout: 25000 });
await page.waitForTimeout(3000);

console.log(`LOCAL_AUTHENTICATED_API_REQUEST=${authRequest && companiesRequest ? 'PASS' : 'FAIL'}`);
console.log(`LOCAL_AUTHENTICATED_CORS_ERROR_COUNT=${corsErrors}`);
console.log(`AUTH_REQUEST=${authRequest ? 'YES' : 'NO'}`);
console.log(`COMPANIES_REQUEST=${companiesRequest ? 'YES' : 'NO'}`);
await browser.close();
process.exit(authRequest && companiesRequest && corsErrors === 0 ? 0 : 1);
EOF

export PLAYWRIGHT_BROWSERS_PATH="${PLAYWRIGHT_BROWSERS_PATH}"
export PILOT_UI_BASE_URL="${BROWSER_ORIGIN}"
export TENANT_W2="${TENANT_W2}"
export BUYER_EMAIL="${BUYER_EMAIL}"
export BUYER_PASSWORD="${BUYER_PASSWORD:-proof-password}"

if [[ ! -d "${PILOT}/node_modules/playwright" ]]; then
  fail "playwright not installed at ${PILOT}"
fi

(cd "${PILOT}" && node "${PROOF_MJS}") || fail "browser authenticated CORS proof"
rm -f "${PROOF_MJS}"
pass "authenticated dashboard fetch without CORS errors"
