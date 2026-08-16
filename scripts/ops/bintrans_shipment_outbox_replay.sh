#!/usr/bin/env bash
# BINTRANS operator wrapper for shipment outbox FAILED replay tooling.
# Default: dry-run. Pass --execute only after operator approval.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BIN="${ROOT}/services/shipment-service/shipment-outbox-replay"

if [[ ! -x "${BIN}" ]]; then
  echo "ERROR: build the CLI first: (cd services/shipment-service && go build -o shipment-outbox-replay ./cmd/shipment-outbox-replay)" >&2
  exit 2
fi

if [[ -z "${DATABASE_URL:-}" ]]; then
  echo "ERROR: DATABASE_URL is required" >&2
  exit 2
fi

exec "${BIN}" "$@"
