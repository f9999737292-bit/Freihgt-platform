#!/usr/bin/env bash
set -euo pipefail

# v0.1 dry-run uses protocol fixtures because exporter PostgreSQL query is not implemented.
echo "Running protocol fixture dry-run (exporter query NOT_IMPLEMENTED in v0.1)..."

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

go test ./packages/statussnapshot/... -run 'TestEmptyAllScopeSnapshot|TestValidateStreamValid' -count=1
go test ./services/control-tower-read-model-service/internal/rebuild/... -run TestImporterDryRunValid -count=1

echo "Protocol fixture dry-run passed."
