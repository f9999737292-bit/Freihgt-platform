#!/usr/bin/env bash
# Offline self-check for registry digest validation helper (alias).
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
exec "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_runtime_images_validate_selfcheck.sh" "$@"
