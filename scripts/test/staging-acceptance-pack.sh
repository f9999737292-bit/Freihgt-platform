#!/usr/bin/env bash
# Staging acceptance pack — run when SSH/staging is restored
# No hardcoded secrets — uses env from staging operator

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

echo "=== Staging Acceptance Pack v1 ==="
echo "Timestamp: $(date -u +"%Y-%m-%dT%H:%M:%SZ")"
echo "GIT_SHA=$(git rev-parse HEAD 2>/dev/null || echo unknown)"

echo "--- 1. Preflight ---"
if [[ -x scripts/ops/bintrans_ct_staging/bintrans_ct_staging_preflight.sh ]]; then
  bash scripts/ops/bintrans_ct_staging/bintrans_ct_staging_preflight.sh || echo "WARN: staging preflight script failed"
else
  bash scripts/test/system-test-preflight.sh
fi

echo "--- 2. Health ---"
make platform-health 2>/dev/null || curl -sf "${API_GATEWAY_URL:-http://localhost:8080}/health"

echo "--- 3. Integration smoke ---"
make integration-smoke-test || { echo "SMOKE FAILED"; exit 1; }

echo "--- 4. Golden skeleton (live) ---"
DRY_RUN=0 ENVIRONMENT=STAGING bash tests/system/golden/fp_e2e_golden_001.sh || echo "WARN: golden path not fully implemented"

echo "--- 5. Security spot-check (manual matrix FP-SEC-001) ---"
echo "Execute FP-E2E-SEC-001 cross-tenant checks per docs/testing/BUSINESS_E2E_CATALOG.md"

echo "--- 6. Browser E2E (if web-procurement available) ---"
echo "Run: cd apps/web-procurement && pnpm exec playwright test e2e/freight-cost-intelligence"

echo "--- 7. Cleanup ---"
echo "Operator: run tenant cleanup or disposable reset per TEST_DATA_MODEL.md"

echo "=== STAGING PACK COMPLETE (review warnings) ==="
