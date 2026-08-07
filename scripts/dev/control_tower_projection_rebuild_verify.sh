#!/usr/bin/env bash
set -euo pipefail

echo "Projection rebuild verify: run dry-run, status, and diff checks"
make control-tower-projection-rebuild-test
git diff --check
