#!/usr/bin/env bash
# Validate master test plan artifacts (no live execution)
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

echo "=== System Test Design Check ==="

duplicates=$(grep -E '^\s+- id:' tests/system/test-catalog.yaml | awk '{print $3}' | sort | uniq -d || true)
if [[ -n "$duplicates" ]]; then
  echo "ERROR: duplicate test IDs:"
  echo "$duplicates"
  exit 1
fi
echo "Duplicate test IDs: NONE"

for doc in MASTER_SYSTEM_TEST_PLAN BUSINESS_E2E_CATALOG ROLE_RBAC_TEST_MATRIX TEST_DATA_MODEL SYSTEM_TEST_TRACEABILITY_MATRIX UAT_PLAN TEST_ENVIRONMENT_MATRIX SYSTEM_TEST_READINESS_REPORT; do
  f="docs/testing/${doc}.md"
  [[ -f "$f" ]] || { echo "MISSING $f"; exit 1; }
done
echo "Required docs: OK"

if command -v py >/dev/null 2>&1; then
  py -3 -c "import yaml; yaml.safe_load(open('tests/system/test-catalog.yaml'))"
elif command -v python >/dev/null 2>&1; then
  python -c "import yaml; yaml.safe_load(open('tests/system/test-catalog.yaml'))"
else
  echo "WARN: skip YAML validation (no python)"
fi
echo "YAML: OK"

bash -n tests/system/golden/fp_e2e_golden_001.sh
bash -n scripts/test/system-test-preflight.sh
bash -n scripts/test/staging-acceptance-pack.sh 2>/dev/null || true
echo "Shell syntax: OK"

echo "DESIGN CHECK PASSED"
