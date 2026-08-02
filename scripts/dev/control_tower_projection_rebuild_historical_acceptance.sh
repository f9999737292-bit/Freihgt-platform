#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT}"

fail() { echo "historical-acceptance: $*" >&2; exit 1; }

command -v go >/dev/null 2>&1 || fail "go is required"

if [[ -n "${GATEWAY_URL:-}" && -n "${JWT:-${JWT_TOKEN:-}}" && -n "${TENANT_ID:-}" ]]; then
  echo "historical-acceptance: gateway-backed mode (GATEWAY_URL=${GATEWAY_URL})" >&2
  go test -tags="integration acceptance" \
    ./services/control-tower-read-model-service/internal/integration/rebuild/... \
    -run '^TestHistoricalAcceptanceIntegration$' -count=1 -v
  exit 0
fi

echo "historical-acceptance: PostgreSQL fixture mode (set GATEWAY_URL+JWT+TENANT_ID for live shadow)" >&2
RUN_REBUILD_ACCEPTANCE_FIXTURE=1 go test -tags="integration acceptance" \
  ./services/control-tower-read-model-service/internal/integration/rebuild/... \
  -run '^TestHistoricalAcceptanceIntegration$' -count=1 -v
