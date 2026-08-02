#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT}"

fail() { echo "rollback-acceptance: $*" >&2; exit 1; }
require_cmd() { command -v "$1" >/dev/null 2>&1 || fail "$1 is required"; }
require_cmd curl

if ! curl -fsS "${GATEWAY_URL:-http://127.0.0.1:8080}/health" >/dev/null 2>&1; then
  fail "GATEWAY_URL health check failed — start shadow stack first"
fi
if ! curl -fsS "${READ_MODEL_URL:-http://127.0.0.1:8089}/health" >/dev/null 2>&1; then
  fail "READ_MODEL_URL health check failed — start shadow stack first"
fi

# Tenant B rollback section of live acceptance script.
ROLLBACK_ONLY=1 exec "${ROOT}/scripts/dev/control_tower_projection_rebuild_live_acceptance.sh"
