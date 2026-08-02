#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT}"

fail() { echo "historical-acceptance: $*" >&2; exit 1; }

require_cmd() { command -v "$1" >/dev/null 2>&1 || fail "$1 is required"; }
require_cmd docker
require_cmd curl

if curl -fsS "${GATEWAY_URL:-http://127.0.0.1:8080}/health" >/dev/null 2>&1 \
  && curl -fsS "${READ_MODEL_URL:-http://127.0.0.1:8089}/health" >/dev/null 2>&1; then
  echo "historical-acceptance: live gateway mode" >&2
  exec "${ROOT}/scripts/dev/control_tower_projection_rebuild_live_acceptance.sh"
fi

echo "historical-acceptance: missing prerequisites: shadow stack (GATEWAY_URL/READ_MODEL_URL health)" >&2
echo "  start: docker compose -f infrastructure/docker-compose/docker-compose.yml \\" >&2
echo "    -f infrastructure/docker-compose/docker-compose.staging-shadow.yml \\" >&2
echo "    --profile messaging --profile read-model --profile observability up -d" >&2
exit 2
