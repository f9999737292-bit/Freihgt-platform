#!/usr/bin/env bash
# System test preflight — disposable stack readiness
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

echo "=== System Test Preflight ==="
echo "GIT_SHA=$(git rev-parse HEAD 2>/dev/null || echo unknown)"

fail=0

check_file() {
  [[ -f "$1" ]] || { echo "MISSING: $1"; fail=1; }
}

check_file docs/testing/MASTER_SYSTEM_TEST_PLAN.md
check_file tests/system/test-catalog.yaml
check_file tests/system/golden/fp_e2e_golden_001.sh

if command -v python >/dev/null 2>&1; then
  python -c "import yaml; yaml.safe_load(open('tests/system/test-catalog.yaml'))" && echo "YAML catalog: OK"
elif command -v py >/dev/null 2>&1; then
  py -3 -c "import yaml; yaml.safe_load(open('tests/system/test-catalog.yaml'))" && echo "YAML catalog: OK"
else
  echo "WARN: python not available — skip YAML parse"
fi

if command -v docker >/dev/null 2>&1; then
  if docker ps --format '{{.Names}}' | grep -q freight_postgres; then
    echo "PostgreSQL container: UP"
  else
    echo "PostgreSQL container: DOWN (run make dev-up or platform-up)"
  fi
else
  echo "WARN: docker not available"
fi

curl -sf http://localhost:8080/health >/dev/null 2>&1 && echo "API Gateway: UP" || echo "API Gateway: DOWN"

if [[ "$fail" -ne 0 ]]; then
  echo "PREFLIGHT FAILED"
  exit 1
fi
echo "PREFLIGHT OK"
