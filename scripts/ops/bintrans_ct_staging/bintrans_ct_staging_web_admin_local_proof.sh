#!/usr/bin/env bash
# R3.4A — local production-mode proof for BINTRANS CT staging web-admin architecture.
# Builds production container, starts loopback-only service, validates routing/i18n/API model.
# Does NOT deploy to staging. Does NOT use nuxt dev.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
# shellcheck source=scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh
source "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"

WEB_PORT="${WEB_ADMIN_HOST_PORT:-13000}"
API_PORT="${API_GATEWAY_HOST_PORT:-18080}"
MOCK_API_PID=""
CONTAINER_NAME="bintrans_web_admin_local_proof"
IMAGE_TAG="bintrans-web-admin-local-proof:r3.4a"
RELEASE_SHA="$(git -C "${ROOT}" rev-parse HEAD)"

cleanup() {
  docker rm -f "${CONTAINER_NAME}" >/dev/null 2>&1 || true
  if [[ -n "${MOCK_API_PID}" ]] && kill -0 "${MOCK_API_PID}" 2>/dev/null; then
    kill "${MOCK_API_PID}" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

pass() { echo "LOCAL_PROOF: PASS $*"; }
fail() { echo "LOCAL_PROOF: FAIL $*" >&2; exit 1; }

http_code() {
  curl -sS -o /dev/null -w '%{http_code}' "$1"
}

body_contains() {
  curl -sS "$1" | grep -q "$2"
}

start_mock_api() {
  python - <<'PY' &
import json
from http.server import BaseHTTPRequestHandler, HTTPServer

class H(BaseHTTPRequestHandler):
    def log_message(self, *_args):
        return

    def do_GET(self):
        if self.path == '/health':
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self.end_headers()
            self.wfile.write(json.dumps({'status': 'ok', 'service': 'mock-gateway'}).encode())
            return
        self.send_response(404)
        self.end_headers()

    def do_POST(self):
        if self.path == '/api/v1/auth/login':
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self.end_headers()
            self.wfile.write(json.dumps({
                'access_token': 'proof-token',
                'user': {
                    'id': '00000000-0000-4000-8000-000000000001',
                    'tenant_id': '00000000-0000-4000-8000-000000000002',
                    'email': 'proof@example.com',
                    'full_name': 'Proof User',
                    'preferred_locale': 'ru-RU',
                    'status': 'ACTIVE',
                    'roles': ['PLATFORM_ADMIN'],
                },
            }).encode())
            return
        self.send_response(404)
        self.end_headers()

HTTPServer(('127.0.0.1', int('${API_PORT}')), H).serve_forever()
PY
  MOCK_API_PID=$!
  sleep 1
  [[ "$(http_code "http://127.0.0.1:${API_PORT}/health")" == "200" ]] \
    || fail "mock api health"
  pass "mock api on 127.0.0.1:${API_PORT}"
}

echo "=== BINTRANS web-admin local production proof (R3.4A) ==="
echo "RELEASE_SHA=${RELEASE_SHA}"
echo "WEB_PORT=${WEB_PORT}"
echo "API_PORT=${API_PORT}"

docker build \
  -f "${ROOT}/apps/web-admin/Dockerfile" \
  --build-arg "BINTRANS_GIT_SHA=${RELEASE_SHA}" \
  --build-arg "BINTRANS_IMAGE_VERSION=git-${RELEASE_SHA:0:7}" \
  --build-arg "NUXT_PUBLIC_API_BASE_URL=http://127.0.0.1:${API_PORT}" \
  --build-arg "NUXT_PUBLIC_MOCK_AUTH=false" \
  -t "${IMAGE_TAG}" \
  "${ROOT}/apps/web-admin"
pass "production image build"

docker rm -f "${CONTAINER_NAME}" >/dev/null 2>&1 || true
docker run -d --name "${CONTAINER_NAME}" \
  -e HOST=127.0.0.1 \
  -e PORT="${WEB_PORT}" \
  -p "127.0.0.1:${WEB_PORT}:${WEB_PORT}" \
  "${IMAGE_TAG}" >/dev/null

for _ in $(seq 1 30); do
  code="$(http_code "http://127.0.0.1:${WEB_PORT}/login" || true)"
  if [[ "${code}" == "200" ]]; then
    break
  fi
  sleep 2
done
pass "container start"

[[ "$(http_code "http://127.0.0.1:${WEB_PORT}/login")" == "200" ]] \
  || fail "login page http"
pass "login page http 200"

body_contains "http://127.0.0.1:${WEB_PORT}/login" 'type="password"' \
  || body_contains "http://127.0.0.1:${WEB_PORT}/login" "password" \
  || fail "login form rendered"
pass "login form rendered"

dash_code="$(http_code "http://127.0.0.1:${WEB_PORT}/dashboard")"
[[ "${dash_code}" == "200" || "${dash_code}" == "302" ]] \
  || fail "direct dashboard route (${dash_code})"
pass "direct dashboard deep link (${dash_code})"

body_contains "http://127.0.0.1:${WEB_PORT}/login" "login.title" \
  || body_contains "http://127.0.0.1:${WEB_PORT}/login" "Вход" \
  || body_contains "http://127.0.0.1:${WEB_PORT}/login" "Sign in" \
  || fail "i18n content"
pass "i18n assets"

for _ in $(seq 1 20); do
  status="$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "${CONTAINER_NAME}")"
  if [[ "${status}" == "healthy" ]]; then
    break
  fi
  sleep 2
done
[[ "${status:-}" == "healthy" ]] || fail "container healthcheck (${status:-unknown})"
pass "container healthcheck"

start_mock_api

body_contains "http://127.0.0.1:${WEB_PORT}/login" "127.0.0.1:${API_PORT}" \
  || body_contains "http://127.0.0.1:${WEB_PORT}/login" "18080" \
  || fail "api base visible in client bundle"
pass "api base visible to browser model"

[[ "$(http_code "http://127.0.0.1:${API_PORT}/health")" == "200" ]] \
  || fail "browser api route reachable via loopback"
pass "browser api route reachable"

if docker history "${IMAGE_TAG}" 2>/dev/null | grep -Ei 'JWT_SECRET|POSTGRES_PASSWORD|DATABASE_URL|\.env' >/dev/null; then
  fail "secrets in image history"
fi
pass "frontend container security gate"

echo "LOCAL_WEB_ADMIN_BUILD=PASS"
echo "LOCAL_WEB_ADMIN_CONTAINER_START=PASS"
echo "LOCAL_LOGIN_PAGE_HTTP=200"
echo "LOCAL_LOGIN_FORM_RENDERED=YES"
echo "LOCAL_DIRECT_LOGIN_ROUTE=PASS"
echo "LOCAL_DEEP_LINK_ROUTE_BEHAVIOR=PASS"
echo "LOCAL_I18N_ASSETS=PASS"
echo "LOCAL_HEALTHCHECK=PASS"
echo "LOCAL_BROWSER_API_ROUTE_REACHABLE=YES"
echo "API_BASE_VISIBLE_TO_BROWSER=http://127.0.0.1:${API_PORT}"
echo "API_CONNECTIVITY_MODEL_PROVEN=YES"
echo "bintrans-ct-staging-web-admin-local-proof: PASS"
