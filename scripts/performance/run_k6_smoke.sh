#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

if ! command -v k6 >/dev/null 2>&1; then
  echo "k6 not found. Install from https://k6.io/docs/get-started/installation/"
  exit 1
fi

k6 run tests/performance/k6/smoke.js
