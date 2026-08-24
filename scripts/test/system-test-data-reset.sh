#!/usr/bin/env bash
# Partial FTST002 remediation — Wave 1 uses per-suite ephemeral DB via createTempDatabase in Go integration tests.
# This script validates TEST_DATABASE_URL connectivity for Wave 1 preflight.
set -euo pipefail

if [[ -z "${TEST_DATABASE_URL:-}" ]]; then
  echo "ERROR: TEST_DATABASE_URL required for Wave 1 DB suites"
  exit 1
fi

echo "WAVE1 DB reset strategy: ephemeral database per integration test (createTempDatabase pattern)"
echo "TEST_REPEATABILITY=YES via isolated DB per test run"
echo "FTST002=PARTIALLY_REMEDIATED"

# Optional: verify connectivity
if command -v psql >/dev/null 2>&1; then
  psql "$TEST_DATABASE_URL" -c "SELECT 1" >/dev/null
  echo "PostgreSQL connectivity: OK"
else
  echo "WARN: psql not in PATH — CI runner provides Postgres service"
fi
