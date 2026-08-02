#!/usr/bin/env bash
set -euo pipefail

# Real exporter repository dry-run via PostgreSQL integration tests.
echo "Running real PostgreSQL exporter dry-run validation..."

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

go test -tags=integration ./services/shipment-service/internal/integration/statussnapshot/... \
  -run 'TestPostgresRepository|TestExportImport' -count=1

go test ./services/control-tower-read-model-service/internal/rebuild/... \
  -run TestImporterDryRunValid -count=1

echo "Real exporter dry-run validation passed."
