#!/usr/bin/env bash
# Delegate to canonical runtime image digest validator (backward-compatible alias).
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
exec "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_runtime_images_validate.sh" "$@"
