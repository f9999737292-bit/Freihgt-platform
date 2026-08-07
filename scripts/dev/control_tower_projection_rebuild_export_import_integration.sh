#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

echo "Running export/import PostgreSQL integration tests (twice)..."

run_once() {
  go test -tags=integration ./services/shipment-service/internal/integration/statussnapshot/... -count=1
  go test -tags=integration ./services/control-tower-read-model-service/internal/integration/rebuild/... -count=1
}

run_once
run_once

echo "Export/import integration tests passed."
