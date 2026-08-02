#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT}"

fail() { echo "rollback-acceptance: $*" >&2; exit 1; }

command -v go >/dev/null 2>&1 || fail "go is required"

go test -tags="integration acceptance" \
  ./services/control-tower-read-model-service/internal/integration/rebuild/... \
  -run '^TestRollbackAcceptanceIntegration$' -count=1 -v
