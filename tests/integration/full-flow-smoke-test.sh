#!/usr/bin/env bash
set -euo pipefail

echo "Running full-flow smoke test..."
echo "Note: current full-flow-smoke-test is an alias to smoke-test.sh and should be expanded later."
bash "$(dirname "$0")/smoke-test.sh"
