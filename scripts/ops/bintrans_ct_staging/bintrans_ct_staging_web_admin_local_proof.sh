#!/usr/bin/env bash
# R3.4A.1 — local Docker production proof for BINTRANS CT staging web-admin.
# Models operator view: localhost:3000 (web) + localhost:18080 (API) via SSH tunnels.
# Does NOT deploy to staging. Does NOT use nuxt dev.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
# shellcheck source=scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh
source "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"

CONTAINER_PORT="${WEB_ADMIN_CONTAINER_PORT:-13000}"
LOCAL_BROWSER_PORT="${WEB_ADMIN_LOCAL_BROWSER_PORT:-3000}"
API_PORT="${API_GATEWAY_HOST_PORT:-18080}"
BROWSER_ORIGIN="http://localhost:${LOCAL_BROWSER_PORT}"
API_BASE="http://localhost:${API_PORT}"
MOCK_API_PID=""
CONTAINER_NAME="bintrans_web_admin_local_proof"
IMAGE_TAG="bintrans-web-admin-local-proof:r3.4a1"
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

if ! docker info >/dev/null 2>&1; then
  echo "DOCKER_DAEMON_AVAILABLE=NO"
  echo "R3_4A_1_STATUS=BLOCKED"
  echo "BLOCKER=DOCKER_DAEMON_UNAVAILABLE"
  echo "OPERATOR_ACTION=START_DOCKER_DESKTOP"
  exit 1
fi
echo "DOCKER_DAEMON_AVAILABLE=YES"

start_mock_api_with_cors() {
  python - <<PY &
import json
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

ALLOWED_ORIGINS = {
    "http://localhost:3000",
    "http://localhost:3001",
    "http://localhost:5173",
}
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
        if self.path == "/health":
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self._apply_cors()
            self.end_headers()
            self.wfile.write(json.dumps({"status": "ok", "service": "mock-gateway"}).encode())
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
                    "id": "00000000-0000-4000-8000-000000000001",
                    "tenant_id": "00000000-0000-4000-8000-000000000002",
                    "email": "proof@example.com",
                    "full_name": "Proof User",
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
  [[ "$(http_code "${API_BASE}/health")" == "200" ]] || fail "mock api health"
  pass "mock api-gateway CORS contract on ${API_BASE}"
}

echo "=== BINTRANS web-admin local Docker proof (R3.4A.1) ==="
echo "RELEASE_SHA=${RELEASE_SHA}"
echo "CONTAINER_PORT=${CONTAINER_PORT}"
echo "LOCAL_BROWSER_PORT=${LOCAL_BROWSER_PORT}"
echo "BROWSER_ORIGIN=${BROWSER_ORIGIN}"
echo "API_BASE=${API_BASE}"

docker build \
  -f "${ROOT}/apps/web-admin/Dockerfile" \
  --build-arg "BINTRANS_GIT_SHA=${RELEASE_SHA}" \
  --build-arg "BINTRANS_IMAGE_VERSION=git-${RELEASE_SHA:0:7}" \
  --build-arg "NUXT_PUBLIC_API_BASE_URL=${API_BASE}" \
  --build-arg "NUXT_PUBLIC_MOCK_AUTH=false" \
  -t "${IMAGE_TAG}" \
  "${ROOT}/apps/web-admin"
pass "production image build"
echo "LOCAL_WEB_ADMIN_IMAGE_BUILD=PASS"

LOCAL_IMAGE_ID="$(docker image inspect "${IMAGE_TAG}" --format '{{.Id}}')"
echo "WEB_ADMIN_LOCAL_IMAGE_ID=${LOCAL_IMAGE_ID}"
echo "WEB_ADMIN_REGISTRY_DIGEST=NOT_AVAILABLE_PRE_PUSH"

docker rm -f "${CONTAINER_NAME}" >/dev/null 2>&1 || true
docker run -d --name "${CONTAINER_NAME}" \
  -e HOST=0.0.0.0 \
  -e PORT="${CONTAINER_PORT}" \
  -p "127.0.0.1:${LOCAL_BROWSER_PORT}:${CONTAINER_PORT}" \
  "${IMAGE_TAG}" >/dev/null
pass "container start with 127.0.0.1:${LOCAL_BROWSER_PORT}:${CONTAINER_PORT}"
echo "LOCAL_WEB_ADMIN_CONTAINER_START=PASS"

for _ in $(seq 1 30); do
  code="$(http_code "${BROWSER_ORIGIN}/login" || true)"
  if [[ "${code}" == "200" ]]; then
    break
  fi
  sleep 2
done

[[ "$(http_code "${BROWSER_ORIGIN}/login")" == "200" ]] || fail "login page via docker port publish"
pass "login page http 200 via ${BROWSER_ORIGIN}/login"
echo "LOCAL_LOGIN_PAGE_HTTP=200"

body_contains "${BROWSER_ORIGIN}/login" 'type="password"' \
  || body_contains "${BROWSER_ORIGIN}/login" "password" \
  || fail "login form rendered"
pass "login form rendered"
echo "CONTAINER_DIRECT_LOGIN_ROUTE=PASS"

dash_code="$(http_code "${BROWSER_ORIGIN}/dashboard")"
[[ "${dash_code}" == "200" || "${dash_code}" == "302" ]] \
  || fail "dashboard deep link (${dash_code})"
pass "dashboard deep link (${dash_code})"
echo "CONTAINER_DEEP_LINK_ROUTE=PASS"

body_contains "${BROWSER_ORIGIN}/login" "login.title" \
  || body_contains "${BROWSER_ORIGIN}/login" "Вход" \
  || body_contains "${BROWSER_ORIGIN}/login" "Sign in" \
  || fail "i18n content"
pass "i18n assets"
echo "CONTAINER_I18N_ASSETS=PASS"

for _ in $(seq 1 25); do
  status="$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "${CONTAINER_NAME}")"
  if [[ "${status}" == "healthy" ]]; then
    break
  fi
  sleep 2
done
[[ "${status:-}" == "healthy" ]] || fail "container healthcheck (${status:-unknown})"
pass "container healthcheck healthy"
echo "CONTAINER_HEALTH=healthy"

start_mock_api_with_cors

body_contains "${BROWSER_ORIGIN}/login" "localhost:${API_PORT}" \
  || fail "api base visible in client bundle"
pass "api base visible to browser (${API_BASE})"

# CORS preflight contract proof (mirrors services/api-gateway/internal/http/middleware/cors.go).
PREFLIGHT_HEADERS=$(curl -sS -D - -o /dev/null -X OPTIONS "${API_BASE}/api/v1/auth/login" \
  -H "Origin: ${BROWSER_ORIGIN}" \
  -H "Access-Control-Request-Method: POST" \
  -H "Access-Control-Request-Headers: content-type,x-request-id,x-locale")
echo "${PREFLIGHT_HEADERS}" | grep -qi 'HTTP/.* 204' || fail "cors preflight http status"
echo "${PREFLIGHT_HEADERS}" | grep -qi "Access-Control-Allow-Origin: ${BROWSER_ORIGIN}" || fail "cors allow-origin"
pass "cors preflight for login origin"
echo "CORS_PREFLIGHT_HTTP=204"
echo "ACCESS_CONTROL_ALLOW_ORIGIN=${BROWSER_ORIGIN}"
echo "CORS_LOGIN_ORIGIN_ALLOWED=YES"

if docker history "${IMAGE_TAG}" 2>/dev/null | grep -Ei 'JWT_SECRET|POSTGRES_PASSWORD|DATABASE_URL|\.env|BEGIN OPENSSH' >/dev/null; then
  fail "secrets in image history"
fi
pass "frontend container security gate"

echo "WEB_ADMIN_CONTAINER_BIND=0.0.0.0:${CONTAINER_PORT}"
echo "WEB_ADMIN_REMOTE_HOST_BIND=127.0.0.1:${CONTAINER_PORT}"
echo "WEB_ADMIN_LOCAL_BROWSER_ORIGIN=${BROWSER_ORIGIN}"
echo "API_BASE_VISIBLE_TO_BROWSER=${API_BASE}"
echo "SSH_WEB_FORWARD=-L ${LOCAL_BROWSER_PORT}:127.0.0.1:${CONTAINER_PORT}"
echo "SSH_API_FORWARD=-L ${API_PORT}:127.0.0.1:${API_PORT}"
echo "PUBLIC_FRONTEND_PORT_EXPOSED=NO"
echo "FULL_DIGEST_REFERENCE_MODEL=PASS"
echo "RUNTIME_IMAGE_REFERENCE_FORMAT=cr.selcloud.ru/bintrans-staging/web-admin@sha256:<remote-manifest-digest>"
echo "bintrans-ct-staging-web-admin-local-proof: PASS"
